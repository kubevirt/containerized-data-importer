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

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	//AnnSmartCloneRequest sets our expected annotation for a CloneRequest
	AnnSmartCloneRequest = "k8s.io/SmartCloneRequest"

	annSmartCloneSnapshot = "cdi.kubevirt.io/smartCloneSnapshot"

	snapshotCloneControllerName = "datavolume-snapshot-clone-controller"
)

// SnapshotCloneReconciler members
type SnapshotCloneReconciler struct {
	CloneReconcilerBase
}

// NewSnapshotCloneController creates a new instance of the datavolume clone controller
func NewSnapshotCloneController(
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
	reconciler := &SnapshotCloneReconciler{
		CloneReconcilerBase: CloneReconcilerBase{
			ReconcilerBase: ReconcilerBase{
				client:               client,
				scheme:               mgr.GetScheme(),
				log:                  log.WithName(snapshotCloneControllerName),
				featureGates:         featuregates.NewFeatureGates(client),
				recorder:             mgr.GetEventRecorderFor(snapshotCloneControllerName),
				installerLabels:      installerLabels,
				shouldUpdateProgress: true,
			},
			clonerImage:         clonerImage,
			importerImage:       importerImage,
			pullPolicy:          pullPolicy,
			cloneSourceAPIGroup: pointer.String(snapshotv1.GroupName),
			cloneSourceKind:     "VolumeSnapshot",
			shortTokenValidator: cc.NewCloneTokenValidator(common.CloneTokenIssuer, tokenPublicKey),
			longTokenValidator:  cc.NewCloneTokenValidator(common.ExtendedCloneTokenIssuer, tokenPublicKey),
			// for long term tokens to handle cross namespace dumb clones
			tokenGenerator: newLongTermCloneTokenGenerator(tokenPrivateKey),
		},
	}

	dataVolumeCloneController, err := controller.New(snapshotCloneControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := reconciler.addDataVolumeSnapshotCloneControllerWatches(mgr, dataVolumeCloneController); err != nil {
		return nil, err
	}

	return dataVolumeCloneController, nil
}

func (r *SnapshotCloneReconciler) addDataVolumeSnapshotCloneControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeSnapshotClone); err != nil {
		return err
	}

	// Watch to reconcile clones created without source
	if err := mgr.GetClient().List(context.TODO(), &snapshotv1.VolumeSnapshotList{}); err != nil {
		if meta.IsNoMatchError(err) {
			// Back out if there's no point to attempt watch
			return nil
		}
		if !cc.IsErrCacheNotStarted(err) {
			return err
		}
	}

	if err := addCloneWithoutSourceWatch(mgr, datavolumeController, &snapshotv1.VolumeSnapshot{}, "spec.source.snapshot", dataVolumeSnapshotClone); err != nil {
		return err
	}

	if err := r.addVolumeCloneSourceWatch(datavolumeController); err != nil {
		return err
	}

	return nil
}

// Reconcile loop for the clone data volumes
func (r *SnapshotCloneReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, req, r)
}

func (r *SnapshotCloneReconciler) prepare(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}

	return nil
}

func (r *SnapshotCloneReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.Snapshot == nil {
		return errors.Errorf("no source set for clone datavolume")
	}
	if err := addCloneToken(dataVolume, pvc); err != nil {
		return err
	}
	sourceNamespace := dataVolume.Spec.Source.Snapshot.Namespace
	if sourceNamespace == "" {
		sourceNamespace = dataVolume.Namespace
	}
	tempPvcName := getTempHostAssistedSourcePvcName(dataVolume)
	pvc.Annotations[cc.AnnCloneRequest] = sourceNamespace + "/" + tempPvcName
	return nil
}

func (r *SnapshotCloneReconciler) sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error) {
	syncState, err := r.syncSnapshotClone(log, req)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState.dvSyncResult, err
}

