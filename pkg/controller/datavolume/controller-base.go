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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

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
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	cloneMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	importMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
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

var (
	httpClient *http.Client

	delayedAnnotations = []string{
		cc.AnnPopulatedFor,
	}
)

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
	snapshot  *snapshotv1.VolumeSnapshot
	dvSyncResult
	usePopulator bool
}

// ReconcilerBase members
type ReconcilerBase struct {
	client               client.Client
	recorder             record.EventRecorder
	scheme               *runtime.Scheme
	log                  logr.Logger
	featureGates         featuregates.FeatureGates
	installerLabels      map[string]string
	shouldUpdateProgress bool
}

func pvcIsPopulatedForDataVolume(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
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

// dataVolumeOp is the datavolume's requested operation
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
			obj:          &corev1.PersistentVolume{},
			field:        claimStorageClassNameField,
			extractValue: extractAvailablePersistentVolumeStorageClassName,
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

// CreateAvailablePersistentVolumeIndex adds storage class name index for available PersistentVolumes
func CreateAvailablePersistentVolumeIndex(fieldIndexer client.FieldIndexer) error {
	return fieldIndexer.IndexField(context.TODO(), &corev1.PersistentVolume{},
		claimStorageClassNameField, extractAvailablePersistentVolumeStorageClassName)
}

func extractAvailablePersistentVolumeStorageClassName(obj client.Object) []string {
	if pv, ok := obj.(*corev1.PersistentVolume); ok && pv.Status.Phase == corev1.VolumeAvailable {
		return []string{pv.Spec.StorageClassName}
	}
	return nil
}

func addDataVolumeControllerCommonWatches(mgr manager.Manager, dataVolumeController controller.Controller, op dataVolumeOp) error {
	appendMatchingDataVolumeRequest := func(ctx context.Context, reqs []reconcile.Request, mgr manager.Manager, namespace, name string) []reconcile.Request {
		dvKey := types.NamespacedName{Namespace: namespace, Name: name}
		dv := &cdiv1.DataVolume{}
		if err := mgr.GetClient().Get(ctx, dvKey, dv); err != nil {
			if !k8serrors.IsNotFound(err) {
				mgr.GetLogger().Error(err, "Failed to get DV", "dvKey", dvKey)
			}
			return reqs
		}
		if getDataVolumeOp(ctx, mgr.GetLogger(), dv, mgr.GetClient()) == op {
			reqs = append(reqs, reconcile.Request{NamespacedName: dvKey})
		}
		return reqs
	}

	// Setup watches
	if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataVolume{}, handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.DataVolume](
		func(ctx context.Context, dv *cdiv1.DataVolume) []reconcile.Request {
			if getDataVolumeOp(ctx, mgr.GetLogger(), dv, mgr.GetClient()) != op {
				return nil
			}
			updatePendingDataVolumesGauge(ctx, mgr.GetLogger(), dv, mgr.GetClient())
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}}}
		}),
	)); err != nil {
		return err
	}
	if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}, handler.TypedEnqueueRequestsFromMapFunc[*corev1.PersistentVolumeClaim](
		func(ctx context.Context, obj *corev1.PersistentVolumeClaim) []reconcile.Request {
			var result []reconcile.Request
			owner := metav1.GetControllerOf(obj)
			if owner != nil && owner.Kind == "DataVolume" {
				result = appendMatchingDataVolumeRequest(ctx, result, mgr, obj.GetNamespace(), owner.Name)
			}
			populatedFor := obj.GetAnnotations()[cc.AnnPopulatedFor]
			if populatedFor != "" {
				result = appendMatchingDataVolumeRequest(ctx, result, mgr, obj.GetNamespace(), populatedFor)
			}
			// it is okay if result contains the same entry twice, will be deduplicated by caller
			return result
		}),
	)); err != nil {
		return err
	}
	if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, handler.TypedEnqueueRequestsFromMapFunc[*corev1.Pod](
		func(ctx context.Context, obj *corev1.Pod) []reconcile.Request {
			owner := metav1.GetControllerOf(obj)
			if owner == nil || owner.Kind != "DataVolume" {
				return nil
			}
			return appendMatchingDataVolumeRequest(ctx, nil, mgr, obj.GetNamespace(), owner.Name)
		}),
	)); err != nil {
		return err
	}
	for _, k := range []client.Object{&corev1.PersistentVolumeClaim{}, &corev1.Pod{}, &cdiv1.ObjectTransfer{}} {
		if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), k, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				if !hasAnnOwnedByDataVolume(obj) {
					return nil
				}
				namespace, name, err := getAnnOwnedByDataVolume(obj)
				if err != nil {
					return nil
				}
				return appendMatchingDataVolumeRequest(ctx, nil, mgr, namespace, name)
			}),
		)); err != nil {
			return err
		}
	}

	// Watch for StorageClass and StorageProfile updates and reconcile the DVs waiting for StorageClass or its complete StorageProfile.
	// Relevant only when the DV StorageSpec has no AccessModes set and no matching StorageClass with compelete StorageProfile yet,
	// so PVC cannot be created (test_id:9922).
	for _, k := range []client.Object{&storagev1.StorageClass{}, &cdiv1.StorageProfile{}} {
		if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), k, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				dvList := &cdiv1.DataVolumeList{}
				if err := mgr.GetClient().List(ctx, dvList, client.MatchingFields{dvPhaseField: ""}); err != nil {
					return nil
				}
				var reqs []reconcile.Request
				for _, dv := range dvList.Items {
					if getDataVolumeOp(ctx, mgr.GetLogger(), &dv, mgr.GetClient()) == op {
						reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
					}
				}
				return reqs
			},
		),
		)); err != nil {
			return err
		}
	}

	// Watch for PV updates to reconcile the DVs waiting for available PV
	// Relevant only when the DV StorageSpec has no AccessModes set and no matching StorageClass yet, so PVC cannot be created (test_id:9924,9925)
	if err := dataVolumeController.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolume{}, handler.TypedEnqueueRequestsFromMapFunc[*corev1.PersistentVolume](
		func(ctx context.Context, pv *corev1.PersistentVolume) []reconcile.Request {
			dvList := &cdiv1.DataVolumeList{}
			if err := mgr.GetClient().List(ctx, dvList, client.MatchingFields{dvPhaseField: ""}); err != nil {
				return nil
			}
			var reqs []reconcile.Request
			for _, dv := range dvList.Items {
				storage := dv.Spec.Storage
				if storage != nil &&
					storage.StorageClassName != nil &&
					*storage.StorageClassName == pv.Spec.StorageClassName &&
					pv.Status.Phase == corev1.VolumeAvailable &&
					getDataVolumeOp(ctx, mgr.GetLogger(), &dv, mgr.GetClient()) == op {
					reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
				}
			}
			return reqs
		},
	),
	)); err != nil {
		return err
	}

	return nil
}

