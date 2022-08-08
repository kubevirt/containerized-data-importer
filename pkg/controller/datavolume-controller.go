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
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// SuccessSynced provides a const to represent a Synced status
	SuccessSynced = "Synced"
	// ErrResourceExists provides a const to indicate a resource exists error
	ErrResourceExists = "ErrResourceExists"
	// ErrResourceMarkedForDeletion provides a const to indicate a resource marked for deletion error
	ErrResourceMarkedForDeletion = "ErrResourceMarkedForDeletion"
	// ErrResourceDoesntExist provides a const to indicate a resource doesn't exist error
	ErrResourceDoesntExist = "ErrResourceDoesntExist"
	// ErrClaimLost provides a const to indicate a claim is lost
	ErrClaimLost = "ErrClaimLost"
	// ErrClaimNotValid provides a const to indicate a claim is not valid
	ErrClaimNotValid = "ErrClaimNotValid"
	// ErrExceededQuota provides a const to indicate the claim has exceeded the quota
	ErrExceededQuota = "ErrExceededQuota"
	// ErrUnableToClone provides a const to indicate some errors are blocking the clone
	ErrUnableToClone = "ErrUnableToClone"
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
	// ImportPaused provides a const to indicate that a multistage import is waiting for the next stage
	ImportPaused = "ImportPaused"
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
	// SmartCloneSourceInUse provides a const to indicate a smart clone is being delayed because the source is in use
	SmartCloneSourceInUse = "SmartCloneSourceInUse"
	// CSICloneInProgress provides a const to indicate  csi volume clone is in progress
	CSICloneInProgress = "CSICloneInProgress"
	// CSICloneSourceInUse provides a const to indicate a csi volume clone is being delayed because the source is in use
	CSICloneSourceInUse = "CSICloneSourceInUse"
	// HostAssistedCloneSourceInUse provides a const to indicate a host-assisted clone is being delayed because the source is in use
	HostAssistedCloneSourceInUse = "HostAssistedCloneSourceInUse"
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
	// MessageResourceMarkedForDeletion provides a const to form a resource marked for deletion error message
	MessageResourceMarkedForDeletion = "Resource %q marked for deletion"
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
	// MessageImportPaused provides a const for a "multistage import paused" message
	MessageImportPaused = "Multistage import into PVC %s is paused"
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
	// MessageCsiCloneInProgress provides a const to form a CSI Volume Clone in progress message
	MessageCsiCloneInProgress = "CSI Volume clone in progress (for pvc %s/%s)"
	// MessageUploadScheduled provides a const to form upload is scheduled message
	MessageUploadScheduled = "Upload into %s scheduled"
	// MessageUploadReady provides a const to form upload is ready message
	MessageUploadReady = "Upload into %s ready"
	// MessageUploadFailed provides a const to form upload has failed message
	MessageUploadFailed = "Upload into %s failed"
	// MessageUploadSucceeded provides a const to form upload has succeeded message
	MessageUploadSucceeded = "Successfully uploaded into %s"
	// MessageSizeDetectionPodFailed provides a const to indicate that the size-detection pod wasn't able to obtain the image size
	MessageSizeDetectionPodFailed = "Size-detection pod failed due to %s"
	// ExpansionInProgress is const representing target PVC expansion
	ExpansionInProgress = "ExpansionInProgress"
	// MessageExpansionInProgress is a const for reporting target expansion
	MessageExpansionInProgress = "Expanding PersistentVolumeClaim for DataVolume %s/%s"
	// NamespaceTransferInProgress is const representing target PVC transfer
	NamespaceTransferInProgress = "NamespaceTransferInProgress"
	// MessageNamespaceTransferInProgress is a const for reporting target transfer
	MessageNamespaceTransferInProgress = "Transferring PersistentVolumeClaim for DataVolume %s/%s"
	// SizeDetectionPodCreated provides a const to indicate that the size-detection pod has been created (reason)
	SizeDetectionPodCreated = "SizeDetectionPodCreated"
	// MessageSizeDetectionPodCreated provides a const to indicate that the size-detection pod has been created (message)
	MessageSizeDetectionPodCreated = "Size-detection pod created"
	// SizeDetectionPodNotReady reports that the size-detection pod has not finished its exectuion (reason)
	SizeDetectionPodNotReady = "SizeDetectionPodNotReady"
	// MessageSizeDetectionPodNotReady reports that the size-detection pod has not finished its exectuion (message)
	MessageSizeDetectionPodNotReady = "The size detection pod is not finished yet"
	// ImportPVCNotReady reports that it's not yet possible to access the source PVC (reason)
	ImportPVCNotReady = "ImportPVCNotReady"
	// MessageImportPVCNotReady reports that it's not yet possible to access the source PVC (message)
	MessageImportPVCNotReady = "The source PVC is not fully imported"
	// CloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (reason)
	CloneValidationFailed = "CloneValidationFailed"
	// MessageCloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (message)
	MessageCloneValidationFailed = "The clone doesn't meet the validation requirements"
	// CloneWithoutSource reports that the source PVC of a clone doesn't exists (reason)
	CloneWithoutSource = "CloneWithoutSource"
	// MessageCloneWithoutSource reports that the source PVC of a clone doesn't exists (message)
	MessageCloneWithoutSource = "The source PVC %s doesn't exist"

	// AnnCSICloneRequest annotation associates object with CSI Clone Request
	AnnCSICloneRequest = "cdi.kubevirt.io/CSICloneRequest"

	// AnnVirtualImageSize annotation contains the Virtual Image size of a PVC used for host-assisted cloning
	AnnVirtualImageSize = "cdi.Kubervirt.io/virtualSize"

	// AnnSourceCapacity annotation contains the storage capacity of a PVC used for host-assisted cloning
	AnnSourceCapacity = "cdi.Kubervirt.io/sourceCapacity"

	// AnnPermissiveClone annotation allows the clone-controller to skip the clone size validation
	AnnPermissiveClone = "cdi.Kubevirt.io/permissiveClone"

	annOwnedByDataVolume = "cdi.kubevirt.io/ownedByDataVolume"

	annOwnerUID = "cdi.kubevirt.io/ownerUID"

	crossNamespaceFinalizer = "cdi.kubevirt.io/dataVolumeFinalizer"

	annReadyForTransfer = "cdi.kubevirt.io/readyForTransfer"

	annCloneType = "cdi.kubevirt.io/cloneType"

	dvPhaseField = "status.phase"
)

type cloneStrategy int

// Possible clone strategies, including default special value NoClone
const (
	NoClone cloneStrategy = iota
	HostAssistedClone
	SmartClone
	CsiClone
)

// Size-detection pod error codes
const (
	NoErr int = iota
	ErrBadArguments
	ErrInvalidFile
	ErrInvalidPath
	ErrBadTermFile
	ErrUnknown
)

var httpClient *http.Client

// DataVolumeEvent reoresents event
type DataVolumeEvent struct {
	eventType string
	reason    string
	message   string
}

// ErrInvalidTermMsg reports that the termination message from the size-detection pod doesn't exists or is not a valid quantity
var ErrInvalidTermMsg = fmt.Errorf("The termination message from the size-detection pod is not-valid")

// DatavolumeReconciler members
type DatavolumeReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	featureGates    featuregates.FeatureGates
	clonerImage     string
	importerImage   string
	pullPolicy      string
	tokenValidator  token.Validator
	tokenGenerator  token.Generator
	installerLabels map[string]string
	sccs            controllerStarter
}

type controllerStarter interface {
	Start(ctx context.Context) error
	StartController()
}
type smartCloneControllerStarter struct {
	log                       logr.Logger
	installerLabels           map[string]string
	startSmartCloneController chan struct{}
	mgr                       manager.Manager
}

func (sccs *smartCloneControllerStarter) Start(ctx context.Context) error {
	started := false
	for {
		select {
		case <-sccs.startSmartCloneController:
			if !started {
				sccs.log.Info("Starting smart clone controller as CSI snapshot CRDs are detected")
				if _, err := NewSmartCloneController(sccs.mgr, sccs.log, sccs.installerLabels); err != nil {
					sccs.log.Error(err, "Unable to setup smart clone controller: %v")
				} else {
					started = true
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (sccs *smartCloneControllerStarter) StartController() {
	sccs.startSmartCloneController <- struct{}{}
}

func hasAnnOwnedByDataVolume(obj metav1.Object) bool {
	_, ok := obj.GetAnnotations()[annOwnedByDataVolume]
	return ok
}

func getAnnOwnedByDataVolume(obj metav1.Object) (string, string, error) {
	val := obj.GetAnnotations()[annOwnedByDataVolume]
	return cache.SplitMetaNamespaceKey(val)
}

func setAnnOwnedByDataVolume(dest, obj metav1.Object) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	if dest.GetAnnotations() == nil {
		dest.SetAnnotations(make(map[string]string))
	}
	dest.GetAnnotations()[annOwnedByDataVolume] = key

	return nil
}

func pvcIsPopulated(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) bool {
	dvName, ok := pvc.Annotations[AnnPopulatedFor]
	return ok && dvName == dv.Name
}

// GetDataVolumeClaimName returns the PVC name associated with the DV
func GetDataVolumeClaimName(dv *cdiv1.DataVolume) string {
	pvcName, ok := dv.Annotations[AnnPrePopulated]
	if ok {
		return pvcName
	}

	return dv.Name
}

// NewDatavolumeController creates a new instance of the datavolume controller.
func NewDatavolumeController(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	clonerImage, importerImage, pullPolicy string,
	tokenPublicKey *rsa.PublicKey,
	tokenPrivateKey *rsa.PrivateKey,
	installerLabels map[string]string,
) (controller.Controller, error) {

	client := mgr.GetClient()
	sccs := &smartCloneControllerStarter{
		log:                       log,
		installerLabels:           installerLabels,
		startSmartCloneController: make(chan struct{}, 1),
		mgr:                       mgr,
	}
	reconciler := &DatavolumeReconciler{
		client:         client,
		scheme:         mgr.GetScheme(),
		log:            log.WithName("datavolume-controller"),
		recorder:       mgr.GetEventRecorderFor("datavolume-controller"),
		featureGates:   featuregates.NewFeatureGates(client),
		clonerImage:    clonerImage,
		importerImage:  importerImage,
		pullPolicy:     pullPolicy,
		tokenValidator: newCloneTokenValidator(common.CloneTokenIssuer, tokenPublicKey),
		// for long term tokens to handle cross namespace dumb clones
		tokenGenerator:  newLongTermCloneTokenGenerator(tokenPrivateKey),
		installerLabels: installerLabels,
		sccs:            sccs,
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

	mgr.Add(sccs)
	return datavolumeController, nil
}

func addDatavolumeControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	// Setup watches
	if err := datavolumeController.Watch(&source.Kind{Type: &cdiv1.DataVolume{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := datavolumeController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			var result []reconcile.Request
			owner := metav1.GetControllerOf(obj)
			if owner != nil && owner.Kind == "DataVolume" {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: obj.GetNamespace(),
						Name:      owner.Name,
					},
				})
			}
			populatedFor := obj.GetAnnotations()[AnnPopulatedFor]
			if populatedFor != "" {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: obj.GetNamespace(),
						Name:      populatedFor,
					},
				})
			}
			// it is okay if result contains the same entry twice, will be deduplicated by caller
			return result
		},
	)); err != nil {
		return err
	}
	if err := datavolumeController.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}); err != nil {
		return err
	}
	for _, k := range []client.Object{&corev1.PersistentVolumeClaim{}, &corev1.Pod{}, &cdiv1.ObjectTransfer{}} {
		if err := datavolumeController.Watch(&source.Kind{Type: k}, handler.EnqueueRequestsFromMapFunc(
			func(obj client.Object) []reconcile.Request {
				if hasAnnOwnedByDataVolume(obj) {
					namespace, name, err := getAnnOwnedByDataVolume(obj)
					if err != nil {
						return nil
					}
					return []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{
								Namespace: namespace,
								Name:      name,
							},
						},
					}
				}
				return nil
			}),
		); err != nil {
			return err
		}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, dvPhaseField, func(obj client.Object) []string {
		return []string{string(obj.(*cdiv1.DataVolume).Status.Phase)}
	}); err != nil {
		return err
	}

	// Watch for SC updates and reconcile the DVs waiting for default SC
	if err := datavolumeController.Watch(&source.Kind{Type: &storagev1.StorageClass{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) (reqs []reconcile.Request) {
			dvList := &cdiv1.DataVolumeList{}
			if err := mgr.GetClient().List(context.TODO(), dvList, client.MatchingFields{dvPhaseField: ""}); err != nil {
				return
			}
			for _, dv := range dvList.Items {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
			}
			return
		},
	),
	); err != nil {
		return err
	}

	// Watch to reconcile clones created without source
	if err := addCloneWithoutSourceWatch(mgr, datavolumeController); err != nil {
		return err
	}

	return nil
}

