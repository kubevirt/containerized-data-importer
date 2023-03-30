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

	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type cloneStrategy int

// Possible clone strategies, including default special value NoClone
const (
	NoClone cloneStrategy = iota
	HostAssistedClone
	SmartClone
	CsiClone
)

const pvcCloneControllerName = "datavolume-pvc-clone-controller"

// ErrInvalidTermMsg reports that the termination message from the size-detection pod doesn't exists or is not a valid quantity
var ErrInvalidTermMsg = fmt.Errorf("The termination message from the size-detection pod is not-valid")

// PvcCloneReconciler members
type PvcCloneReconciler struct {
	CloneReconcilerBase
	sccs controllerStarter
}

// NewPvcCloneController creates a new instance of the datavolume clone controller
func NewPvcCloneController(
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
	reconciler := &PvcCloneReconciler{
		CloneReconcilerBase: CloneReconcilerBase{
			ReconcilerBase: ReconcilerBase{
				client:          client,
				scheme:          mgr.GetScheme(),
				log:             log.WithName(pvcCloneControllerName),
				featureGates:    featuregates.NewFeatureGates(client),
				recorder:        mgr.GetEventRecorderFor(pvcCloneControllerName),
				installerLabels: installerLabels,
			},
			clonerImage:    clonerImage,
			importerImage:  importerImage,
			pullPolicy:     pullPolicy,
			tokenValidator: cc.NewCloneTokenValidator(common.CloneTokenIssuer, tokenPublicKey),
			// for long term tokens to handle cross namespace dumb clones
			tokenGenerator: newLongTermCloneTokenGenerator(tokenPrivateKey),
		},
		sccs: sccs,
	}
	reconciler.Reconciler = reconciler

	dataVolumeCloneController, err := controller.New(pvcCloneControllerName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err = addDataVolumeCloneControllerWatches(mgr, dataVolumeCloneController); err != nil {
		return nil, err
	}

	if err = mgr.Add(sccs); err != nil {
		return nil, err
	}
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
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumePvcClone); err != nil {
		return err
	}

	// Watch to reconcile clones created without source
	if err := addCloneWithoutSourceWatch(mgr, datavolumeController, &corev1.PersistentVolumeClaim{}, "spec.source.pvc"); err != nil {
		return err
	}

	if err := addDataSourceWatch(mgr, datavolumeController); err != nil {
		return err
	}

	return nil
}

func addDataSourceWatch(mgr manager.Manager, c controller.Controller) error {
	const dvDataSourceField = "datasource"

	getKey := func(namespace, name string) string {
		return namespace + "/" + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, dvDataSourceField, func(obj client.Object) []string {
		if sourceRef := obj.(*cdiv1.DataVolume).Spec.SourceRef; sourceRef != nil && sourceRef.Kind == cdiv1.DataVolumeDataSource {
			ns := obj.GetNamespace()
			if sourceRef.Namespace != nil && *sourceRef.Namespace != "" {
				ns = *sourceRef.Namespace
			}
			return []string{getKey(ns, sourceRef.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	mapToDataVolume := func(obj client.Object) (reqs []reconcile.Request) {
		var dvs cdiv1.DataVolumeList
		matchingFields := client.MatchingFields{dvDataSourceField: getKey(obj.GetNamespace(), obj.GetName())}
		if err := mgr.GetClient().List(context.TODO(), &dvs, matchingFields); err != nil {
			c.GetLogger().Error(err, "Unable to list DataVolumes", "matchingFields", matchingFields)
			return
		}
		for _, dv := range dvs.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}})
		}
		return
	}

	if err := c.Watch(&source.Kind{Type: &cdiv1.DataSource{}},
		handler.EnqueueRequestsFromMapFunc(mapToDataVolume),
	); err != nil {
		return err
	}

	return nil
}

// Reconcile loop for the clone data volumes
func (r *PvcCloneReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, req, r)
}

