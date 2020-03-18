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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"time"

	csisnapshotv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	cdischeme "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/scheme"
	informers "kubevirt.io/containerized-data-importer/pkg/client/informers/externalversions/core/v1alpha1"
	listers "kubevirt.io/containerized-data-importer/pkg/client/listers/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	expectations "kubevirt.io/containerized-data-importer/pkg/expectations"
	csiclientset "kubevirt.io/containerized-data-importer/pkg/snapshot-client/clientset/versioned"
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

// DataVolumeController represents the CDI Data Volume Controller
type DataVolumeController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// clientset is a clientset for our own API group
	cdiClientSet clientset.Interface
	csiClientSet csiclientset.Interface
	extClientSet extclientset.Interface

	pvcLister  corelisters.PersistentVolumeClaimLister
	pvcsSynced cache.InformerSynced

	dataVolumesLister listers.DataVolumeLister
	dataVolumesSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder

	pvcExpectations *expectations.UIDTrackingControllerExpectations
}

// DataVolumeEvent reoresents event
type DataVolumeEvent struct {
	eventType string
	reason    string
	message   string
}

// NewDataVolumeController sets up a Data Volume Controller, and return a pointer to
// the newly created Controller
func NewDataVolumeController(
	kubeclientset kubernetes.Interface,
	cdiClientSet clientset.Interface,
	csiClientSet csiclientset.Interface,
	extClientSet extclientset.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	dataVolumeInformer informers.DataVolumeInformer) *DataVolumeController {

	// Create event broadcaster
	// Add datavolume-controller types to the default Kubernetes Scheme so Events can be
	// logged for datavolume-controller types.
	cdischeme.AddToScheme(scheme.Scheme)
	klog.V(3).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.V(2).Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &DataVolumeController{
		kubeclientset:     kubeclientset,
		cdiClientSet:      cdiClientSet,
		csiClientSet:      csiClientSet,
		extClientSet:      extClientSet,
		pvcLister:         pvcInformer.Lister(),
		pvcsSynced:        pvcInformer.Informer().HasSynced,
		dataVolumesLister: dataVolumeInformer.Lister(),
		dataVolumesSynced: dataVolumeInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DataVolumes"),
		recorder:          recorder,
		pvcExpectations:   expectations.NewUIDTrackingControllerExpectations(expectations.NewControllerExpectations()),
	}
	klog.V(2).Info("Setting up event handlers")

	// Set up an event handler for when DataVolume resources change
	dataVolumeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueDataVolume,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueDataVolume(new)
		},
		DeleteFunc: controller.enqueueDataVolume,
	})
	// Set up an event handler for when PVC resources change
	// handleObject function ensures we filter PVCs not created by this controller
	pvcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleAddObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.PersistentVolumeClaim)
			oldDepl := old.(*corev1.PersistentVolumeClaim)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known PVCs.
				// Two different versions of the same PVCs will always have different RVs.
				return
			}
			controller.handleUpdateObject(new)
		},
		DeleteFunc: controller.handleDeleteObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *DataVolumeController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.V(2).Info("Starting DataVolume controller")

	// Wait for the caches to be synced before starting workers
	klog.V(2).Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.pvcsSynced, c.dataVolumesSynced); !ok {
		return errors.Errorf("failed to wait for caches to sync")
	}

	klog.V(2).Info("Starting workers")
	// Launch two workers to process DataVolume resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.V(2).Info("Started workers")
	<-stopCh
	klog.V(2).Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *DataVolumeController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *DataVolumeController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(errors.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// DataVolume resource to be synced.
		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return errors.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.V(2).Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the DataVolume resource
// with the current status of the resource.
func (c *DataVolumeController) syncHandler(key string) error {
	exists := true

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DataVolume resource with this namespace/name
	dataVolume, err := c.dataVolumesLister.DataVolumes(namespace).Get(name)
	if err != nil {
		// The DataVolume resource may no longer exist, in which case we stop
		// processing.
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("dataVolume '%s' in work queue no longer exists", key))
			c.pvcExpectations.DeleteExpectations(key)
			return nil
		}

		return err
	}

	if dataVolume.DeletionTimestamp != nil {
		return nil
	}

	// Get the pvc with the name specified in DataVolume.spec
	pvc, err := c.pvcLister.PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		exists = false
	} else if err != nil {
		return err
	}

	// If the PVC is not controlled by this DataVolume resource, we should log
	// a warning to the event recorder and return
	if pvc != nil && !metav1.IsControlledBy(pvc, dataVolume) {
		msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
		c.recorder.Event(dataVolume, corev1.EventTypeWarning, ErrResourceExists, msg)
		return errors.Errorf(msg)
	}

	// expectations prevent us from creating multiple pods. An expectation forces
	// us to observe a pod's creation in the cache.
	needsSync := c.pvcExpectations.SatisfiedExpectations(key)

	if !exists && needsSync {
		snapshotClassName := c.getSnapshotClassForSmartClone(dataVolume)
		if snapshotClassName != "" {
			klog.V(3).Infof("Smart-Clone via Snapshot is available with Volume Snapshot Class: %s", snapshotClassName)
			newSnapshot := newSnapshot(dataVolume, snapshotClassName)
			_, err := c.csiClientSet.SnapshotV1alpha1().VolumeSnapshots(newSnapshot.Namespace).Create(newSnapshot)
			if err != nil {
				return err
			}
			err = c.updateSmartCloneStatusPhase(cdiv1.SnapshotForSmartCloneInProgress, dataVolume)
			if err != nil {
				return err
			}
		} else {
			newPvc, err := newPersistentVolumeClaim(dataVolume)
			if err != nil {
				return err
			}
			c.pvcExpectations.ExpectCreations(key, 1)
			pvc, err = c.kubeclientset.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Create(newPvc)
			if err != nil {
				c.pvcExpectations.CreationObserved(key)
				return err
			}

			c.scheduleProgressUpdate(dataVolume, pvc.GetUID())
		}
	}

	// Finally, we update the status block of the DataVolume resource to reflect the
	// current state of the world
	err = c.updateDataVolumeStatus(dataVolume, pvc)
	if err != nil {
		return err
	}

	c.recorder.Event(dataVolume, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *DataVolumeController) scheduleProgressUpdate(dataVolume *cdiv1.DataVolume, pvcUID types.UID) {
	var podNamespace string
	if dataVolume.Spec.Source.HTTP != nil {
		podNamespace = dataVolume.Namespace
	} else if dataVolume.Spec.Source.PVC != nil {
		podNamespace = dataVolume.Spec.Source.PVC.Namespace
	} else {
		return
	}

	go func() {
		for {
			time.Sleep(2 * time.Second)
			dataVolume, err := c.dataVolumesLister.DataVolumes(dataVolume.Namespace).Get(dataVolume.Name)
			if k8serrors.IsNotFound(err) {
				// Data volume is no longer there, or not found.
				klog.V(3).Info("DV is gone, cancelling update thread.")
				return
			} else if err != nil {
				klog.Errorf("error retrieving data volume %+v", err)
			}
			if dataVolume.Status.Phase == cdiv1.Succeeded || dataVolume.Status.Phase == cdiv1.Failed {
				// Data volume completed progress, or failed, either way stop queueing the data volume.
				klog.V(3).Infof("DV %s/%s phase is %s, no longer updating progress", dataVolume.Namespace, dataVolume.Name, dataVolume.Status.Phase)
				return
			}
			pod, err := c.getPodFromPvc(podNamespace, pvcUID)
			if err == nil {
				c.updateProgressUsingPod(dataVolume, pod)
				_, err = c.cdiClientSet.CdiV1alpha1().DataVolumes(dataVolume.Namespace).Update(dataVolume)
				if err != nil {
					klog.Errorf("Unable to update data volume %s progress %+v", dataVolume.Name, err)
				}
			}
		}
	}()
}

