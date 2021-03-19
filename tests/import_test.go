package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	namespacePrefix                  = "importer"
	assertionPollInterval            = 2 * time.Second
	controllerSkipPVCCompleteTimeout = 270 * time.Second
	CompletionTimeout                = 270 * time.Second
	BlankImageMD5                    = "cd573cfaace07e7949bc0c46028904ff"
	BlockDeviceMD5                   = "7c55761d39e6428fa27c21d8710a3d19"
)

var _ = Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component]Importer Test Suite", func() {
	var (
		ns string
		f  = framework.NewFramework(namespacePrefix)
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
	})

	DescribeTable("[test_id:2329] Should fail to import images that require too much space", func(uploadURL string) {
		imageURL := fmt.Sprintf(uploadURL, f.CdiInstallNs)

		By(imageURL)
		dv := utils.NewDataVolumeWithHTTPImport("too-large-import", "500Mi", imageURL)
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod"))

		By(fmt.Sprintf("logs for pod -n %s %s", importer.Name, importer.Namespace))
		By("Verify size error in logs")
		Eventually(func() bool {
			log, _ := tests.RunKubectlCommand(f, "logs", importer.Name, "-n", importer.Namespace)
			if strings.Contains(log, "is larger than available size") {
				return true
			}
			if strings.Contains(log, "no space left on device") {
				return true
			}
			if strings.Contains(log, "file largest block is bigger than maxblock") {
				return true
			}
			By("Failed to find error messages about a too large image in log:")
			By(log)
			return false
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	},
		Entry("fail given a large virtual size RAW XZ file", utils.LargeVirtualDiskXz),
		Entry("fail given a large virtual size QCOW2 file", utils.LargeVirtualDiskQcow),
		Entry("fail given a large physical size RAW XZ file", utils.LargePhysicalDiskXz),
		Entry("fail given a large physical size QCOW2 file", utils.LargePhysicalDiskQcow),
	)

	It("[test_id:4967]Should not perform CDI operations on PVC without annotations", func() {
		// Make sure the PVC name is unique, we have no guarantee on order and we are not
		// deleting the PVC at the end of the test, so if another runs first we will fail.
		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition("no-import-ann", "1G", nil, nil))
		By("Verifying PVC with no annotation remains empty")
		matchString := "PVC annotation not found, skipping pvc\t{\"PVC\": \"" + ns + "/" + pvc.Name + "\", \"annotation\": \"" + controller.AnnEndpoint + "\"}"
		fmt.Fprintf(GinkgoWriter, "INFO: matchString: [%s]\n", matchString)
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return log
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
		Expect(err).ToNot(HaveOccurred())
		// Wait a while to see if CDI puts anything in the PVC.
		isEmpty, err := framework.VerifyPVCIsEmpty(f, pvc, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(isEmpty).To(BeTrue())
		// Not deleting PVC as it will be removed with the NS removal.
	})

	It("[test_id:4969]Should create import pod for blank raw image", func() {
		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
			"create-image",
			"1Gi",
			map[string]string{controller.AnnSource: controller.SourceNone, controller.AnnContentType: string(cdiv1.DataVolumeKubeVirt)},
			nil))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Verify the image contents")
		Expect(f.VerifyBlankDisk(f.Namespace, pvc)).To(BeTrue())
		By("Verifying the image is sparse")
		Expect(f.VerifySparse(f.Namespace, pvc)).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")
		if utils.DefaultStorageCSI {
			// CSI storage class, it should respect fsGroup
			By("Checking that disk image group is qemu")
			Expect(f.GetDiskGroup(f.Namespace, pvc, false)).To(Equal("107"))
		}
	})
})