func (r *PvcCloneReconciler) prepare(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}
	if dv.Status.Phase == cdiv1.Succeeded {
		if err := r.cleanup(syncState); err != nil {
			return err
		}
	}
	return nil
}

func (r *PvcCloneReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
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
	pvc.Annotations[cc.AnnCloneToken] = token
	pvc.Annotations[cc.AnnCloneRequest] = sourceNamespace + "/" + dataVolume.Spec.Source.PVC.Name
	return nil
}

func (r *PvcCloneReconciler) sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error) {
	syncState, err := r.syncClone(log, req)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState.dvSyncResult, err
}

func (r *PvcCloneReconciler) syncClone(log logr.Logger, req reconcile.Request) (dvSyncState, error) {
	syncRes, syncErr := r.syncCommon(log, req, r.cleanup, r.prepare)
	if syncErr != nil || syncRes.result != nil {
		return syncRes, syncErr
	}

	pvc := syncRes.pvc
	pvcSpec := syncRes.pvcSpec
	datavolume := syncRes.dvMutated
	transferName := getTransferName(datavolume)

	// Get the most appropiate clone strategy
	selectedCloneStrategy, err := r.selectCloneStrategy(datavolume, pvcSpec)
	if err != nil {
		return syncRes, err
	}
	if selectedCloneStrategy != NoClone {
		cc.AddAnnotation(datavolume, annCloneType, cloneStrategyToCloneType(selectedCloneStrategy))
	}

	pvcPopulated := pvcIsPopulated(pvc, datavolume)
	staticProvisionPending := checkStaticProvisionPending(pvc, datavolume)
	prePopulated := dvIsPrePopulated(datavolume)

	if pvcPopulated || prePopulated || staticProvisionPending {
		return syncRes, nil
	}

	// Check if source PVC exists and do proper validation before attempting to clone
	if done, err := r.validateCloneAndSourcePVC(&syncRes, log); err != nil {
		return syncRes, err
	} else if !done {
		return syncRes, nil
	}

	if selectedCloneStrategy == SmartClone {
		r.sccs.StartController()
	}

	// If the target's size is not specified, we can extract that value from the source PVC
	targetRequest, hasTargetRequest := pvcSpec.Resources.Requests[corev1.ResourceStorage]
	if !hasTargetRequest || targetRequest.IsZero() {
		done, err := r.detectCloneSize(&syncRes, selectedCloneStrategy)
		if err != nil {
			return syncRes, err
		} else if !done {
			// Check if the source PVC is ready to be cloned
			if readyToClone, err := r.isSourceReadyToClone(datavolume, selectedCloneStrategy); err != nil {
				return syncRes, err
			} else if !readyToClone {
				if syncRes.result == nil {
					syncRes.result = &reconcile.Result{}
				}
				syncRes.result.Requeue = true
				return syncRes,
					r.syncCloneStatusPhase(&syncRes, cdiv1.CloneScheduled, nil)
			}
			return syncRes, nil
		}
	}

	if pvc == nil {
		if selectedCloneStrategy == SmartClone {
			snapshotClassName, err := r.getSnapshotClassForSmartClone(datavolume, pvcSpec)
			if err != nil {
				return syncRes, err
			}
			res, err := r.reconcileSmartClonePvc(log, &syncRes, transferName, snapshotClassName)
			syncRes.result = &res
			return syncRes, err
		}
		if selectedCloneStrategy == CsiClone {
			csiDriverAvailable, err := r.storageClassCSIDriverExists(pvcSpec.StorageClassName)
			if err != nil && !k8serrors.IsNotFound(err) {
				return syncRes, err
			}
			if !csiDriverAvailable {
				// err csi clone not possible
				storageClass, err := cc.GetStorageClassByName(r.client, pvcSpec.StorageClassName)
				if err != nil {
					return syncRes, err
				}
				noCsiDriverMsg := "CSI Clone configured, failed to look for CSIDriver - target storage class could not be found"
				if storageClass != nil {
					noCsiDriverMsg = fmt.Sprintf("CSI Clone configured, but no CSIDriver available for %s", storageClass.Name)
				}
				return syncRes,
					r.syncDataVolumeStatusPhaseWithEvent(&syncRes, cdiv1.CloneScheduled, pvc,
						Event{
							eventType: corev1.EventTypeWarning,
							reason:    ErrUnableToClone,
							message:   noCsiDriverMsg,
						})
			}

			res, err := r.reconcileCsiClonePvc(log, &syncRes, transferName)
			syncRes.result = &res
			return syncRes, err
		}

		newPvc, err := r.createPvcForDatavolume(datavolume, pvcSpec, r.updateAnnotations)
		if err != nil {
			if cc.ErrQuotaExceeded(err) {
				syncErr = r.syncDataVolumeStatusPhaseWithEvent(&syncRes, cdiv1.Pending, nil,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
						message:   err.Error(),
					})
				if syncErr != nil {
					log.Error(syncErr, "failed to sync DataVolume status with event")
				}
			}
			return syncRes, err
		}
		pvc = newPvc
	}

	shouldBeMarkedWaitForFirstConsumer, err := r.shouldBeMarkedWaitForFirstConsumer(pvc)
	if err != nil {
		return syncRes, err
	}

	switch selectedCloneStrategy {
	case HostAssistedClone:
		if err := r.ensureExtendedToken(pvc); err != nil {
			return syncRes, err
		}
	case CsiClone:
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if err := r.setCloneOfOnPvc(pvc); err != nil {
				return syncRes, err
			}
		case corev1.ClaimPending:
			r.log.V(3).Info("ClaimPending CSIClone")
			if !shouldBeMarkedWaitForFirstConsumer {
				return syncRes, r.syncCloneStatusPhase(&syncRes, cdiv1.CSICloneInProgress, pvc)
			}
		case corev1.ClaimLost:
			return syncRes,
				r.syncDataVolumeStatusPhaseWithEvent(&syncRes, cdiv1.Failed, pvc,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    ErrClaimLost,
						message:   fmt.Sprintf(MessageErrClaimLost, pvc.Name),
					})
		}
		fallthrough
	case SmartClone:
		if !shouldBeMarkedWaitForFirstConsumer {
			res, err := r.finishClone(log, &syncRes, transferName)
			syncRes.result = &res
			return syncRes, err

		}
	}

	return syncRes, syncErr
}

