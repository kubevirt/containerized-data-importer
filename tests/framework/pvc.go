package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

// CreatePVCFromDefinition is a wrapper around utils.CreatePVCFromDefinition
func (f *Framework) CreatePVCFromDefinition(def *k8sv1.PersistentVolumeClaim) (*k8sv1.PersistentVolumeClaim, error) {
	return utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
}

// CreateBoundPVCFromDefinition is a wrapper around utils.CreatePVCFromDefinition that also force binds pvc on
// on WaitForFirstConsumer storage class by executing f.ForceBindIfWaitForFirstConsumer(pvc)
func (f *Framework) CreateBoundPVCFromDefinition(def *k8sv1.PersistentVolumeClaim) (*k8sv1.PersistentVolumeClaim, error) {
	pvc, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	pvc, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	f.ForceBindIfWaitForFirstConsumer(pvc)
	return pvc, nil
}

// DeletePVC is a wrapper around utils.DeletePVC
func (f *Framework) DeletePVC(pvc *k8sv1.PersistentVolumeClaim) error {
	return utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc)
}

// WaitForPersistentVolumeClaimPhase is a wrapper around utils.WaitForPersistentVolumeClaimPhase
func (f *Framework) WaitForPersistentVolumeClaimPhase(phase k8sv1.PersistentVolumeClaimPhase, pvcName string) error {
	return utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, phase, pvcName)
}

// FindPVC is a wrapper around utils.FindPVC
func (f *Framework) FindPVC(pvcName string) (*k8sv1.PersistentVolumeClaim, error) {
	return utils.FindPVC(f.K8sClient, f.Namespace.Name, pvcName)
}

// ForceBindPvcIfDvIsWaitForFirstConsumer creates a Pod with the PVC for passed in DV mounted under /pvc, which forces the PVC to be scheduled and bound.
func (f *Framework) ForceBindPvcIfDvIsWaitForFirstConsumer(dv *cdiv1.DataVolume) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "verifying pvc was created for dv %s\n", dv.Name)
	// FIXME: #1210, brybacki, tomob this code assumes dvname = pvcname needs to be fixed,
	pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	if f.IsBindingModeWaitForFirstConsumer(pvc.Spec.StorageClassName) {
		err = utils.WaitForDataVolumePhase(f.CdiClient, dv.Namespace, cdiv1.WaitForFirstConsumer, dv.Name)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		createConsumerPod(pvc, f)
	}
}

// WaitPVCDeletedByUID is a wrapper around utils.WaitPVCDeletedByUID
func (f *Framework) WaitPVCDeletedByUID(pvcSpec *k8sv1.PersistentVolumeClaim, timeout time.Duration) (bool, error) {
	return utils.WaitPVCDeletedByUID(f.K8sClient, pvcSpec, timeout)
}

// ForceBindIfWaitForFirstConsumer creates a Pod with the passed in PVC mounted under /pvc, which forces the PVC to be scheduled and bound.
func (f *Framework) ForceBindIfWaitForFirstConsumer(targetPvc *k8sv1.PersistentVolumeClaim) {
	if f.IsBindingModeWaitForFirstConsumer(targetPvc.Spec.StorageClassName) {
		createConsumerPod(targetPvc, f)
	}
}

func createConsumerPod(targetPvc *k8sv1.PersistentVolumeClaim, f *Framework) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: creating \"consumer-pod\" to force binding PVC: %s\n", targetPvc.Name)
	namespace := targetPvc.Namespace

	err := utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetPvc.Namespace, k8sv1.ClaimPending, targetPvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	podName := naming.GetResourceName("consumer-pod", targetPvc.Name)
	executorPod, err := f.CreateNoopPodWithPVC(podName, namespace, targetPvc)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = utils.WaitTimeoutForPodSucceeded(f.K8sClient, executorPod.Name, namespace, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, namespace, k8sv1.ClaimBound, targetPvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	utils.DeletePodNoGrace(f.K8sClient, executorPod, namespace)
}

