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

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const controllerAgentName = "datavolume-controller"

const (
	// SuccessSynced provides a const to represent a Synced status
	SuccessSynced = "Synced"
	// ErrResourceExists provides a const to indicate a resource exists error
	ErrResourceExists = "ErrResourceExists"
	// ErrResourceDoesntExist provides a const to indicate a resource doesn't exist error
	ErrResourceDoesntExist = "ErrResourceDoesntExist"
	// ErrClaimLost provides a const to indicate a claim is lost
	ErrClaimLost = "ErrClaimLost"
	// DataVolumeFailed provides a const to represent DataVolume failed status
	DataVolumeFailed = "DataVolumeFailed"
	// ImportScheduled provides a const to indicate import is scheduled
	ImportScheduled = "ImportScheduled"
	// ImportInProgress provides a const to indicate an import is in progress
	ImportInProgress = "ImportInProgress"
	// ImportFailed provides a const to indicate import has failed
	ImportFailed = "ImportFailed"
	// ImportSucceeded provides a const to indicate import has succeeded
	ImportSucceeded = "ImportSucceeded"
	// CloneScheduled provides a const to indicate clone is scheduled
	CloneScheduled = "CloneScheduled"
	// CloneInProgress provides a const to indicate clone is in progress
	CloneInProgress = "CloneInProgress"
	// SnapshotForSmartCloneInProgress provides a const to indicate snapshot creation for smart-clone is in progress
	SnapshotForSmartCloneInProgress = "SnapshotForSmartCloneInProgress"
	// SnapshotForSmartCloneCreated provides a const to indicate snapshot creation for smart-clone has been completed
	SnapshotForSmartCloneCreated = "SnapshotForSmartCloneCreated"
	// SmartClonePVCInProgress provides a const to indicate snapshot creation for smart-clone is in progress
	SmartClonePVCInProgress = "SmartClonePVCInProgress"
	// SmartCloneSourceInUse provides a const to indicate a smart clone is being delayed becasuse the source is in use
	SmartCloneSourceInUse = "SmartCloneSourceInUse"
	// CloneFailed provides a const to indicate clone has failed
	CloneFailed = "CloneFailed"
	// CloneSucceeded provides a const to indicate clone has succeeded
	CloneSucceeded = "CloneSucceeded"
	// UploadScheduled provides a const to indicate upload is scheduled
	UploadScheduled = "UploadScheduled"
	// UploadReady provides a const to indicate upload is in progress
	UploadReady = "UploadReady"
	// UploadFailed provides a const to indicate upload has failed
	UploadFailed = "UploadFailed"
	// UploadSucceeded provides a const to indicate upload has succeeded
	UploadSucceeded = "UploadSucceeded"
	// MessageResourceExists provides a const to form a resource exists error message
	MessageResourceExists = "Resource %q already exists and is not managed by DataVolume"
	// MessageResourceDoesntExist provides a const to form a resource doesn't exist error message
	MessageResourceDoesntExist = "Resource managed by %q doesn't exist"
	// MessageResourceSynced provides a const to standardize a Resource Synced message
	MessageResourceSynced = "DataVolume synced successfully"
	// MessageErrClaimLost provides a const to form claim lost message
	MessageErrClaimLost = "PVC %s lost"
	// MessageImportScheduled provides a const to form import is scheduled message
	MessageImportScheduled = "Import into %s scheduled"
	// MessageImportInProgress provides a const to form import is in progress message
	MessageImportInProgress = "Import into %s in progress"
	// MessageImportFailed provides a const to form import has failed message
	MessageImportFailed = "Failed to import into PVC %s"
	// MessageImportSucceeded provides a const to form import has succeeded message
	MessageImportSucceeded = "Successfully imported into PVC %s"
	// MessageCloneScheduled provides a const to form clone is scheduled message
	MessageCloneScheduled = "Cloning from %s/%s into %s/%s scheduled"
	// MessageCloneInProgress provides a const to form clone is in progress message
	MessageCloneInProgress = "Cloning from %s/%s into %s/%s in progress"
	// MessageCloneFailed provides a const to form clone has failed message
	MessageCloneFailed = "Cloning from %s/%s into %s/%s failed"
	// MessageCloneSucceeded provides a const to form clone has succeeded message
	MessageCloneSucceeded = "Successfully cloned from %s/%s into %s/%s"
	// MessageSmartCloneInProgress provides a const to form snapshot for smart-clone is in progress message
	MessageSmartCloneInProgress = "Creating snapshot for smart-clone is in progress (for pvc %s/%s)"
	// MessageSmartClonePVCInProgress provides a const to form snapshot for smart-clone is in progress message
	MessageSmartClonePVCInProgress = "Creating PVC for smart-clone is in progress (for pvc %s/%s)"
	// MessageUploadScheduled provides a const to form upload is scheduled message
	MessageUploadScheduled = "Upload into %s scheduled"
	// MessageUploadReady provides a const to form upload is ready message
	MessageUploadReady = "Upload into %s ready"
	// MessageUploadFailed provides a const to form upload has failed message
	MessageUploadFailed = "Upload into %s failed"
	// MessageUploadSucceeded provides a const to form upload has succeeded message
	MessageUploadSucceeded = "Successfully uploaded into %s"
)

