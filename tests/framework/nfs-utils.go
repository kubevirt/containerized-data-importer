package framework

import (
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	timeout         = time.Second * 90
	pollingInterval = time.Second
	pvCount         = 10
)

func createNFSPVs(client *kubernetes.Clientset, cdiNs string) error {
	for i := 1; i <= pvCount; i++ {
		if _, err := utils.CreatePVFromDefinition(client, nfsPVDef(strconv.Itoa(i), utils.NfsService.Spec.ClusterIP)); err != nil {
			// reset rangeCount
			return err
		}
	}
	return nil
}

func deleteNFSPVs(client *kubernetes.Clientset, cdiNs string) error {
	for i := 1; i <= pvCount; i++ {
		pv := nfsPVDef(strconv.Itoa(i), utils.NfsService.Spec.ClusterIP)
		if err := utils.DeletePV(client, pv); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}
	for i := 1; i <= pvCount; i++ {
		pv := nfsPVDef(strconv.Itoa(i), utils.NfsService.Spec.ClusterIP)
		if err := utils.WaitTimeoutForPVDeleted(client, pv, timeout); err != nil {
			return err
		}
	}
	return nil
}

func nfsPVDef(index, serviceIP string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfs-pv" + index,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "nfs",
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
				corev1.ReadWriteMany,
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("30Gi"),
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				NFS: &corev1.NFSVolumeSource{
					Server: serviceIP,
					Path:   "/disk" + index,
				},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
		},
	}
}
