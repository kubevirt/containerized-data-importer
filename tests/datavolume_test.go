package tests

import (
	"context"
	"encoding/base64"
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
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
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
	var targetPvc *v1.PersistentVolumeClaim

	fillData := "123456789012345678901234567890123456789012345678901234567890"
	fillDataFSMD5sum := "fabc176de7eb1b6ca90b3aa4c7e035f3"
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
	httpsTinyCoreZstURL := func() string {
		return fmt.Sprintf(utils.HTTPSTinyCoreZstURL, f.CdiInstallNs)
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
	tinyCoreIsoAuthURL := func() string {
		return fmt.Sprintf(utils.TinyCoreIsoAuthURL, f.CdiInstallNs)
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
	cirrosGCSQCOWURL := func() string {
		return fmt.Sprintf(utils.CirrosGCSQCOWURL, f.CdiInstallNs)
	}
	cirrosGCSRAWURL := func() string {
		return fmt.Sprintf(utils.CirrosGCSRAWURL, f.CdiInstallNs)
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
		Expect(err).ToNot(HaveOccurred())
		dataVolume.Spec.Source.Registry.CertConfigMap = &cm
		return dataVolume
	}

	createProxyRegistryImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithRegistryImport(dataVolumeName, size, url)
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		dataVolume.Spec.Source.Registry.CertConfigMap = &cm
		return dataVolume
	}

	createVddkDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		// Find vcenter-simulator pod
		pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "vcenter-deployment", "app=vcenter")
		Expect(err).ToNot(HaveOccurred())
		Expect(pod).ToNot(BeNil())

		// Get test VM UUID
		id, err := f.RunKubectlCommand("exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmid")
		Expect(err).ToNot(HaveOccurred())
		vmid, err := uuid.Parse(strings.TrimSpace(id))
		Expect(err).ToNot(HaveOccurred())

		// Get disk name
		disk, err := f.RunKubectlCommand("exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmdisk")
		Expect(err).ToNot(HaveOccurred())
		disk = strings.TrimSpace(disk)
		Expect(err).ToNot(HaveOccurred())

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
		return f.CreateVddkWarmImportDataVolume(dataVolumeName, size, url)
	}

	createImageIoDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		stringData := map[string]string{
			common.KeyAccess: "admin@internal",
			common.KeySecret: "123456",
		}
		CreateImageIoDefaultInventory(f)
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		return utils.NewDataVolumeWithImageioImport(dataVolumeName, size, url, s.Name, cm, "123")
	}

	createImageIoDataVolumeNoExtents := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := createImageIoDataVolume(dataVolumeName, size, url)
		CreateImageIoInventoryNoExtents(f)
		return dataVolume
	}

	createImageIoWarmImportDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		stringData := map[string]string{
			common.KeyAccess: "admin@internal",
			common.KeySecret: "123456",
		}
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		diskID := "disk-678"
		snapshots := []string{
			"cirros.raw",
			"cirros-snapshot1.qcow2",
			"cirros-snapshot2.qcow2",
		}
		CreateImageIoWarmImportInventory(f, diskID, "storagedomain-876", snapshots)
		var checkpoints []cdiv1.DataVolumeCheckpoint
		parent := ""
		for _, checkpoint := range snapshots {
			checkpoints = append(checkpoints, cdiv1.DataVolumeCheckpoint{Current: checkpoint, Previous: parent})
			parent = checkpoint
		}
		return utils.NewDataVolumeWithImageioWarmImport(dataVolumeName, size, url, s.Name, cm, diskID, checkpoints, true)
	}

	AfterEach(func() {
		if sourcePvc != nil {
			By("[AfterEach] Clean up sourcePvc PVC")
			err := f.DeletePVC(sourcePvc)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
			sourcePvc = nil
		}
		if targetPvc != nil {
			By("[AfterEach] Clean up targetPvc PVC")
			err := f.DeletePVC(targetPvc)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Err: %s\n", err)
			}
			targetPvc = nil
		}
	})

	Describe("Verify DataVolume", func() {
		type dataVolumeTestArguments struct {
			name                         string
			size                         string
			url                          func() string
			dvFunc                       func(string, string, string) *cdiv1.DataVolume
			errorMessage                 string
			errorMessageFunc             func() string
			eventReason                  string
			phase                        cdiv1.DataVolumePhase
			repeat                       int
			checkPermissions             bool
			readyCondition               *cdiv1.DataVolumeCondition
			boundCondition               *cdiv1.DataVolumeCondition
			boundConditionWithPopulators *cdiv1.DataVolumeCondition
			runningCondition             *cdiv1.DataVolumeCondition
		}

		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
			dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			return dataVolume
		}

		createHTTPSDataVolumeWeirdCertFilename := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CreateCertConfigMapWeirdFilename(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
			dataVolume.Spec.Source.HTTP.CertConfigMap = cm
			return dataVolume
		}

		createHTTPExtraHeaders := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			credentials := fmt.Sprintf("%s:%s", utils.AccessKeyValue, utils.SecretKeyValue)
			credentials = base64.StdEncoding.EncodeToString([]byte(credentials))
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			dataVolume.Spec.Source.HTTP.ExtraHeaders = []string{
				fmt.Sprintf("Authorization: Basic %s", credentials),
			}
			return dataVolume
		}

		createHTTPSecretExtraHeaders := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			credentials := fmt.Sprintf("%s:%s", utils.AccessKeyValue, utils.SecretKeyValue)
			credentials = base64.StdEncoding.EncodeToString([]byte(credentials))
			secretRef := "secretheaders"
			stringData := map[string]string{
				"secret": fmt.Sprintf("Authorization: Basic %s", credentials),
			}

			secret, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, secretRef))
			Expect(err).ToNot(HaveOccurred())

			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			dataVolume.Spec.Source.HTTP.SecretExtraHeaders = []string{secret.Name}
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
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
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

				waitForDvPhase(args.phase, dataVolume, f)

				// verify PVC was created
				By("verifying pvc was created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Verifying the DV has the correct conditions and messages for those conditions")
				usePopulator, err := dvc.CheckPVCUsingPopulators(pvc)
				Expect(err).ToNot(HaveOccurred())
				if usePopulator && args.boundConditionWithPopulators != nil {
					utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, args.readyCondition, args.runningCondition, args.boundConditionWithPopulators)
				} else {
					utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, args.readyCondition, args.runningCondition, args.boundCondition)
				}

				By("Verifying event occurred")
				Eventually(func() bool {
					// Only find DV events, we know the PVC gets the same events
					events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
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
					return k8serrors.IsNotFound(err)
				}, timeout, pollingInterval).Should(BeTrue())
			}
		}

		testDataVolumeWithQuota := func(args dataVolumeTestArguments) {
			By("Configure namespace quota")
			err := f.CreateStorageQuota(int64(2), int64(512*1024*1024))
			Expect(err).ToNot(HaveOccurred())

			// Have to call the function in here, to make sure the BeforeEach in the Framework has run.
			dataVolume := args.dvFunc(args.name, args.size, args.url())
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("Verify Quota was exceeded in events and dv conditions")
			expectedPhase := cdiv1.Pending
			boundCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeBound,
				Status:  v1.ConditionUnknown,
				Message: "exceeded quota",
				Reason:  controller.ErrExceededQuota,
			}
			readyCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeReady,
				Status:  v1.ConditionFalse,
				Message: "exceeded quota",
				Reason:  controller.ErrExceededQuota,
			}
			// for smart clone dv the dv can't be updated, only events will show the quota reason
			if args.name == "dv-clone-test" && f.IsSnapshotStorageClassAvailable() {
				expectedPhase = cdiv1.SnapshotForSmartCloneInProgress
				boundCondition.Message = "in progress"
				boundCondition.Reason = dvc.SnapshotForSmartCloneInProgress
				readyCondition.Message = "in progress"
				readyCondition.Reason = dvc.SnapshotForSmartCloneInProgress
			}
			waitForDvPhase(expectedPhase, dataVolume, f)
			f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(controller.ErrExceededQuota))
			utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, boundCondition, readyCondition)

			By("Increase quota")
			err = f.UpdateStorageQuota(int64(4), int64(4*1024*1024*1024))
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			waitForDvPhase(args.phase, dataVolume, f)

			// verify PVC was created
			By("verifying pvc was created")
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying the DV has the correct conditions and messages for those conditions")
			usePopulator, err := dvc.CheckPVCUsingPopulators(pvc)
			Expect(err).ToNot(HaveOccurred())
			if usePopulator && args.boundConditionWithPopulators != nil {
				utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, args.readyCondition, args.runningCondition, args.boundConditionWithPopulators)
			} else {
				utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, args.readyCondition, args.runningCondition, args.boundCondition)
			}

			By("Verifying event occurred")
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, args.eventReason) && strings.Contains(events, args.errorMessage)
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			By("Cleaning up")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
			}, timeout, pollingInterval).Should(BeTrue())

			err = f.DeleteStorageQuota()
			Expect(err).ToNot(HaveOccurred())
		}

		table.DescribeTable("should", testDataVolume,
			table.Entry("[rfe_id:1115][crit:high][test_id:1357]succeed creating import dv with given valid url", dataVolumeTestArguments{
				name:             "dv-http-import",
				size:             "1Gi",
				url:              tinyCoreIsoURL,
				dvFunc:           utils.NewDataVolumeWithHTTPImport,
				eventReason:      dvc.ImportSucceeded,
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC dv-http-import-invalid-url Pending",
					Reason:  "Pending",
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC dv-http-import-404 Pending",
					Reason:  "Pending",
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC dv-invalid-qcow-large-memory Pending",
					Reason:  "Pending",
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
				eventReason:      dvc.ImportSucceeded,
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
				eventReason:      dvc.ImportSucceeded,
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
				eventReason:      dvc.ImportSucceeded,
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
			table.Entry("[rfe_id:1115][crit:high][test_id:1379]succeed creating import dv with given valid zst url (https)", dataVolumeTestArguments{
				name:             "dv-https-import-zst",
				size:             "1Gi",
				url:              httpsTinyCoreZstURL,
				dvFunc:           createHTTPSDataVolume,
				eventReason:      dvc.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-https-import-zst Bound",
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
				eventReason:      dvc.ImportSucceeded,
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
			table.Entry("[rfe_id:7202][crit:high][posneg:positive][test_id:8277]succeed creating import dv with custom https headers", dataVolumeTestArguments{
				name:             "dv-http-import-headers",
				size:             "1Gi",
				url:              tinyCoreIsoAuthURL,
				dvFunc:           createHTTPExtraHeaders,
				eventReason:      dvc.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-import-headers Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:7202][crit:high][posneg:positive][test_id:8278]succeed creating import dv with custom https headers from a secret", dataVolumeTestArguments{
				name:             "dv-http-import-headers",
				size:             "1Gi",
				url:              tinyCoreIsoAuthURL,
				dvFunc:           createHTTPSecretExtraHeaders,
				eventReason:      dvc.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-http-import-headers Bound",
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
				eventReason:      dvc.ImportSucceeded,
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
				eventReason: dvc.UploadReady,
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC upload-dv Pending",
					Reason:  "Pending",
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
				eventReason: dvc.ImportSucceeded,
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
				errorMessage: "Unable to process data",
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC dv-non-tar-archive Pending",
					Reason:  "Pending",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Unable to process data",
					Reason:  "Error",
				}}),
			table.Entry("[test_id:3932]succeed creating dv from imageio source", dataVolumeTestArguments{
				name:             "dv-imageio-test",
				size:             "1Gi",
				url:              imageioURL,
				dvFunc:           createImageIoDataVolume,
				eventReason:      dvc.ImportSucceeded,
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
			table.PEntry("[quarantine][test_id:3937]succeed creating warm import dv from imageio source", dataVolumeTestArguments{
				// The final snapshot importer pod will give an error due to the static response from the fake imageio
				// it returns the previous snapshot data, which will fail the commit to the target image.
				// the importer pod will restart and then succeed because the fake imageio now sends the
				// right data. This is normal.
				name:             "dv-imageio-test",
				size:             "1Gi",
				url:              imageioURL,
				dvFunc:           createImageIoWarmImportDataVolume,
				eventReason:      dvc.ImportSucceeded,
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
			table.Entry("[test_id:3945]succeed creating dv from imageio source that does not support extents query", dataVolumeTestArguments{
				name:             "dv-imageio-test",
				size:             "1Gi",
				url:              imageioURL,
				dvFunc:           createImageIoDataVolumeNoExtents,
				eventReason:      dvc.ImportSucceeded,
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
				eventReason: dvc.CloneSucceeded,
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
				eventReason:      dvc.ImportSucceeded,
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
				eventReason:      dvc.ImportSucceeded,
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
				eventReason:      dvc.ImportSucceeded,
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
			table.PEntry("[quarantine][test_id:5078]succeed creating warm import dv from VDDK source", dataVolumeTestArguments{
				name:             "dv-import-vddk",
				size:             "1Gi",
				url:              vcenterURL,
				dvFunc:           createVddkWarmImportDataVolume,
				eventReason:      dvc.ImportSucceeded,
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
			table.Entry("[rfe_id:XXXX][crit:high][test_id:XXXX]succeed creating import dv from GCS URL using RAW image", dataVolumeTestArguments{
				name:             "dv-gcs-raw-import",
				size:             "1Gi",
				url:              cirrosGCSRAWURL,
				dvFunc:           utils.NewDataVolumeWithGCSImport,
				eventReason:      dvc.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-gcs-raw-import Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
			table.Entry("[rfe_id:XXXX][crit:high][test_id:XXXX]succeed creating import dv from GCS URL using QCOW2 image", dataVolumeTestArguments{
				name:             "dv-gcs-qcow-import",
				size:             "1Gi",
				url:              cirrosGCSQCOWURL,
				dvFunc:           utils.NewDataVolumeWithGCSImport,
				eventReason:      dvc.ImportSucceeded,
				phase:            cdiv1.Succeeded,
				checkPermissions: true,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-gcs-qcow-import Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Import Complete",
					Reason:  "Completed",
				}}),
		)

		savedVddkConfigMap := common.VddkConfigMap + "-saved"

		createVddkDataVolumeWithInitImageURL := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dv := createVddkDataVolume(dataVolumeName, size, url)
			configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Get(context.TODO(), savedVddkConfigMap, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			vddkURL, ok := configMap.Data[common.VddkConfigDataKey]
			Expect(ok).To(BeTrue())
			dv.Spec.Source.VDDK.InitImageURL = vddkURL
			return dv
		}

		// Similar to previous table, but with additional cleanup steps to save and restore VDDK image config map
		table.DescribeTable("should", func(args dataVolumeTestArguments) {
			_, err := utils.CopyConfigMap(f.K8sClient, f.CdiInstallNs, common.VddkConfigMap, f.CdiInstallNs, savedVddkConfigMap, "")
			Expect(err).ToNot(HaveOccurred())

			err = utils.DeleteConfigMap(f.K8sClient, f.CdiInstallNs, common.VddkConfigMap)
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				_, err := utils.CopyConfigMap(f.K8sClient, f.CdiInstallNs, savedVddkConfigMap, f.CdiInstallNs, common.VddkConfigMap, "")
				Expect(err).ToNot(HaveOccurred())

				err = utils.DeleteConfigMap(f.K8sClient, f.CdiInstallNs, savedVddkConfigMap)
				Expect(err).ToNot(HaveOccurred())
			}()

			testDataVolume(args)
		},
			table.Entry("[test_id:5079]should fail with \"AwaitingVDDK\" reason when VDDK image config map is not present", dataVolumeTestArguments{
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
					Message: fmt.Sprintf("waiting for %s configmap or %s annotation for VDDK image", common.VddkConfigMap, controller.AnnVddkInitImageURL),
					Reason:  common.AwaitingVDDK,
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeRunning,
					Status: v1.ConditionFalse,
				}}),
			table.Entry("[test_id:5080]succeed importing VDDK data volume with init image URL set", dataVolumeTestArguments{
				name:             "dv-import-vddk",
				size:             "1Gi",
				url:              vcenterURL,
				dvFunc:           createVddkDataVolumeWithInitImageURL,
				eventReason:      dvc.ImportSucceeded,
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

		// similar to other tables but with check of quota
		table.DescribeTable("should fail create pvc in namespace with storge quota, then succeed once the quota is large enough", testDataVolumeWithQuota,
			table.Entry("[test_id:7737]when creating import dv with given valid url", dataVolumeTestArguments{
				name:        "dv-http-import",
				size:        "1Gi",
				url:         tinyCoreIsoURL,
				dvFunc:      utils.NewDataVolumeWithHTTPImport,
				eventReason: dvc.ImportSucceeded,
				phase:       cdiv1.Succeeded,
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
			table.Entry("[test_id:7738]when creating upload dv", dataVolumeTestArguments{
				name:        "upload-dv",
				size:        "1Gi",
				url:         func() string { return "" },
				dvFunc:      createUploadDataVolume,
				eventReason: dvc.UploadReady,
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
				boundConditionWithPopulators: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionFalse,
					Message: "PVC upload-dv Pending",
					Reason:  "Pending",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeRunning,
					Status: v1.ConditionTrue,
					Reason: "Pod is running",
				}}),
			table.Entry("[test_id:7739]when creating clone dv", dataVolumeTestArguments{
				name:        "dv-clone-test",
				size:        "500Mi",
				url:         func() string { return fillCommand }, // its not URL, but command, but the parameter lines up.
				dvFunc:      createCloneDataVolume,
				eventReason: dvc.CloneSucceeded,
				phase:       cdiv1.Succeeded,
				readyCondition: &cdiv1.DataVolumeCondition{
					Type:   cdiv1.DataVolumeReady,
					Status: v1.ConditionTrue,
				},
				boundCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeBound,
					Status:  v1.ConditionTrue,
					Message: "PVC dv-clone-test Bound",
					Reason:  "Bound",
				},
				runningCondition: &cdiv1.DataVolumeCondition{
					Type:    cdiv1.DataVolumeRunning,
					Status:  v1.ConditionFalse,
					Message: "Clone Complete",
					Reason:  "Completed",
				}}),
		)

		table.DescribeTable("should have an alert suppressing label on corresponding PVC", func(dvFunc func(string, string, string) *cdiv1.DataVolume, url string) {
			dataVolume := dvFunc("suppress-fillingup-alert-dv", "1Gi", url)

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created and is Bound")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			// Alert-suppressing label exists
			Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
		},
			table.Entry("[test_id:8043]for import DataVolume", utils.NewDataVolumeWithHTTPImport, tinyCoreIsoURL()),
			table.Entry("[test_id:8044]for upload DataVolume", createUploadDataVolume, tinyCoreIsoURL()),
			table.Entry("[test_id:8045]for clone DataVolume", createCloneDataVolume, fillCommand),
		)

		table.DescribeTable("Should pass all DV labels and annotations to the PVC", func(dvFunc func(string, string, string) *cdiv1.DataVolume, url string) {
			dataVolume := dvFunc("pass-all-labels-dv", "1Gi", url)
			dataVolume.Labels = map[string]string{"test-label-1": "test-label-1", "test-label-2": "test-label-2"}
			dataVolume.Annotations = map[string]string{"test-annotation-1": "test-annotation-1", "test-annotation-2": "test-annotation-2"}

			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			// verify PVC was created
			By("verifying pvc was created and is Bound")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			// All labels and annotations passed
			Expect(pvc.Labels["test-label-1"]).To(Equal("test-label-1"))
			Expect(pvc.Labels["test-label-2"]).To(Equal("test-label-2"))
			Expect(pvc.Annotations["test-annotation-1"]).To(Equal("test-annotation-1"))
			Expect(pvc.Annotations["test-annotation-2"]).To(Equal("test-annotation-2"))
		},
			table.Entry("for import DataVolume", utils.NewDataVolumeWithHTTPImport, tinyCoreIsoURL()),
			table.Entry("for upload DataVolume", createUploadDataVolume, tinyCoreIsoURL()),
			table.Entry("for clone DataVolume", createCloneDataVolume, fillCommand),
		)

		It("Should handle a pre populated DV", func() {
			By(fmt.Sprintf("initializing dataVolume marked as prePopulated %s", dataVolumeName))
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", cirrosURL())
			dataVolume.Annotations["cdi.kubevirt.io/storage.prePopulated"] = dataVolumeName

			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying Pending without PVC")
			Eventually(func() cdiv1.DataVolumePhase {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return dv.Status.Phase
			}, timeout, pollingInterval).Should(Equal(cdiv1.Pending))

			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(k8serrors.IsNotFound(err)).To(BeTrue(), "PVC Should not exist")

			By("Create PVC")
			annotations := map[string]string{"cdi.kubevirt.io/storage.populatedFor": dataVolumeName}
			pvc := utils.NewPVCDefinition(dataVolumeName, "100m", annotations, nil)
			pvc = f.CreateBoundPVCFromDefinition(pvc)

			By("Verifying Succeed with PVC Bound")
			err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if controller.GetDataVolumeTTLSeconds(config) >= 0 {
				return
			}

			By("Verifying PVC owned by DV")
			Eventually(func() bool {
				pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcOwner := metav1.GetControllerOf(pvc)
				return pvcOwner != nil && pvcOwner.Kind == "DataVolume" && pvcOwner.Name == dataVolume.Name
			}, timeout, pollingInterval).Should(BeTrue())
		})

		It("[test_id:4961] should handle a pre populated PVC for import DV", func() {
			By(fmt.Sprintf("initializing target PVC %s", dataVolumeName))
			targetPodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			annotations := map[string]string{controller.AnnPopulatedFor: dataVolumeName}
			targetPvcDef := utils.NewPVCDefinition(dataVolumeName, "1G", annotations, nil)
			targetPvc = f.CreateAndPopulateSourcePVC(targetPvcDef, targetPodFillerName, fillCommand)

			By(fmt.Sprintf("creating new populated datavolume %s", dataVolumeName))
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", cirrosURL())
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcName := dv.Annotations["cdi.kubevirt.io/storage.prePopulated"]
				return pvcName == targetPvcDef.Name &&
					dv.Status.Phase == cdiv1.Succeeded &&
					string(dv.Status.Progress) == "N/A"
			}, timeout, pollingInterval).Should(BeTrue())

			By("Verify no import - the contents of prepopulated volume did not change")
			md5Match, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, testFile, fillDataFSMD5sum)
			Expect(err).ToNot(HaveOccurred())
			Expect(md5Match).To(BeTrue())
		})

		It("should handle a pre populated PVC for upload DV", func() {
			By(fmt.Sprintf("initializing target PVC %s", dataVolumeName))
			targetPodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
			annotations := map[string]string{controller.AnnPopulatedFor: dataVolumeName}
			targetPvcDef := utils.NewPVCDefinition(dataVolumeName, "1G", annotations, nil)
			targetPvc = f.CreateAndPopulateSourcePVC(targetPvcDef, targetPodFillerName, fillCommand)

			By(fmt.Sprintf("creating new populated datavolume %s", dataVolumeName))
			dataVolume := utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcName := dv.Annotations["cdi.kubevirt.io/storage.prePopulated"]
				return pvcName == targetPvcDef.Name &&
					dv.Status.Phase == cdiv1.Succeeded &&
					string(dv.Status.Progress) == "N/A"
			}, timeout, pollingInterval).Should(BeTrue())

			By("Verify no upload - the contents of prepopulated volume did not change")
			md5Match, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, testFile, fillDataFSMD5sum)
			Expect(err).ToNot(HaveOccurred())
			Expect(md5Match).To(BeTrue())
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
				err := utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
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
				utils.ModifyDataVolumeWithImportToBlockPV(dataVolume, f.BlockSCName)
			case "warm-import-vddk":
				dataVolume = createVddkWarmImportDataVolume(dataVolumeName, "1Gi", vcenterURL())
				utils.ModifyDataVolumeWithImportToBlockPV(dataVolume, f.BlockSCName)
			case "import-imageio":
				dataVolume = createImageIoDataVolume(dataVolumeName, "1Gi", imageioURL())
				utils.ModifyDataVolumeWithImportToBlockPV(dataVolume, f.BlockSCName)
			case "warm-import-imageio":
				dataVolume = createImageIoWarmImportDataVolume(dataVolumeName, "1Gi", imageioURL())
				utils.ModifyDataVolumeWithImportToBlockPV(dataVolume, f.BlockSCName)
			}
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				f.PrintControllerLog()
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

			By("Verifying event occurred")
			Eventually(func() bool {
				events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace)
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, eventReason)
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		},
			table.Entry("[test_id:3933]succeed creating import dv with given valid url", "import-http", "", tinyCoreIsoURL, "dv-phase-test-1", dvc.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[test_id:3935]succeed import from VDDK to block volume", "import-vddk", "", nil, "dv-vddk-import-test", dvc.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[test_id:3936]succeed warm import from VDDK to block volume", "warm-import-vddk", "", nil, "dv-vddk-warm-import-test", dvc.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[test_id:3938]succeed import from ImageIO to block volume", "import-imageio", "", nil, "dv-imageio-import-test", dvc.ImportSucceeded, cdiv1.Succeeded),
			table.Entry("[test_id:3944]succeed warm import from ImageIO to block volume", "warm-import-imageio", "", nil, "dv-imageio-warm-import-test", dvc.ImportSucceeded, cdiv1.Succeeded),
		)
	})

	table.DescribeTable("Succeed HTTPS import in various formats", func(url func() string, skipOnOpenshift bool) {
		if skipOnOpenshift && utils.IsOpenshift(f.K8sClient) {
			Skip("This test doesn't work when building using centos, see: https://bugzilla.redhat.com/show_bug.cgi?id=2013331")
		}
		By(fmt.Sprintf("Importing from %s", url()))
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", url())
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		dataVolume.Spec.Source.HTTP.CertConfigMap = cm

		By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		pvc, err := utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))
	},
		table.Entry("when importing in the VMDK format", httpsTinyCoreVmdkURL, false),
		table.Entry("When importing in the VDI format", httpsTinyCoreVdiURL, true),
		table.Entry("when importing in the VHD format", httpsTinyCoreVhdURL, false),
		table.Entry("when importing in the VHDX format", httpsTinyCoreVhdxURL, false),
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
				Expect(utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)).To(Succeed())

				By("verifying pvc and pod were created")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				usePopulator, err := dvc.CheckPVCUsingPopulators(pvc)
				Expect(err).ToNot(HaveOccurred())
				podName := pvc.Annotations[controller.AnnImportPod]
				if usePopulator {
					pvcPrimeName := populators.PVCPrimeName(pvc)
					pvcPrime, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), pvcPrimeName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					podName = pvcPrime.Annotations[controller.AnnImportPod]
				}

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
					dv.Annotations[controller.AnnDeleteAfterCompletion] = "false"

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
					err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
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
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
			By(fmt.Sprintf("creating new datavolume %s with annotations", dataVolume.Name))
			dataVolume.Annotations[controller.AnnPodNetwork] = "net1"
			dataVolume.Annotations["annot1"] = "value1"
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("find importer pod")
			var sourcePod *v1.Pod
			Eventually(func() bool {
				sourcePod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
				return err == nil
			}, timeout, pollingInterval).Should(BeTrue())
			By(fmt.Sprintf("Verifying pod %s has correct annotation", sourcePod.Name))
			verifyAnnotations(sourcePod)
		})

		It("[test_id:5365]Uploader pod should have specific datavolume annotations passed but not others", func() {
			dataVolume := utils.NewDataVolumeForUpload(dataVolumeName, "1Gi")
			By(fmt.Sprintf("creating new datavolume %s with annotations", dataVolume.Name))
			dataVolume.Annotations[controller.AnnPodNetwork] = "net1"
			dataVolume.Annotations["annot1"] = "value1"
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("find uploader pod")
			var sourcePod *v1.Pod
			Eventually(func() bool {
				sourcePod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.UploadPodName, common.CDILabelSelector)
				return err == nil
			}, timeout, pollingInterval).Should(BeTrue())
			verifyAnnotations(sourcePod)
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
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			var sourcePod *v1.Pod
			var uploadPod *v1.Pod
			Eventually(func() bool {
				uploadPod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.UploadPodName, common.CDILabelSelector)
				return err == nil
			}, timeout, pollingInterval).Should(BeTrue())
			verifyAnnotations(uploadPod)
			// Remove non existent network so upload pod succeeds and clone can continue (some envs like OpenShift check network validity)
			Eventually(func() bool {
				uploadPod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, "cdi-upload", common.CDILabelSelector)
				if err != nil {
					return false
				}
				delete(uploadPod.Annotations, controller.AnnPodNetwork)
				_, err = f.K8sClient.CoreV1().Pods(dataVolume.Namespace).Update(context.TODO(), uploadPod, metav1.UpdateOptions{})
				return err == nil
			}, 60*time.Second, 2*time.Second).Should(BeTrue())
			Eventually(func() bool {
				sourcePod, err = utils.FindPodBySuffix(f.K8sClient, dataVolume.Namespace, "source-pod", common.CDILabelSelector)
				return err == nil
			}, timeout, pollingInterval).Should(BeTrue())
			verifyAnnotations(sourcePod)
		})
	})

	Describe("Create a PVC using data from StorageProfile", func() {
		var (
			config              *cdiv1.CDIConfig
			origSpec            *cdiv1.CDIConfigSpec
			originalProfileSpec *cdiv1.StorageProfileSpec
			defaultSc           *storagev1.StorageClass
			defaultScName       string
			err                 error
		)

		fillData := "123456789012345678901234567890123456789012345678901234567890"
		testFile := utils.DefaultPvcMountPath + "/source.txt"
		fillCommand := "echo \"" + fillData + "\" >> " + testFile

		createDataVolumeForImport := func(f *framework.Framework, storageSpec cdiv1.StorageSpec) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(
				dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))

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
			Eventually(func() error {
				err := client.Get(context.TODO(), types.NamespacedName{Name: name}, storageProfile)
				Expect(err).ToNot(HaveOccurred())
				storageProfile.Spec = spec
				return client.Update(context.TODO(), storageProfile)
			}, 15*time.Second, time.Second).Should(BeNil())
		}

		configureStorageProfile := func(client client.Client,
			storageClassName string,
			accessModes []v1.PersistentVolumeAccessMode,
			volumeMode v1.PersistentVolumeMode) {

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
			defaultScName = utils.DefaultStorageClass.GetName()
			utils.DefaultStorageClass, err = f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), defaultScName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			defaultSc = utils.DefaultStorageClass

			config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			origSpec = config.Spec.DeepCopy()

			originalProfileSpec = getStorageProfileSpec(f.CrClient, defaultScName)
		})

		AfterEach(func() {
			if defaultSc.Annotations[controller.AnnDefaultStorageClass] != "true" {
				By("Restoring default storage class")
				defaultSc.Annotations[controller.AnnDefaultStorageClass] = "true"
				utils.DefaultStorageClass, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), defaultSc, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			if originalProfileSpec != nil {
				By("Restoring the StorageProfile to original state")
				updateStorageProfileSpec(f.CrClient, defaultScName, *originalProfileSpec)
			}

			By("Restoring CDIConfig to original state")
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				origSpec.DeepCopyInto(config)
			})

			Eventually(func() bool {
				config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return reflect.DeepEqual(config.Spec, *origSpec)
			}, 30*time.Second, time.Second).Should(BeTrue())
		})

		It("[test_id:5911]Import succeeds creating a PVC from DV without accessModes and storageClass name", func() {
			By(fmt.Sprintf("configure storage profile %s", defaultScName))
			configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)
			requestedSize := resource.MustParse("100Mi")

			spec := cdiv1.StorageSpec{
				AccessModes: nil,
				VolumeMode:  nil,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}

			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc created with correct accessModes and storageclass name")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
			Expect(*pvc.Spec.StorageClassName).To(SatisfyAll(Not(BeNil()), Equal(defaultScName)))
		})

		It("[test_id:8170]Import succeeds when storage class is not specified, but access mode is", func() {
			By(fmt.Sprintf("configure storage profile %s", defaultScName))
			configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)
			requestedSize := resource.MustParse("100Mi")

			expectedMode := []v1.PersistentVolumeAccessMode{v1.ReadWriteMany}
			spec := cdiv1.StorageSpec{
				AccessModes: expectedMode,
				VolumeMode:  nil,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}

			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc created with correct accessModes and storageclass name")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal(expectedMode))
			Expect(*pvc.Spec.StorageClassName).To(SatisfyAll(Not(BeNil()), Equal(defaultScName)))
		})

		It("[test_id:8169]Import succeeds when storage class is not specified, but volume mode is", func() {
			By(fmt.Sprintf("configure storage profile %s", defaultScName))
			configureStorageProfile(f.CrClient,
				defaultScName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)
			requestedSize := resource.MustParse("100Mi")

			expectedVolumeMode := v1.PersistentVolumeBlock
			spec := cdiv1.StorageSpec{
				AccessModes: nil,
				VolumeMode:  &expectedVolumeMode,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}

			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying pvc created with correct accessModes and storageclass name")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))
			Expect(*pvc.Spec.VolumeMode).To(Equal(expectedVolumeMode))
			Expect(*pvc.Spec.StorageClassName).To(SatisfyAll(Not(BeNil()), Equal(defaultScName)))
		})

		It("[test_id:5912]Import fails creating a PVC from DV without accessModes and volume mode, no profile", func() {
			// assumes local is available and has no volumeMode
			storageProfileName := findStorageProfileWithoutAccessModes(f.CrClient)
			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			requestedSize := resource.MustParse("100Mi")
			spec := cdiv1.StorageSpec{
				AccessModes:      nil,
				VolumeMode:       nil,
				StorageClassName: &storageProfileName,
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

			By("verifying event occurred")
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, controller.ErrClaimNotValid) && strings.Contains(events, "DataVolume.storage spec is missing accessMode and volumeMode, cannot get access mode from StorageProfile")
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		})

		It("[test_id:8383]Import fails when no default storage class, and recovers when default is set", func() {
			By("updating to no default storage class")
			defaultSc.Annotations[controller.AnnDefaultStorageClass] = "false"
			defaultSc, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), defaultSc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			requestedSize := resource.MustParse("100Mi")
			spec := cdiv1.StorageSpec{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: requestedSize,
					},
				},
			}
			dataVolume := createDataVolumeForImport(f, spec)

			By("verifying event occurred")
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, controller.ErrClaimNotValid) && strings.Contains(events, "DataVolume.storage spec is missing accessMode and no storageClass to choose profile")
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			By("verifying pvc not created")
			_, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			By("restoring default storage class")
			defaultSc.Annotations[controller.AnnDefaultStorageClass] = "true"
			defaultSc, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), defaultSc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc created")
			_, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:5913]Import recovers when user adds accessModes to profile", func() {
			// assumes local is available and has no volumeMode
			storageProfileName := findStorageProfileWithoutAccessModes(f.CrClient)
			By(fmt.Sprintf("creating new datavolume %s without accessModes", dataVolumeName))
			requestedSize := resource.MustParse("100Mi")
			spec := cdiv1.StorageSpec{
				AccessModes:      nil,
				VolumeMode:       nil,
				StorageClassName: &storageProfileName,
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

			By("verifying event occurred")
			Eventually(func() bool {
				// Only find DV events, we know the PVC gets the same events
				events, err := f.RunKubectlCommand("get", "events", "-n", dataVolume.Namespace, "--field-selector=involvedObject.kind=DataVolume")
				if err == nil {
					fmt.Fprintf(GinkgoWriter, "%s", events)
					return strings.Contains(events, controller.ErrClaimNotValid) && strings.Contains(events, "DataVolume.storage spec is missing accessMode and volumeMode, cannot get access mode from StorageProfile")
				}
				fmt.Fprintf(GinkgoWriter, "ERROR: %s\n", err.Error())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			By(fmt.Sprintf("configure storage profile %s", storageProfileName))
			originalProfileSpec := getStorageProfileSpec(f.CrClient, storageProfileName)
			configureStorageProfile(f.CrClient,
				storageProfileName,
				[]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				v1.PersistentVolumeFilesystem)

			By("verifying pvc created with correct accessModes")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.AccessModes).To(Equal([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}))

			By("Restore the profile")
			updateStorageProfileSpec(f.CrClient, storageProfileName, *originalProfileSpec)
		})

		It("[test_id:6483]Import pod should not have size corrected on block", func() {
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
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			configureStorageProfile(f.CrClient,
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
		})

		It("[test_id:6485]Import pod should have size corrected on filesystem", func() {
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
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// given 50 percent overhead, expected size is 2x requestedSize
			expectedSize := resource.MustParse("200Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeFilesystem
			dataVolume := createDataVolumeForUpload(f, cdiv1.StorageSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				VolumeMode:  &volumeMode,
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
			Expect(*pvc.Spec.StorageClassName).To(SatisfyAll(Not(BeNil()), Equal(defaultScName)))
		})

		It("[test_id:6100]Upload pvc should not have size corrected on block volume", func() {
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
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			configureStorageProfile(f.CrClient,
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
		})

		It("[test_id:6101]Clone pod should not have size corrected on block", func() {
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By("creating datavolume for upload")
			volumeMode := v1.PersistentVolumeBlock
			dataVolume := createCloneDataVolume(dataVolumeName,
				cdiv1.StorageSpec{
					AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					VolumeMode:  &volumeMode,
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
			Expect(*pvc.Spec.StorageClassName).To(SatisfyAll(Not(BeNil()), Equal(defaultScName)))
		})

		It("[test_id:6487]Clone pod should not have size corrected on block, when no volumeMode on DV", func() {
			SetFilesystemOverhead(f, "0.50", "0.50")
			requestedSize := resource.MustParse("100Mi")
			// volumeMode Block, so no overhead applied
			expectedSize := resource.MustParse("100Mi")

			By(fmt.Sprintf("configure storage profile %s to volumeModeBlock", defaultScName))
			configureStorageProfile(f.CrClient,
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
		})

		It("[test_id:6102]Clone pod should have size corrected on filesystem", func() {
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

	Describe("Verify that when the required storage class is missing", func() {
		var (
			testSc *storagev1.StorageClass
			pvName string
		)

		testScName := "test-sc"

		updatePV := func(updateFunc func(*v1.PersistentVolume)) {
			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			updateFunc(pv)
			_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		createPV := func(scName string) {
			dv := utils.NewDataVolumeForBlankRawImage("blank-source", "106Mi")
			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)

			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			pvName = pvc.Spec.VolumeName

			By("retaining pv")
			updatePV(func(pv *v1.PersistentVolume) {
				pv.Spec.PersistentVolumeReclaimPolicy = v1.PersistentVolumeReclaimRetain
			})

			err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			updatePV(func(pv *v1.PersistentVolume) {
				pv.Spec.StorageClassName = scName
				pv.Spec.ClaimRef = nil
			})
		}

		createStorageClass := func(scName string) {
			var err error
			By(fmt.Sprintf("creating storage class %s", scName))
			sc := utils.DefaultStorageClass.DeepCopy()
			sc.Name = scName
			sc.ResourceVersion = ""
			sc.Annotations[controller.AnnDefaultStorageClass] = "false"
			testSc, err = f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), sc, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		AfterEach(func() {
			if testSc != nil {
				err := f.K8sClient.StorageV1().StorageClasses().Delete(context.TODO(), testScName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				testSc = nil
			}

			if pvName != "" {
				updatePV(func(pv *v1.PersistentVolume) {
					pv.Spec.PersistentVolumeReclaimPolicy = v1.PersistentVolumeReclaimDelete
				})
				pvName = ""
			}
		})

		table.DescribeTable("import DV using StorageSpec without AccessModes, PVC is created only when", func(scName string, scFunc func(string)) {
			if utils.IsDefaultSCNoProvisioner() {
				Skip("Default storage class has no provisioner. The new storage class won't work")
			}

			By(fmt.Sprintf("verifying no storage class %s", testScName))
			_, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), scName, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())

			By(fmt.Sprintf("creating new datavolume %s with StorageClassName %s", dataVolumeName, scName))
			dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(
				dataVolumeName, "100Mi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
			dataVolume.Spec.Storage.StorageClassName = pointer.String(scName)
			dataVolume.Spec.Storage.AccessModes = nil

			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying event occurred")
			f.ExpectEvent(dataVolume.Namespace).Should(And(ContainSubstring(controller.ErrClaimNotValid), ContainSubstring("DataVolume spec is missing accessMode")))

			By("verifying conditions")
			boundCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeBound,
				Status:  v1.ConditionUnknown,
				Message: "DataVolume spec is missing accessMode",
				Reason:  controller.ErrClaimNotValid,
			}
			readyCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeReady,
				Status:  v1.ConditionFalse,
				Message: "DataVolume spec is missing accessMode",
				Reason:  controller.ErrClaimNotValid,
			}
			utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, boundCondition, readyCondition)

			By("verifying pvc not created")
			_, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			scFunc(scName)

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("waiting for pvc bound phase")
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, dataVolume.Namespace, v1.ClaimBound, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for dv succeeded")
			err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:9922]the storage class is created", testScName, createStorageClass),
			table.Entry("[test_id:9924]PV with the SC name is created", testScName, createPV),
			table.Entry("[test_id:9925]PV with the SC name (\"\" blank) is created", "", createPV),
		)

		newDataVolumeWithStorageSpec := func(scName string) *cdiv1.DataVolume {
			dv := utils.NewDataVolumeWithHTTPImportAndStorageSpec(
				dataVolumeName, "100Mi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
			dv.Spec.Storage.StorageClassName = pointer.String(scName)
			return dv
		}

		newDataVolumeWithPvcSpec := func(scName string) *cdiv1.DataVolume {
			dv := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "100Mi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
			dv.Spec.PVC.StorageClassName = pointer.String(scName)
			return dv
		}

		table.DescribeTable("import DV with AccessModes, PVC is pending until", func(scName string, scFunc func(string), dvFunc func(string) *cdiv1.DataVolume) {
			if utils.IsDefaultSCNoProvisioner() {
				Skip("Default storage class has no provisioner. The new storage class won't work")
			}

			By(fmt.Sprintf("verifying no storage class %s", testScName))
			_, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), scName, metav1.GetOptions{})
			Expect(err).To(HaveOccurred())

			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvFunc(scName))
			Expect(err).ToNot(HaveOccurred())

			By("verifying event occurred")
			f.ExpectEvent(dataVolume.Namespace).Should(And(ContainSubstring("Pending"), ContainSubstring("PVC test-dv Pending")))

			By("verifying conditions")
			boundCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeBound,
				Status:  v1.ConditionFalse,
				Message: "PVC test-dv Pending",
				Reason:  "Pending",
			}
			readyCondition := &cdiv1.DataVolumeCondition{
				Type:   cdiv1.DataVolumeReady,
				Status: v1.ConditionFalse,
			}
			utils.WaitForConditions(f, dataVolume.Name, f.Namespace.Name, timeout, pollingInterval, boundCondition, readyCondition)

			By("verifying pvc created")
			_, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			scFunc(scName)

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("waiting for pvc bound phase")
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, dataVolume.Namespace, v1.ClaimBound, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for dv succeeded")
			err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
		},
			table.Entry("[test_id:9926]the storage class is created (PvcSpec)", testScName, createStorageClass, newDataVolumeWithPvcSpec),
			table.Entry("[test_id:9927]PV with the SC name is created (PvcSpec)", testScName, createPV, newDataVolumeWithPvcSpec),
			table.Entry("[test_id:9928]PV with the SC name (\"\" blank) is created (PvcSpec)", "", createPV, newDataVolumeWithPvcSpec),
			table.Entry("[test_id:9929]the storage class is created (StorageSpec)", testScName, createStorageClass, newDataVolumeWithStorageSpec),
			table.Entry("[test_id:9930]PV with the SC name is created (StorageSpec)", testScName, createPV, newDataVolumeWithStorageSpec),
			table.Entry("[test_id:9931]PV with the SC name (\"\" blank) is created (StorageSpec)", "", createPV, newDataVolumeWithStorageSpec),
		)
	})

	Describe("Progress reporting on import datavolume", func() {
		table.DescribeTable("Should report progress while importing", func(url string) {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(url, f.CdiInstallNs))
			dataVolume.Annotations[controller.AnnDeleteAfterCompletion] = "false"
			By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)
			if err != nil {
				f.PrintControllerLog()
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile(`\d{1,3}\.?\d{1,2}%`)
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())
		},
			table.Entry("[test_id:3934]when image is qcow2", utils.TinyCoreQcow2URLRateLimit),
			table.Entry("[test_id:6902]when image is qcow2.gz", utils.TinyCoreQcow2GzURLRateLimit),
		)
	})

	Describe("[rfe_id:4223][crit:high] DataVolume - WaitForFirstConsumer", func() {
		createBlankRawDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage(dataVolumeName, size)
		}
		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
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
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			utils.CleanupDvPvc(f.K8sClient, f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
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
		createBlankRawDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage(dataVolumeName, size)
		}
		createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
			cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
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
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
			if err != nil {
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if dverr != nil {
					Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
				}
			}
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			utils.CleanupDvPvc(f.K8sClient, f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
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
			Expect(err).ToNot(HaveOccurred())
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"cp /tmp/shared/images/"+originalImageName+" /tmp/shared/images/"+testImageName)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			By("Delete DV")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Cleanup the file")
			fileHostPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, utils.FileHostName, "name="+utils.FileHostName)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"rm -f /tmp/shared/images/"+testImageName)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying pvc was deleted")
			deleted, err := utils.WaitPVCDeleted(f.K8sClient, dataVolume.Name, dataVolume.Namespace, timeout)
			Expect(deleted).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:1990] CDI Data Volume - file is removed from http server while import is in progress", func() {
			dvName := "import-file-removed"
			By(fmt.Sprintf("Creating new datavolume %s", dvName))
			dv := utils.NewDataVolumeWithHTTPImport(dvName, "500Mi", tinyCoreIsoRateLimitURL())
			dv.Annotations[controller.AnnDeleteAfterCompletion] = "false"
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			phase := cdiv1.ImportInProgress
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			// here we want to have more than 0, to be sure it started
			progressRegExp := regexp.MustCompile(`[1-9]\d{0,2}\.?\d{1,2}%`)
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				fmt.Fprintf(GinkgoWriter, "INFO: current progress:%v, matches:%v\n", progress, progressRegExp.MatchString(string(progress)))
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())

			By("Remove source image file & kill http container to force restart")
			fileHostPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, utils.FileHostName, "name="+utils.FileHostName)
			Expect(err).ToNot(HaveOccurred())
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"rm /tmp/shared/images/"+testImageName)
			Expect(err).ToNot(HaveOccurred())

			By("Verify the number of retries on the datavolume")
			Eventually(func() int32 {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				restarts := dv.Status.RestartCount
				return restarts
			}, timeout, pollingInterval).Should(BeNumerically(">=", 1))

			By("Restore the file, import should progress")
			Expect(utils.WaitTimeoutForPodReady(f.K8sClient, fileHostPod.Name, fileHostPod.Namespace, utils.PodWaitForTime)).To(Succeed())
			_, _, err = f.ExecCommandInContainerWithFullOutput(fileHostPod.Namespace, fileHostPod.Name, "http",
				"/bin/sh",
				"-c",
				"cp /tmp/shared/images/"+originalImageName+" /tmp/shared/images/"+testImageName)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for the eventual success")
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())
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
			Expect(utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.ImportInProgress, dataVolume.Name)).To(Succeed())

			By(fmt.Sprintf("Deleting PVC %v (id: %v)", pvc.Name, pvcUID))
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc.Name)
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
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 10*time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Registry import with missing configmap", func() {
		cmName := "cert-registry-cm"

		It("[test_id:4963]Import POD should remain pending until CM exists", func() {
			var pvc *v1.PersistentVolumeClaim

			dataVolumeDef := utils.NewDataVolumeWithRegistryImport("missing-cm-registry-dv", "1Gi", tinyCoreIsoRegistryURL())
			dataVolumeDef.Spec.Source.Registry.CertConfigMap = &cmName
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
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
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
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
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
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
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
					sourcePod, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.UploadPodName, common.CDILabelSelector)
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
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
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
						sourcePod, _ = utils.FindPodBySuffix(f.K8sClient, dataVolume.Namespace, "source-pod", common.CDILabelSelector)
					}
					if uploadPod == nil {
						uploadPod, _ = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.UploadPodName, common.CDILabelSelector)
					}
					return sourcePod != nil && uploadPod != nil
				}, timeout, pollingInterval).Should(BeTrue())
				verifyPodAnnotations(sourcePod)
				verifyPodAnnotations(uploadPod)
			}
		})
	})
})

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
