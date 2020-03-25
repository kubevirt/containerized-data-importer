package tests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	pollingInterval = 2 * time.Second
	timeout         = 270 * time.Second
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]DataVolume tests", func() {

	var sourcePvc *v1.PersistentVolumeClaim

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommand := "echo \"" + fillData + "\" >> " + testFile

	f := framework.NewFrameworkOrDie("dv-func-test")

	tinyCoreIsoURL := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)
	httpsTinyCoreIsoURL := fmt.Sprintf(utils.HTTPSTinyCoreIsoURL, f.CdiInstallNs)
	tinyCoreIsoRegistryURL := fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs)
	tarArchiveURL := fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs)
	InvalidQcowImagesURL := fmt.Sprintf(utils.InvalidQcowImagesURL, f.CdiInstallNs)
	cirrosURL := fmt.Sprintf(utils.CirrosURL, f.CdiInstallNs)
	imageioURL := fmt.Sprintf(utils.ImageioURL, f.CdiInstallNs)

	// Invalid (malicious) QCOW images:
	// An image that causes qemu-img to allocate 152T (original image is 516 bytes)
	invalidQcowLargeSizeURL := InvalidQcowImagesURL + "invalid-qcow-large-size.img"
	// An image that causes qemu-img info to output half a million lines of JSON
	invalidQcowLargeJSONURL := InvalidQcowImagesURL + "invalid-qcow-large-json.img"
	// An image that causes qemu-img info to allocate large amounts of RAM
	invalidQcowLargeMemoryURL := InvalidQcowImagesURL + "invalid-qcow-large-memory.img"
	// An image with a backing file - should be rejected when converted to raw
	invalidQcowBackingFileURL := InvalidQcowImagesURL + "invalid-qcow-backing-file.img"

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up target PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			sourcePvc = nil
		}
	})

	Describe("Verify DataVolume", func() {

		table.DescribeTable("should", func(name, command, url, dataVolumeName, errorMessage, eventReason string, phase cdiv1.DataVolumePhase) {
			repeat := 1
			var dataVolume *cdiv1.DataVolume
			switch name {
			case "imageio":
				cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
				Expect(err).To(BeNil())
				stringData := map[string]string{
					common.KeyAccess: "YWRtaW5AaW50ZXJuYWw=",
					common.KeySecret: "MTIzNDU2",
				}
				s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
				dataVolume = utils.NewDataVolumeWithImageioImport(dataVolumeName, "1Gi", imageioURL, s.Name, cm, "123")
			case "import-http":
				dataVolume = utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)
			case "import-https":
				dataVolume = utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)
				cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
				Expect(err).To(BeNil())
				dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			case "blank":
				dataVolume = utils.NewDataVolumeForBlankRawImage(dataVolumeName, "1Gi")
			case "upload":
				dataVolume = utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			case "clone":
				sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
				pvcDef := utils.NewPVCDefinition(pvcName, "1G", nil, nil)
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

				By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
				dataVolume = utils.NewCloningDataVolume(dataVolumeName, "1Gi", sourcePvc)
			case "import-registry":
				if utils.IsHostpathProvisioner() {
					// Repeat rapidly to make sure we don't get regular and scratch space on different nodes.
					repeat = 10
				}
				dataVolume = utils.NewDataVolumeWithRegistryImport(dataVolumeName, "1Gi", url)
				cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
				Expect(err).To(BeNil())
				dataVolume.Spec.Source.Registry.CertConfigMap = cm
			case "import-archive":
				dataVolume = utils.NewDataVolumeWithArchiveContent(dataVolumeName, "1Gi", url)
			}

			for i := 0; i < repeat; i++ {
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
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprint("Verifying event occurred"))
				Eventually(func() bool {
					events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace)
					if err == nil {
						fmt.Fprintf(GinkgoWriter, "%s", events)
						return strings.Contains(events, eventReason) && strings.Contains(events, errorMessage)
					}
					fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
					return false
				}, timeout, pollingInterval).Should(BeTrue())

				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
					if k8serrors.IsNotFound(err) {
						return true
					}
					return false
				}, timeout, pollingInterval).Should(BeTrue())
			}
		},
			table.Entry("[rfe_id:1115][crit:high][test_id:1357]succeed creating import dv with given valid url", "import-http", "", tinyCoreIsoURL, "dv-phase-test-1", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1358]fail creating import dv due to invalid DNS entry", "import-http", "", "http://i-made-this-up.kube-system/tinyCore.iso", "dv-phase-test-2", "Unable to connect to http data source", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1359]fail creating import dv due to file not found", "import-http", "", tinyCoreIsoURL+"not.real.file", "dv-phase-test-3", "Unable to connect to http data source", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1277][crit:high][test_id:1360]succeed creating clone dv", "clone", fillCommand, "", "dv-clone-test-1", "", controller.CloneSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1111][crit:high][test_id:1361]succeed creating blank image dv", "blank", "", "", "blank-image-dv", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:138][crit:high][test_id:1362]succeed creating upload dv", "upload", "", "", "upload-dv", "", controller.UploadReady, cdiv1.UploadReady),
			table.Entry("[rfe_id:1115][crit:high][test_id:1478]succeed creating import dv with given valid registry url", "import-registry", "", tinyCoreIsoRegistryURL, "dv-phase-test-4", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1115][crit:high][test_id:1379]succeed creating import dv with given valid url (https)", "import-https", "", httpsTinyCoreIsoURL, "dv-phase-test-1", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2555]fail creating import dv: invalid qcow large size", "import-http", "", invalidQcowLargeSizeURL, "dv-invalid-qcow-large-size", "Unable to process data: Invalid format qcow for image", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2554]fail creating import dv: invalid qcow large json", "import-http", "", invalidQcowLargeJSONURL, "dv-invalid-qcow-large-json", "Unable to process data: exit status 1", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2253]fail creating import dv: invalid qcow large memory", "import-http", "", invalidQcowLargeMemoryURL, "dv-invalid-qcow-large-memory", "Unable to process data: exit status 1", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2139]fail creating import dv: invalid qcow backing file", "import-http", "", invalidQcowBackingFileURL, "dv-invalid-qcow-backing-file", "Unable to process data: exit status 1", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("[rfe_id:1947][crit:high][test_id:2145]succeed creating import dv with given tar archive url", "import-archive", "", tarArchiveURL, "tar-archive-dv", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[rfe_id:1947][crit:high][test_id:2220]fail creating import dv with non tar archive url", "import-archive", "", tinyCoreIsoURL, "non-tar-archive-dv", "Unable to process data: exit status 2", controller.ImportFailed, cdiv1.ImportInProgress),
			table.Entry("succeed creating import dv with streaming image conversion", "import-http", "", cirrosURL, "dv-phase-test-1", "", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("succeed creating dv from imageio source", "imageio", "", imageioURL, "dv-phase-test-1", "", controller.ImportSucceeded, cdiv1.Succeeded),
		)
	})

	Describe("[rfe_id:1111][test_id:2001][crit:low][vendor:cnv-qe@redhat.com][level:component]Verify multiple blank disk creations in parallel", func() {
		var (
			dataVolume1, dataVolume2, dataVolume3, dataVolume4 *cdiv1.DataVolume
		)

		AfterEach(func() {
			dvs := []*cdiv1.DataVolume{dataVolume1, dataVolume2, dataVolume3, dataVolume4}
			for _, dv := range dvs {
				cleanDv(f, dv)
			}
		})

		It("Should create all of them successfully", func() {
			dataVolume1 = utils.NewDataVolumeForBlankRawImage("dv-1", "100Mi")
			dataVolume2 = utils.NewDataVolumeForBlankRawImage("dv-2", "100Mi")
			dataVolume3 = utils.NewDataVolumeForBlankRawImage("dv-3", "100Mi")
			dataVolume4 = utils.NewDataVolumeForBlankRawImage("dv-4", "100Mi")

			dvs := []*cdiv1.DataVolume{dataVolume1, dataVolume2, dataVolume3, dataVolume4}
			for _, dv := range dvs {
				_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
			}

			By("Waiting for Datavolume to have succeeded")
			for _, dv := range dvs {
				err := utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(f.VerifyBlankDisk(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv))).To(BeTrue())
			}
		})
	})

	Describe("Verify DataVolume with block mode", func() {
		var err error
		var dataVolume *cdiv1.DataVolume

		AfterEach(func() {
			if dataVolume != nil {
				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		table.DescribeTable("should", func(name, command, url, dataVolumeName, eventReason string, phase cdiv1.DataVolumePhase) {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			switch name {
			case "import-http":
				dataVolume = utils.NewDataVolumeWithHTTPImportToBlockPV(dataVolumeName, "1G", url, f.BlockSCName)
			}
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
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
		},
			table.Entry("succeed creating import dv with given valid url", "import-http", "", tinyCoreIsoURL, "dv-phase-test-1", controller.ImportSucceeded, cdiv1.Succeeded),
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

	Describe("Create/Delete same datavolume in a loop", func() {
		Context("retry loop", func() {
			dataVolumeName := "dv1"
			url := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)
			numTries := 5
			for i := 1; i <= numTries; i++ {
				It(fmt.Sprintf("should succeed on loop %d", i), func() {
					dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)

					By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
					dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
					Expect(err).ToNot(HaveOccurred())

					By(fmt.Sprintf("waiting for datavolume to match phase %s", cdiv1.Succeeded))
					utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)

					By("deleting DataVolume")
					err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
					Expect(err).ToNot(HaveOccurred())

				})
			}
		})
	})

	Describe("Progress reporting on import datavolume", func() {
		It("Should report progress while importing", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile("\\d{1,3}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())
		})
	})
})
