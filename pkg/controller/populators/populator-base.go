/*
Copyright 2023 The CDI Authors.

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

package populators

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type pvcModifierFunc func(pvc *corev1.PersistentVolumeClaim, source client.Object)

// ReconcilerBase members
type ReconcilerBase struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	featureGates    featuregates.FeatureGates
	sourceKind      string
	installerLabels map[string]string
}

const (
	dataSourceRefField = "spec.dataSourceRef"
)

// Interface to store populator-specific methods
type populatorController interface {
	// Returns the specific populator CR
	getPopulationSource(namespace, name string) (client.Object, error)
	// Prepares the PVC' to be populated according to the population source
	updatePVCForPopulation(pvc *corev1.PersistentVolumeClaim, source client.Object)
	// Reconciles the target PVC with populator-specific logic
	reconcileTargetPVC(pvc, pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error)
}

// CreateCommonPopulatorIndexes creates indexes used by all populators
func CreateCommonPopulatorIndexes(mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.PersistentVolumeClaim{}, dataSourceRefField, func(obj client.Object) []string {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		dataSourceRef := pvc.Spec.DataSourceRef
		if dataSourceRef != nil && dataSourceRef.APIGroup != nil &&
			*dataSourceRef.APIGroup == cc.AnnAPIGroup && dataSourceRef.Name != "" {
			return []string{getPopulatorIndexKey(obj.GetNamespace(), pvc.Spec.DataSourceRef.Kind, pvc.Spec.DataSourceRef.Name)}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func addCommonPopulatorsWatches(mgr manager.Manager, c controller.Controller, log logr.Logger, sourceKind string, sourceType client.Object) error {
	// Setup watches
	if err := c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			if IsPVCDataSourceRefKind(pvc, sourceKind) {
				pvcKey := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
				return []reconcile.Request{{NamespacedName: pvcKey}}
			}
			if isPVCPrimeDataSourceRefKind(pvc, sourceKind) {
				owner := metav1.GetControllerOf(pvc)
				pvcKey := types.NamespacedName{Namespace: pvc.Namespace, Name: owner.Name}
				return []reconcile.Request{{NamespacedName: pvcKey}}
			}
			return nil
		}),
	); err != nil {
		return err
	}

	mapDataSourceRefToPVC := func(obj client.Object) (reqs []reconcile.Request) {
		var pvcs corev1.PersistentVolumeClaimList
		matchingFields := client.MatchingFields{dataSourceRefField: getPopulatorIndexKey(obj.GetNamespace(), sourceKind, obj.GetName())}
		if err := mgr.GetClient().List(context.TODO(), &pvcs, matchingFields); err != nil {
			log.Error(err, "Unable to list PVCs", "matchingFields", matchingFields)
			return reqs
		}
		for _, pvc := range pvcs.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}})
		}
		return reqs
	}

	if err := c.Watch(&source.Kind{Type: sourceType},
		handler.EnqueueRequestsFromMapFunc(mapDataSourceRefToPVC),
	); err != nil {
		return err
	}

	return nil
}

func (r *ReconcilerBase) getPVCPrime(pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	pvcPrime := &corev1.PersistentVolumeClaim{}
	pvcPrimeKey := types.NamespacedName{Namespace: pvc.Namespace, Name: PVCPrimeName(pvc)}
	if err := r.client.Get(context.TODO(), pvcPrimeKey, pvcPrime); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	return pvcPrime, nil
}

func (r *ReconcilerBase) getPV(volName string) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: volName}, pv); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pv, nil
}

func (r *ReconcilerBase) rebindPV(targetPVC, pvcPrime *corev1.PersistentVolumeClaim) error {
	pv, err := r.getPV(pvcPrime.Spec.VolumeName)
	if pv == nil {
		return err
	}

	// Examine the claimref for the PV and see if it's still bound to PVC'
	if !isPVBoundToPVC(pv, pvcPrime) {
		// Something is not right if the PV is neither bound to PVC' nor target PVC
		if !isPVBoundToPVC(pv, targetPVC) {
			klog.Errorf("PV bound to unexpected PVC: Could not rebind to target PVC '%s'", targetPVC.Name)
		}
		return nil
	}

	// Rebind PVC to target PVC
	pv.Annotations = make(map[string]string)
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace:       targetPVC.Namespace,
		Name:            targetPVC.Name,
		UID:             targetPVC.UID,
		ResourceVersion: targetPVC.ResourceVersion,
	}
	r.log.V(3).Info("Rebinding PV to target PVC", "PVC", targetPVC.Name)
	if err := r.client.Update(context.TODO(), pv); err != nil {
		return err
	}

	return nil
}

func (r *ReconcilerBase) handleStorageClass(pvc *corev1.PersistentVolumeClaim) (bool, bool, error) {
	if pvc.Spec.StorageClassName == nil {
		return true, false, nil
	}

	waitForFirstConsumer := false
	storageClass, err := cc.GetStorageClassByName(r.client, pvc.Spec.StorageClassName)
	if err != nil {
		return false, waitForFirstConsumer, err
	}

	if checkIntreeStorageClass(pvc, storageClass) {
		r.log.V(2).Info("can't use populator for PVC with in-tree storage class", "namespace", pvc.Namespace, "name", pvc.Name, "provisioner", storageClass.Provisioner)
		return false, waitForFirstConsumer, nil
	}

	if storageClass.VolumeBindingMode != nil && *storageClass.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
		waitForFirstConsumer = true
		nodeName := pvc.Annotations[cc.AnnSelectedNode]
		if nodeName == "" {
			// Wait for the PVC to get a node name before continuing
			return false, waitForFirstConsumer, nil
		}
	}

	return true, waitForFirstConsumer, nil
}

func (r *ReconcilerBase) createPVCPrime(pvc *corev1.PersistentVolumeClaim, source client.Object, waitForFirstConsumer bool, updatePVCForPopulation pvcModifierFunc) (*corev1.PersistentVolumeClaim, error) {
	labels := make(map[string]string)
	labels[common.CDILabelKey] = common.CDILabelValue
	annotations := make(map[string]string)
	annotations[cc.AnnImmediateBinding] = ""
	if waitForFirstConsumer {
		annotations[cc.AnnSelectedNode] = pvc.Annotations[cc.AnnSelectedNode]
	}

	// Assemble PVC' spec
	pvcPrime := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PVCPrimeName(pvc),
			Namespace:   pvc.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      pvc.Spec.AccessModes,
			Resources:        pvc.Spec.Resources,
			StorageClassName: pvc.Spec.StorageClassName,
			VolumeMode:       pvc.Spec.VolumeMode,
		},
	}
	pvcPrime.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(pvc, schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "PersistentVolumeClaim",
		}),
	}
	util.SetRecommendedLabels(pvcPrime, r.installerLabels, "cdi-controller")

	requestedSize, _ := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	// disk or image size, inflate it with overhead
	requestedSize, err := cc.InflateSizeWithOverhead(r.client, requestedSize.Value(), &pvc.Spec)
	if err != nil {
		return nil, err
	}
	pvcPrime.Spec.Resources.Requests[corev1.ResourceStorage] = requestedSize

	// We use the populator-specific pvcModifierFunc to add required annotations
	if updatePVCForPopulation != nil {
		updatePVCForPopulation(pvcPrime, source)
	}

	if err := r.client.Create(context.TODO(), pvcPrime); err != nil {
		return nil, err
	}
	r.recorder.Eventf(pvc, corev1.EventTypeNormal, createdPVCPrimeSuccessfully, messageCreatedPVCPrimeSuccessfully)
	return pvcPrime, nil
}

// reconcile functions

func (r *ReconcilerBase) reconcile(req reconcile.Request, populator populatorController, log logr.Logger) (reconcile.Result, error) {
	// Get the target PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// We first perform the common reconcile steps.
	// We should only continue if we get a valid PVC'
	pvcPrime, err := r.reconcileCommon(pvc, populator, log)
	if err != nil || pvcPrime == nil {
		return reconcile.Result{}, err
	}

	// Each populator reconciles the target PVC in a different way
	if cc.IsUnbound(pvc) {
		return populator.reconcileTargetPVC(pvc, pvcPrime)
	}

	// Making sure to clean PVC'
	return r.reconcileCleanup(pvcPrime)
}

func (r *ReconcilerBase) reconcileCommon(pvc *corev1.PersistentVolumeClaim, populator populatorController, log logr.Logger) (*corev1.PersistentVolumeClaim, error) {
	if pvc.DeletionTimestamp != nil {
		log.V(1).Info("PVC being terminated, ignoring")
		return nil, nil
	}

	// We should ignore PVCs that aren't for this populator to handle
	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef == nil || !IsPVCDataSourceRefKind(pvc, r.sourceKind) || dataSourceRef.Name == "" {
		log.V(1).Info("reconciled unexpected PVC, ignoring")
		return nil, nil
	}
	// Wait until dataSourceRef exists
	populationSource, err := populator.getPopulationSource(pvc.Namespace, dataSourceRef.Name)
	if populationSource == nil {
		return nil, err
	}
	ready, waitForFirstConsumer, err := r.handleStorageClass(pvc)
	if !ready || err != nil {
		return nil, err
	}

	// Get the PVC'
	pvcPrime, err := r.getPVCPrime(pvc)
	if err != nil {
		return nil, err
	}

	// If PVC' doesn't exist and target PVC is not bound, we should create the PVC' to start the population.
	// We still return the nil PVC' as we'll get called again once PVC' exists.
	// If target PVC is bound, we don't really need to populate anything.
	if pvcPrime == nil && cc.IsUnbound(pvc) {
		_, err := r.createPVCPrime(pvc, populationSource, waitForFirstConsumer, populator.updatePVCForPopulation)
		if err != nil {
			r.recorder.Eventf(pvc, corev1.EventTypeWarning, errCreatingPVCPrime, err.Error())
			return nil, err
		}
	}

	return pvcPrime, nil
}

func (r *ReconcilerBase) reconcileCleanup(pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	if pvcPrime != nil {
		if err := r.client.Delete(context.TODO(), pvcPrime); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
