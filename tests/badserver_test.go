package tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

var _ = Describe("Problematic server responses", func() {
	f := framework.NewFrameworkOrDie("badserver-func-test")

	It("should succeed even if HEAD forbidden", func() {
		badServerTinyCoreIso := "http://cdi-bad-webserver.%s:9090/forbidden-HEAD/tinyCore.iso"
		tinyCoreIsoURL := fmt.Sprintf(badServerTinyCoreIso, f.CdiInstallNs)

		dataVolume := utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", tinyCoreIsoURL)
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		By("deleting DataVolume")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should succeed even on a flaky server", func() {
		badServerTinyCoreIso := "http://cdi-bad-webserver.%s:9090/flaky/tinyCore.iso"
		tinyCoreIsoURL := fmt.Sprintf(badServerTinyCoreIso, f.CdiInstallNs)

		dataVolume := utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", tinyCoreIsoURL)
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		By("deleting DataVolume")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
		Expect(err).ToNot(HaveOccurred())
	})
})