var httpClient *http.Client

// DataVolumeEvent reoresents event
type DataVolumeEvent struct {
	eventType string
	reason    string
	message   string
}

// DatavolumeReconciler members
type DatavolumeReconciler struct {
	client       client.Client
	extClientSet extclientset.Interface
	recorder     record.EventRecorder
	scheme       *runtime.Scheme
	log          logr.Logger
	featureGates featuregates.FeatureGates
}

func pvcIsPopulated(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
	dvName, ok := pvc.Annotations[AnnPopulatedFor]
	return ok && dvName == dv.Name
}

// NewDatavolumeController creates a new instance of the datavolume controller.
func NewDatavolumeController(mgr manager.Manager, extClientSet extclientset.Interface, log logr.Logger) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &DatavolumeReconciler{
		client:       client,
		scheme:       mgr.GetScheme(),
		extClientSet: extClientSet,
		log:          log.WithName("datavolume-controller"),
		recorder:     mgr.GetEventRecorderFor("datavolume-controller"),
		featureGates: featuregates.NewFeatureGates(client),
	}
	datavolumeController, err := controller.New("datavolume-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addDatavolumeControllerWatches(mgr, datavolumeController); err != nil {
		return nil, err
	}
	return datavolumeController, nil
}

func addDatavolumeControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	// Add schemes.
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := storagev1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := snapshotv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	// Setup watches
	if err := datavolumeController.Watch(&source.Kind{Type: &cdiv1.DataVolume{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := datavolumeController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}); err != nil {
		return err
	}

	return nil
}

