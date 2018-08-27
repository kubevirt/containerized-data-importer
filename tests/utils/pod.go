package utils

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	PodCreateTime  = defaultPollPeriod
	PodDeleteTime  = defaultPollPeriod
	PodWaitForTime = defaultPollPeriod
)

// Create a Pod with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func CreateExecutorPodWithPVC(clientSet *kubernetes.Clientset, podName, namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return CreatePod(clientSet, namespace, newExecutorPodWithPVC(podName, pvc))
}

func CreatePod(clientSet *kubernetes.Clientset, namespace string, podDef *k8sv1.Pod) (*k8sv1.Pod, error) {
	var pod *k8sv1.Pod
	err := wait.PollImmediate(2*time.Second, PodCreateTime, func() (bool, error) {
		var err error
		pod, err = clientSet.CoreV1().Pods(namespace).Create(podDef)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	return pod, err
}

// Delete the passed in Pod from the passed in Namespace
func DeletePod(clientSet *kubernetes.Clientset, pod *k8sv1.Pod, namespace string) error {
	return wait.PollImmediate(2*time.Second, PodDeleteTime, func() (bool, error) {
		err := clientSet.CoreV1().Pods(namespace).Delete(pod.GetName(), &metav1.DeleteOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

func NewPodWithPVC(podName, cmd string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	return &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: k8sv1.PodSpec{
			RestartPolicy: k8sv1.RestartPolicyNever,
			Containers: []k8sv1.Container{
				{
					Name:    "runner",
					Image:   "fedora:28",
					Command: []string{"/bin/sh", "-c", cmd},
					VolumeMounts: []k8sv1.VolumeMount{
						{
							Name:      pvc.GetName(),
							MountPath: DefaultPvcMountPath,
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

// Finds the first pod which has the passed in prefix. Returns error if multiple pods with the same prefix are found.
func FindPodByPrefix(clientSet *kubernetes.Clientset, namespace, prefix, labelSelector string) (*k8sv1.Pod, error) {
	var result k8sv1.Pod
	var multiplePods bool
	err := wait.PollImmediate(2*time.Second, PodCreateTime, func() (bool, error) {
		multiplePods = false
		podList, err := clientSet.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err == nil {
			var foundPod bool
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, prefix) {
					if !foundPod {
						foundPod = true
						result = pod
					} else {
						fmt.Fprintf(GinkgoWriter, "INFO: First pod name %s\n", result.Name)
						fmt.Fprintf(GinkgoWriter, "INFO: Second pod name %s\n", pod.Name)
						multiplePods = true
					}
				}
			}
			if foundPod {
				return true, nil
			}
			return false, fmt.Errorf("Unable to find pod starting with prefix %s", prefix)
		} else {
			return false, err
		}
	})
	if multiplePods {
		return nil, fmt.Errorf("Multiple pods starting with prefix %s", prefix)
	}
	return &result, err
}

func newExecutorPodWithPVC(podName string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	return NewPodWithPVC(podName, "sleep 5; echo I am an executor pod;", pvc)
}

func WaitTimeoutForPodReady(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) error {
	return WaitTimeoutForPodStatus(clientSet, podName, namespace, k8sv1.PodRunning, timeout)
}

func WaitTimeoutForPodStatus(clientSet *kubernetes.Clientset, podName, namespace string, status k8sv1.PodPhase, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, podStatus(clientSet, podName, namespace, status))
}

func podStatus(clientSet *kubernetes.Clientset, podName, namespace string, status k8sv1.PodPhase) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case status:
			return true, nil
		}
		return false, nil
	}
}

func PodGetNode(clientSet *kubernetes.Clientset, podName, namespace string) (string, error) {
	pod, err := clientSet.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Spec.NodeName, nil
}
