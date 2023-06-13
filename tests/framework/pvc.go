package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

// CreatePVCFromDefinition is a wrapper around utils.CreatePVCFromDefinition
func (f *Framework) CreatePVCFromDefinition(def *k8sv1.PersistentVolumeClaim) (*k8sv1.PersistentVolumeClaim, error) {
	return utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
}

// CreateBoundPVCFromDefinition is a wrapper around utils.CreatePVCFromDefinition that also force binds pvc on
// on WaitForFirstConsumer storage class by executing f.ForceBindIfWaitForFirstConsumer(pvc)
func (f *Framework) CreateBoundPVCFromDefinition(def *k8sv1.PersistentVolumeClaim) *k8sv1.PersistentVolumeClaim {
	pvc, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	pvc, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	f.ForceBindIfWaitForFirstConsumer(pvc)
	return pvc
}

// CreateScheduledPVCFromDefinition is a wrapper around utils.CreatePVCFromDefinition that also triggeres
// the scheduler to dynamically provision a pvc with WaitForFirstConsumer storage class by
// executing f.ForceBindIfWaitForFirstConsumer(pvc)
func (f *Framework) CreateScheduledPVCFromDefinition(def *k8sv1.PersistentVolumeClaim) *k8sv1.PersistentVolumeClaim {
	pvc, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, def)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	pvc, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	f.ForceSchedulingIfWaitForFirstConsumerPopulationPVC(pvc)
	return pvc
}

// DeletePVC is a wrapper around utils.DeletePVC
func (f *Framework) DeletePVC(pvc *k8sv1.PersistentVolumeClaim) error {
	return utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc.Name)
}

// WaitForPersistentVolumeClaimPhase is a wrapper around utils.WaitForPersistentVolumeClaimPhase
func (f *Framework) WaitForPersistentVolumeClaimPhase(phase k8sv1.PersistentVolumeClaimPhase, pvcName string) error {
	return utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, phase, pvcName)
}

// FindPVC is a wrapper around utils.FindPVC
func (f *Framework) FindPVC(pvcName string) (*k8sv1.PersistentVolumeClaim, error) {
	return utils.FindPVC(f.K8sClient, f.Namespace.Name, pvcName)
}

// ForceBindPvcIfDvIsWaitForFirstConsumer creates a Pod with the PVC for passed in DV mounted under /dev/pvc, which forces the PVC to be scheduled and bound.
func (f *Framework) ForceBindPvcIfDvIsWaitForFirstConsumer(dv *cdiv1.DataVolume) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "verifying pvc was created for dv %s\n", dv.Name)
	// FIXME: #1210, brybacki, tomob this code assumes dvname = pvcname needs to be fixed,
	pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "PVC should exist")
	if f.IsBindingModeWaitForFirstConsumer(pvc.Spec.StorageClassName) {
		// check if pvc is a population pvc but not from pvc or snapshot
		if pvc.Spec.DataSourceRef != nil &&
			(dv.Spec.Source == nil || dv.Spec.Source.PVC == nil) &&
			(dv.Spec.Source == nil || dv.Spec.Source.Snapshot == nil) {
			err = utils.WaitForDataVolumePhase(f, dv.Namespace, cdiv1.PendingPopulation, dv.Name)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			createConsumerPodForPopulationPVC(pvc, f)
		} else {
			err = utils.WaitForDataVolumePhase(f, dv.Namespace, cdiv1.WaitForFirstConsumer, dv.Name)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			createConsumerPod(pvc, f)
		}
	}
}

// WaitPVCDeletedByUID is a wrapper around utils.WaitPVCDeletedByUID
func (f *Framework) WaitPVCDeletedByUID(pvcSpec *k8sv1.PersistentVolumeClaim, timeout time.Duration) (bool, error) {
	return utils.WaitPVCDeletedByUID(f.K8sClient, pvcSpec, timeout)
}

// ForceBindIfWaitForFirstConsumer creates a Pod with the passed in PVC mounted under /dev/pvc, which forces the PVC to be scheduled and bound.
func (f *Framework) ForceBindIfWaitForFirstConsumer(targetPvc *k8sv1.PersistentVolumeClaim) {
	if targetPvc.Spec.VolumeName == "" && f.IsBindingModeWaitForFirstConsumer(targetPvc.Spec.StorageClassName) {
		if targetPvc.Spec.DataSourceRef != nil {
			createConsumerPodForPopulationPVC(targetPvc, f)
		} else {
			createConsumerPod(targetPvc, f)
		}
	}
}

