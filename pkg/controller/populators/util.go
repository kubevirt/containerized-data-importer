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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	// annMigratedTo annotation is added to a PVC and PV that is supposed to be
	// dynamically provisioned/deleted by by its corresponding CSI driver
	// through the CSIMigration feature flags. When this annotation is set the
	// Kubernetes components will "stand-down" and the external-provisioner will
	// act on the objects
	annMigratedTo = "pv.kubernetes.io/migrated-to"
)

// IsPVCDataSourceRefKind returns if the PVC has DataSourceRef that
// is equal to the given kind
func IsPVCDataSourceRefKind(pvc *corev1.PersistentVolumeClaim, kind string) bool {
	dataSourceRef := pvc.Spec.DataSourceRef
	return dataSourceRef != nil && dataSourceRef.APIGroup != nil && *dataSourceRef.APIGroup == cc.AnnAPIGroup && dataSourceRef.Kind == kind
}

func isPVCPrimeDataSourceRefKind(pvc *corev1.PersistentVolumeClaim, kind string) bool {
	owner := metav1.GetControllerOf(pvc)
	if owner == nil || owner.Kind != "PersistentVolumeClaim" {
		return false
	}
	populatorKind, _ := pvc.Annotations[cc.AnnPopulatorKind]
	return populatorKind == kind
}

// PVCPrimeName returns the name of the PVC' of a given pvc
func PVCPrimeName(targetPVC *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-%s", primePvcPrefix, targetPVC.UID)
}

func getPopulatorIndexKey(namespace, kind, name string) string {
	return namespace + "/" + kind + "/" + name
}

func isPVCOwnedByDataVolume(pvc *corev1.PersistentVolumeClaim) bool {
	owner := metav1.GetControllerOf(pvc)
	return (owner != nil && owner.Kind == "DataVolume") || cc.HasAnnOwnedByDataVolume(pvc)
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

func isPVBoundToPVC(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) bool {
	claimRef := pv.Spec.ClaimRef
	return claimRef.Name == pvc.Name && claimRef.Namespace == pvc.Namespace && claimRef.UID == pvc.UID
}
