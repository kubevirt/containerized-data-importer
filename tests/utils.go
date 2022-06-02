package tests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

//PrintControllerLog ...
func PrintControllerLog(f *framework.Framework) {
	PrintPodLog(f, f.ControllerPod.Name, f.CdiInstallNs)
}

//PrintPodLog ...
func PrintPodLog(f *framework.Framework, podName, namespace string) {
	log, err := f.RunKubectlCommand("logs", podName, "-n", namespace)
	if err == nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Pod log\n%s\n", log)
	} else {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Unable to get pod log, %s\n", err.Error())
	}
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
