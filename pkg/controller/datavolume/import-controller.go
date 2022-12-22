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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
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

	importControllerName = "datavolume-import-controller"
)

// ImportReconciler members
type ImportReconciler struct {
	ReconcilerBase
}

// NewImportController creates a new instance of the datavolume import controller
func NewImportController(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &ImportReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(importControllerName),
			recorder:        mgr.GetEventRecorderFor(importControllerName),
			featureGates:    featuregates.NewFeatureGates(client),
			installerLabels: installerLabels,
		},
	}
	reconciler.Reconciler = reconciler

	datavolumeController, err := controller.New(importControllerName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addDataVolumeImportControllerWatches(mgr, datavolumeController); err != nil {
		return nil, err
	}

	return datavolumeController, nil
}

func addDataVolumeImportControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, dvPhaseField, func(obj client.Object) []string {
		return []string{string(obj.(*cdiv1.DataVolume).Status.Phase)}
	}); err != nil {
		return err
	}
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeImport); err != nil {
		return err
	}
	return nil
}

func (r ImportReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, annotations map[string]string) error {
	if dataVolume.Spec.Source.HTTP != nil {
		annotations[cc.AnnEndpoint] = dataVolume.Spec.Source.HTTP.URL
		annotations[cc.AnnSource] = cc.SourceHTTP

		if dataVolume.Spec.Source.HTTP.SecretRef != "" {
			annotations[cc.AnnSecret] = dataVolume.Spec.Source.HTTP.SecretRef
		}
		if dataVolume.Spec.Source.HTTP.CertConfigMap != "" {
			annotations[cc.AnnCertConfigMap] = dataVolume.Spec.Source.HTTP.CertConfigMap
		}
		for index, header := range dataVolume.Spec.Source.HTTP.ExtraHeaders {
			annotations[fmt.Sprintf("%s.%d", cc.AnnExtraHeaders, index)] = header
		}
		for index, header := range dataVolume.Spec.Source.HTTP.SecretExtraHeaders {
			annotations[fmt.Sprintf("%s.%d", cc.AnnSecretExtraHeaders, index)] = header
		}
		return nil
	}
	if dataVolume.Spec.Source.S3 != nil {
		annotations[cc.AnnEndpoint] = dataVolume.Spec.Source.S3.URL
		annotations[cc.AnnSource] = cc.SourceS3
		if dataVolume.Spec.Source.S3.SecretRef != "" {
			annotations[cc.AnnSecret] = dataVolume.Spec.Source.S3.SecretRef
		}
		if dataVolume.Spec.Source.S3.CertConfigMap != "" {
			annotations[cc.AnnCertConfigMap] = dataVolume.Spec.Source.S3.CertConfigMap
		}
		return nil
	}
	if dataVolume.Spec.Source.Registry != nil {
		annotations[cc.AnnSource] = cc.SourceRegistry
		pullMethod := dataVolume.Spec.Source.Registry.PullMethod
		if pullMethod != nil && *pullMethod != "" {
			annotations[cc.AnnRegistryImportMethod] = string(*pullMethod)
		}
		url := dataVolume.Spec.Source.Registry.URL
		if url != nil && *url != "" {
			annotations[cc.AnnEndpoint] = *url
		} else {
			imageStream := dataVolume.Spec.Source.Registry.ImageStream
			if imageStream != nil && *imageStream != "" {
				annotations[cc.AnnEndpoint] = *imageStream
				annotations[cc.AnnRegistryImageStream] = "true"
			}
		}
		secretRef := dataVolume.Spec.Source.Registry.SecretRef
		if secretRef != nil && *secretRef != "" {
			annotations[cc.AnnSecret] = *secretRef
		}
		certConfigMap := dataVolume.Spec.Source.Registry.CertConfigMap
		if certConfigMap != nil && *certConfigMap != "" {
			annotations[cc.AnnCertConfigMap] = *certConfigMap
		}
		return nil
	}
	if dataVolume.Spec.Source.Blank != nil {
		annotations[cc.AnnSource] = cc.SourceNone
		return nil
	}
	if dataVolume.Spec.Source.Imageio != nil {
		annotations[cc.AnnEndpoint] = dataVolume.Spec.Source.Imageio.URL
		annotations[cc.AnnSource] = cc.SourceImageio
		annotations[cc.AnnSecret] = dataVolume.Spec.Source.Imageio.SecretRef
		annotations[cc.AnnCertConfigMap] = dataVolume.Spec.Source.Imageio.CertConfigMap
		annotations[cc.AnnDiskID] = dataVolume.Spec.Source.Imageio.DiskID
		return nil
	}
	if dataVolume.Spec.Source.VDDK != nil {
		annotations[cc.AnnEndpoint] = dataVolume.Spec.Source.VDDK.URL
		annotations[cc.AnnSource] = cc.SourceVDDK
		annotations[cc.AnnSecret] = dataVolume.Spec.Source.VDDK.SecretRef
		annotations[cc.AnnBackingFile] = dataVolume.Spec.Source.VDDK.BackingFile
		annotations[cc.AnnUUID] = dataVolume.Spec.Source.VDDK.UUID
		annotations[cc.AnnThumbprint] = dataVolume.Spec.Source.VDDK.Thumbprint
		if dataVolume.Spec.Source.VDDK.InitImageURL != "" {
			annotations[cc.AnnVddkInitImageURL] = dataVolume.Spec.Source.VDDK.InitImageURL
		}
		return nil
	}
	return errors.Errorf("no source set for import datavolume")
}

