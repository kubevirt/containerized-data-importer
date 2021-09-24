package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	k8sv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// PodWaitForTime is the time to wait for Pod operations to complete
	PodWaitForTime = defaultPollPeriod
	// PodWaitForTimeFast is the fast time to wait for Pod operations to complete (30 sec)
	PodWaitForTimeFast = defaultPollPeriodFast
	// PodWaitIntervalFast is the fast polling interval (250ms)
	PodWaitIntervalFast = 250 * time.Millisecond

	podCreateTime = defaultPollPeriod
	podDeleteTime = defaultPollPeriod

	//VerifierPodName is the name of the verifier pod.
	VerifierPodName = "verifier"
)

// DeleteVerifierPod deletes the verifier pod
func DeleteVerifierPod(clientSet *kubernetes.Clientset, namespace string) error {
	return DeletePodByName(clientSet, VerifierPodName, namespace, nil)
}

// CreatePod calls the Kubernetes API to create a Pod
func CreatePod(clientSet *kubernetes.Clientset, namespace string, podDef *k8sv1.Pod) (*k8sv1.Pod, error) {
	err := wait.PollImmediate(2*time.Second, podCreateTime, func() (bool, error) {
		var err error
		_, err = clientSet.CoreV1().Pods(namespace).Create(context.TODO(), podDef, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	})
	return podDef, err
}

// DeletePodByName deletes the pod based on the passed in name from the passed in Namespace
func DeletePodByName(clientSet *kubernetes.Clientset, podName, namespace string, gracePeriod *int64) error {
	_ = clientSet.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{
		GracePeriodSeconds: gracePeriod,
	})
	return wait.PollImmediate(2*time.Second, podDeleteTime, func() (bool, error) {
		_, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// DeletePodNoGrace deletes the passed in Pod from the passed in Namespace
func DeletePodNoGrace(clientSet *kubernetes.Clientset, pod *k8sv1.Pod, namespace string) error {
	zero := int64(0)
	return DeletePodByName(clientSet, pod.Name, namespace, &zero)
}

// DeletePod deletes the passed in Pod from the passed in Namespace
func DeletePod(clientSet *kubernetes.Clientset, pod *k8sv1.Pod, namespace string) error {
	return DeletePodByName(clientSet, pod.Name, namespace, nil)
}

// FindPodBySuffix finds the first pod which has the passed in suffix. Returns error if multiple pods with the same suffix are found.
func FindPodBySuffix(clientSet *kubernetes.Clientset, namespace, suffix, labelSelector string) (*k8sv1.Pod, error) {
	return findPodByCompFunc(clientSet, namespace, suffix, labelSelector, strings.HasSuffix)
}

// FindPodByPrefix finds the first pod which has the passed in prefix. Returns error if multiple pods with the same prefix are found.
func FindPodByPrefix(clientSet *kubernetes.Clientset, namespace, prefix, labelSelector string) (*k8sv1.Pod, error) {
	return findPodByCompFunc(clientSet, namespace, prefix, labelSelector, strings.HasPrefix)
}

// FindPodBySuffixOnce finds once (no polling) the first pod which has the passed in suffix. Returns error if multiple pods with the same suffix are found.
func FindPodBySuffixOnce(clientSet *kubernetes.Clientset, namespace, suffix, labelSelector string) (*k8sv1.Pod, error) {
	return findPodByCompFuncOnce(clientSet, namespace, suffix, labelSelector, strings.HasSuffix)
}

// FindPodByPrefixOnce finds once (no polling) the first pod which has the passed in prefix. Returns error if multiple pods with the same prefix are found.
func FindPodByPrefixOnce(clientSet *kubernetes.Clientset, namespace, prefix, labelSelector string) (*k8sv1.Pod, error) {
	return findPodByCompFuncOnce(clientSet, namespace, prefix, labelSelector, strings.HasPrefix)
}

func findPodByCompFunc(clientSet *kubernetes.Clientset, namespace, prefix, labelSelector string, compFunc func(string, string) bool) (*k8sv1.Pod, error) {
	var result *k8sv1.Pod
	var err error
	wait.PollImmediate(2*time.Second, podCreateTime, func() (bool, error) {
		result, err = findPodByCompFuncOnce(clientSet, namespace, prefix, labelSelector, compFunc)
		if result != nil {
			return true, err
		}
		// If no result yet, continue polling even if there is an error
		return false, nil
	})
	return result, err
}

func findPodByCompFuncOnce(clientSet *kubernetes.Clientset, namespace, prefix, labelSelector string, compFunc func(string, string) bool) (*k8sv1.Pod, error) {
	var result *k8sv1.Pod
	podList, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if compFunc(pod.Name, prefix) {
			if result == nil {
				result = pod.DeepCopy()
			} else {
				fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: First pod name %s in namespace %s\n", result.Name, result.Namespace)
				fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Second pod name %s in namespace %s\n", pod.Name, pod.Namespace)
				return result, fmt.Errorf("Multiple pods starting with prefix %q in namespace %q", prefix, namespace)
			}
		}
	}
	if result == nil {
		return nil, fmt.Errorf("Unable to find pod containing %s", prefix)
	}
	return result, nil
}

// WaitTimeoutForPodReady waits for the given pod to be created and ready
func WaitTimeoutForPodReady(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) error {
	return WaitTimeoutForPodCondition(clientSet, podName, namespace, k8sv1.PodReady, 2*time.Second, timeout)
}

// WaitTimeoutForPodReadyPollPeriod waits for the given pod to be created and ready using the passed in poll period
func WaitTimeoutForPodReadyPollPeriod(clientSet *kubernetes.Clientset, podName, namespace string, pollperiod, timeout time.Duration) error {
	return WaitTimeoutForPodCondition(clientSet, podName, namespace, k8sv1.PodReady, pollperiod, timeout)
}

// WaitTimeoutForPodSucceeded waits for pod to succeed
func WaitTimeoutForPodSucceeded(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) error {
	return WaitTimeoutForPodStatus(clientSet, podName, namespace, k8sv1.PodSucceeded, timeout)
}

// WaitTimeoutForPodFailed waits for pod to fail
func WaitTimeoutForPodFailed(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) error {
	return WaitTimeoutForPodStatus(clientSet, podName, namespace, k8sv1.PodFailed, timeout)
}

// WaitTimeoutForPodStatus waits for the given pod to be created and have a expected status
func WaitTimeoutForPodStatus(clientSet *kubernetes.Clientset, podName, namespace string, status k8sv1.PodPhase, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, podStatus(clientSet, podName, namespace, status))
}

// WaitTimeoutForPodCondition waits for the given pod to be created and have an expected condition
func WaitTimeoutForPodCondition(clientSet *kubernetes.Clientset, podName, namespace string, conditionType k8sv1.PodConditionType, pollperiod, timeout time.Duration) error {
	return wait.PollImmediate(pollperiod, timeout, podCondition(clientSet, podName, namespace, conditionType))
}

func podStatus(clientSet *kubernetes.Clientset, podName, namespace string, status k8sv1.PodPhase) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Checking POD %s phase: %s\n", podName, string(pod.Status.Phase))
		switch pod.Status.Phase {
		case status:
			return true, nil
		}
		return false, nil
	}
}

