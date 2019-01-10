package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"time"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	node             = "node01"
	storageClassName = "manual"
	pvWaitForTime    = 60 * time.Second
)

var _ = Describe("[rfe_id:1125][crit:high][vendor:cnv-qe@redhat.com][level:component]Local Volume tests", func() {

	f := framework.NewFrameworkOrDie("local-volume-func-test")

	It("[test_id:1367]Import to PVC should succeed with local PV allocated to specific node", func() {
		By("Creating PV with NodeAffinity and Binding label")
		pv, err := f.CreatePVFromDefinition(utils.NewPVDefinition("local-volume", "1G", map[string]string{"node": node}, storageClassName))
		Expect(err).ToNot(HaveOccurred())

		By("Verify that PV's phase is Available")
		err = f.WaitTimeoutForPVReady(pv.Name, pvWaitForTime)
		Expect(err).ToNot(HaveOccurred())

		By("Creating PVC with a selector field matches the PV's label")
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+utils.FileHostNs, utils.HTTPNoAuthPort)
		_, err = f.CreatePVCFromDefinition(utils.NewPVCDefinitionWithSelector("local-volume-pvc",
			"1G",
			map[string]string{"node": node},
			map[string]string{controller.AnnSource: controller.SourceHTTP,
				controller.AnnContentType: string(cdiv1.DataVolumeKubeVirt), controller.AnnEndpoint: httpEp + "/tinyCore.iso"},
			nil, storageClassName))
		Expect(err).ToNot(HaveOccurred())

		By("Verify the pod running on the desired node")
		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", f.Namespace.Name+"/"+common.ImporterPodName))
		utils.IsExpectedNode(f.K8sClient, node, importer.Name, importer.Namespace, utils.PodWaitForTime)

		err = utils.DeletePV(f.K8sClient, pv)
		Expect(err).ToNot(HaveOccurred())
	})
})