// addCloneWithoutSourceWatch reconciles clones created without source once the matching PVC is created
func addCloneWithoutSourceWatch(mgr manager.Manager, datavolumeController controller.Controller) error {
	const sourcePvcField = "spec.source.pvc"

	getKey := func(namespace, name string) string {
		return namespace + "/" + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, sourcePvcField, func(obj client.Object) []string {
		if obj.(*cdiv1.DataVolume).Spec.Source != nil {
			if pvc := obj.(*cdiv1.DataVolume).Spec.Source.PVC; pvc != nil {
				ns := getNamespace(pvc.Namespace, obj.GetNamespace())
				return []string{getKey(ns, pvc.Name)}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Function to reconcile DVs that match the selected fields
	dataVolumeMapper := func(obj client.Object) (reqs []reconcile.Request) {
		dvList := &cdiv1.DataVolumeList{}
		namespacedName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
		matchingFields := client.MatchingFields{sourcePvcField: namespacedName.String()}
		if err := mgr.GetClient().List(context.TODO(), dvList, matchingFields); err != nil {
			return
		}
		for _, dv := range dvList.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}})
		}
		return
	}

	if err := datavolumeController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
		handler.EnqueueRequestsFromMapFunc(dataVolumeMapper),
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return true },
			DeleteFunc: func(e event.DeleteEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool { return false },
		}); err != nil {
		return err
	}

	return nil
}

// Reconcile the reconcile loop for the data volumes.
func (r *DatavolumeReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("Datavolume", req.NamespacedName)

	// Get the Datavolume.
	datavolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, datavolume); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	transferName := fmt.Sprintf("cdi-tmp-%s", datavolume.UID)

	if datavolume.DeletionTimestamp != nil {
		log.Info("Datavolume marked for deletion, cleaning up")
		if err := r.cleanupTransfer(log, datavolume, transferName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if err := r.populateSourceIfSourceRef(datavolume); err != nil {
		return reconcile.Result{}, err
	}

	if isCrossNamespaceClone(datavolume) && datavolume.Status.Phase == cdiv1.Succeeded {
		if err := r.cleanupTransfer(log, datavolume, transferName); err != nil {
			return reconcile.Result{}, err
		}
	}

	pvcPopulated := false
	// Get the pvc with the name specified in DataVolume.spec
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: datavolume.Namespace, Name: datavolume.Name}, pvc); err != nil {
		// If the resource doesn't exist, we'll create it
		if k8serrors.IsNotFound(err) {
			pvc = nil
		} else if err != nil {
			return reconcile.Result{}, err
		}
	} else {
		res, err := r.garbageCollect(datavolume, pvc, log)
		if err != nil {
			return reconcile.Result{}, err
		}
		if res != nil {
			return *res, nil
		}

		// If the PVC is not controlled by this DataVolume resource, we should log
		// a warning to the event recorder and return
		pvcPopulated = pvcIsPopulated(pvc, datavolume)
		if !metav1.IsControlledBy(pvc, datavolume) {
			if pvcPopulated {
				if err := r.addOwnerRef(pvc, datavolume); err != nil {
					return reconcile.Result{}, err
				}
			} else {
				msg := fmt.Sprintf(MessageResourceExists, pvc.Name)
				r.recorder.Event(datavolume, corev1.EventTypeWarning, ErrResourceExists, msg)
				return reconcile.Result{}, errors.Errorf(msg)
			}
		}
		// If the PVC is being deleted, we should log a warning to the event recorder and return to wait the deletion complete
		if pvc.DeletionTimestamp != nil {
			msg := fmt.Sprintf(MessageResourceMarkedForDeletion, pvc.Name)
			r.recorder.Event(datavolume, corev1.EventTypeWarning, ErrResourceMarkedForDeletion, msg)
			return reconcile.Result{}, errors.Errorf(msg)
		}
	}

	pvcSpec, err := RenderPvcSpec(r.client, r.recorder, r.log, datavolume)
	if err != nil {
		return reconcile.Result{}, err
	}

	_, dvPrePopulated := datavolume.Annotations[AnnPrePopulated]

	if isClone := datavolume.Spec.Source.PVC != nil; isClone {
		return r.reconcileClone(log, datavolume, pvc, pvcSpec, transferName, dvPrePopulated, pvcPopulated)
	}

	if !dvPrePopulated {
		if pvc == nil {
			newPvc, err := r.createPvcForDatavolume(log, datavolume, pvcSpec)
			if err != nil {
				if errQuotaExceeded(err) {
					r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, datavolume, nil, NoClone,
						DataVolumeEvent{
							eventType: corev1.EventTypeWarning,
							reason:    ErrExceededQuota,
							message:   err.Error(),
						})
				}
				return reconcile.Result{}, err
			}
			pvc = newPvc
		} else {
			if getSource(pvc) == SourceVDDK {
				changed, err := r.getVddkAnnotations(datavolume, pvc)
				if err != nil {
					return reconcile.Result{}, err
				}
				if changed {
					err = r.client.Get(context.TODO(), req.NamespacedName, datavolume)
					if err != nil {
						return reconcile.Result{}, err
					}
				}
			}

			err = r.maybeSetMultiStageAnnotation(pvc, datavolume)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Finally, we update the status block of the DataVolume resource to reflect the
	// current state of the world
	return r.reconcileDataVolumeStatus(datavolume, pvc, NoClone)
}

func (r *DatavolumeReconciler) reconcileClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string,
	prePopulated bool,
	pvcPopulated bool) (reconcile.Result, error) {

	// Get the most appropiate clone strategy
	selectedCloneStrategy, err := r.selectCloneStrategy(datavolume, pvcSpec)
	if err != nil {
		return reconcile.Result{}, err
	}

	if pvcPopulated || prePopulated {
		return r.reconcileDataVolumeStatus(datavolume, pvc, selectedCloneStrategy)
	}

	// Check if source PVC exists and do proper validation before attempting to clone
	if done, err := r.validateCloneAndSourcePVC(datavolume); err != nil {
		return reconcile.Result{}, err
	} else if !done {
		return reconcile.Result{}, nil
	}

	if selectedCloneStrategy == SmartClone {
		r.sccs.StartController()
	}

	// If the target's size is not specified, we can extract that value from the source PVC
	targetRequest, hasTargetRequest := pvcSpec.Resources.Requests[corev1.ResourceStorage]
	if !hasTargetRequest || targetRequest.IsZero() {
		done, err := r.detectCloneSize(datavolume, pvc, pvcSpec, selectedCloneStrategy)
		if err != nil {
			return reconcile.Result{}, err
		} else if !done {
			// Check if the source PVC is ready to be cloned
			if readyToClone, err := r.isSourceReadyToClone(datavolume, selectedCloneStrategy); err != nil {
				return reconcile.Result{}, err
			} else if !readyToClone {
				return reconcile.Result{Requeue: true},
					r.updateCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil, selectedCloneStrategy)
			}
			return reconcile.Result{}, nil
		}
	}

	if pvc == nil {
		if selectedCloneStrategy == SmartClone {
			snapshotClassName, _ := r.getSnapshotClassForSmartClone(datavolume, pvcSpec)
			return r.reconcileSmartClonePvc(log, datavolume, pvcSpec, transferName, snapshotClassName)
		}
		if selectedCloneStrategy == CsiClone {
			csiDriverAvailable, err := r.storageClassCSIDriverExists(pvcSpec.StorageClassName)
			if err != nil && !k8serrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
			if !csiDriverAvailable {
				// err csi clone not possible
				return reconcile.Result{},
					r.updateDataVolumeStatusPhaseWithEvent(cdiv1.CloneScheduled, datavolume, pvc, selectedCloneStrategy,
						DataVolumeEvent{
							eventType: corev1.EventTypeWarning,
							reason:    ErrUnableToClone,
							message:   fmt.Sprintf("CSI Clone configured, but no CSIDriver available for %s", *pvcSpec.StorageClassName),
						})
			}

			return r.reconcileCsiClonePvc(log, datavolume, pvcSpec, transferName)
		}

		newPvc, err := r.createPvcForDatavolume(log, datavolume, pvcSpec)
		if err != nil {
			if errQuotaExceeded(err) {
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, datavolume, nil, selectedCloneStrategy,
					DataVolumeEvent{
						eventType: corev1.EventTypeWarning,
						reason:    ErrExceededQuota,
						message:   err.Error(),
					})
			}
			return reconcile.Result{}, err
		}
		pvc = newPvc
	}

	shouldBeMarkedWaitForFirstConsumer, err := r.shouldBeMarkedWaitForFirstConsumer(pvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	switch selectedCloneStrategy {
	case HostAssistedClone:
		if err := r.ensureExtendedToken(pvc); err != nil {
			return reconcile.Result{}, err
		}
	case CsiClone:
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if err := r.setCloneOfOnPvc(pvc); err != nil {
				return reconcile.Result{}, err
			}
		case corev1.ClaimPending:
			r.log.V(3).Info("ClaimPending CSIClone")
			if !shouldBeMarkedWaitForFirstConsumer {
				return reconcile.Result{}, r.updateCloneStatusPhase(cdiv1.CSICloneInProgress, datavolume, pvc, selectedCloneStrategy)
			}
		case corev1.ClaimLost:
			return reconcile.Result{},
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Failed, datavolume, pvc, selectedCloneStrategy,
					DataVolumeEvent{
						eventType: corev1.EventTypeWarning,
						reason:    ErrClaimLost,
						message:   fmt.Sprintf(MessageErrClaimLost, pvc.Name),
					})
		}
		fallthrough
	case SmartClone:
		if !shouldBeMarkedWaitForFirstConsumer {
			return r.finishClone(log, datavolume, pvc, pvcSpec, transferName, selectedCloneStrategy)
		}
	}

	// Finally, we update the status block of the DataVolume resource to reflect the
	// current state of the world
	return r.reconcileDataVolumeStatus(datavolume, pvc, selectedCloneStrategy)
}

