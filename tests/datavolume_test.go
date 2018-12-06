package tests

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/datavolumecontroller/v1alpha1"
)

const (
	pollingInterval = 2 * time.Second
	timeout         = 90 * time.Second
)

var _ = Describe("DataVolume tests", func() {

	var sourcePvc *v1.PersistentVolumeClaim

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommand := "echo \"" + fillData + "\" >> " + testFile

	f := framework.NewFrameworkOrDie("dv-func-test")

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up target PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			sourcePvc = nil
		}
	})

	Describe("Verify DataVolume", func() {
		table.DescribeTable("with http import source should", func(url string, phase cdiv1.DataVolumePhase, dataVolumeName string, eventReason string) {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)

			// verify PVC was created
			By("verifying pvc was created")
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprint("Verifying event occurred"))
			Eventually(func() bool {
				events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, eventReason)
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

		},
			table.Entry("succeed when given valid url", utils.TinyCoreIsoURL, cdiv1.Succeeded, "dv-phase-test-1", controller.ImportSucceeded),
			table.Entry("fail due to invalid DNS entry", "http://i-made-this-up.kube-system/tinyCore.iso", cdiv1.Failed, "dv-phase-test-2", controller.ImportFailed),
			table.Entry("fail due to file not found", utils.TinyCoreIsoURL+"not.real.file", cdiv1.Failed, "dv-phase-test-3", controller.ImportFailed),
		)

		table.DescribeTable("with clone source should", func(command string, phase cdiv1.DataVolumePhase, dataVolumeName string) {

			sourcePVCName := fmt.Sprintf("%s-src-pvc", dataVolumeName)
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)

			sourcePvc = f.CreateAndPopulateSourcePVC(sourcePVCName, sourcePodFillerName, command)

			By(fmt.Sprintf("creating a new PVC with name %s", sourcePvc.Name))
			dataVolume := utils.NewDataVolumeWithPVCImport(dataVolumeName, "1Gi", sourcePvc)

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created")
			targetPvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			if phase == cdiv1.Succeeded {
				By("verifying DataVolume contents are correct")
				Expect(f.VerifyTargetPVCContent(f.Namespace, targetPvc, testFile, fillData)).To(BeTrue())
			}

			By(fmt.Sprintf("Verifying event %s occurred", controller.CloneSucceeded))
			Eventually(func() bool {
				events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
				Expect(err).NotTo(HaveOccurred())
				return strings.Contains(events, controller.CloneSucceeded)
			}, timeout, pollingInterval).Should(BeTrue())

			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

		},
			table.Entry("succeed when given a source PVC with a data", fillCommand, cdiv1.Succeeded, "dv-clone-test-1"),
		)
	})

	Describe("Delete resources of DataVolume with an invalid URL (POD in retry loop)", func() {
		Context("using invalid import URL for DataVolume", func() {
			dataVolumeName := "invalid-url-dv"
			url := "http://nothing.2.c/here.iso"
			It("should create/delete all resources", func() {
				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)

				By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprintf("waiting for datavolume to match phase %s", "Failed"))
				utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, "Failed", dataVolume.Name)

				// verify PVC was created
				By("verifying pvc and pod were created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				pvcName := pvc.Name
				podName := pvc.Annotations[controller.AnnImportPod]

				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(pvcName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				pod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(podName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("deleting DataVolume")
				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
				Expect(err).ToNot(HaveOccurred())

				By("verifying pod was deleted")
				deleted, err := utils.WaitPodDeleted(f.K8sClient, pod.Name, f.Namespace.Name, timeout)
				Expect(deleted).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				By("verifying pvc was deleted")
				deleted, err = utils.WaitPVCDeleted(f.K8sClient, pvc.Name, f.Namespace.Name, timeout)
				Expect(deleted).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