func (r *SnapshotCloneReconciler) syncSnapshotClone(log logr.Logger, req reconcile.Request) (dvSyncState, error) {
	syncRes, syncErr := r.syncCommon(log, req, r.cleanup, r.prepare)
	if syncErr != nil || syncRes.result != nil {
		return syncRes, syncErr
	}

	pvc := syncRes.pvc
	pvcSpec := syncRes.pvcSpec
	datavolume := syncRes.dvMutated

	requiresNoWork := pvcRequiresNoWork(pvc, datavolume)
	staticProvisionPending := checkStaticProvisionPending(pvc, datavolume)
	_, prePopulated := datavolume.Annotations[cc.AnnPrePopulated]

	if requiresNoWork || prePopulated || staticProvisionPending {
		return syncRes, nil
	}

	if addedToken, err := r.ensureExtendedTokenDV(datavolume); err != nil {
		return syncRes, err
	} else if addedToken {
		// make sure token gets persisted before doing anything else
		return syncRes, nil
	}

	if pvc == nil {
		if datavolume.Spec.Storage != nil {
			done, err := r.detectCloneSize(log, &syncRes)
			if err != nil {
				return syncRes, err
			}
			if !done {
				if syncRes.result == nil {
					syncRes.result = &reconcile.Result{}
				}
				syncRes.result.RequeueAfter = sourceInUseRequeueDuration
				// I think pending is more accurate but doing scheduled to be consistent with PVC controller
				return syncRes, r.syncCloneStatusPhase(&syncRes, cdiv1.CloneScheduled, nil)
			}
		}

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
			if done, err := r.validateAndInitLegacyClone(&syncRes); err != nil {
				return syncRes, err
			} else if !done {
				return syncRes, nil
			}
		}

		targetPvc, err := r.createPvcForDatavolume(datavolume, pvcSpec, pvcModifier)
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
		pvc = targetPvc
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
		cc.AddAnnotation(datavolume, cc.AnnCloneType, string(cdiv1.CloneStrategyHostAssisted))
		if err := r.fallbackToHostAssisted(pvc); err != nil {
			return syncRes, err
		}
	}

	if err := r.ensureExtendedTokenPVC(datavolume, pvc); err != nil {
		return syncRes, err
	}

	return syncRes, syncErr
}

func (r *SnapshotCloneReconciler) detectCloneSize(log logr.Logger, syncState *dvSyncState) (bool, error) {
	pvcSpec := syncState.pvcSpec
	requestedSize := pvcSpec.Resources.Requests[corev1.ResourceStorage]
	if !requestedSize.IsZero() {
		log.V(3).Info("requested size is set, skipping size detection", "size", requestedSize)
		return true, nil
	}

	datavolume := syncState.dvMutated
	nn := types.NamespacedName{Namespace: datavolume.Spec.Source.Snapshot.Namespace, Name: datavolume.Spec.Source.Snapshot.Name}
	snapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(context.TODO(), nn, snapshot); err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(3).Info("snapshot source does not exist", "namespace", nn.Namespace, "name", nn.Name)
			return false, nil
		}

		return false, err
	}

	if snapshot.Status == nil || snapshot.Status.RestoreSize == nil {
		log.V(3).Info("snapshot source does not have restoreSize", "namespace", nn.Namespace, "name", nn.Name)
		return false, nil
	}

	pvcSpec.Resources.Requests[corev1.ResourceStorage] = *snapshot.Status.RestoreSize

	log.V(3).Info("set pvc request size", "size", pvcSpec.Resources.Requests[corev1.ResourceStorage])

	return true, nil
}