var _ = Describe("[rfe_id:4784][crit:high] Importer respects node placement", func() {
	var cr *cdiv1.CDI
	var oldSpec *cdiv1.CDISpec
	f := framework.NewFramework(namespacePrefix)

	// An image that fails import
	invalidQcowLargeSize := func() string {
		return fmt.Sprintf(utils.InvalidQcowImagesURL+"invalid-qcow-large-size.img", f.CdiInstallNs)
	}

	BeforeEach(func() {
		var err error
		cr, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
		}
		Expect(err).ToNot(HaveOccurred())

		oldSpec = cr.Spec.DeepCopy()
		nodes, err := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		Expect(nodes.Items).ToNot(BeEmpty(), "There should be some compute node")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cr.Spec = *oldSpec.DeepCopy()
		_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			cr, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(cr.Spec, oldSpec)
		}, 30*time.Second, time.Second)
	})

	It("[test_id:4783] Should create import pod with node placement", func() {
		cr.Spec.Workloads = tests.TestNodePlacementValues(f)
		_, err := f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})

		By("Waiting for CDI CR update to take effect")
		Eventually(func() bool {
			realCR, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(cr.Spec, realCR.Spec)
		}, 30*time.Second, time.Second)

		dv := utils.NewDataVolumeWithHTTPImport("node-placement-test", "100Mi", invalidQcowLargeSize())
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod"))

		By("Verify the import pod has nodeSelector")
		match := tests.PodSpecHasTestNodePlacementValues(f, importer.Spec)
		Expect(match).To(BeTrue(), fmt.Sprintf("node placement in pod spec\n%v\n differs from node placement values in CDI CR\n%v\n", importer.Spec, cr.Spec.Workloads))
	})
})

var _ = Describe("Importer CDI config manipulation tests", func() {
	var config *cdiv1.CDIConfig
	var origSpec *cdiv1.CDIConfigSpec
	var err error
	f := framework.NewFramework(namespacePrefix)

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

	DescribeTable("Filesystem overhead is honored with a RAW file", func(expectedSuccess bool, globalOverhead, scOverhead string) {
		setFilesystemOverhead(f, config, globalOverhead, scOverhead)

		imageURL := fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs)

		By(imageURL)
		dv := utils.NewDataVolumeWithHTTPImport("too-large-import", "500Mi", imageURL)
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod"))

		if expectedSuccess {
			By("Waiting for import to be completed")
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")
		} else {
			By(fmt.Sprintf("logs for pod -n %s %s", importer.Name, importer.Namespace))
			By("Verify size error in logs")
			Eventually(func() bool {
				log, _ := tests.RunKubectlCommand(f, "logs", importer.Name, "-n", importer.Namespace)
				if strings.Contains(log, "is larger than available size") {
					return true
				}
				if strings.Contains(log, "no space left on device") {
					return true
				}
				By("Failed to find error messages about a too large image in log:")
				By(log)
				return false
			}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
		}
	},
		Entry("Succeed with low global overhead", true, "0.1", ""),
		Entry("[posneg:negative] Fail with high global overhead", false, "0.99", ""),
		Entry("Succeed with low per-storageclass overhead (despite high global overhead)", true, "0.99", "0.1"),
		Entry("[posneg:negative] Fail with high per-storageclass overhead (despite low global overhead)", false, "0.1", "0.99"),
	)
})