func (c *DataVolumeController) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume) string {
	// Check if clone is requested
	if dataVolume.Spec.Source.PVC == nil {
		return ""
	}

	// Check if relevant CRDs are available
	if !IsCsiCrdsDeployed(c.extClientSet) {
		klog.V(3).Infof("Missing CSI snapshotter CRDs, falling back to host assisted clone")
		return ""
	}

	// Find source PVC
	sourcePvcNs := dataVolume.Spec.Source.PVC.Namespace
	if sourcePvcNs == "" {
		sourcePvcNs = dataVolume.Namespace
	}

	pvc, err := c.pvcLister.PersistentVolumeClaims(sourcePvcNs).Get(dataVolume.Spec.Source.PVC.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.V(3).Infof("Source PVC is missing: %s/%s", dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name)
		}
		runtime.HandleError(err)
		return ""
	}

	targetPvcStorageClassName := dataVolume.Spec.PVC.StorageClassName

	// Handle unspecified storage class name, fallback to default storage class
	if targetPvcStorageClassName == nil {
		storageclasses, err := c.kubeclientset.StorageV1().StorageClasses().List(metav1.ListOptions{})
		if err != nil {
			runtime.HandleError(err)
			klog.V(3).Infof("Unable to retrieve available storage classes, falling back to host assisted clone")
			return ""
		}
		for _, storageClass := range storageclasses.Items {
			if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				targetPvcStorageClassName = &storageClass.Name
				break
			}
		}
	}

	if targetPvcStorageClassName == nil {
		klog.V(3).Infof("Target PVC's Storage Class not found")
		return ""
	}

	sourcePvcStorageClassName := pvc.Spec.StorageClassName

	// Compare source and target storage classess
	if *sourcePvcStorageClassName != *targetPvcStorageClassName {
		klog.V(3).Infof("Source PVC and target PVC belong to different storage classes: %s - %s",
			*sourcePvcStorageClassName, *targetPvcStorageClassName)
		return ""
	}

	// Compare source and target namespaces
	if pvc.Namespace != dataVolume.Namespace {
		klog.V(3).Infof("Source PVC and target PVC belong to different namespaces: %s - %s",
			pvc.Namespace, dataVolume.Namespace)
		return ""
	}

	// Fetch the source storage class
	storageclass, err := c.kubeclientset.StorageV1().StorageClasses().Get(*sourcePvcStorageClassName, metav1.GetOptions{})
	if err != nil {
		runtime.HandleError(err)
		klog.V(3).Infof("Unable to retrieve storage classes %s, falling back to host assisted clone", *sourcePvcStorageClassName)
		return ""
	}

	// List the snapshot classes
	scs, err := c.csiClientSet.SnapshotV1alpha1().VolumeSnapshotClasses().List(metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Cannot list snapshot classes, falling back to host assisted clone")
		return ""
	}
	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Snapshotter == storageclass.Provisioner {
			klog.V(3).Infof("smart-clone is applicable for datavolume '%s' with snapshot class '%s'",
				dataVolume.Name, snapshotClass.Name)
			return snapshotClass.Name
		}
	}

	klog.V(3).Infof("Could not match snapshotter with storage class, falling back to host assisted clone")
	return ""
}