func (r *SnapshotCloneReconciler) validateAndInitLegacyClone(syncState *dvSyncState) (bool, error) {
	// Check if source snapshot exists and do proper validation before attempting to clone
	if done, err := r.validateCloneAndSourceSnapshot(syncState); err != nil {
		return false, err
	} else if !done {
		return false, nil
	}

	datavolume := syncState.dvMutated
	pvcSpec := syncState.pvcSpec
	nn := types.NamespacedName{Namespace: datavolume.Spec.Source.Snapshot.Namespace, Name: datavolume.Spec.Source.Snapshot.Name}
	snapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(context.TODO(), nn, snapshot); err != nil {
		return false, err
	}

	valid, err := r.isSnapshotValidForClone(snapshot)
	if err != nil || !valid {
		return false, err
	}

	if err := r.createTempHostAssistedSourcePvc(datavolume, snapshot, pvcSpec, syncState); err != nil {
		return false, err
	}

	return true, nil
}

// validateCloneAndSourceSnapshot checks if the source snapshot of a clone exists and does proper validation
func (r *SnapshotCloneReconciler) validateCloneAndSourceSnapshot(syncState *dvSyncState) (bool, error) {
	datavolume := syncState.dvMutated
	nn := types.NamespacedName{Namespace: datavolume.Spec.Source.Snapshot.Namespace, Name: datavolume.Spec.Source.Snapshot.Name}
	snapshot := &snapshotv1.VolumeSnapshot{}
	err := r.client.Get(context.TODO(), nn, snapshot)
	if err != nil {
		// Clone without source
		if k8serrors.IsNotFound(err) {
			syncEventErr := r.syncDataVolumeStatusPhaseWithEvent(syncState, datavolume.Status.Phase, nil,
				Event{
					eventType: corev1.EventTypeWarning,
					reason:    CloneWithoutSource,
					message:   fmt.Sprintf(MessageCloneWithoutSource, "snapshot", datavolume.Spec.Source.Snapshot.Name),
				})
			if syncEventErr != nil {
				r.log.Error(syncEventErr, "failed sync status phase")
			}
			return false, nil
		}
		return false, err
	}

	err = cc.ValidateSnapshotClone(snapshot, &datavolume.Spec)
	if err != nil {
		r.recorder.Event(datavolume, corev1.EventTypeWarning, CloneValidationFailed, MessageCloneValidationFailed)
		return false, err
	}

	return true, nil
}

func (r *SnapshotCloneReconciler) createTempHostAssistedSourcePvc(dv *cdiv1.DataVolume, snapshot *snapshotv1.VolumeSnapshot, targetPvcSpec *corev1.PersistentVolumeClaimSpec, syncState *dvSyncState) error {
	tempPvcName := getTempHostAssistedSourcePvcName(dv)
	tempHostAssistedSourcePvc, err := r.makePvcFromSnapshot(tempPvcName, dv, snapshot, targetPvcSpec)
	if err != nil {
		return err
	}
	// Don't need owner refs for host assisted since clone controller will fail on IsPopulated
	tempHostAssistedSourcePvc.OwnerReferences = nil
	if err := setAnnOwnedByDataVolume(tempHostAssistedSourcePvc, dv); err != nil {
		return err
	}
	tempHostAssistedSourcePvc.Annotations[cc.AnnOwnerUID] = string(dv.UID)
	tempHostAssistedSourcePvc.Labels[common.CDIComponentLabel] = common.CloneFromSnapshotFallbackPVCCDILabel
	// Figure out storage class of source snap
	// Can only restore to original storage class, but there might be several SCs with same driver
	// So we do best effort here
	vsc := &snapshotv1.VolumeSnapshotClass{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *snapshot.Spec.VolumeSnapshotClassName}, vsc); err != nil {
		return err
	}
	sc, err := r.getStorageClassCorrespondingToSnapClass(vsc.Driver)
	if err != nil {
		return err
	}
	tempHostAssistedSourcePvc.Spec.StorageClassName = &sc
	// TODO: set source volume mode as well from snapcontent.sourceVolumeMode
	// might also want readonlymany for this PVC at all times

	currentTempHostAssistedSourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(tempHostAssistedSourcePvc), currentTempHostAssistedSourcePvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		if err := r.client.Create(context.TODO(), tempHostAssistedSourcePvc); err != nil {
			if cc.ErrQuotaExceeded(err) {
				syncEventErr := r.syncDataVolumeStatusPhaseWithEvent(syncState, cdiv1.Pending, nil,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
						message:   err.Error(),
					})
				if syncEventErr != nil {
					r.log.Error(syncEventErr, "failed sync status phase")
				}
			}
			return err
		}
	}

	return nil
}