// ForceSchedulingIfWaitForFirstConsumerPopulationPVC creates a Pod with the passed in PVC mounted under /dev/pvc, which forces the PVC to be scheduled for provisioning.
func (f *Framework) ForceSchedulingIfWaitForFirstConsumerPopulationPVC(targetPvc *k8sv1.PersistentVolumeClaim) {
	if f.IsBindingModeWaitForFirstConsumer(targetPvc.Spec.StorageClassName) {
		createConsumerPodForPopulationPVC(targetPvc, f)
	}
}

// CreateConsumerPod create a pod that consumes the given PVC
func (f *Framework) CreateConsumerPod(targetPvc *k8sv1.PersistentVolumeClaim) *k8sv1.Pod {
	namespace := targetPvc.Namespace

	err := utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetPvc.Namespace, k8sv1.ClaimPending, targetPvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	podName := naming.GetResourceName("consumer-pod", targetPvc.Name)
	executorPod, err := f.CreateNoopPodWithPVC(podName, namespace, targetPvc)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return executorPod
}

func createConsumerPod(targetPvc *k8sv1.PersistentVolumeClaim, f *Framework) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: creating \"consumer-pod\" to force binding PVC: %s\n", targetPvc.Name)
	executorPod := f.CreateConsumerPod(targetPvc)

	namespace := targetPvc.Namespace
	err := utils.WaitTimeoutForPodSucceeded(f.K8sClient, executorPod.Name, namespace, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, namespace, k8sv1.ClaimBound, targetPvc.Name)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	gomega.Expect(utils.DeletePodNoGrace(f.K8sClient, executorPod, namespace)).Should(gomega.Succeed())
}

func createConsumerPodForPopulationPVC(targetPvc *k8sv1.PersistentVolumeClaim, f *Framework) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: creating \"consumer-pod\" to get 'selected-node' annotation on PVC: %s\n", targetPvc.Name)
	executorPod := f.CreateConsumerPod(targetPvc)

	namespace := targetPvc.Namespace
	selectedNode, status, err := utils.WaitForPVCAnnotation(f.K8sClient, namespace, targetPvc, controller.AnnSelectedNode)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(status).To(gomega.BeTrue())
	gomega.Expect(selectedNode).ToNot(gomega.BeEmpty())

	err = utils.DeletePodNoGrace(f.K8sClient, executorPod, namespace)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

