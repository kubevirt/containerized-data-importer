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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
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

	// MessageImportScheduled provides a const to form import is scheduled message
	MessageImportScheduled = "Import into %s scheduled"
	// MessageImportInProgress provides a const to form import is in progress message
	MessageImportInProgress = "Import into %s in progress"
	// MessageImportFailed provides a const to form import has failed message
	MessageImportFailed = "Failed to import into PVC %s"
	// MessageImportSucceeded provides a const to form import has succeeded message
	MessageImportSucceeded = "Successfully imported into PVC %s"

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
	if err := datavolumeController.Watch(source.Kind(mgr.GetCache(), &cdiv1.VolumeImportSource{}, handler.TypedEnqueueRequestForOwner[*cdiv1.VolumeImportSource](
		mgr.GetScheme(), mgr.GetClient().RESTMapper(), &cdiv1.DataVolume{}, handler.OnlyControllerOwner()))); err != nil {
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

	if checkpoint := cc.GetNextCheckpoint(pvc, r.getCheckpointArgs(dataVolume)); checkpoint != nil {
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
		if r.shouldReconcileVolumeSourceCR(&syncState) {
			err := r.reconcileVolumeImportSourceCR(&syncState)
			if err != nil {
				return syncState, err
			}
		}
		pvcModifier = r.updatePVCForPopulation
	}

	if err := r.handlePvcCreation(log, &syncState, pvcModifier); err != nil {
		syncErr = err
	}

	if syncState.pvc != nil && syncErr == nil && !syncState.usePopulator {
		r.setVddkAnnotations(&syncState)
		syncErr = cc.MaybeSetPvcMultiStageAnnotation(syncState.pvc, r.getCheckpointArgs(syncState.dvMutated))
	}
	return syncState, syncErr
}

func (r *ImportReconciler) cleanup(syncState *dvSyncState) error {
	// The cleanup is to delete the volumeImportSourceCR which is used only with populators,
	// it is owner by the DV so will be deleted when dv is deleted
	// also we can already delete once dv is succeeded
	usePopulator, err := checkDVUsingPopulators(syncState.dvMutated)
	if err != nil {
		return err
	}
	if usePopulator && !r.shouldReconcileVolumeSourceCR(syncState) {
		return r.deleteVolumeImportSourceCR(syncState)
	}

	return nil
}

func isPVCImportPopulation(pvc *corev1.PersistentVolumeClaim) bool {
	return populators.IsPVCDataSourceRefKind(pvc, cdiv1.VolumeImportSourceRef)
}

func (r *ImportReconciler) shouldUpdateStatusPhase(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (bool, error) {
	pvcCopy := pvc.DeepCopy()
	requiresWork, err := r.pvcRequiresWork(pvcCopy, dv)
	if err != nil {
		return false, err
	}
	if isPVCImportPopulation(pvcCopy) {
		// Better to play it safe and check the PVC Prime too
		// before updating DV phase.
		nn := types.NamespacedName{Namespace: pvcCopy.Namespace, Name: populators.PVCPrimeName(pvcCopy)}
		err := r.client.Get(context.TODO(), nn, pvcCopy)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
	}
	_, ok := pvcCopy.Annotations[cc.AnnImportPod]
	return ok && pvcCopy.Status.Phase == corev1.ClaimBound && requiresWork, nil
}

func (r *ImportReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		update, err := r.shouldUpdateStatusPhase(pvc, dataVolumeCopy)
		if !update || err != nil {
			return err
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
		if cc.IsMultiStageImportInProgress(pvc) {
			// Multi-stage annotations will be updated by import-populator if populators are in use
			if !isPVCImportPopulation(pvc) {
				if err := cc.UpdatesMultistageImportSucceeded(pvc, r.getCheckpointArgs(dataVolumeCopy)); err != nil {
					return err
				}
			}
			// this is a multistage import, set the datavolume status to paused
			dataVolumeCopy.Status.Phase = cdiv1.Paused
			event.eventType = corev1.EventTypeNormal
			event.reason = cc.ImportPaused
			event.message = fmt.Sprintf(cc.MessageImportPaused, pvc.Name)
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

func (r *ImportReconciler) getCheckpointArgs(dv *cdiv1.DataVolume) *cc.CheckpointArgs {
	return &cc.CheckpointArgs{
		Checkpoints: dv.Spec.Checkpoints,
		IsFinal:     dv.Spec.FinalCheckpoint,
		Client:      r.client,
		Log:         r.log,
	}
}

func volumeImportSourceName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("%s-%s", volumeImportSourcePrefix, dv.UID)
}

func (r *ImportReconciler) reconcileVolumeImportSourceCR(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	importSource := &cdiv1.VolumeImportSource{}
	importSourceName := volumeImportSourceName(dv)
	isMultiStage := dv.Spec.Source != nil && len(dv.Spec.Checkpoints) > 0 &&
		(dv.Spec.Source.VDDK != nil || dv.Spec.Source.Imageio != nil)

	// check if import source already exists
	if exists, err := cc.GetResource(context.TODO(), r.client, dv.Namespace, importSourceName, importSource); err != nil {
		return err
	} else if exists {
		return r.updateVolumeImportSourceIfNeeded(importSource, dv, isMultiStage)
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

	if isMultiStage {
		importSource.Spec.TargetClaim = &dv.Name
		importSource.Spec.Checkpoints = dv.Spec.Checkpoints
		importSource.Spec.FinalCheckpoint = &dv.Spec.FinalCheckpoint
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

func (r *ImportReconciler) updateVolumeImportSourceIfNeeded(source *cdiv1.VolumeImportSource, dv *cdiv1.DataVolume, isMultiStage bool) error {
	// Updates are only needed in multistage imports
	if !isMultiStage {
		return nil
	}

	// Unchanged checkpoint API, no update needed
	finalCheckpoint := false
	if source.Spec.FinalCheckpoint != nil {
		finalCheckpoint = *source.Spec.FinalCheckpoint
	}
	if reflect.DeepEqual(source.Spec.Checkpoints, dv.Spec.Checkpoints) &&
		finalCheckpoint == dv.Spec.FinalCheckpoint {
		return nil
	}

	source.Spec.Checkpoints = dv.Spec.Checkpoints
	source.Spec.FinalCheckpoint = &dv.Spec.FinalCheckpoint
	return r.client.Update(context.TODO(), source)
}
