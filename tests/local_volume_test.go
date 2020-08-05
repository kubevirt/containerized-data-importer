package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	storageClassName = "manual"
	pvWaitForTime    = 60 * time.Second
)

var _ = Describe("[rfe_id:1125][crit:high][vendor:cnv-qe@redhat.com][level:component]Local Volume tests", func() {
	var (
		pv   *v1.PersistentVolume
		err  error
		node string
	)

	f := framework.NewFramework("local-volume-func-test")

	BeforeEach(func() {
		nodes, err := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		nodeRef := utils.GetSchedulableNode(nodes)
		Expect(nodeRef).ToNot(BeNil())
		node = *nodeRef

		By("Creating PV with NodeAffinity and Binding label")
		pv, err = f.CreatePVFromDefinition(utils.NewPVDefinition("local-volume", "1G", storageClassName, node, map[string]string{"node": node}))
		Expect(err).ToNot(HaveOccurred())

		By("Verify that PV's phase is Available")
		err = f.WaitTimeoutForPVReady(pv.Name, pvWaitForTime)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if pv != nil {
			err = utils.DeletePV(f.K8sClient, pv)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("[test_id:1367]Import to PVC should succeed with local PV allocated to specific node", func() {
		By("Creating PVC with a selector field matches the PV's label")
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPRateLimitPort)
		_, err = f.CreatePVCFromDefinition(utils.NewPVCDefinitionWithSelector("local-volume-pvc",
			"1G",
			storageClassName,
			map[string]string{"node": node},
			map[string]string{controller.AnnSource: controller.SourceHTTP,
				controller.AnnContentType: string(cdiv1.DataVolumeKubeVirt), controller.AnnEndpoint: httpEp + "/tinyCore.iso"},
			nil))
		Expect(err).ToNot(HaveOccurred())

		By("Verify the pod running on the desired node: " + node)
		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", f.Namespace.Name+"/"+common.ImporterPodName))
		err = utils.IsExpectedNode(f.K8sClient, node, importer.Name, importer.Namespace, utils.PodWaitForTime)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to find pod on node %s, %v", importer.Name, err))
	})
})