func (r *DatavolumeReconciler) ensureExtendedToken(pvc *corev1.PersistentVolumeClaim) error {
	_, ok := pvc.Annotations[AnnExtendedCloneToken]
	if ok {
		return nil
	}

	token, ok := pvc.Annotations[AnnCloneToken]
	if !ok {
		return fmt.Errorf("token missing")
	}

	payload, err := r.tokenValidator.Validate(token)
	if err != nil {
		return err
	}

	if payload.Params == nil {
		payload.Params = make(map[string]string)
	}
	payload.Params["uid"] = string(pvc.UID)

	newToken, err := r.tokenGenerator.Generate(payload)
	if err != nil {
		return err
	}

	pvc.Annotations[AnnExtendedCloneToken] = newToken

	if err := r.updatePVC(pvc); err != nil {
		return err
	}

	return nil
}

func (r *DatavolumeReconciler) selectCloneStrategy(datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec) (cloneStrategy, error) {
	preferredCloneStrategy, err := r.getCloneStrategy(datavolume)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return NoClone, nil
		}
		return NoClone, err
	}

	bindingMode, err := r.getStorageClassBindingMode(pvcSpec.StorageClassName)
	if err != nil {
		return NoClone, err
	}

	if preferredCloneStrategy != nil && *preferredCloneStrategy == cdiv1.CloneStrategyCsiClone {
		csiClonePossible, err := r.advancedClonePossible(datavolume, pvcSpec)
		if err != nil {
			return NoClone, err
		}

		if csiClonePossible &&
			(!isCrossNamespaceClone(datavolume) || *bindingMode == storagev1.VolumeBindingImmediate) {
			return CsiClone, nil
		}
	} else if preferredCloneStrategy != nil && *preferredCloneStrategy == cdiv1.CloneStrategySnapshot {
		snapshotClassName, err := r.getSnapshotClassForSmartClone(datavolume, pvcSpec)
		if err != nil {
			return NoClone, err
		}
		snapshotClassAvailable := snapshotClassName != ""

		snapshotPossible, err := r.advancedClonePossible(datavolume, pvcSpec)
		if err != nil {
			return NoClone, err
		}

		if snapshotClassAvailable && snapshotPossible &&
			(!isCrossNamespaceClone(datavolume) || *bindingMode == storagev1.VolumeBindingImmediate) {
			return SmartClone, nil
		}
	}

	return HostAssistedClone, nil
}

func (r *DatavolumeReconciler) createPvcForDatavolume(log logr.Logger, datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaim, error) {
	log.Info("Creating PVC for datavolume")
	newPvc, err := r.newPersistentVolumeClaim(datavolume, pvcSpec, datavolume.Namespace, datavolume.Name)
	if err != nil {
		return nil, err
	}
	util.SetRecommendedLabels(newPvc, r.installerLabels, "cdi-controller")

	checkpoint := r.getNextCheckpoint(datavolume, newPvc)
	if checkpoint != nil { // Initialize new warm import annotations before creating PVC
		newPvc.ObjectMeta.Annotations[AnnCurrentCheckpoint] = checkpoint.Current
		newPvc.ObjectMeta.Annotations[AnnPreviousCheckpoint] = checkpoint.Previous
		newPvc.ObjectMeta.Annotations[AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)
	}
	if err := r.client.Create(context.TODO(), newPvc); err != nil {
		return nil, err
	}

	return newPvc, nil
}

func (r *DatavolumeReconciler) reconcileCsiClonePvc(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string) (reconcile.Result, error) {

	log = log.WithName("reconcileCsiClonePvc")
	pvcName := datavolume.Name

	if isCrossNamespaceClone(datavolume) {
		pvcName = transferName

		result, err := r.doCrossNamespaceClone(log, datavolume, pvcSpec, pvcName, false, CsiClone)
		if result != nil {
			return *result, err
		}
	}

	if datavolume.Status.Phase == cdiv1.NamespaceTransferInProgress {
		return reconcile.Result{}, nil
	}

	sourcePvcNs := datavolume.Spec.Source.PVC.Namespace
	if sourcePvcNs == "" {
		sourcePvcNs = datavolume.Namespace
	}
	log.V(3).Info("CSI-Clone is available")

	// Get source pvc
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: sourcePvcNs, Name: datavolume.Spec.Source.PVC.Name}, sourcePvc); err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(3).Info("Source PVC no longer exists")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	// Check if the source PVC is ready to be cloned
	if readyToClone, err := r.isSourceReadyToClone(datavolume, CsiClone); err != nil {
		return reconcile.Result{}, err
	} else if !readyToClone {
		return reconcile.Result{Requeue: true},
			r.updateCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil, CsiClone)
	}

	log.Info("Creating PVC for datavolume")
	cloneTargetPvc, err := r.newVolumeClonePVC(datavolume, sourcePvc, pvcSpec, pvcName)
	if err != nil {
		return reconcile.Result{}, err
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: cloneTargetPvc.Namespace, Name: cloneTargetPvc.Name}, pvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		if err := r.client.Create(context.TODO(), cloneTargetPvc); err != nil && !k8serrors.IsAlreadyExists(err) {
			if errQuotaExceeded(err) {
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, datavolume, nil, CsiClone,
					DataVolumeEvent{
						eventType: corev1.EventTypeWarning,
						reason:    ErrExceededQuota,
						message:   err.Error(),
					})
			}
			return reconcile.Result{}, err
		}
	} else {
		// PVC already exists, check for name clash
		pvcControllerRef := metav1.GetControllerOf(cloneTargetPvc)
		pvcClashControllerRef := metav1.GetControllerOf(pvc)

		if pvc.Name == cloneTargetPvc.Name &&
			pvc.Namespace == cloneTargetPvc.Namespace &&
			!reflect.DeepEqual(pvcControllerRef, pvcClashControllerRef) {
			return reconcile.Result{}, errors.Errorf("Target Pvc Name in use")
		}

		if pvc.Status.Phase == corev1.ClaimBound {
			if err := r.setCloneOfOnPvc(pvc); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, r.updateCloneStatusPhase(cdiv1.CSICloneInProgress, datavolume, nil, CsiClone)
}

// When the clone is finished some additional actions may be applied
// like namespaceTransfer Cleanup or size expansion
func (r *DatavolumeReconciler) finishClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	//DO Nothing, not yet ready
	if pvc.Annotations[AnnCloneOf] != "true" {
		return reconcile.Result{}, nil
	}

	// expand for non-namespace case
	return r.expandPvcAfterClone(log, datavolume, pvc, pvcSpec, selectedCloneStrategy)
}

func (r *DatavolumeReconciler) setCloneOfOnPvc(pvc *corev1.PersistentVolumeClaim) error {
	if v, ok := pvc.Annotations[AnnCloneOf]; !ok || v != "true" {
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations[AnnCloneOf] = "true"

		return r.updatePVC(pvc)
	}

	return nil
}

func (r *DatavolumeReconciler) expandPvcAfterClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	return r.expandPvcAfterCloneFunc(log, datavolume, pvc, pvcSpec, false, selectedCloneStrategy, cdiv1.Succeeded)
}

// temporary pvc is used when the clone src and tgt are in two distinct namespaces
func (r *DatavolumeReconciler) expandTmpPvcAfterClone(
	log logr.Logger,
	datavolume *cdiv1.DataVolume,
	tmpPVC *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	return r.expandPvcAfterCloneFunc(log, datavolume, tmpPVC, pvcSpec, true, selectedCloneStrategy, cdiv1.NamespaceTransferInProgress)
}

func (r *DatavolumeReconciler) expandPvcAfterCloneFunc(
	log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	isTemp bool,
	selectedCloneStrategy cloneStrategy,
	nextPhase cdiv1.DataVolumePhase) (reconcile.Result, error) {

	done, err := r.expand(log, datavolume, pvc, pvcSpec)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !done {
		return reconcile.Result{Requeue: true},
			r.updateCloneStatusPhase(cdiv1.ExpansionInProgress, datavolume, pvc, selectedCloneStrategy)
	}

	if isTemp {
		// trigger transfer and next reconcile should have pvcExists == true
		pvc.Annotations[annReadyForTransfer] = "true"
		pvc.Annotations[AnnPopulatedFor] = datavolume.Name
		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, r.updateCloneStatusPhase(nextPhase, datavolume, pvc, selectedCloneStrategy)
}

func cloneStrategyToCloneType(selectedCloneStrategy cloneStrategy) string {
	switch selectedCloneStrategy {
	case SmartClone:
		return "snapshot"
	case CsiClone:
		return "csivolumeclone"
	case HostAssistedClone:
		return "network"
	}
	return ""
}

func (r *DatavolumeReconciler) reconcileSmartClonePvc(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string,
	snapshotClassName string) (reconcile.Result, error) {

	pvcName := datavolume.Name

	if isCrossNamespaceClone(datavolume) {
		pvcName = transferName
		result, err := r.doCrossNamespaceClone(log, datavolume, pvcSpec, pvcName, true, SmartClone)
		if result != nil {
			return *result, err
		}
	}

	if datavolume.Status.Phase == cdiv1.NamespaceTransferInProgress {
		return reconcile.Result{}, nil
	}

	r.log.V(3).Info("Smart-Clone via Snapshot is available with Volume Snapshot Class",
		"snapshotClassName", snapshotClassName)

	newSnapshot := newSnapshot(datavolume, pvcName, snapshotClassName)
	util.SetRecommendedLabels(newSnapshot, r.installerLabels, "cdi-controller")

	if err := setAnnOwnedByDataVolume(newSnapshot, datavolume); err != nil {
		return reconcile.Result{}, err
	}

	nn := client.ObjectKeyFromObject(newSnapshot)
	if err := r.client.Get(context.TODO(), nn, newSnapshot.DeepCopy()); err != nil {
		if !k8serrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		// Check if the source PVC is ready to be cloned
		if readyToClone, err := r.isSourceReadyToClone(datavolume, SmartClone); err != nil {
			return reconcile.Result{}, err
		} else if !readyToClone {
			return reconcile.Result{Requeue: true},
				r.updateCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil, SmartClone)
		}

		targetPvc := &corev1.PersistentVolumeClaim{}
		if err := r.client.Get(context.TODO(), nn, targetPvc); err != nil {
			if !k8serrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			if err := r.client.Create(context.TODO(), newSnapshot); err != nil {
				if !k8serrors.IsAlreadyExists(err) {
					return reconcile.Result{}, err
				}
			} else {
				r.log.V(1).Info("snapshot created successfully", "snapshot.Namespace", newSnapshot.Namespace, "snapshot.Name", newSnapshot.Name)
			}
		}
	}

	return reconcile.Result{}, r.updateCloneStatusPhase(cdiv1.SnapshotForSmartCloneInProgress, datavolume, nil, SmartClone)
}

