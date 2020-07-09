package tests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
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
	httpsTinyCoreQcow2URL := fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs)
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

	createRegistryImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithRegistryImport(dataVolumeName, size, url)
		cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.Registry.CertConfigMap = cm
		return dataVolume
	}

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up target PVC")
			err := f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			sourcePvc = nil
		}
	})

	Describe("Verify DataVolume", func() {
		type dataVolumeTestArguments struct {
			name             string
			size             string
			url              string
			dvFunc           func(string, string, string) *cdiv1.DataVolume
			errorMessage     string
			eventReason      string
			phase            cdiv1.DataVolumePhase
			repeat           int
			checkPermissions bool
			readyCondition   *cdiv1.DataVolumeCondition
			boundCondition   *cdiv1.DataVolumeCondition
			runningCondition *cdiv1.DataVolumeCondition
		}

		createImageIoDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			stringData := map[string]string{
				common.KeyAccess: "YWRtaW5AaW50ZXJuYWw=",
				common.KeySecret: "MTIzNDU2",
			}
			s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
			return utils.NewDataVolumeWithImageioImport(dataVolumeName, size, url, s.Name, cm, "123")
		}

		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			return dataVolume
		}

		createCloneDataVolume := func(dataVolumeName, size, command string) *cdiv1.DataVolume {
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			pvcDef := utils.NewPVCDefinition(pvcName, size, nil, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

			By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
			return utils.NewCloningDataVolume(dataVolumeName, size, sourcePvc)
		}

		createBlankRawDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage(dataVolumeName, size)
		}

		createUploadDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dataVolumeName, size)
		}

		table.DescribeTable("should", func(args dataVolumeTestArguments) {
			// Have to call the function in here, to make sure the BeforeEach in the Framework has run.
			dataVolume := args.dvFunc(args.name, args.size, args.url)
			startTime := time.Now()
			repeat := 1
			if utils.IsHostpathProvisioner() && args.repeat > 0 {
				// Repeat rapidly to make sure we don't get regular and scratch space on different nodes.
				repeat = args.repeat
			}

			for i := 0; i < repeat; i++ {
				By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprintf("waiting for datavolume to match phase %s", string(args.phase)))
				err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, args.phase, dataVolume.Name)
				if err != nil {
					dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
					if dverr != nil {
						Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
					}
				}
				Expect(err).ToNot(HaveOccurred())

				By("Verifying the DV has the correct conditions and messages for those conditions")
				Eventually(func() bool {
					// Doing this as eventually because in failure scenarios, we could still be in a retry and the running condition
					// will not match if the pod hasn't failed and the backoff is not long enough yet
					resultDv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
					Expect(dverr).ToNot(HaveOccurred())
					return verifyConditions(resultDv.Status.Conditions, startTime, args.readyCondition, args.runningCondition, args.boundCondition)
				}, timeout, pollingInterval).Should(BeTrue())

				// verify PVC was created
				By("verifying pvc was created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprint("Verifying event occurred"))
				Eventually(func() bool {
					// Only find DV events, we know the PVC gets the same events
					events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
					if err == nil {
						fmt.Fprintf(GinkgoWriter, "%s", events)
						return strings.Contains(events, args.eventReason) && strings.Contains(events, args.errorMessage)
					}
					fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
					return false
				}, timeout, pollingInterval).Should(BeTrue())

				if args.checkPermissions {
					// Verify the created disk image has the right permissions.
					By("Verifying permissions are 660")
					Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")
					err := utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())
				}
				By("Cleaning up")
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
			table.Entry("[rfe_id:1115][crit:high][test_id:1357]succeed creating import dv with given valid url", dataVolumeTestArguments{
				name:             "dv-http-import",
				size:             "1Gi",
				url:              tinyCoreIsoURL,
				dvFunc:           utils.NewDataVolumeWithHTTPImport,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-import Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1358]fail creating import dv due to invalid DNS entry", dataVolumeTestArguments{
				name:         "dv-http-import-invalid-url",
				size:         "1Gi",
				url:          "http://i-made-this-up.kube-system/tinyCore.iso",
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to connect to http data source",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-import-invalid-url Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to connect to http data source: Get http://i-made-this-up.kube-system/tinyCore.iso: dial tcp: lookup i-made-this-up.kube-system",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1359]fail creating import dv due to file not found", dataVolumeTestArguments{
				name:         "dv-http-import-404",
				size:         "1Gi",
				url:          tinyCoreIsoURL + "not.real.file",
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to connect to http data source: expected status code 200, got 404. Status: 404 Not Found",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-import-404 Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to connect to http data source: expected status code 200, got 404. Status: 404 Not Found",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2555]fail creating import dv: invalid qcow large size", dataVolumeTestArguments{
				name:         "dv-invalid-qcow-large-size",
				size:         "1Gi",
				url:          invalidQcowLargeSizeURL,
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to process data: Invalid format qcow for image " + invalidQcowLargeSizeURL,
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-invalid-qcow-large-size Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data: Invalid format qcow for image " + invalidQcowLargeSizeURL,
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2554]fail creating import dv: invalid qcow large json", dataVolumeTestArguments{
				name:         "dv-invalid-qcow-large-json",
				size:         "1Gi",
				url:          invalidQcowLargeJSONURL,
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to process data: qemu-img: curl: The requested URL returned error: 416 Requested Range Not Satisfiable",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-invalid-qcow-large-json Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data: qemu-img: curl: The requested URL returned error: 416 Requested Range Not Satisfiable",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2253]fail creating import dv: invalid qcow large memory", dataVolumeTestArguments{
				name:         "dv-invalid-qcow-large-memory",
				size:         "1Gi",
				url:          invalidQcowLargeMemoryURL,
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to process data: qemu-img: Could not open '/data/disk.img': L1 size too big",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-invalid-qcow-large-memory Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data: qemu-img: Could not open '/data/disk.img': L1 size too big",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2139]fail creating import dv: invalid qcow backing file", dataVolumeTestArguments{
				name:         "dv-invalid-qcow-backing-file",
				size:         "1Gi",
				url:          invalidQcowBackingFileURL,
				dvFunc:       utils.NewDataVolumeWithHTTPImport,
				errorMessage: "Unable to process data: qemu-img: Could not open '/data/disk.img': L1 size too big",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-invalid-qcow-backing-file Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data: qemu-img: Could not open '/data/disk.img': L1 size too big",
					Reason:  "Error",
				}}),
			table.Entry("[test_id:3931]succeed creating import dv with streaming image conversion", dataVolumeTestArguments{
				name:             "dv-http-stream-import",
				size:             "1Gi",
				url:              cirrosURL,
				dvFunc:           utils.NewDataVolumeWithHTTPImport,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-stream-import Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1115][crit:high][test_id:1379]succeed creating import dv with given valid url (https)", dataVolumeTestArguments{
				name:             "dv-https-import",
				size:             "1Gi",
				url:              httpsTinyCoreIsoURL,
				dvFunc:           createHTTPSDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-https-import Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1115][crit:high][test_id:1379]succeed creating import dv with given valid qcow2 url (https) should require scratchspace", dataVolumeTestArguments{
				name:             "dv-https-import-qcow2",
				size:             "1Gi",
				url:              httpsTinyCoreQcow2URL,
				dvFunc:           createHTTPSDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-https-import-qcow2 Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1111][crit:high][test_id:1361]succeed creating blank image dv", dataVolumeTestArguments{
				name:             "blank-image-dv",
				size:             "1Gi",
				dvFunc:           createBlankRawDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC blank-image-dv Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:138][crit:high][test_id:1362]succeed creating upload dv", dataVolumeTestArguments{
				name:        "upload-dv",
				size:        "1Gi",
				dvFunc:      createUploadDataVolume,
				eventReason: controller.UploadReady,
				phase:       cdiv1.UploadReady,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
					Reason: "TransferRunning",
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC upload-dv Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeRunning,
					Status: v1.ConditionTrue,
					Reason: "Pod is running",
				}}),
			table.Entry("[rfe_id:1947][crit:high][test_id:2145]succeed creating import dv with given tar archive url", dataVolumeTestArguments{
				name:        "dv-tar-archive",
				size:        "1Gi",
				url:         tarArchiveURL,
				dvFunc:      utils.NewDataVolumeWithArchiveContent,
				eventReason: controller.ImportSucceeded,
				phase:       cdiv1.Succeeded,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-tar-archive Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1947][crit:high][test_id:2220]fail creating import dv with non tar archive url", dataVolumeTestArguments{
				name:         "dv-non-tar-archive",
				size:         "1Gi",
				url:          tinyCoreIsoURL,
				dvFunc:       utils.NewDataVolumeWithArchiveContent,
				errorMessage: "Unable to process data: exit status 2",
				eventReason:  "Error",
				phase:        cdiv1.ImportInProgress,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-non-tar-archive Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data: exit status 2",
					Reason:  "Error",
				}}),
			table.Entry("[test_id:3932]succeed creating dv from imageio source", dataVolumeTestArguments{
				name:             "dv-imageio-test",
				size:             "1Gi",
				url:              imageioURL,
				dvFunc:           createImageIoDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-imageio-test Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1277][crit:high][test_id:1360]succeed creating clone dv", dataVolumeTestArguments{
				name:        "dv-clone-test1",
				size:        "1Gi",
				url:         fillCommand, // its not URL, but command, but the parameter lines up.
				dvFunc:      createCloneDataVolume,
				eventReason: controller.CloneSucceeded,
				phase:       cdiv1.Succeeded,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-clone-test1 Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Clone Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:1115][crit:high][test_id:1478]succeed creating import dv with given valid registry url", dataVolumeTestArguments{
				name:             "dv-import-registry",
				size:             "1Gi",
				url:              tinyCoreIsoRegistryURL,
				dvFunc:           createRegistryImportDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				repeat:           10,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-import-registry Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
		)

		It("should handle a pre populated PVC", func() {
			By(fmt.Sprintf("initializing source PVC %s", dataVolumeName))
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			annotations := map[string]string{"cdi.kubevirt.io/storage.populatedFor": dataVolumeName}
			pvcDef := utils.NewPVCDefinition(dataVolumeName, "1G", annotations, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand)

			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", cirrosURL)
			By(fmt.Sprintf("creating new populated datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcName := dv.Annotations["cdi.kubevirt.io/storage.prePopulated"]
				return pvcName == pvcDef.Name &&
					dv.Status.Phase == cdiv1.Succeeded &&
					string(dv.Status.Progress) == "N/A"
			}, timeout, pollingInterval).Should(BeTrue())
		})
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
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
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
			table.Entry("[test_id:3933]succeed creating import dv with given valid url", "import-http", "", tinyCoreIsoURL, "dv-phase-test-1", controller.ImportSucceeded, cdiv1.Succeeded),
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

				By(fmt.Sprintf("waiting for datavolume to match phase %s", cdiv1.ImportInProgress))
				utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)

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
				It(fmt.Sprintf("[test_id:3939][test_id:3940][test_id:3941][test_id:3942][test_id:3943]should succeed on loop %d", i), func() {
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
		It("[test_id:3934]Should report progress while importing", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile("\\d{1,3}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())
		})
	})

	Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component][test] CDI Import from HTTP/S3", func() {
		const (
			originalImageName = "cirros-qcow2.img"
			testImageName     = "cirros-qcow2-1990.img"
		)
		var (
			dataVolume              *cdiv1.DataVolume
			err                     error
			tinyCoreIsoRateLimitURL = "http://cdi-file-host." + f.CdiInstallNs + ":82/cirros-qcow2-1990.img"
		)

		BeforeEach(func() {
			By("Prepare the file")
			fileHostPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, utils.FileHostName, "name="+utils.FileHostName)
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"cp /tmp/shared/images/"+originalImageName+" /tmp/shared/images/"+testImageName)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			By("Delete DV")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Cleanup the file")
			fileHostPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, utils.FileHostName, "name="+utils.FileHostName)
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"rm -f /tmp/shared/images/"+testImageName)
			Expect(err).To(BeNil())

			By("Verifying pvc was deleted")
			deleted, err := utils.WaitPVCDeleted(f.K8sClient, dataVolume.Name, dataVolume.Namespace, timeout)
			Expect(deleted).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:1990] CDI Data Volume - file is removed from http server while import is in progress", func() {
			dvName := "import-file-removed"
			By(fmt.Sprintf("Creating new datavolume %s", dvName))
			dv := utils.NewDataVolumeWithHTTPImport(dvName, "500Mi", tinyCoreIsoRateLimitURL)
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())

			phase := cdiv1.ImportInProgress
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			// here we want to have more than 0, to be sure it started
			progressRegExp := regexp.MustCompile("[1-9]\\d{0,2}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				fmt.Fprintf(GinkgoWriter, "INFO: current progress:%v, matches:%v\n", progress, progressRegExp.MatchString(string(progress)))
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())

			By("Remove source image file & kill http container to force restart")
			fileHostPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, utils.FileHostName, "name="+utils.FileHostName)
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"rm /tmp/shared/images/"+testImageName)
			Expect(err).To(BeNil())

			By("Verify the number of retries on the datavolume")
			Eventually(func() int32 {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(dataVolume.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				restarts := dv.Status.RestartCount
				return restarts
			}, timeout, pollingInterval).Should(BeNumerically(">=", 1))

			By("Restore the file, import should progress")
			utils.WaitTimeoutForPodReady(f.K8sClient, fileHostPod.Name, fileHostPod.Namespace, utils.PodWaitForTime)
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"cp /tmp/shared/images/"+originalImageName+" /tmp/shared/images/"+testImageName)
			Expect(err).To(BeNil())

			By("Wait for the eventual success")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 300*time.Second)
			Expect(err).To(BeNil())
		})
	})

	Describe("Delete PVC during registry import", func() {
		var dataVolume *cdiv1.DataVolume

		AfterEach(func() {
			if dataVolume != nil {
				By("[AfterEach] Clean up DV")
				err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				dataVolume = nil
			}
		})

		It("Should create a new PVC when PVC is deleted during import", func() {
			dataVolumeSpec := createRegistryImportDataVolume(dataVolumeName, "1Gi", tinyCoreIsoRegistryURL)
			By(fmt.Sprintf("creating new datavolume %s", dataVolumeSpec.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolumeSpec)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for DV's PVC")
			pvc, err := utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			pvcUID := pvc.GetUID()

			By("Wait for import to start")
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)

			By(fmt.Sprintf("Deleting PVC %v", pvc.Name))
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc)
			Expect(err).ToNot(HaveOccurred())
			deleted, err := f.WaitPVCDeletedByUID(pvc, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())

			By("Wait for PVC to be recreated")
			pvc, err = utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			Expect(pvc.GetUID()).ToNot(Equal(pvcUID))

			By("Wait for DV to succeed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 10*time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Registry import with missing configmap", func() {
		const cmName = "cert-registry-cm"

		It("Import POD should remain pending until CM exists", func() {
			var pvc *corev1.PersistentVolumeClaim

			dataVolumeDef := utils.NewDataVolumeWithRegistryImport("missing-cm-registry-dv", "1Gi", tinyCoreIsoRegistryURL)
			dataVolumeDef.Spec.Source.Registry.CertConfigMap = cmName
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolumeDef)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			Eventually(func() bool {
				// TODO: fix this to use the mechanism to find the correct PVC once we decouple the DV and PVC names
				pvc, _ = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(dataVolume.Name, metav1.GetOptions{})
				return pvc != nil && pvc.Name != ""
			}, timeout, pollingInterval).Should(BeTrue())

			By("Verifying the POD remains pending for 30 seconds")
			podName := naming.GetResourceName(common.ImporterPodName, pvc.Name)
			Consistently(func() bool {
				pod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(podName, metav1.GetOptions{})
				if err == nil {
					// Found the pod
					Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
					if len(pod.Status.ContainerStatuses) == 1 && pod.Status.ContainerStatuses[0].State.Waiting != nil {
						Expect(pod.Status.ContainerStatuses[0].State.Waiting.Reason).To(Equal("ContainerCreating"))
					}
					fmt.Fprintf(GinkgoWriter, "INFO: pod found, pending, container creating: %s\n", podName)
				} else if k8serrors.IsNotFound(err) {
					fmt.Fprintf(GinkgoWriter, "INFO: pod not found: %s\n", podName)
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				return true
			}, time.Second*30, time.Second).Should(BeTrue())

			By("Creating the config map")
			_, err = utils.CopyRegistryCertConfigMapDestName(f.K8sClient, f.Namespace.Name, f.CdiInstallNs, cmName)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(cdiv1.Succeeded)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func verifyConditions(actualConditions []cdiv1.DataVolumeCondition, startTime time.Time, testConditions ...*cdiv1.DataVolumeCondition) bool {
	for _, condition := range testConditions {
		if condition != nil {
			actualCondition := findConditionByType(condition.Type, actualConditions)
			if actualCondition != nil {
				if actualCondition.Status != condition.Status {
					fmt.Fprintf(GinkgoWriter, "INFO: Condition.Status does not match for type: %s\n", condition.Type)
					return false
				}
				if strings.Compare(actualCondition.Reason, condition.Reason) != 0 {
					fmt.Fprintf(GinkgoWriter, "INFO: Condition.Reason does not match for type: %s, reason expected [%s], reason found: [%s]\n", condition.Type, condition.Reason, actualCondition.Reason)
					return false
				}
				if !strings.Contains(actualCondition.Message, condition.Message) {
					fmt.Fprintf(GinkgoWriter, "INFO: Condition.Message does not match for type: %s, message expected: [%s],  message found: [%s]\n", condition.Type, condition.Message, actualCondition.Message)
					return false
				}
			}
		}
	}
	return true
}

func findConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