func (r *SnapshotCloneReconciler) getStorageClassCorrespondingToSnapClass(driver string) (string, error) {
	matches := []storagev1.StorageClass{}

	storageClasses := &storagev1.StorageClassList{}
	if err := r.client.List(context.TODO(), storageClasses); err != nil {
		r.log.V(3).Info("Unable to retrieve available storage classes")
		return "", errors.New("unable to retrieve storage classes")
	}
	for _, storageClass := range storageClasses.Items {
		if storageClass.Provisioner == driver {
			matches = append(matches, storageClass)
		}
	}

	if len(matches) > 1 {
		r.log.V(3).Info("more than one storage class match for snapshot driver, picking first", "storageClass.Name", matches[0].Name)
		return matches[0].Name, nil
	}
	if len(matches) == 0 {
		return "", errors.New("no storage class match for snapshot driver")
	}

	return matches[0].Name, nil
}

func (r *SnapshotCloneReconciler) makePvcFromSnapshot(pvcName string, dv *cdiv1.DataVolume, snapshot *snapshotv1.VolumeSnapshot, targetPvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaim, error) {
	newPvc, err := newPvcFromSnapshot(dv, pvcName, snapshot, targetPvcSpec)
	if err != nil {
		return nil, err
	}
	// Don't accidentally reconcile this one in smart clone controller
	delete(newPvc.Annotations, AnnSmartCloneRequest)
	delete(newPvc.Annotations, annSmartCloneSnapshot)
	newPvc.Labels[common.CDIComponentLabel] = "cdi-clone-from-snapshot-source"
	util.SetRecommendedLabels(newPvc, r.installerLabels, "cdi-controller")
	newPvc.OwnerReferences = nil

	if newPvc.Namespace == dv.Namespace {
		newPvc.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(dv, schema.GroupVersionKind{
				Group:   cdiv1.SchemeGroupVersion.Group,
				Version: cdiv1.SchemeGroupVersion.Version,
				Kind:    "DataVolume",
			}),
		}
	} else {
		if err := setAnnOwnedByDataVolume(newPvc, dv); err != nil {
			return nil, err
		}
		newPvc.Annotations[cc.AnnOwnerUID] = string(dv.UID)
	}

	return newPvc, nil
}

func getTempHostAssistedSourcePvcName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("%s-host-assisted-source-pvc", dv.GetUID())
}

func (r *SnapshotCloneReconciler) cleanup(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	if err := r.populateSourceIfSourceRef(dv); err != nil {
		return err
	}

	// This cleanup should be done if dv is marked for deletion or in case it succeeded
	if dv.DeletionTimestamp == nil && dv.Status.Phase != cdiv1.Succeeded {
		return nil
	}

	r.log.V(3).Info("Cleanup initiated in dv snapshot clone controller")

	if err := r.reconcileVolumeCloneSourceCR(syncState); err != nil {
		return err
	}

	if err := r.cleanupHostAssistedSnapshotClone(dv); err != nil {
		return err
	}

	return nil
}

