package tests

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]SmartClone tests that modify CDI CR", func() {
	var cdiCr cdiv1.CDI
	var cdiCrSpec *cdiv1.CDISpec

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommandFilesystem := "echo -n \"" + fillData + "\" >> " + testFile

	f := framework.NewFramework("dv-func-test")

	BeforeEach(func() {
		By("Saving CDI CR spec")
		crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(crList.Items)).To(Equal(1))

		cdiCrSpec = crList.Items[0].Spec.DeepCopy()
		cdiCr = crList.Items[0]
	})

	AfterEach(func() {
		By("Restoring CDI CR spec to original state")
		crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(crList.Items)).To(Equal(1))

		newCdiCr := crList.Items[0]
		newCdiCr.Spec = *cdiCrSpec
		_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), &newCdiCr, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:5278]Verify DataVolume Smart Cloning gets disabled by tunable", func() {
		if !f.IsSnapshotStorageClassAvailable() {
			Skip("Smart Clone is not applicable")
		}
		var cloneStrategy cdiv1.CDICloneStrategy = cdiv1.CloneStrategyHostAssisted
		cdiCr.Spec.CloneStrategyOverride = &cloneStrategy
		_, err := f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), &cdiCr, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		dataVolume := createDataVolume("dv-smart-clone-test-1", fillCommandFilesystem, v1.PersistentVolumeFilesystem, f.SnapshotSCName, f)
		verifyEvent(controller.CloneInProgress, dataVolume.Namespace, f)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, testFile, fillData)

		events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
		Expect(err).ToNot(HaveOccurred())
		if strings.Contains(events, controller.SnapshotForSmartCloneInProgress) {
			Fail(fmt.Sprintf("seen event SmartClonePVCInProgress. Events: %s", events))
		}
		if strings.Contains(events, controller.SmartClonePVCInProgress) {
			Fail(fmt.Sprintf("seen event SmartClonePVCInProgress. Events: %s", events))
		}
	})
})

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]SmartClone tests", func() {
	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommandFilesystem := "echo -n \"" + fillData + "\" >> " + testFile
	fillCommandBlock := "echo -n \"" + fillData + "\" | dd of=" + utils.DefaultPvcMountPath

	f := framework.NewFramework("dv-func-test")

	It("[rfe_id:1106][test_id:3494][crit:high][vendor:cnv-qe@redhat.com][level:component] Verify DataVolume Smart Cloning - volumeMode filesystem - Positive flow", func() {
		if !f.IsSnapshotStorageClassAvailable() {
			Skip("Smart Clone is not applicable")
		}
		dataVolume := createDataVolume("dv-smart-clone-test-1", fillCommandFilesystem, v1.PersistentVolumeFilesystem, f.SnapshotSCName, f)
		verifyEvent(controller.SnapshotForSmartCloneInProgress, dataVolume.Namespace, f)
		verifyEvent(controller.SmartClonePVCInProgress, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, testFile, fillData)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
	})

	It("[rfe_id:1106][test_id:3495][crit:high][vendor:cnv-qe@redhat.com][level:component] Verify DataVolume Smart Cloning - volumeMode block - Positive flow", func() {
		if !f.IsSnapshotStorageClassAvailable() {
			Skip("Smart Clone is not applicable")
		}
		dataVolume := createDataVolume("dv-smart-clone-test-1", fillCommandBlock, v1.PersistentVolumeBlock, f.SnapshotSCName, f)
		verifyEvent(controller.SnapshotForSmartCloneInProgress, dataVolume.Namespace, f)
		verifyEvent(controller.SmartClonePVCInProgress, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, utils.DefaultPvcMountPath, fillData)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
	})

	It("[test_id:4987]Verify DataVolume Smart Cloning - volumeMode filesystem - Waits for source to be available", func() {
		if !f.IsSnapshotStorageClassAvailable() {
			Skip("Smart Clone is not applicable")
		}
		sourcePvc := createAndPopulateSourcePVC("dv-smart-clone-test-1", fillCommandFilesystem, v1.PersistentVolumeFilesystem, f.SnapshotSCName, f)
		pod, err := f.CreateExecutorPodWithPVC("temp-pod", f.Namespace.Name, sourcePvc)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			pod, err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return pod.Status.Phase == v1.PodRunning
		}, 90*time.Second, 2*time.Second).Should(BeTrue())

		By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
		dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
		if f.SnapshotSCName != "" {
			dataVolume.Spec.PVC.StorageClassName = &f.SnapshotSCName
		}

		By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		verifyEvent(controller.SmartCloneSourceInUse, dataVolume.Namespace, f)
		err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		verifyEvent(controller.SnapshotForSmartCloneInProgress, dataVolume.Namespace, f)
		verifyEvent(controller.SmartClonePVCInProgress, dataVolume.Namespace, f)
		// Verify PVC's content
		verifyPVC(dataVolume, f, testFile, fillData)
		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)
	})

	It("[rfe_id:1106][test_id:3496][crit:high][vendor:cnv-qe@redhat.com][level:component] Verify DataVolume Smart Cloning - Check regular clone works", func() {
		smartApplicable := f.IsSnapshotStorageClassAvailable()
		sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
		if err == nil {
			value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
			if smartApplicable && ok && strings.Compare(value, "true") == 0 {
				Skip("Cannot test regular cloning if Smart Clone is applicable in default Storage Class")
			}
		}

		dataVolume := createDataVolume("dv-smart-clone-test-negative", fillCommandFilesystem, v1.PersistentVolumeFilesystem, "", f)

		// Wait for operation Succeeded
		waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
		verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)

		events, _ := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
		Expect(strings.Contains(events, controller.SnapshotForSmartCloneInProgress)).To(BeFalse())
	})
})