var _ = Describe("[rfe_id:1118][crit:high][vendor:cnv-qe@redhat.com][level:component]Importer Test Suite-prometheus", func() {
	var prometheusURL string
	var portForwardCmd *exec.Cmd
	var err error
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	f := framework.NewFramework(namespacePrefix)

	BeforeEach(func() {
		_, err := f.CreatePrometheusServiceInNs(f.Namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error creating prometheus service")
	})

	AfterEach(func() {
		By("Stop port forwarding")
		if portForwardCmd != nil {
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}
	})

	It("[test_id:4970]Import pod should have prometheus stats available while importing", func() {
		var endpoint *v1.Endpoints
		c := f.K8sClient
		ns := f.Namespace.Name
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPRateLimitPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.qcow2",
			controller.AnnSecret:   "",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.qcow2"))
		pvc, err := utils.CreatePVCFromDefinition(c, ns, utils.NewPVCDefinition("import-e2e", "40Mi", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred(), "Error creating PVC")
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(c, ns, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", ns+"/"+common.ImporterPodName))

		l, err := labels.Parse(common.PrometheusLabel)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() int {
			endpoint, err = c.CoreV1().Endpoints(ns).Get(context.TODO(), "kubevirt-prometheus-metrics", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			_, err := c.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: l.String()})
			Expect(err).ToNot(HaveOccurred())
			return len(endpoint.Subsets)
		}, 60, 1).Should(Equal(1))

		By("Set up port forwarding")
		prometheusURL, portForwardCmd, err = startPrometheusPortForward(f)
		Expect(err).ToNot(HaveOccurred())

		By("checking if the endpoint contains the metrics port and only one matching subset")
		Expect(endpoint.Subsets[0].Ports).To(HaveLen(1))
		Expect(endpoint.Subsets[0].Ports[0].Name).To(Equal("metrics"))
		Expect(endpoint.Subsets[0].Ports[0].Port).To(Equal(int32(8443)))

		if importer.OwnerReferences[0].UID == pvc.GetUID() {
			var importRegExp = regexp.MustCompile("progress\\{ownerUID\\=\"" + string(pvc.GetUID()) + "\"\\} (\\d{1,3}\\.?\\d*)")
			Eventually(func() bool {
				fmt.Fprintf(GinkgoWriter, "INFO: Connecting to URL: %s\n", prometheusURL+"/metrics")
				resp, err := client.Get(prometheusURL + "/metrics")
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						bodyBytes, err := ioutil.ReadAll(resp.Body)
						Expect(err).NotTo(HaveOccurred())
						match := importRegExp.FindStringSubmatch(string(bodyBytes))
						if match != nil {
							return true
						}
					} else {
						fmt.Fprintf(GinkgoWriter, "INFO: received status code: %d\n", resp.StatusCode)
					}
				} else {
					fmt.Fprintf(GinkgoWriter, "INFO: collecting metrics failed: %v\n", err)
				}
				return false
			}, 90, 1).Should(BeTrue())
		} else {
			Fail("importer owner reference doesn't match PVC")
		}
	})
})

func startPrometheusPortForward(f *framework.Framework) (string, *exec.Cmd, error) {
	lp := "28443"
	pm := lp + ":8443"
	url := "https://127.0.0.1:" + lp

	cmd := tests.CreateKubectlCommand(f, "-n", f.Namespace.Name, "port-forward", "svc/kubevirt-prometheus-metrics", pm)
	err := cmd.Start()
	if err != nil {
		return "", nil, err
	}

	return url, cmd, nil
}

