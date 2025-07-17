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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	primePvcPrefix = "prime"

	// errCreatingPVCPrime provides a const to indicate we failed to create PVC prime for population
	errCreatingPVCPrime = "ErrCreatingPVCPrime"

	// createdPVCPrimeSuccessfully provides a const to indicate we created PVC prime for population (reason)
	createdPVCPrimeSuccessfully = "CreatedPVCPrimeSuccessfully"
	// messageCreatedPVCPrimeSuccessfully provides a const to indicate we created PVC prime for population (message)
	messageCreatedPVCPrimeSuccessfully = "PVC Prime created successfully"

	// retainedPVCPrime provides a const to indicate that the PVC prime has been retained in lost state (reason)
	retainedPVCPrime = "RetainedPVCPrime"
	// messageRetainedPVCPrime provides a const to indicate that the PVC prime has been retained in lost state (message)
	messageRetainedPVCPrime = "PVC Prime retained in Lost state for debugging purposes"

	// annMigratedTo annotation is added to a PVC and PV that is supposed to be
	// dynamically provisioned/deleted by its corresponding CSI driver
	// through the CSIMigration feature flags. When this annotation is set the
	// Kubernetes components will "stand-down" and the external-provisioner will
	// act on the objects
	annMigratedTo = "pv.kubernetes.io/migrated-to"
)

// IsPVCDataSourceRefKind returns if the PVC has a valid DataSourceRef that
// is equal to the given kind
func IsPVCDataSourceRefKind(pvc *corev1.PersistentVolumeClaim, kind string) bool {
	dataSourceRef := pvc.Spec.DataSourceRef
	return isDataSourceRefValid(dataSourceRef) && dataSourceRef.Kind == kind
}

func isDataSourceRefValid(dataSourceRef *corev1.TypedObjectReference) bool {
	return dataSourceRef != nil && dataSourceRef.APIGroup != nil &&
		*dataSourceRef.APIGroup == cc.AnnAPIGroup && dataSourceRef.Name != ""
}

func getPopulationSourceNamespace(pvc *corev1.PersistentVolumeClaim) string {
	namespace := pvc.GetNamespace()
	// The populator CR can be in a different namespace from the target PVC
	// if the CrossNamespaceVolumeDataSource feature gate is enabled in the
	// kube-apiserver and the kube-controller-manager.
	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef != nil && dataSourceRef.Namespace != nil && *dataSourceRef.Namespace != "" {
		namespace = *pvc.Spec.DataSourceRef.Namespace
	}
	return namespace
}

func isPVCPrimeDataSourceRefKind(pvc *corev1.PersistentVolumeClaim, kind string) bool {
	owner := metav1.GetControllerOf(pvc)
	if owner == nil || owner.Kind != "PersistentVolumeClaim" {
		return false
	}
	populatorKind := pvc.Annotations[cc.AnnPopulatorKind]
	return populatorKind == kind
}

// PVCPrimeName returns the name of the PVC' of a given pvc
func PVCPrimeName(targetPVC *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-%s", primePvcPrefix, targetPVC.UID)
}

func getPopulatorIndexKey(apiGroup, kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s/%s", apiGroup, kind, namespace, name)
}

func claimReadyForPopulation(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (bool, string, error) {
	if pvc.Spec.StorageClassName == nil {
		// maybe storageclass will get assigned later
		return false, "", nil
	}

	if *pvc.Spec.StorageClassName == "" {
		return false, "", fmt.Errorf("PVC %s/%s has empty storage class name", pvc.Namespace, pvc.Name)
	}

	nodeName := ""
	storageClass, err := cc.GetStorageClassByNameWithK8sFallback(ctx, c, pvc.Spec.StorageClassName)
	if err != nil {
		return false, nodeName, err
	}

	if storageClass == nil {
		return false, nodeName, fmt.Errorf("storage class %s not found", *pvc.Spec.StorageClassName)
	}

	if checkIntreeStorageClass(pvc, storageClass) {
		return false, nodeName, fmt.Errorf("can't use populator for PVC %s/%s with in-tree storage class %s", pvc.Namespace, pvc.Name, storageClass.Provisioner)
	}

	if storageClass.VolumeBindingMode != nil && *storageClass.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
		nodeName = pvc.Annotations[cc.AnnSelectedNode]
		isImmediateBindingRequested := cc.ImmediateBindingRequested(pvc)
		if nodeName == "" && !isImmediateBindingRequested {
			// Wait for the PVC to get a node name before continuing
			return false, nodeName, nil
		}
	}

	return true, nodeName, nil
}

func checkIntreeStorageClass(pvc *corev1.PersistentVolumeClaim, sc *storagev1.StorageClass) bool {
	if !strings.HasPrefix(sc.Provisioner, "kubernetes.io/") {
		// This is not an in-tree StorageClass
		return false
	}

	if pvc.Annotations != nil {
		if migrated := pvc.Annotations[annMigratedTo]; migrated != "" {
			// The PVC is migrated to CSI
			return false
		}
	}

	// The SC is in-tree & PVC is not migrated
	return true
}