// VerifyPVCIsEmpty verifies a passed in PVC is empty, returns true if the PVC is empty, false if it is not. Optionaly, specify node for the pod.
func VerifyPVCIsEmpty(f *Framework, pvc *k8sv1.PersistentVolumeClaim, node string) (bool, error) {
	var err error
	var executorPod *k8sv1.Pod
	if node != "" {
		executorPod, err = f.CreateExecutorPodWithPVCSpecificNode(utils.VerifierPodName, f.Namespace.Name, pvc, node, true)
	} else {
		executorPod, err = f.CreateExecutorPodWithPVC(utils.VerifierPodName, f.Namespace.Name, pvc, true)
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = f.WaitTimeoutForPodReady(executorPod.Name, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	output, stderr, err := f.ExecShellInPod(executorPod.Name, f.Namespace.Name, "ls -1 /dev/pvc | wc -l")
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}
	found := strings.Compare("0", output) == 0
	if !found {
		// Could be that a file system was created and it has 'lost+found' directory in it, check again.
		output, stderr, err := f.ExecShellInPod(executorPod.Name, f.Namespace.Name, "ls -1 /dev/pvc")
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

	f.PopulatePVC(sourcePvc, podName, fillCommand)
	return sourcePvc
}

// PopulatePVC populates a PVC using a pod with the provided pod name and command
func (f *Framework) PopulatePVC(pvc *k8sv1.PersistentVolumeClaim, podName string, fillCommand string) {
	pod, err := f.CreatePod(f.NewPodWithPVC(podName, fillCommand+"&& sync", pvc, false))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = f.WaitTimeoutForPodStatus(pod.Name, k8sv1.PodSucceeded, utils.PodWaitForTime)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = f.DeletePod(pod)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
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
	var err error
	executorPod, err := f.startVerifierPod(namespace, pvc)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: could not start verifier pod: [%s]\n", err)
		return "", err
	}

	cmd := "md5sum " + fileName
	if numBytes > 0 {
		cmd = fmt.Sprintf("head -c %d %s 1> /dev/null && head -c %d %s | md5sum", numBytes, fileName, numBytes, fileName)
	}

	var output, stderr string
	err = wait.PollImmediate(2*time.Second, 10*time.Second, func() (bool, error) {
		output, stderr, err = f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: pod command execution failed, retrying: stderr: [%s]\n", stderr)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", err
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: md5sum found %s\n", string(output[:32]))
	// Don't delete pod, other verification might happen.
	return output[:32], nil
}

func (f *Framework) verifyInPod(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, cmd string, verifyFn func(output, stderr string) (bool, error)) (bool, error) {
	executorPod, err := f.startVerifierPod(namespace, pvc)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: could not start verifier pod: [%s]\n", err)
		return false, err
	}

	output, stderr, err := f.ExecShellInPod(executorPod.Name, namespace.Name, cmd)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: stderr: [%s]\n", stderr)
		return false, err
	}

	return verifyFn(output, stderr)
}

// VerifyBlankDisk checks a blank disk on a file mode PVC by validating that the disk.img file is sparse.
func (f *Framework) VerifyBlankDisk(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	cmd := fmt.Sprintf("tr -d '\\000' <%s/disk.img | grep -q -m 1 ^ || echo \"All zeros\"", utils.DefaultPvcMountPath)

	return f.verifyInPod(namespace, pvc, cmd, func(output, stderr string) (bool, error) {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: empty file check %s\n", output)
		return strings.Compare("All zeros", string(output)) == 0, nil
	})
}

// VerifySparse checks a disk image being sparse after creation/resize.
func (f *Framework) VerifySparse(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, imagePath string) (bool, error) {
	var info image.ImgInfo
	var imageContentSize int64
	err := f.GetImageInfo(namespace, pvc, imagePath, &info)
	if err != nil {
		return false, err
	}
	// qemu-img info gives us ActualSize but that is size on disk
	// which isn't important to us in this comparison; we compare content size
	err = f.GetImageContentSize(namespace, pvc, imagePath, &imageContentSize)
	if err != nil {
		return false, err
	}
	if info.ActualSize-imageContentSize >= units.MiB {
		return false, fmt.Errorf("Diff between content size %d and size on disk %d is significant, something's not right", imageContentSize, info.ActualSize)
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: VerifySparse comparison: Virtual: %d vs Content: %d\n", info.VirtualSize, imageContentSize)
	return info.VirtualSize >= imageContentSize, nil
}

// VerifyFSOverhead checks whether virtual size is smaller than actual size. That means FS Overhead has been accounted for.
// NOTE: this assertion is only valid when preallocation is used.
func (f *Framework) VerifyFSOverhead(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, preallocation bool) (bool, error) {
	if !preallocation {
		return false, fmt.Errorf("VerifyFSOverhead is only valid when preallocation is used")
	}

	var info image.ImgInfo
	err := f.GetImageInfo(namespace, pvc, utils.DefaultImagePath, &info)
	if err != nil {
		return false, err
	}

	requestedSize := pvc.Spec.Resources.Requests[k8sv1.ResourceStorage]
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: VerifyFSOverhead comparison: Virtual: %d, Actual: %d, requestedSize: %d\n", info.VirtualSize, info.ActualSize, requestedSize.Value())
	return info.VirtualSize <= info.ActualSize && info.VirtualSize < requestedSize.Value(), nil
}

// VerifyImagePreallocated checks that image's virtual size is roughly equal to actual size
func (f *Framework) VerifyImagePreallocated(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	var info image.ImgInfo
	err := f.GetImageInfo(namespace, pvc, utils.DefaultImagePath, &info)
	if err != nil {
		return false, err
	}

	return info.ActualSize >= info.VirtualSize, nil
}

// VerifyPermissions returns the group of a disk image.
func (f *Framework) VerifyPermissions(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	cmd := fmt.Sprintf("x=$(ls -ln %s/disk.img); y=($x); echo ${y[0]}", utils.DefaultPvcMountPath)

	return f.verifyInPod(namespace, pvc, cmd, func(output, stderr string) (bool, error) {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: permissions of disk.img: %s\n", output)

		return strings.Compare(output, "-rw-rw----.") == 0, nil
	})
}

// GetDiskGroup returns the group of a disk image.
func (f *Framework) GetDiskGroup(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, deletePod bool) (string, error) {
	executorPod, err := f.startVerifierPod(namespace, pvc)
	if err != nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: could not start verifier pod: [%s]\n", err)
		return "", err
	}

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
	cmd := "ls -I lost+found " + utils.DefaultPvcMountPath + " | wc -l"

	return f.verifyInPod(namespace, pvc, cmd, func(output, stderr string) (bool, error) {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: file count found %s\n", string(output))
		return strings.Compare(count, output) == 0, nil
	})
}

// RunCommandAndCaptureOutput runs a command on a pod that has the passed in PVC mounted and captures the output.
func (f *Framework) RunCommandAndCaptureOutput(pvc *k8sv1.PersistentVolumeClaim, cmd string, readOnly bool) (string, error) {
	executorPod, err := f.CreateExecutorPodWithPVC("execute-command", f.Namespace.Name, pvc, readOnly)
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
func (f *Framework) NewPodWithPVC(podName, cmd string, pvc *k8sv1.PersistentVolumeClaim, readOnly bool) *k8sv1.Pod {
	importerImage := f.GetEnvVarValue("IMPORTER_IMAGE")
	volumeName := naming.GetLabelNameFromResourceName(pvc.GetName())
	pod := &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Annotations: map[string]string{
				"cdi.kubevirt.io/testing": podName,
			},
		},
		Spec: k8sv1.PodSpec{
			// this may be causing an issue
			TerminationGracePeriodSeconds: &[]int64{10}[0],
			RestartPolicy:                 k8sv1.RestartPolicyOnFailure,
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
		},
	}

	volumeMode := pvc.Spec.VolumeMode
	if volumeMode != nil && *volumeMode == k8sv1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices(pvc, volumeName)
	} else {
		pod.Spec.Containers[0].VolumeMounts = addVolumeMounts(pvc, volumeName, readOnly)
	}

	controller.SetRestrictedSecurityContext(&pod.Spec)

	return pod
}

