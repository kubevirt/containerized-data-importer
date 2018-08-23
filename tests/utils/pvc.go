package utils

import (
	"fmt"
	"time"

	k8sv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultPvcMountPath = "/pvc"
	pvcPollInterval     = 2 * time.Second
	PVCCreateTime       = 30 * time.Second
	PVCDeleteTime       = 30 * time.Second
	PVCPhaseTime        = 30 * time.Second
)

// Creates a PVC in the passed in namespace from the passed in PersistentVolumeClaim definition.
// An example of creating a PVC without annotations looks like this:
// CreatePVCFromDefinition(client, namespace, NewPVCDefinition(name, size, nil, nil))
func CreatePVCFromDefinition(clientSet *kubernetes.Clientset, namespace string, def *k8sv1.PersistentVolumeClaim) (*k8sv1.PersistentVolumeClaim, error) {
	var pvc *k8sv1.PersistentVolumeClaim
	err := wait.PollImmediate(pvcPollInterval, PVCCreateTime, func() (bool, error) {
		var err error
		pvc, err = clientSet.CoreV1().PersistentVolumeClaims(namespace).Create(def)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return pvc, nil
}

// Delete the passed in PVC
func DeletePVC(clientSet *kubernetes.Clientset, namespace string, pvc *k8sv1.PersistentVolumeClaim) error {
	return wait.PollImmediate(pvcPollInterval, PVCDeleteTime, func() (bool, error) {
		err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(pvc.GetName(), nil)
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// Find the passed in PVC
func FindPVC(clientSet *kubernetes.Clientset, namespace, pvcName string) (*k8sv1.PersistentVolumeClaim, error) {
	return clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, metav1.GetOptions{})
}

// Wait for annotation on PVC
func WaitForPVCAnnotation(clientSet *kubernetes.Clientset, namespace string, pvc *k8sv1.PersistentVolumeClaim, annotation string) (string, bool, error) {
	var result string
	err := wait.PollImmediate(pvcPollInterval, PVCCreateTime, func() (bool, error) {
		var err error
		var found bool
		pvc, err = FindPVC(clientSet, namespace, pvc.Name)
		result, found = pvc.ObjectMeta.Annotations[annotation]
		if err == nil && found {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return "", false, err
	}
	return result, true, nil
}

// Creates a PVC definition using the passed in name and requested size.
// You can use the following annotation keys to request an import or clone. The values are defined in the controller package
// AnnEndpoint
// AnnSecret
// AnnCloneRequest
// You can also pass in any label you want.
func NewPVCDefinition(pvcName string, size string, annotations, labels map[string]string) *k8sv1.PersistentVolumeClaim {
	return &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcName,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: k8sv1.PersistentVolumeClaimSpec{
			AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
			Resources: k8sv1.ResourceRequirements{
				Requests: k8sv1.ResourceList{
					k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
				},
			},
		},
	}
}

// Wait for the PVC to be in a particular phase (Pending, Bound, or Lost)
func WaitForPersistentVolumeClaimPhase(clientSet *kubernetes.Clientset, namespace string, phase k8sv1.PersistentVolumeClaimPhase, pvcName string) error {
	err := wait.PollImmediate(pvcPollInterval, PVCPhaseTime, func() (bool, error) {
		pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, metav1.GetOptions{})
		if err != nil || pvc.Status.Phase != phase {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("PersistentVolumeClaim %s not in phase %s within %v", pvcName, phase, PVCPhaseTime)
	}
	return nil
}
