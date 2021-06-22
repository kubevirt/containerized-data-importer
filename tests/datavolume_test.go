package tests

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
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
	httpsTinyCoreVmdkURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreVmdkURL, f.CdiInstallNs)
	}
	httpsTinyCoreVdiURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreVdiURL, f.CdiInstallNs)
	}
	httpsTinyCoreVhdURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreVhdURL, f.CdiInstallNs)
	}
	httpsTinyCoreVhdxURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreVhdxURL, f.CdiInstallNs)
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
	// An image that causes qemu-img info to allocate large amounts of RAM
	invalidQcowLargeMemoryURL := func() string {
		return InvalidQcowImagesURL() + "invalid-qcow-large-memory.img"
	}

	errorInvalidQcowLargeMemory := func() string {
		return `Unable to process data: qemu-img: Could not open 'json: {"file.driver": "http", "file.url": "` + invalidQcowLargeMemoryURL() + `", "file.timeout": 3600}': L1 size too big`
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

		createHTTPSDataVolumeWeirdCertFilename := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CreateCertConfigMapWeirdFilename(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
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

		testDataVolume := func(args dataVolumeTestArguments) {
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
		}

		table.DescribeTable("should", testDataVolume,
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
			table.Entry("succeed creating import dv with custom https cert that has a weird filename", dataVolumeTestArguments{
				name:             "dv-https-import-qcow2",
				size:             "1Gi",
				url:              httpsTinyCoreQcow2URL,
				dvFunc:           createHTTPSDataVolumeWeirdCertFilename,
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
			table.Entry("[rfe_id:4334][test_id:6433]succeed creating import dv with given valid registry url and DV barely big enough", dataVolumeTestArguments{
				name:             "dv-import-registry",
				size:             "22Mi", // The image has 18M
				url:              tinyCoreIsoRegistryURL,
				dvFunc:           createRegistryImportDataVolume,
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
					Message: "Import Complete; VDDK: {\"Version\":\"1.2.3\",\"Host\":\"esx.test\"}",
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
					Message: "Import Complete; VDDK: {\"Version\":\"1.2.3\",\"Host\":\"esx.test\"}",
					Reason:  "Completed",
				}}),
		)

		// Similar to previous table, but with additional cleanup steps
		table.DescribeTable("should", func(args dataVolumeTestArguments) {
			configMap, err := RunKubectlCommand(f, "get", "configmap", "-o", "yaml", "-n", f.CdiInstallNs, common.VddkConfigMap)
			Expect(err).ToNot(HaveOccurred())

			_, err = RunKubectlCommand(f, "delete", "configmap", "-n", f.CdiInstallNs, common.VddkConfigMap)
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				command := CreateKubectlCommand(f, "create", "-n", f.CdiInstallNs, "-f", "-")
				command.Stdin = strings.NewReader(configMap)
				err := command.Run()
				Expect(err).ToNot(HaveOccurred())
			}()

			testDataVolume(args)
		},
			table.Entry("[test_id:5079]should fail with \"AwaitingVDDK\" reason when VDDK credentials config map is not present", dataVolumeTestArguments{
				name:             "dv-awaiting-vddk",
				size:             "1Gi",
				url:              vcenterURL,
				dvFunc:           createVddkDataVolume,
				eventReason:      common.AwaitingVDDK,
				phase:            cdiv1.ImportScheduled,
				checkPermissions: false,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionFalse,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: fmt.Sprintf("waiting for %s configmap for VDDK image", common.VddkConfigMap),
					Reason:  common.AwaitingVDDK,
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeRunning,
					Status: v1.ConditionFalse,
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

	table.DescribeTable("Succeed HTTPS import in various formats", func(url func() string) {
		By(fmt.Sprintf("Importing from %s", url()))
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url())
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.HTTP.CertConfigMap = cm

		By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
	},
		table.Entry("when importing in the VMDK format", httpsTinyCoreVmdkURL),
		table.Entry("When importing in the VDI format", httpsTinyCoreVdiURL),
		table.Entry("when importing in the VHD format", httpsTinyCoreVhdURL),
		table.Entry("when importing in the VHDX format", httpsTinyCoreVhdxURL),
	)

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

			By("Trying to create cloned DV a couple of times until we catch the elusive pods")
			var sourcePod *v1.Pod
			var uploadPod *v1.Pod
			Eventually(func() bool {
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

				By("Trying to catch the pods...")
				err = wait.PollImmediate(fastPollingInterval, 10*time.Second, func() (bool, error) {
					sourcePod, err = utils.FindPodBysuffix(f.K8sClient, dataVolume.Namespace, "source-pod", common.CDILabelSelector)
					if err != nil {
						return false, err
					}
					uploadPod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, "cdi-upload", common.CDILabelSelector)
					if err != nil {
						return false, err
					}
					return true, nil
				})

				if err != nil {
					By("Failed to find pods - cleaning up so we can try again")
					err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
					Eventually(func() bool {
						_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
						if k8serrors.IsNotFound(err) {
							return true
						}
						return false
					}, timeout, pollingInterval).Should(BeTrue())

					return false
				}

				return true
			}, timeout, pollingInterval).Should(BeTrue())

			By("Found pods! checking annotations")
			verifyAnnotations(sourcePod)
			verifyAnnotations(uploadPod)
		})
	})

	Describe("Create a PVC using data from StorageProfile", func() {
		var (
			config   *cdiv1.CDIConfig
			origSpec *cdiv1.CDIConfigSpec
			err      error
		)

		fillData := "123456789012345678901234567890123456789012345678901234567890"
		testFile := utils.DefaultPvcMountPath + "/source.txt"
		fillCommand := "echo \"" + fillData + "\" >> " + testFile

		createDataVolumeForImport := func(f *framework.Framework, storageSpec cdiv1.StorageSpec) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(
				dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))

			dataVolume.Spec.Storage = &storageSpec

			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			return dv
		}

		createDataVolumeForUpload := func(f *framework.Framework, storageSpec cdiv1.StorageSpec) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeForUpload(dataVolumeName, "1Mi")
			dataVolume.Spec.PVC = nil
			dataVolume.Spec.Storage = &storageSpec

			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			return dv
		}

		createCloneDataVolume := func(dataVolumeName string, storageSpec cdiv1.StorageSpec, command string) *cdiv1.DataVolume {
			sourcePodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			pvcDef := utils.NewPVCDefinition(pvcName, "10Mi", nil, nil)
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, command)

			By(fmt.Sprintf("creating a new target PVC (datavolume) to clone %s", sourcePvc.Name))
			dataVolume := utils.NewCloningDataVolume(dataVolumeName, "10Mi", sourcePvc)
			dataVolume.Spec.PVC = nil
			dataVolume.Spec.Storage = &storageSpec
			dataVolume.Annotations[controller.AnnImmediateBinding] = "true"

			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			return dv
		}

		getStorageProfileSpec := func(client client.Client, storageClassName string) *cdiv1.StorageProfileSpec {
			storageProfile := &cdiv1.StorageProfile{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, storageProfile)
			Expect(err).ToNot(HaveOccurred())
			originalProfileSpec := storageProfile.Spec.DeepCopy()
			return originalProfileSpec
		}

		updateStorageProfileSpec := func(client client.Client, name string, spec cdiv1.StorageProfileSpec) {
			storageProfile := &cdiv1.StorageProfile{}
			err := client.Get(context.TODO(), types.NamespacedName{Name: name}, storageProfile)
			Expect(err).ToNot(HaveOccurred())
			storageProfile.Spec = spec
			err = client.Update(context.TODO(), storageProfile)
			Expect(err).ToNot(HaveOccurred())
		}

		configureStorageProfile := func(client client.Client,
			storageClassName string,
			accessModes []v1.PersistentVolumeAccessMode,
			volumeMode v1.PersistentVolumeMode) *cdiv1.StorageProfileSpec {

			originalProfileSpec := getStorageProfileSpec(client, storageClassName)
			propertySet := cdiv1.ClaimPropertySet{AccessModes: accessModes, VolumeMode: &volumeMode}
			updateStorageProfileSpec(client,
				storageClassName,
				cdiv1.StorageProfileSpec{ClaimPropertySets: []cdiv1.ClaimPropertySet{propertySet}})

			Eventually(func() cdiv1.ClaimPropertySet {
				profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), storageClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if len(profile.Status.ClaimPropertySets) > 0 {
					return profile.Status.ClaimPropertySets[0]
				}
				return cdiv1.ClaimPropertySet{}
			}, time.Second*30, time.Second).Should(Equal(propertySet))

			return originalProfileSpec
		}

		findStorageProfileWithoutAccessModes := func(client client.Client) string {
			storageProfiles := &cdiv1.StorageProfileList{}
			err := client.List(context.TODO(), storageProfiles)
			Expect(err).ToNot(HaveOccurred())
			for _, storageProfile := range storageProfiles.Items {
				if len(storageProfile.Status.ClaimPropertySets) == 0 {
					// No access modes set.
					return *storageProfile.Status.StorageClass
				}
				for _, properties := range storageProfile.Status.ClaimPropertySets {
					if len(properties.AccessModes) == 0 {
						// No access modes set.
						return *storageProfile.Status.StorageClass
					}
				}
			}
			Skip("No storage profiles without access mode found")
			return ""
		}

		BeforeEach(func() {
			config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			origSpec = config.Spec.DeepCopy()
		})

		AfterEach(func() {
			By("Restoring CDIConfig to original state")
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				origSpec.DeepCopyInto(config)
			})

			Eventually(func() bool {
				config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return reflect.DeepEqual(config.Spec, origSpec)
			}, 30*time.Second, time.Second)
		})

		It("[test_id:5911]Import succeeds creating a PVC from DV without accessModes", func() {
			defaultScName := utils.DefaultStorageClass.GetName()

			By(fmt.Sprintf("configure storage profile %s", defaultScName))
			originalProfileSpec := configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)
			requestedSize := resource.MustParse("100Mi")

			spec := cdiv1.StorageSpec{
				AccessModes:      nil,
				VolumeMode:       nil,
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}

			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc created with correct accessModes")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
		})

		It("[test_id:5912]Import fails creating a PVC from DV without accessModes, no profile", func() {
			// assumes local is available and has no volumeMode
			defaultScName := findStorageProfileWithoutAccessModes(f.CrClient)
			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			requestedSize := resource.MustParse("100Mi")
			spec := cdiv1.StorageSpec{
				AccessModes:      nil,
				VolumeMode:       nil,
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc not created")
			_, err := utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			By(fmt.Sprint("verifying event occurred"))
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, controller.ErrClaimNotValid) && strings.Contains(events, "DataVolume.storage spec is missing accessMode and cannot get access mode from StorageProfile")
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		})

		It("[test_id:5913]Import recovers when user adds accessModes to profile", func() {
			// assumes local is available and has no volumeMode
			defaultScName := findStorageProfileWithoutAccessModes(f.CrClient)
			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			requestedSize := resource.MustParse("100Mi")
			spec := cdiv1.StorageSpec{
				AccessModes:      nil,
				VolumeMode:       nil,
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc not created")
			_, err := utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			By(fmt.Sprint("verifying event occurred"))
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := RunKubectlCommand(f, "get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, controller.ErrClaimNotValid) && strings.Contains(events, "DataVolume.storage spec is missing accessMode and cannot get access mode from StorageProfile")
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			By(fmt.Sprintf("configure storage profile %s", defaultScName))
			originalProfileSpec := configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)

			By("verifying pvc created with correct accessModes")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
		})

		It("[test_id:6483]Import pod should not have size corrected on block", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeBlock
			dataVolume := createDataVolumeForImport(f,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					VolumeMode:       &volumeMode,
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				})

			By("verifying pvc created with correct size")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
		})

		It("[test_id:6484]Import pod should not have size corrected on block, when no volumeMode on DV", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			originalProfileSpec := configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeBlock)

			By("creating datavolume for upload")
			dataVolume := createDataVolumeForImport(f,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				})

			By("verifying pvc created with correct size")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
		})

		It("[test_id:6485]Import pod should have size corrected on filesystem", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// given 50 percent overhead, expected size is 2x requestedSize
			expectedSize := resource.MustParse("200Mi")

			By("creating clone dataVolume")
			volumeMode := v1.PersistentVolumeFilesystem
			dataVolume := createDataVolumeForImport(f,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					VolumeMode:       &volumeMode,
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				})

			By("verifying pvc created with correct size")
			// eventually because pvc will have to be resized if smart clone
			Eventually(func() bool {
				pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
				return pvc.Spec.Resources.Requests.Storage().Cmp(expectedSize) == 0
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("[test_id:6099]Upload pvc should have size corrected on filesystem volume", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// given 50 percent overhead, expected size is 2x requestedSize
			expectedSize := resource.MustParse("200Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeFilesystem
			dataVolume := createDataVolumeForUpload(f, cdiv1.StorageSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				VolumeMode:       &volumeMode,
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			})

			By("verifying pvc created with correct accessModes")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
		})

		It("[test_id:6100]Upload pvc should not have size corrected on block volume", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeBlock

			dataVolume := createDataVolumeForUpload(f, cdiv1.StorageSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				VolumeMode:       &volumeMode,
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			})

			By("verifying pvc created with correct accessModes")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
		})

		It("[test_id:6486]Upload pvc should not have size corrected on block volume, when no volumeMode on DV", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			originalProfileSpec := configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeBlock)

			By("creating datavolume for upload")
			dataVolume := createDataVolumeForUpload(f, cdiv1.StorageSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				StorageClassName: &defaultScName,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			})

			By("verifying pvc created with correct accessModes and size")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
		})

		It("[test_id:6101]Clone pod should not have size corrected on block", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeBlock
			dataVolume := createCloneDataVolume(dataVolumeName,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					VolumeMode:       &volumeMode,
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				}, fillCommand)

			By("verifying pvc created with correct size")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
		})

		It("[test_id:6487]Clone pod should not have size corrected on block, when no volumeMode on DV", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			originalProfileSpec := configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeBlock)

			By("creating datavolume for upload")
			dataVolume := createCloneDataVolume(dataVolumeName,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				}, fillCommand)

			By("verifying pvc created with correct size")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
		})

		It("[test_id:6102]Clone pod should have size corrected on filesystem", func() {
			defaultScName := utils.DefaultStorageClass.GetName()
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// given 50 percent overhead, expected size is 2x requestedSize
			expectedSize := resource.MustParse("200Mi")

			By("creating clone dataVolume")
			volumeMode := v1.PersistentVolumeFilesystem
			dataVolume := createCloneDataVolume(dataVolumeName,
				cdiv1.StorageSpec{
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					VolumeMode:       &volumeMode,
					StorageClassName: &defaultScName,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: requestedSize,
						},
					},
				}, fillCommand)

			By("verifying pvc created with correct size")
			// eventually because pvc will have to be resized if smart clone
			Eventually(func() bool {
				pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
				return pvc.Spec.Resources.Requests.Storage().Cmp(expectedSize) == 0
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
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

	Describe("Priority class on datavolume should transfer to  pods", func() {
		verifyPodAnnotations := func(pod *v1.Pod) {
			By("verifying priority class")
			Expect(pod.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
		}

		It("Importer pod should have priority class specified on datavolume", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URLRateLimit, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s with priority class", dataVolume.Name))
			dataVolume.Spec.PriorityClassName = "system-cluster-critical"
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
				verifyPodAnnotations(sourcePod)
			}
		})

		It("Uploader pod should have priority class specified on datavolume", func() {
			dataVolume := utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			By(fmt.Sprintf("creating new datavolume %s with priority class\"", dataVolume.Name))
			dataVolume.Spec.PriorityClassName = "system-cluster-critical"
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
				verifyPodAnnotations(sourcePod)
			}
		})

		It("Cloner pod should have priority class specified on datavolume", func() {
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
			By(fmt.Sprintf("creating new source dv %s with priority class", sourceDv.Name))
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, sourceDv.Namespace, sourceDv.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			dataVolume := utils.NewCloningDataVolume(dataVolumeName, "1Gi", pvc)
			Expect(dataVolume).ToNot(BeNil())

			By(fmt.Sprintf("creating new datavolume %s with priority class", dataVolume.Name))
			dataVolume.Spec.PriorityClassName = "system-cluster-critical"
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
				verifyPodAnnotations(sourcePod)
				verifyPodAnnotations(uploadPod)
			}
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

func SetFilesystemOverhead(f *framework.Framework, globalOverhead, scOverhead string) {
	defaultSCName := utils.DefaultStorageClass.GetName()
	testedFilesystemOverhead := &cdiv1.FilesystemOverhead{}
	if globalOverhead != "" {
		testedFilesystemOverhead.Global = cdiv1.Percent(globalOverhead)
	}
	if scOverhead != "" {
		testedFilesystemOverhead.StorageClass = map[string]cdiv1.Percent{defaultSCName: cdiv1.Percent(scOverhead)}
	}

	By(fmt.Sprintf("Updating CDIConfig filesystem overhead to %v", testedFilesystemOverhead))
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.FilesystemOverhead = testedFilesystemOverhead.DeepCopy()
	})
	Expect(err).ToNot(HaveOccurred())
	By(fmt.Sprintf("Waiting for filsystem overhead status to be set to %v", testedFilesystemOverhead))
	Eventually(func() bool {
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		if scOverhead != "" {
			return config.Status.FilesystemOverhead.StorageClass[defaultSCName] == cdiv1.Percent(scOverhead)
		}
		return config.Status.FilesystemOverhead.StorageClass[defaultSCName] == cdiv1.Percent(globalOverhead)
	}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig filesystem overhead wasn't set")
}
