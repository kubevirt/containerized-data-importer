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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	clone "kubevirt.io/containerized-data-importer/pkg/controller/clone"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
)

const (
	sourceInUseRequeueDuration = 5 * time.Second

	pvcCloneControllerName = "datavolume-pvc-clone-controller"

	volumeCloneSourcePrefix = "volume-clone-source"
)

// ErrInvalidTermMsg reports that the termination message from the size-detection pod doesn't exists or is not a valid quantity
var ErrInvalidTermMsg = fmt.Errorf("the termination message from the size-detection pod is not-valid")

// PvcCloneReconciler members
type PvcCloneReconciler struct {
	CloneReconcilerBase
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
	reconciler := &PvcCloneReconciler{
		CloneReconcilerBase: CloneReconcilerBase{
			ReconcilerBase: ReconcilerBase{
				client:               client,
				scheme:               mgr.GetScheme(),
				log:                  log.WithName(pvcCloneControllerName),
				featureGates:         featuregates.NewFeatureGates(client),
				recorder:             mgr.GetEventRecorderFor(pvcCloneControllerName),
				installerLabels:      installerLabels,
				shouldUpdateProgress: true,
			},
			clonerImage:         clonerImage,
			importerImage:       importerImage,
			pullPolicy:          pullPolicy,
			cloneSourceKind:     "PersistentVolumeClaim",
			shortTokenValidator: cc.NewCloneTokenValidator(common.CloneTokenIssuer, tokenPublicKey),
			longTokenValidator:  cc.NewCloneTokenValidator(common.ExtendedCloneTokenIssuer, tokenPublicKey),
			// for long term tokens to handle cross namespace dumb clones
			tokenGenerator: newLongTermCloneTokenGenerator(tokenPrivateKey),
		},
	}

	dataVolumeCloneController, err := controller.New(pvcCloneControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err = reconciler.addDataVolumeCloneControllerWatches(mgr, dataVolumeCloneController); err != nil {
		return nil, err
	}

	return dataVolumeCloneController, nil
}

func (r *PvcCloneReconciler) addDataVolumeCloneControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumePvcClone); err != nil {
		return err
	}

	// Watch to reconcile clones created without source
	if err := addCloneWithoutSourceWatch(mgr, datavolumeController, &corev1.PersistentVolumeClaim{}, "spec.source.pvc", dataVolumePvcClone); err != nil {
		return err
	}

	if err := addDataSourceWatch(mgr, datavolumeController, "pvcdatasource", dataVolumePvcClone); err != nil {
		return err
	}

	if err := r.addVolumeCloneSourceWatch(mgr, datavolumeController); err != nil {
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
	return nil
}

func (r *PvcCloneReconciler) cleanup(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}

	if dv.DeletionTimestamp == nil && dv.Status.Phase != cdiv1.Succeeded {
		return nil
	}

	r.log.V(3).Info("Cleanup initiated in dv pvc clone controller")

	if err := r.reconcileVolumeCloneSourceCR(syncState); err != nil {
		return err
	}

	if err := r.cleanupSizeDetectionPod(dv, syncState.pvc); err != nil {
		return err
	}

	return nil
}

func (r *PvcCloneReconciler) cleanupSizeDetectionPod(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if pvc != nil && !cc.ShouldDeletePod(pvc) {
		return nil
	}

	nn := types.NamespacedName{Namespace: dv.Spec.Source.PVC.Namespace, Name: dv.Spec.Source.PVC.Name}
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), nn, sourcePvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	pod := &corev1.Pod{
		ObjectMeta: *makeSizeDetectionObjectMeta(sourcePvc),
	}
	if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(pod), pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	ownerRef := metav1.GetControllerOf(pod)
	var hasDataVolumeOwner bool
	var ownerNamespace, ownerName string
	if ownerRef != nil && ownerRef.Kind == "DataVolume" {
		hasDataVolumeOwner = true
		ownerNamespace = pod.GetNamespace()
		ownerName = ownerRef.Name
	} else if hasAnnOwnedByDataVolume(pod) {
		hasDataVolumeOwner = true
		var err error
		ownerNamespace, ownerName, err = getAnnOwnedByDataVolume(pod)
		if err != nil {
			return nil
		}
	}
	if !hasDataVolumeOwner {
		return nil
	}

	if ownerNamespace != dv.Namespace || ownerName != dv.Name {
		return nil
	}
	if _, ok := pod.Labels[common.CDIComponentLabel]; !ok {
		return nil
	}

	if err := r.client.Delete(context.TODO(), pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func addCloneToken(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	// first clear out tokens that may have already been added
	delete(pvc.Annotations, cc.AnnCloneToken)
	delete(pvc.Annotations, cc.AnnExtendedCloneToken)
	if isCrossNamespaceClone(dv) {
		// only want this initially
		// extended token is added later
		token, ok := dv.Annotations[cc.AnnCloneToken]
		if !ok {
			return errors.Errorf("no clone token")
		}
		cc.AddAnnotation(pvc, cc.AnnCloneToken, token)
	}
	return nil
}

func volumeCloneSourceName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("%s-%s", volumeCloneSourcePrefix, dv.UID)
}

