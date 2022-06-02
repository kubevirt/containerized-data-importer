package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"time"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var (
	nodeSelectorTestValue = map[string]string{"kubernetes.io/arch": runtime.GOARCH}
	tolerationsTestValue  = []v1.Toleration{{Key: "test", Value: "123"}}
	affinityTestValue     = &v1.Affinity{}
)

//RunKubectlCommand runs a kubectl Cmd and returns output and err
func RunKubectlCommand(f *framework.Framework, args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := CreateKubectlCommand(f, args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
		// err will not always be nil calling kubectl, this is expected on no results for instance.
		// still return the value and let the called decide what to do.
		return string(stdOutBytes), err
	}
	return string(stdOutBytes), nil
}

// CreateKubectlCommand returns the Cmd to execute kubectl
func CreateKubectlCommand(f *framework.Framework, args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}

//PrintControllerLog ...
func PrintControllerLog(f *framework.Framework) {
	PrintPodLog(f, f.ControllerPod.Name, f.CdiInstallNs)
}

//PrintPodLog ...
func PrintPodLog(f *framework.Framework, podName, namespace string) {
	log, err := RunKubectlCommand(f, "logs", podName, "-n", namespace)
	if err == nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Pod log\n%s\n", log)
	} else {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Unable to get pod log, %s\n", err.Error())
	}
}

// TestNodePlacementValues returns a pre-defined set of node placement values for testing purposes.
// The values chosen are valid, but the pod will likely not be schedulable.
func TestNodePlacementValues(f *framework.Framework) sdkapi.NodePlacement {
	nodes, _ := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	affinityTestValue = &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{Key: "kubernetes.io/hostname", Operator: v1.NodeSelectorOpIn, Values: []string{nodes.Items[0].Name}},
						},
					},
				},
			},
		},
	}

	return sdkapi.NodePlacement{
		NodeSelector: nodeSelectorTestValue,
		Affinity:     affinityTestValue,
		Tolerations:  tolerationsTestValue,
	}
}

// PodSpecHasTestNodePlacementValues compares if the pod spec has the set of node placement values defined for testing purposes
func PodSpecHasTestNodePlacementValues(f *framework.Framework, podSpec v1.PodSpec) bool {
	if !reflect.DeepEqual(podSpec.NodeSelector, nodeSelectorTestValue) {
		fmt.Printf("mismatched nodeSelectors, podSpec:\n%v\nExpected:\n%v\n", podSpec.NodeSelector, nodeSelectorTestValue)
		return false
	}
	if !reflect.DeepEqual(podSpec.Affinity, affinityTestValue) {
		fmt.Printf("mismatched affinity, podSpec:\n%v\nExpected:\n%v\n", *podSpec.Affinity, affinityTestValue)
		return false
	}
	foundMatchingTolerations := false
	for _, toleration := range podSpec.Tolerations {
		if toleration == tolerationsTestValue[0] {
			foundMatchingTolerations = true
		}
	}
	if !foundMatchingTolerations {
		fmt.Printf("no matching tolerations found. podSpec:\n%v\nExpected:\n%v\n", podSpec.Tolerations, tolerationsTestValue)
		return false
	}
	return true
}

// WaitForConditions waits until the data volume conditions match the expected conditions
func WaitForConditions(f *framework.Framework, dataVolumeName string, timeout, pollingInterval time.Duration, expectedConditions ...*cdiv1.DataVolumeCondition) {
	gomega.Eventually(func() bool {
		resultDv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		gomega.Expect(dverr).ToNot(gomega.HaveOccurred())
		return VerifyConditions(resultDv.Status.Conditions, expectedConditions)
	}, timeout, pollingInterval).Should(gomega.BeTrue())
}

// VerifyConditions checks if the conditions match testConditions
func VerifyConditions(actualConditions []cdiv1.DataVolumeCondition, testConditions []*cdiv1.DataVolumeCondition) bool {
	for _, condition := range testConditions {
		if condition != nil {
			actualCondition := findConditionByType(condition.Type, actualConditions)
			if actualCondition != nil {
				if actualCondition.Status != condition.Status {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Status does not match for type: %s, status expected: [%s], status found: [%s]\n", condition.Type, condition.Status, actualCondition.Status)
					return false
				}
				if strings.Compare(actualCondition.Reason, condition.Reason) != 0 {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Reason does not match for type: %s, reason expected [%s], reason found: [%s]\n", condition.Type, condition.Reason, actualCondition.Reason)
					return false
				}
				if !strings.Contains(actualCondition.Message, condition.Message) {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Message does not match for type: %s, message expected: [%s],  message found: [%s]\n", condition.Type, condition.Message, actualCondition.Message)
					return false
				}
			}
		}
	}
	return true
}

func findConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// CreateVddkWarmImportDataVolume fetches snapshot information from vcsim and returns a multi-stage VDDK data volume
func CreateVddkWarmImportDataVolume(f *framework.Framework, dataVolumeName, size, url string) *cdiv1.DataVolume {
	// Find vcenter-simulator pod
	pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "vcenter-deployment", "app=vcenter")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(pod).ToNot(gomega.BeNil())

	// Get test VM UUID
	id, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmid")
	gomega.Expect(err).To(gomega.BeNil())
	vmid, err := uuid.Parse(strings.TrimSpace(id))
	gomega.Expect(err).To(gomega.BeNil())

	// Get snapshot 1 ID
	previousCheckpoint, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmsnapshot1")
	gomega.Expect(err).To(gomega.BeNil())
	previousCheckpoint = strings.TrimSpace(previousCheckpoint)
	gomega.Expect(err).To(gomega.BeNil())

	// Get snapshot 2 ID
	currentCheckpoint, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmsnapshot2")
	gomega.Expect(err).To(gomega.BeNil())
	currentCheckpoint = strings.TrimSpace(currentCheckpoint)
	gomega.Expect(err).To(gomega.BeNil())

	// Get disk name
	disk, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmdisk")
	gomega.Expect(err).To(gomega.BeNil())
	disk = strings.TrimSpace(disk)
	gomega.Expect(err).To(gomega.BeNil())

	// Create VDDK login secret
	stringData := map[string]string{
		common.KeyAccess: "user",
		common.KeySecret: "pass",
	}
	backingFile := disk
	secretRef := "vddksecret"
	thumbprint := "testprint"
	finalCheckpoint := true
	s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, secretRef))

	return utils.NewDataVolumeWithVddkWarmImport(dataVolumeName, size, backingFile, s.Name, thumbprint, url, vmid.String(), currentCheckpoint, previousCheckpoint, finalCheckpoint)
}
