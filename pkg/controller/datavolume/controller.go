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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
)

var httpClient *http.Client

// Event represents DV controller event
type Event struct {
	eventType string
	reason    string
	message   string
}

type dataVolumeSyncResult struct {
	dv      *cdiv1.DataVolume
	pvc     *corev1.PersistentVolumeClaim
	pvcSpec *v1.PersistentVolumeClaimSpec
	res     *reconcile.Result
}

// Reconciler interface
type Reconciler interface {
	updateAnnotations(dataVolume *cdiv1.DataVolume, annotations map[string]string) error
	updateStatus(log logr.Logger, syncRes dataVolumeSyncResult, syncErr error) (reconcile.Result, error)
}

// ReconcilerBase members
type ReconcilerBase struct {
	Reconciler
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

type dataVolumeOp int

const (
	dataVolumeNop dataVolumeOp = iota
	dataVolumeImport
	dataVolumeUpload
	dataVolumeClone
)

func addDataVolumeControllerCommonWatches(mgr manager.Manager, dataVolumeController controller.Controller, op dataVolumeOp) error {
	appendMatchingDataVolumeRequest := func(reqs []reconcile.Request, mgr manager.Manager, namespace, name string) []reconcile.Request {
		dvKey := types.NamespacedName{Namespace: namespace, Name: name}
		dv := &cdiv1.DataVolume{}
		if err := mgr.GetClient().Get(context.TODO(), dvKey, dv); err != nil {
			mgr.GetLogger().Error(err, "Failed to get DV", "dvKey", dvKey)
			return reqs
		}
		if getDataVolumeOp(dv) == op {
			reqs = append(reqs, reconcile.Request{NamespacedName: dvKey})
		}
		return reqs
	}

	// Setup watches
	if err := dataVolumeController.Watch(&source.Kind{Type: &cdiv1.DataVolume{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			dv := obj.(*cdiv1.DataVolume)
			if getDataVolumeOp(dv) != op {
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
				if getDataVolumeOp(&dv) == op {
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

func getDataVolumeOp(dv *cdiv1.DataVolume) dataVolumeOp {
	src := dv.Spec.Source
	if (src != nil && src.PVC != nil) || dv.Spec.SourceRef != nil {
		return dataVolumeClone
	}
	if src == nil {
		return dataVolumeNop
	}
	if src.Upload != nil {
		return dataVolumeUpload
	}
	if src.HTTP != nil || src.S3 != nil || src.Registry != nil || src.Blank != nil || src.Imageio != nil || src.VDDK != nil {
		return dataVolumeImport
	}
	return dataVolumeNop
}

// Reconcile loop for the data volumes
func (r ReconcilerBase) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("DataVolume", req.NamespacedName)
	res, syncErr := r.sync(log, req, nil, nil)
	return r.updateStatus(log, res, syncErr)
}

func (r ReconcilerBase) sync(log logr.Logger, req reconcile.Request, cleanup, prepare func(*cdiv1.DataVolume) error) (dataVolumeSyncResult, error) {
	var ctx dataVolumeSyncResult

	dv, err := r.getDataVolume(req.NamespacedName)
	if dv == nil {
		ctx.res = &reconcile.Result{}
		return ctx, err
	}
	if dv.DeletionTimestamp != nil {
		log.Info("DataVolume marked for deletion, cleaning up")
		if cleanup != nil {
			err := cleanup(dv)
			return ctx, err
		}
		ctx.res = &reconcile.Result{}
		return ctx, nil
	}

	if prepare != nil {
		if err := prepare(dv); err != nil {
			return ctx, err
		}
	}

	pvc, err := r.getPVC(dv)
	if err != nil {
		return ctx, err
	}
	if pvc != nil {
		res, err := r.garbageCollect(dv, pvc, log)
		if err != nil {
			return ctx, err
		}
		if res != nil {
			ctx.res = res
			return ctx, nil
		}
		if err := r.validatePVC(dv, pvc); err != nil {
			return ctx, err
		}
	}

	pvcSpec, err := renderPvcSpec(r.client, r.recorder, log, dv)
	if err != nil {
		return ctx, err
	}
	ctx.dv = dv
	ctx.pvc = pvc
	ctx.pvcSpec = pvcSpec
	return ctx, nil
}

func (r *ReconcilerBase) validatePVC(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	// If the PVC is not controlled by this DataVolume resource, we should log
	// a warning to the event recorder and return
	pvcPopulated := pvcIsPopulated(pvc, dv)
	if !metav1.IsControlledBy(pvc, dv) {
		if pvcPopulated {
			if err := r.addOwnerRef(pvc, dv); err != nil {
				return err
			}
		} else {
			msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
			r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceExists, msg)
			return errors.Errorf(msg)
		}
	}
	// If the PVC is being deleted, we should log a warning to the event recorder and return to wait the deletion complete
	if pvc.DeletionTimestamp != nil {
		msg := fmt.Sprintf(MessageResourceMarkedForDeletion, pvc.Name)
		r.recorder.Event(dv, corev1.EventTypeWarning, ErrResourceMarkedForDeletion, msg)
		return errors.Errorf(msg)
	}
	return nil
}

func (r *ReconcilerBase) getPVC(dv *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	key := types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}
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

func (r *ReconcilerBase) createPvcForDatavolume(datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec,
	pvcModifier func(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim)) (*corev1.PersistentVolumeClaim, error) {
	newPvc, err := r.newPersistentVolumeClaim(datavolume, pvcSpec, datavolume.Namespace, datavolume.Name)
	if err != nil {
		return nil, err
	}
	util.SetRecommendedLabels(newPvc, r.installerLabels, "cdi-controller")

	if pvcModifier != nil {
		pvcModifier(datavolume, newPvc)
	}
	if err := r.client.Create(context.TODO(), newPvc); err != nil {
		return nil, err
	}

	return newPvc, nil
}

func (r *ReconcilerBase) getStorageClassBindingMode(storageClassName *string) (*storagev1.VolumeBindingMode, error) {
	// Handle unspecified storage class name, fallback to default storage class
	storageClass, err := cc.GetStorageClassByName(r.client, storageClassName)
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

func (r *ReconcilerBase) reconcileProgressUpdate(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	var podNamespace string
	if datavolume.Status.Progress == "" {
		datavolume.Status.Progress = "N/A"
	}

	if datavolume.Spec.Source.PVC != nil {
		podNamespace = datavolume.Spec.Source.PVC.Namespace
	} else {
		podNamespace = datavolume.Namespace
	}

	if datavolume.Status.Phase == cdiv1.Succeeded || datavolume.Status.Phase == cdiv1.Failed {
		// Data volume completed progress, or failed, either way stop queueing the data volume.
		r.log.Info("Datavolume finished, no longer updating progress", "Namespace", datavolume.Namespace, "Name", datavolume.Name, "Phase", datavolume.Status.Phase)
		return reconcile.Result{}, nil
	}
	pod, err := r.getPodFromPvc(podNamespace, pvc)
	if err == nil {
		if pod.Status.Phase != corev1.PodRunning {
			// Avoid long timeouts and error traces from HTTP get when pod is already gone
			return reconcile.Result{}, nil
		}
		if err := updateProgressUsingPod(datavolume, pod); err != nil {
			return reconcile.Result{}, err
		}
	}
	// We are not done yet, force a re-reconcile in 2 seconds to get an update.
	return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
}

type modifyFunc func(dataVolume *cdiv1.DataVolume)

func (r *ReconcilerBase) updateDataVolumeStatusPhaseWithEvent(
	phase cdiv1.DataVolumePhase,
	dataVolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	modify modifyFunc,
	event Event) error {

	var dataVolumeCopy = dataVolume.DeepCopy()
	curPhase := dataVolumeCopy.Status.Phase

	dataVolumeCopy.Status.Phase = phase

	reason := ""
	if pvc == nil {
		reason = event.reason
	}
	r.updateConditions(dataVolumeCopy, pvc, reason)

	if modify != nil {
		modify(dataVolumeCopy)
	}

	return r.emitEvent(dataVolume, dataVolumeCopy, curPhase, dataVolume.Status.Conditions, &event)
}

type updateStatusPhaseFunc func(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error

func (r ReconcilerBase) reconcileDataVolumeStatus(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, modify modifyFunc, updateStatusPhase updateStatusPhaseFunc) (reconcile.Result, error) {
	dataVolumeCopy := dataVolume.DeepCopy()
	var event Event
	var err error
	result := reconcile.Result{}

	curPhase := dataVolumeCopy.Status.Phase
	if pvc != nil {
		dataVolumeCopy.Status.ClaimName = pvc.Name

		// the following check is for a case where the request is to create a blank disk for a block device.
		// in that case, we do not create a pod as there is no need to create a blank image.
		// instead, we just mark the DV phase as 'Succeeded' so any consumer will be able to use it.
		phase := pvc.Annotations[cc.AnnPodPhase]
		if phase == string(cdiv1.Succeeded) {
			if err := updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
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
					if dataVolumeCopy.Annotations == nil {
						dataVolumeCopy.Annotations = make(map[string]string)
					}
					dataVolumeCopy.Annotations[cc.AnnPrePopulated] = pvc.Name
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				} else {
					if err := updateStatusPhase(pvc, dataVolumeCopy, &event); err != nil {
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
		result, err = r.reconcileProgressUpdate(dataVolumeCopy, pvc)
		if err != nil {
			return result, err
		}
	} else {
		_, ok := dataVolumeCopy.Annotations[cc.AnnPrePopulated]
		if ok {
			dataVolumeCopy.Status.Phase = cdiv1.Pending
		}
	}

	if modify != nil {
		modify(dataVolumeCopy)
	}

	currentCond := make([]cdiv1.DataVolumeCondition, len(dataVolumeCopy.Status.Conditions))
	copy(currentCond, dataVolumeCopy.Status.Conditions)
	r.updateConditions(dataVolumeCopy, pvc, "")
	return result, r.emitEvent(dataVolume, dataVolumeCopy, curPhase, currentCond, &event)
}

func (r *ReconcilerBase) updateConditions(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, reason string) {
	var anno map[string]string

	if dataVolume.Status.Conditions == nil {
		dataVolume.Status.Conditions = make([]cdiv1.DataVolumeCondition, 0)
	}

	if pvc != nil {
		anno = pvc.Annotations
	} else {
		anno = make(map[string]string)
	}

	readyStatus := corev1.ConditionUnknown
	switch dataVolume.Status.Phase {
	case cdiv1.Succeeded:
		readyStatus = corev1.ConditionTrue
	case cdiv1.Unknown:
		readyStatus = corev1.ConditionUnknown
	default:
		readyStatus = corev1.ConditionFalse
	}

	dataVolume.Status.Conditions = updateBoundCondition(dataVolume.Status.Conditions, pvc, reason)
	dataVolume.Status.Conditions = UpdateReadyCondition(dataVolume.Status.Conditions, readyStatus, "", reason)
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
	// Only update the object if something actually changed in the status.
	if !reflect.DeepEqual(dataVolume, dataVolumeCopy) {
		if err := r.updateDataVolume(dataVolumeCopy); err != nil {
			r.log.Error(err, "Unable to update datavolume", "name", dataVolumeCopy.Name)
			return err
		}
		// Emit the event only when the status change happens, not every time
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
		if shouldIgnorePod(&pod, pvc) {
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

// If this is a completed pod that was used for one checkpoint of a multi-stage import, it
// should be ignored by pod lookups as long as the retainAfterCompletion annotation is set.
func shouldIgnorePod(pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim) bool {
	retain := pvc.ObjectMeta.Annotations[cc.AnnPodRetainAfterCompletion]
	checkpoint := pvc.ObjectMeta.Annotations[cc.AnnCurrentCheckpoint]
	if checkpoint != "" && pod.Status.Phase == corev1.PodSucceeded {
		return retain == "true"
	}
	return false
}

func updateProgressUsingPod(dataVolumeCopy *cdiv1.DataVolume, pod *corev1.Pod) error {
	httpClient := buildHTTPClient()
	// Example value: import_progress{ownerUID="b856691e-1038-11e9-a5ab-525500d15501"} 13.45
	var importRegExp = regexp.MustCompile("progress\\{ownerUID\\=\"" + string(dataVolumeCopy.UID) + "\"\\} (\\d{1,3}\\.?\\d*)")

	port, err := getPodMetricsPort(pod)
	if err == nil && pod.Status.PodIP != "" {
		url := fmt.Sprintf("https://%s:%d/metrics", pod.Status.PodIP, port)
		resp, err := httpClient.Get(url)
		if err != nil {
			if errConnectionRefused(err) {
				return nil
			}
			return err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		match := importRegExp.FindStringSubmatch(string(body))
		if match == nil {
			// No match
			return nil
		}
		if f, err := strconv.ParseFloat(match[1], 64); err == nil {
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress(fmt.Sprintf("%.2f%%", f))
		}
		return nil
	}
	return err
}

func errConnectionRefused(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}

func getPodMetricsPort(pod *corev1.Pod) (int, error) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "metrics" {
				return int(port.ContainerPort), nil
			}
		}
	}
	return 0, errors.New("Metrics port not found in pod")
}

// buildHTTPClient generates an http client that accepts any certificate, since we are using
// it to get prometheus data it doesn't matter if someone can intercept the data. Once we have
// a mechanism to properly sign the server, we can update this method to get a proper client.
func buildHTTPClient() *http.Client {
	if httpClient == nil {
		defaultTransport := http.DefaultTransport.(*http.Transport)
		// Create new Transport that ignores self-signed SSL
		tr := &http.Transport{
			Proxy:                 defaultTransport.Proxy,
			DialContext:           defaultTransport.DialContext,
			MaxIdleConns:          defaultTransport.MaxIdleConns,
			IdleConnTimeout:       defaultTransport.IdleConnTimeout,
			ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
			TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		}
		httpClient = &http.Client{
			Transport: tr,
		}
	}
	return httpClient
}

// newPersistentVolumeClaim creates a new PVC the DataVolume resource.
// It also sets the appropriate OwnerReferences on the resource
// which allows handleObject to discover the DataVolume resource
// that 'owns' it.
func (r *ReconcilerBase) newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume, targetPvcSpec *corev1.PersistentVolumeClaimSpec, namespace string, name string) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		common.CDILabelKey: common.CDILabelValue,
	}
	if util.ResolveVolumeMode(targetPvcSpec.VolumeMode) == corev1.PersistentVolumeFilesystem {
		labels[common.KubePersistentVolumeFillingUpSuppressLabelKey] = common.KubePersistentVolumeFillingUpSuppressLabelValue
	}
	annotations := make(map[string]string)

	for k, v := range dataVolume.ObjectMeta.Annotations {
		annotations[k] = v
	}

	annotations[cc.AnnPodRestarts] = "0"
	annotations[cc.AnnContentType] = string(getContentType(dataVolume))

	if err := r.Reconciler.updateAnnotations(dataVolume, annotations); err != nil {
		return nil, err
	}

	if dataVolume.Spec.PriorityClassName != "" {
		annotations[cc.AnnPriorityClassName] = dataVolume.Spec.PriorityClassName
	}
	annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(r.client, dataVolume))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: *targetPvcSpec,
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

func getContentType(dv *cdiv1.DataVolume) cdiv1.DataVolumeContentType {
	if dv.Spec.ContentType == cdiv1.DataVolumeArchive {
		return cdiv1.DataVolumeArchive
	}
	return cdiv1.DataVolumeKubeVirt
}

// Whenever the controller updates a DV, we must make sure to nil out spec.source when spec.sourceRef is set
func (r *ReconcilerBase) updateDataVolume(dv *cdiv1.DataVolume) error {
	if dv.Spec.SourceRef != nil {
		dv.Spec.Source = nil
	}
	return r.client.Update(context.TODO(), dv)
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