func (r *PvcCloneReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.PVC == nil {
		return errors.Errorf("no source set for clone datavolume")
	}
	if err := addCloneToken(dataVolume, pvc); err != nil {
		return err
	}
	sourceNamespace := dataVolume.Spec.Source.PVC.Namespace
	if sourceNamespace == "" {
		sourceNamespace = dataVolume.Namespace
	}
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
	staticProvisionPending := checkStaticProvisionPending(pvc, datavolume)
	prePopulated := dvIsPrePopulated(datavolume)
	requiresWork, err := r.pvcRequiresWork(pvc, datavolume)
	if err != nil {
		return syncRes, err
	}

	if !requiresWork || prePopulated || staticProvisionPending {
		return syncRes, nil
	}

	if addedToken, err := r.ensureExtendedTokenDV(datavolume); err != nil {
		return syncRes, err
	} else if addedToken {
		// make sure token gets persisted before doing anything else
		return syncRes, nil
	}

	if pvc == nil {
		// Check if source PVC exists and do proper validation before attempting to clone
		if done, err := r.validateCloneAndSourcePVC(&syncRes, log); err != nil {
			return syncRes, err
		} else if !done {
			return syncRes, nil
		}

		// Always call detect size, it will handle the case where size is specified
		// and detection pod not necessary
		if datavolume.Spec.Storage != nil {
			done, err := r.detectCloneSize(&syncRes)
			if err != nil {
				return syncRes, err
			} else if !done {
				// Check if the source PVC is ready to be cloned
				if readyToClone, err := r.isSourceReadyToClone(datavolume); err != nil {
					return syncRes, err
				} else if !readyToClone {
					if syncRes.result == nil {
						syncRes.result = &reconcile.Result{}
					}
					syncRes.result.RequeueAfter = sourceInUseRequeueDuration
					return syncRes, r.syncCloneStatusPhase(&syncRes, cdiv1.CloneScheduled, nil)
				}
				return syncRes, nil
			}
		}

		finalPvcSpec := pvcSpec
		pvcModifier := r.updateAnnotations

		if syncRes.usePopulator {
			if isCrossNamespaceClone(datavolume) {
				if !cc.HasFinalizer(datavolume, crossNamespaceFinalizer) {
					cc.AddFinalizer(datavolume, crossNamespaceFinalizer)
					return syncRes, r.syncCloneStatusPhase(&syncRes, cdiv1.CloneScheduled, nil)
				}
			}
			pvcModifier = r.updatePVCForPopulation
		} else {
			// Check if we can use CSI clone when populators are disabled
			sc, _ := clone.GetStorageClassForClaim(context.TODO(), r.client, &corev1.PersistentVolumeClaim{Spec: *pvcSpec})
			if sc != nil {
				if advised, ok := storagecapabilities.GetAdvisedCloneStrategy(sc); ok && advised == cdiv1.CloneStrategyCsiClone {
					sourcePvc, err := r.findSourcePvc(datavolume)
					if err != nil {
						return syncRes, err
					}
					csiCloneSpec, err := r.newVolumeCloneFromPVCSpec(datavolume, sourcePvc, pvcSpec)
					if err != nil {
						return syncRes, err
					}
					finalPvcSpec = csiCloneSpec
					pvcModifier = r.updateCSICloneFromPVCAnnotations
				}
			}
		}

		// Create PVC with determined spec and modifier
		pvc, err = r.createPvcForDatavolume(datavolume, finalPvcSpec, pvcModifier)
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
	}

	if syncRes.usePopulator {
		if err := r.reconcileVolumeCloneSourceCR(&syncRes); err != nil {
			return syncRes, err
		}

		ct, ok := pvc.Annotations[cc.AnnCloneType]
		if ok {
			cc.AddAnnotation(datavolume, cc.AnnCloneType, ct)
		}
	} else {
		// Determine clone strategy and annotate DV accordingly
		sc, _ := clone.GetStorageClassForClaim(context.TODO(), r.client, pvc)
		var advised *cdiv1.CDICloneStrategy
		if sc != nil {
			if s, ok := storagecapabilities.GetAdvisedCloneStrategy(sc); ok {
				advised = &s
			}
		}
		if advised != nil && *advised == cdiv1.CloneStrategyCsiClone &&
			((pvc.Spec.DataSource != nil && pvc.Spec.DataSource.Kind == "PersistentVolumeClaim") ||
				(pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == "PersistentVolumeClaim")) {
			cc.AddAnnotation(datavolume, cc.AnnCloneType, string(cdiv1.CloneStrategyCsiClone))
			if pvc.Status.Phase == corev1.ClaimBound {
				if pvc.Annotations == nil || pvc.Annotations[cc.AnnPopulatedFor] != datavolume.Name {
					pvcCpy := pvc.DeepCopy()
					cc.AddAnnotation(pvcCpy, cc.AnnPopulatedFor, datavolume.Name)
					if err := r.updatePVC(pvcCpy); err != nil {
						return syncRes, err
					}
					pvc = pvcCpy
				}
			}
		} else {
			cc.AddAnnotation(datavolume, cc.AnnCloneType, string(cdiv1.CloneStrategyHostAssisted))
			if err := r.fallbackToHostAssisted(pvc); err != nil {
				return syncRes, err
			}
		}
	}

	if err := r.ensureExtendedTokenPVC(datavolume, pvc); err != nil {
		return syncRes, err
	}

	if !syncRes.usePopulator {
		cloneType, ok := datavolume.Annotations[cc.AnnCloneType]
		if ok && cloneType == string(cdiv1.CloneStrategyCsiClone) {
			shouldBeMarkedWaitForFirstConsumer, err := r.shouldBeMarkedWaitForFirstConsumer(pvc)
			if err != nil {
				return syncRes, err
			}

			switch pvc.Status.Phase {
			case corev1.ClaimBound:
				return syncRes, nil
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
							reason:    "ClaimLost",
							message:   fmt.Sprintf("PVC %s lost", pvc.Name),
						})
			}
		}
	}

	return syncRes, syncErr
}