var _ = Describe("Importer Test Suite-Block_device", func() {
	f := framework.NewFramework(namespacePrefix)
	var pvc *v1.PersistentVolumeClaim
	var err error

	AfterEach(func() {
		if pvc != nil {
			f.DeletePVC(pvc)
		}
	})

	It("[test_id:4971]Should create import pod for block pv", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.iso"))

		pvc, err = f.CreatePVCFromDefinition(utils.NewBlockPVCDefinition(
			"import-image-to-block-pvc",
			"500Mi",
			pvcAnn,
			nil,
			f.BlockSCName))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, "/pvc", utils.UploadFileMD5, utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())

	})

	It("[test_id:4972]Should create blank raw image for block PV", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		dv := utils.NewDataVolumeForBlankRawImageBlock("create-blank-image-to-block-pvc", "500Mi", f.BlockSCName)
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Waiting for import to be completed")
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Verifying a message was printed to indicate a request for a blank disk on a block device")
		Eventually(func() bool {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return strings.Contains(log, "attempting to create blank disk for block mode")
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("[rfe_id:1947][crit:high][test_id:2145][vendor:cnv-qe@redhat.com][level:component]Importer Archive ContentType", func() {
	f := framework.NewFramework(namespacePrefix)

	It("Should import archive content type tar file", func() {
		c := f.K8sClient
		ns := f.Namespace.Name
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint:    httpEp + "/archive.tar",
			controller.AnnContentType: "archive",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/archive.tar"))
		pvc, err := utils.CreatePVCFromDefinition(c, ns, utils.NewPVCDefinition("import-archive", "100Mi", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred(), "Error creating PVC")
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		found, err := utils.WaitPVCPodStatusSucceeded(c, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Verify the target PVC contents")
		same, err := f.VerifyTargetPVCArchiveContent(f.Namespace, pvc, "3")
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())
	})
})

var _ = Describe("PVC import phase matches pod phase", func() {
	f := framework.NewFramework(namespacePrefix)

	It("[test_id:4980]Should never go to failed even if import fails", func() {
		c := f.K8sClient
		ns := f.Namespace.Name
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/invaliddoesntexist",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/invaliddoesntexist"))
		pvc, err := utils.CreatePVCFromDefinition(c, ns, utils.NewPVCDefinition("import-archive", "100Mi", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred(), "Error creating PVC")
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		found, err := utils.WaitPVCPodStatusRunning(c, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Verifying the phase annotation on the PVC never gets to failed")
		// Try for 20 seconds.
		stopTime := time.Now().Add(time.Second * 20)
		for time.Now().Before(stopTime) {
			testPvc, err := c.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(testPvc.GetAnnotations()[controller.AnnPodPhase]).To(BeEquivalentTo(v1.PodRunning))
			time.Sleep(time.Millisecond * 50)
		}
	})
})

var _ = Describe("Namespace with quota", func() {
	f := framework.NewFramework(namespacePrefix)
	var (
		orgConfig *v1.ResourceRequirements
	)

	BeforeEach(func() {
		By("Capturing original CDIConfig state")
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		orgConfig = config.Spec.PodResourceRequirements
		fmt.Fprintf(GinkgoWriter, "INFO: original config: %v\n", orgConfig)
	})

	AfterEach(func() {
		By("Restoring CDIConfig to original state")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.PodResourceRequirements = orgConfig
		})
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(config.Spec.PodResourceRequirements, orgConfig)
		}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not properly restored to original value")
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "INFO: new config: %v\n", config.Spec.PodResourceRequirements)
	})

	It("[test_id:4981]Should create import pod in namespace with quota", func() {
		err := f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.iso"))

		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-block-pvc",
			"500Mi",
			pvcAnn,
			nil))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, "/pvc", "d41d8cd98f00b204e9800998ecf8427e", utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")

	})

	It("[test_id:4982]Should fail to create import pod in namespace with quota, with resource limits higher in CDIConfig", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(2), int64(512*1024*1024), int64(2), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(512*1024*1024), int64(1), int64(1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.iso"))

		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-pvc",
			"500Mi",
			pvcAnn,
			nil))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify Quota was exceeded in logs")
		matchString := strings.Trim(fmt.Sprintf(`"name": "import-image-to-pvc", "namespace": "%s", "error": "pods \"importer-import-image-to-pvc\" is forbidden: exceeded quota: test-quota`, f.Namespace.Name), " ")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return strings.Trim(log, " ")
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
	})

	It("[test_id:4983]Should fail to create import pod in namespace with quota, then succeed once the quota is large enough", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(1), int64(512*1024*1024), int64(1), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(256*1024*1024), int64(1), int64(256*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.iso"))

		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-pvc",
			"500Mi",
			pvcAnn,
			nil))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify Quota was exceeded in logs")
		matchString := strings.Trim(fmt.Sprintf(`"name": "import-image-to-pvc", "namespace": "%s", "error": "pods \"importer-import-image-to-pvc\" is forbidden: exceeded quota: test-quota`, f.Namespace.Name), " ")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return strings.Trim(log, " ")
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

		err = f.UpdateQuotaInNs(int64(2), int64(512*1024*1024), int64(2), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, "/pvc", "d41d8cd98f00b204e9800998ecf8427e", utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")
	})

	It("[test_id:4984]Should create import pod in namespace with quota with CDIConfig within limits", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(0), int64(0), int64(1), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(512*1024*1024), int64(2), int64(1*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.iso"))

		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-block-pvc",
			"500Mi",
			pvcAnn,
			nil))
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, "/pvc", "d41d8cd98f00b204e9800998ecf8427e", utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")

	})
})

