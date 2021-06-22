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
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kubevirt.io/containerized-data-importer/pkg/util"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/token"
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
	// MessageUploadScheduled provides a const to form upload is scheduled message
	MessageUploadScheduled = "Upload into %s scheduled"
	// MessageUploadReady provides a const to form upload is ready message
	MessageUploadReady = "Upload into %s ready"
	// MessageUploadFailed provides a const to form upload has failed message
	MessageUploadFailed = "Upload into %s failed"
	// MessageUploadSucceeded provides a const to form upload has succeeded message
	MessageUploadSucceeded = "Successfully uploaded into %s"
	// ExpansionInProgress is const representing target PVC expansion
	ExpansionInProgress = "ExpansionInProgress"
	// MessageExpansionInProgress is a const for reporting target expansion
	MessageExpansionInProgress = "Expanding PersistentVolumeClaim for DataVolume %s/%s"
	// NamespaceTransferInProgress is const representing target PVC transfer
	NamespaceTransferInProgress = "NamespaceTransferInProgress"
	// MessageNamespaceTransferInProgress is a const for reporting target transfer
	MessageNamespaceTransferInProgress = "Transferring PersistentVolumeClaim for DataVolume %s/%s"

	annOwnedByDataVolume = "cdi.kubevirt.io/ownedByDataVolume"

	annOwnerUID = "cdi.kubevirt.io/ownerUID"

	crossNamespaceFinalizer = "cdi.kubevirt.io/dataVolumeFinalizer"

	annReadyForTransfer = "cdi.kubevirt.io/readyForTransfer"

	annCloneType = "cdi.kubevirt.io/cloneType"
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
	client         client.Client
	extClientSet   extclientset.Interface
	recorder       record.EventRecorder
	scheme         *runtime.Scheme
	log            logr.Logger
	featureGates   featuregates.FeatureGates
	image          string
	pullPolicy     string
	tokenValidator token.Validator
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
	mgr manager.Manager,
	extClientSet extclientset.Interface,
	log logr.Logger,
	image, pullPolicy string,
	apiServerKey *rsa.PublicKey,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &DatavolumeReconciler{
		client:         client,
		scheme:         mgr.GetScheme(),
		extClientSet:   extClientSet,
		log:            log.WithName("datavolume-controller"),
		recorder:       mgr.GetEventRecorderFor("datavolume-controller"),
		featureGates:   featuregates.NewFeatureGates(client),
		image:          image,
		pullPolicy:     pullPolicy,
		tokenValidator: newCloneTokenValidator(apiServerKey),
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

	cloneStrategy, err := r.getCloneStrategy()
	if err != nil {
		return reconcile.Result{}, err
	}

	bindingMode, err := r.getStorageClassBindingMode(pvcSpec.StorageClassName)
	if err != nil {
		return reconcile.Result{}, err
	}

	snapshotClassName, err := r.getSnapshotClassForSmartClone(datavolume, pvcSpec)
	doSmartClone := err == nil &&
		cloneStrategy == cdiv1.CloneStrategySnapshot &&
		(!isCrossNamespaceClone(datavolume) || *bindingMode == storagev1.VolumeBindingImmediate)

	if !pvcExists {
		if doSmartClone {
			pvcName := datavolume.Name
			if isCrossNamespaceClone(datavolume) {
				pvcName = transferName
				initialized, err := r.initTransfer(log, datavolume, pvcName)
				if err != nil {
					return reconcile.Result{}, err
				}

				// get reconciled again v soon
				if !initialized {
					return reconcile.Result{},
						r.updateSmartCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil)
				}

				tmpPVC := &corev1.PersistentVolumeClaim{}
				nn := types.NamespacedName{Namespace: datavolume.Spec.Source.PVC.Namespace, Name: pvcName}
				if err := r.client.Get(context.TODO(), nn, tmpPVC); err != nil {
					if !k8serrors.IsNotFound(err) {
						return reconcile.Result{}, err
					}
				} else if tmpPVC.Annotations[AnnCloneOf] == "true" {
					done, err := r.expand(log, datavolume, tmpPVC, pvcSpec)
					if err != nil {
						return reconcile.Result{}, err
					}
					if !done {
						return reconcile.Result{},
							r.updateSmartCloneStatusPhase(cdiv1.ExpansionInProgress, datavolume, nil)
					}

					// trigger transfer and next reconcile should have pvcExists == true
					tmpPVC.Annotations[annReadyForTransfer] = "true"
					tmpPVC.Annotations[AnnPopulatedFor] = datavolume.Name
					if err := r.client.Update(context.TODO(), tmpPVC); err != nil {
						return reconcile.Result{}, err
					}

					return reconcile.Result{},
						r.updateSmartCloneStatusPhase(cdiv1.NamespaceTransferInProgress, datavolume, nil)
				} else {
					return reconcile.Result{}, nil
				}
			}

			if datavolume.Status.Phase == cdiv1.NamespaceTransferInProgress {
				return reconcile.Result{}, nil
			}

			r.log.V(3).Info("Smart-Clone via Snapshot is available with Volume Snapshot Class",
				"snapshotClassName", snapshotClassName)

			newSnapshot := newSnapshot(datavolume, pvcName, snapshotClassName)
			if err := setAnnOwnedByDataVolume(newSnapshot, datavolume); err != nil {
				return reconcile.Result{}, err
			}

			nn := client.ObjectKeyFromObject(newSnapshot)
			if err := r.client.Get(context.TODO(), nn, newSnapshot.DeepCopy()); err != nil {
				if !k8serrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}

				inUse, err := r.sourceInUse(datavolume)
				if err != nil {
					return reconcile.Result{}, err
				}
				populated, err := r.isSourcePVCPopulated(datavolume)
				if err != nil {
					return reconcile.Result{}, err
				}
				if inUse || !populated {
					return reconcile.Result{Requeue: true},
						r.updateSmartCloneStatusPhase(cdiv1.CloneScheduled, datavolume, nil)
				}

				if err := r.client.Create(context.TODO(), newSnapshot); err != nil {
					if k8serrors.IsAlreadyExists(err) {
						return reconcile.Result{}, nil
					}

					return reconcile.Result{}, err
				}

				return reconcile.Result{},
					r.updateSmartCloneStatusPhase(cdiv1.SnapshotForSmartCloneInProgress, datavolume, nil)
			}

			return reconcile.Result{}, nil
		}
		log.Info("Creating PVC for datavolume")
		newPvc, err := r.newPersistentVolumeClaim(datavolume, pvcSpec)
		if err != nil {
			return reconcile.Result{}, err
		}
		checkpoint := r.getNextCheckpoint(datavolume, newPvc)
		if checkpoint != nil { // Initialize new warm import annotations before creating PVC
			newPvc.ObjectMeta.Annotations[AnnCurrentCheckpoint] = checkpoint.Current
			newPvc.ObjectMeta.Annotations[AnnPreviousCheckpoint] = checkpoint.Previous
			newPvc.ObjectMeta.Annotations[AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)
		}
		if err := r.client.Create(context.TODO(), newPvc); err != nil {
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

		if pvc.Status.Phase == corev1.ClaimBound {
			// If a PVC already exists with no multi-stage annotations, check if it
			// needs them set (if not already finished with an import).
			multiStageImport := (len(datavolume.Spec.Checkpoints) > 0)
			multiStageAnnotationsSet := metav1.HasAnnotation(pvc.ObjectMeta, AnnCurrentCheckpoint)
			multiStageAlreadyDone := metav1.HasAnnotation(pvc.ObjectMeta, AnnMultiStageImportDone)
			if multiStageImport && !multiStageAnnotationsSet && !multiStageAlreadyDone {
				err := r.setMultistageImportAnnotations(datavolume, pvc)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	if doSmartClone {
		if isCrossNamespaceClone(datavolume) && datavolume.Status.Phase == cdiv1.Succeeded {
			if err := r.cleanupTransfer(log, datavolume, transferName); err != nil {
				return reconcile.Result{}, err
			}

			// done, done
			return reconcile.Result{}, nil
		}

		if pvc.Annotations[AnnCloneOf] != "true" {
			return reconcile.Result{}, nil
		}

		// expand for non-namespace case
		done, err := r.expand(log, datavolume, pvc, pvcSpec)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !done {
			return reconcile.Result{},
				r.updateSmartCloneStatusPhase(cdiv1.ExpansionInProgress, datavolume, pvc)
		}

		// done
		return reconcile.Result{}, r.updateSmartCloneStatusPhase(cdiv1.Succeeded, datavolume, pvc)
	}

	// Finally, we update the status block of the DataVolume resource to reflect the
	// current state of the world
	return r.reconcileDataVolumeStatus(datavolume, pvc)
}

func (r *DatavolumeReconciler) getVddkAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	var dataVolumeCopy = dataVolume.DeepCopy()
	if vddkHost := pvc.Annotations[AnnVddkHostConnection]; vddkHost != "" {
		addAnnotation(dataVolumeCopy, AnnVddkHostConnection, vddkHost)
	}
	if vddkVersion := pvc.Annotations[AnnVddkVersion]; vddkVersion != "" {
		addAnnotation(dataVolumeCopy, AnnVddkVersion, vddkVersion)
	}

	// only update if something has changed
	if !reflect.DeepEqual(dataVolume, dataVolumeCopy) {
		return true, r.client.Update(context.TODO(), dataVolumeCopy)
	}
	return false, nil
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
		pod, _ := r.getPodFromPvc(podNamespace, pvcCopy.ObjectMeta.UID)
		if pod == nil && phase == string(corev1.PodSucceeded) {
			// Reset PVC phase so importer will create a new pod
			pvcCopy.ObjectMeta.Annotations[AnnPodPhase] = string(corev1.PodUnknown)
		}
		// else: There's a pod already running, no need to try to start a new one.
	}
	// else: There aren't any checkpoints ready to be copied over.

	// only update if something has changed
	if !reflect.DeepEqual(pvc, pvcCopy) {
		return r.client.Update(context.TODO(), pvcCopy)
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
		return r.client.Update(context.TODO(), pvcCopy)
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

func (r *DatavolumeReconciler) sourceInUse(dv *cdiv1.DataVolume) (bool, error) {
	pods, err := GetPodsUsingPVCs(r.client, dv.Spec.Source.PVC.Namespace, sets.NewString(dv.Spec.Source.PVC.Name), false)
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

func (r *DatavolumeReconciler) initTransfer(log logr.Logger, dv *cdiv1.DataVolume, name string) (bool, error) {
	initialized := true

	log.Info("Initializing transfer")

	if !HasFinalizer(dv, crossNamespaceFinalizer) {
		AddFinalizer(dv, crossNamespaceFinalizer)
		if err := r.client.Update(context.TODO(), dv); err != nil {
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

	log.Info("Doing cleanup")

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
	if err := r.client.Update(context.TODO(), dv); dv != nil {
		return err
	}

	return nil
}

func expansionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return "cdi-expand-" + string(pvc.UID)
}

func (r *DatavolumeReconciler) expand(log logr.Logger, dv *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim, targetSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
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

	log.V(3).Info("Expand sizes", "req", requestedSize, "cur", currentSize, "act", actualSize, "exp", expansionRequired)

	if expansionRequired && requestedSize.Cmp(currentSize) != 0 {
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = requestedSize
		if err := r.client.Update(context.TODO(), pvc); err != nil {
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
		if err != nil {
			return false, err
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
					Image:           r.image,
					ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
					Command:         []string{"/bin/bash"},
					Args:            []string{"-c", "echo", "'hello cdi'"},
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
		volumeMode, err := getDefaultVolumeMode(c, storageClass)
		if err != nil {
			return nil, err
		}
		return volumeMode, nil
	}

	return nil, errors.Errorf("no target storage defined")
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

func (r *DatavolumeReconciler) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (string, error) {
	// TODO: Figure out if this belongs somewhere else, seems like something for the smart clone controller.
	// Check if clone is requested
	sourcePvcSpec := dataVolume.Spec.Source.PVC
	if sourcePvcSpec == nil {
		return "", errors.New("no source PVC provided")
	}

	// Check if relevant CRDs are available
	if !IsCsiCrdsDeployed(r.extClientSet) {
		r.log.V(3).Info("Missing CSI snapshotter CRDs, falling back to host assisted clone")
		return "", errors.New("CSI snapshot CRDs not found")
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
		return "", errors.New("source PVC not found")
	}

	targetPvcStorageClassName := targetStorageSpec.StorageClassName
	targetStorageClass, err := GetStorageClassByName(r.client, targetPvcStorageClassName)
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

	sourceVolumeMode := resolveVolumeMode(pvc.Spec.VolumeMode)
	targetSpecVolumeMode, err := getStorageVolumeMode(r.client, dataVolume, targetStorageClass)
	if err != nil {
		return "", err
	}
	targetVolumeMode := resolveVolumeMode(targetSpecVolumeMode)
	if sourceVolumeMode != targetVolumeMode {
		r.log.V(3).Info("Source PVC and target PVC have different volume modes, falling back to host assisted clone", "source volume mode",
			sourceVolumeMode, "target volume mode", targetVolumeMode)
		return "", errors.New("Source PVC and target PVC have different volume modes, falling back to host assisted clone")
	}

	// Fetch the source storage class
	srcStorageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *sourcePvcStorageClassName}, srcStorageClass); err != nil {
		r.log.V(3).Info("Unable to retrieve storage class, falling back to host assisted clone", "storage class", *sourcePvcStorageClassName)
		return "", errors.New("unable to retrieve storage class, falling back to host assisted clone")
	}

	srcCapacity, hasSrcCapacity := pvc.Status.Capacity[corev1.ResourceStorage]
	targetRequest, hasTargetRequest := targetStorageSpec.Resources.Requests[corev1.ResourceStorage]
	allowExpansion := srcStorageClass.AllowVolumeExpansion != nil && *srcStorageClass.AllowVolumeExpansion
	if !hasSrcCapacity || !hasTargetRequest || (srcCapacity.Cmp(targetRequest) < 0 && !allowExpansion) {
		return "", errors.New("source/target sizes not compatible")
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

func (r *DatavolumeReconciler) getCloneStrategy() (cdiv1.CDICloneStrategy, error) {
	cr, err := GetActiveCDI(r.client)
	if err != nil {
		return cdiv1.CloneStrategySnapshot, err
	}

	if cr == nil {
		return cdiv1.CloneStrategySnapshot, fmt.Errorf("no active CDI")
	}

	if cr.Spec.CloneStrategyOverride == nil {
		return cdiv1.CloneStrategySnapshot, nil
	}

	r.log.V(3).Info(fmt.Sprintf("Overriding default clone strategy with %s", *cr.Spec.CloneStrategyOverride))
	return *cr.Spec.CloneStrategyOverride, nil
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
}

func (r *DatavolumeReconciler) updateSmartCloneStatusPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	var dataVolumeCopy = dataVolume.DeepCopy()
	var event DataVolumeEvent

	curPhase := dataVolumeCopy.Status.Phase

	switch phase {
	case cdiv1.CloneScheduled:
		dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneScheduled
		event.message = fmt.Sprintf(MessageCloneScheduled, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.SnapshotForSmartCloneInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.SnapshotForSmartCloneInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = SnapshotForSmartCloneInProgress
		event.message = fmt.Sprintf(MessageSmartCloneInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name)
	case cdiv1.ExpansionInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.ExpansionInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = ExpansionInProgress
		event.message = fmt.Sprintf(MessageExpansionInProgress, dataVolumeCopy.Namespace, dataVolumeCopy.Name)
	case cdiv1.NamespaceTransferInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.NamespaceTransferInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = NamespaceTransferInProgress
		event.message = fmt.Sprintf(MessageNamespaceTransferInProgress, dataVolumeCopy.Namespace, dataVolumeCopy.Name)
	case cdiv1.Succeeded:
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, dataVolume.Spec.Source.PVC.Namespace, dataVolume.Spec.Source.PVC.Name, dataVolume.Namespace, dataVolume.Name)
	}

	r.updateConditions(dataVolumeCopy, pvc)

	addAnnotation(dataVolumeCopy, annCloneType, "snapshot")

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
	result := reconcile.Result{}

	curPhase := dataVolumeCopy.Status.Phase
	if pvc != nil {
		storageClassBindingMode, err := r.getStorageClassBindingMode(pvc.Spec.StorageClassName)
		if err != nil {
			return reconcile.Result{}, err
		}

		// the following check is for a case where the request is to create a blank disk for a block device.
		// in that case, we do not create a pod as there is no need to create a blank image.
		// instead, we just mark the DV phase as 'Succeeded' so any consumer will be able to use it.
		phase := pvc.Annotations[AnnPodPhase]
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

	if dataVolumeCopy.Spec.Source.PVC != nil {
		// XXX should probably be is status
		addAnnotation(dataVolumeCopy, annCloneType, "network")
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
			if common.ErrConnectionRefused(err) {
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
func (r *DatavolumeReconciler) newPersistentVolumeClaim(dataVolume *cdiv1.DataVolume, targetPvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		"app": "containerized-data-importer",
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
		if dataVolume.Spec.Source.S3.CertConfigMap != "" {
			annotations[AnnCertConfigMap] = dataVolume.Spec.Source.S3.CertConfigMap
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
	if dataVolume.Spec.PriorityClassName != "" {
		annotations[AnnPriorityClassName] = dataVolume.Spec.PriorityClassName
	}
	annotations[AnnPreallocationRequested] = strconv.FormatBool(GetPreallocation(r.client, dataVolume))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   dataVolume.Namespace,
			Name:        dataVolume.Name,
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

// RenderPvcSpec creates a new PVC Spec based on either the dv.spec.pvc or dv.spec.storage section
func RenderPvcSpec(client client.Client, recorder record.EventRecorder, log logr.Logger, dv *cdiv1.DataVolume) (*corev1.PersistentVolumeClaimSpec, error) {
	if dv.Spec.PVC != nil {
		return dv.Spec.PVC, nil
	}

	if dv.Spec.Storage != nil {
		return pvcFromStorage(client, recorder, log, dv)
	}

	return nil, errors.Errorf("datavolume one of {pvc, storage} field is required")
}

func pvcFromStorage(client client.Client, recorder record.EventRecorder, log logr.Logger, dv *cdiv1.DataVolume) (*corev1.PersistentVolumeClaimSpec, error) {
	storage := dv.Spec.Storage
	pvcSpec := copyStorageAsPvc(log, storage)

	storageClass, err := GetStorageClassByName(client, storage.StorageClassName)
	if err != nil {
		return nil, err
	}

	if storageClass == nil {
		// Not even default storageClass on the cluster, cannot apply the defaults, verify spec is ok
		if len(pvcSpec.AccessModes) == 0 {
			log.V(1).Info("Cannot set accessMode for new pvc", "namespace", dv.Namespace, "name", dv.Name)
			recorder.Eventf(dv, corev1.EventTypeWarning, ErrClaimNotValid, "DataVolume.storage spec is missing accessMode and no storageClass to choose profile")
			return nil, errors.Errorf("DataVolume spec is missing accessMode")
		}

		return pvcSpec, nil
	}

	// given storageClass we can apply defaults if needed
	if len(pvcSpec.AccessModes) == 0 {
		accessModes, err := getDefaultAccessModes(client, storageClass)
		if err != nil {
			log.V(1).Info("Cannot set accessMode for new pvc", "namespace", dv.Namespace, "name", dv.Name)
			recorder.Eventf(dv, corev1.EventTypeWarning, ErrClaimNotValid,
				fmt.Sprintf("DataVolume.storage spec is missing accessMode and cannot get access mode from StorageProfile %s", getName(storageClass)))
			return nil, err
		}
		pvcSpec.AccessModes = append(pvcSpec.AccessModes, accessModes...)
	}
	if pvcSpec.VolumeMode == nil || *pvcSpec.VolumeMode == "" {
		volumeMode, err := getDefaultVolumeMode(client, storageClass)
		if err != nil {
			return nil, err
		}
		pvcSpec.VolumeMode = volumeMode
	}

	requestedVolumeSize, err := volumeSize(client, storage, pvcSpec.VolumeMode)
	if err != nil {
		return nil, err
	}
	if pvcSpec.Resources.Requests == nil {
		pvcSpec.Resources.Requests = corev1.ResourceList{}
	}
	pvcSpec.Resources.Requests[corev1.ResourceStorage] = *requestedVolumeSize

	return pvcSpec, nil
}

func copyStorageAsPvc(log logr.Logger, storage *cdiv1.StorageSpec) *corev1.PersistentVolumeClaimSpec {
	input := storage.DeepCopy()
	log.V(1).Info("Cannot set accessMode for new pvc", "storage", storage)
	pvcSpec := &corev1.PersistentVolumeClaimSpec{
		AccessModes:      input.AccessModes,
		Selector:         input.Selector,
		Resources:        input.Resources,
		VolumeName:       input.VolumeName,
		StorageClassName: input.StorageClassName,
		VolumeMode:       input.VolumeMode,
		DataSource:       input.DataSource,
	}

	return pvcSpec
}

func volumeSize(c client.Client, storage *cdiv1.StorageSpec, volumeMode *corev1.PersistentVolumeMode) (*resource.Quantity, error) {
	// resources.requests[storage] - just copy it to pvc,
	requestedSize, found := storage.Resources.Requests[corev1.ResourceStorage]
	if !found {
		return nil, errors.Errorf("Datavolume Spec is not valid - missing storage size")
	}

	// disk or image size, inflate it with overhead
	if resolveVolumeMode(volumeMode) == corev1.PersistentVolumeFilesystem {
		fsOverhead, err := GetFilesystemOverheadForStorageClass(c, storage.StorageClassName)
		if err != nil {
			return nil, err
		}
		fsOverheadFloat, _ := strconv.ParseFloat(string(fsOverhead), 64)
		requiredSpace := GetRequiredSpace(fsOverheadFloat, requestedSize.Value())

		return resource.NewScaledQuantity(requiredSpace, 0), nil
	}

	return &requestedSize, nil
}

func getName(storageClass *storagev1.StorageClass) string {
	if storageClass != nil {
		return storageClass.Name
	}
	return ""
}

func getDefaultVolumeMode(c client.Client, storageClass *storagev1.StorageClass) (*corev1.PersistentVolumeMode, error) {
	if storageClass == nil {
		// fallback to k8s defaults
		return nil, nil
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get StorageProfile")
	}
	if len(storageProfile.Status.ClaimPropertySets) > 0 {
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		return volumeMode, nil
	}

	// since volumeMode is optional - > gracefully fallback to k8s defaults,
	return nil, nil
}

func getDefaultAccessModes(c client.Client, storageClass *storagev1.StorageClass) ([]corev1.PersistentVolumeAccessMode, error) {
	if storageClass == nil {
		return nil, errors.Errorf("no accessMode defined on DV, no StorageProfile ")
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, errors.Wrap(err, "no accessMode defined on DV, cannot get StorageProfile")
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		return accessModes, nil
	}

	// no accessMode configured on storageProfile
	return nil, errors.Errorf("no accessMode defined on StorageProfile for %s StorageClass", storageClass.Name)
}

// GetRequiredSpace calculates space required taking file system overhead into account
func GetRequiredSpace(filesystemOverhead float64, requestedSpace int64) int64 {
	blockSize := int64(512)
	// count overhead as a percentage of the whole/new size
	spaceWithOverhead := int64(float64(requestedSpace) / (1 - filesystemOverhead))
	// qemu-img will round up, making us use more than the usable space.
	// This later conflicts with image size validation.
	qemuImgCorrection := util.RoundUp(spaceWithOverhead, blockSize)
	return qemuImgCorrection
}
