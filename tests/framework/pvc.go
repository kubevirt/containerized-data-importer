package framework

import (
	"strings"
	"time"

	. "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/tests/utils"
)

func (f *Framework) CreatePVCFromDefinition(def *k8sv1.PersistentVolumeClaim) (*k8sv1.PersistentVolumeClaim, error) {
	return utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
}

func (f *Framework) DeletePVC(pvc *k8sv1.PersistentVolumeClaim) error {
	return utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc)
}

func (f *Framework) WaitForPersistentVolumeClaimPhase(phase k8sv1.PersistentVolumeClaimPhase, pvcName string) error {
	return utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, phase, pvcName)
}

func (f *Framework) CreateExecutorPodWithPVC(podName string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return utils.CreateExecutorPodWithPVC(f.K8sClient, podName, f.Namespace.Name, pvc)
}

func (f *Framework) DeletePod(pod *k8sv1.Pod) error {
	return utils.DeletePod(f.K8sClient, pod, f.Namespace.Name)
}

func (f *Framework) WaitTimeoutForPodReady(podName string, timeout time.Duration) error {
	return utils.WaitTimeoutForPodReady(f.K8sClient, podName, f.Namespace.Name, timeout)
}

// Verify passed in PVC is empty, returns true if the PVC is empty, false if it is not.
func VerifyPVCIsEmpty(f *Framework, pvc *k8sv1.PersistentVolumeClaim) bool {
	executorPod, err := f.CreateExecutorPodWithPVC("verify-pvc-empty", pvc)
	Expect(err).ToNot(HaveOccurred())
	err = f.WaitTimeoutForPodReady(executorPod.Name, utils.PodWaitForTime)
	Expect(err).ToNot(HaveOccurred())
	output := f.ExecShellInPod(executorPod.Name, "ls -1 /pvc | wc -l")
	f.DeletePod(executorPod)
	return strings.Compare("0", output) == 0
}