// newVolumeCloneFromPVCSpec creates a PVC spec for CSI clone from another PVC
func (r *PvcCloneReconciler) newVolumeCloneFromPVCSpec(dv *cdiv1.DataVolume, sourcePvc *corev1.PersistentVolumeClaim, targetPvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaimSpec, error) {
	csiCloneSpec := targetPvcSpec.DeepCopy()

	if csiCloneSpec.Resources.Requests == nil {
		csiCloneSpec.Resources.Requests = corev1.ResourceList{}
	}
	if sourcePvc.Status.Capacity != nil {
		if sourceSize := sourcePvc.Status.Capacity.Storage(); sourceSize != nil {
			csiCloneSpec.Resources.Requests[corev1.ResourceStorage] = *sourceSize
		}
	}

	// Set DataSource to point to the source PVC
	csiCloneSpec.DataSource = &corev1.TypedLocalObjectReference{
		Kind: "PersistentVolumeClaim",
		Name: dv.Spec.Source.PVC.Name,
	}

	return csiCloneSpec, nil
}

// updateCSICloneFromPVCAnnotations sets CSI clone from PVC specific annotations
func (r *PvcCloneReconciler) updateCSICloneFromPVCAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.PVC == nil {
		return errors.Errorf("no PVC source set for clone datavolume")
	}
	if err := addCloneToken(dataVolume, pvc); err != nil {
		return err
	}
	pvc.Annotations[AnnCSICloneRequest] = "true"
	return nil
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
	pods, err := cc.GetPodsUsingPVCs(context.TODO(), r.client, dv.Spec.Source.PVC.Namespace, sets.New(dv.Spec.Source.PVC.Name), false)
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

	err = validateClone(sourcePvc, &datavolume.Spec)
	if err != nil {
		syncErr := r.syncDataVolumeStatusPhaseWithEvent(syncState, datavolume.Status.Phase, nil,
			Event{
				eventType: corev1.EventTypeWarning,
				reason:    CloneValidationFailed,
				message:   fmt.Sprintf(MessageCloneValidationFailed, err.Error()),
			})
		if syncErr != nil {
			log.Error(syncErr, "failed to sync DataVolume status with event")
		}
		return false, nil
	}

	return true, nil
}

// validateClone compares a clone spec against its source PVC to validate its creation
func validateClone(sourcePVC *corev1.PersistentVolumeClaim, spec *cdiv1.DataVolumeSpec) error {
	var targetResources corev1.VolumeResourceRequirements

	valid, sourceContentType, targetContentType := validateContentTypes(sourcePVC, spec)
	if !valid {
		msg := fmt.Sprintf("Source contentType (%s) and target contentType (%s) do not match", sourceContentType, targetContentType)
		return errors.New(msg)
	}

	isSizelessClone := false
	explicitPvcRequest := spec.PVC != nil
	if explicitPvcRequest {
		targetResources = spec.PVC.Resources
	} else {
		targetResources = spec.Storage.Resources
		// The storage size in the target DV can be empty
		// when cloning using the 'Storage' API
		if _, ok := targetResources.Requests[corev1.ResourceStorage]; !ok {
			isSizelessClone = true
		}
	}

	// TODO: Spec.Storage API needs a better more complex check to validate clone size - to account for fsOverhead
	// simple size comparison will not work here
	if (!isSizelessClone && cc.GetVolumeMode(sourcePVC) == corev1.PersistentVolumeBlock) || explicitPvcRequest {
		if err := cc.ValidateRequestedCloneSize(sourcePVC.Spec.Resources, targetResources); err != nil {
			return err
		}
	}

	return nil
}

