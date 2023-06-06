/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datavolume

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// ErrResourceExists provides a const to indicate a resource exists error
	ErrResourceExists = "ErrResourceExists"
	// ErrResourceMarkedForDeletion provides a const to indicate a resource marked for deletion error
	ErrResourceMarkedForDeletion = "ErrResourceMarkedForDeletion"
	// ErrClaimLost provides a const to indicate a claim is lost
	ErrClaimLost = "ErrClaimLost"

	// MessageResourceMarkedForDeletion provides a const to form a resource marked for deletion error message
	MessageResourceMarkedForDeletion = "Resource %q marked for deletion"
	// MessageResourceExists provides a const to form a resource exists error message
	MessageResourceExists = "Resource %q already exists and is not managed by DataVolume"
	// MessageErrClaimLost provides a const to form claim lost message
	MessageErrClaimLost = "PVC %s lost"

	dvPhaseField = "status.phase"

	claimRefField = "spec.claimRef"

	claimStorageClassNameField = "spec.storageClassName"
)

var httpClient *http.Client

// Event represents DV controller event
type Event struct {
	eventType string
	reason    string
	message   string
}

type statusPhaseSync struct {
	phase  cdiv1.DataVolumePhase
	pvcKey *client.ObjectKey
	event  Event
}

type dvSyncResult struct {
	result    *reconcile.Result
	phaseSync *statusPhaseSync
}

type dvSyncState struct {
	dv        *cdiv1.DataVolume
	dvMutated *cdiv1.DataVolume
	pvc       *corev1.PersistentVolumeClaim
	pvcSpec   *corev1.PersistentVolumeClaimSpec
	dvSyncResult
}

// ReconcilerBase members
type ReconcilerBase struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	featureGates    featuregates.FeatureGates
	installerLabels map[string]string
}

func pvcIsPopulated(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
	if pvc == nil || dv == nil {
		return false
	}
	dvName, ok := pvc.Annotations[cc.AnnPopulatedFor]
	return ok && dvName == dv.Name
}

func dvIsPrePopulated(dv *cdiv1.DataVolume) bool {
	_, ok := dv.Annotations[cc.AnnPrePopulated]
	return ok
}

func checkStaticProvisionPending(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
	if pvc == nil || dv == nil {
		return false
	}
	if _, ok := dv.Annotations[cc.AnnCheckStaticVolume]; !ok {
		return false
	}
	_, ok := pvc.Annotations[cc.AnnPersistentVolumeList]
	return ok
}

func shouldSetDataVolumePending(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
	if checkStaticProvisionPending(pvc, dv) {
		return true
	}

	if pvc != nil {
		return false
	}

	return dvIsPrePopulated(dv) || (dv.Status.Phase == cdiv1.PhaseUnset)
}

type dataVolumeOp int

const (
	dataVolumeNop dataVolumeOp = iota
	dataVolumeImport
	dataVolumeUpload
	dataVolumePvcClone
	dataVolumeSnapshotClone
	dataVolumePopulator
)

type indexArgs struct {
	obj          client.Object
	field        string
	extractValue client.IndexerFunc
}

func getIndexArgs() []indexArgs {
	return []indexArgs{
		{
			obj:   &cdiv1.DataVolume{},
			field: dvPhaseField,
			extractValue: func(obj client.Object) []string {
				return []string{string(obj.(*cdiv1.DataVolume).Status.Phase)}
			},
		},
		{
			obj:   &corev1.PersistentVolume{},
			field: claimRefField,
			extractValue: func(obj client.Object) []string {
				if pv, ok := obj.(*corev1.PersistentVolume); ok {
					if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace != "" && pv.Spec.ClaimRef.Name != "" {
						return []string{claimRefIndexKeyFunc(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)}
					}
				}
				return nil
			},
		},
		{
			obj:   &corev1.PersistentVolume{},
			field: claimStorageClassNameField,
			extractValue: func(obj client.Object) []string {
				if pv, ok := obj.(*corev1.PersistentVolume); ok && pv.Status.Phase == corev1.VolumeAvailable {
					return []string{pv.Spec.StorageClassName}
				}
				return nil
			},
		},
	}
}

func claimRefIndexKeyFunc(namespace, name string) string {
	return namespace + "/" + name
}

// CreateCommonIndexes creates indexes used by all controllers
func CreateCommonIndexes(mgr manager.Manager) error {
	for _, ia := range getIndexArgs() {
		if err := mgr.GetFieldIndexer().IndexField(context.TODO(), ia.obj, ia.field, ia.extractValue); err != nil {
			return err
		}
	}
	return nil
}