// VerifyPVCIsEmpty verifies a passed in PVC is empty, returns true if the PVC is empty, false if it is not. Optionaly, specify node for the pod.
func VerifyPVCIsEmpty(f *Framework, pvc *k8sv1.PersistentVolumeClaim, node string) (bool, error) {
	var err error
	var executorPod *k8sv1.Pod
	if node != "" {
		executorPod, err = f.CreateExecutorPodWithPVCSpecificNode(utils.VerifierPodName, f.Namespace.Name, pvc, node)
	} else {
		executorPod, err = f.CreateExecutorPodWithPVC(utils.VerifierPodName, f.Namespace.Name, pvc)
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = f.WaitTimeoutForPodReady(executorPod.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	output, stderr, err := f.ExecShellInPod(executorPod.Name, f.Namespace.Name, "ls -1 /pvc | wc -l")
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	found := strings.Compare("0", output) == 0
	if !found {
		// Could be that a file system was created and it has 'lost+found' directory in it, check again.
		output, stderr, err := f.ExecShellInPod(executorPod.Name, f.Namespace.Name, "ls -1 /pvc")
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
			return false, err
		}
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: files found: %s\n", output)
		found = strings.Compare("lost+found", output) == 0
	}
	return found, nil
}

// CreateAndPopulateSourcePVC Creates and populates a PVC using the provided POD and command
func (f *Framework) CreateAndPopulateSourcePVC(pvcDef *k8sv1.PersistentVolumeClaim, podName string, fillCommand string) *k8sv1.PersistentVolumeClaim {
	// Create the source PVC and populate it with a file, so we can verify the clone.
	sourcePvc, err := f.CreatePVCFromDefinition(pvcDef)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	pod, err := f.CreatePod(f.NewPodWithPVC(podName, fillCommand, sourcePvc))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = f.WaitTimeoutForPodStatus(pod.Name, k8sv1.PodSucceeded, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return sourcePvc
}

// VerifyTargetPVCContentMD5 provides a function to check the md5 of data on a PVC and ensure it matches that which is provided
func (f *Framework) VerifyTargetPVCContentMD5(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, fileName string, expectedHash string, numBytes ...int64) (bool, error) {
	if len(numBytes) == 0 {
		numBytes = append(numBytes, 0)
	}

	md5, err := f.GetMD5(namespace, pvc, fileName, numBytes[0])
	if err != nil {
		return false, err
	}

	return expectedHash == md5, nil
}

// GetMD5 returns the MD5 of a file on a PVC
func (f *Framework) GetMD5(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, fileName string, numBytes int64) (string, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cmd := "md5sum " + fileName
	if numBytes > 0 {
		cmd = fmt.Sprintf("head -c %d %s | md5sum", numBytes, fileName)
	}

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return "", err
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: md5sum found %s\n", string(output[:32]))
	// Don't delete pod, other verification might happen.
	return output[:32], nil
}

// VerifyBlankDisk checks a blank disk on a file mode PVC by validating that the disk.img file is sparse.
func (f *Framework) VerifyBlankDisk(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cmd := fmt.Sprintf("tr -d '\\000' <%s/disk.img | grep -q -m 1 ^ || echo \"All zeros\"", utils.DefaultPvcMountPath)

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)

	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: empty file check %s\n", output)
	return strings.Compare("All zeros", string(output)) == 0, nil
}

// VerifySparse checks a disk image being sparse after creation/resize.
func (f *Framework) VerifySparse(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cmd := fmt.Sprintf("qemu-img info %s/disk.img --output=json", utils.DefaultPvcMountPath)

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)

	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: qemu-img info output %s\n", output)
	var info image.ImgInfo
	err = json.Unmarshal([]byte(output), &info)
	if err != nil {
		klog.Errorf("Invalid JSON:\n%s\n", string(output))
	}
	return info.VirtualSize >= info.ActualSize, nil
}

// VerifyPermissions returns the group of a disk image.
func (f *Framework) VerifyPermissions(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cmd := fmt.Sprintf("x=$(ls -ln %s/disk.img); y=($x); echo ${y[0]}", utils.DefaultPvcMountPath)

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: permissions of disk.img: %s\n", output)

	return strings.Compare(output, "-rw-rw----.") == 0, nil
}

// GetDiskGroup returns the group of a disk image.
func (f *Framework) GetDiskGroup(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, deletePod bool) (string, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	cmd := fmt.Sprintf("ls -ln %s/disk.img", utils.DefaultPvcMountPath)

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: ln -ln disk.img: %s\n", output)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
	}
	cmd = fmt.Sprintf("x=$(ls -ln %s/disk.img); y=($x); echo ${y[3]}", utils.DefaultPvcMountPath)

	output, stderr, err = f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)

	if deletePod {
		err := f.K8sClient.CoreV1().Pods(namespace.Name).Delete(context.TODO(), executorPod.Name, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			if _, err := f.K8sClient.CoreV1().Pods(namespace.Name).Get(context.TODO(), executorPod.Name, metav1.GetOptions{}); err != nil {
				if apierrs.IsNotFound(err) {
					return true
				}
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			return false
		}, 90*time.Second, 2*time.Second).Should(gomega.BeTrue())
	}

	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return "", err
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: gid of disk.img: %s\n", output)
	return output, nil
}