var _ = Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component] Add a field to DataVolume to track the number of retries", func() {
	f := framework.NewFramework(namespacePrefix)

	var (
		dataVolume           *cdiv1.DataVolume
		err                  error
		tinyCoreIsoURL       = func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
		invalidQcowImagesURL = func() string { return fmt.Sprintf(utils.InvalidQcowImagesURL, f.CdiInstallNs) }
	)

	AfterEach(func() {
		By("Delete DV")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return true
			}
			return false
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("[test_id:3994] Import datavolume with good url will leave dv retry count unchanged", func() {
		dvName := "import-dv"
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeWithHTTPImport(dvName, "100Mi", tinyCoreIsoURL())
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Verify retry annotation on PVC")
		restartsValue, status, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodRestarts)
		Expect(err).ToNot(HaveOccurred())
		Expect(status).To(BeTrue())
		Expect(restartsValue).To(Equal("0"))

		By("Verify the number of retries on the datavolume")
		dv, err = f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(dv.Status.RestartCount).To(BeNumerically("==", 0))
	})

	It("[test_id:3996] Import datavolume with bad url will increase dv retry count", func() {
		dvName := "import-dv-bad-url"
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeWithHTTPImport(dvName, "100Mi", invalidQcowImagesURL())
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())
		//pvc = utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.ImportInProgress
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Verify retry annotation on PVC")
		Eventually(func() int {
			restarts, status, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodRestarts)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(BeTrue())
			i, err := strconv.Atoi(restarts)
			Expect(err).ToNot(HaveOccurred())
			return i
		}, timeout, pollingInterval).Should(BeNumerically(">=", 1))

		By("Verify the number of retries on the datavolume")
		Eventually(func() int32 {
			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			restarts := dv.Status.RestartCount
			return restarts
		}, timeout, pollingInterval).Should(BeNumerically(">=", 1))
	})
})