func newSnapshot(dataVolume *cdiv1.DataVolume, snapshotClassName string) *csisnapshotv1.VolumeSnapshot {
	annotations := make(map[string]string)
	annotations[AnnSmartCloneRequest] = "true"
	className := snapshotClassName
	labels := map[string]string{
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	snapshot := &csisnapshotv1.VolumeSnapshot{
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: csisnapshotv1.SchemeGroupVersion.String(),
			Kind:       "VolumeSnapshot",
		},
		Status: csisnapshotv1.VolumeSnapshotStatus{},
		Spec: csisnapshotv1.VolumeSnapshotSpec{
			Source: &corev1.TypedLocalObjectReference{
				Name: dataVolume.Spec.Source.PVC.Name,
				Kind: "PersistentVolumeClaim",
			},
			VolumeSnapshotClassName: &className,
		},
	}
	return snapshot
}

func (c *DataVolumeController) updateImportStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
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

func (c *DataVolumeController) updateSmartCloneStatusPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume) error {
	var dataVolumeCopy = dataVolume.DeepCopy()
	var event DataVolumeEvent

	switch phase {
	case cdiv1.SnapshotForSmartCloneInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.SnapshotForSmartCloneInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = SnapshotForSmartCloneInProgress
		event.message = fmt.Sprintf(MessageSmartCloneInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name)
	}

	return c.emitEvent(dataVolume, dataVolumeCopy, &event)
}

