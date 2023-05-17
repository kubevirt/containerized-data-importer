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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util"
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

	uidField = "metadata.uid"
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

type indexArgs struct {
	obj          client.Object
	field        string
	extractValue client.IndexerFunc
}

func getIndexArgs() []indexArgs {
	return []indexArgs{
		{
			obj:   &corev1.PersistentVolumeClaim{},
			field: dataSourceRefField,
			extractValue: func(obj client.Object) []string {
				pvc := obj.(*corev1.PersistentVolumeClaim)
				dataSourceRef := pvc.Spec.DataSourceRef
				if isDataSourceRefValid(dataSourceRef) {
					namespace := getPopulationSourceNamespace(pvc)
					apiGroup := *dataSourceRef.APIGroup
					return []string{getPopulatorIndexKey(apiGroup, dataSourceRef.Kind, namespace, dataSourceRef.Name)}
				}
				return nil
			},
		},
		{
			obj:   &corev1.PersistentVolumeClaim{},
			field: uidField,
			extractValue: func(obj client.Object) []string {
				return []string{string(obj.GetUID())}
			},
		},
	}
}

// CreateCommonPopulatorIndexes creates indexes used by all populators
func CreateCommonPopulatorIndexes(mgr manager.Manager) error {
	for _, ia := range getIndexArgs() {
		if err := mgr.GetFieldIndexer().IndexField(context.TODO(), ia.obj, ia.field, ia.extractValue); err != nil {
			return err
		}
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
		matchingFields := client.MatchingFields{
			dataSourceRefField: getPopulatorIndexKey(cc.AnnAPIGroup, sourceKind, obj.GetNamespace(), obj.GetName()),
		}
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

// TODO - this is only used in unit test - remove it
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

	// disk or image size, inflate it with overhead
	requestedSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	requestedSize, err := cc.InflateSizeWithOverhead(context.TODO(), r.client, requestedSize.Value(), &pvc.Spec)
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
	if !IsPVCDataSourceRefKind(pvc, r.sourceKind) {
		log.V(1).Info("reconciled unexpected PVC, ignoring")
		return nil, nil
	}
	// TODO: Remove this check once we support cross-namespace dataSourceRef
	if dataSourceRef.Namespace != nil {
		log.V(1).Info("cross-namespace dataSourceRef not supported yet, ignoring")
		return nil, nil
	}

	// Wait until dataSourceRef exists
	namespace := getPopulationSourceNamespace(pvc)
	populationSource, err := populator.getPopulationSource(namespace, dataSourceRef.Name)
	if populationSource == nil {
		return nil, err
	}
	// Check storage class
	ready, nodeName, err := claimReadyForPopulation(context.TODO(), r.client, pvc)
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
		_, err := r.createPVCPrime(pvc, populationSource, nodeName != "", populator.updatePVCForPopulation)
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