// validateContentTypes compares the content type of a clone DV against its source PVC's one
func validateContentTypes(sourcePVC *corev1.PersistentVolumeClaim, spec *cdiv1.DataVolumeSpec) (bool, cdiv1.DataVolumeContentType, cdiv1.DataVolumeContentType) {
	sourceContentType := cc.GetPVCContentType(sourcePVC)
	targetContentType := spec.ContentType
	if targetContentType == "" {
		targetContentType = cdiv1.DataVolumeKubeVirt
	}
	return sourceContentType == targetContentType, sourceContentType, targetContentType
}

// isSourceReadyToClone handles the reconciling process of a clone when the source PVC is not ready
func (r *PvcCloneReconciler) isSourceReadyToClone(datavolume *cdiv1.DataVolume) (bool, error) {
	// TODO preper const
	eventReason := "CloneSourceInUse"

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
func (r *PvcCloneReconciler) detectCloneSize(syncState *dvSyncState) (bool, error) {
	sourcePvc, err := r.findSourcePvc(syncState.dvMutated)
	if err != nil {
		return false, err
	}

	// because of filesystem overhead calculations when cloning
	// even if storage size is requested we have to calculate source size
	// when source is filesystem and target is block
	requestedSize, hasSize := syncState.pvcSpec.Resources.Requests[corev1.ResourceStorage]
	sizeRequired := !hasSize || requestedSize.IsZero()
	targetIsBlock := syncState.pvcSpec.VolumeMode != nil && *syncState.pvcSpec.VolumeMode == corev1.PersistentVolumeBlock
	sourceIsFilesystem := cc.GetVolumeMode(sourcePvc) == corev1.PersistentVolumeFilesystem
	// have to be explicit here or detection pod will crash
	sourceIsKubevirt := sourcePvc.Annotations[cc.AnnContentType] == string(cdiv1.DataVolumeKubeVirt)
	if !sizeRequired && (!targetIsBlock || !sourceIsFilesystem || !sourceIsKubevirt) {
		return true, nil
	}

	var targetSize int64
	sourceCapacity := sourcePvc.Status.Capacity.Storage()

	// Due to possible filesystem overhead complications when cloning
	// using host-assisted strategy, we create a pod that automatically
	// collects the size of the original virtual image with 'qemu-img'.
	// If the original PVC's volume mode is "block",
	// we simply extract the value from the original PVC's spec.
	if sourceIsFilesystem && sourceIsKubevirt {
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

	var isPermissiveClone bool
	if sizeRequired {
		// Allow the clone-controller to skip the size comparison requirement
		// if the source's size ends up being larger due to overhead differences
		// TODO: Fix this in next PR that uses actual size also in validation
		isPermissiveClone = sourceCapacity.CmpInt64(targetSize) == 1
	} else {
		isPermissiveClone = requestedSize.CmpInt64(targetSize) >= 0
	}

	if isPermissiveClone {
		syncState.dvMutated.Annotations[cc.AnnPermissiveClone] = "true"
	}

	if !sizeRequired {
		return true, nil
	}

	// Parse size into a 'Quantity' struct and, if needed, inflate it with filesystem overhead
	targetCapacity, err := cc.InflateSizeWithOverhead(context.TODO(), r.client, targetSize, syncState.pvcSpec)
	if err != nil {
		return false, err
	}

	if syncState.pvcSpec.Resources.Requests == nil {
		syncState.pvcSpec.Resources.Requests = corev1.ResourceList{}
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
	populated, err := cc.IsPopulated(sourcePvc, r.client)
	if err != nil {
		return 0, err
	}
	if !populated {
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
	// Finally, delete the pod
	if targetPvc != nil && cc.ShouldDeletePod(targetPvc) {
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
	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(context.TODO(), r.client)
	if err != nil {
		return nil
	}
	// Generate individual specs
	objectMeta := makeSizeDetectionObjectMeta(sourcePvc)
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

	cc.CopyAllowedAnnotations(dv, pod)
	cc.SetRestrictedSecurityContext(&pod.Spec)

	return pod
}

// makeSizeDetectionObjectMeta creates and returns the object metadata for the size-detection pod
func makeSizeDetectionObjectMeta(sourcePvc *corev1.PersistentVolumeClaim) *metav1.ObjectMeta {
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
func isPodComplete(pod *corev1.Pod) bool {
	return pod != nil && pod.Status.Phase == corev1.PodSucceeded
}
