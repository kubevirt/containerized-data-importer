package utils

import (
	"time"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	PodCreateTime  = 30 * time.Second
	PodDeleteTime  = 30 * time.Second
	PodWaitForTime = 30 * time.Second
)

// Create a Pod with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func CreateExecutorPodWithPVC(clientSet *kubernetes.Clientset, podName, namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	var pod *k8sv1.Pod
	podDef := newExecutorPodWithPVC(podName, "ls -1 /pvc | wc -l", pvc)
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

func WaitTimeoutForPodReady(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, podRunning(clientSet, podName, namespace))
}

func podRunning(clientSet *kubernetes.Clientset, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case k8sv1.PodRunning:
			return true, nil
		}
		return false, nil
	}
}
