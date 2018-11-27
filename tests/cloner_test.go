package tests

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
	"strings"
	"time"
)

const (
	testSuiteName                    = "Cloner Test Suite"
	namespacePrefix                  = "cloner"
	sourcePodFillerName              = "fill-source"
	sourcePVCName                    = "source-pvc"
	fillData                         = "123456789012345678901234567890123456789012345678901234567890"
	testFile                         = utils.DefaultPvcMountPath + "/source.txt"
	fillCommand                      = "echo \"" + fillData + "\" >> " + testFile
	assertionPollInterval            = 2 * time.Second
	cloneCompleteTimeout             = 90 * time.Second
	controllerSkipPVCCompleteTimeout = 90 * time.Second
)

var _ = Describe(testSuiteName, func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)

	var sourcePvc *v1.PersistentVolumeClaim

	BeforeEach(func() {
		sourcePvc = f.CreateAndPopulateSourcePVC(sourcePVCName, sourcePodFillerName, fillCommand)
	})

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up target PVC")
			err := f.DeletePVC(sourcePvc)
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
		It("Should not clone anything when CloneOf annotation exists", func() {
			cloneOfAnnoExistenceTest(f, f.Namespace.Name)
		})
	})

})

func doCloneTest(f *framework.Framework, targetNs *v1.Namespace) {
	// Create targetPvc in new NS.
	targetPvc, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNs.Name, utils.NewPVCDefinition(
		"target-pvc",
		"1G",
		map[string]string{controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName},
		nil))
	Expect(err).ToNot(HaveOccurred())
	fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)

	By("Find cloner pods")
	sourcePod, err := f.FindPodByPrefix(common.ClonerSourcePodName)
	if err != nil {
		PrintControllerLog(f)
	}
	Expect(err).ToNot(HaveOccurred())
	targetPod, err := utils.FindPodByPrefix(f.K8sClient, targetNs.Name, common.ClonerTargetPodName, common.CDILabelSelector)
	if err != nil {
		PrintControllerLog(f)
	}
	Expect(err).ToNot(HaveOccurred())

	By("Verifying that the source and target pods are scheduled on the same node")
	Eventually(func() bool {
		srcNode, err := utils.PodGetNode(f.K8sClient, sourcePod.Name, sourcePod.Namespace)
		Expect(err).ToNot(HaveOccurred())
		tgtNode, err := utils.PodGetNode(f.K8sClient, targetPod.Name, targetPod.Namespace)
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: Source POD host %s\n", srcNode)
		fmt.Fprintf(GinkgoWriter, "INFO: Target POD host %s\n", tgtNode)
		return srcNode == tgtNode
	}, cloneCompleteTimeout, assertionPollInterval).Should(BeTrue())

	By("Verify the clone annotation is on the target PVC")
	_, cloneAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnCloneOf)
	if err != nil {
		PrintControllerLog(f)
		PrintPodLog(f, targetPod.Name, targetPod.Namespace)
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(cloneAnnotationFound).To(BeTrue())

	By("Verify the clone status is success on the target PVC")
	Eventually(func() string {
		status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodPhase)
		Expect(err).ToNot(HaveOccurred())
		Expect(phaseAnnotation).To(BeTrue())
		return status
	}, cloneCompleteTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

	// Clone is completed, verify the content matches the source.
	Expect(f.VerifyTargetPVCContent(targetNs, targetPvc, testFile, fillData)).To(BeTrue())
	// Clean up PVC, the AfterEach will also clean it up, through the Namespace delete.
	if targetPvc != nil {
		err = utils.DeletePVC(f.K8sClient, targetNs.Name, targetPvc)
		Expect(err).ToNot(HaveOccurred())
	}

	By("Verify source pod deleted")
	deleted, err := utils.WaitPodDeleted(f.K8sClient, sourcePod.Name, targetPod.Namespace, defaultTimeout)
	Expect(err).ToNot(HaveOccurred())
	Expect(deleted).To(BeTrue())

	By("Verify target pod deleted")
	deleted, err = utils.WaitPodDeleted(f.K8sClient, targetPod.Name, targetNs.Name, defaultTimeout)
	Expect(err).ToNot(HaveOccurred())
	Expect(deleted).To(BeTrue())
}

func cloneOfAnnoExistenceTest(f *framework.Framework, targetNamespaceName string) {
	// Create targetPvc
	By(fmt.Sprintf("Creating target pvc: %s/target-pvc", targetNamespaceName))
	_, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNamespaceName, utils.NewPVCDefinition(
		"target-pvc",
		"1G",
		map[string]string{controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName, controller.AnnCloneOf: "true"},
		nil))
	Expect(err).ToNot(HaveOccurred())
	By("Checking no cloning pods were created")

	Eventually(func() bool {
		log, err := RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return strings.Contains(log, "pvc annotation \""+controller.AnnCloneOf+"\" exists indicating cloning completed, skipping pvc \""+f.Namespace.Name+"/target-pvc\"")
	}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	Expect(err).ToNot(HaveOccurred())
}