func (r *DatavolumeReconciler) doCrossNamespaceClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	pvcName string,
	returnWhenCloneInProgress bool,
	selectedCloneStrategy cloneStrategy) (*reconcile.Result, error) {

	initialized, err := r.initTransfer(log, datavolume, pvcName)
	if err != nil {
		return &reconcile.Result{}, err
	}

	// get reconciled again v soon
	if !initialized {
		return &reconcile.Result{}, r.updateCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil, selectedCloneStrategy)
	}

	tmpPVC := &corev1.PersistentVolumeClaim{}
	nn := types.NamespacedName{Namespace: datavolume.Spec.Source.PVC.Namespace, Name: pvcName}
	if err := r.client.Get(context.TODO(), nn, tmpPVC); err != nil {
		if !k8serrors.IsNotFound(err) {
			return &reconcile.Result{}, err
		}
	} else if tmpPVC.Annotations[AnnCloneOf] == "true" {
		result, err := r.expandTmpPvcAfterClone(log, datavolume, tmpPVC, pvcSpec, selectedCloneStrategy)
		return &result, err
	} else {
		// AnnCloneOf != true, so cloneInProgress
		if returnWhenCloneInProgress {
			return &reconcile.Result{}, nil
		}
	}

	return nil, nil
}

func (r *DatavolumeReconciler) getVddkAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	var dataVolumeCopy = dataVolume.DeepCopy()
	if vddkHost := pvc.Annotations[AnnVddkHostConnection]; vddkHost != "" {
		AddAnnotation(dataVolumeCopy, AnnVddkHostConnection, vddkHost)
	}
	if vddkVersion := pvc.Annotations[AnnVddkVersion]; vddkVersion != "" {
		AddAnnotation(dataVolumeCopy, AnnVddkVersion, vddkVersion)
	}

	// only update if something has changed
	if !reflect.DeepEqual(dataVolume, dataVolumeCopy) {
		return true, r.updateDataVolume(dataVolumeCopy)
	}
	return false, nil
}

// Sets the annotation if pvc needs it, and does not have it yet
func (r *DatavolumeReconciler) maybeSetMultiStageAnnotation(pvc *corev1.PersistentVolumeClaim, datavolume *cdiv1.DataVolume) error {
	if pvc.Status.Phase == corev1.ClaimBound {
		// If a PVC already exists with no multi-stage annotations, check if it
		// needs them set (if not already finished with an import).
		multiStageImport := (len(datavolume.Spec.Checkpoints) > 0)
		multiStageAnnotationsSet := metav1.HasAnnotation(pvc.ObjectMeta, AnnCurrentCheckpoint)
		multiStageAlreadyDone := metav1.HasAnnotation(pvc.ObjectMeta, AnnMultiStageImportDone)
		if multiStageImport && !multiStageAnnotationsSet && !multiStageAlreadyDone {
			err := r.setMultistageImportAnnotations(datavolume, pvc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Set the PVC annotations related to multi-stage imports so that they point to the next checkpoint to copy.
func (r *DatavolumeReconciler) setMultistageImportAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	pvcCopy := pvc.DeepCopy()

	// Only mark this checkpoint complete if it was completed by the current pod.
	// This keeps us from skipping over checkpoints when a reconcile fails at a bad time.
	uuidAlreadyUsed := false
	for key, value := range pvcCopy.Annotations {
		if strings.HasPrefix(key, r.getCheckpointCopiedKey("")) { // Blank checkpoint name to get just the prefix
			if value == pvcCopy.Annotations[AnnCurrentPodID] {
				uuidAlreadyUsed = true
				break
			}
		}
	}
	if !uuidAlreadyUsed {
		// Mark checkpoint complete by saving UID of current pod to a
		// PVC annotation specific to this checkpoint.
		currentCheckpoint := pvcCopy.Annotations[AnnCurrentCheckpoint]
		if currentCheckpoint != "" {
			currentPodID := pvcCopy.Annotations[AnnCurrentPodID]
			annotation := r.getCheckpointCopiedKey(currentCheckpoint)
			pvcCopy.ObjectMeta.Annotations[annotation] = currentPodID
			r.log.V(1).Info("UUID not already used, marking checkpoint completed by current pod ID.", "checkpoint", currentCheckpoint, "podId", currentPodID)
		} else {
			r.log.Info("Cannot mark empty checkpoint complete. Check DataVolume spec for empty checkpoints.")
		}
	}
	// else: If the UID was already used for another transfer, then we are
	// just waiting for a new pod to start up to transfer the next checkpoint.

	// Set multi-stage PVC annotations so further reconcile loops will create new pods as needed.
	checkpoint := r.getNextCheckpoint(dataVolume, pvcCopy)
	if checkpoint != nil { // Only move to the next checkpoint if there is a next checkpoint to move to
		pvcCopy.ObjectMeta.Annotations[AnnCurrentCheckpoint] = checkpoint.Current
		pvcCopy.ObjectMeta.Annotations[AnnPreviousCheckpoint] = checkpoint.Previous
		pvcCopy.ObjectMeta.Annotations[AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)

		// Check to see if there is a running pod for this PVC. If there are
		// more checkpoints to copy but the PVC is stopped in Succeeded,
		// reset the phase to get another pod started for the next checkpoint.
		var podNamespace string
		if dataVolume.Spec.Source.PVC != nil {
			podNamespace = dataVolume.Spec.Source.PVC.Namespace
		} else {
			podNamespace = dataVolume.Namespace
		}
		phase := pvcCopy.ObjectMeta.Annotations[AnnPodPhase]
		pod, _ := r.getPodFromPvc(podNamespace, pvcCopy)
		if pod == nil && phase == string(corev1.PodSucceeded) {
			// Reset PVC phase so importer will create a new pod
			pvcCopy.ObjectMeta.Annotations[AnnPodPhase] = string(corev1.PodUnknown)
			delete(pvcCopy.ObjectMeta.Annotations, AnnImportPod)
		}
		// else: There's a pod already running, no need to try to start a new one.
	}
	// else: There aren't any checkpoints ready to be copied over.

	// only update if something has changed
	if !reflect.DeepEqual(pvc, pvcCopy) {
		return r.updatePVC(pvcCopy)
	}
	return nil
}

// Clean up PVC annotations after a multi-stage import.
func (r *DatavolumeReconciler) deleteMultistageImportAnnotations(pvc *corev1.PersistentVolumeClaim) error {
	pvcCopy := pvc.DeepCopy()
	delete(pvcCopy.Annotations, AnnCurrentCheckpoint)
	delete(pvcCopy.Annotations, AnnPreviousCheckpoint)
	delete(pvcCopy.Annotations, AnnFinalCheckpoint)
	delete(pvcCopy.Annotations, AnnCurrentPodID)

	prefix := r.getCheckpointCopiedKey("")
	for key := range pvcCopy.Annotations {
		if strings.HasPrefix(key, prefix) {
			delete(pvcCopy.Annotations, key)
		}
	}

	pvcCopy.ObjectMeta.Annotations[AnnMultiStageImportDone] = "true"

	// only update if something has changed
	if !reflect.DeepEqual(pvc, pvcCopy) {
		return r.updatePVC(pvcCopy)
	}
	return nil
}

// Single place to hold the scheme for annotations that indicate a checkpoint
// has already been copied. Currently storage.checkpoint.copied.[checkpoint] = ID,
// where ID is the UID of the pod that successfully transferred that checkpoint.
func (r *DatavolumeReconciler) getCheckpointCopiedKey(checkpoint string) string {
	return AnnCheckpointsCopied + "." + checkpoint
}

// Find out if this checkpoint has already been copied by looking for an annotation
// like storage.checkpoint.copied.[checkpoint]. If it exists, then this checkpoint
// was already copied.
func (r *DatavolumeReconciler) checkpointAlreadyCopied(pvc *corev1.PersistentVolumeClaim, checkpoint string) bool {
	annotation := r.getCheckpointCopiedKey(checkpoint)
	return metav1.HasAnnotation(pvc.ObjectMeta, annotation)
}

// Compare the list of checkpoints in the DataVolume spec with the annotations on the
// PVC indicating which checkpoints have already been copied. Return the first checkpoint
// that does not have this annotation, meaning the first checkpoint that has not yet been copied.
type checkpointRecord struct {
	cdiv1.DataVolumeCheckpoint
	IsFinal bool
}

func (r *DatavolumeReconciler) getNextCheckpoint(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) *checkpointRecord {
	numCheckpoints := len(dataVolume.Spec.Checkpoints)
	if numCheckpoints < 1 {
		return nil
	}

	// If there are no annotations, get the first checkpoint from the spec
	if pvc.ObjectMeta.Annotations[AnnCurrentCheckpoint] == "" {
		checkpoint := &checkpointRecord{
			cdiv1.DataVolumeCheckpoint{
				Current:  dataVolume.Spec.Checkpoints[0].Current,
				Previous: dataVolume.Spec.Checkpoints[0].Previous,
			},
			(numCheckpoints == 1) && dataVolume.Spec.FinalCheckpoint,
		}
		return checkpoint
	}

	// If there are annotations, keep checking the spec checkpoint list for an existing "copied.X" annotation until the first one not found
	for count, specCheckpoint := range dataVolume.Spec.Checkpoints {
		if specCheckpoint.Current == "" {
			r.log.Info(fmt.Sprintf("DataVolume spec has a blank 'current' entry in checkpoint %d", count))
			continue
		}
		if !r.checkpointAlreadyCopied(pvc, specCheckpoint.Current) {
			checkpoint := &checkpointRecord{
				cdiv1.DataVolumeCheckpoint{
					Current:  specCheckpoint.Current,
					Previous: specCheckpoint.Previous,
				},
				(numCheckpoints == (count + 1)) && dataVolume.Spec.FinalCheckpoint,
			}
			return checkpoint
		}
	}

	return nil
}

// Verify that the source PVC has been completely populated.
func (r *DatavolumeReconciler) isSourcePVCPopulated(dv *cdiv1.DataVolume) (bool, error) {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.Source.PVC.Name, Namespace: dv.Spec.Source.PVC.Namespace}, sourcePvc); err != nil {
		return false, err
	}
	return IsPopulated(sourcePvc, r.client)
}

func (r *DatavolumeReconciler) sourceInUse(dv *cdiv1.DataVolume, eventReason string) (bool, error) {
	pods, err := GetPodsUsingPVCs(r.client, dv.Spec.Source.PVC.Namespace, sets.NewString(dv.Spec.Source.PVC.Name), false)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		r.log.V(1).Info("Cannot snapshot",
			"namespace", dv.Namespace, "name", dv.Name, "pod namespace", pod.Namespace, "pod name", pod.Name)
		r.recorder.Eventf(dv, corev1.EventTypeWarning, eventReason,
			"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, dv.Spec.Source.PVC.Name)
	}

	return len(pods) > 0, nil
}

func (r *DatavolumeReconciler) initTransfer(log logr.Logger, dv *cdiv1.DataVolume, name string) (bool, error) {
	initialized := true

	log.Info("Initializing transfer")

	if !HasFinalizer(dv, crossNamespaceFinalizer) {
		AddFinalizer(dv, crossNamespaceFinalizer)
		if err := r.updateDataVolume(dv); err != nil {
			return false, err
		}

		initialized = false
	}

	ot := &cdiv1.ObjectTransfer{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name}, ot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		if err := validateCloneTokenDV(r.tokenValidator, dv); err != nil {
			return false, err
		}

		ot = &cdiv1.ObjectTransfer{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					common.CDILabelKey:       common.CDILabelValue,
					common.CDIComponentLabel: "",
				},
			},
			Spec: cdiv1.ObjectTransferSpec{
				Source: cdiv1.TransferSource{
					Kind:      "PersistentVolumeClaim",
					Namespace: dv.Spec.Source.PVC.Namespace,
					Name:      name,
					RequiredAnnotations: map[string]string{
						annReadyForTransfer: "true",
					},
				},
				Target: cdiv1.TransferTarget{
					Namespace: &dv.Namespace,
					Name:      &dv.Name,
				},
			},
		}
		util.SetRecommendedLabels(ot, r.installerLabels, "cdi-controller")

		if err := setAnnOwnedByDataVolume(ot, dv); err != nil {
			return false, err
		}

		if err := r.client.Create(context.TODO(), ot); err != nil {
			return false, err
		}

		initialized = false
	}

	return initialized, nil
}