func getDataVolumeOp(ctx context.Context, log logr.Logger, dv *cdiv1.DataVolume, client client.Client) dataVolumeOp {
	src := dv.Spec.Source

	if dv.Spec.SourceRef != nil {
		return getSourceRefOp(ctx, log, dv, client)
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

func getSourceRefOp(ctx context.Context, log logr.Logger, dv *cdiv1.DataVolume, client client.Client) dataVolumeOp {
	dataSource := &cdiv1.DataSource{}
	ns := dv.Namespace
	if dv.Spec.SourceRef.Namespace != nil && *dv.Spec.SourceRef.Namespace != "" {
		ns = *dv.Spec.SourceRef.Namespace
	}
	nn := types.NamespacedName{Namespace: ns, Name: dv.Spec.SourceRef.Name}
	if err := client.Get(ctx, nn, dataSource); err != nil {
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

func updatePendingDataVolumesGauge(ctx context.Context, log logr.Logger, dv *cdiv1.DataVolume, c client.Client) {
	if !cc.IsDataVolumeUsingDefaultStorageClass(dv) {
		return
	}

	countPending, err := getDefaultStorageClassDataVolumeCount(ctx, c, string(cdiv1.Pending))
	if err != nil {
		log.V(3).Error(err, "Failed listing the pending DataVolumes")
		return
	}
	countUnset, err := getDefaultStorageClassDataVolumeCount(ctx, c, string(cdiv1.PhaseUnset))
	if err != nil {
		log.V(3).Error(err, "Failed listing the unset DataVolumes")
		return
	}

	metrics.SetDataVolumePending(countPending + countUnset)
}

func getDefaultStorageClassDataVolumeCount(ctx context.Context, c client.Client, dvPhase string) (int, error) {
	dvList := &cdiv1.DataVolumeList{}
	if err := c.List(ctx, dvList, client.MatchingFields{dvPhaseField: dvPhase}); err != nil {
		return 0, err
	}

	dvCount := 0
	for _, dv := range dvList.Items {
		if cc.IsDataVolumeUsingDefaultStorageClass(&dv) {
			dvCount++
		}
	}

	return dvCount, nil
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

	if cleanup != nil {
		if err := cleanup(&syncState); err != nil {
			return syncState, err
		}
	}

	if dv.DeletionTimestamp != nil {
		log.Info("DataVolume marked for deletion")
		syncState.result = &reconcile.Result{}
		return syncState, nil
	}

	if prepare != nil {
		if err := prepare(&syncState); err != nil {
			return syncState, err
		}
	}

	syncState.pvcSpec, err = renderPvcSpec(r.client, r.recorder, log, syncState.dvMutated, syncState.pvc)
	if err != nil {
		if syncErr := r.syncDataVolumeStatusPhaseWithEvent(&syncState, cdiv1.PhaseUnset, nil,
			Event{corev1.EventTypeWarning, cc.ErrClaimNotValid, err.Error()}); syncErr != nil {
			log.Error(syncErr, "failed to sync DataVolume status with event")
		}
		if errors.Is(err, ErrStorageClassNotFound) {
			syncState.result = &reconcile.Result{}
			return syncState, nil
		}
		return syncState, err
	}

	syncState.usePopulator, err = r.shouldUseCDIPopulator(&syncState)
	if err != nil {
		return syncState, err
	}
	updateDataVolumeUseCDIPopulator(&syncState)

	if err := r.handleStaticVolume(&syncState, log); err != nil || syncState.result != nil {
		return syncState, err
	}

	if err := r.handleDelayedAnnotations(&syncState, log); err != nil || syncState.result != nil {
		return syncState, err
	}

	if err = updateDataVolumeDefaultInstancetypeLabels(r.client, &syncState); err != nil {
		return syncState, err
	}

	if syncState.pvc != nil {
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
		_, ok := syncState.dv.Annotations[cc.AnnExtendedCloneToken]
		_, ok2 := syncState.dvMutated.Annotations[cc.AnnExtendedCloneToken]
		if err := r.updateDataVolume(syncState.dvMutated); err != nil {
			r.log.Error(err, "Unable to sync update dv meta", "name", syncState.dvMutated.Name)
			return err
		}
		if !ok && ok2 {
			delta := time.Since(syncState.dv.ObjectMeta.CreationTimestamp.Time)
			log.V(3).Info("Adding extended DataVolume token took", "delta", delta)
		}
		syncState.dv = syncState.dvMutated.DeepCopy()
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
			// handle as "populatedFor" going forward
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

func (r *ReconcilerBase) handleDelayedAnnotations(syncState *dvSyncState, log logr.Logger) error {
	dataVolume := syncState.dv
	if dataVolume.Status.Phase != cdiv1.Succeeded {
		return nil
	}

	if syncState.pvc == nil {
		return nil
	}

	pvcCpy := syncState.pvc.DeepCopy()
	for _, anno := range delayedAnnotations {
		if val, ok := dataVolume.Annotations[anno]; ok {
			// only add if not already present
			if _, ok := pvcCpy.Annotations[anno]; !ok {
				cc.AddAnnotation(pvcCpy, anno, val)
			}
		}
	}

	if !reflect.DeepEqual(syncState.pvc, pvcCpy) {
		if err := r.updatePVC(pvcCpy); err != nil {
			return err
		}
		syncState.pvc = pvcCpy
		syncState.result = &reconcile.Result{}
	}

	return nil
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
			if err := CheckVolumeSatisfyClaim(&pv, pvc); err != nil {
				continue
			}
			log.Info("Found matching volume for DV", "pv", pv.Name)
			pvNames = append(pvNames, pv.Name)
		}
	}
	return pvNames, nil
}

func (r *ReconcilerBase) handlePrePopulation(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) {
	if pvc.Status.Phase == corev1.ClaimBound && pvcIsPopulatedForDataVolume(pvc, dv) {
		cc.AddAnnotation(dv, cc.AnnPrePopulated, pvc.Name)
	}
}

func (r *ReconcilerBase) validatePVC(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	// If the PVC is being deleted, we should log a warning to the event recorder and return to wait the deletion complete
	// don't bother with owner refs is the pvc is deleted
	if pvc.DeletionTimestamp != nil {
		msg := fmt.Sprintf(MessageResourceMarkedForDeletion, pvc.Name)
		r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceMarkedForDeletion, msg)
		return errors.New(msg)
	}
	// If the PVC is not controlled by this DataVolume resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(pvc, dv) {
		requiresWork, err := r.pvcRequiresWork(pvc, dv)
		if err != nil {
			return err
		}
		if !requiresWork {
			if err := r.addOwnerRef(pvc, dv); err != nil {
				return err
			}
		} else {
			msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
			r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceExists, msg)
			return errors.New(msg)
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
	storageClass, err := cc.GetStorageClassByNameWithK8sFallback(context.TODO(), r.client, storageClassName)
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

	if !r.shouldUpdateProgress {
		return nil
	}

	if usePopulator, _ := CheckPVCUsingPopulators(pvc); usePopulator {
		if progress, ok := pvc.Annotations[cc.AnnPopulatorProgress]; ok {
			datavolume.Status.Progress = cdiv1.DataVolumeProgress(progress)
		} else {
			datavolume.Status.Progress = "N/A"
		}
		return nil
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
	pod, err := cc.GetPodFromPvc(r.client, podNamespace, pvc)
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
	return r.emitEvent(dataVolume, dataVolumeCopy, curPhase, dataVolume.Status.Conditions, pvc, &event)
}

func (r *ReconcilerBase) updateStatus(req reconcile.Request, phaseSync *statusPhaseSync, dvc dvController) (reconcile.Result, error) {
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
		requiresWork, err := r.pvcRequiresWork(pvc, dataVolumeCopy)
		if err != nil {
			return reconcile.Result{}, err
		}
		if phase == string(cdiv1.Succeeded) && requiresWork {
			if err := dvc.updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
				return reconcile.Result{}, err
			}
		} else {
			switch pvc.Status.Phase {
			case corev1.ClaimPending:
				if requiresWork {
					if err := r.updateStatusPVCPending(pvc, dvc, dataVolumeCopy, &event); err != nil {
						return reconcile.Result{}, err
					}
				} else {
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
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

				if requiresWork {
					if err := dvc.updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
						return reconcile.Result{}, err
					}
				} else {
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
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

		if i, err := strconv.ParseInt(pvc.Annotations[cc.AnnPodRestarts], 10, 32); err == nil && i >= 0 {
			dataVolumeCopy.Status.RestartCount = int32(i)
		}
		if err := r.reconcileProgressUpdate(dataVolumeCopy, pvc, &result); err != nil {
			return result, err
		}
	}

	currentCond := make([]cdiv1.DataVolumeCondition, len(dataVolumeCopy.Status.Conditions))
	copy(currentCond, dataVolumeCopy.Status.Conditions)
	r.updateConditions(dataVolumeCopy, pvc, "", "")
	return result, r.emitEvent(dv, dataVolumeCopy, curPhase, currentCond, pvc, &event)
}

func (r ReconcilerBase) updateStatusPVCPending(pvc *corev1.PersistentVolumeClaim, dvc dvController, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	usePopulator, err := CheckPVCUsingPopulators(pvc)
	if err != nil {
		return err
	}
	if usePopulator {
		// when using populators the target pvc phase will stay pending until the population completes,
		// hence if not wffc we should update the dv phase according to the pod phase
		shouldBeMarkedPendingPopulation, err := r.shouldBeMarkedPendingPopulation(pvc)
		if err != nil {
			return err
		}
		if shouldBeMarkedPendingPopulation {
			dataVolumeCopy.Status.Phase = cdiv1.PendingPopulation
		} else if err := dvc.updateStatusPhase(pvc, dataVolumeCopy, event); err != nil {
			return err
		}
		return nil
	}

	shouldBeMarkedWaitForFirstConsumer, err := r.shouldBeMarkedWaitForFirstConsumer(pvc)
	if err != nil {
		return err
	}
	if shouldBeMarkedWaitForFirstConsumer {
		dataVolumeCopy.Status.Phase = cdiv1.WaitForFirstConsumer
	} else {
		dataVolumeCopy.Status.Phase = cdiv1.Pending
	}
	return nil
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
	if pvc != nil {
		dataVolume.Status.Conditions = appendPVCEventToBoundCondition(dataVolume.Status.Conditions, pvc, r.getLatestPrimePVCEvent(pvc))
	}
}

func (r *ReconcilerBase) getLatestPrimePVCEvent(pvc *corev1.PersistentVolumeClaim) string {
	events := &corev1.EventList{}

	err := r.client.List(context.TODO(), events,
		client.InNamespace(pvc.GetNamespace()),
		client.MatchingFields{"involvedObject.name": pvc.GetName(),
			"involvedObject.uid": string(pvc.GetUID())},
	)
	// Sort event lists by most recent
	sort.Slice(events.Items, func(i, j int) bool {
		return events.Items[i].FirstTimestamp.Time.After(events.Items[j].FirstTimestamp.Time)
	})

	pvcPrime, exists := pvc.GetAnnotations()[cc.AnnAPIGroup+"/storage.populator.pvcPrime"]

	// only want to return events that have come from pvcPrime
	if err != nil || len(events.Items) == 0 || !exists {
		return ""
	}

	pvcPrime = fmt.Sprintf("[%s] :", pvcPrime)

	for _, event := range events.Items {
		if strings.Contains(event.Message, pvcPrime) {
			res := strings.Split(event.Message, pvcPrime)
			r.log.V(1).Info("DANNY", "event", res[len(res)-1])
			return res[len(res)-1]
		}
	}
	return ""
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
	if curReady.Status == corev1.ConditionFalse && curRunning.Status == corev1.ConditionFalse &&
		dvBoundOrPopulationInProgress(dataVolume, curBound) {
		// Bound or in progress, not ready, and not running.
		// Avoiding triggering an event for scratch space required since it will be addressed
		// by CDI and sounds more drastic than it actually is.
		if curRunning.Message != "" && curRunning.Message != common.ScratchSpaceRequired &&
			(orgRunning == nil || orgRunning.Message != curRunning.Message) {
			r.recorder.Event(dataVolume, corev1.EventTypeWarning, curRunning.Reason, curRunning.Message)
		}
	}
}

func (r *ReconcilerBase) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, curPhase cdiv1.DataVolumePhase, originalCond []cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim, event *Event) error {
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
		if pvc != nil {
			populators.CopyEvents(pvc, dataVolumeCopy, r.client, r.log, r.recorder)
		}

		r.emitConditionEvent(dataVolumeCopy, originalCond)
	}
	return nil
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

	// Used for both import and clone, so it should match both metric names
	progressReport, err := cc.GetProgressReportFromURL(context.TODO(), url, httpClient,
		fmt.Sprintf("%s|%s", importMetrics.ImportProgressMetricName, cloneMetrics.CloneProgressMetricName),
		string(dataVolumeCopy.UID))
	if err != nil {
		return err
	}
	if progressReport != "" {
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
	annotations[cc.AnnContentType] = string(cc.GetContentType(dataVolume.Spec.ContentType))
	if dataVolume.Spec.PriorityClassName != "" {
		annotations[cc.AnnPriorityClassName] = dataVolume.Spec.PriorityClassName
	}
	annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(context.TODO(), r.client, dataVolume.Spec.Preallocation))
	annotations[cc.AnnCreatedForDataVolume] = string(dataVolume.UID)

	if dataVolume.Spec.Storage != nil && labels[common.PvcApplyStorageProfileLabel] == "true" {
		isWebhookPvcRenderingEnabled, err := featuregates.IsWebhookPvcRenderingEnabled(r.client)
		if err != nil {
			return nil, err
		}
		if isWebhookPvcRenderingEnabled {
			labels[common.PvcApplyStorageProfileLabel] = "true"
			if targetPvcSpec.VolumeMode == nil {
				targetPvcSpec.VolumeMode = ptr.To[corev1.PersistentVolumeMode](cdiv1.PersistentVolumeFromStorageProfile)
			}
		}
	}

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

	for _, anno := range delayedAnnotations {
		delete(pvc.Annotations, anno)
	}

	return pvc, nil
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

// storageClassWaitForFirstConsumer returns if the binding mode of a given storage class is WFFC
func (r *ReconcilerBase) storageClassWaitForFirstConsumer(storageClass *string) (bool, error) {
	storageClassBindingMode, err := r.getStorageClassBindingMode(storageClass)
	if err != nil {
		return false, err
	}

	return storageClassBindingMode != nil && *storageClassBindingMode == storagev1.VolumeBindingWaitForFirstConsumer, nil
}

// shouldBeMarkedWaitForFirstConsumer decided whether we should mark DV as WFFC
func (r *ReconcilerBase) shouldBeMarkedWaitForFirstConsumer(pvc *corev1.PersistentVolumeClaim) (bool, error) {
	wffc, err := r.storageClassWaitForFirstConsumer(pvc.Spec.StorageClassName)
	if err != nil {
		return false, err
	}

	honorWaitForFirstConsumerEnabled, err := r.featureGates.HonorWaitForFirstConsumerEnabled()
	if err != nil {
		return false, err
	}

	res := honorWaitForFirstConsumerEnabled && wffc &&
		pvc.Status.Phase == corev1.ClaimPending

	return res, nil
}

func (r *ReconcilerBase) shouldReconcileVolumeSourceCR(syncState *dvSyncState) bool {
	if syncState.pvc == nil {
		return true
	}
	phase := syncState.pvc.Annotations[cc.AnnPodPhase]
	return phase != string(corev1.PodSucceeded) || syncState.dvMutated.Status.Phase != cdiv1.Succeeded
}

// shouldBeMarkedPendingPopulation decides whether we should mark DV as PendingPopulation
func (r *ReconcilerBase) shouldBeMarkedPendingPopulation(pvc *corev1.PersistentVolumeClaim) (bool, error) {
	wffc, err := r.storageClassWaitForFirstConsumer(pvc.Spec.StorageClassName)
	if err != nil {
		return false, err
	}
	nodeName := pvc.Annotations[cc.AnnSelectedNode]
	immediateBindingRequested := cc.ImmediateBindingRequested(pvc)

	return wffc && nodeName == "" && !immediateBindingRequested, nil
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

// shouldUseCDIPopulator returns if the population of the PVC should be done using
// CDI populators.
// Currently it will use populators only if:
// * storageClass used is CSI storageClass
// * annotation cdi.kubevirt.io/storage.usePopulator is not set by user to "false"
func (r *ReconcilerBase) shouldUseCDIPopulator(syncState *dvSyncState) (bool, error) {
	dv := syncState.dvMutated
	if usePopulator, ok := dv.Annotations[cc.AnnUsePopulator]; ok {
		boolUsePopulator, err := strconv.ParseBool(usePopulator)
		if err != nil {
			return false, err
		}
		return boolUsePopulator, nil
	}
	log := r.log.WithValues("DataVolume", dv.Name, "Namespace", dv.Namespace)
	usePopulator, err := storageClassCSIDriverExists(r.client, r.log, syncState.pvcSpec.StorageClassName)
	if err != nil {
		return false, err
	}
	if !usePopulator {
		if syncState.pvcSpec.StorageClassName != nil {
			log.Info("Not using CDI populators, storage class is not a CSI storage", "storageClass", *syncState.pvcSpec.StorageClassName)
		}
	}

	return usePopulator, nil
}

func (r *ReconcilerBase) pvcRequiresWork(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (bool, error) {
	if pvc == nil || dv == nil {
		return true, nil
	}
	if pvcIsPopulatedForDataVolume(pvc, dv) {
		return false, nil
	}
	canAdopt, err := cc.AllowClaimAdoption(r.client, pvc, dv)
	if err != nil {
		return true, err
	}
	if canAdopt {
		return false, nil
	}
	return true, nil
}
