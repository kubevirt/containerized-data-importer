package tests

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("Problematic server responses", func() {
	f := framework.NewFramework("badserver-func-test")
	const dvWaitTimeout = 500 * time.Second
	var dataVolume *cdiv1.DataVolume

	It("[rfe_id:4109][test_id:4110][crit:low][vendor:cnv-qe@redhat.com][level:component] Should succeed even if HEAD forbidden", func() {
		badServerTinyCoreIso := "http://cdi-bad-webserver.%s:9090/forbidden-HEAD/tinyCore.iso"
		tinyCoreIsoURL := fmt.Sprintf(badServerTinyCoreIso, f.CdiInstallNs)

		dataVolume = utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", tinyCoreIsoURL)
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
	})

	It("[rfe_id:4191][test_id:4193][crit:low][vendor:cnv-qe@redhat.com][level:component] Should succeed even on a flaky server", func() {
		badServerTinyCoreIso := "http://cdi-bad-webserver.%s:9090/flaky/tinyCore.iso"
		tinyCoreIsoURL := fmt.Sprintf(badServerTinyCoreIso, f.CdiInstallNs)

		dataVolume := utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", tinyCoreIsoURL)
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, dvWaitTimeout)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("deleting DataVolume")
		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
		Expect(err).ToNot(HaveOccurred())
	})
})
