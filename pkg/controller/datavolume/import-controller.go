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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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

	volumeImportSourcePrefix = "volume-import-source"
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
			client:               client,
			scheme:               mgr.GetScheme(),
			log:                  log.WithName(importControllerName),
			recorder:             mgr.GetEventRecorderFor(importControllerName),
			featureGates:         featuregates.NewFeatureGates(client),
			installerLabels:      installerLabels,
			shouldUpdateProgress: true,
		},
	}

	datavolumeController, err := controller.New(importControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
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
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeImport); err != nil {
		return err
	}
	if err := datavolumeController.Watch(&source.Kind{Type: &cdiv1.VolumeImportSource{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}); err != nil {
		return err
	}
	return nil
}

func (r *ImportReconciler) updatePVCForPopulation(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.HTTP == nil &&
		dataVolume.Spec.Source.S3 == nil &&
		dataVolume.Spec.Source.GCS == nil &&
		dataVolume.Spec.Source.Registry == nil &&
		dataVolume.Spec.Source.Imageio == nil &&
		dataVolume.Spec.Source.VDDK == nil &&
		dataVolume.Spec.Source.Blank == nil {
		return errors.Errorf("no source set for import datavolume")
	}
	if err := cc.AddImmediateBindingAnnotationIfWFFCDisabled(pvc, r.featureGates); err != nil {
		return err
	}
	apiGroup := cc.AnnAPIGroup
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     cdiv1.VolumeImportSourceRef,
		Name:     volumeImportSourceName(dataVolume),
	}
	return nil
}

func (r *ImportReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	annotations := pvc.Annotations

	if checkpoint := r.getNextCheckpoint(dataVolume, pvc); checkpoint != nil {
		annotations[cc.AnnCurrentCheckpoint] = checkpoint.Current
		annotations[cc.AnnPreviousCheckpoint] = checkpoint.Previous
		annotations[cc.AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)
	}

	if http := dataVolume.Spec.Source.HTTP; http != nil {
		cc.UpdateHTTPAnnotations(annotations, http)
		return nil
	}
	if s3 := dataVolume.Spec.Source.S3; s3 != nil {
		cc.UpdateS3Annotations(annotations, s3)
		return nil
	}
	if gcs := dataVolume.Spec.Source.GCS; gcs != nil {
		cc.UpdateGCSAnnotations(annotations, gcs)
		return nil
	}
	if registry := dataVolume.Spec.Source.Registry; registry != nil {
		cc.UpdateRegistryAnnotations(annotations, registry)
		return nil
	}
	if imageio := dataVolume.Spec.Source.Imageio; imageio != nil {
		cc.UpdateImageIOAnnotations(annotations, imageio)
		return nil
	}
	if vddk := dataVolume.Spec.Source.VDDK; vddk != nil {
		cc.UpdateVDDKAnnotations(annotations, vddk)
		return nil
	}
	if dataVolume.Spec.Source.Blank != nil {
		annotations[cc.AnnSource] = cc.SourceNone
		return nil
	}
	return errors.Errorf("no source set for import datavolume")
}

// Reconcile loop for the import data volumes
func (r *ImportReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, req, r)
}

func (r *ImportReconciler) sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error) {
	syncState, err := r.syncImport(log, req)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState.dvSyncResult, err
}

func (r *ImportReconciler) syncImport(log logr.Logger, req reconcile.Request) (dvSyncState, error) {
	syncState, syncErr := r.syncCommon(log, req, r.cleanup, nil)
	if syncErr != nil || syncState.result != nil {
		return syncState, syncErr
	}

	pvcModifier := r.updateAnnotations
	if syncState.usePopulator {
		if syncState.dvMutated.Status.Phase != cdiv1.Succeeded {
			err := r.createVolumeImportSourceCR(&syncState)
			if err != nil {
				return syncState, err
			}
		}
		pvcModifier = r.updatePVCForPopulation
	}

	if err := r.handlePvcCreation(log, &syncState, pvcModifier); err != nil {
		syncErr = err
	}

	if syncState.pvc != nil && syncErr == nil {
		r.setVddkAnnotations(&syncState)
		syncErr = r.maybeSetPvcMultiStageAnnotation(syncState.pvc, syncState.dvMutated)
	}
	return syncState, syncErr
}

func (r *ImportReconciler) cleanup(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	// The cleanup is to delete the volumeImportSourceCR which is used only with populators,
	// it is owner by the DV so will be deleted when dv is deleted
	// also we can already delete once dv is succeeded
	usePopulator, err := checkDVUsingPopulators(syncState.dvMutated)
	if err != nil {
		return err
	}
	if usePopulator && dv.Status.Phase == cdiv1.Succeeded {
		return r.deleteVolumeImportSourceCR(syncState)
	}

	return nil
}

