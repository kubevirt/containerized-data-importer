package utils

import (
	corev1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	storageClassPollInterval = defaultPollInterval
	storageClassCreateTime   = defaultPollPeriod
	storageClassDeleteTime   = defaultPollPeriod
)

// CreateStorageClassFromDefinition creates a StorageClass from the passed in StorageClass definition.
func CreateStorageClassFromDefinition(clientSet *kubernetes.Clientset, def *storageV1.StorageClass) (*storageV1.StorageClass, error) {
	var storageClass *storageV1.StorageClass
	err := wait.PollImmediate(storageClassPollInterval, storageClassCreateTime, func() (bool, error) {
		var err error
		storageClass, err = clientSet.StorageV1().StorageClasses().Create(def)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return storageClass, nil
}

// NewStorageClassForBlockPVDefinition creates a StorageClass definition for Block PV
func NewStorageClassForBlockPVDefinition(storageClassName string) *storageV1.StorageClass {
	volumeBindingMode := storageV1.VolumeBindingWaitForFirstConsumer
	reclaimPolicy := corev1.PersistentVolumeReclaimPolicy(corev1.PersistentVolumeReclaimDelete)
	return &storageV1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
		},
		VolumeBindingMode: &volumeBindingMode,
		ReclaimPolicy:     &reclaimPolicy,
		Provisioner:       "kubernetes.io/no-provisioner",
	}
}

// DeleteStorageClass deletes the passed in storageClass
func DeleteStorageClass(clientSet *kubernetes.Clientset, storageClass *storageV1.StorageClass) error {
	return wait.PollImmediate(pvPollInterval, storageClassDeleteTime, func() (bool, error) {
		err := clientSet.StorageV1().StorageClasses().Delete(storageClass.Name, nil)
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// GetStorageClass uses the provided client to attempt to get the specified storage class by name
func GetStorageClass(clientSet *kubernetes.Clientset, scName string) (*storageV1.StorageClass, error) {
	sc, err := clientSet.StorageV1().StorageClasses().Get(scName, metav1.GetOptions{})
	return sc, err
}

// StorageClassExists is a simple check to see if the given SC exists or not
func StorageClassExists(clientSet *kubernetes.Clientset, scName string) (bool, error) {
	_, err := GetStorageClass(clientSet, scName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