func (c *DataVolumeController) updateCloneStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
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

func (c *DataVolumeController) updateUploadStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
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
			// TODO: Use a more generic In Progess, like maybe TransferInProgress.
			dataVolumeCopy.Status.Phase = cdiv1.UploadReady
			event.eventType = corev1.EventTypeNormal
			event.reason = UploadReady
			event.message = fmt.Sprintf(MessageUploadReady, pvc.Name)
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

func (c *DataVolumeController) updateDataVolumeStatus(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	dataVolumeCopy := dataVolume.DeepCopy()
	var event DataVolumeEvent

	curPhase := dataVolumeCopy.Status.Phase
	if pvc == nil {
		if curPhase != cdiv1.PhaseUnset && curPhase != cdiv1.Pending && curPhase != cdiv1.SnapshotForSmartCloneInProgress {

			// if pvc doesn't exist and we're not still initializing, then
			// something has gone wrong. Perhaps the PVC was deleted out from
			// underneath the DataVolume
			dataVolumeCopy.Status.Phase = cdiv1.Failed
			event.eventType = corev1.EventTypeWarning
			event.reason = DataVolumeFailed
			event.message = fmt.Sprintf(MessageResourceDoesntExist, dataVolume.Name)
		}

	} else {

		switch pvc.Status.Phase {
		case corev1.ClaimPending:
			dataVolumeCopy.Status.Phase = cdiv1.Pending
			// the following check is for a case where the request is to create a blank disk for a block device.
			// in that case, we do not create a pod as there is no need to create a blank image.
			// instead, we just mark the DV phase as 'Succeeded' so any consumer will be able to use it.
			phase, _ := pvc.Annotations[AnnPodPhase]
			if phase == string(cdiv1.Succeeded) {
				dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				c.updateImportStatusPhase(pvc, dataVolumeCopy, &event)
			}
		case corev1.ClaimBound:
			switch dataVolumeCopy.Status.Phase {
			case cdiv1.Pending:
				dataVolumeCopy.Status.Phase = cdiv1.PVCBound
			case cdiv1.Unknown:
				dataVolumeCopy.Status.Phase = cdiv1.PVCBound
			}

			_, ok := pvc.Annotations[AnnImportPod]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.ImportScheduled
				c.updateImportStatusPhase(pvc, dataVolumeCopy, &event)
			}
			_, ok = pvc.Annotations[AnnCloneRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
				c.updateCloneStatusPhase(pvc, dataVolumeCopy, &event)
			}
			_, ok = pvc.Annotations[AnnUploadRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
				c.updateUploadStatusPhase(pvc, dataVolumeCopy, &event)
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

	return c.emitEvent(dataVolume, dataVolumeCopy, &event)
}

func (c *DataVolumeController) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) error {
	// Only update the object if something actually changed in the status.
	if !reflect.DeepEqual(dataVolume.Status, dataVolumeCopy.Status) {
		_, err := c.cdiClientSet.CdiV1alpha1().DataVolumes(dataVolume.Namespace).Update(dataVolumeCopy)
		// Emit the event only when the status change happens, not every time
		if event.eventType != "" {
			c.recorder.Event(dataVolume, event.eventType, event.reason, event.message)
		}
		return err
	}
	return nil
}

// getPodFromPvc determines the pod associated with the pvc UID passed in.
func (c *DataVolumeController) getPodFromPvc(namespace string, pvcUID types.UID) (*corev1.Pod, error) {
	l, _ := labels.Parse(common.PrometheusLabel)
	pods, err := c.kubeclientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: l.String()})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		for _, or := range pod.OwnerReferences {
			if or.UID == pvcUID {
				return &pod, nil
			}
		}

		val, exists := pod.Labels[CloneUniqueID]
		if exists && val == string(pvcUID)+"-source-pod" {
			return &pod, nil
		}
	}
	return nil, errors.Errorf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvcUID), namespace)
}