func (r *DatavolumeReconciler) cleanupTransfer(log logr.Logger, dv *cdiv1.DataVolume, name string) error {
	if !HasFinalizer(dv, crossNamespaceFinalizer) {
		return nil
	}

	log.V(1).Info("Doing cleanup")

	if dv.DeletionTimestamp != nil && dv.Status.Phase != cdiv1.Succeeded {
		// delete all potential PVCs that may not have owner refs
		namespaces := []string{dv.Namespace}
		names := []string{dv.Name}
		if dv.Spec.Source.PVC != nil &&
			dv.Spec.Source.PVC.Namespace != "" &&
			dv.Spec.Source.PVC.Namespace != dv.Namespace {
			namespaces = append(namespaces, dv.Spec.Source.PVC.Namespace)
			names = append(names, name)
		}

		for i := range namespaces {
			pvc := &corev1.PersistentVolumeClaim{}
			nn := types.NamespacedName{Namespace: namespaces[i], Name: names[i]}
			if err := r.client.Get(context.TODO(), nn, pvc); err != nil {
				if !k8serrors.IsNotFound(err) {
					return err
				}
			} else {
				pod := &corev1.Pod{}
				nn := types.NamespacedName{Namespace: namespaces[i], Name: expansionPodName(pvc)}
				if err := r.client.Get(context.TODO(), nn, pod); err != nil {
					if !k8serrors.IsNotFound(err) {
						return err
					}
				} else {
					if err := r.client.Delete(context.TODO(), pod); err != nil {
						if !k8serrors.IsNotFound(err) {
							return err
						}
					}
				}

				if err := r.client.Delete(context.TODO(), pvc); err != nil {
					if !k8serrors.IsNotFound(err) {
						return err
					}
				}
			}
		}
	}

	ot := &cdiv1.ObjectTransfer{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name}, ot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		if ot.DeletionTimestamp == nil {
			if err := r.client.Delete(context.TODO(), ot); err != nil {
				if !k8serrors.IsNotFound(err) {
					return err
				}
			}
		}
		return fmt.Errorf("waiting for ObjectTransfer %s to delete", name)
	}

	RemoveFinalizer(dv, crossNamespaceFinalizer)
	if err := r.updateDataVolume(dv); dv != nil {
		return err
	}

	return nil
}

func expansionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return "cdi-expand-" + string(pvc.UID)
}

func (r *DatavolumeReconciler) expand(log logr.Logger,
	dv *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	targetSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
	if pvc.Status.Phase != corev1.ClaimBound {
		return false, fmt.Errorf("cannot expand volume in %q phase", pvc.Status.Phase)
	}

	requestedSize, hasRequested := targetSpec.Resources.Requests[corev1.ResourceStorage]
	currentSize, hasCurrent := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	actualSize, hasActual := pvc.Status.Capacity[corev1.ResourceStorage]
	if !hasRequested || !hasCurrent || !hasActual {
		return false, fmt.Errorf("PVC sizes missing")
	}

	expansionRequired := actualSize.Cmp(requestedSize) < 0
	updateRequestSizeRequired := currentSize.Cmp(requestedSize) < 0

	log.V(3).Info("Expand sizes", "req", requestedSize, "cur", currentSize, "act", actualSize, "exp", expansionRequired)

	if updateRequestSizeRequired {
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = requestedSize
		if err := r.updatePVC(pvc); err != nil {
			return false, err
		}

		return false, nil
	}

	podName := expansionPodName(pvc)
	podExists := true
	pod := &corev1.Pod{}
	nn := types.NamespacedName{Namespace: pvc.Namespace, Name: podName}
	if err := r.client.Get(context.TODO(), nn, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		podExists = false
	}

	if !expansionRequired && !podExists {
		// finally done
		return true, nil
	}

	hasPendingResizeCondition := false
	for _, cond := range pvc.Status.Conditions {
		if cond.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			hasPendingResizeCondition = true
			break
		}
	}

	if !podExists && !hasPendingResizeCondition {
		// wait for resize condition
		return false, nil
	}

	if !podExists {
		var err error
		pod, err = r.createExpansionPod(pvc, dv, podName)
		// Check if pod has failed and, in that case, record an event with the error
		if podErr := handleFailedPod(err, podName, pvc, r.recorder, r.client); podErr != nil {
			return false, podErr
		}
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		if err := r.client.Delete(context.TODO(), pod); err != nil {
			if k8serrors.IsNotFound(err) {
				return true, err
			}

			return false, err
		}

		return false, nil
	}

	return false, nil
}

func (r *DatavolumeReconciler) createExpansionPod(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, podName string) (*corev1.Pod, error) {
	resourceRequirements, err := GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := GetWorkloadNodePlacement(r.client)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "cdi-expander",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "dummy",
					Image:           r.clonerImage,
					ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
					Command:         []string{"/bin/bash"},
					Args:            []string{"-c", "echo", "'hello cdi'"},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"ALL",
							},
						},
						AllowPrivilegeEscalation: pointer.BoolPtr(false),
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
			NodeSelector: workloadNodePlacement.NodeSelector,
			Tolerations:  workloadNodePlacement.Tolerations,
			Affinity:     workloadNodePlacement.Affinity,
		},
	}
	util.SetRecommendedLabels(pod, r.installerLabels, "cdi-controller")

	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	if err := setAnnOwnedByDataVolume(pod, dv); err != nil {
		return nil, err
	}

	if err := r.client.Create(context.TODO(), pod); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	return pod, nil
}

func (r *DatavolumeReconciler) getStorageClassBindingMode(storageClassName *string) (*storagev1.VolumeBindingMode, error) {
	// Handle unspecified storage class name, fallback to default storage class
	storageClass, err := GetStorageClassByName(r.client, storageClassName)
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

func getStorageVolumeMode(c client.Client, dataVolume *cdiv1.DataVolume, storageClass *storagev1.StorageClass) (*corev1.PersistentVolumeMode, error) {
	if dataVolume.Spec.PVC != nil {
		return dataVolume.Spec.PVC.VolumeMode, nil
	} else if dataVolume.Spec.Storage != nil {
		if dataVolume.Spec.Storage.VolumeMode != nil {
			return dataVolume.Spec.Storage.VolumeMode, nil
		}
		volumeMode, err := getDefaultVolumeMode(c, storageClass, dataVolume.Spec.Storage.AccessModes)
		if err != nil {
			return nil, err
		}
		return volumeMode, nil
	}

	return nil, errors.Errorf("no target storage defined")
}

func (r *DatavolumeReconciler) reconcileProgressUpdate(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
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

func (r *DatavolumeReconciler) storageClassCSIDriverExists(storageClassName *string) (bool, error) {
	log := r.log.WithName("getCsiDriverForStorageClass").V(3)

	storageClass, err := GetStorageClassByName(r.client, storageClassName)
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

func (r *DatavolumeReconciler) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (string, error) {
	log := r.log.WithName("getSnapshotClassForSmartClone").V(3)
	// Check if relevant CRDs are available
	if !IsCsiCrdsDeployed(r.client, r.log) {
		log.Info("Missing CSI snapshotter CRDs, falling back to host assisted clone")
		return "", nil
	}

	targetPvcStorageClassName := targetStorageSpec.StorageClassName
	targetStorageClass, err := GetStorageClassByName(r.client, targetPvcStorageClassName)
	if err != nil {
		return "", err
	}
	if targetStorageClass == nil {
		log.Info("Target PVC's Storage Class not found")
		return "", nil
	}
	targetPvcStorageClassName = &targetStorageClass.Name
	// Fetch the source storage class
	srcStorageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *targetPvcStorageClassName}, srcStorageClass); err != nil {
		log.Info("Unable to retrieve storage class, falling back to host assisted clone", "storage class", *targetPvcStorageClassName)
		return "", err
	}

	// List the snapshot classes
	scs := &snapshotv1.VolumeSnapshotClassList{}
	if err := r.client.List(context.TODO(), scs); err != nil {
		log.Info("Cannot list snapshot classes, falling back to host assisted clone")
		return "", err
	}
	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Driver == srcStorageClass.Provisioner {
			log.Info("smart-clone is applicable for datavolume", "datavolume",
				dataVolume.Name, "snapshot class", snapshotClass.Name)
			return snapshotClass.Name, nil
		}
	}

	log.Info("Could not match snapshotter with storage class, falling back to host assisted clone")
	return "", nil

}

// Returns true if methods different from HostAssisted are possible,
// both snapshot and csi volume clone share the same basic requirements
func (r *DatavolumeReconciler) advancedClonePossible(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
	log := r.log.WithName("ClonePossible").V(3)

	sourcePvc, err := r.findSourcePvc(dataVolume)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, errors.New("source PVC not found")
		}
		return false, err
	}

	targetStorageClass, err := GetStorageClassByName(r.client, targetStorageSpec.StorageClassName)
	if err != nil {
		return false, err
	}
	if targetStorageClass == nil {
		log.Info("Target PVC's Storage Class not found")
		return false, nil
	}

	if ok := r.validateSameStorageClass(sourcePvc, targetStorageClass); !ok {
		return false, nil
	}

	if ok, err := r.validateSameVolumeMode(dataVolume, sourcePvc, targetStorageClass); !ok || err != nil {
		return false, err
	}

	return r.validateAdvancedCloneSizeCompatible(dataVolume, sourcePvc, targetStorageSpec)
}

func (r *DatavolumeReconciler) validateSameStorageClass(
	sourcePvc *corev1.PersistentVolumeClaim,
	targetStorageClass *storagev1.StorageClass) bool {

	targetPvcStorageClassName := &targetStorageClass.Name
	sourcePvcStorageClassName := sourcePvc.Spec.StorageClassName

	// Compare source and target storage classess
	if *sourcePvcStorageClassName != *targetPvcStorageClassName {
		r.log.V(3).Info("Source PVC and target PVC belong to different storage classes",
			"source storage class", *sourcePvcStorageClassName,
			"target storage class", *targetPvcStorageClassName)
		return false
	}

	return true
}

func (r *DatavolumeReconciler) validateSameVolumeMode(
	dataVolume *cdiv1.DataVolume,
	sourcePvc *corev1.PersistentVolumeClaim,
	targetStorageClass *storagev1.StorageClass) (bool, error) {

	sourceVolumeMode := util.ResolveVolumeMode(sourcePvc.Spec.VolumeMode)
	targetSpecVolumeMode, err := getStorageVolumeMode(r.client, dataVolume, targetStorageClass)
	if err != nil {
		return false, err
	}
	targetVolumeMode := util.ResolveVolumeMode(targetSpecVolumeMode)

	if sourceVolumeMode != targetVolumeMode {
		r.log.V(3).Info("Source PVC and target PVC have different volume modes, falling back to host assisted clone",
			"source volume mode", sourceVolumeMode, "target volume mode", targetVolumeMode)
		return false, nil
	}

	return true, nil
}