func addDataVolumeControllerCommonWatches(mgr manager.Manager, dataVolumeController controller.Controller, op dataVolumeOp) error {
	appendMatchingDataVolumeRequest := func(reqs []reconcile.Request, mgr manager.Manager, namespace, name string) []reconcile.Request {
		dvKey := types.NamespacedName{Namespace: namespace, Name: name}
		dv := &cdiv1.DataVolume{}
		if err := mgr.GetClient().Get(context.TODO(), dvKey, dv); err != nil {
			if !k8serrors.IsNotFound(err) {
				mgr.GetLogger().Error(err, "Failed to get DV", "dvKey", dvKey)
			}
			return reqs
		}
		if getDataVolumeOp(mgr.GetLogger(), dv, mgr.GetClient()) == op {
			reqs = append(reqs, reconcile.Request{NamespacedName: dvKey})
		}
		return reqs
	}

	// Setup watches
	if err := dataVolumeController.Watch(&source.Kind{Type: &cdiv1.DataVolume{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			dv := obj.(*cdiv1.DataVolume)
			if getDataVolumeOp(mgr.GetLogger(), dv, mgr.GetClient()) != op {
				return nil
			}
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}}}
		}),
	); err != nil {
		return err
	}
	if err := dataVolumeController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			var result []reconcile.Request
			owner := metav1.GetControllerOf(obj)
			if owner != nil && owner.Kind == "DataVolume" {
				result = appendMatchingDataVolumeRequest(result, mgr, obj.GetNamespace(), owner.Name)
			}
			populatedFor := obj.GetAnnotations()[cc.AnnPopulatedFor]
			if populatedFor != "" {
				result = appendMatchingDataVolumeRequest(result, mgr, obj.GetNamespace(), populatedFor)
			}
			// it is okay if result contains the same entry twice, will be deduplicated by caller
			return result
		}),
	); err != nil {
		return err
	}
	if err := dataVolumeController.Watch(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			owner := metav1.GetControllerOf(obj)
			if owner == nil || owner.Kind != "DataVolume" {
				return nil
			}
			return appendMatchingDataVolumeRequest(nil, mgr, obj.GetNamespace(), owner.Name)
		}),
	); err != nil {
		return err
	}
	for _, k := range []client.Object{&corev1.PersistentVolumeClaim{}, &corev1.Pod{}, &cdiv1.ObjectTransfer{}} {
		if err := dataVolumeController.Watch(&source.Kind{Type: k}, handler.EnqueueRequestsFromMapFunc(
			func(obj client.Object) []reconcile.Request {
				if !hasAnnOwnedByDataVolume(obj) {
					return nil
				}
				namespace, name, err := getAnnOwnedByDataVolume(obj)
				if err != nil {
					return nil
				}
				return appendMatchingDataVolumeRequest(nil, mgr, namespace, name)
			}),
		); err != nil {
			return err
		}
	}

	// Watch for SC updates and reconcile the DVs waiting for default SC
	if err := dataVolumeController.Watch(&source.Kind{Type: &storagev1.StorageClass{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) (reqs []reconcile.Request) {
			dvList := &cdiv1.DataVolumeList{}
			if err := mgr.GetClient().List(context.TODO(), dvList, client.MatchingFields{dvPhaseField: ""}); err != nil {
				return
			}
			for _, dv := range dvList.Items {
				if getDataVolumeOp(mgr.GetLogger(), &dv, mgr.GetClient()) == op {
					reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
				}
			}
			return
		},
	),
	); err != nil {
		return err
	}

	// Watch for PV updates to reconcile the DVs waiting for available PV
	if err := dataVolumeController.Watch(&source.Kind{Type: &corev1.PersistentVolume{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) (reqs []reconcile.Request) {
			pv := obj.(*corev1.PersistentVolume)
			dvList := &cdiv1.DataVolumeList{}
			if err := mgr.GetClient().List(context.TODO(), dvList, client.MatchingFields{dvPhaseField: ""}); err != nil {
				return
			}
			for _, dv := range dvList.Items {
				storage := dv.Spec.Storage
				if storage != nil &&
					storage.StorageClassName != nil &&
					*storage.StorageClassName == pv.Spec.StorageClassName &&
					pv.Status.Phase == corev1.VolumeAvailable &&
					getDataVolumeOp(mgr.GetLogger(), &dv, mgr.GetClient()) == op {
					reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
				}
			}
			return
		},
	),
	); err != nil {
		return err
	}

	return nil
}

func getDataVolumeOp(log logr.Logger, dv *cdiv1.DataVolume, client client.Client) dataVolumeOp {
	src := dv.Spec.Source

	if dv.Spec.SourceRef != nil {
		return getSourceRefOp(log, dv, client)
	}
	if src != nil && src.PVC != nil {
		return dataVolumePvcClone
	}
	if src != nil && src.Snapshot != nil {
		return dataVolumeSnapshotClone
	}
	if src == nil {
		if dvUsesVolumePopulator(dv) {
			return dataVolumePopulator
		}
		return dataVolumeNop
	}
	if src.Upload != nil {
		return dataVolumeUpload
	}
	if src.HTTP != nil || src.S3 != nil || src.GCS != nil || src.Registry != nil || src.Blank != nil || src.Imageio != nil || src.VDDK != nil {
		return dataVolumeImport
	}

	return dataVolumeNop
}

