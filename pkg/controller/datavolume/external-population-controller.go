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
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
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
			client:               client,
			scheme:               mgr.GetScheme(),
			log:                  log.WithName(populatorControllerName),
			recorder:             mgr.GetEventRecorderFor(populatorControllerName),
			featureGates:         featuregates.NewFeatureGates(client),
			installerLabels:      installerLabels,
			shouldUpdateProgress: false,
		},
	}

	datavolumeController, err := controller.New(populatorControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
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
	if syncErr == nil && syncState.pvc != nil {
		if err := r.ensureImmediateBindingForWFFC(log, syncState.pvc); err != nil {
			syncErr = err
		}
	}
	return syncState, syncErr
}

// ensureImmediateBindingForWFFC sets the selected-node annotation on an externally
// populated PVC when immediate binding is requested and the storage class uses
// WaitForFirstConsumer binding mode. Without this, the PVC stays Pending because
// no consumer pod is created to trigger node selection in the external population path.
func (r *PopulatorReconciler) ensureImmediateBindingForWFFC(log logr.Logger, pvc *corev1.PersistentVolumeClaim) error {
	if pvc.Status.Phase != corev1.ClaimPending {
		return nil
	}
	if !cc.ImmediateBindingRequested(pvc) {
		return nil
	}
	if pvc.Annotations[cc.AnnSelectedNode] != "" {
		return nil
	}
	wffc, err := r.storageClassWaitForFirstConsumer(pvc.Spec.StorageClassName)
	if err != nil || !wffc {
		return err
	}

	nodeName, err := r.selectNodeForImmediateBinding(pvc)
	if err != nil {
		return err
	}
	if nodeName == "" {
		return nil
	}

	log.V(3).Info("Setting selected-node for immediate WFFC binding on externally populated PVC",
		"pvc", pvc.Name, "node", nodeName)
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[cc.AnnSelectedNode] = nodeName
	return r.client.Update(context.TODO(), pvc)
}

// selectNodeForImmediateBinding selects a node for WFFC binding.
// For PVC clones, it uses the node where the source PV is located.
// Otherwise, it picks any schedulable Ready node.
func (r *PopulatorReconciler) selectNodeForImmediateBinding(pvc *corev1.PersistentVolumeClaim) (string, error) {
	// For PVC-to-PVC clones, try to use the source PVC's node for data locality
	if isPvcPopulation(pvc) {
		sourceName := ""
		if pvc.Spec.DataSourceRef != nil {
			sourceName = pvc.Spec.DataSourceRef.Name
		} else if pvc.Spec.DataSource != nil {
			sourceName = pvc.Spec.DataSource.Name
		}
		if sourceName != "" {
			sourcePvc := &corev1.PersistentVolumeClaim{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      sourceName,
				Namespace: pvc.Namespace,
			}, sourcePvc); err == nil && sourcePvc.Spec.VolumeName != "" {
				pv := &corev1.PersistentVolume{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{
					Name: sourcePvc.Spec.VolumeName,
				}, pv); err == nil {
					if nodeName := nodeNameFromPV(pv); nodeName != "" {
						return nodeName, nil
					}
				}
			}
		}
	}

	// Fallback: pick any schedulable Ready node
	nodeList := &corev1.NodeList{}
	if err := r.client.List(context.TODO(), nodeList); err != nil {
		return "", err
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if isNodeSchedulable(node) {
			return node.Name, nil
		}
	}
	return "", nil
}

// nodeNameFromPV extracts a node name from a PersistentVolume's node affinity.
func nodeNameFromPV(pv *corev1.PersistentVolume) string {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return ""
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == corev1.LabelHostname && expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) > 0 {
				return expr.Values[0]
			}
		}
	}
	return ""
}

// isNodeSchedulable returns true if a node is Ready and not unschedulable.
func isNodeSchedulable(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		return false
	}
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
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
