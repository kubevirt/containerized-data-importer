package framework

import (
	"time"

	k8sv1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

func (f *Framework) CreatePod(podDef *k8sv1.Pod) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, f.Namespace.Name, podDef)
}

func (f *Framework) DeletePod(pod *k8sv1.Pod) error {
	return utils.DeletePod(f.K8sClient, pod, f.Namespace.Name)
}

func (f *Framework) WaitTimeoutForPodReady(podName string, timeout time.Duration) error {
	return utils.WaitTimeoutForPodReady(f.K8sClient, podName, f.Namespace.Name, timeout)
}

func (f *Framework) WaitTimeoutForPodStatus(podName string, status k8sv1.PodPhase, timeout time.Duration) error {
	return utils.WaitTimeoutForPodStatus(f.K8sClient, podName, f.Namespace.Name, status, timeout)
}

func (f *Framework) FindPodByPrefix(prefix string) (*k8sv1.Pod, error) {
	return utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, prefix, common.CDI_LABEL_SELECTOR)
}
