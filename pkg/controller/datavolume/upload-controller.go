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
	v1 "k8s.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(uploadControllerName),
			recorder:        mgr.GetEventRecorderFor(uploadControllerName),
			featureGates:    featuregates.NewFeatureGates(client),
			installerLabels: installerLabels,
		},
	}
	reconciler.Reconciler = reconciler

	datavolumeController, err := controller.New(uploadControllerName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumeUpload); err != nil {
		return nil, err
	}
	return datavolumeController, nil
}

func (r UploadReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, annotations map[string]string) error {
	if dataVolume.Spec.Source.Upload == nil {
		return errors.Errorf("no source set for upload datavolume")
	}
	annotations[cc.AnnUploadRequest] = ""
	return nil
}

func (r UploadReconciler) updateStatus(log logr.Logger, syncRes dataVolumeSyncResult, syncErr error) (reconcile.Result, error) {
	if syncErr != nil {
		return reconcile.Result{}, syncErr
	}
	if syncRes.res != nil {
		return *syncRes.res, nil
	}
	//FIXME: pass syncRes instead of args
	if err := r.reconcileUpload(log, syncRes.dv, &syncRes.pvc, syncRes.pvcSpec); err != nil {
		return reconcile.Result{}, err
	}
	return r.reconcileDataVolumeStatus(syncRes.dv, syncRes.pvc, nil, r.updateStatusPhase)
}

func (r *UploadReconciler) reconcileUpload(log logr.Logger, dv *cdiv1.DataVolume, pvc **corev1.PersistentVolumeClaim, pvcSpec *v1.PersistentVolumeClaimSpec) error {
	if _, dvPrePopulated := dv.Annotations[cc.AnnPrePopulated]; dvPrePopulated {
		return nil
	}
	if *pvc != nil {
		return nil
	}

	newPvc, err := r.createPvcForDatavolume(dv, pvcSpec, nil)
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

func (r UploadReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		_, ok = pvc.Annotations[cc.AnnUploadRequest]
		if !ok || pvc.Status.Phase != corev1.ClaimBound || pvcIsPopulated(pvc, dataVolumeCopy) {
			return nil
		}
		dataVolumeCopy.Status.Phase = cdiv1.UploadScheduled
	}
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