func getSourceRefOp(log logr.Logger, dv *cdiv1.DataVolume, client client.Client) dataVolumeOp {
	dataSource := &cdiv1.DataSource{}
	ns := dv.Namespace
	if dv.Spec.SourceRef.Namespace != nil && *dv.Spec.SourceRef.Namespace != "" {
		ns = *dv.Spec.SourceRef.Namespace
	}
	nn := types.NamespacedName{Namespace: ns, Name: dv.Spec.SourceRef.Name}
	if err := client.Get(context.TODO(), nn, dataSource); err != nil {
		log.Error(err, "Unable to get DataSource", "namespacedName", nn)
		return dataVolumeNop
	}

	switch {
	case dataSource.Spec.Source.PVC != nil:
		return dataVolumePvcClone
	case dataSource.Spec.Source.Snapshot != nil:
		return dataVolumeSnapshotClone
	default:
		return dataVolumeNop
	}
}

type dvController interface {
	reconcile.Reconciler
	sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error)
	updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error
}

func (r *ReconcilerBase) reconcile(ctx context.Context, req reconcile.Request, dvc dvController) (reconcile.Result, error) {
	log := r.log.WithValues("DataVolume", req.NamespacedName)
	syncRes, syncErr := dvc.sync(log, req)
	res, err := r.updateStatus(req, syncRes.phaseSync, dvc)
	if syncErr != nil {
		err = syncErr
	}
	if syncRes.result != nil {
		res = *syncRes.result
	}
	return res, err
}

type dvSyncStateFunc func(*dvSyncState) error

func (r *ReconcilerBase) syncCommon(log logr.Logger, req reconcile.Request, cleanup, prepare dvSyncStateFunc) (dvSyncState, error) {
	syncState, err := r.syncDvPvcState(log, req, cleanup, prepare)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState, err
}

func (r *ReconcilerBase) syncDvPvcState(log logr.Logger, req reconcile.Request, cleanup, prepare dvSyncStateFunc) (dvSyncState, error) {
	syncState := dvSyncState{}
	dv, err := r.getDataVolume(req.NamespacedName)
	if dv == nil || err != nil {
		syncState.result = &reconcile.Result{}
		return syncState, err
	}
	syncState.dv = dv
	syncState.dvMutated = dv.DeepCopy()
	syncState.pvc, err = r.getPVC(req.NamespacedName)
	if err != nil {
		return syncState, err
	}

	if dv.DeletionTimestamp != nil {
		log.Info("DataVolume marked for deletion, cleaning up")
		if cleanup != nil {
			if err := cleanup(&syncState); err != nil {
				return syncState, err
			}
		}
		syncState.result = &reconcile.Result{}
		return syncState, nil
	}

	if prepare != nil {
		if err := prepare(&syncState); err != nil {
			return syncState, err
		}
	}

	syncState.pvcSpec, err = renderPvcSpec(r.client, r.recorder, log, dv, syncState.pvc)
	if err != nil {
		if syncErr := r.syncDataVolumeStatusPhaseWithEvent(&syncState, cdiv1.PhaseUnset, nil,
			Event{corev1.EventTypeWarning, cc.ErrClaimNotValid, err.Error()}); syncErr != nil {
			log.Error(syncErr, "failed to sync DataVolume status with event")
		}
		return syncState, err
	}

	if err := r.handleStaticVolume(&syncState, log); err != nil || syncState.result != nil {
		return syncState, err
	}

	if syncState.pvc != nil {
		if err := r.updatePvcMeta(&syncState); err != nil {
			return syncState, err
		}
		if err := r.garbageCollect(&syncState, log); err != nil {
			return syncState, err
		}
		if syncState.result != nil || syncState.dv == nil {
			return syncState, nil
		}
		if err := r.validatePVC(dv, syncState.pvc); err != nil {
			return syncState, err
		}
		r.handlePrePopulation(syncState.dvMutated, syncState.pvc)
	}

	return syncState, nil
}

func (r *ReconcilerBase) syncUpdate(log logr.Logger, syncState *dvSyncState) error {
	if syncState.dv == nil || syncState.dvMutated == nil {
		return nil
	}
	if !reflect.DeepEqual(syncState.dv.Status, syncState.dvMutated.Status) {
		return fmt.Errorf("status update is not allowed in sync phase")
	}
	if !reflect.DeepEqual(syncState.dv.ObjectMeta, syncState.dvMutated.ObjectMeta) {
		if err := r.updateDataVolume(syncState.dvMutated); err != nil {
			r.log.Error(err, "Unable to sync update dv meta", "name", syncState.dvMutated.Name)
			return err
		}
	}
	return nil
}