func (r ImportReconciler) updateStatus(log logr.Logger, syncRes dataVolumeSyncResult, syncErr error) (reconcile.Result, error) {
	if syncErr != nil {
		return reconcile.Result{}, syncErr
	}
	if syncRes.res != nil {
		return *syncRes.res, nil
	}
	//FIXME: pass syncRes instead of args
	if err := r.reconcileDataVolumePVC(log, syncRes.dv, &syncRes.pvc, syncRes.pvcSpec); err != nil {
		return reconcile.Result{}, err
	}
	return r.reconcileDataVolumeStatus(syncRes.dv, syncRes.pvc, nil, r.updateStatusPhase)

}

func (r *ImportReconciler) reconcileDataVolumePVC(log logr.Logger, dv *cdiv1.DataVolume, pvc **corev1.PersistentVolumeClaim, pvcSpec *v1.PersistentVolumeClaimSpec) error {
	if _, dvPrePopulated := dv.Annotations[cc.AnnPrePopulated]; dvPrePopulated {
		return nil
	}
	if *pvc == nil {
		newPvc, err := r.createPvcForDatavolume(dv, pvcSpec, r.updateCheckpoint)
		if err != nil {
			if cc.ErrQuotaExceeded(err) {
				r.updateDataVolumeStatusPhaseWithEvent(cdiv1.Pending, dv, nil, nil,
					Event{
						eventType: corev1.EventTypeWarning,
						reason:    cc.ErrExceededQuota,
						message:   err.Error(),
					})
			}
			return err
		}
		*pvc = newPvc
		return nil
	}
	if cc.GetSource(*pvc) == cc.SourceVDDK {
		changed, err := r.getVddkAnnotations(dv, *pvc)
		if err != nil {
			return err
		}
		if changed {
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}, dv)
			if err != nil {
				return err
			}
		}
	}

	if err := r.maybeSetMultiStageAnnotation(*pvc, dv); err != nil {
		return err
	}
	return nil
}

