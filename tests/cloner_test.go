package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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
	verifyPodDeletedTimeout          = 90 * time.Second
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
		smartApplicable := f.IsSnapshotStorageClassAvailable()
		sc, err := f.K8sClient.StorageV1().StorageClasses().Get(f.SnapshotSCName, metav1.GetOptions{})
		if err == nil {
			value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
			if smartApplicable && ok && strings.Compare(value, "true") == 0 {
				Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
			}
		}

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

	It("[posneg:negative][test_id:3617]Should clone across nodes when multiple local volumes exist,", func() {
		// Get nodes, need at least 2
		nodeList, err := f.K8sClient.CoreV1().Nodes().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		if len(nodeList.Items) < 2 {
			Skip("Need at least 2 nodes to copy accross nodes")
		}
		nodeMap := make(map[string]bool)
		for _, node := range nodeList.Items {
			if ok, _ := nodeMap[node.Name]; !ok {
				nodeMap[node.Name] = true
			}
		}
		// Find PVs and identify local storage, the PVs should already exist.
		pvList, err := f.K8sClient.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		var sourcePV, targetPV *v1.PersistentVolume
		var storageClassName string
		// Verify we have PVs to at least 2 nodes.
		pvNodeNames := make(map[string]int)
		for _, pv := range pvList.Items {
			if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil || len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms) == 0 {
				// Not a local volume PV
				continue
			}
			pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
			if pv.Spec.ClaimRef == nil {
				// PV is available and not claimed.
				if val, ok := pvNodeNames[pvNode]; !ok {
					pvNodeNames[pvNode] = 1
				} else {
					pvNodeNames[pvNode] = val + 1
				}
			}
		}
		if len(pvNodeNames) < 2 {
			Skip("Need PVs on at least 2 nodes to test")
		}

		// Find the source and target PVs so we can label them.
		for _, pv := range pvList.Items {
			if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil || len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms) == 0 {
				// Not a local volume PV
				continue
			}
			if sourcePV == nil {
				if pv.Spec.StorageClassName != "" {
					storageClassName = pv.Spec.StorageClassName
				}
				pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
				if ok, val := nodeMap[pvNode]; ok && val {
					nodeMap[pvNode] = false
					By("Labeling PV " + pv.Name + " as source")
					sourcePV = &pv
					if sourcePV.GetLabels() == nil {
						sourcePV.SetLabels(make(map[string]string))
					}
					sourcePV.GetLabels()["source-pv"] = "yes"
					sourcePV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(sourcePV)
					Expect(err).ToNot(HaveOccurred())
				}
			} else if targetPV == nil {
				pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
				if ok, val := nodeMap[pvNode]; ok && val {
					nodeMap[pvNode] = false
					By("Labeling PV " + pv.Name + " as target")
					targetPV = &pv
					if targetPV.GetLabels() == nil {
						targetPV.SetLabels(make(map[string]string))
					}
					targetPV.GetLabels()["target-pv"] = "yes"
					targetPV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(targetPV)
					Expect(err).ToNot(HaveOccurred())
					break
				}
			}
		}
		Expect(sourcePV).ToNot(BeNil())
		Expect(targetPV).ToNot(BeNil())
		// Source and target PVs have been annotated, now create PVCs with label selectors.
		sourceSelector := make(map[string]string)
		sourceSelector["source-pv"] = "yes"
		sourcePVCDef := utils.NewPVCDefinitionWithSelector(sourcePVCName, "1G", storageClassName, sourceSelector, nil, nil)
		sourcePVCDef.Namespace = f.Namespace.Name
		sourcePVC := f.CreateAndPopulateSourcePVC(sourcePVCDef, sourcePodFillerName, fillCommand+testFile)
		sourcePVC, err = f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(sourcePVC.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePVC.Spec.VolumeName).To(Equal(sourcePV.Name))

		targetDV := utils.NewCloningDataVolume("target-dv", "1G", sourcePVCDef)
		targetDV.Spec.PVC.StorageClassName = &storageClassName
		targetLabelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{
				"target-pv": "yes",
			},
		}
		targetDV.Spec.PVC.Selector = &targetLabelSelector
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
		Expect(err).ToNot(HaveOccurred())

		targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(targetPvc.Spec.VolumeName).To(Equal(targetPV.Name))

		fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
		utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
		completeClone(f, f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum)
	})
})

