package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	namespacePrefix                  = "cloner"
	sourcePodFillerName              = "fill-source"
	sourcePVCName                    = "source-pvc"
	fillData                         = "123456789012345678901234567890123456789012345678901234567890"
	fillDataFSMD5sum                 = "fabc176de7eb1b6ca90b3aa4c7e035f3"
	testBaseDir                      = utils.DefaultPvcMountPath
	testFile                         = "/source.txt"
	fillCommand                      = "echo \"" + fillData + "\" >> " + testBaseDir
	blockFillCommand                 = "dd if=/dev/urandom bs=4096 of=" + testBaseDir + " || echo this is fine"
	assertionPollInterval            = 2 * time.Second
	cloneCompleteTimeout             = 90 * time.Second
	controllerSkipPVCCompleteTimeout = 90 * time.Second
)

var _ = Describe("[rfe_id:1277][crit:high][vendor:cnv-qe@redhat.com][level:component]Cloner Test Suite", func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)

	var sourcePvc *v1.PersistentVolumeClaim
	var targetPvc *v1.PersistentVolumeClaim
	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up source PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
		}
		if targetPvc != nil {
			By("[AfterEach] Clean up target PVC")
			err := f.DeletePVC(targetPvc)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("[test_id:1354]Should clone data within same namespace", func() {
		pvcDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
		pvcDef.Namespace = f.Namespace.Name
		sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile)
		doFileBasedCloneTest(f, pvcDef, f.Namespace)
	})

	It("[test_id:1355]Should clone data across different namespaces", func() {
		pvcDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
		pvcDef.Namespace = f.Namespace.Name
		sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile)
		targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
			framework.NsPrefixLabel: f.NsPrefix,
		})
		Expect(err).NotTo(HaveOccurred())
		f.AddNamespaceToDelete(targetNs)
		doFileBasedCloneTest(f, pvcDef, targetNs)
	})

	It("[test_id:1356]Should not clone anything when CloneOf annotation exists", func() {
		pvcDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
		sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile)
		cloneOfAnnoExistenceTest(f, f.Namespace.Name)
	})
})

var _ = Describe("Block PV Cloner Test", func() {
	var (
		sourcePvc, targetPvc *v1.PersistentVolumeClaim
		sourcePV, targetPV   *v1.PersistentVolume
		storageClass         *storageV1.StorageClass
	)

	f := framework.NewFrameworkOrDie(namespacePrefix)

	BeforeEach(func() {
		err := f.ClearBlockPV()
		Expect(err).NotTo(HaveOccurred())

		pod, err := utils.FindPodByPrefix(f.K8sClient, "cdi", "cdi-block-device", "kubevirt.io=cdi-block-device")
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get pod %q", "cdi"+"/"+"cdi-block-device"))
		nodeName := pod.Spec.NodeName

		By(fmt.Sprintf("Creating storageClass for Block PVs"))
		storageClass, err = f.CreateStorageClassFromDefinition(utils.NewStorageClassForBlockPVDefinition("manual"))
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("Creating Block PV for source PVC"))
		sourcePV, err = f.CreatePVFromDefinition(utils.NewBlockPVDefinition("local-source-volume", "500M", nil, "manual", nodeName))
		Expect(err).ToNot(HaveOccurred())

		By("Verify that source PV's phase is Available")
		err = f.WaitTimeoutForPVReady(sourcePV.Name, 60*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("Creating Block PV for target PVC"))
		targetPV, err = f.CreatePVFromDefinition(utils.NewTargetBlockPVDefinition("local-target-volume", "500M", nil, "manual", nodeName))
		Expect(err).ToNot(HaveOccurred())

		By("Verify that target PV's phase is Available")
		err = f.WaitTimeoutForPVReady(targetPV.Name, 60*time.Second)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up source Block PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
		}
		if targetPvc != nil {
			By("[AfterEach] Clean up target Block PVC")
			err := f.DeletePVC(targetPvc)
			Expect(err).ToNot(HaveOccurred())
		}
		if sourcePV != nil {
			By("[AfterEach] Clean up source Block PV")
			err := utils.DeletePV(f.K8sClient, sourcePV)
			Expect(err).ToNot(HaveOccurred())
		}
		if targetPV != nil {
			By("[AfterEach] Clean up target Block PV")
			err := utils.DeletePV(f.K8sClient, targetPV)
			Expect(err).ToNot(HaveOccurred())
		}
		if storageClass != nil {
			By("[AfterEach] Clean up storage class for block PVs")
			err := utils.DeleteStorageClass(f.K8sClient, storageClass)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("Should clone data across namespaces", func() {
		pvcDef := utils.NewBlockPVCDefinition(sourcePVCName, "500M", nil, nil, "manual")
		sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, "fill-source-block-pod", blockFillCommand)
		sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, testBaseDir, 0)
		Expect(err).ToNot(HaveOccurred())

		targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
			framework.NsPrefixLabel: f.NsPrefix,
		})
		Expect(err).NotTo(HaveOccurred())
		f.AddNamespaceToDelete(targetNs)

		targetDV := utils.NewDataVolumeCloneToBlockPV("target-dv", "500M", sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
		Expect(err).ToNot(HaveOccurred())

		targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
		utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
		completeClone(f, targetNs, targetPvc, testBaseDir, sourceMD5)
	})
})

func doFileBasedCloneTest(f *framework.Framework, srcPVCDef *v1.PersistentVolumeClaim, targetNs *v1.Namespace) {
	// Create targetPvc in new NS.
	targetDV := utils.NewCloningDataVolume("target-dv", "1G", srcPVCDef)
	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
	Expect(err).ToNot(HaveOccurred())

	targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
	Expect(err).ToNot(HaveOccurred())

	fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)
	completeClone(f, targetNs, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum)
}

func completeClone(f *framework.Framework, targetNs *v1.Namespace, targetPvc *k8sv1.PersistentVolumeClaim, filePath, expectedMD5 string) {
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

	Expect(f.VerifyTargetPVCContentMD5(targetNs, targetPvc, filePath, expectedMD5)).To(BeTrue())
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