func (r ImportReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		_, ok := pvc.Annotations[cc.AnnImportPod]
		if !ok || pvc.Status.Phase != corev1.ClaimBound || pvcIsPopulated(pvc, dataVolumeCopy) {
			return nil
		}
		dataVolumeCopy.Status.Phase = cdiv1.ImportScheduled
	}
	if !ok {
		return nil
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
		if _, ok := pvc.Annotations[cc.AnnCurrentCheckpoint]; ok {
			if err := r.updatesMultistageImportSucceeded(pvc, dataVolumeCopy); err != nil {
				return err
			}
			// this is a multistage import, set the datavolume status to paused
			dataVolumeCopy.Status.Phase = cdiv1.Paused
			event.eventType = corev1.EventTypeNormal
			event.reason = ImportPaused
			event.message = fmt.Sprintf(MessageImportPaused, pvc.Name)
			break
		}
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
		dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress("100.0%")
		event.eventType = corev1.EventTypeNormal
		event.reason = ImportSucceeded
		event.message = fmt.Sprintf(MessageImportSucceeded, pvc.Name)
	}
	return nil
}

func (r ImportReconciler) updatesMultistageImportSucceeded(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume) error {
	if multiStageImport := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCurrentCheckpoint); !multiStageImport {
		return nil
	}
	// The presence of the current checkpoint annotation indicates it is a stage in a multistage import.
	// If all the checkpoints have been copied, then we need to remove the annotations from the PVC and
	// set the DataVolume status to Succeeded. Otherwise, we need to set the status to Paused to indicate that
	// the import is not yet done, and change the annotations to advance to the next checkpoint.
	currentCheckpoint := pvc.Annotations[cc.AnnCurrentCheckpoint]
	alreadyCopied := r.checkpointAlreadyCopied(pvc, currentCheckpoint)
	finalCheckpoint, _ := strconv.ParseBool(pvc.Annotations[cc.AnnFinalCheckpoint])

	if finalCheckpoint && alreadyCopied {
		// Last checkpoint done, so clean up and mark DV success
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
		if err := r.deleteMultistageImportAnnotations(pvc); err != nil {
			return err
		}
	} else {
		// Single stage of a multi-stage import
		dataVolumeCopy.Status.Phase = cdiv1.Paused
		// Advances annotations to next checkpoint
		if err := r.setMultistageImportAnnotations(dataVolumeCopy, pvc); err != nil {
			return err
		}
	}
	return nil
}

func (r *ImportReconciler) updateCheckpoint(datavolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) {
	checkpoint := r.getNextCheckpoint(datavolume, pvc)
	if checkpoint != nil {
		pvc.ObjectMeta.Annotations[cc.AnnCurrentCheckpoint] = checkpoint.Current
		pvc.ObjectMeta.Annotations[cc.AnnPreviousCheckpoint] = checkpoint.Previous
		pvc.ObjectMeta.Annotations[cc.AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)
	}
}

func (r *ImportReconciler) getVddkAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	var dataVolumeCopy = dataVolume.DeepCopy()
	if vddkHost := pvc.Annotations[cc.AnnVddkHostConnection]; vddkHost != "" {
		cc.AddAnnotation(dataVolumeCopy, cc.AnnVddkHostConnection, vddkHost)
	}
	if vddkVersion := pvc.Annotations[cc.AnnVddkVersion]; vddkVersion != "" {
		cc.AddAnnotation(dataVolumeCopy, cc.AnnVddkVersion, vddkVersion)
	}

	// only update if something has changed
	if !reflect.DeepEqual(dataVolume, dataVolumeCopy) {
		return true, r.updateDataVolume(dataVolumeCopy)
	}
	return false, nil
}

