package populators

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

// ReconcilerBase members
type ReconcilerBase struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	featureGates    featuregates.FeatureGates
	installerLabels map[string]string
}

func (r *ReconcilerBase) getPVCPrime(pvcPrimeName, namespace string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvcPrimeKey := types.NamespacedName{Namespace: namespace, Name: pvcPrimeName}
	if err := r.client.Get(context.TODO(), pvcPrimeKey, pvc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	return pvc, nil
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

func (r *ReconcilerBase) handleStorageClass(pvc *corev1.PersistentVolumeClaim) (bool, bool, error) {
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
		nodeName := pvc.Annotations[AnnSelectedNode]
		if nodeName == "" {
			// Wait for the PVC to get a node name before continuing
			return false, waitForFirstConsumer, nil
		}
	}

	return true, waitForFirstConsumer, nil
}

type pvcModifierFunc func(pvc *corev1.PersistentVolumeClaim, source client.Object)

func (r *ReconcilerBase) createPVCPrime(pvc *corev1.PersistentVolumeClaim, source client.Object, waitForFirstConsumer bool, pvcModifier pvcModifierFunc) (*corev1.PersistentVolumeClaim, error) {
	labels := map[string]string{
		common.CDILabelKey: common.CDILabelValue,
	}
	annotations := make(map[string]string)
	annotations[cc.AnnImmediateBinding] = ""
	if waitForFirstConsumer {
		annotations[AnnSelectedNode] = pvc.Annotations[AnnSelectedNode]
	}
	pvcPrimeName := PVCPrimeName(pvc)
	pvcPrime := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcPrimeName,
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

	if pvcModifier != nil {
		pvcModifier(pvcPrime, source)
	}
	if err := r.client.Create(context.TODO(), pvcPrime); err != nil {
		return nil, err
	}

	r.recorder.Eventf(pvc, corev1.EventTypeNormal, createdPVCPrimeSucceesfully, createdPVCPrimeSucceesfully)
	return pvcPrime, nil
}

func (r *ReconcilerBase) rebindPV(targetPVC, pvcPrime *corev1.PersistentVolumeClaim) error {
	pv, err := r.getPV(pvcPrime.Spec.VolumeName)
	if pv == nil {
		return err
	}

	// Examine the claimref for the PV and see if it's still bound to PVC'
	claimRef := pv.Spec.ClaimRef
	if claimRef.Name != pvcPrime.Name || claimRef.Namespace != pvcPrime.Namespace || claimRef.UID != pvcPrime.UID {
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

	const dataSourceRefField = "spec.dataSourceRef"

	indexKeyFunc := func(namespace, name string) string {
		return namespace + "/" + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.PersistentVolumeClaim{}, dataSourceRefField, func(obj client.Object) []string {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		if IsPVCDataSourceRefKind(pvc, sourceKind) && pvc.Spec.DataSourceRef.Name != "" {
			return []string{indexKeyFunc(obj.GetNamespace(), pvc.Spec.DataSourceRef.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	mapDataSourceRefToPVC := func(obj client.Object) (reqs []reconcile.Request) {
		var pvcs corev1.PersistentVolumeClaimList
		matchingFields := client.MatchingFields{dataSourceRefField: indexKeyFunc(obj.GetNamespace(), obj.GetName())}
		if err := mgr.GetClient().List(context.TODO(), &pvcs, matchingFields); err != nil {
			log.Error(err, "Unable to list PVCs", "matchingFields", matchingFields)
			return reqs
		}
		for _, pvc := range pvcs.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}})
		}
		return
	}

	if err := c.Watch(&source.Kind{Type: sourceType},
		handler.EnqueueRequestsFromMapFunc(mapDataSourceRefToPVC),
	); err != nil {
		return err
	}

	return nil
}