func (c *DataVolumeController) updateProgressUsingPod(dataVolumeCopy *cdiv1.DataVolume, pod *corev1.Pod) {
	httpClient := buildHTTPClient()
	// Example value: import_progress{ownerUID="b856691e-1038-11e9-a5ab-525500d15501"} 13.45
	var importRegExp = regexp.MustCompile("progress\\{ownerUID\\=\"" + string(dataVolumeCopy.UID) + "\"\\} (\\d{1,3}\\.?\\d*)")

	port, err := c.getPodMetricsPort(pod)
	if err == nil {
		url := fmt.Sprintf("https://%s:%d/metrics", pod.Status.PodIP, port)
		klog.V(3).Info("Connecting to URL: " + url)
		resp, err := httpClient.Get(url)
		if err != nil {
			klog.Errorf("%+v", err)
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}

		match := importRegExp.FindStringSubmatch(string(body))
		if match == nil {
			klog.V(3).Info("No match found")
			// No match
			return
		}
		if f, err := strconv.ParseFloat(match[1], 64); err == nil {
			klog.V(3).Info("Setting progress to: " + match[1])
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress(fmt.Sprintf("%.2f%%", f))
		}
	}
}

func (c *DataVolumeController) getPodMetricsPort(pod *corev1.Pod) (int, error) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "metrics" {
				return int(port.ContainerPort), nil
			}
		}
	}
	klog.V(3).Infof("Unable to find metrics port on pod: %s", pod.Name)
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

// enqueueDataVolume takes a DataVolume resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than DataVolume.
func (c *DataVolumeController) enqueueDataVolume(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *DataVolumeController) handleAddObject(obj interface{}) {
	c.handleObject(obj, "add")
}
func (c *DataVolumeController) handleUpdateObject(obj interface{}) {
	c.handleObject(obj, "update")
}
func (c *DataVolumeController) handleDeleteObject(obj interface{}) {
	c.handleObject(obj, "delete")
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the DataVolume resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that DataVolume resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *DataVolumeController) handleObject(obj interface{}, verb string) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(3).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(3).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a DataVolume, we should not do anything more
		// with it.
		if ownerRef.Kind != "DataVolume" {
			return
		}

		dataVolume, err := c.dataVolumesLister.DataVolumes(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(3).Infof("ignoring orphaned object '%s' of datavolume '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		if verb == "add" {
			dataVolumeKey, err := cache.MetaNamespaceKeyFunc(dataVolume)
			if err != nil {
				runtime.HandleError(err)
				return
			}

			c.pvcExpectations.CreationObserved(dataVolumeKey)
		}
		c.enqueueDataVolume(dataVolume)
		return
	}
}

// newPersistentVolumeClaim creates a new PVC the DataVolume resource.
// It also sets the appropriate OwnerReferences on the resource
// which allows handleObject to discover the DataVolume resource
// that 'owns' it.
func newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		"cdi-controller": dataVolume.Name,
		"app":            "containerized-data-importer",
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