func (r *DatavolumeReconciler) validateAdvancedCloneSizeCompatible(
	dataVolume *cdiv1.DataVolume,
	sourcePvc *corev1.PersistentVolumeClaim,
	targetStorageSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
	srcStorageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *sourcePvc.Spec.StorageClassName}, srcStorageClass); IgnoreNotFound(err) != nil {
		return false, err
	}

	srcRequest, hasSrcRequest := sourcePvc.Spec.Resources.Requests[corev1.ResourceStorage]
	srcCapacity, hasSrcCapacity := sourcePvc.Status.Capacity[corev1.ResourceStorage]
	targetRequest, hasTargetRequest := targetStorageSpec.Resources.Requests[corev1.ResourceStorage]
	allowExpansion := srcStorageClass.AllowVolumeExpansion != nil && *srcStorageClass.AllowVolumeExpansion
	if !hasSrcRequest || !hasSrcCapacity || !hasTargetRequest {
		// return error so we retry the reconcile
		return false, errors.New("source/target size info missing")
	}

	if srcCapacity.Cmp(targetRequest) < 0 && !allowExpansion {
		return false, nil
	}

	if srcRequest.Cmp(targetRequest) > 0 && !targetRequest.IsZero() {
		return false, nil
	}

	return true, nil
}

func (r *DatavolumeReconciler) getCloneStrategy(dataVolume *cdiv1.DataVolume) (*cdiv1.CDICloneStrategy, error) {
	defaultCloneStrategy := cdiv1.CloneStrategySnapshot
	sourcePvc, err := r.findSourcePvc(dataVolume)
	if err != nil {
		return nil, err
	}
	storageClass, err := GetStorageClassByName(r.client, sourcePvc.Spec.StorageClassName)
	if err != nil {
		return nil, err
	}

	strategyOverride, err := r.getGlobalCloneStrategyOverride()
	if err != nil {
		return nil, err
	}
	if strategyOverride != nil {
		return strategyOverride, nil
	}

	// do check storageProfile and apply the preferences
	strategy, err := r.getPreferredCloneStrategyForStorageClass(storageClass)
	if err != nil {
		return nil, err
	}
	if strategy != nil {
		return strategy, err
	}

	return &defaultCloneStrategy, nil
}

func (r *DatavolumeReconciler) findSourcePvc(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
	sourcePvcSpec := dataVolume.Spec.Source.PVC
	if sourcePvcSpec == nil {
		return nil, errors.New("no source PVC provided")
	}

	// Find source PVC
	sourcePvcNs := sourcePvcSpec.Namespace
	if sourcePvcNs == "" {
		sourcePvcNs = dataVolume.Namespace
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: sourcePvcNs, Name: sourcePvcSpec.Name}, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			r.log.V(3).Info("Source PVC is missing", "source namespace", sourcePvcSpec.Namespace, "source name", sourcePvcSpec.Name)
		}
		return nil, err
	}
	return pvc, nil
}

func (r *DatavolumeReconciler) getGlobalCloneStrategyOverride() (*cdiv1.CDICloneStrategy, error) {
	cr, err := GetActiveCDI(r.client)
	if err != nil {
		return nil, err
	}

	if cr == nil {
		return nil, fmt.Errorf("no active CDI")
	}

	if cr.Spec.CloneStrategyOverride == nil {
		return nil, nil
	}

	r.log.V(3).Info(fmt.Sprintf("Overriding default clone strategy with %s", *cr.Spec.CloneStrategyOverride))
	return cr.Spec.CloneStrategyOverride, nil
}

func newSnapshot(dataVolume *cdiv1.DataVolume, snapshotName, snapshotClassName string) *snapshotv1.VolumeSnapshot {
	annotations := make(map[string]string)
	annotations[AnnSmartCloneRequest] = "true"
	className := snapshotClassName
	labels := map[string]string{
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	snapshotNamespace := dataVolume.Namespace
	if dataVolume.Spec.Source.PVC.Namespace != "" {
		snapshotNamespace = dataVolume.Spec.Source.PVC.Namespace
	}
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:        snapshotName,
			Namespace:   snapshotNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &dataVolume.Spec.Source.PVC.Name,
			},
			VolumeSnapshotClassName: &className,
		},
	}
	if dataVolume.Namespace == snapshotNamespace {
		snapshot.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(dataVolume, schema.GroupVersionKind{
				Group:   cdiv1.SchemeGroupVersion.Group,
				Version: cdiv1.SchemeGroupVersion.Version,
				Kind:    "DataVolume",
			}),
		}
	}
	return snapshot
}

// NewVolumeClonePVC creates a PVC object to be used during CSI volume cloning.
func (r *DatavolumeReconciler) newVolumeClonePVC(dv *cdiv1.DataVolume,
	sourcePvc *corev1.PersistentVolumeClaim,
	targetPvcSpec *corev1.PersistentVolumeClaimSpec,
	pvcName string) (*corev1.PersistentVolumeClaim, error) {

	// Override name - might be temporary pod when in transfer
	pvcNamespace := dv.Namespace
	if dv.Spec.Source.PVC.Namespace != "" {
		pvcNamespace = dv.Spec.Source.PVC.Namespace
	}

	pvc, err := r.newPersistentVolumeClaim(dv, targetPvcSpec, pvcNamespace, pvcName)
	if err != nil {
		return nil, err
	}

	// be sure correct clone method is selected
	delete(pvc.Annotations, AnnCloneRequest)
	pvc.Annotations[AnnCSICloneRequest] = "true"

	// take source size while cloning
	if pvc.Spec.Resources.Requests == nil {
		pvc.Spec.Resources.Requests = corev1.ResourceList{}
	}
	sourceSize := sourcePvc.Status.Capacity.Storage()
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = *sourceSize

	pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
		Name: dv.Spec.Source.PVC.Name,
		Kind: "PersistentVolumeClaim",
	}

	return pvc, nil
}

func (r *DatavolumeReconciler) updateImportStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if !ok {
		return
	}
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
		event.eventType = corev1.EventTypeWarning
		event.reason = ImportFailed
		event.message = fmt.Sprintf(MessageImportFailed, pvc.Name)
	case string(corev1.PodSucceeded):
		_, ok := pvc.Annotations[AnnCurrentCheckpoint]
		if ok {
			// this is a multistage import, set the datavolume status to paused
			dataVolumeCopy.Status.Phase = cdiv1.Paused
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportPaused
			event.message = fmt.Sprintf(MessageImportPaused, pvc.Name)
		} else {
			dataVolumeCopy.Status.Phase = cdiv1.Succeeded
			dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress("100.0%")
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportSucceeded
			event.message = fmt.Sprintf(MessageImportSucceeded, pvc.Name)
		}
	}
}