func (r *ReconcilerBase) handleStaticVolume(syncState *dvSyncState, log logr.Logger) error {
	if _, ok := syncState.dvMutated.Annotations[cc.AnnCheckStaticVolume]; !ok {
		return nil
	}

	if syncState.pvc == nil {
		volumes, err := r.getAvailableVolumesForDV(syncState, log)
		if err != nil {
			return err
		}

		if len(volumes) == 0 {
			log.Info("No PVs for DV")
			return nil
		}

		if err := r.handlePvcCreation(log, syncState, func(_ *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
			bs, err := json.Marshal(volumes)
			if err != nil {
				return err
			}
			cc.AddAnnotation(pvc, cc.AnnPersistentVolumeList, string(bs))
			return nil
		}); err != nil {
			return err
		}

		// set result to make sure callers don't do anything else in sync
		syncState.result = &reconcile.Result{}
		return nil
	}

	volumeAnno, ok := syncState.pvc.Annotations[cc.AnnPersistentVolumeList]
	if !ok {
		// etiher did not create the PVC here OR bind to expected PV succeeded
		return nil
	}

	if cc.IsUnbound(syncState.pvc) {
		// set result to make sure callers don't do anything else in sync
		syncState.result = &reconcile.Result{}
		return nil
	}

	var volumes []string
	if err := json.Unmarshal([]byte(volumeAnno), &volumes); err != nil {
		return err
	}

	for _, v := range volumes {
		if v == syncState.pvc.Spec.VolumeName {
			pvcCpy := syncState.pvc.DeepCopy()
			// handle as "populatedFor" going foreward
			cc.AddAnnotation(pvcCpy, cc.AnnPopulatedFor, syncState.dvMutated.Name)
			delete(pvcCpy.Annotations, cc.AnnPersistentVolumeList)
			if err := r.updatePVC(pvcCpy); err != nil {
				return err
			}
			syncState.pvc = pvcCpy
			return nil
		}
	}

	// delete the pvc and hope for better luck...
	pvcCpy := syncState.pvc.DeepCopy()
	if err := r.client.Delete(context.TODO(), pvcCpy, &client.DeleteOptions{}); err != nil {
		return err
	}

	syncState.pvc = pvcCpy

	return fmt.Errorf("DataVolume bound to unexpected PV %s", syncState.pvc.Spec.VolumeName)
}

func (r *ReconcilerBase) getAvailableVolumesForDV(syncState *dvSyncState, log logr.Logger) ([]string, error) {
	pvList := &corev1.PersistentVolumeList{}
	fields := client.MatchingFields{claimRefField: claimRefIndexKeyFunc(syncState.dv.Namespace, syncState.dv.Name)}
	if err := r.client.List(context.TODO(), pvList, fields); err != nil {
		return nil, err
	}
	if syncState.pvcSpec == nil {
		return nil, fmt.Errorf("missing pvc spec")
	}
	var pvNames []string
	for _, pv := range pvList.Items {
		if pv.Status.Phase == corev1.VolumeAvailable {
			pvc := &corev1.PersistentVolumeClaim{
				Spec: *syncState.pvcSpec,
			}
			if err := checkVolumeSatisfyClaim(&pv, pvc); err != nil {
				continue
			}
			log.Info("Found matching volume for DV", "pv", pv.Name)
			pvNames = append(pvNames, pv.Name)
		}
	}
	return pvNames, nil
}

func (r *ReconcilerBase) handlePrePopulation(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) {
	if pvc.Status.Phase == corev1.ClaimBound && pvcIsPopulated(pvc, dv) {
		cc.AddAnnotation(dv, cc.AnnPrePopulated, pvc.Name)
	}
}

func (r *ReconcilerBase) validatePVC(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	// If the PVC is being deleted, we should log a warning to the event recorder and return to wait the deletion complete
	// don't bother with owner refs is the pvc is deleted
	if pvc.DeletionTimestamp != nil {
		msg := fmt.Sprintf(MessageResourceMarkedForDeletion, pvc.Name)
		r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceMarkedForDeletion, msg)
		return errors.Errorf(msg)
	}
	// If the PVC is not controlled by this DataVolume resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(pvc, dv) {
		if pvcIsPopulated(pvc, dv) {
			if err := r.addOwnerRef(pvc, dv); err != nil {
				return err
			}
		} else {
			msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
			r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceExists, msg)
			return errors.Errorf(msg)
		}
	}
	return nil
}

func (r *ReconcilerBase) getPVC(key types.NamespacedName) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), key, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pvc, nil
}

func (r *ReconcilerBase) getDataVolume(key types.NamespacedName) (*cdiv1.DataVolume, error) {
	dv := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), key, dv); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return dv, nil
}

