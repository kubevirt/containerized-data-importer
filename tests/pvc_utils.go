package tests

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultPvcMountPath = "/pvc"
	poll                = 2 * time.Second
)

// Creates a PVC in the passed in namespace from the passed in PersistentVolumeClaim definition.
// An example of creating a PVC without annotations looks like this:
// CreatePVCFromDefinition(client, namespace, NewPVCDefinition(name, size, nil))
func (f *Framework) CreatePVCFromDefinition(namespace string, def *k8sv1.PersistentVolumeClaim) *k8sv1.PersistentVolumeClaim {
	var pvc *k8sv1.PersistentVolumeClaim
	err := wait.PollImmediate(poll, defaultTimeout, func() (bool, error) {
		var err error
		pvc, err = f.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(def)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		Fail("Unable to create PVC: " + def.GetName() + ", error: " + err.Error())
	}
	return pvc
}

// Delete the passed in PVC
func (f *Framework) DeletePVC(pvc *k8sv1.PersistentVolumeClaim) {
	err := wait.PollImmediate(poll, defaultTimeout, func() (bool, error) {
		err := f.KubeClient.CoreV1().PersistentVolumeClaims(f.namespace.GetName()).Delete(pvc.GetName(), nil)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		Fail("Unable to delete PVC: " + pvc.GetName() + ", error: " + err.Error())
	}
}

// Creates a PVC definition using the passed in name and requested size.
// You can use the following annotation keys to request an import or clone. The values are defined in the controller package
// AnnEndpoint
// AnnSecret
// AnnCloneRequest
func NewPVCDefinition(pvcName string, size string, annotations map[string]string) *k8sv1.PersistentVolumeClaim {
	return &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcName,
			Annotations: annotations,
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

func (f *Framework) WaitForPersistentVolumeClaimPhase(phase k8sv1.PersistentVolumeClaimPhase, pvcName string) error {
	for start := time.Now(); time.Since(start) < defaultTimeout; time.Sleep(poll) {
		pvc, err := f.KubeClient.CoreV1().PersistentVolumeClaims(f.namespace.GetName()).Get(pvcName, metav1.GetOptions{})
		if err != nil {
			continue
		} else {
			if pvc.Status.Phase == phase {
				return nil
			}
		}
	}
	return fmt.Errorf("PersistentVolumeClaim %s not in phase %s within %v", pvcName, phase, defaultTimeout)
}

// Create a Pod with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateExecutorPodWithPVC(podName string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	var pod *k8sv1.Pod
	podDef := newExecutorPodWithPVC(podName, "ls -1 /pvc | wc -l", pvc)
	err := wait.PollImmediate(2*time.Second, defaultTimeout, func() (bool, error) {
		var err error
		pod, err = f.KubeClient.CoreV1().Pods(f.namespace.GetName()).Create(podDef)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		Fail("Unable to create Pod: " + podName + ", error: " + err.Error())
	}
	return pod
}

// Delete the passed in Pod from the passed in Namespace
func (f *Framework) DeletePodFromNamespace(pod *k8sv1.Pod, namespace *k8sv1.Namespace) {
	err := wait.PollImmediate(2*time.Second, defaultTimeout, func() (bool, error) {
		err := f.KubeClient.CoreV1().Pods(namespace.GetName()).Delete(pod.GetName(), &metav1.DeleteOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		Fail("Unable to delete Pod: " + pod.GetName() + ", error: " + err.Error())
	}
}

// Delete the passed in Pod from the Framework primary namespace
func (f *Framework) DeletePod(pod *k8sv1.Pod) {
	f.DeletePodFromNamespace(pod, f.namespace)
}

func newExecutorPodWithPVC(podName, cmd string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	return &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: k8sv1.PodSpec{
			RestartPolicy: k8sv1.RestartPolicyNever,
			Containers: []k8sv1.Container{
				{
					Name:    "runner",
					Image:   "fedora:28",
					Command: []string{"/bin/sh", "-c", "sleep 5; echo I am an executor pod;"},
					VolumeMounts: []k8sv1.VolumeMount{
						{
							Name:      pvc.GetName(),
							MountPath: defaultPvcMountPath,
						},
					},
				},
			},
			Volumes: []k8sv1.Volume{
				{
					Name: pvc.GetName(),
					VolumeSource: k8sv1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.GetName(),
						},
					},
				},
			},
		},
	}
}

func (f *Framework) waitTimeoutForPodReady(podName string, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, f.podRunning(podName, f.namespace.GetName()))
}

func (f *Framework) podRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := f.KubeClient.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case k8sv1.PodRunning:
			return true, nil
		case k8sv1.PodFailed, k8sv1.PodSucceeded:
			return false, nil
		}
		return false, nil
	}
}

func (f *Framework) VerifyPVCIsEmpty(pvc *k8sv1.PersistentVolumeClaim) bool {
	executorPod := f.CreateExecutorPodWithPVC("verify-pvc-empty", pvc)
	err := f.waitTimeoutForPodReady(executorPod.GetName(), defaultTimeout)
	Expect(err).To(BeNil())
	output := f.ExecShellInPod(executorPod.GetName(), "ls -1 /pvc | wc -l")
	f.DeletePod(executorPod)
	return strings.Compare("0", output) == 0
}