func podCondition(clientSet *kubernetes.Clientset, podName, namespace string, conditionType k8sv1.PodConditionType) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == conditionType {
				fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Checking POD %s condition: %s=%s\n", podName, string(cond.Type), string(cond.Status))
				if cond.Status == k8sv1.ConditionTrue {
					return true, nil
				}
			}
		}
		return false, nil
	}
}

// PodGetNode returns the node on which a given pod is executing
func PodGetNode(clientSet *kubernetes.Clientset, podName, namespace string) (string, error) {
	pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Spec.NodeName, nil
}

// WaitPodDeleted waits fo a pod to no longer exist
// returns whether the pod is deleted along with any error
func WaitPodDeleted(clientSet *kubernetes.Clientset, podName, namespace string, timeout time.Duration) (bool, error) {
	var result bool
	err := wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
		_, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				result = true
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	return result, err
}

// IsExpectedNode waits to check if the specified pod is schedule on the specified node
func IsExpectedNode(clientSet *kubernetes.Clientset, nodeName, podName, namespace string, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, isExpectedNode(clientSet, nodeName, podName, namespace))
}

// returns true is the specified pod running on the specified nodeName. Otherwise returns false
func isExpectedNode(clientSet *kubernetes.Clientset, nodeName, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Checking Node name: %s\n", string(pod.Spec.NodeName))
		if pod.Spec.NodeName == nodeName {
			return true, nil
		}
		return false, nil
	}
}

// GetSchedulableNode return a schedulable node from a nodes list
func GetSchedulableNode(nodes *v1.NodeList) *string {
	for _, node := range nodes.Items {
		if node.Spec.Taints == nil {
			return &node.Name
		}
		schedulableNode := true
		for _, taint := range node.Spec.Taints {
			if taint.Effect == "NoSchedule" {
				schedulableNode = false
				break
			}
		}
		if schedulableNode {
			return &node.Name
		}
	}
	return nil
}