type pvcModifierFunc func(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error

func (r *ReconcilerBase) createPvcForDatavolume(datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec,
	pvcModifier pvcModifierFunc) (*corev1.PersistentVolumeClaim, error) {
	newPvc, err := r.newPersistentVolumeClaim(datavolume, pvcSpec, datavolume.Namespace, datavolume.Name, pvcModifier)
	if err != nil {
		return nil, err
	}
	util.SetRecommendedLabels(newPvc, r.installerLabels, "cdi-controller")
	if err := r.client.Create(context.TODO(), newPvc); err != nil {
		return nil, err
	}
	return newPvc, nil
}

func (r *ReconcilerBase) getStorageClassBindingMode(storageClassName *string) (*storagev1.VolumeBindingMode, error) {
	// Handle unspecified storage class name, fallback to default storage class
	storageClass, err := cc.GetStorageClassByName(context.TODO(), r.client, storageClassName)
	if err != nil {
		return nil, err
	}

	if storageClass != nil && storageClass.VolumeBindingMode != nil {
		return storageClass.VolumeBindingMode, nil
	}

	// no storage class, then the assumption is immediate binding
	volumeBindingImmediate := storagev1.VolumeBindingImmediate
	return &volumeBindingImmediate, nil
}

func (r *ReconcilerBase) reconcileProgressUpdate(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, result *reconcile.Result) error {
	var podNamespace string
	if datavolume.Status.Progress == "" {
		datavolume.Status.Progress = "N/A"
	}

	if datavolume.Spec.Source != nil && datavolume.Spec.Source.PVC != nil {
		podNamespace = datavolume.Spec.Source.PVC.Namespace
	} else {
		podNamespace = datavolume.Namespace
	}

	if datavolume.Status.Phase == cdiv1.Succeeded || datavolume.Status.Phase == cdiv1.Failed {
		// Data volume completed progress, or failed, either way stop queueing the data volume.
		r.log.Info("Datavolume finished, no longer updating progress", "Namespace", datavolume.Namespace, "Name", datavolume.Name, "Phase", datavolume.Status.Phase)
		return nil
	}
	pod, err := r.getPodFromPvc(podNamespace, pvc)
	if err == nil {
		if pod.Status.Phase != corev1.PodRunning {
			// Avoid long timeouts and error traces from HTTP get when pod is already gone
			return nil
		}
		if err := updateProgressUsingPod(datavolume, pod); err != nil {
			return err
		}
	}
	// We are not done yet, force a re-reconcile in 2 seconds to get an update.
	result.RequeueAfter = 2 * time.Second
	return nil
}

func (r *ReconcilerBase) syncDataVolumeStatusPhaseWithEvent(syncState *dvSyncState, phase cdiv1.DataVolumePhase, pvc *corev1.PersistentVolumeClaim, event Event) error {
	if syncState.phaseSync != nil {
		return fmt.Errorf("phaseSync is already set")
	}
	syncState.phaseSync = &statusPhaseSync{phase: phase, event: event}
	if pvc != nil {
		key := client.ObjectKeyFromObject(pvc)
		syncState.phaseSync.pvcKey = &key
	}
	return nil
}

func (r *ReconcilerBase) updateDataVolumeStatusPhaseSync(ps *statusPhaseSync, dv *cdiv1.DataVolume, dvCopy *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	var condPvc *corev1.PersistentVolumeClaim
	var err error
	if ps.pvcKey != nil {
		if pvc == nil || *ps.pvcKey != client.ObjectKeyFromObject(pvc) {
			condPvc, err = r.getPVC(*ps.pvcKey)
			if err != nil {
				return err
			}
		} else {
			condPvc = pvc
		}
	}
	return r.updateDataVolumeStatusPhaseWithEvent(ps.phase, dv, dvCopy, condPvc, ps.event)
}

func (r *ReconcilerBase) updateDataVolumeStatusPhaseWithEvent(
	phase cdiv1.DataVolumePhase,
	dataVolume *cdiv1.DataVolume,
	dataVolumeCopy *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	event Event) error {

	if dataVolume == nil {
		return nil
	}

	curPhase := dataVolumeCopy.Status.Phase
	dataVolumeCopy.Status.Phase = phase

	reason := ""
	message := ""
	if pvc == nil {
		reason = event.reason
		message = event.message
	}
	r.updateConditions(dataVolumeCopy, pvc, reason, message)
	return r.emitEvent(dataVolume, dataVolumeCopy, curPhase, dataVolume.Status.Conditions, &event)
}

func (r ReconcilerBase) updateStatus(req reconcile.Request, phaseSync *statusPhaseSync, dvc dvController) (reconcile.Result, error) {
	result := reconcile.Result{}
	dv, err := r.getDataVolume(req.NamespacedName)
	if dv == nil || err != nil {
		return reconcile.Result{}, err
	}

	dataVolumeCopy := dv.DeepCopy()

	pvc, err := r.getPVC(req.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if phaseSync != nil {
		err = r.updateDataVolumeStatusPhaseSync(phaseSync, dv, dataVolumeCopy, pvc)
		return reconcile.Result{}, err
	}

	curPhase := dataVolumeCopy.Status.Phase
	var event Event

	if shouldSetDataVolumePending(pvc, dataVolumeCopy) {
		dataVolumeCopy.Status.Phase = cdiv1.Pending
	} else if pvc != nil {
		dataVolumeCopy.Status.ClaimName = pvc.Name

		phase := pvc.Annotations[cc.AnnPodPhase]
		if phase == string(cdiv1.Succeeded) {
			if err := dvc.updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
				return reconcile.Result{}, err
			}
		} else {
			switch pvc.Status.Phase {
			case corev1.ClaimPending:
				shouldBeMarkedWaitForFirstConsumer, err := r.shouldBeMarkedWaitForFirstConsumer(pvc)
				if err != nil {
					return reconcile.Result{}, err
				}
				if shouldBeMarkedWaitForFirstConsumer {
					dataVolumeCopy.Status.Phase = cdiv1.WaitForFirstConsumer
				} else {
					dataVolumeCopy.Status.Phase = cdiv1.Pending
				}
			case corev1.ClaimBound:
				switch dataVolumeCopy.Status.Phase {
				case cdiv1.Pending:
					dataVolumeCopy.Status.Phase = cdiv1.PVCBound
				case cdiv1.WaitForFirstConsumer:
					dataVolumeCopy.Status.Phase = cdiv1.PVCBound
				case cdiv1.Unknown:
					dataVolumeCopy.Status.Phase = cdiv1.PVCBound
				}

				if pvcIsPopulated(pvc, dataVolumeCopy) {
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				} else {
					if err := dvc.updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
						return reconcile.Result{}, err
					}
				}

			case corev1.ClaimLost:
				dataVolumeCopy.Status.Phase = cdiv1.Failed
				event.eventType = corev1.EventTypeWarning
				event.reason = ErrClaimLost
				event.message = fmt.Sprintf(MessageErrClaimLost, pvc.Name)
			default:
				if pvc.Status.Phase != "" {
					dataVolumeCopy.Status.Phase = cdiv1.Unknown
				}
			}
		}
		if i, err := strconv.Atoi(pvc.Annotations[cc.AnnPodRestarts]); err == nil && i >= 0 {
			dataVolumeCopy.Status.RestartCount = int32(i)
		}
		if err := r.reconcileProgressUpdate(dataVolumeCopy, pvc, &result); err != nil {
			return result, err
		}
	}

	currentCond := make([]cdiv1.DataVolumeCondition, len(dataVolumeCopy.Status.Conditions))
	copy(currentCond, dataVolumeCopy.Status.Conditions)
	r.updateConditions(dataVolumeCopy, pvc, "", "")
	return result, r.emitEvent(dv, dataVolumeCopy, curPhase, currentCond, &event)
}

