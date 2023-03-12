package framework

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"

	k8sv1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

// CreatePod is a wrapper around utils.CreatePod
func (f *Framework) CreatePod(podDef *k8sv1.Pod) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, f.Namespace.Name, podDef)
}

// DeletePod is a wrapper around utils.DeletePod
func (f *Framework) DeletePod(pod *k8sv1.Pod) error {
	ns := f.Namespace.Name
	if pod.Namespace != "" {
		ns = pod.Namespace
	}
	return utils.DeletePod(f.K8sClient, pod, ns)
}

// WaitTimeoutForPodReady is a wrapper around utils.WaitTimeouotForPodReady
func (f *Framework) WaitTimeoutForPodReady(podName string, timeout time.Duration) error {
	return utils.WaitTimeoutForPodReady(f.K8sClient, podName, f.Namespace.Name, timeout)
}

// WaitTimeoutForPodStatus is a wrapper around utils.WaitTimeouotForPodStatus
func (f *Framework) WaitTimeoutForPodStatus(podName string, status k8sv1.PodPhase, timeout time.Duration) error {
	return utils.WaitTimeoutForPodStatus(f.K8sClient, podName, f.Namespace.Name, status, timeout)
}

// FindPodByPrefix is a wrapper around utils.FindPodByPrefix
func (f *Framework) FindPodByPrefix(prefix string) (*k8sv1.Pod, error) {
	return utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, prefix, common.CDILabelSelector)
}

// FindPodBySuffix is a wrapper around utils.FindPodBySuffix
func (f *Framework) FindPodBySuffix(suffix string) (*k8sv1.Pod, error) {
	return utils.FindPodBySuffix(f.K8sClient, f.Namespace.Name, suffix, common.CDILabelSelector)
}

// PrintControllerLog ...
func (f *Framework) PrintControllerLog() {
	f.PrintPodLog(f.ControllerPod.Name, f.CdiInstallNs)
}

// PrintPodLog ...
func (f *Framework) PrintPodLog(podName, namespace string) {
	log, err := f.RunKubectlCommand("logs", podName, "-n", namespace)
	if err == nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Pod log\n%s\n", log)
	} else {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Unable to get pod log, %s\n", err.Error())
	}
}