func verifyPVC(dataVolume *cdiv1.DataVolume, f *framework.Framework, testPath string, expectedData string) {
	hash := md5.Sum([]byte(expectedData))
	md5sum := hex.EncodeToString(hash[:])
	By("verifying pvc was created")
	targetPvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	By(fmt.Sprint("Verifying target PVC content"))
	Expect(f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, testPath, md5sum, int64(len(expectedData)))).To(BeTrue())
}

func waitForDvPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume, f *framework.Framework) {
	By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
	err := utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
	if err != nil {
		PrintControllerLog(f)
		dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		if dverr != nil {
			Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
		}
	}
}

func createAndPopulateSourcePVC(dataVolumeName, command string, volumeMode v1.PersistentVolumeMode, scName string, f *framework.Framework) *v1.PersistentVolumeClaim {
	By(fmt.Sprintf("Storage Class name: %s", scName))
	sourcePVCName := fmt.Sprintf("%s-src-pvc", dataVolumeName)
	sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
	pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
	pvcDef.Spec.VolumeMode = &volumeMode
	if scName != "" {
		pvcDef.Spec.StorageClassName = &scName
	}
	return f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)
}

func createDataVolume(dataVolumeName, command string, volumeMode v1.PersistentVolumeMode, scName string, f *framework.Framework) *cdiv1.DataVolume {
	sourcePvc := createAndPopulateSourcePVC(dataVolumeName, command, volumeMode, scName, f)

	By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
	dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
	if scName != "" {
		dataVolume.Spec.PVC.StorageClassName = &scName
	}

	By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
	Expect(err).ToNot(HaveOccurred())

	By("verifying pvc was created, force bind if needed")
	pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
	Expect(err).ToNot(HaveOccurred())
	f.ForceBindIfWaitForFirstConsumer(pvc)

	return dataVolume
}

func verifyEvent(eventReason string, dataVolumeNamespace string, f *framework.Framework) {
	By(fmt.Sprintf("Verifying event occurred: %s", eventReason))
	Eventually(func() bool {
		events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolumeNamespace)
		if err == nil {
			fmt.Fprintf(GinkgoWriter, "%s", events)
			return strings.Contains(events, eventReason)
		}
		fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
		return false
	}, timeout, pollingInterval).Should(BeTrue())
}