// Reconcile the reconcile loop for the data volumes.
func (r *DatavolumeReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("Datavolume", req.NamespacedName)

	// Get the Datavolume.
	datavolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, datavolume); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if datavolume.DeletionTimestamp != nil {
		log.Info("Datavolume marked for deletion, skipping")
		return reconcile.Result{}, nil
	}

	pvcExists := true
	// Get the pvc with the name specified in DataVolume.spec
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: datavolume.Namespace, Name: datavolume.Name}, pvc); err != nil {
		// If the resource doesn't exist, we'll create it
		if k8serrors.IsNotFound(err) {
			pvcExists = false
		} else if err != nil {
			return reconcile.Result{}, err
		}

	} else {
		// If the PVC is not controlled by this DataVolume resource, we should log
		// a warning to the event recorder and return
		if !metav1.IsControlledBy(pvc, datavolume) {
			if pvcIsPopulated(pvc, datavolume) {
				if err := r.addOwnerRef(pvc, datavolume); err != nil {
					return reconcile.Result{}, err
				}
			} else {
				msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
				r.recorder.Event(datavolume, corev1.EventTypeWarning, ErrResourceExists, msg)
				return reconcile.Result{}, errors.Errorf(msg)
			}
		}
	}

	if !pvcExists {
		snapshotClassName, err := r.getSnapshotClassForSmartClone(datavolume)
		if err == nil {
			r.log.V(3).Info("Smart-Clone via Snapshot is available with Volume Snapshot Class", "snapshotClassName", snapshotClassName)
			if requeue, err := r.sourceInUse(datavolume); requeue || err != nil {
				return reconcile.Result{Requeue: requeue}, err
			}
			if populated, err := r.isSourcePVCPopulated(datavolume); !populated || err != nil {
				return reconcile.Result{Requeue: !populated}, err
			}
			newSnapshot := newSnapshot(datavolume, snapshotClassName)
			if err := r.client.Create(context.TODO(), newSnapshot); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return reconcile.Result{}, nil
				}
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, r.updateSmartCloneStatusPhase(cdiv1.SnapshotForSmartCloneInProgress, datavolume)
		}
		log.Info("Creating PVC for datavolume")
		newPvc, err := newPersistentVolumeClaim(datavolume)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err := r.client.Create(context.TODO(), newPvc); err != nil {
			return reconcile.Result{}, err
		}
		pvc = newPvc
	}

	// Finally, we update the status block of the DataVolume resource to reflect the
	// current state of the world
	return r.reconcileDataVolumeStatus(datavolume, pvc)
}

// Verify that the source PVC has been completely populated.
func (r *DatavolumeReconciler) isSourcePVCPopulated(dv *cdiv1.DataVolume) (bool, error) {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.Source.PVC.Name, Namespace: dv.Spec.Source.PVC.Namespace}, sourcePvc); err != nil {
		return false, err
	}
	return IsPopulated(sourcePvc, r.client)
}

func (r *DatavolumeReconciler) sourceInUse(dv *cdiv1.DataVolume) (bool, error) {
	pods, err := getPodsUsingPVCs(r.client, dv.Spec.Source.PVC.Namespace, sets.NewString(dv.Spec.Source.PVC.Name), false)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		r.log.V(1).Info("Cannot snapshot",
			"namespace", dv.Namespace, "name", dv.Name, "pod namespace", pod.Namespace, "pod name", pod.Name)
		r.recorder.Eventf(dv, corev1.EventTypeWarning, SmartCloneSourceInUse,
			"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, dv.Spec.Source.PVC.Name)
	}

	return len(pods) > 0, nil
}

