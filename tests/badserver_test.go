package tests

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/common"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("Problematic server responses", func() {
	f := framework.NewFramework("badserver-func-test")
	var dataVolume *cdiv1.DataVolume

	DescribeTable("Importing from cdi bad web server", func(pathname string) {
		cdiBadServer := fmt.Sprintf("http://cdi-bad-webserver.%s:9090", f.CdiInstallNs)
		dataVolume = utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", cdiBadServer+pathname)
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("[rfe_id:4109][test_id:4110][crit:low][vendor:cnv-qe@redhat.com][level:component] Should succeed even if HEAD forbidden", "/forbidden-HEAD/cirros-qcow2.img"),
		Entry("[rfe_id:4191][test_id:4193][crit:low][vendor:cnv-qe@redhat.com][level:component] Should succeed even on a flaky server", "/flaky/cirros-qcow2.img"),
		Entry("[rfe_id:4326][test_id:5076][crit:low][vendor:cnv-qe@redhat.com][level:component] Should succeed even if Accept-Ranges doesn't exist", "/no-accept-ranges/cirros-qcow2.img"),
	)

	It("Should warn against unexpected content-types when importing a kubevirt img", func() {
		cdiBadServer := fmt.Sprintf("http://cdi-bad-webserver.%s:9090", f.CdiInstallNs)
		pathname := "/bad-content-type/cirros-qcow2.img"
		dataVolume = utils.NewDataVolumeWithHTTPImport("badserver-dv", "1Gi", cdiBadServer+pathname)
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		By("creating DataVolume")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Find importer pod after completion")
		importer, err := utils.FindPodByPrefixOnce(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).ToNot(HaveOccurred())
		Expect(importer.DeletionTimestamp).To(BeNil())

		By("Verify HTTP request error in importer log")
		Eventually(func() bool {
			log, _ := f.RunKubectlCommand("logs", importer.Name, "-n", importer.Namespace)
			return strings.Contains(log, "Unexpected content type 'text/html'. Content might not be a KubeVirt image.")
		}, time.Minute, pollingInterval).Should(BeTrue())
	})

	AfterEach(func() {
		By("deleting DataVolume")
		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
		Expect(err).ToNot(HaveOccurred())
	})
})