func (f *Framework) newExecutorPodWithPVC(podName string, pvc *k8sv1.PersistentVolumeClaim, readOnly bool) *k8sv1.Pod {
	return f.NewPodWithPVC(podName, "while true; do echo hello; sleep 2;done", pvc, readOnly)
}

// GetEnvVarValue gets an environmemnt variable value from the cdi-deployment container
func (f *Framework) GetEnvVarValue(name string) (importerImage string) {
	for _, e := range f.ControllerPod.Spec.Containers[0].Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

// CreateExecutorPodWithPVC creates a Pod with the passed in PVC mounted under /dev/pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateExecutorPodWithPVC(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim, readOnly bool) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, namespace, f.newExecutorPodWithPVC(podName, pvc, readOnly))
}

// CreateExecutorPodWithPVCSpecificNode creates a Pod on a specific node with the passed in PVC mounted under /dev/pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateExecutorPodWithPVCSpecificNode(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim, node string, readOnly bool) (*k8sv1.Pod, error) {
	var pod = f.newExecutorPodWithPVC(podName, pvc, readOnly)
	pod.Spec.NodeSelector = map[string]string{
		"kubernetes.io/hostname": node,
	}
	return utils.CreatePod(f.K8sClient, namespace, pod)
}

// CreateNoopPodWithPVC creates a short living pod, that might be used to force bind a pvc
func (f *Framework) CreateNoopPodWithPVC(podName, namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return utils.CreatePod(f.K8sClient, namespace, f.NewPodWithPVC(podName, "echo I am vm doppleganger pod;", pvc, true))
}

// CreateVerifierPodWithPVC creates a Pod called verifier, with the passed in PVC mounted under /dev/pvc. You can then use the executor utilities to
// run commands against the PVC through this Pod.
func (f *Framework) CreateVerifierPodWithPVC(namespace string, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	return f.CreateExecutorPodWithPVC(utils.VerifierPodName, namespace, pvc, true)
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
func addVolumeMounts(pvc *k8sv1.PersistentVolumeClaim, volumeName string, readOnly bool) []k8sv1.VolumeMount {
	volumeMounts := []k8sv1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: utils.DefaultPvcMountPath,
			ReadOnly:  readOnly,
		},
	}
	return volumeMounts
}

// GetImageInfo returns qemu-img information about given image
func (f *Framework) GetImageInfo(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, imagePath string, info *image.ImgInfo) error {
	cmd := fmt.Sprintf("qemu-img info %s --output=json", imagePath)

	_, err := f.verifyInPod(namespace, pvc, cmd, func(output, stderr string) (bool, error) {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: qemu-img info output %s\n", output)

		err := json.Unmarshal([]byte(output), info)
		if err != nil {
			klog.Errorf("Invalid JSON:\n%s\n", string(output))
			return false, err
		}

		return true, nil
	})

	return err
}

// GetImageContentSize returns the content size (as opposed to size on disk) of an image
func (f *Framework) GetImageContentSize(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim, imagePath string, imageSize *int64) error {
	cmd := fmt.Sprintf("du -s --apparent-size -B 1 %s | cut -f 1", imagePath)

	_, err := f.verifyInPod(namespace, pvc, cmd, func(output, stderr string) (bool, error) {
		fmt.Fprintf(ginkgo.GinkgoWriter, "CMD (%s) output %s\n", cmd, output)

		size, err := strconv.ParseInt(output, 10, 64)
		if err != nil {
			klog.Errorf("Invalid image content size:\n%s\n", string(output))
			return false, err
		}
		*imageSize = size

		return true, nil
	})

	return err
}

func (f *Framework) startVerifierPod(namespace *k8sv1.Namespace, pvc *k8sv1.PersistentVolumeClaim) (*k8sv1.Pod, error) {
	var executorPod *k8sv1.Pod
	var err error

	executorPod, err = f.CreateVerifierPodWithPVC(namespace.Name, pvc)
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return executorPod, err
	}
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)

	return executorPod, err
}
