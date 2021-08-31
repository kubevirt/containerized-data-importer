package tests

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
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
			utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)
		}
	})

	It("Verify DataVolume CSI Volume Cloning - volumeMode filesystem - Positive flow", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Volume Clone is not applicable")
		}

		By(fmt.Sprintf("configure storage profile %s", f.CsiCloneSCName))
		utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, f.CsiCloneSCName, originalProfileSpec, cdiv1.CloneStrategyCsiClone)

		dataVolume, md5 := createDataVolume("dv-csi-clone-test-1", utils.DefaultImagePath, v1.PersistentVolumeFilesystem, f.CsiCloneSCName, f)
		verifyEvent(string(cdiv1.CSICloneInProgress), dataVolume.Namespace, f)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultImagePath, md5)
	})

	It("Verify DataVolume CSI Cloning - volumeMode block - Positive flow", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Volume Clone is not applicable")
		}

		By(fmt.Sprintf("configure storage profile %s", f.CsiCloneSCName))
		utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, f.CsiCloneSCName, originalProfileSpec, cdiv1.CloneStrategyCsiClone)

		dataVolume, expectedMd5 := createDataVolume("dv-csi-clone-test-1", utils.DefaultPvcMountPath, v1.PersistentVolumeBlock, f.CsiCloneSCName, f)
		verifyEvent(controller.CSICloneInProgress, dataVolume.Namespace, f)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultPvcMountPath, expectedMd5)
	})

	It("[posneg:negative][test_id:6655] Support for CSI Clone strategy in storage profile with SC HPP - negative", func() {
		if f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("Test should only run on non-csi storage")
		}

		By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
		utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyCsiClone)

		dataVolume, _ := createDataVolumeDontWait("dv-csi-clone-test-1", utils.DefaultImagePath, v1.PersistentVolumeFilesystem, cloneStorageClassName, f)
		waitForDvPhase(cdiv1.CloneScheduled, dataVolume, f)
		verifyEvent(controller.ErrUnableToClone, dataVolume.Namespace, f)
	})
})

func createDataVolumeDontWait(dataVolumeName, testPath string, volumeMode v1.PersistentVolumeMode, scName string, f *framework.Framework) (*cdiv1.DataVolume, string) {
	sourcePvc := createAndPopulateSourcePVC(dataVolumeName, volumeMode, scName, f)
	md5, err := f.GetMD5(f.Namespace, sourcePvc, testPath, utils.UploadFileSize)
	Expect(err).ToNot(HaveOccurred())
	zero := int64(0)
	err = utils.DeletePodByName(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, &zero)

	By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
	dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
	if scName != "" {
		dataVolume.Spec.PVC.StorageClassName = &scName
	}
	By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
	dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
	Expect(err).ToNot(HaveOccurred())

	return dataVolume, md5
}
