package tests

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component][crit:high][rfe_id:4219] CSI Volume cloning tests", func() {
	var originalProfileSpec *cdiv1.StorageProfileSpec
	var cloneStorageClassName string

	f := framework.NewFramework("dv-func-test")

	BeforeEach(func() {
		cloneStorageClassName = utils.DefaultStorageClass.GetName()
		if f.IsCSIVolumeCloneStorageClassAvailable() {
			cloneStorageClassName = f.CsiCloneSCName
		}

		By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))

		spec, err := utils.GetStorageProfileSpec(f.CdiClient, cloneStorageClassName)
		originalProfileSpec = spec
		Expect(err).ToNot(HaveOccurred())
		By(fmt.Sprintf("Got original storage profile: %v", originalProfileSpec))
	})

	AfterEach(func() {
		if originalProfileSpec != nil {
			By("Restore the profile")
			Expect(utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)).To(Succeed())
		}
	})

	It("Verify DataVolume CSI Volume Cloning - volumeMode filesystem - Positive flow", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Volume Clone is not applicable")
		}

		By(fmt.Sprintf("configure storage profile %s", f.CsiCloneSCName))
		Expect(
			utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, f.CsiCloneSCName, originalProfileSpec, cdiv1.CloneStrategyCsiClone),
		).To(Succeed())

		dataVolume, md5 := createDataVolume("dv-csi-clone-test-1", utils.DefaultImagePath, v1.PersistentVolumeFilesystem, f.CsiCloneSCName, f)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(controller.CloneSucceeded))
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultImagePath, md5)
		// Verify csi clone took place
		verifyCSIClone(dataVolume, f)
	})

	It("Verify DataVolume CSI Cloning - volumeMode block - Positive flow", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Volume Clone is not applicable")
		}

		By(fmt.Sprintf("configure storage profile %s", f.CsiCloneSCName))
		Expect(
			utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, f.CsiCloneSCName, originalProfileSpec, cdiv1.CloneStrategyCsiClone),
		).To(Succeed())

		dataVolume, expectedMd5 := createDataVolume("dv-csi-clone-test-1", utils.DefaultPvcMountPath, v1.PersistentVolumeBlock, f.CsiCloneSCName, f)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(controller.CloneSucceeded))
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultPvcMountPath, expectedMd5)
		// Verify csi clone took place
		verifyCSIClone(dataVolume, f)
	})

	It("StorageProfile setting ignored with non-csi clone", func() {
		if f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("Test should only run on non-csi storage")
		}
		if utils.DefaultStorageClassCsiDriver != nil {
			Skip("default storage class has CSI Driver, cannot run test")
		}

		By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
		Expect(
			utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyCsiClone),
		).To(Succeed())

		dataVolume, _ := createDataVolumeDontWait("dv-csi-clone-test-1", utils.DefaultImagePath, v1.PersistentVolumeFilesystem, cloneStorageClassName, f)
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
	})

	It("[test_id:7736] Should fail to create pvc in namespace with storage quota, then succeed once the quota is large enough", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Volume Clone is not applicable")
		}

		By(fmt.Sprintf("configure storage profile %s", f.CsiCloneSCName))
		Expect(
			utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, f.CsiCloneSCName, originalProfileSpec, cdiv1.CloneStrategyCsiClone),
		).To(Succeed())

		sourcePvc, md5 := createAndVerifySourcePVC("dv-csi-clone-test-1", utils.DefaultImagePath, f.CsiCloneSCName, v1.PersistentVolumeFilesystem, f)
		By("Configure namespace quota after source is ready")
		Expect(f.CreateStorageQuota(int64(2), int64(1024*1024*1024))).To(Succeed())

		dataVolume := createCloneDataVolumeFromSource(sourcePvc, "dv-csi-clone-test-1", f.CsiCloneSCName, f)
		By("Verify Quota was exceeded in events and dv conditions")
		waitForDvPhase(cdiv1.Pending, dataVolume, f)
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(cc.ErrExceededQuota))
		boundCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeBound,
			Status:  v1.ConditionUnknown,
			Message: "exceeded quota",
			Reason:  cc.ErrExceededQuota,
		}
		readyCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeReady,
			Status:  v1.ConditionFalse,
			Message: "exceeded quota",
			Reason:  cc.ErrExceededQuota,
		}
		utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, boundCondition, readyCondition)

		By("Increase quota")
		Expect(f.UpdateStorageQuota(int64(3), int64(4*1024*1024*1024))).To(Succeed())

		By("Verify clone completed after quota increase")
		// Wait for operation Succeeded
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(controller.CloneSucceeded))
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultImagePath, md5)
		// Verify csi clone took place
		verifyCSIClone(dataVolume, f)

		Expect(f.DeleteStorageQuota()).To(Succeed())
	})

})

func createAndVerifySourcePVC(dataVolumeName, testPath, scName string, volumeMode v1.PersistentVolumeMode, f *framework.Framework) (*v1.PersistentVolumeClaim, string) {
	sourcePvc := createAndPopulateSourcePVC(dataVolumeName, volumeMode, scName, f)
	md5, err := f.GetMD5(f.Namespace, sourcePvc, testPath, utils.UploadFileSize)
	Expect(err).ToNot(HaveOccurred())
	zero := int64(0)
	err = utils.DeletePodByName(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, &zero)
	Expect(err).ToNot(HaveOccurred())

	return sourcePvc, md5
}

func createCloneDataVolumeFromSource(sourcePvc *v1.PersistentVolumeClaim, dataVolumeName, scName string, f *framework.Framework) *cdiv1.DataVolume {
	By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
	dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
	if scName != "" {
		dataVolume.Spec.PVC.StorageClassName = &scName
	}
	By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
	Expect(err).ToNot(HaveOccurred())

	return dataVolume
}

func createDataVolumeDontWait(dataVolumeName, testPath string, volumeMode v1.PersistentVolumeMode, scName string, f *framework.Framework) (*cdiv1.DataVolume, string) {
	sourcePvc, md5 := createAndVerifySourcePVC(dataVolumeName, testPath, scName, volumeMode, f)
	dataVolume := createCloneDataVolumeFromSource(sourcePvc, dataVolumeName, scName, f)

	return dataVolume, md5
}

func verifyCSIClone(dataVolume *cdiv1.DataVolume, f *framework.Framework) {
	targetPvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(targetPvc.Spec.DataSource.Kind).To(Equal("VolumeCloneSource"))
	Expect(targetPvc.Spec.DataSourceRef.Kind).To(Equal("VolumeCloneSource"))
}
