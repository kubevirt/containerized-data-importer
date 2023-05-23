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
func NewPopulatorController(ctx context.Context, mgr manager.Manager, log logr.Logger, installerLabels map[string]string) (controller.Controller, error) {
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
func (r *PopulatorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.reconcile(ctx, req, r)
}

// If dataSourceRef is set (external populator), use an empty spec.Source field
func (r *PopulatorReconciler) prepare(syncState *dvSyncState) error {
	if !dvUsesVolumePopulator(syncState.dv) {
		return errors.Errorf("undefined population source")
	}
	// TODO - let's revisit this
	syncState.dv.Spec.Source = &cdiv1.DataVolumeSource{}
	syncState.dvMutated.Spec.Source = &cdiv1.DataVolumeSource{}
	return nil
}

// checkPopulationRequirements returns true if the PVC meets the requirements to be populated
func (r *PopulatorReconciler) checkPopulationRequirements(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, event *Event) (bool, error) {
	csiDriverAvailable, err := storageClassCSIDriverExists(r.client, r.log, pvc.Spec.StorageClassName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, err
	}
	// Non-snapshot population will only work with CSI drivers
	if !isSnapshotPopulation(pvc) && !csiDriverAvailable {
		r.recorder.Event(pvc, corev1.EventTypeWarning, NoCSIDriverForExternalPopulation, MessageNoCSIDriverForExternalPopulation)
		event.reason = NoCSIDriverForExternalPopulation
		event.message = MessageNoCSIDriverForExternalPopulation
		event.eventType = corev1.EventTypeWarning
		return false, nil
	}

	return r.checkAnyVolumeDataSource(pvc, dv, event), nil
}

// checkAnyVolumeDataSource determines if the AnyVolumeDataSource feature gate is required/enabled
func (r *PopulatorReconciler) checkAnyVolumeDataSource(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, event *Event) bool {
	// We don't need the AnyVolumeDataSource feature gate if
	// the PVC is externally populated by a PVC or Snapshot.
	nonPopulator := isPvcPopulation(pvc) || isSnapshotPopulation(pvc)

	// If the AnyVolumeDataSource feature gate is disabled, Kubernetes drops
	// the contents of the dataSourceRef field. We can then determine if the
	// feature is enabled or not by checking that field after creating the PVC.
	enabled := pvc.Spec.DataSourceRef != nil

	populationSupported := enabled || nonPopulator
	if !populationSupported {
		r.recorder.Event(pvc, corev1.EventTypeWarning, NoAnyVolumeDataSource, MessageNoAnyVolumeDataSource)
		event.reason = NoAnyVolumeDataSource
		event.message = MessageNoAnyVolumeDataSource
		event.eventType = corev1.EventTypeWarning
	} else {
		event.reason = ExternalPopulationSucceeded
		event.message = fmt.Sprintf(MessageExternalPopulationSucceeded, pvc.Name, getPopulatorName(pvc))
		event.eventType = corev1.EventTypeNormal
	}

	return populationSupported
}

// getPopulatorName returns the name of the passed PVC's populator
func getPopulatorName(pvc *corev1.PersistentVolumeClaim) string {
	var populatorName string
	if pvc.Spec.DataSourceRef != nil {
		populatorName = pvc.Spec.DataSourceRef.Name
	} else {
		populatorName = pvc.Spec.DataSource.Name
	}
	return populatorName
}

// dvUsesVolumePopulator returns true if the datavolume's PVC is meant to be externally populated
func dvUsesVolumePopulator(dv *cdiv1.DataVolume) bool {
	return isDataSourcePopulated(dv) || isDataSourceRefPopulated(dv)
}

// isDataSourcePopulated returns true if the DataVolume has a populated DataSource field
func isDataSourcePopulated(dv *cdiv1.DataVolume) bool {
	return (dv.Spec.PVC != nil && dv.Spec.PVC.DataSource != nil) ||
		(dv.Spec.Storage != nil && dv.Spec.Storage.DataSource != nil)
}

// isDataSourceRefPopulated returns true if the DataVolume has a populated DataSourceRef field
func isDataSourceRefPopulated(dv *cdiv1.DataVolume) bool {
	return (dv.Spec.PVC != nil && dv.Spec.PVC.DataSourceRef != nil) ||
		(dv.Spec.Storage != nil && dv.Spec.Storage.DataSourceRef != nil)
}

// isPvcPopulation returns true if a PVC's population source is of PVC kind
func isPvcPopulation(pvc *corev1.PersistentVolumeClaim) bool {
	return (pvc.Spec.DataSource != nil && pvc.Spec.DataSource.Kind == "PersistentVolumeClaim") ||
		(pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == "PersistentVolumeClaim")
}

// isSnapshotPopulation returns true if a PVC's population source is of Snapshot kind
func isSnapshotPopulation(pvc *corev1.PersistentVolumeClaim) bool {
	return (pvc.Spec.DataSource != nil && pvc.Spec.DataSource.Kind == "VolumeSnapshot") ||
		(pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == "VolumeSnapshot")
}

// Generic controller functions

func (r *PopulatorReconciler) updateAnnotations(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if !dvUsesVolumePopulator(dataVolume) {
		return errors.Errorf("undefined population source")
	}
	pvc.Annotations[cc.AnnExternalPopulation] = "true"
	return nil
}

func (r *PopulatorReconciler) sync(log logr.Logger, req reconcile.Request) (dvSyncResult, error) {
	syncState, err := r.syncExternalPopulation(log, req)
	if err == nil {
		err = r.syncUpdate(log, &syncState)
	}
	return syncState.dvSyncResult, err
}

func (r *PopulatorReconciler) syncExternalPopulation(log logr.Logger, req reconcile.Request) (dvSyncState, error) {
	syncState, syncErr := r.syncCommon(log, req, nil, r.prepare)
	if syncErr != nil || syncState.result != nil {
		return syncState, syncErr
	}
	if err := r.handlePvcCreation(log, &syncState, r.updateAnnotations); err != nil {
		syncErr = err
	}
	return syncState, syncErr
}

func (r *PopulatorReconciler) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	// * Population by Snapshots doesn't have additional requirements.
	// * Population by PVC requires CSI drivers.
	// * Population by external populators requires both CSI drivers and the AnyVolumeDataSource feature gate.
	if supported, err := r.checkPopulationRequirements(pvc, dataVolumeCopy, event); err != nil {
		return err
	} else if supported {
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
	}
	return nil
}