func (r *ReconcilerBase) updateConditions(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, reason, message string) {
	var anno map[string]string

	if dataVolume.Status.Conditions == nil {
		dataVolume.Status.Conditions = make([]cdiv1.DataVolumeCondition, 0)
	}

	if pvc != nil {
		anno = pvc.Annotations
	} else {
		anno = make(map[string]string)
	}

	var readyStatus corev1.ConditionStatus
	switch dataVolume.Status.Phase {
	case cdiv1.Succeeded:
		readyStatus = corev1.ConditionTrue
	case cdiv1.Unknown:
		readyStatus = corev1.ConditionUnknown
	default:
		readyStatus = corev1.ConditionFalse
	}

	dataVolume.Status.Conditions = updateBoundCondition(dataVolume.Status.Conditions, pvc, message, reason)
	dataVolume.Status.Conditions = UpdateReadyCondition(dataVolume.Status.Conditions, readyStatus, message, reason)
	dataVolume.Status.Conditions = updateRunningCondition(dataVolume.Status.Conditions, anno)
}

func (r *ReconcilerBase) emitConditionEvent(dataVolume *cdiv1.DataVolume, originalCond []cdiv1.DataVolumeCondition) {
	r.emitBoundConditionEvent(dataVolume, FindConditionByType(cdiv1.DataVolumeBound, dataVolume.Status.Conditions), FindConditionByType(cdiv1.DataVolumeBound, originalCond))
	r.emitFailureConditionEvent(dataVolume, originalCond)
}

func (r *ReconcilerBase) emitBoundConditionEvent(dataVolume *cdiv1.DataVolume, current, original *cdiv1.DataVolumeCondition) {
	// We know reason and message won't be empty for bound.
	if current != nil && (original == nil || current.Status != original.Status || current.Reason != original.Reason || current.Message != original.Message) {
		r.recorder.Event(dataVolume, corev1.EventTypeNormal, current.Reason, current.Message)
	}
}

func (r *ReconcilerBase) emitFailureConditionEvent(dataVolume *cdiv1.DataVolume, originalCond []cdiv1.DataVolumeCondition) {
	curReady := FindConditionByType(cdiv1.DataVolumeReady, dataVolume.Status.Conditions)
	curBound := FindConditionByType(cdiv1.DataVolumeBound, dataVolume.Status.Conditions)
	curRunning := FindConditionByType(cdiv1.DataVolumeRunning, dataVolume.Status.Conditions)
	orgRunning := FindConditionByType(cdiv1.DataVolumeRunning, originalCond)

	if curReady == nil || curBound == nil || curRunning == nil {
		return
	}
	if curReady.Status == corev1.ConditionFalse && curRunning.Status == corev1.ConditionFalse && curBound.Status == corev1.ConditionTrue {
		//Bound, not ready, and not running
		if curRunning.Message != "" && orgRunning.Message != curRunning.Message {
			r.recorder.Event(dataVolume, corev1.EventTypeWarning, curRunning.Reason, curRunning.Message)
		}
	}
}