func (r *DatavolumeReconciler) getStorageClassBindingMode(dataVolume *cdiv1.DataVolume) (*storagev1.VolumeBindingMode, error) {
	// Handle unspecified storage class name, fallback to default storage class
	storageClass, err := r.getStorageClassByName(dataVolume.Spec.PVC.StorageClassName)
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

func (r *DatavolumeReconciler) reconcileProgressUpdate(datavolume *cdiv1.DataVolume, pvcUID types.UID) (reconcile.Result, error) {
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
	pod, err := r.getPodFromPvc(podNamespace, pvcUID)
	if err == nil {
		if err := updateProgressUsingPod(datavolume, pod); err != nil {
			return reconcile.Result{}, err
		}
	}
	// We are not done yet, force a re-reconcile in 2 seconds to get an update.
	return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
}

func (r *DatavolumeReconciler) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume) (string, error) {
	// TODO: Figure out if this belongs somewhere else, seems like something for the smart clone controller.
	// Check if clone is requested
	if dataVolume.Spec.Source.PVC == nil {
		return "", errors.New("no source PVC provided")
	}

	// Check if relevant CRDs are available
	if !IsCsiCrdsDeployed(r.extClientSet) {
		r.log.V(3).Info("Missing CSI snapshotter CRDs, falling back to host assisted clone")
		return "", errors.New("CSI snapshot CRDs not found")
	}

	// Find source PVC
	sourcePvcNs := dataVolume.Spec.Source.PVC.Namespace
	if sourcePvcNs == "" {
		sourcePvcNs = dataVolume.Namespace
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: sourcePvcNs, Name: dataVolume.Spec.Source.PVC.Name}, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			r.log.V(3).Info("Source PVC is missing", "source namespace", dataVolume.Spec.Source.PVC.Namespace, "source name", dataVolume.Spec.Source.PVC.Name)
		}
		return "", errors.New("source PVC not found")
	}

	targetPvcStorageClassName := dataVolume.Spec.PVC.StorageClassName
	targetStorageClass, err := r.getStorageClassByName(targetPvcStorageClassName)
	if err != nil {
		return "", err
	}
	if targetStorageClass == nil {
		r.log.V(3).Info("Target PVC's Storage Class not found")
		return "", errors.New("Target PVC storage class not found")
	}
	targetPvcStorageClassName = &targetStorageClass.Name
	sourcePvcStorageClassName := pvc.Spec.StorageClassName

	// Compare source and target storage classess
	if *sourcePvcStorageClassName != *targetPvcStorageClassName {
		r.log.V(3).Info("Source PVC and target PVC belong to different storage classes", "source storage class",
			*sourcePvcStorageClassName, "target storage class", *targetPvcStorageClassName)
		return "", errors.New("source PVC and target PVC belong to different storage classes")
	}

	// Compare source and target namespaces
	if pvc.Namespace != dataVolume.Namespace {
		r.log.V(3).Info("Source PVC and target PVC belong to different namespaces", "source namespace",
			pvc.Namespace, "target namespace", dataVolume.Namespace)
		return "", errors.New("source PVC and target PVC belong to different namespaces")
	}

	// Fetch the source storage class
	srcStorageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *sourcePvcStorageClassName}, srcStorageClass); err != nil {
		r.log.V(3).Info("Unable to retrieve storage class, falling back to host assisted clone", "storage class", *sourcePvcStorageClassName)
		return "", errors.New("unable to retrieve storage class, falling back to host assisted clone")
	}

	// List the snapshot classes
	scs := &snapshotv1.VolumeSnapshotClassList{}
	if err := r.client.List(context.TODO(), scs); err != nil {
		r.log.V(3).Info("Cannot list snapshot classes, falling back to host assisted clone")
		return "", errors.New("cannot list snapshot classes, falling back to host assisted clone")
	}
	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Driver == srcStorageClass.Provisioner {
			r.log.V(3).Info("smart-clone is applicable for datavolume", "datavolume",
				dataVolume.Name, "snapshot class", snapshotClass.Name)
			return snapshotClass.Name, nil
		}
	}

	r.log.V(3).Info("Could not match snapshotter with storage class, falling back to host assisted clone")
	return "", errors.New("could not match snapshotter with storage class, falling back to host assisted clone")
}

// getStorageClassByName looks up the storage class based on the name. If no storage class is found returns nil
func (r *DatavolumeReconciler) getStorageClassByName(name *string) (*storagev1.StorageClass, error) {
	// look up storage class by name
	if name == nil {
		storageClasses := &storagev1.StorageClassList{}
		if err := r.client.List(context.TODO(), storageClasses); err != nil {
			r.log.V(3).Info("Unable to retrieve available storage classes")
			return nil, errors.New("unable to retrieve storage classes")
		}
		for _, storageClass := range storageClasses.Items {
			if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				return &storageClass, nil
			}
		}
	} else {
		storageClass := &storagev1.StorageClass{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *name}, storageClass); err != nil {
			r.log.V(3).Info("Unable to retrieve storage class", "storage class name", *name)
			return nil, errors.New("unable to retrieve storage class")
		}
		return storageClass, nil
	}
	// No storage class found, just return nil for storage class and let caller deal with it.
	return nil, nil
}

