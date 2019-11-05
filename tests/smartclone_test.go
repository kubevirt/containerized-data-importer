package tests

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]SmartClone tests", func() {

	var sourcePvc *v1.PersistentVolumeClaim

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommand := "echo \"" + fillData + "\" >> " + testFile

	f := framework.NewFrameworkOrDie("dv-func-test")

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up source PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			sourcePvc = nil
		}
	})

	Describe("Verify DataVolume Smart Cloning - Positive flow", func() {
		It("succeed creating smart-clone dv", func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Smart Clone is not applicable")
			}
			dataVolume := createDataVolume("dv-smart-clone-test-1", sourcePvc, fillCommand, f.SnapshotSCName, f)
			// Wait for snapshot creation to start
			waitForDvPhase(cdiv1.SnapshotForSmartCloneInProgress, dataVolume, f)
			verifyEvent(controller.SnapshotForSmartCloneInProgress, dataVolume.Namespace, f)
			// Wait for PVC creation to start
			waitForDvPhase(cdiv1.SmartClonePVCInProgress, dataVolume, f)
			verifyEvent(controller.SmartClonePVCInProgress, dataVolume.Namespace, f)
			// Verify PVC's content
			verifyPVC(dataVolume, f)
			// Wait for operation Succeeded
			waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
			verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)

			// Cleanup
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Verify DataVolume Smart Cloning - Check regular clone works", func() {
		It("Verify inapplicable smart-clone dv", func() {
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test regular cloning if Smart Clone is applicable in default Storage Class")
				}
			}

			dataVolume := createDataVolume("dv-smart-clone-test-negative", sourcePvc, fillCommand, "", f)

			// Wait for operation Succeeded
			waitForDvPhase(cdiv1.Succeeded, dataVolume, f)
			verifyEvent(controller.CloneSucceeded, dataVolume.Namespace, f)

			events, _ := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
			Expect(strings.Contains(events, controller.SnapshotForSmartCloneInProgress)).To(BeFalse())

			// Cleanup
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func verifyPVC(dataVolume *cdiv1.DataVolume, f *framework.Framework) {
	By("verifying pvc was created")
	targetPvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	By(fmt.Sprint("Verifying target PVC content"))
	Expect(f.VerifyTargetPVCContent(f.Namespace, targetPvc, fillData, testBaseDir, testFile)).To(BeTrue())
}

func waitForDvPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume, f *framework.Framework) {
	By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
	err := utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
	if err != nil {
		PrintControllerLog(f)
		dv, dverr := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
		if dverr != nil {
			Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
		}
	}
}

func createDataVolume(dataVolumeName string, sourcePvc *v1.PersistentVolumeClaim, command string, scName string, f *framework.Framework) *cdiv1.DataVolume {
	By(fmt.Sprintf("Storage Class name: %s", scName))
	sourcePVCName := fmt.Sprintf("%s-src-pvc", dataVolumeName)
	sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
	pvcDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
	if scName != "" {
		pvcDef.Spec.StorageClassName = &scName
	}
	sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

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
