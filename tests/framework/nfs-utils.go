package framework

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	// DefaultNfsPvSize is the default nfs pv capacity
	DefaultNfsPvSize = "10Gi"

	// BiggerNfsPvSize is the bigger nfs pv capacity
	BiggerNfsPvSize = "20Gi"

	// ExtraNfsDiskPrefix is the prefix for extra nfs disks
	ExtraNfsDiskPrefix = "/extraDisk"

	timeout         = time.Second * 90
	pollingInterval = time.Second
	pvCount         = 10
	defaultPrefix   = "/disk"
)

func createNFSPVs(client *kubernetes.Clientset, cdiNs string) error {
	ip := utils.NfsService.Spec.ClusterIP
	for i := 1; i <= pvCount; i++ {
		if _, err := utils.CreatePVFromDefinition(client, NfsPvDef(i, defaultPrefix, ip, DefaultNfsPvSize)); err != nil {
			// reset rangeCount
			return err
		}
	}
	return nil
}

func deleteNFSPVs(client *kubernetes.Clientset, cdiNs string) error {
	for i := 1; i <= pvCount; i++ {
		pv := NfsPvDef(i, defaultPrefix, utils.NfsService.Spec.ClusterIP, DefaultNfsPvSize)
		if err := utils.DeletePV(client, pv); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}
	for i := 1; i <= pvCount; i++ {
		pv := NfsPvDef(i, defaultPrefix, utils.NfsService.Spec.ClusterIP, DefaultNfsPvSize)
		if err := utils.WaitTimeoutForPVDeleted(client, pv, timeout); err != nil {
			return err
		}
	}
	return nil
}

// NfsPvDef creates pv defs for nfs
func NfsPvDef(index int, prefix, serviceIP, size string) *corev1.PersistentVolume {
	is := strconv.Itoa(index)
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("nfs-pv%s%s", strings.Replace(strings.ToLower(prefix), "/", "-", -1), is),
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "nfs",
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
				corev1.ReadWriteMany,
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(size),
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				NFS: &corev1.NFSVolumeSource{
					Server: serviceIP,
					Path:   prefix + is,
				},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
		},
	}
}
