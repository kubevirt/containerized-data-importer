package tests

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	pollingInterval = 2 * time.Second
	timeout         = 90 * time.Second
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]DataVolume tests", func() {

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
		table.DescribeTable("should", func(name, command, url, dataVolumeName, eventReason string, phase cdiv1.DataVolumePhase) {
			var dataVolume *cdiv1.DataVolume
			switch name {
			case "import-http":
				dataVolume = utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)
			case "blank":
				dataVolume = utils.NewDataVolumeForBlankRawImage(dataVolumeName, "1Gi")
			case "upload":
				dataVolume = utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			case "clone":
				sourcePVCName := fmt.Sprintf("%s-src-pvc", dataVolumeName)
				sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
				sourcePvc = f.CreateAndPopulateSourcePVC(sourcePVCName, sourcePodFillerName, command)

				By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
				dataVolume = utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
			case "import-registry":
				dataVolume = utils.NewDataVolumeWithRegistryImport(dataVolumeName, "1Gi", url)
			}

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
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
			table.Entry("[rfe_id:1115][crit:high][test_id:1357]succeed creating import dv with given valid url", "import-http", "", utils.TinyCoreIsoURL, "dv-phase-test-1", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1358]fail creating import dv due to invalid DNS entry", "import-http", "", "http://i-made-this-up.kube-system/tinyCore.iso", "dv-phase-test-2", controller.ImportFailed, cdiv1.Failed),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1359]fail creating import dv due to file not found", "import-http", "", utils.TinyCoreIsoURL+"not.real.file", "dv-phase-test-3", controller.ImportFailed, cdiv1.Failed),
			table.Entry("[rfe_id:1277][crit:high][test_id:1360]succeed creating clone dv", "clone", fillCommand, "", "dv-clone-test-1", controller.CloneSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1111][crit:high][test_id:1361]succeed creating blank image dv", "blank", "", "", "blank-image-dv", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:138][crit:high][test_id:1362]succeed creating upload dv", "upload", "", "", "upload-dv", controller.UploadReady, cdiv1.Succeeded),
			table.Entry("[rfe_id:1115][crit:high][test_id:1478]succeed creating import dv with given valid registry url", "import-registry", "", utils.TinyCoreIsoRegistryURL, "dv-phase-test-4", controller.ImportSucceeded, cdiv1.Succeeded),
		)
	})

	Describe("[rfe_id:1115][crit:high][posneg:negative]Delete resources of DataVolume with an invalid URL (POD in retry loop)", func() {
		Context("using invalid import URL for DataVolume", func() {
			dataVolumeName := "invalid-url-dv"
			url := "http://nothing.2.c/here.iso"
			It("[test_id:1363]should create/delete all resources", func() {
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