var _ = Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component] CDI Label Naming - Import", func() {
	f := framework.NewFramework(namespacePrefix)

	var (
		// pvc            *v1.PersistentVolumeClaim
		dataVolume     *cdiv1.DataVolume
		err            error
		tinyCoreIsoURL = func() string { return fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs) }
	)

	AfterEach(func() {
		By("Delete DV")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return true
			}
			return false
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("[test_id:4269] Create datavolume with short name with import of archive - will generate scratch space and import pod names", func() {
		dvName := "import-short-name-dv"
		By(fmt.Sprintf("Creating new datavolume %s", dvName))

		dv := utils.NewDataVolumeWithArchiveContent(dvName, "1Gi", tinyCoreIsoURL())
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:4270] Create datavolume with long name with import of archive - will generate scratch space and import pod names", func() {
		// 20 chars + 100ch + 40chars
		dvName160Characters := "import-long-name-dv-" +
			"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
			"123456789-123456789-123456789-1234567890"
		By(fmt.Sprintf("Creating new datavolume %s", dvName160Characters))
		dv := utils.NewDataVolumeWithArchiveContent(dvName160Characters, "1Gi", tinyCoreIsoURL())
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:4271] Create datavolume with long name including special character '.' with import of archive - will generate scratch space and import pod names", func() {
		// 20 chars + 100ch + 40chars with dot
		dvName160Characters := "import-long-name-dv." +
			"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
			"123456789-123456789-123456789-1234567890"
		By(fmt.Sprintf("Creating new datavolume %s", dvName160Characters))

		dv := utils.NewDataVolumeWithArchiveContent(dvName160Characters, "1Gi", tinyCoreIsoURL())
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("Preallocation", func() {
	f := framework.NewFramework(namespacePrefix)
	dvName := "import-dv"

	var (
		dataVolume          *cdiv1.DataVolume
		err                 error
		tinyCoreIsoURL      = func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
		tinyCoreQcow2URL    = func() string { return fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs) }
		tinyCoreTarURL      = func() string { return fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs) }
		tinyCoreRegistryURL = func() string { return fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs) }
		imageioURL          = func() string { return fmt.Sprintf(utils.ImageioURL, f.CdiInstallNs) }
		vcenterURL          = func() string { return fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs) }
		config              *cdiv1.CDIConfig
		origSpec            *cdiv1.CDIConfigSpec
	)

	BeforeEach(func() {
		config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		origSpec = config.Spec.DeepCopy()
	})

	AfterEach(func() {
		By("Delete DV")
		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return true
			}
			return false
		}, timeout, pollingInterval).Should(BeTrue())

		By("Restoring CDIConfig to original state")
		err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			origSpec.DeepCopyInto(config)
		})

		Eventually(func() bool {
			config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), "cdi", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(config.Spec, origSpec)
		}, 30*time.Second, time.Second)
	})

	It("Importer should add preallocation when requested", func() {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeWithHTTPImport(dvName, "100Mi", tinyCoreIsoURL())
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Equal("true"))
	})

	It("Importer should not add preallocation when preallocation=false", func() {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dataVolume = utils.NewDataVolumeWithHTTPImport(dvName, "100Mi", tinyCoreIsoURL())
		preallocation := false
		dataVolume.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).ShouldNot(Equal("true"))
	})

	DescribeTable("All import paths should contain Preallocation step", func(shouldPreallocate bool, dvFunc func() *cdiv1.DataVolume) {
		dv := dvFunc()
		By(fmt.Sprintf("Creating new datavolume %s", dv.Name))
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		should := ContainSubstring("New phase: Preallocate")
		if !shouldPreallocate {
			should = Not(should)
		}

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		if shouldPreallocate {
			Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Equal("true"))
		} else {
			Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).ShouldNot(Equal("true"))
		}
	},
		Entry("HTTP import (ISO image)", true, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreIsoURL())
		}),
		Entry("HTTP import (QCOW2 image)", true, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreQcow2URL())
		}),
		Entry("HTTP import (TAR image)", true, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreTarURL())
		}),
		Entry("HTTP import (archive content)", false, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithArchiveContent("import-dv", "100Mi", tinyCoreTarURL())
		}),
		Entry("HTTP Import (TAR image, block DataVolume)", true, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreTarURL(), f.BlockSCName)
		}),
		Entry("HTTP Import (ISO image, block DataVolume)", true, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreIsoURL(), f.BlockSCName)
		}),
		Entry("HTTP Import (QCOW2 image, block DataVolume)", true, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreQcow2URL(), f.BlockSCName)
		}),
		Entry("ImageIO import", true, func() *cdiv1.DataVolume {
			cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			stringData := map[string]string{
				common.KeyAccess: "YWRtaW5AaW50ZXJuYWw=",
				common.KeySecret: "MTIzNDU2",
			}
			s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
			return utils.NewDataVolumeWithImageioImport("import-dv", "100Mi", imageioURL(), s.Name, cm, "123")
		}),
		Entry("Registry import", true, func() *cdiv1.DataVolume {
			dataVolume = utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", tinyCoreRegistryURL())
			cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(BeNil())
			dataVolume.Spec.Source.Registry.CertConfigMap = cm
			return dataVolume
		}),
		Entry("VddkImport", true, func() *cdiv1.DataVolume {
			// Find vcenter-simulator pod
			pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "vcenter-deployment", "app=vcenter")
			Expect(err).ToNot(HaveOccurred())
			Expect(pod).ToNot(BeNil())

			// Get test VM UUID
			id, err := tests.RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmid")
			Expect(err).To(BeNil())
			vmid, err := uuid.Parse(strings.TrimSpace(id))
			Expect(err).To(BeNil())

			// Get disk name
			disk, err := tests.RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "--", "cat", "/tmp/vmdisk")
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

			return utils.NewDataVolumeWithVddkImport("import-dv", "100Mi", backingFile, s.Name, thumbprint, vcenterURL(), vmid.String())
		}),
		Entry("Blank image", true, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage("import-dv", "100Mi")
		}),
		Entry("Blank block DataVolume", true, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeForBlankRawImageBlock("import-dv", "100Mi", f.BlockSCName)
		}),
	)

	It("Filesystem overhead is honored with blank volume", func() {
		setFilesystemOverhead(f, config, "0.055", "0.055")

		dv := utils.NewDataVolumeForBlankRawImage("import-dv", "100Mi")
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify the pod status is succeeded on the target PVC")
		found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(f.VerifyFSOverhead(f.Namespace, pvc, preallocation)).To(BeTrue())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Equal("true"))
	})
})

func setFilesystemOverhead(f *framework.Framework, config *cdiv1.CDIConfig, globalOverhead, scOverhead string) {
	defaultSCName := utils.DefaultStorageClass.GetName()
	testedFilesystemOverhead := &cdiv1.FilesystemOverhead{}
	if globalOverhead != "" {
		testedFilesystemOverhead.Global = cdiv1.Percent(globalOverhead)
	}
	if scOverhead != "" {
		testedFilesystemOverhead.StorageClass = map[string]cdiv1.Percent{defaultSCName: cdiv1.Percent(scOverhead)}
	}
	config.Spec.FilesystemOverhead = testedFilesystemOverhead.DeepCopy()
	By(fmt.Sprintf("Updating CDIConfig filesystem overhead to %v", config.Spec.FilesystemOverhead))
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
