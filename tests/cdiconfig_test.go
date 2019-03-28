package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

var _ = Describe("CDI config tests", func() {
	f := framework.NewFrameworkOrDie("cdiconfig-test")

	It("should have the default storage class as its scratchSpaceStorageClass", func() {
		storageClasses, err := f.K8sClient.StorageV1().StorageClasses().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		var expectedSc string
		for _, sc := range storageClasses.Items {
			if defaultClassValue, ok := sc.Annotations[controller.AnnDefaultStorageClass]; ok {
				if defaultClassValue == "true" {
					expectedSc = sc.Name
					break
				}
			}
		}
		By("Expecting default storage class to be: " + expectedSc)
		config, err := f.CdiClient.CdiV1alpha1().CDIConfigs().Get(common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(expectedSc).To(Equal(config.Status.ScratchSpaceStorageClass))
	})

	//TODO: Once get multiple storage classes, write a test to update the scratch space storage class to something besides the default.
})