// VerifyTargetPVCArchiveContent provides a function to check if the number of files extracted from an archive matches the passed in value
func (f *Framework) VerifyTargetPVCArchiveContent(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, count string) (bool, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if !apierrs.IsAlreadyExists(err) {
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, "ls "+utils.DefaultPvcMountPath+" | wc -l")
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: file count found %s\n", string(output))
	return strings.Compare(count, output) == 0, nil
}

// RunCommandAndCaptureOutput runs a command on a pod that has the passed in PVC mounted and captures the output.
func (f *Framework) RunCommandAndCaptureOutput(pvc *k8sv1.PersistentVolumeClaim, cmd string) (string, error) {
	executorPod, err := f.CreateExecutorPodWithPVC("execute-command", f.Namespace.Name, pvc)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = f.WaitTimeoutForPodReady(executorPod.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	output, stderr, err := f.ExecShellInPod(executorPod.Name, f.Namespace.Name, cmd)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return "", err
	}
	err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), executorPod.Name, metav1.DeleteOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return output, nil
}

// NewPodWithPVC creates a new pod that mounts the given PVC
func (f *Framework) NewPodWithPVC(podName, cmd string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	var importerImage string
	volumeName := naming.GetLabelNameFromResourceName(pvc.GetName())
	fsGroup := common.QemuSubGid
	for _, e := range f.ControllerPod.Spec.Containers[0].Env {
		if e.Name == "IMPORTER_IMAGE" {
			importerImage = e.Value
		}
	}
	pod := &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Annotations: map[string]string{
				"cdi.kubevirt.io/testing": podName,
			},
		},
		Spec: k8sv1.PodSpec{
			RestartPolicy: k8sv1.RestartPolicyNever,
			Containers: []k8sv1.Container{
				{
					Name:    "runner",
					Image:   importerImage,
					Command: []string{"/bin/sh", "-c", cmd},
					Resources: k8sv1.ResourceRequirements{
						Limits: map[k8sv1.ResourceName]resource.Quantity{
							k8sv1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							k8sv1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
						Requests: map[k8sv1.ResourceName]resource.Quantity{
							k8sv1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
							k8sv1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
					},
				},
			},
			Volumes: []k8sv1.Volume{
				{
					Name: volumeName,
					VolumeSource: k8sv1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.GetName(),
						},
					},
				},
			},
			SecurityContext: &k8sv1.PodSecurityContext{
				FSGroup: &fsGroup,
			},
		},
	}

	volumeMode := pvc.Spec.VolumeMode
	if volumeMode != nil && *volumeMode == k8sv1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices(pvc, volumeName)
	} else {
		pod.Spec.Containers[0].VolumeMounts = addVolumeMounts(pvc, volumeName)
	}
	return pod
}

func (f *Framework) newExecutorPodWithPVC(podName string, pvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	return f.NewPodWithPVC(podName, "while true; do echo hello; sleep 2;done", pvc)
}

// CreateExecutorPodWithPVC creates a Pod with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateExecutorPodWithPVC(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, namespace, f.newExecutorPodWithPVC(podName, pvc))
}

// CreateExecutorPodWithPVCSpecificNode creates a Pod on a specific node with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateExecutorPodWithPVCSpecificNode(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim, node string) (*k8sv1.Pod, error) {
	var pod = f.newExecutorPodWithPVC(podName, pvc)
	pod.Spec.NodeSelector = map[string]string{
		"kubernetes.io/hostname": node,
	}
	return utils.CreatePod(f.K8sClient, namespace, pod)
}

// CreateNoopPodWithPVC creates a short living pod, that might be used to force bind a pvc
func (f *Framework) CreateNoopPodWithPVC(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, namespace, f.NewPodWithPVC(podName, "echo I am vm doppleganger pod;", pvc))
}

// CreateVerifierPodWithPVC creates a Pod called verifier, with the passed in PVC mounted under /pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateVerifierPodWithPVC(namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return f.CreateExecutorPodWithPVC(utils.VerifierPodName, namespace, pvc)
}

func addVolumeDevices(pvc *k8sv1.PersistentVolumeClaim, volumeName string) []k8sv1.VolumeDevice {
	volumeDevices := []k8sv1.VolumeDevice{
		{
			Name:       volumeName,
			DevicePath: utils.DefaultPvcMountPath,
		},
	}
	return volumeDevices
}

// this is being called for pods using PV with filesystem volume mode
func addVolumeMounts(pvc *k8sv1.PersistentVolumeClaim, volumeName string) []k8sv1.VolumeMount {
	volumeMounts := []k8sv1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: utils.DefaultPvcMountPath,
		},
	}
	return volumeMounts
}
