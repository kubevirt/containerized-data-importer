package tests

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/extensions/table"

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
	fastPollingInterval = 20 * time.Millisecond
	pollingInterval     = 2 * time.Second
	timeout             = 270 * time.Second
	shortTimeout        = 30 * time.Second
)

var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]DataVolume tests", func() {

	var sourcePvc *v1.PersistentVolumeClaim

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	testFile := utils.DefaultPvcMountPath + "/source.txt"
	fillCommand := "echo \"" + fillData + "\" >> " + testFile

	f := framework.NewFramework("dv-func-test")

	tinyCoreIsoURL := func() string {
		return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)
	}
	httpsTinyCoreIsoURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreIsoURL, f.CdiInstallNs)
	}
	httpsTinyCoreQcow2URL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs)
	}
	tinyCoreQcow2URL := func() string {
		return fmt.Sprintf(utils.TinyCoreQcow2URL+".gz", f.CdiInstallNs)
	}
	tinyCoreIsoRegistryURL := func() string {
		return fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs)
	}
	tinyCoreIsoRegistryProxyURL := func() string {
		return fmt.Sprintf(utils.TinyCoreIsoRegistryProxyURL, f.CdiInstallNs)
	}
	tarArchiveURL := func() string {
		return fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs)
	}
	InvalidQcowImagesURL := func() string {
		return fmt.Sprintf(utils.InvalidQcowImagesURL, f.CdiInstallNs)
	}
	cirrosURL := func() string {
		return fmt.Sprintf(utils.CirrosURL, f.CdiInstallNs)
	}
	imageioURL := func() string {
		return fmt.Sprintf(utils.ImageioURL, f.CdiInstallNs)
	}
	vcenterURL := func() string {
		return fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs)
	}

	// Invalid (malicious) QCOW images:
	// An image that causes qemu-img to allocate 152T (original image is 516 bytes)
	invalidQcowLargeSizeURL := func() string {
		return InvalidQcowImagesURL() + "invalid-qcow-large-size.img"
	}
	// An image that causes qemu-img info to output half a million lines of JSON
	invalidQcowLargeJSONURL := func() string {
		return InvalidQcowImagesURL() + "invalid-qcow-large-json.img"
	}
	// An image that causes qemu-img info to allocate large amounts of RAM
	invalidQcowLargeMemoryURL := func() string {
		return InvalidQcowImagesURL() + "invalid-qcow-large-memory.img"
	}
	// An image with a backing file - should be rejected when converted to raw
	invalidQcowBackingFileURL := func() string {
		return InvalidQcowImagesURL() + "invalid-qcow-backing-file.img"
	}

	errorInvalidQcowLargeMemory := func() string {
		return `Unable to process data: qemu-img: Could not open 'json: {"file.driver": "http", "file.url": "` + invalidQcowLargeMemoryURL() + `", "file.timeout": 3600}': L1 size too big`
	}
	errorInvalidQcowBackingFile := func() string {
		return `Unable to process data: qemu-img: Could not open 'json: {"file.driver": "http", "file.url": "` + invalidQcowBackingFileURL() + `", "file.timeout": 3600}': L1 size too big`
	}

	createRegistryImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithRegistryImport(dataVolumeName, size, url)
		cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.Registry.CertConfigMap = cm
		return dataVolume
	}

	createProxyRegistryImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithRegistryImport(dataVolumeName, size, url)
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.Registry.CertConfigMap = cm
		return dataVolume
	}

	createVddkDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		// Find vcenter-simulator pod
		pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "vcenter-deployment", "app=vcenter")
		Expect(err).ToNot(HaveOccurred())
		Expect(pod).ToNot(BeNil())

		// Get test VM UUID
		id, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmid")
		Expect(err).To(BeNil())
		vmid, err := uuid.Parse(strings.TrimSpace(id))
		Expect(err).To(BeNil())

		// Get disk name
		disk, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmdisk")
		Expect(err).To(BeNil())
		disk = strings.TrimSpace(disk)
		Expect(err).To(BeNil())

		// Create VDDK login secret
		stringData := map[string]string{
			common.KeyAccess: "user",
			common.KeySecret: "pass",
		}
		backingFile := disk
		secretRef := "vddksecret"
		thumbprint := "testprint"
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, secretRef))

		return utils.NewDataVolumeWithVddkImport(dataVolumeName, size, backingFile, s.Name, thumbprint, url, vmid.String())
	}

	createVddkWarmImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		// Find vcenter-simulator pod
		pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "vcenter-deployment", "app=vcenter")
		Expect(err).ToNot(HaveOccurred())
		Expect(pod).ToNot(BeNil())

		// Get test VM UUID
		id, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmid")
		Expect(err).To(BeNil())
		vmid, err := uuid.Parse(strings.TrimSpace(id))
		Expect(err).To(BeNil())

		// Get snapshot 1 ID
		previousCheckpoint, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmsnapshot1")
		Expect(err).To(BeNil())
		previousCheckpoint = strings.TrimSpace(previousCheckpoint)
		Expect(err).To(BeNil())

		// Get snapshot 2 ID
		currentCheckpoint, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmsnapshot2")
		Expect(err).To(BeNil())
		currentCheckpoint = strings.TrimSpace(currentCheckpoint)
		Expect(err).To(BeNil())

		// Get disk name
		disk, err := RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmdisk")
		Expect(err).To(BeNil())
		disk = strings.TrimSpace(disk)
		Expect(err).To(BeNil())

		// Create VDDK login secret
		stringData := map[string]string{
			common.KeyAccess: "user",
			common.KeySecret: "pass",
		}
		backingFile := disk
		secretRef := "vddksecret"
		thumbprint := "testprint"
		finalCheckpoint := true
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, secretRef))

		return utils.NewDataVolumeWithVddkWarmImport(dataVolumeName, size, backingFile, s.Name, thumbprint, url, vmid.String(), currentCheckpoint, previousCheckpoint, finalCheckpoint)
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
			url              func() string
			dvFunc           func(string, string, string) *cdiv1.DataVolume
			errorMessage     string
			errorMessageFunc func() string
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
			dataVolume := args.dvFunc(args.name, args.size, args.url())
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
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				By(fmt.Sprintf("waiting for datavolume to match phase %s", string(args.phase)))
				err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, args.phase, dataVolume.Name)
				if err != nil {
					dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					if dverr != nil {
						Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
					}
				}
				Expect(err).ToNot(HaveOccurred())

				By("Verifying the DV has the correct conditions and messages for those conditions")
				Eventually(func() bool {
					// Doing this as eventually because in failure scenarios, we could still be in a retry and the running condition
					// will not match if the pod hasn't failed and the backoff is not long enough yet
					resultDv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					Expect(dverr).ToNot(HaveOccurred())
					return verifyConditions(resultDv.Status.Conditions, startTime, args.readyCondition, args.runningCondition, args.boundCondition)
				}, timeout, pollingInterval).Should(BeTrue())

				// verify PVC was created
				By("verifying pvc was created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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
					Eventually(func() bool {
						result, _ := f.VerifyPermissions(f.Namespace, pvc)
						return result
					}, shortTimeout, pollingInterval).Should(BeTrue(), "Permissions on disk image are not 660")
					err := utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())
				}
				By("Cleaning up")
				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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
				url:          func() string { return "http://i-made-this-up.kube-system/tinyCore.iso" },
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
					Message: "Unable to connect to http data source",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1115][crit:high][posneg:negative][test_id:1359]fail creating import dv due to file not found", dataVolumeTestArguments{
				name:         "dv-http-import-404",
				size:         "1Gi",
				url:          func() string { return tinyCoreIsoURL() + "not.real.file" },
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
				errorMessage: "Unable to process data: Invalid format qcow for image",
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
					Message: "Unable to process data: Invalid format qcow for image",
					Reason:  "Error",
				},
			}),
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
				name:             "dv-invalid-qcow-large-memory",
				size:             "1Gi",
				url:              invalidQcowLargeMemoryURL,
				dvFunc:           utils.NewDataVolumeWithHTTPImport,
				errorMessageFunc: errorInvalidQcowLargeMemory,
				eventReason:      "Error",
				phase:            cdiv1.ImportInProgress,
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
					Message: "L1 size too big",
					Reason:  "Error",
				}}),
			table.Entry("[rfe_id:1120][crit:high][posneg:negative][test_id:2139]fail creating import dv: invalid qcow backing file", dataVolumeTestArguments{
				name:             "dv-invalid-qcow-backing-file",
				size:             "1Gi",
				url:              invalidQcowBackingFileURL,
				dvFunc:           utils.NewDataVolumeWithHTTPImport,
				errorMessageFunc: errorInvalidQcowBackingFile,
				eventReason:      "Error",
				phase:            cdiv1.ImportInProgress,
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
					Message: "L1 size too big",
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
			table.Entry("[rfe_id:1115][crit:high][test_id:1379]succeed creating import dv with given valid qcow2 url (https)", dataVolumeTestArguments{
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
				url:              func() string { return "" },
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
				url:         func() string { return "" },
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
				url:         func() string { return fillCommand }, // its not URL, but command, but the parameter lines up.
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
			table.Entry("[test_id:5077]succeed creating import dv from VDDK source", dataVolumeTestArguments{
				name:             "dv-import-vddk",
				size:             "1Gi",
				url:              vcenterURL,
				dvFunc:           createVddkDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: false,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-import-vddk Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[test_id:5078]succeed creating warm import dv from VDDK source", dataVolumeTestArguments{
				name:             "dv-import-vddk",
				size:             "1Gi",
				url:              vcenterURL,
				dvFunc:           createVddkWarmImportDataVolume,
				eventReason:      controller.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: false,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-import-vddk Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
		)

		It("[test_id:4961]should handle a pre populated PVC", func() {
			By(fmt.Sprintf("initializing source PVC %s", dataVolumeName))
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			annotations := map[string]string{"cdi.kubevirt.io/storage.populatedFor": dataVolumeName}
			pvcDef := utils.NewPVCDefinition(dataVolumeName, "1G", annotations, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand)

			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", cirrosURL())
			By(fmt.Sprintf("creating new populated datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())

				By("verifying pvc was created")
				pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindIfWaitForFirstConsumer(pvc)
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

		table.DescribeTable("should", func(name, command string, url func() string, dataVolumeName, eventReason string, phase cdiv1.DataVolumePhase) {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			switch name {
			case "import-http":
				dataVolume = utils.NewDataVolumeWithHTTPImportToBlockPV(dataVolumeName, "1G", url(), f.BlockSCName)
			case "import-vddk":
				dataVolume = createVddkDataVolume(dataVolumeName, "1Gi", vcenterURL())
				utils.ModifyDataVolumeWithVDDKImportToBlockPV(dataVolume, f.BlockSCName)
			case "warm-import-vddk":
				dataVolume = createVddkWarmImportDataVolume(dataVolumeName, "1Gi", vcenterURL())
				utils.ModifyDataVolumeWithVDDKImportToBlockPV(dataVolume, f.BlockSCName)
			}
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created")
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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
			table.Entry("[test_id:3935]succeed import from VDDK to block volume", "import-vddk", "", nil, "dv-vddk-import-test", controller.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[test_id:3936]succeed warm import from VDDK to block volume", "warm-import-vddk", "", nil, "dv-vddk-warm-import-test", controller.ImportSucceeded, cdiv1.Succeeded),
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
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				By(fmt.Sprintf("waiting for datavolume to match phase %s", cdiv1.ImportInProgress))
				utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)

				By("verifying pvc and pod were created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				podName := pvc.Annotations[controller.AnnImportPod]

				pod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName, metav1.GetOptions{})
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
			numTries := 5
			It(fmt.Sprintf("[test_id:3939][test_id:3940][test_id:3941][test_id:3942][test_id:3943] should succeed on %d loops", numTries), func() {
				var prevPvcUID string
				dataVolumeName := "test-dv"
				dataVolumeNamespace := f.Namespace
				for i := 1; i <= numTries; i++ {
					By(fmt.Sprintf("running loop %d", i))
					url := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)
					dv := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url)

					By(fmt.Sprintf("creating new datavolume %s", dataVolumeName))
					// the DV creation must not fail eventhough the PVC of the previous created DV is still terminating
					dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, dataVolumeNamespace.Name, dv)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

					By("verifying pvc was created and is bound")
					_, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())

					By("verifying if the created pvc is not the old deleted one")
					var pvc *v1.PersistentVolumeClaim
					Eventually(func() bool {
						pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
						if err != nil {
							return false
						}
						// Return true if the pvc is not being deleted, and the UID is no longer the original one.
						return pvc.DeletionTimestamp == nil && string(pvc.UID) != prevPvcUID
					}, timeout, pollingInterval).Should(BeTrue())
					// We use the PVC UID to confirm later a new PVC was created
					prevPvcUID = string(pvc.UID)

					By(fmt.Sprintf("waiting for datavolume to match phase %s", cdiv1.Succeeded))
					err = utils.WaitForDataVolumePhase(f.CdiClient, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())

					By("deleting DataVolume")
					err = utils.DeleteDataVolume(f.CdiClient, dataVolume.Namespace, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
				}
			})
		})
	})

	Describe("Pass specific datavolume annotations to the transfer pods", func() {
		verifyAnnotations := func(pod *v1.Pod) {
			By("verifying passed annotation")
			Expect(pod.Annotations[controller.AnnPodNetwork]).To(Equal("net1"))
			Expect(pod.Annotations[controller.AnnPodSidecarInjection]).To(Equal(controller.AnnPodSidecarInjectionDefault))
			By("verifying non-passed annotation")
			Expect(pod.Annotations["annot1"]).ToNot(Equal("value1"))
		}

		It("[test_id:5353]Importer pod should have specific datavolume annotations passed but not others", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s with annotations", dataVolume.Name))
			dataVolume.Annotations[controller.AnnPodNetwork] = "net1"
			dataVolume.Annotations["annot1"] = "value1"
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("verifying the Datavolume is not complete yet")
			foundDv, err := f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if foundDv.Status.Phase != cdiv1.Succeeded {
				By("find importer pod")
				var sourcePod *v1.Pod
				Eventually(func() bool {
					sourcePod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
					return err == nil
				}, timeout, pollingInterval).Should(BeTrue())
				verifyAnnotations(sourcePod)
			}
		})

		It("[test_id:5365]Uploader pod should have specific datavolume annotations passed but not others", func() {
			dataVolume := utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			By(fmt.Sprintf("creating new datavolume %s with annotations", dataVolume.Name))
			dataVolume.Annotations[controller.AnnPodNetwork] = "net1"
			dataVolume.Annotations["annot1"] = "value1"
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("verifying the Datavolume is not complete yet")
			foundDv, err := f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if foundDv.Status.Phase != cdiv1.Succeeded {
				By("find uploader pod")
				var sourcePod *v1.Pod
				Eventually(func() bool {
					sourcePod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, "cdi-upload", common.CDILabelSelector)
					return err == nil
				}, timeout, pollingInterval).Should(BeTrue())
				verifyAnnotations(sourcePod)
			}
		})

		It("[test_id:5366]Cloner pod should have specific datavolume annotations passed but not others", func() {
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test if annotations are present when all pvcs are smart clone capable.")
				}
			}

			sourceDv := utils.NewDataVolumeWithHTTPImport("source-dv", "1Gi", tinyCoreQcow2URL())
			Expect(sourceDv).ToNot(BeNil())
			By(fmt.Sprintf("creating new source dv %s with annotations", sourceDv.Name))
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, sourceDv.Namespace, sourceDv.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", pvc)
			Expect(dataVolume).ToNot(BeNil())

			By(fmt.Sprintf("creating new datavolume %s with annotations", dataVolume.Name))
			dataVolume.Annotations[controller.AnnPodNetwork] = "net1"
			dataVolume.Annotations["annot1"] = "value1"
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("verifying the Datavolume is not complete yet")
			foundDv, err := f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if foundDv.Status.Phase != cdiv1.Succeeded {
				By("find source and target pod")
				var sourcePod *v1.Pod
				var uploadPod *v1.Pod
				Eventually(func() bool {
					if sourcePod == nil {
						sourcePod, _ = utils.FindPodBysuffix(f.K8sClient, dataVolume.Namespace, "source-pod", common.CDILabelSelector)
					}
					if uploadPod == nil {
						uploadPod, _ = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, "cdi-upload", common.CDILabelSelector)
					}
					return sourcePod != nil && uploadPod != nil
				}, timeout, fastPollingInterval).Should(BeTrue())
				verifyAnnotations(sourcePod)
				verifyAnnotations(uploadPod)
			}
		})
	})

	Describe("Progress reporting on import datavolume", func() {
		It("[test_id:3934]Should report progress while importing", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile("\\d{1,3}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())
		})
	})

	Describe("[rfe_id:4223][crit:high] DataVolume - WaitForFirstConsumer", func() {
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

		createBlankRawDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage(dataVolumeName, size)
		}
		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			return dataVolume
		}
		createUploadDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dataVolumeName, size)
		}

		createCloneDataVolume := func(dataVolumeName, size, command string) *cdiv1.DataVolume {
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			pvcDef := utils.NewPVCDefinition(pvcName, size, nil, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

			By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
			return utils.NewCloningDataVolume(dataVolumeName, size, sourcePvc)
		}
		var original *bool
		noSuchFileFileURL := utils.InvalidQcowImagesURL + "no-such-file.img"

		BeforeEach(func() {
			previousValue, err := utils.DisableFeatureGate(f.CrClient, featuregates.HonorWaitForFirstConsumer)
			Expect(err).ToNot(HaveOccurred())
			original = previousValue
		})

		AfterEach(func() {
			if original != nil && *original {
				// restore
				_, err := utils.EnableFeatureGate(f.CrClient, featuregates.HonorWaitForFirstConsumer)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		table.DescribeTable("Feature Gate - disabled", func(
			dvName string,
			url func() string,
			dvFunc func(string, string, string) *cdiv1.DataVolume,
			phase cdiv1.DataVolumePhase) {
			if !utils.IsHostpathProvisioner() {
				Skip("Not HPP")
			}
			size := "1Gi"
			By("Verify No FeatureGates")
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Spec.FeatureGates).To(BeNil())

			dataVolume := dvFunc(dvName, size, url())

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created and is Bound")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		},
			table.Entry("[test_id:4459] Import Positive flow",
				"dv-wffc-http-import",
				func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) },
				utils.NewDataVolumeWithHTTPImport,
				cdiv1.Succeeded),
			table.Entry("[test_id:4460] Import invalid url",
				"dv-wffc-http-url-not-valid-import",
				func() string { return fmt.Sprintf(noSuchFileFileURL, f.CdiInstallNs) },
				utils.NewDataVolumeWithHTTPImport,
				cdiv1.ImportInProgress),
			table.Entry("[test_id:4461] Import qcow2 scratch space",
				"dv-wffc-qcow2-import",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createHTTPSDataVolume,
				cdiv1.Succeeded),
			table.Entry("[test_id:4462] Import blank image",
				"dv-wffc-blank-import",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createBlankRawDataVolume,
				cdiv1.Succeeded),
			table.Entry("[test_id:4463] Upload - positive flow",
				"dv-wffc-upload",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createUploadDataVolume,
				cdiv1.UploadReady),
			table.Entry("[test_id:4464] Clone - positive flow",
				"dv-wffc-clone",
				func() string { return fillCommand }, // its not URL, but command, but the parameter lines up.
				createCloneDataVolume,
				cdiv1.Succeeded),
		)
	})

	Describe("[crit:high] DataVolume - WaitForFirstConsumer", func() {
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

		createBlankRawDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage(dataVolumeName, size)
		}
		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			return dataVolume
		}
		createUploadDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dataVolumeName, size)
		}

		createCloneDataVolume := func(dataVolumeName, size, command string) *cdiv1.DataVolume {
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			pvcDef := utils.NewPVCDefinition(pvcName, size, nil, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

			By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
			return utils.NewCloningDataVolume(dataVolumeName, size, sourcePvc)
		}

		table.DescribeTable("WFFC Feature Gate enabled - ImmediateBinding requested", func(
			dvName string,
			url func() string,
			dvFunc func(string, string, string) *cdiv1.DataVolume,
			phase cdiv1.DataVolumePhase) {
			if !utils.IsHostpathProvisioner() {
				Skip("Not HPP")
			}
			size := "1Gi"

			dataVolume := dvFunc(dvName, size, url())
			dataVolume.Annotations[controller.AnnImmediateBinding] = "true"

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created and is Bound")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		},
			table.Entry("Import qcow2 scratch space",
				"dv-immediate-wffc-qcow2-import",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createHTTPSDataVolume,
				cdiv1.Succeeded),
			table.Entry("Import blank image",
				"dv-immediate-wffc-blank-import",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createBlankRawDataVolume,
				cdiv1.Succeeded),
			table.Entry("Upload - positive flow",
				"dv-immediate-wffc-upload",
				func() string { return fmt.Sprintf(utils.HTTPSTinyCoreQcow2URL, f.CdiInstallNs) },
				createUploadDataVolume,
				cdiv1.UploadReady),
			table.Entry("Clone - positive flow",
				"dv-immediate-wffc-clone",
				func() string { return fillCommand }, // its not URL, but command, but the parameter lines up.
				createCloneDataVolume,
				cdiv1.Succeeded),
		)
	})

	Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component][test] CDI Import from HTTP/S3", func() {
		const (
			originalImageName = "cirros-qcow2.img"
			testImageName     = "cirros-qcow2-1990.img"
		)
		var (
			dataVolume              *cdiv1.DataVolume
			err                     error
			tinyCoreIsoRateLimitURL = func() string { return "http://cdi-file-host." + f.CdiInstallNs + ":82/cirros-qcow2-1990.img" }
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
			dv := utils.NewDataVolumeWithHTTPImport(dvName, "500Mi", tinyCoreIsoRateLimitURL())
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			phase := cdiv1.ImportInProgress
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			// here we want to have more than 0, to be sure it started
			progressRegExp := regexp.MustCompile("[1-9]\\d{0,2}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
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

		It("[test_id:4962]Should create a new PVC when PVC is deleted during import", func() {
			dataVolumeSpec := createProxyRegistryImportDataVolume(dataVolumeName, "1Gi", tinyCoreIsoRegistryProxyURL())
			By(fmt.Sprintf("Creating new datavolume %s", dataVolumeSpec.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolumeSpec)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Waiting for DV's PVC")
			pvc, err := utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			pvcUID := pvc.GetUID()

			By("Wait for import to start")
			utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)

			By(fmt.Sprintf("Deleting PVC %v (id: %v)", pvc.Name, pvcUID))
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc)
			Expect(err).ToNot(HaveOccurred())
			deleted, err := f.WaitPVCDeletedByUID(pvc, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())

			By("Wait for PVC to be recreated")
			pvc, err = utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			By(fmt.Sprintf("Recreated PVC %v (id: %v)", pvc.Name, pvc.GetUID()))
			Expect(pvc.GetUID()).ToNot(Equal(pvcUID))
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("Wait for DV to succeed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 10*time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Registry import with missing configmap", func() {
		const cmName = "cert-registry-cm"

		It("[test_id:4963]Import POD should remain pending until CM exists", func() {
			var pvc *v1.PersistentVolumeClaim

			dataVolumeDef := utils.NewDataVolumeWithRegistryImport("missing-cm-registry-dv", "1Gi", tinyCoreIsoRegistryURL())
			dataVolumeDef.Spec.Source.Registry.CertConfigMap = cmName
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolumeDef)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("verifying pvc was created")
			Eventually(func() bool {
				// TODO: fix this to use the mechanism to find the correct PVC once we decouple the DV and PVC names
				pvc, _ = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return pvc != nil && pvc.Name != ""
			}, timeout, pollingInterval).Should(BeTrue())

			By("Verifying the POD remains pending for 30 seconds")
			podName := naming.GetResourceName(common.ImporterPodName, pvc.Name)
			Consistently(func() bool {
				pod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName, metav1.GetOptions{})
				if err == nil {
					// Found the pod
					Expect(pod.Status.Phase).To(Equal(v1.PodPending))
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

	Describe("Delete scratch PVC during import", func() {
		var dataVolume *cdiv1.DataVolume

		AfterEach(func() {
			if dataVolume != nil {
				By("[AfterEach] Clean up DV")
				err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				dataVolume = nil
			}
		})

		It("Should create a new scratch PVC when PVC is deleted during import", func() {
			// The test tries to catch issues in handling a deleted scratch PVC during import. There were at least
			// two problems found and both required a very specific timing between scratch delete and controller actions.
			// When quickly retrying to delete a scratch a few times the probability of catching the problem is increased.
			// If there are no problems the test always PASSES
			// In case of bugs in controller it should fail most of the time .
			dvName := "import-bug"
			By(fmt.Sprintf("Creating new datavolume %s", dvName))
			dv := createRegistryImportDataVolume(dvName, "500Mi", tinyCoreIsoRegistryURL())
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Waiting for DV's PVC")
			_, err = utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			By("Wait for DV in ImportScheduled")
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			scratchPvcName := naming.GetResourceName(dataVolume.Name, common.ScratchNameSuffix)
			By("Trying to delete scratch PVC " + scratchPvcName)

			deleteCounter := 0
			// The number of retries was chosen empirically. Retrying 5 times is enough to catch the problem.
			retries := 5
			Eventually(func() int {
				err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(context.TODO(), scratchPvcName, metav1.DeleteOptions{})
				if err == nil {
					deleteCounter++
					By(fmt.Sprintf("Deleted scratch PVC %s %v", scratchPvcName, deleteCounter))
				}
				return deleteCounter
			}, 270*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", retries))

			By("Wait for PVC to be recreated")
			scratchPvc, err := utils.WaitForPVC(f.K8sClient, f.Namespace.Name, scratchPvcName)
			Expect(err).ToNot(HaveOccurred())
			By(fmt.Sprintf("Recreated PVC %v (id: %v)", scratchPvc.Name, scratchPvc.GetUID()))

			By("Wait for DV to succeed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 10*time.Minute)
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