func (r *DatavolumeReconciler) updateCloneStatusPhase(phase cdiv1.DataVolumePhase,
	dataVolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	selectedCloneStrategy cloneStrategy) error {

	var event DataVolumeEvent

	switch phase {
	case cdiv1.CloneScheduled:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneScheduled
		event.message = fmt.Sprintf(MessageCloneScheduled, dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.SnapshotForSmartCloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = SnapshotForSmartCloneInProgress
		event.message = fmt.Sprintf(MessageSmartCloneInProgress, dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name)
	case cdiv1.CSICloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = string(cdiv1.CSICloneInProgress)
		event.message = fmt.Sprintf(MessageCsiCloneInProgress, dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name)
	case cdiv1.ExpansionInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = ExpansionInProgress
		event.message = fmt.Sprintf(MessageExpansionInProgress, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.NamespaceTransferInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = NamespaceTransferInProgress
		event.message = fmt.Sprintf(MessageNamespaceTransferInProgress, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.Succeeded:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name, dataVolume.Namespace, dataVolume.Name)
	}

	return r.updateDataVolumeStatusPhaseWithEvent(phase, dataVolume, pvc, selectedCloneStrategy, event)
}

func (r *DatavolumeReconciler) updateDataVolumeStatusPhaseWithEvent(
	phase cdiv1.DataVolumePhase,
	dataVolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	selectedCloneStrategy cloneStrategy,
	event DataVolumeEvent) error {

	var dataVolumeCopy = dataVolume.DeepCopy()
	curPhase := dataVolumeCopy.Status.Phase

	dataVolumeCopy.Status.Phase = phase

	reason := ""
	if pvc == nil {
		reason = event.reason
	}
	r.updateConditions(dataVolumeCopy, pvc, reason)
	AddAnnotation(dataVolumeCopy, annCloneType, cloneStrategyToCloneType(selectedCloneStrategy))

	return r.emitEvent(dataVolume, dataVolumeCopy, curPhase, dataVolume.Status.Conditions, &event)
}

func (r *DatavolumeReconciler) updateNetworkCloneStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if !ok {
		return
	}
	switch phase {
	case string(corev1.PodPending):
		dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneScheduled
		event.message = fmt.Sprintf(MessageCloneScheduled, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
	case string(corev1.PodRunning):
		dataVolumeCopy.Status.Phase = cdiv1.CloneInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneInProgress
		event.message = fmt.Sprintf(MessageCloneInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, pvc.Namespace, pvc.Name)
	case string(corev1.PodFailed):
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

func (r *DatavolumeReconciler) updateUploadStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) {
	phase, ok := pvc.Annotations[AnnPodPhase]
	if !ok {
		return
	}
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

func (r *DatavolumeReconciler) reconcileDataVolumeStatus(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {
	dataVolumeCopy := dataVolume.DeepCopy()
	var event DataVolumeEvent
	var err error
	result := reconcile.Result{}

	curPhase := dataVolumeCopy.Status.Phase
	if pvc != nil {
		dataVolumeCopy.Status.ClaimName = pvc.Name

		// the following check is for a case where the request is to create a blank disk for a block device.
		// in that case, we do not create a pod as there is no need to create a blank image.
		// instead, we just mark the DV phase as 'Succeeded' so any consumer will be able to use it.
		phase := pvc.Annotations[AnnPodPhase]
		if phase == string(cdiv1.Succeeded) {
			updateImport := true
			_, ok := pvc.Annotations[AnnCloneRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
				r.updateNetworkCloneStatusPhase(pvc, dataVolumeCopy, &event)
				updateImport = false
			}
			_, ok = pvc.Annotations[AnnUploadRequest]
			if ok {
				dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
				r.updateUploadStatusPhase(pvc, dataVolumeCopy, &event)
				updateImport = false
			}
			if updateImport {
				multiStageImport := metav1.HasAnnotation(pvc.ObjectMeta, AnnCurrentCheckpoint)
				if multiStageImport {
					// The presence of the current checkpoint annotation
					// indicates that this is a stage in a multistage import.
					// If all the checkpoints have been copied, then
					// we need to remove the annotations from the PVC and
					// set the DataVolume status to Succeeded. Otherwise,
					// we need to set the status to Paused to indicate that
					// the import is not yet done, and change the annotations
					// to advance to the next checkpoint.

					currentCheckpoint := pvc.Annotations[AnnCurrentCheckpoint]
					alreadyCopied := r.checkpointAlreadyCopied(pvc, currentCheckpoint)
					finalCheckpoint, _ := strconv.ParseBool(pvc.Annotations[AnnFinalCheckpoint])

					if finalCheckpoint && alreadyCopied { // Last checkpoint done! Clean up and mark DV success.
						dataVolumeCopy.Status.Phase = cdiv1.Succeeded
						err = r.deleteMultistageImportAnnotations(pvc)
						if err != nil {
							return reconcile.Result{}, err
						}
					} else { // Single stage of a multi-stage import
						dataVolumeCopy.Status.Phase = cdiv1.Paused
						err = r.setMultistageImportAnnotations(dataVolumeCopy, pvc) // Advances annotations to next checkpoint
						if err != nil {
							return reconcile.Result{}, err
						}
					}
				} else {
					dataVolumeCopy.Status.Phase = cdiv1.Succeeded
				}
				r.updateImportStatusPhase(pvc, dataVolumeCopy, &event)
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
						r.updateNetworkCloneStatusPhase(pvc, dataVolumeCopy, &event)
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
		result, err = r.reconcileProgressUpdate(dataVolumeCopy, pvc)
		if err != nil {
			return result, err
		}
	} else {
		_, ok := dataVolumeCopy.Annotations[AnnPrePopulated]
		if ok {
			dataVolumeCopy.Status.Phase = cdiv1.Pending
		}
	}

	if selectedCloneStrategy != NoClone {
		AddAnnotation(dataVolumeCopy, annCloneType, cloneStrategyToCloneType(selectedCloneStrategy))
	}

	currentCond := make([]cdiv1.DataVolumeCondition, len(dataVolumeCopy.Status.Conditions))
	copy(currentCond, dataVolumeCopy.Status.Conditions)
	r.updateConditions(dataVolumeCopy, pvc, "")
	return result, r.emitEvent(dataVolume, dataVolumeCopy, curPhase, currentCond, &event)
}

func (r *DatavolumeReconciler) updateConditions(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, reason string) {
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
	dataVolume.Status.Conditions = updateReadyCondition(dataVolume.Status.Conditions, readyStatus, "", reason)
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
func (r *DatavolumeReconciler) getPodFromPvc(namespace string, pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
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

	return r.updatePVC(pvc)
}

// If this is a completed pod that was used for one checkpoint of a multi-stage import, it
// should be ignored by pod lookups as long as the retainAfterCompletion annotation is set.
func shouldIgnorePod(pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim) bool {
	retain := pvc.ObjectMeta.Annotations[AnnPodRetainAfterCompletion]
	checkpoint := pvc.ObjectMeta.Annotations[AnnCurrentCheckpoint]
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

func errQuotaExceeded(err error) bool {
	return strings.Contains(err.Error(), "exceeded quota:")
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
func (r *DatavolumeReconciler) newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume, targetPvcSpec *corev1.PersistentVolumeClaimSpec, namespace string, name string) (*corev1.PersistentVolumeClaim, error) {
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
		for index, header := range dataVolume.Spec.Source.HTTP.ExtraHeaders {
			annotations[fmt.Sprintf("%s.%d", AnnExtraHeaders, index)] = header
		}
		for index, header := range dataVolume.Spec.Source.HTTP.SecretExtraHeaders {
			annotations[fmt.Sprintf("%s.%d", AnnSecretExtraHeaders, index)] = header
		}
	} else if dataVolume.Spec.Source.S3 != nil {
		annotations[AnnEndpoint] = dataVolume.Spec.Source.S3.URL
		annotations[AnnSource] = SourceS3
		if dataVolume.Spec.Source.S3.SecretRef != "" {
			annotations[AnnSecret] = dataVolume.Spec.Source.S3.SecretRef
		}
		if dataVolume.Spec.Source.S3.CertConfigMap != "" {
			annotations[AnnCertConfigMap] = dataVolume.Spec.Source.S3.CertConfigMap
		}
	} else if dataVolume.Spec.Source.Registry != nil {
		annotations[AnnSource] = SourceRegistry
		pullMethod := dataVolume.Spec.Source.Registry.PullMethod
		if pullMethod != nil && *pullMethod != "" {
			annotations[AnnRegistryImportMethod] = string(*pullMethod)
		}
		url := dataVolume.Spec.Source.Registry.URL
		if url != nil && *url != "" {
			annotations[AnnEndpoint] = *url
		} else {
			imageStream := dataVolume.Spec.Source.Registry.ImageStream
			if imageStream != nil && *imageStream != "" {
				annotations[AnnEndpoint] = *imageStream
				annotations[AnnRegistryImageStream] = "true"
			}
		}
		annotations[AnnContentType] = string(dataVolume.Spec.ContentType)
		secretRef := dataVolume.Spec.Source.Registry.SecretRef
		if secretRef != nil && *secretRef != "" {
			annotations[AnnSecret] = *secretRef
		}
		certConfigMap := dataVolume.Spec.Source.Registry.CertConfigMap
		if certConfigMap != nil && *certConfigMap != "" {
			annotations[AnnCertConfigMap] = *certConfigMap
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
		annotations[AnnContentType] = string(dataVolume.Spec.ContentType)
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
		if dataVolume.Spec.Source.VDDK.InitImageURL != "" {
			annotations[AnnVddkInitImageURL] = dataVolume.Spec.Source.VDDK.InitImageURL
		}
	} else {
		return nil, errors.Errorf("no source set for datavolume")
	}
	if dataVolume.Spec.PriorityClassName != "" {
		annotations[AnnPriorityClassName] = dataVolume.Spec.PriorityClassName
	}
	annotations[AnnPreallocationRequested] = strconv.FormatBool(GetPreallocation(r.client, dataVolume))

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
		pvc.Annotations[annOwnerUID] = string(dataVolume.UID)
	}

	return pvc, nil
}

// If sourceRef is set, populate spec.Source with data from the DataSource
func (r *DatavolumeReconciler) populateSourceIfSourceRef(dv *cdiv1.DataVolume) error {
	if dv.Spec.SourceRef == nil {
		return nil
	}
	if dv.Spec.SourceRef.Kind != cdiv1.DataVolumeDataSource {
		return errors.Errorf("Unsupported sourceRef kind %s, currently only %s is supported", dv.Spec.SourceRef.Kind, cdiv1.DataVolumeDataSource)
	}
	ns := dv.Namespace
	if dv.Spec.SourceRef.Namespace != nil && *dv.Spec.SourceRef.Namespace != "" {
		ns = *dv.Spec.SourceRef.Namespace
	}
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.SourceRef.Name, Namespace: ns}, dataSource); err != nil {
		return err
	}
	dv.Spec.Source = &cdiv1.DataVolumeSource{
		PVC: dataSource.Spec.Source.PVC,
	}
	return nil
}

// Whenever the controller updates a DV, we must make sure to nil out spec.source when spec.sourceRef is set
func (r *DatavolumeReconciler) updateDataVolume(dv *cdiv1.DataVolume) error {
	if dv.Spec.SourceRef != nil {
		dv.Spec.Source = nil
	}
	return r.client.Update(context.TODO(), dv)
}

func (r *DatavolumeReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim) error {
	return r.client.Update(context.TODO(), pvc)
}

func getName(storageClass *storagev1.StorageClass) string {
	if storageClass != nil {
		return storageClass.Name
	}
	return ""
}

func (r *DatavolumeReconciler) getPreferredCloneStrategyForStorageClass(storageClass *storagev1.StorageClass) (*cdiv1.CDICloneStrategy, error) {
	if storageClass == nil {
		// fallback to defaults
		return nil, nil
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get StorageProfile")
	}

	return storageProfile.Status.CloneStrategy, nil
}

// GetRequiredSpace calculates space required taking file system overhead into account
func GetRequiredSpace(filesystemOverhead float64, requestedSpace int64) int64 {
	// the `image` has to be aligned correctly, so the space requested has to be aligned to
	// next value that is a multiple of a block size
	alignedSize := util.RoundUp(requestedSpace, util.DefaultAlignBlockSize)

	// count overhead as a percentage of the whole/new size, including aligned image
	// and the space required by filesystem metadata
	spaceWithOverhead := int64(math.Ceil(float64(alignedSize) / (1 - filesystemOverhead)))
	return spaceWithOverhead
}

func newLongTermCloneTokenGenerator(key *rsa.PrivateKey) token.Generator {
	return token.NewGenerator(common.ExtendedCloneTokenIssuer, key, 10*365*24*time.Hour)
}

func (r *DatavolumeReconciler) garbageCollect(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*reconcile.Result, error) {
	if dataVolume.Status.Phase != cdiv1.Succeeded {
		return nil, nil
	}
	cdiConfig := &cdiv1.CDIConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		return nil, err
	}
	dvTTL := cdiConfig.Spec.DataVolumeTTLSeconds
	if dvTTL == nil || *dvTTL < 0 {
		log.Info("Garbage Collection is disabled")
		return nil, nil
	}
	// Current DV still has TTL, so reconcile will return with the needed RequeueAfter
	if delta := getDeltaTTL(dataVolume, *dvTTL); delta > 0 {
		return &reconcile.Result{RequeueAfter: delta}, nil
	}
	if err := r.detachPvcDeleteDv(pvc, dataVolume, log); err != nil {
		return nil, err
	}
	return &reconcile.Result{}, nil
}

func (r *DatavolumeReconciler) detachPvcDeleteDv(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, log logr.Logger) error {
	if dv.Status.Phase != cdiv1.Succeeded {
		return nil
	}
	dvDelete := dv.Annotations[AnnDeleteAfterCompletion]
	if dvDelete != "true" {
		log.Info("DataVolume is not annotated to be garbage collected")
		return nil
	}
	updatePvcOwnerRefs(pvc, dv)
	delete(pvc.Annotations, AnnPopulatedFor)
	if err := r.updatePVC(pvc); err != nil {
		return err
	}
	if err := r.client.Delete(context.TODO(), dv); err != nil {
		return err
	}
	return nil
}

func updatePvcOwnerRefs(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
	refs := pvc.OwnerReferences
	for i, r := range refs {
		if r.UID == dv.UID {
			pvc.OwnerReferences = append(refs[:i], refs[i+1:]...)
			break
		}
	}
	pvc.OwnerReferences = append(pvc.OwnerReferences, dv.OwnerReferences...)
}

func getDeltaTTL(dv *cdiv1.DataVolume, ttl int32) time.Duration {
	delta := time.Second * time.Duration(ttl)
	if cond := findConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions); cond != nil {
		delta -= time.Now().Sub(cond.LastTransitionTime.Time)
	}
	return delta
}

// validateCloneAndSourcePVC checks if the source PVC of a clone exists and does proper validation
func (r *DatavolumeReconciler) validateCloneAndSourcePVC(datavolume *cdiv1.DataVolume) (bool, error) {
	sourcePvc, err := r.findSourcePvc(datavolume)
	if err != nil {
		// Clone without source
		if k8serrors.IsNotFound(err) {
			r.updateDataVolumeStatusPhaseWithEvent(datavolume.Status.Phase, datavolume, nil, NoClone,
				DataVolumeEvent{
					eventType: corev1.EventTypeWarning,
					reason:    CloneWithoutSource,
					message:   fmt.Sprintf(MessageCloneWithoutSource, datavolume.Spec.Source.PVC.Name),
				})
			return false, nil
		}
		return false, err
	}

	err = ValidateClone(sourcePvc, &datavolume.Spec)
	if err != nil {
		r.recorder.Event(datavolume, corev1.EventTypeWarning, CloneValidationFailed, MessageCloneValidationFailed)
		return false, err
	}

	return true, nil
}

// isSourceReadyToClone handles the reconciling process of a clone when the source PVC is not ready
func (r *DatavolumeReconciler) isSourceReadyToClone(
	datavolume *cdiv1.DataVolume,
	selectedCloneStrategy cloneStrategy) (bool, error) {

	var eventReason string

	switch selectedCloneStrategy {
	case SmartClone:
		eventReason = SmartCloneSourceInUse
	case CsiClone:
		eventReason = CSICloneSourceInUse
	case HostAssistedClone:
		eventReason = HostAssistedCloneSourceInUse
	}
	// Check if any pods are using the source PVC
	inUse, err := r.sourceInUse(datavolume, eventReason)
	if err != nil {
		return false, err
	}
	// Check if the source PVC is fully populated
	populated, err := r.isSourcePVCPopulated(datavolume)
	if err != nil {
		return false, err
	}

	if inUse || !populated {
		return false, nil
	}

	return true, nil
}

// shouldBeMarkedWaitForFirstConsumer decided whether we should mark DV as WFFC
func (r *DatavolumeReconciler) shouldBeMarkedWaitForFirstConsumer(pvc *corev1.PersistentVolumeClaim) (bool, error) {
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

// detectCloneSize obtains and assigns the original PVC's size when cloning using an empty storage value
func (r *DatavolumeReconciler) detectCloneSize(
	dv *cdiv1.DataVolume,
	targetPvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	cloneType cloneStrategy) (bool, error) {

	var targetSize int64
	sourcePvc, err := r.findSourcePvc(dv)
	if err != nil {
		return false, err
	}
	sourceCapacity := sourcePvc.Status.Capacity.Storage()

	// Due to possible filesystem overhead complications when cloning
	// using host-assisted strategy, we create a pod that automatically
	// collects the size of the original virtual image with 'qemu-img'.
	// If another strategy is used or the original PVC's volume mode
	// is "block", we simply extract the value from the original PVC's spec.
	if cloneType == HostAssistedClone &&
		getVolumeMode(sourcePvc) == corev1.PersistentVolumeFilesystem &&
		GetContentType(sourcePvc) == string(cdiv1.DataVolumeKubeVirt) {
		var available bool
		// If available, we first try to get the virtual size from previous iterations
		targetSize, available = getSizeFromAnnotations(sourcePvc)
		if !available {
			targetSize, err = r.getSizeFromPod(targetPvc, sourcePvc, dv)
			if err != nil {
				return false, err
			} else if targetSize == 0 {
				return false, nil
			}
		}

	} else {
		targetSize, _ = sourceCapacity.AsInt64()
	}

	// Allow the clone-controller to skip the size comparison requirement
	// if the source's size ends up being larger due to overhead differences
	// TODO: Fix this in next PR that uses actual size also in validation
	if sourceCapacity.CmpInt64(targetSize) == 1 {
		dv.Annotations[AnnPermissiveClone] = "true"
	}

	// Parse size into a 'Quantity' struct and, if needed, inflate it with filesystem overhead
	targetCapacity, err := inflateSizeWithOverhead(r.client, targetSize, pvcSpec)
	if err != nil {
		return false, err
	}

	pvcSpec.Resources.Requests[corev1.ResourceStorage] = targetCapacity
	return true, nil
}

// getSizeFromAnnotations checks the source PVC's annotations and returns the requested size if it has already been obtained
func getSizeFromAnnotations(sourcePvc *corev1.PersistentVolumeClaim) (int64, bool) {
	virtualImageSize, available := sourcePvc.Annotations[AnnVirtualImageSize]
	if available {
		sourceCapacity, available := sourcePvc.Annotations[AnnSourceCapacity]
		currCapacity := sourcePvc.Status.Capacity
		// Checks if the original PVC's capacity has changed
		if available && currCapacity.Storage().Cmp(resource.MustParse(sourceCapacity)) == 0 {
			// Parse the raw string containing the image size into a 64-bit int
			imgSizeInt, _ := strconv.ParseInt(virtualImageSize, 10, 64)
			return imgSizeInt, true
		}
	}

	return 0, false
}

// getSizeFromPod attempts to get the image size from a pod that directly obtains said value from the source PVC
func (r *DatavolumeReconciler) getSizeFromPod(targetPvc, sourcePvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (int64, error) {
	// The pod should not be created until the source PVC has finished the import process
	if !isPVCComplete(sourcePvc) {
		r.recorder.Event(dv, corev1.EventTypeNormal, ImportPVCNotReady, MessageImportPVCNotReady)
		return 0, nil
	}

	pod, err := r.getOrCreateSizeDetectionPod(sourcePvc, dv)
	// Check if pod has failed and, in that case, record an event with the error
	if podErr := handleFailedPod(err, sizeDetectionPodName(sourcePvc), targetPvc, r.recorder, r.client); podErr != nil {
		return 0, podErr
	} else if !isPodComplete(pod) {
		r.recorder.Event(dv, corev1.EventTypeNormal, SizeDetectionPodNotReady, MessageSizeDetectionPodNotReady)
		return 0, nil
	}

	// Parse raw image size from the pod's termination message
	if pod.Status.ContainerStatuses == nil ||
		pod.Status.ContainerStatuses[0].State.Terminated == nil ||
		pod.Status.ContainerStatuses[0].State.Terminated.ExitCode > 0 {
		return 0, r.handleSizeDetectionError(pod, dv, sourcePvc)
	}
	termMsg := pod.Status.ContainerStatuses[0].State.Terminated.Message
	imgSize, _ := strconv.ParseInt(termMsg, 10, 64)
	// Update Source PVC annotations
	if err := r.updateClonePVCAnnotations(sourcePvc, termMsg); err != nil {
		return imgSize, err
	}
	// Finally, detelete the pod
	if shouldDeletePod(sourcePvc) {
		err = r.client.Delete(context.TODO(), pod)
		if err != nil && !k8serrors.IsNotFound(err) {
			return imgSize, err
		}
	}

	return imgSize, nil
}

// getOrCreateSizeDetectionPod gets the size-detection pod if it already exists/creates it if not
func (r *DatavolumeReconciler) getOrCreateSizeDetectionPod(
	sourcePvc *corev1.PersistentVolumeClaim,
	dv *cdiv1.DataVolume) (*corev1.Pod, error) {

	podName := sizeDetectionPodName(sourcePvc)
	pod := &corev1.Pod{}
	nn := types.NamespacedName{Namespace: sourcePvc.Namespace, Name: podName}

	// Trying to get the pod if it already exists/create it if not
	if err := r.client.Get(context.TODO(), nn, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		// Generate the pod spec
		pod = r.makeSizeDetectionPodSpec(sourcePvc, dv)
		if pod == nil {
			return nil, errors.Errorf("Size-detection pod spec could not be generated")
		}
		// Create the pod
		if err := r.client.Create(context.TODO(), pod); err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return nil, err
			}
		}

		r.recorder.Event(dv, corev1.EventTypeNormal, SizeDetectionPodCreated, MessageSizeDetectionPodCreated)
		r.log.V(3).Info(MessageSizeDetectionPodCreated, "pod.Name", pod.Name, "pod.Namespace", pod.Namespace)
	}

	return pod, nil
}

// makeSizeDetectionPodSpec creates and returns the full size-detection pod spec
func (r *DatavolumeReconciler) makeSizeDetectionPodSpec(
	sourcePvc *corev1.PersistentVolumeClaim,
	dv *cdiv1.DataVolume) *corev1.Pod {

	workloadNodePlacement, err := GetWorkloadNodePlacement(r.client)
	if err != nil {
		return nil
	}
	// Generate individual specs
	objectMeta := makeSizeDetectionObjectMeta(sourcePvc, dv)
	volume := makeSizeDetectionVolumeSpec(sourcePvc.Name)
	container := r.makeSizeDetectionContainerSpec(volume.Name)
	if container == nil {
		return nil
	}
	// Assemble the pod
	pod := &corev1.Pod{
		ObjectMeta: *objectMeta,
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				*container,
			},
			Volumes: []corev1.Volume{
				*volume,
			},
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			NodeSelector:      workloadNodePlacement.NodeSelector,
			Tolerations:       workloadNodePlacement.Tolerations,
			Affinity:          workloadNodePlacement.Affinity,
			PriorityClassName: getPriorityClass(sourcePvc),
		},
	}

	return pod
}

// makeSizeDetectionObjectMeta creates and returns the object metadata for the size-detection pod
func makeSizeDetectionObjectMeta(sourcePvc *corev1.PersistentVolumeClaim, dataVolume *cdiv1.DataVolume) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      sizeDetectionPodName(sourcePvc),
		Namespace: sourcePvc.Namespace,
		Labels: map[string]string{
			common.CDILabelKey:       common.CDILabelValue,
			common.CDIComponentLabel: common.ImporterPodName,
		},
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(dataVolume, schema.GroupVersionKind{
				Group:   cdiv1.SchemeGroupVersion.Group,
				Version: cdiv1.SchemeGroupVersion.Version,
				Kind:    "DataVolume",
			}),
		},
	}
}

