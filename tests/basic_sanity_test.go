package tests_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

var _ = Describe("[rfe_id:1347][crit:high][vendor:cnv-qe@redhat.com][level:component]Basic Sanity", func() {
	f := framework.NewFramework("sanity", framework.Config{
		SkipNamespaceCreation: true,
		FeatureGates:          []string{featuregates.HonorWaitForFirstConsumer},
	})

	Context("[test_id:1348]CDI service account should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand(f, "get", "sa", "cdi-sa", "-n", f.CdiInstallNs)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-sa"))
		})
	})

	Context("[test_id:1349]CDI Cluster role should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand(f, "get", "clusterrole", "cdi")
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi"))
		})
	})

	Context("[test_id:1350]CDI Cluster role binding should exist", func() {
		It("Should succeed", func() {
			result, err := tests.RunKubectlCommand(f, "get", "clusterrolebinding", "cdi-sa")
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-sa"))
		})
	})

	Context("CDI deployment should exist", func() {
		It("[test_id:1351]Should succeed", func() {
			result, err := tests.RunKubectlCommand(f, "get", "deployment", "cdi-deployment", "-n", f.CdiInstallNs)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("cdi-deployment"))
		})
		It("[test_id:1352]There should be 1 replica", func() {
			result, err := tests.RunKubectlCommand(f, "get", "deployment", "cdi-deployment", "-o", "jsonpath={.spec.replicas}", "-n", f.CdiInstallNs)
			Expect(err).To(BeNil())
			Expect(result).To(ContainSubstring("1"))
		})
	})

	Context("cdi-sa RBAC rules are correct", func() {
		It("[test_id:1353]rules should match expectation", func() {
			sa := fmt.Sprintf("system:serviceaccount:" + f.CdiInstallNs + ":cdi-sa")

			eventExpectedResult := make(map[string]string)
			eventExpectedResult["get"] = "no"
			eventExpectedResult["list"] = "no"
			eventExpectedResult["watch"] = "no"
			eventExpectedResult["delete"] = "no"
			eventExpectedResult["create"] = "yes"
			eventExpectedResult["update"] = "no"
			eventExpectedResult["patch"] = "yes"
			eventExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(f, eventExpectedResult, "events", sa)

			pvcExpectedResult := make(map[string]string)
			pvcExpectedResult["get"] = "yes"
			pvcExpectedResult["list"] = "yes"
			pvcExpectedResult["watch"] = "yes"
			pvcExpectedResult["delete"] = "yes"
			pvcExpectedResult["create"] = "yes"
			pvcExpectedResult["update"] = "yes"
			pvcExpectedResult["patch"] = "no"
			pvcExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(f, pvcExpectedResult, "persistentvolumeclaims", sa)
			ValidateRBACForResource(f, pvcExpectedResult, "persistentvolumeclaims/finalizers", sa)

			podExpectedResult := make(map[string]string)
			podExpectedResult["get"] = "yes"
			podExpectedResult["list"] = "yes"
			podExpectedResult["watch"] = "yes"
			podExpectedResult["delete"] = "yes"
			podExpectedResult["create"] = "yes"
			podExpectedResult["update"] = "no"
			podExpectedResult["patch"] = "no"
			podExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(f, podExpectedResult, "pods", sa)
			ValidateRBACForResource(f, podExpectedResult, "pods/finalizers", sa)

			secretsExpectedResult := make(map[string]string)
			secretsExpectedResult["get"] = "no"
			secretsExpectedResult["list"] = "no"
			secretsExpectedResult["watch"] = "no"
			secretsExpectedResult["delete"] = "no"
			secretsExpectedResult["create"] = "no"
			secretsExpectedResult["update"] = "no"
			secretsExpectedResult["patch"] = "no"
			secretsExpectedResult["deletecollection"] = "no"
			ValidateRBACForResource(f, secretsExpectedResult, "secrets", sa)
		})
	})

	Context("CRDs must be a structural schema", func() {
		table.DescribeTable("crd name", func(crdName string) {
			crd, err := f.ExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				Skip("Doesn't work on openshift 3.11")
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(crd.ObjectMeta.Name).To(Equal(crdName))
			for _, cond := range crd.Status.Conditions {
				if cond.Type == extv1.CustomResourceDefinitionConditionType("NonStructuralSchema") {
					if cond.Status == extv1.ConditionTrue {
						Fail(fmt.Sprintf("CRD %s is not a structural schema", crdName))
					}
				}
			}
		},
			table.Entry("CDIConfigs", "cdiconfigs.cdi.kubevirt.io"),
			table.Entry("CDIs", "cdis.cdi.kubevirt.io"),
			table.Entry("Datavolumes", "datavolumes.cdi.kubevirt.io"),
		)
	})
})

func ValidateRBACForResource(f *framework.Framework, expectedResults map[string]string, resource string, sa string) {
	for verb, expectedRes := range expectedResults {
		By(fmt.Sprintf("verifying cdi-sa "+resource+" rules, for verb %s", verb))

		result, err := tests.RunKubectlCommand(f, "auth", "can-i", "--as", sa, verb, resource)
		if expectedRes != "no" {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(result).To(ContainSubstring(expectedRes))
	}
}