func (r *ReconcilerBase) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, curPhase cdiv1.DataVolumePhase, originalCond []cdiv1.DataVolumeCondition, event *Event) error {
	if !reflect.DeepEqual(dataVolume.ObjectMeta, dataVolumeCopy.ObjectMeta) {
		return fmt.Errorf("meta update is not allowed in updateStatus phase")
	}
	// Update status subresource only if changed
	if !reflect.DeepEqual(dataVolume.Status, dataVolumeCopy.Status) {
		if err := r.client.Status().Update(context.TODO(), dataVolumeCopy); err != nil {
			r.log.Error(err, "unable to update datavolume status", "name", dataVolumeCopy.Name)
			return err
		}
		// Emit the event only on status phase change
		if event.eventType != "" && curPhase != dataVolumeCopy.Status.Phase {
			r.recorder.Event(dataVolumeCopy, event.eventType, event.reason, event.message)
		}
		r.emitConditionEvent(dataVolumeCopy, originalCond)
	}
	return nil
}

// getPodFromPvc determines the pod associated with the pvc passed in.
func (r *ReconcilerBase) getPodFromPvc(namespace string, pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
	l, _ := labels.Parse(common.PrometheusLabelKey)
	pods := &corev1.PodList{}
	listOptions := client.ListOptions{
		LabelSelector: l,
	}
	if err := r.client.List(context.TODO(), pods, &listOptions); err != nil {
		return nil, err
	}

	pvcUID := pvc.GetUID()
	for _, pod := range pods.Items {
		if cc.ShouldIgnorePod(&pod, pvc) {
			continue
		}
		for _, or := range pod.OwnerReferences {
			if or.UID == pvcUID {
				return &pod, nil
			}
		}

		// TODO: check this
		val, exists := pod.Labels[cc.CloneUniqueID]
		if exists && val == string(pvcUID)+common.ClonerSourcePodNameSuffix {
			return &pod, nil
		}
	}
	return nil, errors.Errorf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvcUID), namespace)
}

func (r *ReconcilerBase) addOwnerRef(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) error {
	if err := controllerutil.SetControllerReference(dv, pvc, r.scheme); err != nil {
		return err
	}

	return r.updatePVC(pvc)
}

func updateProgressUsingPod(dataVolumeCopy *cdiv1.DataVolume, pod *corev1.Pod) error {
	httpClient = cc.BuildHTTPClient(httpClient)
	url, err := cc.GetMetricsURL(pod)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}

	// Example value: import_progress{ownerUID="b856691e-1038-11e9-a5ab-525500d15501"} 13.45
	var importRegExp = regexp.MustCompile("progress\\{ownerUID\\=\"" + string(dataVolumeCopy.UID) + "\"\\} (\\d{1,3}\\.?\\d*)")
	if progressReport, err := cc.GetProgressReportFromURL(url, importRegExp, httpClient); err != nil {
		return err
	} else if progressReport != "" {
		if f, err := strconv.ParseFloat(progressReport, 64); err == nil {
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress(fmt.Sprintf("%.2f%%", f))
		}
	}
	return nil
}

// newPersistentVolumeClaim creates a new PVC for the DataVolume resource.
// It also sets the appropriate OwnerReferences on the resource
// which allows handleObject to discover the DataVolume resource
// that 'owns' it.
func (r *ReconcilerBase) newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume, targetPvcSpec *corev1.PersistentVolumeClaimSpec, namespace, name string, pvcModifier pvcModifierFunc) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		common.CDILabelKey: common.CDILabelValue,
	}
	if util.ResolveVolumeMode(targetPvcSpec.VolumeMode) == corev1.PersistentVolumeFilesystem {
		labels[common.KubePersistentVolumeFillingUpSuppressLabelKey] = common.KubePersistentVolumeFillingUpSuppressLabelValue
	}
	for k, v := range dataVolume.Labels {
		labels[k] = v
	}

	annotations := make(map[string]string)
	for k, v := range dataVolume.ObjectMeta.Annotations {
		annotations[k] = v
	}
	annotations[cc.AnnPodRestarts] = "0"
	annotations[cc.AnnContentType] = cc.GetContentType(string(dataVolume.Spec.ContentType))
	if dataVolume.Spec.PriorityClassName != "" {
		annotations[cc.AnnPriorityClassName] = dataVolume.Spec.PriorityClassName
	}
	annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(context.TODO(), r.client, dataVolume.Spec.Preallocation))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: *targetPvcSpec,
	}

	if pvcModifier != nil {
		if err := pvcModifier(dataVolume, pvc); err != nil {
			return nil, err
		}
	}

	if pvc.Namespace == dataVolume.Namespace {
		pvc.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(dataVolume, schema.GroupVersionKind{
				Group:   cdiv1.SchemeGroupVersion.Group,
				Version: cdiv1.SchemeGroupVersion.Version,
				Kind:    "DataVolume",
			}),
		}
	} else {
		if err := setAnnOwnedByDataVolume(pvc, dataVolume); err != nil {
			return nil, err
		}
		pvc.Annotations[cc.AnnOwnerUID] = string(dataVolume.UID)
	}

	return pvc, nil
}