func newSnapshot(dataVolume *cdiv1.DataVolume, snapshotClassName string) *snapshotv1.VolumeSnapshot {
	annotations := make(map[string]string)
	annotations[AnnSmartCloneRequest] = "true"
	className := snapshotClassName
	labels := map[string]string{
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolume.Name,
			Namespace:   dataVolume.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(dataVolume, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataVolume",
				}),
			},
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &dataVolume.Spec.Source.PVC.Name,
			},
			VolumeSnapshotClassName: &className,
		},
	}
	return snapshot
}

func (r *DatavolumeReconciler) updateImportStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if ok {
		switch phase {
		case string(corev1.PodPending):
			// TODO: Use a more generic Scheduled, like maybe TransferScheduled.
			dataVolumeCopy.Status.Phase = cdiv1.ImportScheduled
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportScheduled
			event.message = fmt.Sprintf(MessageImportScheduled, pvc.Name)
		case string(corev1.PodRunning):
			// TODO: Use a more generic In Progess, like maybe TransferInProgress.
			dataVolumeCopy.Status.Phase = cdiv1.ImportInProgress
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportInProgress
			event.message = fmt.Sprintf(MessageImportInProgress, pvc.Name)
		case string(corev1.PodFailed):
			dataVolumeCopy.Status.Phase = cdiv1.Failed
			event.eventType = corev1.EventTypeWarning
			event.reason = ImportFailed
			event.message = fmt.Sprintf(MessageImportFailed, pvc.Name)
		case string(corev1.PodSucceeded):
			dataVolumeCopy.Status.Phase = cdiv1.Succeeded
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress("100.0%")
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportSucceeded
			event.message = fmt.Sprintf(MessageImportSucceeded, pvc.Name)
		}
	}
}

func (r *DatavolumeReconciler) updateSmartCloneStatusPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume) error {
	var dataVolumeCopy = dataVolume.DeepCopy()
	var event DataVolumeEvent

	curPhase := dataVolumeCopy.Status.Phase

	switch phase {
	case cdiv1.SnapshotForSmartCloneInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.SnapshotForSmartCloneInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = SnapshotForSmartCloneInProgress
		event.message = fmt.Sprintf(MessageSmartCloneInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name)
	}

	return r.emitEvent(dataVolume, dataVolumeCopy, curPhase, dataVolume.Status.Conditions, &event)
}