func (r *PvcCloneReconciler) selectCloneStrategy(datavolume *cdiv1.DataVolume, pvcSpec *corev1.PersistentVolumeClaimSpec) (cloneStrategy, error) {
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
	if bindingMode != nil && *bindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
		waitForFirstConsumerEnabled, err := cc.IsWaitForFirstConsumerEnabled(datavolume, r.featureGates)
		if err != nil {
			return NoClone, err
		}
		if !waitForFirstConsumerEnabled {
			return HostAssistedClone, nil
		}
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

func (r *PvcCloneReconciler) reconcileCsiClonePvc(log logr.Logger,
	syncRes *dvSyncState,
	transferName string) (reconcile.Result, error) {

	log = log.WithName("reconcileCsiClonePvc")
	datavolume := syncRes.dvMutated
	pvcSpec := syncRes.pvcSpec
	pvcName := datavolume.Name

	if isCrossNamespaceClone(datavolume) {
		pvcName = transferName

		result, err := r.doCrossNamespaceClone(log, syncRes, pvcName, datavolume.Spec.Source.PVC.Namespace, false, CsiClone)
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
			r.syncCloneStatusPhase(syncRes, cdiv1.CloneScheduled, nil)
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
				syncErr := r.syncDataVolumeStatusPhaseWithEvent(syncRes, cdiv1.Pending, nil,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
						message:   err.Error(),
					})
				if syncErr != nil {
					log.Error(syncErr, "failed to sync DataVolume status with event")
				}
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

	return reconcile.Result{}, r.syncCloneStatusPhase(syncRes, cdiv1.CSICloneInProgress, nil)
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

func (r *PvcCloneReconciler) reconcileSmartClonePvc(log logr.Logger,
	syncState *dvSyncState,
	transferName string,
	snapshotClassName string) (reconcile.Result, error) {

	datavolume := syncState.dvMutated
	pvcName := datavolume.Name

	if isCrossNamespaceClone(datavolume) {
		pvcName = transferName
		result, err := r.doCrossNamespaceClone(log, syncState, pvcName, datavolume.Spec.Source.PVC.Namespace, true, SmartClone)
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
				r.syncCloneStatusPhase(syncState, cdiv1.CloneScheduled, nil)
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

	return reconcile.Result{}, r.syncCloneStatusPhase(syncState, cdiv1.SnapshotForSmartCloneInProgress, nil)
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

// Verify that the source PVC has been completely populated.
func (r *PvcCloneReconciler) isSourcePVCPopulated(dv *cdiv1.DataVolume) (bool, error) {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.Source.PVC.Name, Namespace: dv.Spec.Source.PVC.Namespace}, sourcePvc); err != nil {
		return false, err
	}
	return cc.IsPopulated(sourcePvc, r.client)
}

func (r *PvcCloneReconciler) sourceInUse(dv *cdiv1.DataVolume, eventReason string) (bool, error) {
	pods, err := cc.GetPodsUsingPVCs(r.client, dv.Spec.Source.PVC.Namespace, sets.New[string](dv.Spec.Source.PVC.Name), false)
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

func (r *PvcCloneReconciler) cleanup(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	r.log.V(3).Info("Cleanup initiated in dv PVC clone controller")

	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}

	if isCrossNamespaceClone(dv) {
		if err := r.cleanupTransfer(dv); err != nil {
			return err
		}
	}

	return nil
}

func (r *PvcCloneReconciler) getSnapshotClassForSmartClone(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (string, error) {
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
func (r *PvcCloneReconciler) advancedClonePossible(dataVolume *cdiv1.DataVolume, targetStorageSpec *corev1.PersistentVolumeClaimSpec) (bool, error) {
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

	return r.validateAdvancedCloneSizeCompatible(sourcePvc, targetStorageSpec)
}

func (r *PvcCloneReconciler) validateSameStorageClass(
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

func (r *PvcCloneReconciler) validateSameVolumeMode(
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

func (r *PvcCloneReconciler) validateAdvancedCloneSizeCompatible(
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

func (r *PvcCloneReconciler) getCloneStrategy(dataVolume *cdiv1.DataVolume) (*cdiv1.CDICloneStrategy, error) {
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

func (r *PvcCloneReconciler) findSourcePvc(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
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

func (r *PvcCloneReconciler) getGlobalCloneStrategyOverride() (*cdiv1.CDICloneStrategy, error) {
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
func (r *PvcCloneReconciler) newVolumeClonePVC(dv *cdiv1.DataVolume,
	sourcePvc *corev1.PersistentVolumeClaim,
	targetPvcSpec *corev1.PersistentVolumeClaimSpec,
	pvcName string) (*corev1.PersistentVolumeClaim, error) {

	// Override name - might be temporary pod when in transfer
	pvcNamespace := dv.Namespace
	if dv.Spec.Source.PVC.Namespace != "" {
		pvcNamespace = dv.Spec.Source.PVC.Namespace
	}

	pvc, err := r.newPersistentVolumeClaim(dv, targetPvcSpec, pvcNamespace, pvcName, r.updateAnnotations)
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

func (r *PvcCloneReconciler) getPreferredCloneStrategyForStorageClass(storageClass *storagev1.StorageClass) (*cdiv1.CDICloneStrategy, error) {
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
func (r *PvcCloneReconciler) validateCloneAndSourcePVC(syncState *dvSyncState, log logr.Logger) (bool, error) {
	datavolume := syncState.dvMutated
	sourcePvc, err := r.findSourcePvc(datavolume)
	if err != nil {
		// Clone without source
		if k8serrors.IsNotFound(err) {
			syncErr := r.syncDataVolumeStatusPhaseWithEvent(syncState, datavolume.Status.Phase, nil,
				Event{
					eventType: corev1.EventTypeWarning,
					reason:    CloneWithoutSource,
					message:   fmt.Sprintf(MessageCloneWithoutSource, "pvc", datavolume.Spec.Source.PVC.Name),
				})
			if syncErr != nil {
				log.Error(syncErr, "failed to sync DataVolume status with event")
			}
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
func (r *PvcCloneReconciler) isSourceReadyToClone(
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
func (r *PvcCloneReconciler) detectCloneSize(syncState *dvSyncState, cloneType cloneStrategy) (bool, error) {
	var targetSize int64
	sourcePvc, err := r.findSourcePvc(syncState.dvMutated)
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
			targetSize, err = r.getSizeFromPod(syncState.pvc, sourcePvc, syncState.dvMutated)
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
		syncState.dvMutated.Annotations[cc.AnnPermissiveClone] = "true"
	}

	// Parse size into a 'Quantity' struct and, if needed, inflate it with filesystem overhead
	targetCapacity, err := inflateSizeWithOverhead(r.client, targetSize, syncState.pvcSpec)
	if err != nil {
		return false, err
	}

	syncState.pvcSpec.Resources.Requests[corev1.ResourceStorage] = targetCapacity
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
func (r *PvcCloneReconciler) getSizeFromPod(targetPvc, sourcePvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (int64, error) {
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
func (r *PvcCloneReconciler) getOrCreateSizeDetectionPod(
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
func (r *PvcCloneReconciler) makeSizeDetectionPodSpec(
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
	imagePullSecrets, err := cc.GetImagePullSecrets(r.client)
	if err != nil {
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
			ImagePullSecrets:  imagePullSecrets,
		},
	}

	if sourcePvc.Namespace == dv.Namespace {
		pod.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(dv, schema.GroupVersionKind{
				Group:   cdiv1.SchemeGroupVersion.Group,
				Version: cdiv1.SchemeGroupVersion.Version,
				Kind:    "DataVolume",
			}),
		}
	} else {
		if err := setAnnOwnedByDataVolume(pod, dv); err != nil {
			return nil
		}
		pod.Annotations[cc.AnnOwnerUID] = string(dv.UID)
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
	}
}

// makeSizeDetectionContainerSpec creates and returns the size-detection pod's Container spec
func (r *PvcCloneReconciler) makeSizeDetectionContainerSpec(volName string) *corev1.Container {
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
func (r *PvcCloneReconciler) handleSizeDetectionError(pod *corev1.Pod, dv *cdiv1.DataVolume, sourcePvc *corev1.PersistentVolumeClaim) error {
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
func (r *PvcCloneReconciler) updateClonePVCAnnotations(sourcePvc *corev1.PersistentVolumeClaim, virtualSize string) error {
	currCapacity := sourcePvc.Status.Capacity
	sourcePvc.Annotations[AnnVirtualImageSize] = virtualSize
	sourcePvc.Annotations[AnnSourceCapacity] = currCapacity.Storage().String()
	return r.client.Update(context.TODO(), sourcePvc)
}

// sizeDetectionPodName returns the name of the size-detection pod accoding to the source PVC's UID
func sizeDetectionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("size-detection-%s", pvc.UID)
}

// isPodComplete returns true if a pod is in 'Succeeded' phase, false if not
func isPodComplete(pod *v1.Pod) bool {
	return pod != nil && pod.Status.Phase == v1.PodSucceeded
}
