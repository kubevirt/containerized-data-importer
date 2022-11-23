/*
Copyright 2022 The CDI Authors.

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
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ErrUnableToClone provides a const to indicate some errors are blocking the clone
	ErrUnableToClone = "ErrUnableToClone"

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

	crossNamespaceFinalizer = "cdi.kubevirt.io/dataVolumeFinalizer"

	annReadyForTransfer = "cdi.kubevirt.io/readyForTransfer"

	annCloneType = "cdi.kubevirt.io/cloneType"

	cloneControllerName = "datavolume-clone-controller"
)

type cloneStrategy int

// Possible clone strategies, including default special value NoClone
const (
	NoClone cloneStrategy = iota
	HostAssistedClone
	SmartClone
	CsiClone
)

// ErrInvalidTermMsg reports that the termination message from the size-detection pod doesn't exists or is not a valid quantity
var ErrInvalidTermMsg = fmt.Errorf("The termination message from the size-detection pod is not-valid")

// CloneReconciler members
type CloneReconciler struct {
	ReconcilerBase
	clonerImage    string
	importerImage  string
	pullPolicy     string
	tokenValidator token.Validator
	tokenGenerator token.Generator
	sccs           controllerStarter
}

// NewCloneController creates a new instance of the datavolume clone controller
func NewCloneController(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	clonerImage string,
	importerImage string,
	pullPolicy string,
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
	reconciler := &CloneReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(cloneControllerName),
			featureGates:    featuregates.NewFeatureGates(client),
			recorder:        mgr.GetEventRecorderFor(cloneControllerName),
			installerLabels: installerLabels,
		},
		clonerImage:    clonerImage,
		importerImage:  importerImage,
		pullPolicy:     pullPolicy,
		tokenValidator: cc.NewCloneTokenValidator(common.CloneTokenIssuer, tokenPublicKey),
		// for long term tokens to handle cross namespace dumb clones
		tokenGenerator: newLongTermCloneTokenGenerator(tokenPrivateKey),
		sccs:           sccs,
	}
	reconciler.Reconciler = reconciler

	dataVolumeCloneController, err := controller.New(cloneControllerName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := addDataVolumeCloneControllerWatches(mgr, dataVolumeCloneController); err != nil {
		return nil, err
	}

	mgr.Add(sccs)
	return dataVolumeCloneController, nil
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

func addDataVolumeCloneControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeClone); err != nil {
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
		//FIXME: what about sourceRef?
		if source := obj.(*cdiv1.DataVolume).Spec.Source; source != nil {
			if pvc := source.PVC; pvc != nil {
				ns := cc.GetNamespace(pvc.Namespace, obj.GetNamespace())
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
			if getDataVolumeOp(&dv) == dataVolumeClone {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}})
			}
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

// Reconcile loop for the clone data volumes
// Unlike the base class sync call, for clone we pass cleanup and prepare
func (r CloneReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("DataVolume", req.NamespacedName)
	res, syncErr := r.sync(log, req, r.cleanup, r.prepare)
	return r.updateStatus(log, res, syncErr)
}

func (r CloneReconciler) prepare(dv *cdiv1.DataVolume) error {
	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}
	if isCrossNamespaceClone(dv) && dv.Status.Phase == cdiv1.Succeeded {
		if err := r.cleanup(dv); err != nil {
			return err
		}
	}
	return nil
}

func (r CloneReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, annotations map[string]string) error {
	if dataVolume.Spec.Source.PVC == nil {
		return errors.Errorf("no source set for clone datavolume")
	}
	sourceNamespace := dataVolume.Spec.Source.PVC.Namespace
	if sourceNamespace == "" {
		sourceNamespace = dataVolume.Namespace
	}
	token, ok := dataVolume.Annotations[cc.AnnCloneToken]
	if !ok {
		return errors.Errorf("no clone token")
	}
	annotations[cc.AnnCloneToken] = token
	annotations[cc.AnnCloneRequest] = sourceNamespace + "/" + dataVolume.Spec.Source.PVC.Name
	return nil
}

func (r CloneReconciler) updateStatus(log logr.Logger, syncRes dataVolumeSyncResult, syncErr error) (reconcile.Result, error) {
	if syncErr != nil {
		return reconcile.Result{}, syncErr
	}
	if syncRes.res != nil {
		return *syncRes.res, nil
	}
	//FIXME: pass syncRes instead of args
	return r.reconcileClone(log, syncRes.dv, syncRes.pvc, syncRes.pvcSpec, getTransferName(syncRes.dv))
}

func getTransferName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("cdi-tmp-%s", dv.UID)
}

func (r *CloneReconciler) reconcileClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string) (reconcile.Result, error) {

	// Get the most appropiate clone strategy
	selectedCloneStrategy, err := r.selectCloneStrategy(datavolume, pvcSpec)
	if err != nil {
		return reconcile.Result{}, err
	}

	setCloneType := cloneTypeModifier(selectedCloneStrategy)

	pvcPopulated := pvcIsPopulated(pvc, datavolume)
	_, prePopulated := datavolume.Annotations[cc.AnnPrePopulated]

	if pvcPopulated || prePopulated {
		return r.reconcileDataVolumeStatus(datavolume, pvc, setCloneType, r.updateStatusPhase)
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
			snapshotClassName, err := r.getSnapshotClassForSmartClone(datavolume, pvcSpec)
			if err != nil {
				return reconcile.Result{}, err
			}
			return r.reconcileSmartClonePvc(log, datavolume, pvcSpec, transferName, snapshotClassName)
		}
		if selectedCloneStrategy == CsiClone {
			csiDriverAvailable, err := r.storageClassCSIDriverExists(pvcSpec.StorageClassName)
			if err != nil && !k8serrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
			if !csiDriverAvailable {
				// err csi clone not possible
				storageClass, err := cc.GetStorageClassByName(r.client, pvcSpec.StorageClassName)
				if err != nil {
					return reconcile.Result{}, err
				}
				noCsiDriverMsg := "CSI Clone configured, failed to look for CSIDriver - target storage class could not be found"
				if storageClass != nil {
					noCsiDriverMsg = fmt.Sprintf("CSI Clone configured, but no CSIDriver available for %s", storageClass.Name)
				}
				return reconcile.Result{},
					r.updateDataVolumeStatusPhaseWithEvent(cdiv1.CloneScheduled, datavolume, pvc, setCloneType,
						Event{
							eventType: corev1.EventTypeWarning,
							reason:    ErrUnableToClone,
							message:   noCsiDriverMsg,
						})
			}

			return r.reconcileCsiClonePvc(log, datavolume, pvcSpec, transferName)
		}

		newPvc, err := r.createPvcForDatavolume(datavolume, pvcSpec, nil)
		if err != nil {
			if cc.ErrQuotaExceeded(err) {
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, datavolume, nil, setCloneType,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
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
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Failed, datavolume, pvc, setCloneType,
					Event{
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

	return r.reconcileDataVolumeStatus(datavolume, pvc, setCloneType, r.updateStatusPhase)
}

func (r CloneReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		_, ok = pvc.Annotations[cc.AnnCloneRequest]
		if !ok || pvc.Status.Phase != corev1.ClaimBound || pvcIsPopulated(pvc, dataVolumeCopy) {
			return nil
		}
		dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
	}
	if !ok {
		return nil
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
	return nil
}

func (r *CloneReconciler) ensureExtendedToken(pvc *corev1.PersistentVolumeClaim) error {
	_, ok := pvc.Annotations[cc.AnnExtendedCloneToken]
	if ok {
		return nil
	}

	token, ok := pvc.Annotations[cc.AnnCloneToken]
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

	pvc.Annotations[cc.AnnExtendedCloneToken] = newToken

	if err := r.updatePVC(pvc); err != nil {
		return err
	}

	return nil
}

func (r *CloneReconciler) selectCloneStrategy(datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec) (cloneStrategy, error) {
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

func (r *CloneReconciler) reconcileCsiClonePvc(log logr.Logger,
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
			if cc.ErrQuotaExceeded(err) {
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, datavolume, nil, cloneTypeModifier(CsiClone),
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
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
func (r *CloneReconciler) finishClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	transferName string,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	//DO Nothing, not yet ready
	if pvc.Annotations[cc.AnnCloneOf] != "true" {
		return reconcile.Result{}, nil
	}

	// expand for non-namespace case
	return r.expandPvcAfterClone(log, datavolume, pvc, pvcSpec, selectedCloneStrategy)
}

func (r *CloneReconciler) setCloneOfOnPvc(pvc *corev1.PersistentVolumeClaim) error {
	if v, ok := pvc.Annotations[cc.AnnCloneOf]; !ok || v != "true" {
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations[cc.AnnCloneOf] = "true"

		return r.updatePVC(pvc)
	}

	return nil
}

func (r *CloneReconciler) expandPvcAfterClone(log logr.Logger,
	datavolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	return r.expandPvcAfterCloneFunc(log, datavolume, pvc, pvcSpec, false, selectedCloneStrategy, cdiv1.Succeeded)
}

// temporary pvc is used when the clone src and tgt are in two distinct namespaces
func (r *CloneReconciler) expandTmpPvcAfterClone(
	log logr.Logger,
	datavolume *cdiv1.DataVolume,
	tmpPVC *corev1.PersistentVolumeClaim,
	pvcSpec *corev1.PersistentVolumeClaimSpec,
	selectedCloneStrategy cloneStrategy) (reconcile.Result, error) {

	return r.expandPvcAfterCloneFunc(log, datavolume, tmpPVC, pvcSpec, true, selectedCloneStrategy, cdiv1.NamespaceTransferInProgress)
}

func (r *CloneReconciler) expandPvcAfterCloneFunc(
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
		pvc.Annotations[cc.AnnPopulatedFor] = datavolume.Name
		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, r.updateCloneStatusPhase(nextPhase, datavolume, pvc, selectedCloneStrategy)
}

func cloneTypeModifier(selectedCloneStrategy cloneStrategy) modifyFunc {
	return func(datavolume *cdiv1.DataVolume) {
		if selectedCloneStrategy != NoClone {
			cc.AddAnnotation(datavolume, annCloneType, cloneStrategyToCloneType(selectedCloneStrategy))
		}
	}
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

func (r *CloneReconciler) reconcileSmartClonePvc(log logr.Logger,
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

func (r *CloneReconciler) doCrossNamespaceClone(log logr.Logger,
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
	} else if tmpPVC.Annotations[cc.AnnCloneOf] == "true" {
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

// Verify that the source PVC has been completely populated.
func (r *CloneReconciler) isSourcePVCPopulated(dv *cdiv1.DataVolume) (bool, error) {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.Source.PVC.Name, Namespace: dv.Spec.Source.PVC.Namespace}, sourcePvc); err != nil {
		return false, err
	}
	return cc.IsPopulated(sourcePvc, r.client)
}

func (r *CloneReconciler) sourceInUse(dv *cdiv1.DataVolume, eventReason string) (bool, error) {
	pods, err := cc.GetPodsUsingPVCs(r.client, dv.Spec.Source.PVC.Namespace, sets.NewString(dv.Spec.Source.PVC.Name), false)
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

func (r *CloneReconciler) initTransfer(log logr.Logger, dv *cdiv1.DataVolume, name string) (bool, error) {
	initialized := true

	log.Info("Initializing transfer")

	if !cc.HasFinalizer(dv, crossNamespaceFinalizer) {
		cc.AddFinalizer(dv, crossNamespaceFinalizer)
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

		if err := cc.ValidateCloneTokenDV(r.tokenValidator, dv); err != nil {
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

func (r CloneReconciler) cleanup(dv *cdiv1.DataVolume) error {
	transferName := getTransferName(dv)
	if !cc.HasFinalizer(dv, crossNamespaceFinalizer) {
		return nil
	}

	r.log.V(1).Info("Doing cleanup")

	if dv.DeletionTimestamp != nil && dv.Status.Phase != cdiv1.Succeeded {
		// delete all potential PVCs that may not have owner refs
		namespaces := []string{dv.Namespace}
		names := []string{dv.Name}
		if dv.Spec.Source.PVC != nil &&
			dv.Spec.Source.PVC.Namespace != "" &&
			dv.Spec.Source.PVC.Namespace != dv.Namespace {
			namespaces = append(namespaces, dv.Spec.Source.PVC.Namespace)
			names = append(names, transferName)
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
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: transferName}, ot); err != nil {
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
		return fmt.Errorf("waiting for ObjectTransfer %s to delete", transferName)
	}

	cc.RemoveFinalizer(dv, crossNamespaceFinalizer)
	if err := r.updateDataVolume(dv); err != nil {
		return err
	}

	return nil
}

func expansionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return "cdi-expand-" + string(pvc.UID)
}

func (r *CloneReconciler) expand(log logr.Logger,
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
		if podErr := cc.HandleFailedPod(err, podName, pvc, r.recorder, r.client); podErr != nil {
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

func (r *CloneReconciler) createExpansionPod(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, podName string) (*corev1.Pod, error) {
	resourceRequirements, err := cc.GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(r.client)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
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
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: cc.DataVolName,
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
		pod.Spec.Containers[0].VolumeDevices = cc.AddVolumeDevices()
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      cc.DataVolName,
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

	cc.SetRestrictedSecurityContext(&pod.Spec)

	if err := r.client.Create(context.TODO(), pod); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	return pod, nil
}

func (r *CloneReconciler) storageClassCSIDriverExists(storageClassName *string) (bool, error) {
	log := r.log.WithName("getCsiDriverForStorageClass").V(3)

	storageClass, err := cc.GetStorageClassByName(r.client, storageClassName)
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

func (r *CloneReconciler) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (string, error) {
	log := r.log.WithName("getSnapshotClassForSmartClone").V(3)
	// Check if relevant CRDs are available
	if !isCsiCrdsDeployed(r.client, r.log) {
		log.Info("Missing CSI snapshotter CRDs, falling back to host assisted clone")
		return "", nil
	}

	targetPvcStorageClassName := targetStorageSpec.StorageClassName
	targetStorageClass, err := cc.GetStorageClassByName(r.client, targetPvcStorageClassName)
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

// isCsiCrdsDeployed checks whether the CSI snapshotter CRD are deployed
func isCsiCrdsDeployed(c client.Client, log logr.Logger) bool {
	version := "v1"
	vsClass := "volumesnapshotclasses." + snapshotv1.GroupName
	vsContent := "volumesnapshotcontents." + snapshotv1.GroupName
	vs := "volumesnapshots." + snapshotv1.GroupName

	return isCrdDeployed(c, vsClass, version, log) &&
		isCrdDeployed(c, vsContent, version, log) &&
		isCrdDeployed(c, vs, version, log)
}

// isCrdDeployed checks whether a CRD is deployed
func isCrdDeployed(c client.Client, name, version string, log logr.Logger) bool {
	crd := &extv1.CustomResourceDefinition{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: name}, crd)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Info("Error looking up CRD", "crd name", name, "version", version, "error", err)
		}
		return false
	}

	for _, v := range crd.Spec.Versions {
		if v.Name == version && v.Served {
			return true
		}
	}

	return false
}

// Returns true if methods different from HostAssisted are possible,
// both snapshot and csi volume clone share the same basic requirements
func (r *CloneReconciler) advancedClonePossible(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
	log := r.log.WithName("ClonePossible").V(3)

	sourcePvc, err := r.findSourcePvc(dataVolume)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, errors.New("source PVC not found")
		}
		return false, err
	}

	targetStorageClass, err := cc.GetStorageClassByName(r.client, targetStorageSpec.StorageClassName)
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

func (r *CloneReconciler) validateSameStorageClass(
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

func (r *CloneReconciler) validateSameVolumeMode(
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

func (r *CloneReconciler) validateAdvancedCloneSizeCompatible(
	dataVolume *cdiv1.DataVolume,
	sourcePvc *corev1.PersistentVolumeClaim,
	targetStorageSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
	srcStorageClass := &storagev1.StorageClass{}
	if sourcePvc.Spec.StorageClassName == nil {
		return false, fmt.Errorf("Source PVC Storage Class name wasn't populated yet by PVC controller")
	}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *sourcePvc.Spec.StorageClassName}, srcStorageClass); cc.IgnoreNotFound(err) != nil {
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

func (r *CloneReconciler) getCloneStrategy(dataVolume *cdiv1.DataVolume) (*cdiv1.CDICloneStrategy, error) {
	defaultCloneStrategy := cdiv1.CloneStrategySnapshot
	sourcePvc, err := r.findSourcePvc(dataVolume)
	if err != nil {
		return nil, err
	}
	storageClass, err := cc.GetStorageClassByName(r.client, sourcePvc.Spec.StorageClassName)
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

func (r *CloneReconciler) findSourcePvc(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
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

func (r *CloneReconciler) getGlobalCloneStrategyOverride() (*cdiv1.CDICloneStrategy, error) {
	cr, err := cc.GetActiveCDI(r.client)
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

// NewVolumeClonePVC creates a PVC object to be used during CSI volume cloning.
func (r *CloneReconciler) newVolumeClonePVC(dv *cdiv1.DataVolume,
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
	delete(pvc.Annotations, cc.AnnCloneRequest)
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

func (r *CloneReconciler) updateCloneStatusPhase(phase cdiv1.DataVolumePhase,
	dataVolume *cdiv1.DataVolume,
	pvc *corev1.PersistentVolumeClaim,
	selectedCloneStrategy cloneStrategy) error {

	var event Event

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

	return r.updateDataVolumeStatusPhaseWithEvent(phase, dataVolume, pvc, cloneTypeModifier(selectedCloneStrategy), event)
}

// If sourceRef is set, populate spec.Source with data from the DataSource
func (r *CloneReconciler) populateSourceIfSourceRef(dv *cdiv1.DataVolume) error {
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

func (r *CloneReconciler) getPreferredCloneStrategyForStorageClass(storageClass *storagev1.StorageClass) (*cdiv1.CDICloneStrategy, error) {
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

// validateCloneAndSourcePVC checks if the source PVC of a clone exists and does proper validation
func (r *CloneReconciler) validateCloneAndSourcePVC(datavolume *cdiv1.DataVolume) (bool, error) {
	sourcePvc, err := r.findSourcePvc(datavolume)
	if err != nil {
		// Clone without source
		if k8serrors.IsNotFound(err) {
			r.updateDataVolumeStatusPhaseWithEvent(datavolume.Status.Phase, datavolume, nil, cloneTypeModifier(NoClone),
				Event{
					eventType: corev1.EventTypeWarning,
					reason:    CloneWithoutSource,
					message:   fmt.Sprintf(MessageCloneWithoutSource, datavolume.Spec.Source.PVC.Name),
				})
			return false, nil
		}
		return false, err
	}

	err = cc.ValidateClone(sourcePvc, &datavolume.Spec)
	if err != nil {
		r.recorder.Event(datavolume, corev1.EventTypeWarning, CloneValidationFailed, MessageCloneValidationFailed)
		return false, err
	}

	return true, nil
}

// isSourceReadyToClone handles the reconciling process of a clone when the source PVC is not ready
func (r *CloneReconciler) isSourceReadyToClone(
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

// detectCloneSize obtains and assigns the original PVC's size when cloning using an empty storage value
func (r *CloneReconciler) detectCloneSize(
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
		cc.GetVolumeMode(sourcePvc) == corev1.PersistentVolumeFilesystem &&
		cc.GetContentType(sourcePvc) == string(cdiv1.DataVolumeKubeVirt) {
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
		dv.Annotations[cc.AnnPermissiveClone] = "true"
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
func (r *CloneReconciler) getSizeFromPod(targetPvc, sourcePvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (int64, error) {
	// The pod should not be created until the source PVC has finished the import process
	if !cc.IsPVCComplete(sourcePvc) {
		r.recorder.Event(dv, corev1.EventTypeNormal, ImportPVCNotReady, MessageImportPVCNotReady)
		return 0, nil
	}

	pod, err := r.getOrCreateSizeDetectionPod(sourcePvc, dv)
	// Check if pod has failed and, in that case, record an event with the error
	if podErr := cc.HandleFailedPod(err, sizeDetectionPodName(sourcePvc), targetPvc, r.recorder, r.client); podErr != nil {
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
	if cc.ShouldDeletePod(sourcePvc) {
		err = r.client.Delete(context.TODO(), pod)
		if err != nil && !k8serrors.IsNotFound(err) {
			return imgSize, err
		}
	}

	return imgSize, nil
}

// getOrCreateSizeDetectionPod gets the size-detection pod if it already exists/creates it if not
func (r *CloneReconciler) getOrCreateSizeDetectionPod(
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
func (r *CloneReconciler) makeSizeDetectionPodSpec(
	sourcePvc *corev1.PersistentVolumeClaim,
	dv *cdiv1.DataVolume) *corev1.Pod {

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(r.client)
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
			PriorityClassName: cc.GetPriorityClass(sourcePvc),
		},
	}

	cc.SetRestrictedSecurityContext(&pod.Spec)

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
func (r *CloneReconciler) makeSizeDetectionContainerSpec(volName string) *corev1.Container {
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
	}

	// Get and assign container's default resource requirements
	resourceRequirements, err := cc.GetDefaultPodResourceRequirements(r.client)
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
		Name: cc.DataVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
}

// handleSizeDetectionError handles the termination of the size-detection pod in case of error
func (r *CloneReconciler) handleSizeDetectionError(pod *corev1.Pod, dv *cdiv1.DataVolume, sourcePvc *corev1.PersistentVolumeClaim) error {
	var event Event
	var exitCode int

	if pod.Status.ContainerStatuses == nil || pod.Status.ContainerStatuses[0].State.Terminated == nil {
		exitCode = cc.ErrUnknown
	} else {
		exitCode = int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
	}

	// We attempt to delete the pod
	err := r.client.Delete(context.TODO(), pod)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	switch exitCode {
	case cc.ErrBadArguments:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrBadArguments"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case cc.ErrInvalidPath:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrInvalidPath"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case cc.ErrInvalidFile:
		event.eventType = corev1.EventTypeWarning
		event.reason = "ErrInvalidFile"
		event.message = fmt.Sprintf(MessageSizeDetectionPodFailed, event.reason)
	case cc.ErrBadTermFile:
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
func (r *CloneReconciler) updateClonePVCAnnotations(sourcePvc *corev1.PersistentVolumeClaim, virtualSize string) error {
	currCapacity := sourcePvc.Status.Capacity
	sourcePvc.Annotations[AnnVirtualImageSize] = virtualSize
	sourcePvc.Annotations[AnnSourceCapacity] = currCapacity.Storage().String()
	return r.client.Update(context.TODO(), sourcePvc)
}

// sizeDetectionPodName returns the name of the size-detection pod accoding to the source PVC's UID
func sizeDetectionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("size-detection-%s", pvc.UID)
}

func isCrossNamespaceClone(dv *cdiv1.DataVolume) bool {
	if dv.Spec.Source.PVC == nil {
		return false
	}

	return dv.Spec.Source.PVC.Namespace != "" && dv.Spec.Source.PVC.Namespace != dv.Namespace
}

// isPodComplete returns true if a pod is in 'Succeeded' phase, false if not
func isPodComplete(pod *v1.Pod) bool {
	return pod != nil && pod.Status.Phase == v1.PodSucceeded
}
