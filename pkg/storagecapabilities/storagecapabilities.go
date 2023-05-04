// Provides the capabilities (or features) for some well known storage provisioners.

package storagecapabilities

import (
	"context"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StorageCapabilities is a simple holder of storage capabilities (accessMode etc.)
type StorageCapabilities struct {
	AccessMode v1.PersistentVolumeAccessMode
	VolumeMode v1.PersistentVolumeMode
}

// CapabilitiesByProvisionerKey defines default capabilities for different storage classes
var CapabilitiesByProvisionerKey = map[string][]StorageCapabilities{
	// hostpath-provisioner
	"kubevirt.io.hostpath-provisioner": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem}},
	"kubevirt.io/hostpath-provisioner": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem}},
}

// ProvisionerNoobaa is the provisioner string for the Noobaa object bucket provisioner which does not work with CDI
const ProvisionerNoobaa = "openshift-storage.noobaa.io/obc"

// UnsupportedProvisioners is a hash of provisioners which are known not to work with CDI
var UnsupportedProvisioners = map[string]struct{}{
	// The following provisioners may be found in Rook/Ceph deployments and are related to object storage
	"openshift-storage.ceph.rook.io/bucket": {},
	ProvisionerNoobaa:                       {},
}

// Get finds and returns a predefined StorageCapabilities for a given StorageClass
func Get(cl client.Client, sc *storagev1.StorageClass) ([]StorageCapabilities, error) {
	if sc.Provisioner == "kubernetes.io/no-provisioner" {
		return capabilitiesForNoProvisioner(cl, sc)
	}
	capabilities := CapabilitiesByProvisionerKey[sc.Provisioner]
	return capabilities, nil
}

func capabilitiesForNoProvisioner(cl client.Client, sc *storagev1.StorageClass) ([]StorageCapabilities, error) {
	pvs := &v1.PersistentVolumeList{}
	if err := cl.List(context.TODO(), pvs); err != nil {
		return []StorageCapabilities{}, err
	}
	capabilities := []StorageCapabilities{}
	for _, pv := range pvs.Items {
		if pv.Spec.StorageClassName == sc.Name {
			for _, accessMode := range pv.Spec.AccessModes {
				capabilities = append(capabilities, StorageCapabilities{
					AccessMode: accessMode,
					VolumeMode: util.ResolveVolumeMode(pv.Spec.VolumeMode),
				})
			}
		}
	}
	capabilities = uniqueCapabilities(capabilities)
	return capabilities, nil
}

func uniqueCapabilities(input []StorageCapabilities) []StorageCapabilities {
	capabilitiesMap := make(map[StorageCapabilities]bool)
	for _, capability := range input {
		capabilitiesMap[capability] = true
	}
	output := []StorageCapabilities{}
	for capability := range capabilitiesMap {
		output = append(output, capability)
	}
	return output
}
