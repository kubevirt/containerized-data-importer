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

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	// ExternalPopulationSucceeded provides a const to indicate that the external population of the PVC has succeeded (reason)
	ExternalPopulationSucceeded = "ExternalPopulationSucceeded"
	// MessageExternalPopulationSucceeded provides a const to indicate that the external population of the PVC has succeeded (message)
	MessageExternalPopulationSucceeded = "PVC %s successfully populated by %s"
	// NoAnyVolumeDataSource provides a const to indicate that the AnyVolumeDataSource feature gate is not enabled (reason)
	NoAnyVolumeDataSource = "NoAnyVolumeDataSource"
	// MessageNoAnyVolumeDataSource provides a const to indicate that the AnyVolumeDataSource feature gate is not enabled (message)
	MessageNoAnyVolumeDataSource = "AnyVolumeDataSource feature gate is not enabled: External population not supported"
	// NoCSIDriverForExternalPopulation provides a const to indicate that no CSI drivers were found for external population (reason)
	NoCSIDriverForExternalPopulation = "NoCSIDriverForExternalPopulation"
	// MessageNoCSIDriverForExternalPopulation provides a const to indicate that no CSI drivers were found for external population (message)
	MessageNoCSIDriverForExternalPopulation = "No CSI drivers were found: External population not supported"

	populatorControllerName = "datavolume-external-population-controller"
)

// PopulatorReconciler members
type PopulatorReconciler struct {
	ReconcilerBase
}

// NewPopulatorController creates a new instance of the datavolume external population controller
func NewPopulatorController(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &PopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(populatorControllerName),
			recorder:        mgr.GetEventRecorderFor(populatorControllerName),
			featureGates:    featuregates.NewFeatureGates(client),
			installerLabels: installerLabels,
		},
	}
	reconciler.Reconciler = reconciler

	datavolumeController, err := controller.New(populatorControllerName, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return nil, err
	}

	if err := addDataVolumeControllerCommonWatches(mgr, datavolumeController, dataVolumePopulator); err != nil {
		return nil, err
	}

	return datavolumeController, nil
}

// Reconcile loop for externally populated DataVolumes
func (r PopulatorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("DataVolume", req.NamespacedName)
	return r.updateStatus(r.sync(log, req))
}

// If dataSourceRef is set (external populator), use an empty spec.Source field
func (r PopulatorReconciler) prepare(syncRes *dataVolumeSyncResult) error {
	if !dvUsesExternalPopulator(syncRes.dv) {
		return errors.Errorf("undefined population source")
	}
	syncRes.dvMutated.Spec.Source = &cdiv1.DataVolumeSource{}
	return nil
}

// checkAnyVolumeDataSource determines if the AnyVolumeDataSource feature gate is enabled or not
func (r *PopulatorReconciler) checkAnyVolumeDataSource(pvc *corev1.PersistentVolumeClaim, event *Event) (bool, error) {
	csiDriverAvailable, err := r.storageClassCSIDriverExists(pvc.Spec.StorageClassName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, err
	}
	// AnyVolumeDataSource feature only works with CSI drivers
	if !csiDriverAvailable {
		r.recorder.Event(pvc, corev1.EventTypeWarning, NoCSIDriverForExternalPopulation, MessageNoCSIDriverForExternalPopulation)
		event.reason = NoCSIDriverForExternalPopulation
		event.message = MessageNoCSIDriverForExternalPopulation
		event.eventType = corev1.EventTypeWarning
		return false, nil
	}

	// If the AnyVolumeDataSource feature gate is disabled, Kubernetes drops the contents of the dataSourceRef field.
	// We can then determine if the feature is enabled or not by checking that field after creating the PVC.
	enabled := pvc.Spec.DataSourceRef != nil
	if !enabled {
		r.recorder.Event(pvc, corev1.EventTypeWarning, NoAnyVolumeDataSource, MessageNoAnyVolumeDataSource)
		event.reason = NoAnyVolumeDataSource
		event.message = MessageNoAnyVolumeDataSource
		event.eventType = corev1.EventTypeWarning
	} else {
		event.reason = ExternalPopulationSucceeded
		event.message = fmt.Sprintf(MessageExternalPopulationSucceeded, pvc.Name, pvc.Spec.DataSourceRef.Name)
		event.eventType = corev1.EventTypeNormal
	}

	return enabled, nil
}

// dvUsesExternalPopulator returns true if the datavolume's PVC is meant to be externally populated
func dvUsesExternalPopulator(dv *cdiv1.DataVolume) bool {
	if (dv.Spec.PVC != nil && (dv.Spec.PVC.DataSourceRef != nil || dv.Spec.PVC.DataSource != nil)) ||
		(dv.Spec.Storage != nil && (dv.Spec.Storage.DataSourceRef != nil || dv.Spec.Storage.DataSource != nil)) {
		return true
	}
	return false
}

// Generic controller functions

func (r PopulatorReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if !dvUsesExternalPopulator(dataVolume) {
		return errors.Errorf("undefined population source")
	}
	pvc.Annotations[cc.AnnExternalPopulation] = "true"
	return nil
}

func (r PopulatorReconciler) sync(log logr.Logger, req reconcile.Request) (dataVolumeSyncResult, error) {
	syncRes, syncErr := r.syncExternalPopulation(log, req)
	if err := r.syncUpdate(log, &syncRes); err != nil {
		syncErr = err
	}
	return syncRes, syncErr
}

func (r PopulatorReconciler) syncExternalPopulation(log logr.Logger, req reconcile.Request) (dataVolumeSyncResult, error) {
	syncRes, syncErr := r.syncCommon(log, req, nil, r.prepare)
	if syncErr != nil || syncRes.result != nil {
		return *syncRes, syncErr
	}
	if err := r.handlePvcCreation(log, syncRes, r.updateAnnotations); err != nil {
		syncErr = err
	}
	return *syncRes, syncErr
}

func (r PopulatorReconciler) updateStatus(syncRes dataVolumeSyncResult, syncErr error) (reconcile.Result, error) {
	if syncErr != nil {
		return getReconcileResult(syncRes.result), syncErr
	}
	res, err := r.updateStatusCommon(syncRes, r.updateStatusPhase)
	if err != nil {
		syncErr = err
	}
	return res, syncErr
}

func (r PopulatorReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	// The PVC will only be populated if the AnyVolumeDataSource feature is succesfully enabled
	if enabled, err := r.checkAnyVolumeDataSource(pvc, event); err != nil {
		return err
	} else if enabled {
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
	}
	return nil
}