func (r *SnapshotCloneReconciler) cleanupHostAssistedSnapshotClone(dv *cdiv1.DataVolume) error {
	tempPvcName := getTempHostAssistedSourcePvcName(dv)
	nn := types.NamespacedName{Namespace: dv.Spec.Source.Snapshot.Namespace, Name: tempPvcName}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), nn, pvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if !hasAnnOwnedByDataVolume(pvc) {
		return nil
	}
	namespace, name, err := getAnnOwnedByDataVolume(pvc)
	if err != nil {
		return err
	}
	if namespace != dv.Namespace || name != dv.Name {
		return nil
	}
	if v, ok := pvc.Labels[common.CDIComponentLabel]; !ok || v != common.CloneFromSnapshotFallbackPVCCDILabel {
		return nil
	}
	// TODO: escape hatch for users that don't mind the overhead and would like to keep this PVC
	// for future clones?

	if err := r.client.Delete(context.TODO(), pvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// isSnapshotValidForClone returns true if the passed snapshot is valid for cloning
func (r *SnapshotCloneReconciler) isSnapshotValidForClone(snapshot *snapshotv1.VolumeSnapshot) (bool, error) {
	if snapshot.Status == nil {
		r.log.V(3).Info("Snapshot does not have status populated yet")
		return false, nil
	}
	if !cc.IsSnapshotReady(snapshot) {
		r.log.V(3).Info("snapshot not ReadyToUse, while we allow this, probably going to be an issue going forward", "namespace", snapshot.Namespace, "name", snapshot.Name)
	}
	if snapshot.Status.Error != nil {
		errMessage := "no details"
		if msg := snapshot.Status.Error.Message; msg != nil {
			errMessage = *msg
		}
		return false, fmt.Errorf("snapshot in error state with msg: %s", errMessage)
	}
	if snapshot.Spec.VolumeSnapshotClassName == nil || *snapshot.Spec.VolumeSnapshotClassName == "" {
		return false, fmt.Errorf("snapshot %s/%s does not have volume snap class populated, can't clone", snapshot.Name, snapshot.Namespace)
	}
	return true, nil
}

func newPvcFromSnapshot(obj metav1.Object, name string, snapshot *snapshotv1.VolumeSnapshot, targetPvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaim, error) {
	targetPvcSpecCopy := targetPvcSpec.DeepCopy()
	restoreSize := snapshot.Status.RestoreSize
	if restoreSize == nil || restoreSize.Sign() == -1 {
		return nil, fmt.Errorf("snapshot has no RestoreSize")
	}
	if restoreSize.IsZero() {
		reqSize := targetPvcSpec.Resources.Requests[corev1.ResourceStorage]
		restoreSize = &reqSize
	}

	key, err := cache.MetaNamespaceKeyFunc(snapshot)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"cdi-controller":         snapshot.Name,
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	for k, v := range obj.GetLabels() {
		labels[k] = v
	}

	annotations := map[string]string{
		AnnSmartCloneRequest:          "true",
		cc.AnnRunningCondition:        string(corev1.ConditionFalse),
		cc.AnnRunningConditionMessage: cc.CloneComplete,
		cc.AnnRunningConditionReason:  "Completed",
		annSmartCloneSnapshot:         key,
	}
	for k, v := range obj.GetAnnotations() {
		annotations[k] = v
	}

	if util.ResolveVolumeMode(targetPvcSpecCopy.VolumeMode) == corev1.PersistentVolumeFilesystem {
		labels[common.KubePersistentVolumeFillingUpSuppressLabelKey] = common.KubePersistentVolumeFillingUpSuppressLabelValue
	}

	target := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   snapshot.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSource: &corev1.TypedLocalObjectReference{
				Name:     snapshot.Name,
				Kind:     "VolumeSnapshot",
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
			},
			VolumeMode:       targetPvcSpecCopy.VolumeMode,
			AccessModes:      targetPvcSpecCopy.AccessModes,
			StorageClassName: targetPvcSpecCopy.StorageClassName,
			Resources:        targetPvcSpecCopy.Resources,
		},
	}

	if target.Spec.Resources.Requests == nil {
		target.Spec.Resources.Requests = corev1.ResourceList{}
	}

	target.Spec.Resources.Requests[corev1.ResourceStorage] = *restoreSize

	ownerRef := metav1.GetControllerOf(snapshot)
	if ownerRef != nil {
		target.OwnerReferences = append(target.OwnerReferences, *ownerRef)
	}

	return target, nil
}
