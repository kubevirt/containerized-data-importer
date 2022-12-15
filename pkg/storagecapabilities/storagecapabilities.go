// Provides the capabilities (or features) for some well known storage provisioners.

package storagecapabilities

import (
	"context"
	"strings"

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
	// nfs-csi
	"nfs.csi.k8s.io": {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	// ceph-rbd
	"kubernetes.io/rbd":                  createRbdCapabilities(),
	"rbd.csi.ceph.com":                   createRbdCapabilities(),
	"rook-ceph.rbd.csi.ceph.com":         createRbdCapabilities(),
	"openshift-storage.rbd.csi.ceph.com": createRbdCapabilities(),
	// ceph-fs
	"cephfs.csi.ceph.com":                   {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	"openshift-storage.cephfs.csi.ceph.com": {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	// dell-unity-csi
	"csi-unity.dellemc.com": createDellUnityCapabilities(),
	// storageos
	"kubernetes.io/storageos": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem}},
	"storageos":               {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem}},
	//AWSElasticBlockStore
	"kubernetes.io/aws-ebs": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	"ebs.csi.aws.com":       {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	// AWSFIle is done by a pod
	//Azure disk
	"kubernetes.io/azure-disk": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	"disk.csi.azure.com":       {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	//Azure file
	"kubernetes.io/azure-file": {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	"file.csi.azure.com":       {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	// GCE Persistent Disk
	"kubernetes.io/gce-pd":  {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	"pd.csi.storage.gke.io": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	// Portworx in-tree CSI
	"kubernetes.io/portworx-volume/shared": {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	"kubernetes.io/portworx-volume":        {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem}},
	// Portworx CSI
	"pxd.openstorage.org/shared": createOpenStorageSharedVolumeCapabilities(),
	"pxd.openstorage.org":        createOpenStorageVolumeCapabilities(),
	"pxd.portworx.com/shared":    createOpenStorageSharedVolumeCapabilities(),
	"pxd.portworx.com":           createOpenStorageVolumeCapabilities(),
	// Trident
	"csi.trident.netapp.io/ontap-nas": {{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem}},
	"csi.trident.netapp.io/ontap-san": {{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock}},
	// topolvm
	"topolvm.cybozu.com": createTopoLVMCapabilities(),
	// OpenStack Cinder
	"cinder.csi.openstack.org": createCinderVolumeCapabilities(),
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
func Get(cl client.Client, sc *storagev1.StorageClass) ([]StorageCapabilities, bool) {
	provisionerKey := storageProvisionerKey(sc)
	if provisionerKey == "kubernetes.io/no-provisioner" {
		return capabilitiesForNoProvisioner(cl, sc)
	}
	capabilities, found := CapabilitiesByProvisionerKey[provisionerKey]
	return capabilities, found
}

func isLocalStorageOperator(sc *storagev1.StorageClass) bool {
	_, found := sc.Labels["local.storage.openshift.io/owner-name"]
	return found
}

func knownNoProvisioner(sc *storagev1.StorageClass) bool {
	if isLocalStorageOperator(sc) {
		return true
	}
	return false
}

func capabilitiesForNoProvisioner(cl client.Client, sc *storagev1.StorageClass) ([]StorageCapabilities, bool) {
	// There's so many no-provisioner storage classes, let's start slow with the known ones.
	if !knownNoProvisioner(sc) {
		return []StorageCapabilities{}, false
	}
	pvs := &v1.PersistentVolumeList{}
	err := cl.List(context.TODO(), pvs)
	if err != nil {
		return []StorageCapabilities{}, false
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
	return capabilities, len(capabilities) > 0
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

func storageProvisionerKey(sc *storagev1.StorageClass) string {
	keyMapper, found := storageClassToProvisionerKeyMapper[sc.Provisioner]
	if found {
		return keyMapper(sc)
	}
	// by default the Provisioner name is the key
	return sc.Provisioner
}

var storageClassToProvisionerKeyMapper = map[string]func(sc *storagev1.StorageClass) string{
	"pxd.openstorage.org": func(sc *storagev1.StorageClass) string {
		//https://docs.portworx.com/portworx-install-with-kubernetes/storage-operations/create-pvcs/create-shared-pvcs/
		val := sc.Parameters["shared"]
		if val == "true" {
			return "pxd.openstorage.org/shared"
		}
		return "pxd.openstorage.org"
	},
	"kubernetes.io/portworx-volume": func(sc *storagev1.StorageClass) string {
		val := sc.Parameters["shared"]
		if val == "true" {
			return "kubernetes.io/portworx-volume/shared"
		}
		return "kubernetes.io/portworx-volume"
	},
	"pxd.portworx.com": func(sc *storagev1.StorageClass) string {
		//https://docs.portworx.com/portworx-install-with-kubernetes/storage-operations/csi/volumelifecycle/#create-shared-csi-enabled-volumes
		val := sc.Parameters["shared"]
		if val == "true" {
			return "pxd.portworx.com/shared"
		}
		return "pxd.portworx.com"
	},
	"csi.trident.netapp.io": func(sc *storagev1.StorageClass) string {
		//https://netapp-trident.readthedocs.io/en/stable-v20.04/kubernetes/concepts/objects.html#kubernetes-storageclass-objects
		val := sc.Parameters["backendType"]
		if strings.HasPrefix(val, "ontap-nas") {
			return "csi.trident.netapp.io/ontap-nas"
		}
		if strings.HasPrefix(val, "ontap-san") {
			return "csi.trident.netapp.io/ontap-san"
		}
		return "UNKNOWN"
	},
}

func createRbdCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}

func createDellUnityCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
		{AccessMode: v1.ReadOnlyMany, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadOnlyMany, VolumeMode: v1.PersistentVolumeFilesystem},
		{AccessMode: v1.ReadWriteOncePod, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOncePod, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}

func createTopoLVMCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
		{AccessMode: v1.ReadWriteOncePod, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOncePod, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}

func createOpenStorageVolumeCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}

func createOpenStorageSharedVolumeCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}

func createCinderVolumeCapabilities() []StorageCapabilities {
	return []StorageCapabilities{
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
		{AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	}
}