func isPVCImportPopulation(pvc *corev1.PersistentVolumeClaim) bool {
	return populators.IsPVCDataSourceRefKind(pvc, cdiv1.VolumeImportSourceRef)
}

func (r *ImportReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	importPopulation := isPVCImportPopulation(pvc)
	if phase != string(corev1.PodSucceeded) && !importPopulation {
		_, ok := pvc.Annotations[cc.AnnImportPod]
		if !ok || pvc.Status.Phase != corev1.ClaimBound || pvcIsPopulated(pvc, dataVolumeCopy) {
			return nil
		}
	}
	dataVolumeCopy.Status.Phase = cdiv1.ImportScheduled
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

func (r *ImportReconciler) setVddkAnnotations(syncState *dvSyncState) {
	if cc.GetSource(syncState.pvc) != cc.SourceVDDK {
		return
	}
	if vddkHost := syncState.pvc.Annotations[cc.AnnVddkHostConnection]; vddkHost != "" {
		cc.AddAnnotation(syncState.dvMutated, cc.AnnVddkHostConnection, vddkHost)
	}
	if vddkVersion := syncState.pvc.Annotations[cc.AnnVddkVersion]; vddkVersion != "" {
		cc.AddAnnotation(syncState.dvMutated, cc.AnnVddkVersion, vddkVersion)
	}
}

func (r *ImportReconciler) updatesMultistageImportSucceeded(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume) error {
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
		if err := r.setPvcMultistageImportAnnotations(dataVolumeCopy, pvc); err != nil {
			return err
		}
	}
	return nil
}

// Sets the annotation if pvc needs it, and does not have it yet
func (r *ImportReconciler) maybeSetPvcMultiStageAnnotation(pvc *corev1.PersistentVolumeClaim, datavolume *cdiv1.DataVolume) error {
	if pvc.Status.Phase == corev1.ClaimBound {
		// If a PVC already exists with no multi-stage annotations, check if it
		// needs them set (if not already finished with an import).
		multiStageImport := (len(datavolume.Spec.Checkpoints) > 0)
		multiStageAnnotationsSet := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCurrentCheckpoint)
		multiStageAlreadyDone := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnMultiStageImportDone)
		if multiStageImport && !multiStageAnnotationsSet && !multiStageAlreadyDone {
			err := r.setPvcMultistageImportAnnotations(datavolume, pvc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Set the PVC annotations related to multi-stage imports so that they point to the next checkpoint to copy.
func (r *ImportReconciler) setPvcMultistageImportAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
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

func volumeImportSourceName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("%s-%s", volumeImportSourcePrefix, dv.UID)
}

func (r *ImportReconciler) createVolumeImportSourceCR(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	importSource := &cdiv1.VolumeImportSource{}
	importSourceName := volumeImportSourceName(dv)

	// check if import source already exists
	if exists, err := cc.GetResource(context.TODO(), r.client, dv.Namespace, importSourceName, importSource); err != nil || exists {
		return err
	}

	source := &cdiv1.ImportSourceType{}
	if http := dv.Spec.Source.HTTP; http != nil {
		source.HTTP = http
	} else if s3 := dv.Spec.Source.S3; s3 != nil {
		source.S3 = s3
	} else if gcs := dv.Spec.Source.GCS; gcs != nil {
		source.GCS = gcs
	} else if registry := dv.Spec.Source.Registry; registry != nil {
		source.Registry = registry
	} else if imageio := dv.Spec.Source.Imageio; imageio != nil {
		source.Imageio = imageio
	} else if vddk := dv.Spec.Source.VDDK; vddk != nil {
		source.VDDK = vddk
	} else {
		// Our dv shouldn't be without source
		// Defaulting to Blank source
		source.Blank = &cdiv1.DataVolumeBlankImage{}
	}

	importSource = &cdiv1.VolumeImportSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      importSourceName,
			Namespace: dv.Namespace,
		},
		Spec: cdiv1.VolumeImportSourceSpec{
			Source:        source,
			ContentType:   dv.Spec.ContentType,
			Preallocation: dv.Spec.Preallocation,
		},
	}

	if err := controllerutil.SetControllerReference(dv, importSource, r.scheme); err != nil {
		return err
	}

	if err := r.client.Create(context.TODO(), importSource); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (r *ImportReconciler) deleteVolumeImportSourceCR(syncState *dvSyncState) error {
	importSourceName := volumeImportSourceName(syncState.dvMutated)
	importSource := &cdiv1.VolumeImportSource{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: importSourceName, Namespace: syncState.dvMutated.Namespace}, importSource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		if err := r.client.Delete(context.TODO(), importSource); err != nil {
			if !k8serrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