// makeSizeDetectionContainerSpec creates and returns the size-detection pod's Container spec
func (r *DatavolumeReconciler) makeSizeDetectionContainerSpec(volName string) *corev1.Container {
	container := corev1.Container{
		Name:            "size-detection-volume",
		Image:           r.importerImage,
		ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
		Command:         []string{"/usr/bin/cdi-image-size-detection"},
		Args:            []string{"-image-path", common.ImporterWritePath},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: common.ImporterVolumePath,
				Name:      volName,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: pointer.BoolPtr(false),
		},
	}

	// Get and assign container's default resource requirements
	resourceRequirements, err := GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil
	}
	if resourceRequirements != nil {
		container.Resources = *resourceRequirements
	}

	return &container
}

// makeSizeDetectionVolumeSpec creates and returns the size-detection pod's Volume spec
func makeSizeDetectionVolumeSpec(pvcName string) *corev1.Volume {
	return &corev1.Volume{
		Name: DataVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
}

// handleSizeDetectionError handles the termination of the size-detection pod in case of error
func (r *DatavolumeReconciler) handleSizeDetectionError(pod *corev1.Pod, dv *cdiv1.DataVolume, sourcePvc *corev1.PersistentVolumeClaim) error {
	var event DataVolumeEvent
	var exitCode int

	if pod.Status.ContainerStatuses == nil || pod.Status.ContainerStatuses[0].State.Terminated == nil {
		exitCode = ErrUnknown
	} else {
		exitCode = int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
	}

	// We attempt to delete the pod
	err := r.client.Delete(context.TODO(), pod)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	switch exitCode {
	case ErrBadArguments:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrBadArguments"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case ErrInvalidPath:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrInvalidPath"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case ErrInvalidFile:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrInvalidFile"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case ErrBadTermFile:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrBadTermFile"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	default:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrUnknown"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	}

	r.recorder.Event(dv, event.eventType, event.reason, event.message)
	return ErrInvalidTermMsg
}

// updateClonePVCAnnotations updates the clone-related annotations of the source PVC
func (r *DatavolumeReconciler) updateClonePVCAnnotations(sourcePvc *corev1.PersistentVolumeClaim, virtualSize string) error {
	currCapacity := sourcePvc.Status.Capacity
	sourcePvc.Annotations[AnnVirtualImageSize] = virtualSize
	sourcePvc.Annotations[AnnSourceCapacity] = currCapacity.Storage().String()
	return r.client.Update(context.TODO(), sourcePvc)
}

// sizeDetectionPodName returns the name of the size-detection pod accoding to the source PVC's UID
func sizeDetectionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("size-detection-%s", pvc.UID)
}
