package tests

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	testSuiteName        = "Cloner Test Suite"
	namespacePrefix      = "cloner"
	sourcePodFillerName  = "fill-source"
	sourcePVCName        = "source-pvc"
	fillData             = "123456789012345678901234567890123456789012345678901234567890"
	testFile             = utils.DefaultPvcMountPath + "/source.txt"
	fillCommand          = "echo \"" + fillData + "\" >> " + testFile
	cloneCompleteTimeout = 10 * time.Second
	testCompleteTimeout  = 300 * time.Second
)

var _ = Describe(testSuiteName, func() {
	f, err := framework.NewFramework(namespacePrefix, framework.Config{})
	if err != nil {
		Fail("Unable to create framework struct")
	}

	var sourcePvc *v1.PersistentVolumeClaim

	BeforeEach(func() {
		sourcePvc = createAndPopulateSourcePVC(f)
	})

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up target PVC")
			err = f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("A valid source and target given", func() {
		It("Should clone data within same name space", func() {
			doCloneTest(f, f.Namespace)
		})

		It("Should clone data across different namespaces", func() {
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			doCloneTest(f, targetNs)
		})
	})

})

func createAndPopulateSourcePVC(f *framework.Framework) *v1.PersistentVolumeClaim {
	// Create the source PVC and populate it with a file, so we can verify the clone.
	By("[BeforeEach] Creating the source PVC")
	sourcePvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil))
	Expect(err).ToNot(HaveOccurred())
	By("[BeforeEach] Populating the source PVC with data")
	pod, err := f.CreatePod(utils.NewPodWithPVC(sourcePodFillerName, fillCommand, sourcePvc))
	Expect(err).ToNot(HaveOccurred())
	err = f.WaitTimeoutForPodStatus(pod.Name, v1.PodSucceeded, utils.PodWaitForTime)
	Expect(err).ToNot(HaveOccurred())
	return sourcePvc
}

func doCloneTest(f *framework.Framework, targetNs *v1.Namespace) {
	// Create targetPvc in new NS.
	targetPvc, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNs.Name, utils.NewPVCDefinition(
		"target-pvc",
		"1G",
		map[string]string{controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName},
		nil))
	Expect(err).ToNot(HaveOccurred())
	err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)

	By("Find cloner pods")
	sourcePod, err := f.FindPodByPrefix(common.CLONER_SOURCE_PODNAME)
	Expect(err).ToNot(HaveOccurred())
	targetPod, err := utils.FindPodByPrefix(f.K8sClient, targetNs.Name, common.CLONER_TARGET_PODNAME, common.CDI_LABEL_SELECTOR)
	Expect(err).ToNot(HaveOccurred())

	By("Source and Target pods have to be on same node")
	fmt.Fprintf(GinkgoWriter, "INFO: Source POD host %s\n", sourcePod.Spec.NodeName)
	fmt.Fprintf(GinkgoWriter, "INFO: Target POD host %s\n", targetPod.Spec.NodeName)
	Expect(sourcePod.Spec.NodeName).To(Equal(targetPod.Spec.NodeName))

	err = f.WaitTimeoutForPodStatus(sourcePod.Name, v1.PodSucceeded, cloneCompleteTimeout)
	Expect(err).ToNot(HaveOccurred())
	err = utils.WaitTimeoutForPodStatus(f.K8sClient, targetPod.Name, targetNs.Name, v1.PodSucceeded, cloneCompleteTimeout)
	Expect(err).ToNot(HaveOccurred())

	By("Verify the clone annotation is on the target PVC")
	_, cloneAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnCloneOf)
	Expect(err).ToNot(HaveOccurred())
	Expect(cloneAnnotationFound).To(BeTrue())

	By("Verify the clone status is success on the target PVC")
	status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodPhase)
	Expect(phaseAnnotation).To(BeTrue())
	Expect(status).Should(BeEquivalentTo(v1.PodSucceeded))

	// Clone is completed, verify the content matches the source.
	Expect(verifyTargetContent(f, targetNs, targetPvc)).To(BeTrue())
	// Clean up PVC, the AfterEach will also clean it up, through the Namespace delete.
	if targetPvc != nil {
		err = utils.DeletePVC(f.K8sClient, targetNs.Name, targetPvc)
		Expect(err).ToNot(HaveOccurred())
	}
}

func verifyTargetContent(f *framework.Framework, namespace *v1.Namespace, pvc *v1.PersistentVolumeClaim) bool {
	By("Verify target PVC content matches source PVC")
	executorPod, err := utils.CreateExecutorPodWithPVC(f.K8sClient, "verify-pvc-content", namespace.Name, pvc)
	Expect(err).ToNot(HaveOccurred())
	err = utils.WaitTimeoutForPodReady(f.K8sClient, executorPod.Name, namespace.Name, utils.PodWaitForTime)
	Expect(err).ToNot(HaveOccurred())
	output := f.ExecShellInPod(executorPod.Name, namespace.Name, "cat "+testFile)
	f.DeletePod(executorPod)
	return strings.Compare(fillData, output) == 0
}