func (r *ReconcilerBase) updatePvcMeta(syncState *dvSyncState) error {
	dv := syncState.dvMutated

	// Update the PVC meta only if DV garbage collection is disabled
	if dv.Annotations[cc.AnnDeleteAfterCompletion] == "true" {
		return nil
	}

	pvcCopy := syncState.pvc.DeepCopy()
	for k, v := range dv.Labels {
		cc.AddLabel(pvcCopy, k, v)
	}
	for k, v := range dv.Annotations {
		cc.AddAnnotation(pvcCopy, k, v)
	}

	if pvcCopy.Namespace == dv.Namespace {
		if pvcCopy.OwnerReferences == nil {
			pvcCopy.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(dv, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataVolume",
				}),
			}
		}
	} else {
		if err := setAnnOwnedByDataVolume(pvcCopy, dv); err != nil {
			return err
		}
		cc.AddAnnotation(pvcCopy, cc.AnnOwnerUID, string(dv.UID))
	}

	if !reflect.DeepEqual(syncState.pvc, pvcCopy) {
		if err := r.updatePVC(pvcCopy); err != nil {
			return err
		}
		syncState.pvc = pvcCopy
	}

	return nil
}

// Whenever the controller updates a DV, we must make sure to nil out spec.source when using other population methods
func (r *ReconcilerBase) updateDataVolume(dv *cdiv1.DataVolume) error {
	// Restore so we don't nil out the dv that is being worked on
	var sourceCopy *cdiv1.DataVolumeSource

	if dv.Spec.SourceRef != nil || dvUsesVolumePopulator(dv) {
		sourceCopy = dv.Spec.Source
		dv.Spec.Source = nil
	}

	err := r.client.Update(context.TODO(), dv)
	if dv.Spec.SourceRef != nil || dvUsesVolumePopulator(dv) {
		dv.Spec.Source = sourceCopy
	}
	return err
}

func (r *ReconcilerBase) updatePVC(pvc *corev1.PersistentVolumeClaim) error {
	return r.client.Update(context.TODO(), pvc)
}

func newLongTermCloneTokenGenerator(key *rsa.PrivateKey) token.Generator {
	return token.NewGenerator(common.ExtendedCloneTokenIssuer, key, 10*365*24*time.Hour)
}

// shouldBeMarkedWaitForFirstConsumer decided whether we should mark DV as WFFC
func (r *ReconcilerBase) shouldBeMarkedWaitForFirstConsumer(pvc *corev1.PersistentVolumeClaim) (bool, error) {
	storageClassBindingMode, err := r.getStorageClassBindingMode(pvc.Spec.StorageClassName)
	if err != nil {
		return false, err
	}

	honorWaitForFirstConsumerEnabled, err := r.featureGates.HonorWaitForFirstConsumerEnabled()
	if err != nil {
		return false, err
	}

	res := honorWaitForFirstConsumerEnabled &&
		storageClassBindingMode != nil && *storageClassBindingMode == storagev1.VolumeBindingWaitForFirstConsumer &&
		pvc.Status.Phase == corev1.ClaimPending

	return res, nil
}

// handlePvcCreation works as a wrapper for non-clone PVC creation and error handling
func (r *ReconcilerBase) handlePvcCreation(log logr.Logger, syncState *dvSyncState, pvcModifier pvcModifierFunc) error {
	if syncState.pvc != nil {
		return nil
	}
	if dvIsPrePopulated(syncState.dvMutated) {
		return nil
	}
	// Creating the PVC
	newPvc, err := r.createPvcForDatavolume(syncState.dvMutated, syncState.pvcSpec, pvcModifier)
	if err != nil {
		if cc.ErrQuotaExceeded(err) {
			syncErr := r.syncDataVolumeStatusPhaseWithEvent(syncState, cdiv1.Pending, nil, Event{corev1.EventTypeWarning, cc.ErrExceededQuota, err.Error()})
			if syncErr != nil {
				log.Error(syncErr, "failed to sync DataVolume status with event")
			}
		}
		return err
	}
	syncState.pvc = newPvc

	return nil
}

// storageClassCSIDriverExists returns true if the passed storage class has CSI drivers available
func (r *ReconcilerBase) storageClassCSIDriverExists(storageClassName *string) (bool, error) {
	log := r.log.WithName("getCsiDriverForStorageClass").V(3)

	storageClass, err := cc.GetStorageClassByName(context.TODO(), r.client, storageClassName)
	if err != nil {
		return false, err
	}
	if storageClass == nil {
		log.Info("Target PVC's Storage Class not found")
		return false, nil
	}

	csiDriver := &storagev1.CSIDriver{}

	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: storageClass.Provisioner}, csiDriver); err != nil {
		return false, err
	}

	return true, nil
}
