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
	// UploadScheduled provides a const to indicate upload is scheduled
	UploadScheduled = "UploadScheduled"
	// UploadReady provides a const to indicate upload is in progress
	UploadReady = "UploadReady"
	// UploadFailed provides a const to indicate upload has failed
	UploadFailed = "UploadFailed"
	// UploadSucceeded provides a const to indicate upload has succeeded
	UploadSucceeded = "UploadSucceeded"

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

	uploadControllerName = "datavolume-upload-controller"

	volumeUploadSourcePrefix = "volume-upload-source"
)

// UploadReconciler members
type UploadReconciler struct {
	ReconcilerBase
}

// NewUploadController creates a new instance of the datavolume upload controller
func NewUploadController(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &UploadReconciler{
		ReconcilerBase: ReconcilerBase{
			client:               client,
			scheme:               mgr.GetScheme(),
			log:                  log.WithName(uploadControllerName),
			recorder:             mgr.GetEventRecorderFor(uploadControllerName),
			featureGates:         featuregates.NewFeatureGates(client),
			installerLabels:      installerLabels,
			shouldUpdateProgress: false,
		},
	}

	datavolumeController, err := controller.New(uploadControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addDataVolumeUploadControllerWatches(mgr, datavolumeController); err != nil {
		return nil, err
	}

	return datavolumeController, nil
}

func addDataVolumeUploadControllerWatches(mgr manager.Manager, datavolumeController controller.Controller) error {
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeUpload); err != nil {
		return err
	}
	if err := datavolumeController.Watch(&source.Kind{Type: &cdiv1.VolumeUploadSource{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}); err != nil {
		return err
	}
	return nil
}

func (r *UploadReconciler) updatePVCForPopulation(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.Upload == nil {
		return errors.Errorf("no source set for upload datavolume")
	}
	if err := cc.AddImmediateBindingAnnotationIfWFFCDisabled(pvc, r.featureGates); err != nil {
		return err
	}
	apiGroup := cc.AnnAPIGroup
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     cdiv1.VolumeUploadSourceRef,
		Name:     volumeUploadSourceName(dataVolume),
	}
	return nil
}

func (r *UploadReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.Upload == nil {
		return errors.Errorf("no source set for upload datavolume")
	}
	pvc.Annotations[cc.AnnUploadRequest] = ""
	return nil
}

// Reconcile loop for the upload data volumes
func (r *UploadReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, req, r)
}

func (r *UploadReconciler) sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error) {
	syncState, err := r.syncUpload(log, req)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState.dvSyncResult, err
}

func (r *UploadReconciler) syncUpload(log logr.Logger, req reconcile.Request) (dvSyncState, error) {
	syncState, syncErr := r.syncCommon(log, req, r.cleanup, nil)
	if syncErr != nil || syncState.result != nil {
		return syncState, syncErr
	}

	pvcModifier := r.updateAnnotations
	if syncState.usePopulator {
		if syncState.dvMutated.Status.Phase != cdiv1.Succeeded {
			err := r.createVolumeUploadSourceCR(&syncState)
			if err != nil {
				return syncState, err
			}
		}
		pvcModifier = r.updatePVCForPopulation
	}

	if err := r.handlePvcCreation(log, &syncState, pvcModifier); err != nil {
		syncErr = err
	}
	return syncState, syncErr
}

func (r *UploadReconciler) cleanup(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	// The cleanup is to delete the volumeUploadSourceCR which is used only with populators,
	// it is owner by the DV so will be deleted when dv is deleted
	// also we can already delete once dv is succeeded
	usePopulator, err := checkDVUsingPopulators(syncState.dvMutated)
	if err != nil {
		return err
	}
	if usePopulator && dv.Status.Phase == cdiv1.Succeeded {
		return r.deleteVolumeUploadSourceCR(syncState)
	}

	return nil
}

func isPVCUploadPopulation(pvc *corev1.PersistentVolumeClaim) bool {
	return populators.IsPVCDataSourceRefKind(pvc, cdiv1.VolumeUploadSourceRef)
}

func (r *UploadReconciler) shouldUpdateStatusPhase(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) (bool, error) {
	pvcCopy := pvc.DeepCopy()
	if isPVCUploadPopulation(pvcCopy) {
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
	_, ok := pvcCopy.Annotations[cc.AnnUploadRequest]
	requiresWork, err := r.pvcRequiresWork(pvcCopy, dv)
	if err != nil {
		return false, err
	}
	return ok && pvcCopy.Status.Phase == corev1.ClaimBound && requiresWork, nil
}

func (r *UploadReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		update, err := r.shouldUpdateStatusPhase(pvc, dataVolumeCopy)
		if !update || err != nil {
			return err
		}
	}
	dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
	if !ok {
		return nil
	}

	switch phase {
	case string(corev1.PodPending):
		// TODO: Use a more generic Scheduled, like maybe TransferScheduled.
		dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
		event.eventType = corev1.EventTypeNormal
		event.reason = UploadScheduled
		event.message = fmt.Sprintf(MessageUploadScheduled, pvc.Name)
	case string(corev1.PodRunning):
		running := pvc.Annotations[cc.AnnPodReady]
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
	return nil
}

func volumeUploadSourceName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("%s-%s", volumeUploadSourcePrefix, dv.UID)
}

func (r *UploadReconciler) createVolumeUploadSourceCR(syncState *dvSyncState) error {
	uploadSourceName := volumeUploadSourceName(syncState.dvMutated)
	uploadSource := &cdiv1.VolumeUploadSource{}
	// check if uploadSource already exists
	if exists, err := cc.GetResource(context.TODO(), r.client, syncState.dvMutated.Namespace, uploadSourceName, uploadSource); err != nil || exists {
		return err
	}

	uploadSource = &cdiv1.VolumeUploadSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uploadSourceName,
			Namespace: syncState.dv.Namespace,
		},
		Spec: cdiv1.VolumeUploadSourceSpec{
			ContentType:   syncState.dv.Spec.ContentType,
			Preallocation: syncState.dv.Spec.Preallocation,
		},
	}

	if err := controllerutil.SetControllerReference(syncState.dvMutated, uploadSource, r.scheme); err != nil {
		return err
	}
	if err := r.client.Create(context.TODO(), uploadSource); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (r *UploadReconciler) deleteVolumeUploadSourceCR(syncState *dvSyncState) error {
	uploadSourceName := volumeUploadSourceName(syncState.dvMutated)
	uploadSource := &cdiv1.VolumeUploadSource{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: uploadSourceName, Namespace: syncState.dvMutated.Namespace}, uploadSource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		if err := r.client.Delete(context.TODO(), uploadSource); err != nil {
			if !k8serrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