var _ = Describe("Validate creating multiple clones of same source Data Volume", func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)
	tinyCoreIsoURL := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)

	var (
		sourceDv, targetDv1, targetDv2, targetDv3 *cdiv1.DataVolume
		err                                       error
	)

	AfterEach(func() {
		dvs := []*cdiv1.DataVolume{sourceDv, targetDv1, targetDv2, targetDv3}
		for _, dv := range dvs {
			cleanDv(f, dv)
		}
	})

	It("[rfe_id:1277][test_id:1891][crit:High][vendor:cnv-qe@redhat.com][level:component]Should allow multiple clones from a single source datavolume", func() {
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum /pvc/disk.img")
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		By("Cloning from the source DataVolume to target1")
		targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		By("Cloning from the source DataVolume to target2")
		targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		By("Cloning from the target1 DataVolume to target3")
		targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, targetDv1.Name, targetDv1.Spec.PVC.StorageClassName, targetDv1.Spec.PVC.VolumeMode)
		dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}

		for _, dv := range dvs {
			_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			matchFile := filepath.Join(testBaseDir, "disk.img")
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), matchFile, md5sum[:32])).To(BeTrue())
			_, err = utils.WaitPodDeleted(f.K8sClient, "verify-pvc-md5", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("[rfe_id:1277][test_id:1891][crit:High][vendor:cnv-qe@redhat.com][level:component]Should allow multiple clones from a single source datavolume in block volume mode", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL, f.BlockSCName)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
		retry := 0
		for err != nil && retry < 10 {
			retry++
			md5sum, err = f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
		}
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		By("Cloning from the source DataVolume to target1")
		targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		By("Cloning from the source DataVolume to target2")
		targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		By("Cloning from the target1 DataVolume to target3")
		targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, targetDv1.Name, targetDv1.Spec.PVC.StorageClassName, targetDv1.Spec.PVC.VolumeMode)
		dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}

		for _, dv := range dvs {
			_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), testBaseDir, md5sum[:32])).To(BeTrue())
			_, err = utils.WaitPodDeleted(f.K8sClient, "verify-pvc-md5", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())
		}
	})
})

var _ = Describe("Validate Data Volume clone to smaller size", func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)
	tinyCoreIsoURL := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)

	var (
		sourceDv, targetDv *cdiv1.DataVolume
		err                error
	)

	AfterEach(func() {
		if sourceDv != nil {
			By("Cleaning up source DV")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, sourceDv.Name)
			Expect(err).ToNot(HaveOccurred())
		}
		if targetDv != nil {
			By("Cleaning up target DV")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, targetDv.Name)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("[rfe_id:1126][test_id:1896][crit:High][vendor:cnv-qe@redhat.com][level:component] Should not allow cloning into a smaller sized data volume", func() {
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum /pvc/disk.img")
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		By("Cloning from the source DataVolume to under sized target")
		targetDv = utils.NewDataVolumeForImageCloning("target-dv", "50Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "target resources requests storage size is smaller than the source")).To(BeTrue())

		By("Cloning from the source DataVolume to properly sized target")
		targetDv = utils.NewDataVolumeForImageCloning("target-dv", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for clone to be completed")
		err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, targetDv.Name, 3*90*time.Second)
		Expect(err).ToNot(HaveOccurred())
		matchFile := filepath.Join(testBaseDir, "disk.img")
		Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv), matchFile, md5sum[:32])).To(BeTrue())
		By("Verifying the image is not sparse")
		Expect(f.VerifyNotSparse(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv))).To(BeTrue())

	})

	It("[rfe_id:1126][test_id:1896][crit:High][vendor:cnv-qe@redhat.com][level:component] Should not allow cloning into a smaller sized data volume in block volume mode", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL, f.BlockSCName)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		By("Cloning from the source DataVolume to under sized target")
		targetDv = utils.NewDataVolumeForImageCloning("target-dv", "50Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "target resources requests storage size is smaller than the source")).To(BeTrue())

		By("Cloning from the source DataVolume to properly sized target")
		targetDv = utils.NewDataVolumeForImageCloning("target-dv", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for clone to be completed")
		err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, targetDv.Name, 3*90*time.Second)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv), testBaseDir, md5sum[:32])).To(BeTrue())
	})

})