// Sets the annotation if pvc needs it, and does not have it yet
func (r *ImportReconciler) maybeSetMultiStageAnnotation(pvc *corev1.PersistentVolumeClaim, datavolume *cdiv1.DataVolume) error {
	if pvc.Status.Phase == corev1.ClaimBound {
		// If a PVC already exists with no multi-stage annotations, check if it
		// needs them set (if not already finished with an import).
		multiStageImport := (len(datavolume.Spec.Checkpoints) > 0)
		multiStageAnnotationsSet := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCurrentCheckpoint)
		multiStageAlreadyDone := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnMultiStageImportDone)
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
func (r *ImportReconciler) setMultistageImportAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	pvcCopy := pvc.DeepCopy()

	// Only mark this checkpoint complete if it was completed by the current pod.
	// This keeps us from skipping over checkpoints when a reconcile fails at a bad time.
	uuidAlreadyUsed := false
	for key, value := range pvcCopy.Annotations {
		if strings.HasPrefix(key, r.getCheckpointCopiedKey("")) { // Blank checkpoint name to get just the prefix
			if value == pvcCopy.Annotations[cc.AnnCurrentPodID] {
				uuidAlreadyUsed = true
				break
			}
		}
	}
	if !uuidAlreadyUsed {
		// Mark checkpoint complete by saving UID of current pod to a
		// PVC annotation specific to this checkpoint.
		currentCheckpoint := pvcCopy.Annotations[cc.AnnCurrentCheckpoint]
		if currentCheckpoint != "" {
			currentPodID := pvcCopy.Annotations[cc.AnnCurrentPodID]
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
		pvcCopy.ObjectMeta.Annotations[cc.AnnCurrentCheckpoint] = checkpoint.Current
		pvcCopy.ObjectMeta.Annotations[cc.AnnPreviousCheckpoint] = checkpoint.Previous
		pvcCopy.ObjectMeta.Annotations[cc.AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)

		// Check to see if there is a running pod for this PVC. If there are
		// more checkpoints to copy but the PVC is stopped in Succeeded,
		// reset the phase to get another pod started for the next checkpoint.
		var podNamespace string
		if dataVolume.Spec.Source.PVC != nil {
			podNamespace = dataVolume.Spec.Source.PVC.Namespace
		} else {
			podNamespace = dataVolume.Namespace
		}
		phase := pvcCopy.ObjectMeta.Annotations[cc.AnnPodPhase]
		pod, _ := r.getPodFromPvc(podNamespace, pvcCopy)
		if pod == nil && phase == string(corev1.PodSucceeded) {
			// Reset PVC phase so importer will create a new pod
			pvcCopy.ObjectMeta.Annotations[cc.AnnPodPhase] = string(corev1.PodUnknown)
			delete(pvcCopy.ObjectMeta.Annotations, cc.AnnImportPod)
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
func (r *ImportReconciler) deleteMultistageImportAnnotations(pvc *corev1.PersistentVolumeClaim) error {
	pvcCopy := pvc.DeepCopy()
	delete(pvcCopy.Annotations, cc.AnnCurrentCheckpoint)
	delete(pvcCopy.Annotations, cc.AnnPreviousCheckpoint)
	delete(pvcCopy.Annotations, cc.AnnFinalCheckpoint)
	delete(pvcCopy.Annotations, cc.AnnCurrentPodID)

	prefix := r.getCheckpointCopiedKey("")
	for key := range pvcCopy.Annotations {
		if strings.HasPrefix(key, prefix) {
			delete(pvcCopy.Annotations, key)
		}
	}

	pvcCopy.ObjectMeta.Annotations[cc.AnnMultiStageImportDone] = "true"

	// only update if something has changed
	if !reflect.DeepEqual(pvc, pvcCopy) {
		return r.updatePVC(pvcCopy)
	}
	return nil
}

// Single place to hold the scheme for annotations that indicate a checkpoint
// has already been copied. Currently storage.checkpoint.copied.[checkpoint] = ID,
// where ID is the UID of the pod that successfully transferred that checkpoint.
func (r *ImportReconciler) getCheckpointCopiedKey(checkpoint string) string {
	return cc.AnnCheckpointsCopied + "." + checkpoint
}

// Find out if this checkpoint has already been copied by looking for an annotation
// like storage.checkpoint.copied.[checkpoint]. If it exists, then this checkpoint
// was already copied.
func (r *ImportReconciler) checkpointAlreadyCopied(pvc *corev1.PersistentVolumeClaim, checkpoint string) bool {
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

func (r *ImportReconciler) getNextCheckpoint(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) *checkpointRecord {
	numCheckpoints := len(dataVolume.Spec.Checkpoints)
	if numCheckpoints < 1 {
		return nil
	}

	// If there are no annotations, get the first checkpoint from the spec
	if pvc.ObjectMeta.Annotations[cc.AnnCurrentCheckpoint] == "" {
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
