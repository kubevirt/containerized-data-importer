package populators

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	primePvcPrefix = "prime"

	// errCreatingPVCPrime provides a const to indicate we failed to create PVC prime for population
	errCreatingPVCPrime = "ErrCreatingPVCPrime"
	// createdPVCPrimeSucceesfully provides a const to indicate we created PVC prime for population
	createdPVCPrimeSucceesfully = "CreatedPVCPrimeSucceesfully"

	// AnnSelectedNode annotation is added to a PVC that has been triggered by scheduler to
	// be dynamically provisioned. Its value is the name of the selected node.
	AnnSelectedNode = "volume.kubernetes.io/selected-node"
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
	_, ok := pvc.Annotations[cc.AnnUploadRequest]
	if ok {
		return kind == cdiv1.UploadSourceRef
	}
	return false
}

// PVCPrimeName returns the name of the PVC' of a given pvc
func PVCPrimeName(targetPVC *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-%s", primePvcPrefix, targetPVC.UID)
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