var _ = Describe("Validate Data Volume should clone multiple clones in parallel", func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)
	tinyCoreIsoURL := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)

	var (
		sourceDv, targetDv1, targetDv2, targetDv3 *cdiv1.DataVolume
		err                                       error
	)

	AfterEach(func() {
		dvs := []*cdiv1.DataVolume{sourceDv, targetDv1, targetDv2, targetDv3}
		for _, dv := range dvs {
			cleanDv(f, dv)
		}
	})

	It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel", func() {
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum /pvc/disk.img")
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		// By not waiting for completion, we will start 3 transfers in parallell
		By("Cloning from the source DataVolume to target1")
		targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv1)
		Expect(err).ToNot(HaveOccurred())
		By("Cloning from the source DataVolume to target2 in parallel")
		targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv2)
		Expect(err).ToNot(HaveOccurred())
		By("Cloning from the source DataVolume to target3 in parallel")
		targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv3)
		Expect(err).ToNot(HaveOccurred())

		dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}
		for _, dv := range dvs {
			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			matchFile := filepath.Join(testBaseDir, "disk.img")
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), matchFile, md5sum[:32])).To(BeTrue())
			_, err = utils.WaitPodDeleted(f.K8sClient, "verify-pvc-md5", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel for block devices", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		By("Creating a source from a real image")
		sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL, f.BlockSCName)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
		Expect(err).ToNot(HaveOccurred())
		By("Waiting for import to be completed")
		utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

		By("Calculating the md5sum of the source data volume")
		md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

		// By not waiting for completion, we will start 3 transfers in parallell
		By("Cloning from the source DataVolume to target1")
		targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv1)
		Expect(err).ToNot(HaveOccurred())
		By("Cloning from the source DataVolume to target2 in parallel")
		targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv2)
		Expect(err).ToNot(HaveOccurred())
		By("Cloning from the source DataVolume to target3 in parallel")
		targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv3)
		Expect(err).ToNot(HaveOccurred())

		dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}
		for _, dv := range dvs {
			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), testBaseDir, md5sum[:32])).To(BeTrue())
			_, err = utils.WaitPodDeleted(f.K8sClient, "verify-pvc-md5", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())
		}
	})
})

var _ = Describe("Block PV Cloner Test", func() {
	var (
		sourcePvc, targetPvc *v1.PersistentVolumeClaim
	)

	f := framework.NewFrameworkOrDie(namespacePrefix)

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
	})

	It("Should clone data across namespaces", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		pvcDef := utils.NewBlockPVCDefinition(sourcePVCName, "500M", nil, nil, f.BlockSCName)
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

func completeClone(f *framework.Framework, targetNs *v1.Namespace, targetPvc *v1.PersistentVolumeClaim, filePath, expectedMD5 string) {
	By("Find cloner pods")
	sourcePod, err := f.FindPodByPrefix(common.ClonerSourcePodName)
	if err != nil {
		PrintControllerLog(f)
	}
	Expect(err).ToNot(HaveOccurred())

	By("Verify the clone annotation is on the target PVC")
	_, cloneAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnCloneOf)
	if err != nil {
		PrintControllerLog(f)
		PrintPodLog(f, sourcePod.Name, sourcePod.Namespace)
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
	deleted, err := utils.WaitPodDeleted(f.K8sClient, sourcePod.Name, sourcePod.Namespace, defaultTimeout)
	Expect(err).ToNot(HaveOccurred())
	Expect(deleted).To(BeTrue())
}

func cloneOfAnnoExistenceTest(f *framework.Framework, targetNamespaceName string) {
	// Create targetPvc
	By(fmt.Sprintf("Creating target pvc: %s/target-pvc", targetNamespaceName))
	_, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNamespaceName, utils.NewPVCDefinition(
		"target-pvc",
		"1G",
		map[string]string{
			controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName,
			controller.AnnCloneOf:      "true",
			controller.AnnPodPhase:     "Succeeded",
		},
		nil))
	Expect(err).ToNot(HaveOccurred())
	By("Checking no cloning pods were created")

	Eventually(func() bool {
		log, err := RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return strings.Contains(log, targetNamespaceName+"/target-pvc not doing anything with: upload=false, clone=true, succeeded=true")
	}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() bool {
		log, err := RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return strings.Contains(log, fmt.Sprintf("Cleaning up for PVC %s/target-pvc", targetNamespaceName))
	}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	Expect(err).ToNot(HaveOccurred())
}

func cleanDv(f *framework.Framework, dv *cdiv1.DataVolume) {
	if dv != nil {
		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dv.Name)
		Expect(err).ToNot(HaveOccurred())
	}
}