func (r *DatavolumeReconciler) updateCloneStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if ok {
		switch phase {
		case string(corev1.PodPending):
			// TODO: Use a more generic Scheduled, like maybe TransferScheduled.
			dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
			event.eventType = corev1.EventTypeNormal
			event.reason = CloneScheduled
			event.message = fmt.Sprintf(MessageCloneScheduled, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
		case string(corev1.PodRunning):
			// TODO: Use a more generic In Progess, like maybe TransferInProgress.
			dataVolumeCopy.Status.Phase = cdiv1.CloneInProgress
			event.eventType = corev1.EventTypeNormal
			event.reason = CloneInProgress
			event.message = fmt.Sprintf(MessageCloneInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
		case string(corev1.PodFailed):
			dataVolumeCopy.Status.Phase = cdiv1.Failed
			event.eventType = corev1.EventTypeWarning
			event.reason = CloneFailed
			event.message = fmt.Sprintf(MessageCloneFailed, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
		case string(corev1.PodSucceeded):
			dataVolumeCopy.Status.Phase = cdiv1.Succeeded
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress("100.0%")
			event.eventType = corev1.EventTypeNormal
			event.reason = CloneSucceeded
			event.message = fmt.Sprintf(MessageCloneSucceeded, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
		}

	}
}

func (r *DatavolumeReconciler) updateUploadStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if ok {
		switch phase {
		case string(corev1.PodPending):
			// TODO: Use a more generic Scheduled, like maybe TransferScheduled.
			dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
			event.eventType = corev1.EventTypeNormal
			event.reason = UploadScheduled
			event.message = fmt.Sprintf(MessageUploadScheduled, pvc.Name)
		case string(corev1.PodRunning):
			running := pvc.Annotations[AnnPodReady]
			if running == "true" {
				// TODO: Use a more generic In Progess, like maybe TransferInProgress.
				dataVolumeCopy.Status.Phase = cdiv1.UploadReady
				event.eventType = corev1.EventTypeNormal
				event.reason = UploadReady
				event.message = fmt.Sprintf(MessageUploadReady, pvc.Name)
			}
		case string(corev1.PodFailed):
			dataVolumeCopy.Status.Phase = cdiv1.Failed
			event.eventType = corev1.EventTypeWarning
			event.reason = UploadFailed
			event.message = fmt.Sprintf(MessageUploadFailed, pvc.Name)
		case string(corev1.PodSucceeded):
			dataVolumeCopy.Status.Phase = cdiv1.Succeeded
			event.eventType = corev1.EventTypeNormal
			event.reason = UploadSucceeded
			event.message = fmt.Sprintf(MessageUploadSucceeded, pvc.Name)
		}
	}
}

func (r *DatavolumeReconciler) reconcileDataVolumeStatus(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	dataVolumeCopy := dataVolume.DeepCopy()
	var event DataVolumeEvent

	storageClassBindingMode, err := r.getStorageClassBindingMode(dataVolume)
	if err != nil {
		return reconcile.Result{}, err
	}

	result := reconcile.Result{}

	curPhase := dataVolumeCopy.Status.Phase
	if pvc != nil {
		// the following check is for a case where the request is to create a blank disk for a block device.
		// in that case, we do not create a pod as there is no need to create a blank image.
		// instead, we just mark the DV phase as 'Succeeded' so any consumer will be able to use it.
		phase, _ := pvc.Annotations[AnnPodPhase]
		if phase == string(cdiv1.Succeeded) {
			updateImport := true
			_, ok := pvc.Annotations[AnnCloneRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
				r.updateCloneStatusPhase(pvc, dataVolumeCopy, &event)
				updateImport = false
			}
			_, ok = pvc.Annotations[AnnUploadRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
				r.updateUploadStatusPhase(pvc, dataVolumeCopy, &event)
				updateImport = false
			}
			if updateImport {
				dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				r.updateImportStatusPhase(pvc, dataVolumeCopy, &event)
			}
		} else {
			switch pvc.Status.Phase {
			case corev1.ClaimPending:
				honorWaitForFirstConsumerEnabled, err := r.featureGates.HonorWaitForFirstConsumerEnabled()
				if err != nil {
					return reconcile.Result{}, err
				}
				if honorWaitForFirstConsumerEnabled &&
					*storageClassBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
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
					dataVolumeCopy.Annotations[AnnPrePopulated] = pvc.Name
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				} else {
					_, ok := pvc.Annotations[AnnImportPod]
					if ok {
						dataVolumeCopy.Status.Phase = cdiv1.ImportScheduled
						r.updateImportStatusPhase(pvc, dataVolumeCopy, &event)
					}
					_, ok = pvc.Annotations[AnnCloneRequest]
					if ok {
						dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
						r.updateCloneStatusPhase(pvc, dataVolumeCopy, &event)
					}
					_, ok = pvc.Annotations[AnnUploadRequest]
					if ok {
						dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
						r.updateUploadStatusPhase(pvc, dataVolumeCopy, &event)
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
		if i, err := strconv.Atoi(pvc.Annotations[AnnPodRestarts]); err == nil && i >= 0 {
			dataVolumeCopy.Status.RestartCount = int32(i)
		}
		result, err = r.reconcileProgressUpdate(dataVolumeCopy, pvc.GetUID())
		if err != nil {
			return result, err
		}
	}

	currentCond := make([]cdiv1.DataVolumeCondition, len(dataVolumeCopy.Status.Conditions))
	copy(currentCond, dataVolumeCopy.Status.Conditions)
	r.updateConditions(dataVolumeCopy, pvc)
	return result, r.emitEvent(dataVolume, dataVolumeCopy, curPhase, currentCond, &event)
}

func (r *DatavolumeReconciler) updateConditions(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) {
	var anno map[string]string

	if dataVolume.Status.Conditions == nil {
		dataVolume.Status.Conditions = make([]cdiv1.DataVolumeCondition, 0)
	}

	if pvc != nil {
		anno = pvc.Annotations
	} else {
		anno = make(map[string]string, 0)
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

	dataVolume.Status.Conditions = updateBoundCondition(dataVolume.Status.Conditions, pvc)
	dataVolume.Status.Conditions = updateReadyCondition(dataVolume.Status.Conditions, readyStatus, "", "")
	dataVolume.Status.Conditions = updateRunningCondition(dataVolume.Status.Conditions, anno)
}

func (r *DatavolumeReconciler) emitConditionEvent(dataVolume *cdiv1.DataVolume, originalCond []cdiv1.DataVolumeCondition) {
	r.emitBoundConditionEvent(dataVolume, findConditionByType(cdiv1.DataVolumeBound, dataVolume.Status.Conditions), findConditionByType(cdiv1.DataVolumeBound, originalCond))
	r.emitFailureConditionEvent(dataVolume, originalCond)
}

func (r *DatavolumeReconciler) emitBoundConditionEvent(dataVolume *cdiv1.DataVolume, current, original *cdiv1.DataVolumeCondition) {
	// We know reason and message won't be empty for bound.
	if current != nil && (original == nil || current.Status != original.Status || current.Reason != original.Reason || current.Message != original.Message) {
		r.recorder.Event(dataVolume, corev1.EventTypeNormal, current.Reason, current.Message)
	}
}

func (r *DatavolumeReconciler) emitFailureConditionEvent(dataVolume *cdiv1.DataVolume, originalCond []cdiv1.DataVolumeCondition) {
	curReady := findConditionByType(cdiv1.DataVolumeReady, dataVolume.Status.Conditions)
	curBound := findConditionByType(cdiv1.DataVolumeBound, dataVolume.Status.Conditions)
	curRunning := findConditionByType(cdiv1.DataVolumeRunning, dataVolume.Status.Conditions)
	orgRunning := findConditionByType(cdiv1.DataVolumeRunning, originalCond)

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

func (r *DatavolumeReconciler) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, curPhase cdiv1.DataVolumePhase, originalCond []cdiv1.DataVolumeCondition, event *DataVolumeEvent) error {
	// Only update the object if something actually changed in the status.
	if !reflect.DeepEqual(dataVolume, dataVolumeCopy) {
		if err := r.client.Update(context.TODO(), dataVolumeCopy); err != nil {
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

// getPodFromPvc determines the pod associated with the pvc UID passed in.
func (r *DatavolumeReconciler) getPodFromPvc(namespace string, pvcUID types.UID) (*corev1.Pod, error) {
	l, _ := labels.Parse(common.PrometheusLabel)
	pods := &corev1.PodList{}
	listOptions := client.ListOptions{
		LabelSelector: l,
	}
	if err := r.client.List(context.TODO(), pods, &listOptions); err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		for _, or := range pod.OwnerReferences {
			if or.UID == pvcUID {
				return &pod, nil
			}
		}

		// TODO: check this
		val, exists := pod.Labels[CloneUniqueID]
		if exists && val == string(pvcUID)+common.ClonerSourcePodNameSuffix {
			return &pod, nil
		}
	}
	return nil, errors.Errorf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvcUID), namespace)
}

func (r *DatavolumeReconciler) addOwnerRef(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) error {
	if err := controllerutil.SetControllerReference(dv, pvc, r.scheme); err != nil {
		return err
	}

	return r.client.Update(context.TODO(), pvc)
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
func newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		"app": "containerized-data-importer",
	}

	if dataVolume.Spec.PVC == nil {
		// TODO remove this requirement and dynamically generate
		// PVC spec if not present on DataVolume
		return nil, errors.Errorf("datavolume.pvc field is required")
	}

	annotations := make(map[string]string)

	for k, v := range dataVolume.ObjectMeta.Annotations {
		annotations[k] = v
	}

	annotations[AnnPodRestarts] = "0"
	if dataVolume.Spec.Source.HTTP != nil {
		annotations[AnnEndpoint] = dataVolume.Spec.Source.HTTP.URL
		annotations[AnnSource] = SourceHTTP
		if dataVolume.Spec.ContentType == cdiv1.DataVolumeArchive {
			annotations[AnnContentType] = string(cdiv1.DataVolumeArchive)
		} else {
			annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		}
		if dataVolume.Spec.Source.HTTP.SecretRef != "" {
			annotations[AnnSecret] = dataVolume.Spec.Source.HTTP.SecretRef
		}
		if dataVolume.Spec.Source.HTTP.CertConfigMap != "" {
			annotations[AnnCertConfigMap] = dataVolume.Spec.Source.HTTP.CertConfigMap
		}
	} else if dataVolume.Spec.Source.S3 != nil {
		annotations[AnnEndpoint] = dataVolume.Spec.Source.S3.URL
		annotations[AnnSource] = SourceS3
		if dataVolume.Spec.Source.S3.SecretRef != "" {
			annotations[AnnSecret] = dataVolume.Spec.Source.S3.SecretRef
		}
	} else if dataVolume.Spec.Source.Registry != nil {
		annotations[AnnSource] = SourceRegistry
		annotations[AnnEndpoint] = dataVolume.Spec.Source.Registry.URL
		annotations[AnnContentType] = string(dataVolume.Spec.ContentType)
		if dataVolume.Spec.Source.Registry.SecretRef != "" {
			annotations[AnnSecret] = dataVolume.Spec.Source.Registry.SecretRef
		}
		if dataVolume.Spec.Source.Registry.CertConfigMap != "" {
			annotations[AnnCertConfigMap] = dataVolume.Spec.Source.Registry.CertConfigMap
		}
	} else if dataVolume.Spec.Source.PVC != nil {
		sourceNamespace := dataVolume.Spec.Source.PVC.Namespace
		if sourceNamespace == "" {
			sourceNamespace = dataVolume.Namespace
		}
		token, ok := dataVolume.Annotations[AnnCloneToken]
		if !ok {
			return nil, errors.Errorf("no clone token")
		}
		annotations[AnnCloneToken] = token
		annotations[AnnCloneRequest] = sourceNamespace + "/" + dataVolume.Spec.Source.PVC.Name
	} else if dataVolume.Spec.Source.Upload != nil {
		annotations[AnnUploadRequest] = ""
	} else if dataVolume.Spec.Source.Blank != nil {
		annotations[AnnSource] = SourceNone
		annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
	} else if dataVolume.Spec.Source.Imageio != nil {
		annotations[AnnEndpoint] = dataVolume.Spec.Source.Imageio.URL
		annotations[AnnSource] = SourceImageio
		annotations[AnnSecret] = dataVolume.Spec.Source.Imageio.SecretRef
		annotations[AnnCertConfigMap] = dataVolume.Spec.Source.Imageio.CertConfigMap
		annotations[AnnDiskID] = dataVolume.Spec.Source.Imageio.DiskID
	} else if dataVolume.Spec.Source.VDDK != nil {
		annotations[AnnEndpoint] = dataVolume.Spec.Source.VDDK.URL
		annotations[AnnSource] = SourceVDDK
		annotations[AnnSecret] = dataVolume.Spec.Source.VDDK.SecretRef
		annotations[AnnBackingFile] = dataVolume.Spec.Source.VDDK.BackingFile
		annotations[AnnUUID] = dataVolume.Spec.Source.VDDK.UUID
		annotations[AnnThumbprint] = dataVolume.Spec.Source.VDDK.Thumbprint
	} else {
		return nil, errors.Errorf("no source set for datavolume")
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolume.Name,
			Namespace:   dataVolume.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(dataVolume, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataVolume",
				}),
			},
		},
		Spec: *dataVolume.Spec.PVC,
	}, nil
}
