// Package storagecapabilities provides the capabilities (or features) for some well known storage provisioners.
package storagecapabilities

import (
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

// StorageCapabilities is a simple holder of storage capabilities (accessMode etc.)
type StorageCapabilities struct {
	AccessMode v1.PersistentVolumeAccessMode
	VolumeMode v1.PersistentVolumeMode
}

var capabilitiesByProvisionerKey = map[string]StorageCapabilities{
	"kubevirt.io/hostpath-provisioner": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	// ceph-rbd
	"kubernetes.io/rbd":                  {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
	"rbd.csi.ceph.com":                   {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
	"rook-ceph.rbd.csi.ceph.com":         {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
	"openshift-storage.rbd.csi.ceph.com": {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeBlock},
	// ceph-fs
	"cephfs.csi.ceph.com":                   {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	"openshift-storage.cephfs.csi.ceph.com": {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	// storageos
	"kubernetes.io/storageos": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	"storageos":               {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	//AWSElasticBlockStore
	"kubernetes.io/aws-ebs": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	"ebs.csi.aws.com":       {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	// AWSFIle is done by a pod
	//Azure disk
	"kubernetes.io/azure-disk": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	"disk.csi.azure.com":       {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	//Azure file
	"kubernetes.io/azure-file": {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	"file.csi.azure.com":       {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	// GCE Persistent Disk
	"kubernetes.io/gce-pd":  {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	"pd.csi.storage.gke.io": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
	// portworx
	"kubernetes.io/portworx-volume/shared": {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	"pxd.openstorage.org/shared":           {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	"kubernetes.io/portworx-volume":        {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	"pxd.openstorage.org":                  {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeFilesystem},
	// Trident
	"csi.trident.netapp.io/ontap-nas": {AccessMode: v1.ReadWriteMany, VolumeMode: v1.PersistentVolumeFilesystem},
	"csi.trident.netapp.io/ontap-san": {AccessMode: v1.ReadWriteOnce, VolumeMode: v1.PersistentVolumeBlock},
}

// Get finds and returns a predefined StorageCapabilities for a given StorageClass
func Get(sc *storagev1.StorageClass) (StorageCapabilities, bool) {
	provisionerKey := storageProvisionerKey(sc)
	capabilities, found := capabilitiesByProvisionerKey[provisionerKey]
	return capabilities, found
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
	"csi.trident.netapp.io": func(sc *storagev1.StorageClass) string {
		//https://netapp-trident.readthedocs.io/en/stable-v20.04/kubernetes/concepts/objects.html#kubernetes-storageclass-objects
		val := sc.Parameters["backendType"]
		if val == "ontap-nas" {
			return "csi.trident.netapp.io/ontap-nas"
		} else if val == "ontap-san" {
			return "csi.trident.netapp.io/ontap-san"
		}
		return "UNKNOWN"
	},
}
