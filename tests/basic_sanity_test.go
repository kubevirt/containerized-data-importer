package tests_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests"
)

const (
	TestSuiteName = "Basic Sanity"
)

var _ = Describe(TestSuiteName, func() {
	Describe("CDI service account should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand("get", "sa", "cdi-sa", "-n", tests.CDIInstallNamespace)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-sa"))
		})
	})

	Describe("CDI Cluster role should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand("get", "clusterrole", "cdi")
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi"))
		})
	})

	Describe("CDI Cluster role binding should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand("get", "clusterrolebinding", "cdi-sa")
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-sa"))
		})
	})

	Describe("CDI deployment should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand("get", "deployment", "cdi-deployment", "-n", tests.CDIInstallNamespace)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-deployment"))
		})
		It("There should be 1 replica", func() {
			result, err := tests.RunKubectlCommand("get", "deployment", "cdi-deployment", "-o", "jsonpath={.spec.replicas}", "-n", tests.CDIInstallNamespace)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("1"))
		})
	})

	Describe("cdi-sa RBAC rules are correct", func() {
		It("rules should match expectation", func() {
			sa := fmt.Sprintf("system:serviceaccount:" + tests.CDIInstallNamespace + ":cdi-sa")

			eventExpectedResult := make(map[string]string)
			eventExpectedResult["get"] = "no"
			eventExpectedResult["list"] = "no"
			eventExpectedResult["watch"] = "no"
			eventExpectedResult["delete"] = "no"
			eventExpectedResult["create"] = "yes"
			eventExpectedResult["update"] = "yes"
			eventExpectedResult["patch"] = "yes"
			eventExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(eventExpectedResult, "events", sa)

			pvcExpectedResult := make(map[string]string)
			pvcExpectedResult["get"] = "yes"
			pvcExpectedResult["list"] = "yes"
			pvcExpectedResult["watch"] = "yes"
			pvcExpectedResult["delete"] = "no"
			pvcExpectedResult["create"] = "yes"
			pvcExpectedResult["update"] = "yes"
			pvcExpectedResult["patch"] = "yes"
			pvcExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(pvcExpectedResult, "persistentvolumeclaims", sa)
			ValidateRBACForResource(pvcExpectedResult, "persistentvolumeclaims/finalizers", sa)

			podExpectedResult := make(map[string]string)
			podExpectedResult["get"] = "yes"
			podExpectedResult["list"] = "yes"
			podExpectedResult["watch"] = "yes"
			podExpectedResult["delete"] = "yes"
			podExpectedResult["create"] = "yes"
			podExpectedResult["update"] = "no"
			podExpectedResult["patch"] = "no"
			podExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(podExpectedResult, "pods", sa)
			ValidateRBACForResource(podExpectedResult, "pods/finalizers", sa)

			secretsExpectedResult := make(map[string]string)
			secretsExpectedResult["get"] = "yes"
			secretsExpectedResult["list"] = "yes"
			secretsExpectedResult["watch"] = "yes"
			secretsExpectedResult["delete"] = "no"
			secretsExpectedResult["create"] = "yes"
			secretsExpectedResult["update"] = "no"
			secretsExpectedResult["patch"] = "no"
			secretsExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(secretsExpectedResult, "secrets", sa)
		})
	})
})

func ValidateRBACForResource(expectedResults map[string]string, resource string, sa string) {
	for verb, expectedRes := range expectedResults {
		By(fmt.Sprintf("verifying cdi-sa "+resource+" rules, for verb %s", verb))
		result, err := tests.RunKubectlCommand("auth", "can-i", "--as", sa, verb, resource)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(ContainSubstring(expectedRes))
	}
}
