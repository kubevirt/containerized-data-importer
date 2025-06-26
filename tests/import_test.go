package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
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

var _ = Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component]Importer Test Suite", Serial, func() {
	var (
		ns string
		f  = framework.NewFramework(namespacePrefix)
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
	})

	DescribeTable("[test_id:2329] Should fail to import images that require too much space", Label("no-kubernetes-in-docker"), func(uploadURL string) {
		imageURL := fmt.Sprintf(uploadURL, f.CdiInstallNs)

		By(imageURL)
		dv := utils.NewDataVolumeWithHTTPImport("too-large-import", "500Mi", imageURL)
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), "Unable to get importer pod")

		By(fmt.Sprintf("logs for pod -n %s %s", importer.Name, importer.Namespace))
		By("Verify datavolume too small condition")
		runningCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeRunning,
			Status:  v1.ConditionFalse,
			Message: "DataVolume too small to contain image",
			Reason:  "Error",
		}
		utils.WaitForConditions(f, dv.Name, f.Namespace.Name, controllerSkipPVCCompleteTimeout, assertionPollInterval, runningCondition)
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
		Expect(err).ToNot(HaveOccurred())

		By("Verifying PVC with no annotation remains empty")
		matchString := fmt.Sprintf("PVC annotation not found, skipping pvc\t{\"PVC\": {\"name\":\"%s\",\"namespace\":\"%s\"}, \"annotation\": \"%s\"}", pvc.Name, ns, controller.AnnEndpoint)
		Eventually(func() ([]byte, error) {
			return f.K8sClient.CoreV1().
				Pods(f.CdiInstallNs).
				GetLogs(f.ControllerPod.Name, &v1.PodLogOptions{SinceTime: &metav1.Time{Time: CurrentSpecReport().StartTime}}).
				DoRaw(context.Background())
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

		// Wait a while to see if CDI puts anything in the PVC.
		isEmpty, err := framework.VerifyPVCIsEmpty(f, pvc, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(isEmpty).To(BeTrue())
		// Not deleting PVC as it will be removed with the NS removal.
	})

	It("[test_id:4969]Should create import pod for blank raw image", func() {
		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition(
			"create-image",
			"1Gi",
			map[string]string{controller.AnnSource: controller.SourceNone, controller.AnnContentType: string(cdiv1.DataVolumeKubeVirt)},
			nil))

		By("Verify the pod status is succeeded on the target PVC")
		found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Verify the image contents")
		Expect(f.VerifyBlankDisk(f.Namespace, pvc)).To(BeTrue())
		By("Verifying the image is sparse")
		Expect(f.VerifySparse(f.Namespace, pvc, utils.DefaultImagePath)).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")
		if utils.DefaultStorageCSIRespectsFsGroup {
			// CSI storage class, it should respect fsGroup
			By("Checking that disk image group is qemu")
			Expect(f.GetDiskGroup(f.Namespace, pvc, false)).To(Equal("107"))
		}
	})

	It("[test_id:6687] Should retain the importer pod after completion with dv annotation cdi.kubevirt.io/storage.pod.retainAfterCompletion=true", func() {
		dataVolume := utils.NewDataVolumeWithHTTPImport("import-pod-retain-test", "100Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verify pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Find importer pod after completion")
		importer, err := utils.FindPodByPrefixOnce(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).ToNot(HaveOccurred())
		Expect(importer.DeletionTimestamp).To(BeNil())
	})

	It("[test_id:6688] Should retain all multi-stage importer pods after completion with dv annotation cdi.kubevirt.io/storage.pod.retainAfterCompletion=true", Label("VDDK"), func() {
		vcenterURL := fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs)
		dataVolume := f.CreateVddkWarmImportDataVolume("import-pod-retain-test", "100Mi", vcenterURL)

		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verify pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Find importer pods after completion")
		for _, checkpoint := range dataVolume.Spec.Checkpoints {
			pvcName := dataVolume.Name
			// When using populators, the PVC Prime name is used to build the importer pod
			if usePopulator, _ := dvc.CheckPVCUsingPopulators(pvc); usePopulator {
				pvcName = populators.PVCPrimeName(pvc)
			}
			name := fmt.Sprintf("%s-%s-checkpoint-%s", common.ImporterPodName, pvcName, checkpoint.Current)
			By("Find importer pod " + name)
			importer, err := utils.FindPodByPrefixOnce(f.K8sClient, dataVolume.Namespace, name, common.CDILabelSelector)
			Expect(err).ToNot(HaveOccurred())
			Expect(importer.DeletionTimestamp).To(BeNil())
		}
	})
})

var _ = Describe("[Istio] Namespace sidecar injection", Serial, func() {
	var (
		f = framework.NewFramework(namespacePrefix)

		// Istio sidecar injection prevents access to external resources, so we cannot use internal urls (http://cdi-file-host) for the test
		tinyCoreIsoExternalURL = "http://tinycorelinux.net/12.x/x86/release/TinyCore-current.iso"
	)

	BeforeEach(func() {
		value := os.Getenv("KUBEVIRT_DEPLOY_ISTIO")
		if value != "true" {
			Skip("No Istio enabled, skipping.")
		}

		By("Enable istio sidecar injection in namespace")
		labelPatch := `[{"op":"add","path":"/metadata/labels/istio-injection","value":"enabled" }]`
		_, err := f.K8sClient.CoreV1().Namespaces().Patch(context.TODO(), f.Namespace.Name, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
		Expect(err).ShouldNot(HaveOccurred())

		By("Create istio sidecar")
		sidecarRes := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "sidecars"}
		registryOnlySidecar := generateRegistryOnlySidecar()
		_, err = f.DynamicClient.Resource(sidecarRes).Namespace(f.Namespace.Name).Create(context.TODO(), registryOnlySidecar, metav1.CreateOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("[test_id:6498] Should fail to import with namespace sidecar injection enabled, and sidecar.istio.io/inject set to true", func() {
		dataVolume := utils.NewDataVolumeWithHTTPImport("istio-sidecar-injection-test", "100Mi", tinyCoreIsoExternalURL)
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		// We set the Immediate Binding annotation to true, to eliminate creation of the consumer pod, which will also fail due to the Istio sidecar.
		dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
		// A single service mesh provider is deployed so either Istio or Linkerd, not both.
		dataVolume.Annotations[controller.AnnPodSidecarInjectionIstio] = "true"
		dataVolume.Annotations[controller.AnnPodSidecarInjectionLinkerd] = "enabled"
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		var importer *v1.Pod
		By("Find importer pod")
		Eventually(func() bool {
			importer, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
			return err == nil
		}, timeout, pollingInterval).Should(BeTrue())

		By("Verify HTTP request error in importer log")
		Eventually(func() (string, error) {
			out, err := f.K8sClient.CoreV1().
				Pods(importer.Namespace).
				GetLogs(importer.Name, &v1.PodLogOptions{
					SinceTime: &metav1.Time{Time: CurrentSpecReport().StartTime},
					Container: "importer",
				}).
				DoRaw(context.Background())
			return string(out), err
		}, time.Minute, pollingInterval).Should(Or(
			ContainSubstring("HTTP request errored"),
			ContainSubstring("502 Bad Gateway"),
		))
	})

	It("[test_id:6492] Should successfully import with namespace sidecar injection enabled and default sidecar.istio.io/inject", func() {
		dataVolume := utils.NewDataVolumeWithHTTPImport("istio-sidecar-injection-test", "100Mi", tinyCoreIsoExternalURL)
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verify pvc was created")
		_, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Find importer pod")
		Eventually(func() bool {
			_, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
			return err == nil
		}, timeout, pollingInterval).Should(BeTrue())

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")
	})
})

var _ = Describe("[rfe_id:4784][crit:high] Importer respects node placement", Serial, func() {
	var cr *cdiv1.CDI
	var oldSpec *cdiv1.CDISpec
	f := framework.NewFramework(namespacePrefix)

	// An image that fails import
	imageTooLargeSize := func() string {
		return fmt.Sprintf(utils.LargeVirtualDiskXz, f.CdiInstallNs)
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
		if cr == nil {
			return
		}
		cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cr.Spec = *oldSpec.DeepCopy()
		_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			cr, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(cr.Spec, *oldSpec)
		}, 30*time.Second, time.Second).Should(BeTrue())
		cr = nil
	})

	It("[test_id:4783] Should create import pod with node placement", func() {
		cr.Spec.Workloads = f.TestNodePlacementValues()
		_, err := f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for CDI CR update to take effect")
		Eventually(func() bool {
			realCR, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(cr.Spec, realCR.Spec)
		}, 30*time.Second, time.Second).Should(BeTrue())

		dv := utils.NewDataVolumeWithHTTPImport("node-placement-test", "100Mi", imageTooLargeSize())
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), "Unable to get importer pod")

		By("Verify the import pod has nodeSelector")
		Expect(importer.Spec.NodeSelector).To(Equal(framework.NodeSelectorTestValue))
		By("Verify the import pod has affinity")
		Expect(importer.Spec.Affinity).To(Equal(framework.AffinityTestValue))
		By("Verify the import pod has tolerations")
		Expect(importer.Spec.Tolerations).To(ContainElement(framework.TolerationsTestValue[0]))
	})
})

var _ = Describe("[rfe_id:1118][crit:high][vendor:cnv-qe@redhat.com][level:component]Importer Test Suite-prometheus", Serial, func() {
	var prometheusURL string
	var portForwardCmd *exec.Cmd
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // It's not production code
		},
	}
	f := framework.NewFramework(namespacePrefix)

	BeforeEach(func() {
		_, err := f.CreatePrometheusServiceInNs(f.Namespace.Name)
		Expect(err).NotTo(HaveOccurred(), "Error creating prometheus service")
	})

	AfterEach(func() {
		By("Stop port forwarding")
		afterCMD(portForwardCmd)
	})

	// Skipping this test until we can get progress information again. What happens is that the go
	// http client cannot determine the total size, and thus the prometheus endpoint is not initialized
	// This causes this test to now fail because the endpoint is not there, skipping for now.
	PIt("[test_id:4970]Import pod should have prometheus stats available while importing", func() {
		var endpoint *v1.Endpoints
		c := f.K8sClient
		ns := f.Namespace.Name
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPRateLimitPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.qcow2",
			controller.AnnSecret:   "",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/tinyCore.qcow2"))
		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition("import-e2e", "40Mi", pvcAnn, nil))

		importer, err := utils.FindPodByPrefix(c, ns, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", ns+"/"+common.ImporterPodName))

		l, err := labels.Parse(common.PrometheusLabelKey)
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
						bodyBytes, err := io.ReadAll(resp.Body)
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

	cmd := f.CreateKubectlCommand("-n", f.Namespace.Name, "port-forward", "svc/kubevirt-prometheus-metrics", pm)
	err := cmd.Start()
	if err != nil {
		return "", nil, err
	}

	return url, cmd, nil
}

var _ = Describe("Importer Test Suite-Block_device", func() {
	f := framework.NewFramework(namespacePrefix)
	var (
		pvc            *v1.PersistentVolumeClaim
		err            error
		tinyCoreIsoURL = func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
	)

	AfterEach(func() {
		if pvc != nil {
			Expect(f.DeletePVC(pvc)).To(Succeed())
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

		pvc = f.CreateBoundPVCFromDefinition(utils.NewBlockPVCDefinition(
			"import-image-to-block-pvc",
			"500Mi",
			pvcAnn,
			nil,
			f.BlockSCName))

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultPvcMountPath, utils.UploadFileMD5, utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())

	})

	DescribeTable("Should create blank raw image for block PV", func(consumer bool) {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		dv := utils.NewDataVolumeForBlankRawImageBlock("create-blank-image-to-block-pvc", "500Mi", f.BlockSCName)
		if !consumer {
			controller.AddAnnotation(dv, controller.AnnImmediateBinding, "true")
		}
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		if consumer {
			f.ForceBindIfWaitForFirstConsumer(pvc)
		}

		By("Waiting for import to be completed")
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("[test_id:4972] with first consumer", true),
		Entry("with bind immediate annotation", false),
	)

	It("Should perform fsync syscall after qemu-img convert to raw", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		dataVolume := utils.NewDataVolumeWithHTTPImportToBlockPV("qemu-img-convert-fsync-test", "4Gi", tinyCoreIsoURL(), f.BlockSCName)
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume.SetAnnotations(map[string]string{})
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		var importer *v1.Pod
		By("Find importer pod")
		Eventually(func() bool {
			importer, err = utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
			return err == nil
		}, timeout, pollingInterval).Should(BeTrue())

		By("Verify fsync() syscall was made")
		matchString := fmt.Sprintf("Successfully completed fsync(%s) syscall", common.WriteBlockPath)
		Eventually(func() (string, error) {
			out, err := f.K8sClient.CoreV1().
				Pods(importer.Namespace).
				GetLogs(importer.Name, &v1.PodLogOptions{SinceTime: &metav1.Time{Time: CurrentSpecReport().StartTime}}).
				DoRaw(context.Background())
			return string(out), err
		}, 3*time.Minute, pollingInterval).Should(ContainSubstring(matchString))

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		zero := int64(0)
		err = utils.DeletePodByName(f.K8sClient, fmt.Sprintf("%s-%s", common.ImporterPodName, dataVolume.Name), f.Namespace.Name, &zero)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dataVolume), utils.DefaultPvcMountPath, utils.UploadFileMD5, utils.UploadFileSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())

		By("Delete DV")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("[rfe_id:1947][crit:high][test_id:2145][vendor:cnv-qe@redhat.com][level:component]Importer Archive ContentType", Serial, func() {
	f := framework.NewFramework(namespacePrefix)

	It("Should import archive content type tar file", func() {
		c := f.K8sClient
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint:    httpEp + "/archive.tar",
			controller.AnnContentType: "archive",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/archive.tar"))
		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition("import-archive", "100Mi", pvcAnn, nil))

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

var _ = Describe("PVC import phase matches pod phase", Serial, func() {
	f := framework.NewFramework(namespacePrefix)

	It("[test_id:4980]Should never go to failed even if import fails", func() {
		c := f.K8sClient
		ns := f.Namespace.Name
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPNoAuthPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/invaliddoesntexist",
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", httpEp+"/invaliddoesntexist"))
		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition("import-archive", "100Mi", pvcAnn, nil))

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

var _ = Describe("Namespace with quota", Serial, func() {
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

		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-pvc",
			"500Mi",
			pvcAnn,
			nil))

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
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

		f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-pvc",
			"500Mi",
			pvcAnn,
			nil))

		By("Check the expected event")
		msg := fmt.Sprintf(controller.MessageErrStartingPod, "importer-import-image-to-pvc")
		f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(msg))
		f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrExceededQuota))
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

		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-pvc",
			"500Mi",
			pvcAnn,
			nil))

		By("Check the expected event")
		msg := fmt.Sprintf(controller.MessageErrStartingPod, "importer-import-image-to-pvc")
		f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(msg))
		f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrExceededQuota))

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
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
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

		pvc := f.CreateBoundPVCFromDefinition(utils.NewPVCDefinition(
			"import-image-to-block-pvc",
			"500Mi",
			pvcAnn,
			nil))

		By("Verify the pod status is succeeded on the target PVC")
		Eventually(func() string {
			status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
			Expect(err).ToNot(HaveOccurred())
			Expect(phaseAnnotation).To(BeTrue())
			return status
		}, CompletionTimeout, assertionPollInterval).Should(BeEquivalentTo(v1.PodSucceeded))

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
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
			return k8serrors.IsNotFound(err)
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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

	It("[test_id:3996] Import datavolume with bad url will increase dv retry count", Serial, func() {
		if f.IsPrometheusAvailable() {
			dataVolumeNoUnusualRestartTest(f)
		}

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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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
		}, timeout, pollingInterval).Should(BeNumerically(">", common.UnusualRestartCountThreshold))

		if f.IsPrometheusAvailable() {
			dataVolumeUnusualRestartTest(f)
		}
	})
})

var _ = Describe("[rfe_id:1115][crit:high][vendor:cnv-qe@redhat.com][level:component] CDI Label Naming - Import", func() {
	f := framework.NewFramework(namespacePrefix)

	var (
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
			return k8serrors.IsNotFound(err)
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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
		dataVolume              *cdiv1.DataVolume
		err                     error
		tinyCoreIsoURL          = func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
		tinyCoreQcow2URL        = func() string { return fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs) }
		tinyCoreTarURL          = func() string { return fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs) }
		tinyCoreRegistryURL     = func() string { return fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs) }
		imageioURL              = func() string { return fmt.Sprintf(utils.ImageioURL, f.CdiInstallNs) }
		vcenterURL              = func() string { return fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs) }
		config                  *cdiv1.CDIConfig
		origSpec                *cdiv1.CDIConfigSpec
		trustedRegistryURL      = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix) }
		trustedRegistryURLQcow2 = func() string { return fmt.Sprintf(utils.TrustedRegistryURLQcow2, f.DockerPrefix) }
		trustedRegistryIS       = func() string { return fmt.Sprintf(utils.TrustedRegistryIS, f.DockerPrefix) }
	)

	BeforeEach(func() {
		config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		origSpec = config.Spec.DeepCopy()
	})

	AfterEach(func() {
		if dataVolume != nil {
			By("Delete DV")
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
			}, timeout, pollingInterval).Should(BeTrue())
			dataVolume = nil
		}

		By("Restoring CDIConfig to original state")
		err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			origSpec.DeepCopyInto(config)
		})

		Eventually(func() bool {
			config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(config.Spec, *origSpec)
		}, 30*time.Second, time.Second).Should(BeTrue())
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Equal("true"))

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))

		ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
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
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).ShouldNot(Equal("true"))

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))

		By("Verify preallocated size")
		ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	DescribeTable("[test_id:7241] All import paths should contain Preallocation step", func(shouldPreallocate bool, expectedMD5, path string, dvFunc func() *cdiv1.DataVolume) {
		dv := dvFunc()
		By(fmt.Sprintf("Creating new datavolume %s", dv.Name))
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
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

			By("Verify content")
			md5, err := f.GetMD5(f.Namespace, pvc, path, utils.MD5PrefixSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(md5).To(Equal(expectedMD5))

			if !f.IsBlockVolumeStorageClassAvailable() {
				// Block volumes can't be read with qemu-img
				ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			}
		} else {
			Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).ShouldNot(Equal("true"))
		}

		if dv.Spec.Source.Registry != nil && dv.Spec.Source.Registry.ImageStream != nil {
			By("Verify image lookup annotation")
			podName := pvc.Annotations[controller.AnnImportPod]
			if pvc.Spec.DataSourceRef != nil {
				Expect(podName).To(BeEmpty())
			} else {
				// when using populators when the population completes PVC' and
				// the importer pod are deleted, so can't check the annotation
				// TODO: any suggestions? putting the check before dv completes is
				// still racy
				pod, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Annotations[controller.AnnOpenShiftImageLookup]).To(Equal("*"))
			}
		}
	},
		Entry("HTTP import (ISO image)", true, utils.TinyCoreMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreIsoURL())
		}),
		Entry("HTTP import (QCOW2 image)", true, utils.TinyCoreMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreQcow2URL())
		}),
		Entry("HTTP import (TAR image)", true, utils.TinyCoreTarMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", tinyCoreTarURL())
		}),
		Entry("HTTP import (archive content)", false, "", "", func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithArchiveContent("import-dv", "100Mi", tinyCoreTarURL())
		}),
		Entry("HTTP Import (TAR image, block DataVolume)", true, utils.TinyCoreTarMD5, utils.DefaultPvcMountPath, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreTarURL(), f.BlockSCName)
		}),
		Entry("HTTP Import (ISO image, block DataVolume)", true, utils.TinyCoreMD5, utils.DefaultPvcMountPath, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreIsoURL(), f.BlockSCName)
		}),
		Entry("HTTP Import (QCOW2 image, block DataVolume)", true, utils.TinyCoreMD5, utils.DefaultPvcMountPath, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeWithHTTPImportToBlockPV("import-dv", "4Gi", tinyCoreQcow2URL(), f.BlockSCName)
		}),
		Entry("ImageIO import", Label("ImageIO"), Serial, true, utils.ImageioMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
			stringData := map[string]string{
				common.KeyAccess: "admin@internal",
				common.KeySecret: "12345",
			}
			tests.CreateImageIoDefaultInventory(f)
			s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
			return utils.NewDataVolumeWithImageioImport("import-dv", "100Mi", imageioURL(), s.Name, cm, "123")
		}),
		Entry("Registry import", true, utils.TinyCoreMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			dataVolume = utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", tinyCoreRegistryURL())
			cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
			dataVolume.Spec.Source.Registry.CertConfigMap = &cm
			return dataVolume
		}),
		Entry("Registry node pull import raw", true, utils.TinyCoreMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			pullMethod := cdiv1.RegistryPullNode
			dataVolume = utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", trustedRegistryURL())
			dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
			return dataVolume
		}),
		Entry("Registry node pull import qcow2", true, utils.CirrosMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			pullMethod := cdiv1.RegistryPullNode
			dataVolume = utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", trustedRegistryURLQcow2())
			dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
			return dataVolume
		}),
		Entry("Registry ImageStream-wannabe node pull import", true, utils.TinyCoreMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			pullMethod := cdiv1.RegistryPullNode
			imageStreamWannabe := trustedRegistryIS()
			dataVolume = utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", "")
			dataVolume.Spec.Source.Registry.URL = nil
			dataVolume.Spec.Source.Registry.ImageStream = &imageStreamWannabe
			dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			return dataVolume
		}),
		Entry("VddkImport", Label("VDDK"), true, utils.VcenterMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
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

			return utils.NewDataVolumeWithVddkImport("import-dv", "100Mi", backingFile, s.Name, thumbprint, vcenterURL(), vmid.String())
		}),
		Entry("Blank image", true, utils.BlankMD5, utils.DefaultImagePath, func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForBlankRawImage("import-dv", "100Mi")
		}),
		Entry("Blank block DataVolume", true, utils.BlankMD5, utils.DefaultPvcMountPath, func() *cdiv1.DataVolume {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			return utils.NewDataVolumeForBlankRawImageBlock("import-dv", "1Gi", f.BlockSCName)
		}),
	)

	It("Filesystem overhead is honored with blank volume", Serial, func() {
		tests.SetFilesystemOverhead(f, "0.055", "0.055")

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

		// incase of using populators the requested size with the fsoverhead
		// is put only on the PVC' which at thisd point we can't check
		// TODO: any suggestions? getting the requested size from PVC' in earlier
		// point in the test seems to be racy
		if pvc.Spec.DataSourceRef == nil {
			Expect(f.VerifyFSOverhead(f.Namespace, pvc, preallocation)).To(BeTrue())
		}

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Equal("true"))

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.BlankMD5))

		ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
	})
})

var _ = Describe("Import populator", func() {
	f := framework.NewFramework(namespacePrefix)

	var (
		err                error
		pvc                *v1.PersistentVolumeClaim
		pvcPrime           *v1.PersistentVolumeClaim
		tinyCoreIsoURL     = func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
		tinyCoreArchiveURL = func() string { return fmt.Sprintf(utils.TarArchiveURL, f.CdiInstallNs) }
		trustedRegistryURL = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix) }
		imageioURL         = func() string { return fmt.Sprintf(utils.ImageioURL, f.CdiInstallNs) }
		vcenterURL         = func() string { return fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs) }
	)

	// importPopulationPVCDefinition creates a PVC with import datasourceref
	importPopulationPVCDefinition := func() *v1.PersistentVolumeClaim {
		pvcDef := utils.NewPVCDefinition("import-populator-pvc-test", "1Gi", nil, nil)
		apiGroup := controller.AnnAPIGroup
		pvcDef.Spec.DataSourceRef = &v1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     cdiv1.VolumeImportSourceRef,
			Name:     "import-populator-test",
		}
		return pvcDef
	}

	// importPopulatorCR creates an import source CR
	importPopulatorCR := func(namespace string, contentType cdiv1.DataVolumeContentType, preallocation bool, source *cdiv1.ImportSourceType) *cdiv1.VolumeImportSource {
		return &cdiv1.VolumeImportSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "import-populator-test",
				Namespace: namespace,
			},
			Spec: cdiv1.VolumeImportSourceSpec{
				Source:        source,
				ContentType:   contentType,
				Preallocation: &preallocation,
			},
		}
	}

	// ImporSource creation functions

	createHTTPImportPopulatorCR := func(contentType cdiv1.DataVolumeContentType, preallocation bool) error {
		By("Creating Import Populator CR with HTTP source")
		url := tinyCoreArchiveURL()
		if contentType == cdiv1.DataVolumeKubeVirt {
			url = tinyCoreIsoURL()
		}
		source := &cdiv1.ImportSourceType{
			HTTP: &cdiv1.DataVolumeSourceHTTP{
				URL: url,
			},
		}
		importPopulatorCR := importPopulatorCR(f.Namespace.Name, contentType, preallocation, source)
		_, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), importPopulatorCR, metav1.CreateOptions{})
		return err
	}

	createRegistryImportPopulatorCR := func(contentType cdiv1.DataVolumeContentType, preallocation bool) error {
		By("Creating Import Populator CR with Registry source")
		registryURL := trustedRegistryURL()
		pullMethod := cdiv1.RegistryPullNode
		source := &cdiv1.ImportSourceType{
			Registry: &cdiv1.DataVolumeSourceRegistry{
				URL:        &registryURL,
				PullMethod: &pullMethod,
			},
		}
		importPopulatorCR := importPopulatorCR(f.Namespace.Name, contentType, preallocation, source)
		_, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), importPopulatorCR, metav1.CreateOptions{})
		return err
	}

	createImageIOImportPopulatorCR := func(contentType cdiv1.DataVolumeContentType, preallocation bool) error {
		By("Creating Import Populator CR with ImageIO source")
		cm, err := utils.CopyImageIOCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		stringData := map[string]string{
			common.KeyAccess: "admin@internal",
			common.KeySecret: "12345",
		}
		tests.CreateImageIoDefaultInventory(f)
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		source := &cdiv1.ImportSourceType{
			Imageio: &cdiv1.DataVolumeSourceImageIO{
				URL:           imageioURL(),
				SecretRef:     s.Name,
				CertConfigMap: cm,
				DiskID:        "123",
			},
		}
		importPopulatorCR := importPopulatorCR(f.Namespace.Name, contentType, preallocation, source)
		_, err = f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), importPopulatorCR, metav1.CreateOptions{})
		return err
	}

	createVDDKImportPopulatorCR := func(contentType cdiv1.DataVolumeContentType, preallocation bool) error {
		By("Creating Import Populator CR with VDDK source")
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
		source := &cdiv1.ImportSourceType{
			VDDK: &cdiv1.DataVolumeSourceVDDK{
				BackingFile: backingFile,
				SecretRef:   s.Name,
				Thumbprint:  thumbprint,
				URL:         vcenterURL(),
				UUID:        vmid.String(),
			},
		}

		importPopulatorCR := importPopulatorCR(f.Namespace.Name, contentType, preallocation, source)
		_, err = f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), importPopulatorCR, metav1.CreateOptions{})
		return err
	}

	createBlankImportPopulatorCR := func(contentType cdiv1.DataVolumeContentType, preallocation bool) error {
		By("Creating Import Populator CR with blank source")
		source := &cdiv1.ImportSourceType{
			Blank: &cdiv1.DataVolumeBlankImage{},
		}
		importPopulatorCR := importPopulatorCR(f.Namespace.Name, contentType, preallocation, source)
		_, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), importPopulatorCR, metav1.CreateOptions{})
		return err
	}

	verifyCleanup := func(pvc *v1.PersistentVolumeClaim) {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}
	}

	BeforeEach(func() {
		if utils.DefaultStorageClassCsiDriver == nil {
			Skip("No CSI driver found")
		}
		verifyCleanup(pvc)
	})

	AfterEach(func() {
		By("Deleting verifier pod")
		err := utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())

		err = f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Delete(context.TODO(), "import-populator-test", metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		if pvc != nil {
			By("Delete import population PVC")
			err = f.DeletePVC(pvc)
			Expect(err).ToNot(HaveOccurred())
			pvc = nil
		}
	})

	DescribeTable("should import fileSystem PVC", func(expectedMD5 string, volumeImportSourceFunc func(cdiv1.DataVolumeContentType, bool) error, preallocation, webhookRendering bool) {
		pvc = importPopulationPVCDefinition()

		if webhookRendering {
			controller.AddLabel(pvc, common.PvcApplyStorageProfileLabel, "true")
			// Unset AccessModes which will be set by the webhook rendering
			pvc.Spec.AccessModes = nil
		}

		pvc = f.CreateScheduledPVCFromDefinition(pvc)
		err = volumeImportSourceFunc(cdiv1.DataVolumeKubeVirt, preallocation)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(expectedMD5))

		if preallocation {
			By("Verifying the image is preallocated")
			ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		} else {
			By("Verifying the image is sparse")
			Expect(f.VerifySparse(f.Namespace, pvc, utils.DefaultImagePath)).To(BeTrue())
		}

		if utils.DefaultStorageCSIRespectsFsGroup {
			// CSI storage class, it should respect fsGroup
			By("Checking that disk image group is qemu")
			Expect(f.GetDiskGroup(f.Namespace, pvc, false)).To(Equal("107"))
		}

		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	},
		Entry("[test_id:11001]with HTTP image and preallocation", utils.TinyCoreMD5, createHTTPImportPopulatorCR, true, false),
		Entry("[test_id:11002]with HTTP image without preallocation", utils.TinyCoreMD5, createHTTPImportPopulatorCR, false, false),
		Entry("[rfe_id:10985][crit:high][test_id:11003]with HTTP image and preallocation, with incomplete PVC webhook rendering", utils.TinyCoreMD5, createHTTPImportPopulatorCR, true, true),
		Entry("[test_id:11004]with Registry image and preallocation", utils.TinyCoreMD5, createRegistryImportPopulatorCR, true, false),
		Entry("[test_id:11005]with Registry image without preallocation", utils.TinyCoreMD5, createRegistryImportPopulatorCR, false, false),
		Entry("[test_id:11006]with ImageIO image with preallocation", Label("ImageIO"), Serial, utils.ImageioMD5, createImageIOImportPopulatorCR, true, false),
		Entry("[test_id:11007]with ImageIO image without preallocation", Label("ImageIO"), Serial, utils.ImageioMD5, createImageIOImportPopulatorCR, false, false),
		Entry("[test_id:11008]with VDDK image with preallocation", Label("VDDK"), utils.VcenterMD5, createVDDKImportPopulatorCR, true, false),
		Entry("[test_id:11009]with VDDK image without preallocation", Label("VDDK"), utils.VcenterMD5, createVDDKImportPopulatorCR, false, false),
		Entry("[test_id:11010]with Blank image with preallocation", utils.BlankMD5, createBlankImportPopulatorCR, true, false),
		Entry("[test_id:11011]with Blank image without preallocation", utils.BlankMD5, createBlankImportPopulatorCR, false, false),
	)

	DescribeTable("should import Block PVC", func(expectedMD5 string, volumeImportSourceFunc func(cdiv1.DataVolumeContentType, bool) error) {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}

		pvc = importPopulationPVCDefinition()
		volumeMode := v1.PersistentVolumeBlock
		pvc.Spec.VolumeMode = &volumeMode
		pvc = f.CreateScheduledPVCFromDefinition(pvc)
		err = volumeImportSourceFunc(cdiv1.DataVolumeKubeVirt, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultPvcMountPath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(expectedMD5))
		By("Verifying the image is sparse")
		Expect(f.VerifySparse(f.Namespace, pvc, utils.DefaultPvcMountPath)).To(BeTrue())

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	},
		Entry("with HTTP image", utils.TinyCoreMD5, createHTTPImportPopulatorCR),
		Entry("with Registry image", utils.TinyCoreMD5, createRegistryImportPopulatorCR),
		Entry("with ImageIO image", Label("ImageIO"), Serial, utils.ImageioMD5, createImageIOImportPopulatorCR),
		Entry("with VDDK image", Label("VDDK"), utils.VcenterMD5, createVDDKImportPopulatorCR),
		Entry("with Blank image", utils.BlankMD5, createBlankImportPopulatorCR),
	)

	It("should import archive", func() {
		pvc = importPopulationPVCDefinition()
		pvc = f.CreateScheduledPVCFromDefinition(pvc)
		err = createHTTPImportPopulatorCR(cdiv1.DataVolumeArchive, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		same, err := f.VerifyTargetPVCArchiveContent(f.Namespace, pvc, "3")
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should handle static allocated PVC with import populator", func() {
		pvc = importPopulationPVCDefinition()
		pvc = f.CreateScheduledPVCFromDefinition(pvc)
		err = createHTTPImportPopulatorCR(cdiv1.DataVolumeKubeVirt, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		pvName := pvc.Spec.VolumeName

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))
		sourceMD5 := md5

		By("Retaining PV")
		Eventually(func() error {
			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			pv.Spec.PersistentVolumeReclaimPolicy = v1.PersistentVolumeReclaimRetain
			// We shouldn't make the test fail if there's a conflict with the update request.
			// These errors are usually transient and should be fixed in subsequent retries.
			_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
			return err
		}, timeout, pollingInterval).Should(Succeed())

		By("Forcing cleanup")
		err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())
		err = f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Delete(context.TODO(), "import-populator-test", metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())

		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		verifyCleanup(pvc)

		By("Making PV available")
		Eventually(func(g Gomega) bool {
			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(pv.Spec.ClaimRef.Namespace).To(Equal(pvc.Namespace))
			g.Expect(pv.Spec.ClaimRef.Name).To(Equal(pvc.Name))
			if pv.Status.Phase == v1.VolumeAvailable {
				return true
			}
			pv.Spec.ClaimRef.ResourceVersion = ""
			pv.Spec.ClaimRef.UID = ""
			_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
			// We shouldn't make the test fail if there's a conflict with the update request.
			// These errors are usually transient and should be fixed in subsequent retries.
			g.Expect(err).ToNot(HaveOccurred())
			return false
		}, timeout, pollingInterval).Should(BeTrue())

		// Start the whole process again, but with unscheduled PVC
		pvc = importPopulationPVCDefinition()
		pvc, err = f.CreatePVCFromDefinition(pvc)
		Expect(err).ToNot(HaveOccurred())
		err = createHTTPImportPopulatorCR(cdiv1.DataVolumeKubeVirt, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound to the expected PV")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Spec.VolumeName).To(Equal(pvName))

		pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(controller.IsPVBoundToPVC(pv, pvc)).To(BeTrue())
		Expect(pv.CreationTimestamp.Before(&pvc.CreationTimestamp)).To(BeTrue())

		By("Verify content")
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, sourceMD5, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue())
	})

	It("should import with ImmediateBinding requested", func() {
		pvc = importPopulationPVCDefinition()
		controller.AddAnnotation(pvc, controller.AnnImmediateBinding, "")
		pvc, err = f.CreatePVCFromDefinition(pvc)
		Expect(err).ToNot(HaveOccurred())
		err = createHTTPImportPopulatorCR(cdiv1.DataVolumeKubeVirt, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))

		By("Verifying the image is preallocated")
		ok, err := f.VerifyImagePreallocated(f.Namespace, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())

		if utils.DefaultStorageCSIRespectsFsGroup {
			// CSI storage class, it should respect fsGroup
			By("Checking that disk image group is qemu")
			Expect(f.GetDiskGroup(f.Namespace, pvc, false)).To(Equal("107"))
		}

		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should do multi-stage import with dataVolume populator flow", Label("VDDK"), func() {
		vcenterURL := fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs)
		dataVolume := f.CreateVddkWarmImportDataVolume("multi-stage-import-test", "100Mi", vcenterURL)
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verify pvc was created")
		pvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceSchedulingIfWaitForFirstConsumerPopulationPVC(pvc)

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")
	})

	It("Should update volumeImportSource accordingly when doing a multi-stage import", Label("VDDK"), func() {
		vcenterURL := fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs)
		dataVolume := f.CreateVddkWarmImportDataVolume("multi-stage-import-test", "100Mi", vcenterURL)

		// Set FinalCheckpoint to false to pause the DataVolume
		dataVolume.Spec.FinalCheckpoint = false
		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		volumeImportSourceName := fmt.Sprintf("%s-%s", "volume-import-source", dataVolume.UID)

		By("Verify pvc was created")
		pvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		By("Verify volumeimportSource")
		volumeImportSource, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Get(context.TODO(), volumeImportSourceName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(dataVolume.Spec.Checkpoints, volumeImportSource.Spec.Checkpoints)).To(BeTrue())

		By("Patch DataVolume checkpoints")
		dataVolume, err = f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		patch := `[{"op":"replace","path":"/spec/checkpoints","value":[{"current":"test","previous":"foo"},{"current":"foo","previous":"test"}]}]`
		dataVolume, err = f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Patch(context.TODO(), dataVolume.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Check volumeImportSource is also updated")
		Eventually(func() bool {
			volumeImportSource, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Get(context.TODO(), volumeImportSourceName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return reflect.DeepEqual(dataVolume.Spec.Checkpoints, volumeImportSource.Spec.Checkpoints)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should do multi-stage import with manually created volumeImportSource and PVC", Label("VDDK"), func() {
		pvcName := "multi-stage-import-pvc-test"
		importSourceName := "multi-stage-import-test"
		vcenterURL := fmt.Sprintf(utils.VcenterURL, f.CdiInstallNs)

		By(fmt.Sprintf("Create volumeImportSource %s", importSourceName))
		volumeImportSource := f.CreateVddkWarmImportPopulatorSource(importSourceName, pvcName, vcenterURL)
		_, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
			context.TODO(), volumeImportSource, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("Create PVC to be populated %s", pvcName))
		pvcDef := utils.NewPVCDefinition(pvcName, "1Gi", nil, nil)
		apiGroup := controller.AnnAPIGroup
		pvcDef.Spec.DataSourceRef = &v1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     cdiv1.VolumeImportSourceRef,
			Name:     importSourceName,
		}
		pvc = f.CreateScheduledPVCFromDefinition(pvcDef)

		By("Verify PVC prime was created")
		pvcPrime, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("should continue normally with the population even if the volueImportSource is deleted", func() {
		pvc = importPopulationPVCDefinition()
		controller.AddAnnotation(pvc, controller.AnnImmediateBinding, "")
		pvc, err = f.CreatePVCFromDefinition(pvc)
		Expect(err).ToNot(HaveOccurred())
		err = createHTTPImportPopulatorCR(cdiv1.DataVolumeKubeVirt, true)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		err = f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Delete(context.TODO(), "import-populator-test", metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Wait for PVC prime to be deleted")
		Eventually(func() bool {
			// Make sure pvcPrime was deleted after import population
			_, err := f.FindPVC(pvcPrime.Name)
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("should retain PVC Prime and importer pod with AnnPodRetainAfterCompletion", func() {
		dataVolume := utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify PVC prime was created")
		pvcPrime, err = utils.WaitForPVC(f.K8sClient, pvc.Namespace, populators.PVCPrimeName(pvc))
		Expect(err).ToNot(HaveOccurred())

		By("Verify target PVC is bound")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))

		By("Verify 100.0% annotation")
		progress, ok, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPopulatorProgress)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(progress).Should(BeEquivalentTo("100.0%"))

		By("Verify PVC Prime is Lost")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvcPrime.Namespace, v1.ClaimLost, pvcPrime.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Find importer pod after completion")
		importer, err := utils.FindPodByPrefixOnce(f.K8sClient, pvcPrime.Namespace, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).ToNot(HaveOccurred())
		Expect(importer.DeletionTimestamp).To(BeNil())

		By("Cleanup importer Pod, DataVolume and PVC Prime")
		zero := int64(0)
		err = utils.DeletePodByName(f.K8sClient, fmt.Sprintf("%s-%s", common.ImporterPodName, pvcPrime.Name), f.Namespace.Name, &zero)
		Expect(err).ToNot(HaveOccurred())
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		err = f.DeletePVC(pvcPrime)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should recreate and reimport pvc if it was deleted", func() {
		dataVolume := utils.NewDataVolumeWithHTTPImport("import-dv", "100Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dv.Namespace, cdiv1.Succeeded, dv.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Delete PVC and wait for it to be deleted")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		deleted, err := f.WaitPVCDeletedByUID(pvc, time.Minute)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())

		pvc, err = utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		By("Verify target PVC is bound again")
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(utils.TinyCoreMD5))
	})
})

var _ = Describe("Containerdisk envs to PVC labels", func() {
	f := framework.NewFramework(namespacePrefix)

	// The corresponding env var is defined in tests/BUILD.bazel (for pullMethod node)
	// and tools/cdi-func-test-registry-init/populate-registry.sh (for pullMethod pod).
	const (
		testKubevirtIoKey           = "test.kubevirt.io/test"
		testKubevirtIoValue         = "testvalue"
		testKubevirtIoKeyExisting   = "test.kubevirt.io/existing"
		testKubevirtIoValueExisting = "existing"
	)

	var (
		tinyCoreRegistryURL = func() string { return fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs) }
		trustedRegistryURL  = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix) }
		trustedRegistryIS   = func() string { return fmt.Sprintf(utils.TrustedRegistryIS, f.DockerPrefix) }
	)

	DescribeTable("Import should add KUBEVIRT_IO_ env vars to PVC labels when source is registry", func(pullMethod cdiv1.RegistryPullMethod, urlFn func() string, isImageStream bool) {
		dataVolume := utils.NewDataVolumeWithRegistryImport("import-dv", "100Mi", urlFn())
		// The existing key should not be overwritten
		dataVolume.ObjectMeta.Labels = map[string]string{
			testKubevirtIoKeyExisting: testKubevirtIoValueExisting,
		}

		if isImageStream {
			dataVolume.Spec.Source.Registry.URL = nil
			dataVolume.Spec.Source.Registry.ImageStream = ptr.To(urlFn())
			dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		}

		dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
		if pullMethod == cdiv1.RegistryPullPod {
			cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).ToNot(HaveOccurred())
			dataVolume.Spec.Source.Registry.CertConfigMap = &cm
		}

		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) map[string]string {
			pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			g.Expect(err).ToNot(HaveOccurred())
			return pvc.GetLabels()
		}, timeout, pollingInterval).Should(And(
			HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue),
			HaveKeyWithValue(testKubevirtIoKeyExisting, testKubevirtIoValueExisting),
		))
	},
		Entry("with pullMethod pod", cdiv1.RegistryPullPod, tinyCoreRegistryURL, false),
		Entry("with pullMethod node", cdiv1.RegistryPullNode, trustedRegistryURL, false),
		Entry("with pullMethod node", cdiv1.RegistryPullNode, trustedRegistryIS, true),
	)
})

var _ = Describe("Propagate DV Labels to Importer Pod", func() {
	f := framework.NewFramework(namespacePrefix)

	const (
		testKubevirtKey    = "test.kubevirt.io/test"
		testKubevirtValue  = "true"
		testNonKubevirtKey = "testLabel"
		testNonKubevirtVal = "none"
	)

	DescribeTable("Import pod should inherit any labels from Data Volume", func(usePopulator string) {

		dataVolume := utils.NewDataVolumeWithHTTPImport("label-test", "100Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
		dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
		dataVolume.Annotations[controller.AnnUsePopulator] = usePopulator

		dataVolume.Labels = map[string]string{
			testKubevirtKey:    testKubevirtValue,
			testNonKubevirtKey: testNonKubevirtVal,
		}

		By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verify pvc was created")
		_, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for import to be completed")
		err = utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Find importer pod")
		importer, err := utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).ToNot(HaveOccurred())

		By("Check labels were appended")
		importLabels := importer.GetLabels()
		Expect(importLabels).Should(HaveKeyWithValue(testKubevirtKey, testKubevirtValue))
		Expect(importLabels).Should(HaveKeyWithValue(testNonKubevirtKey, testNonKubevirtVal))

	},
		Entry("With Populators", "true"),
		Entry("Without Populators", "false"),
	)
})

var _ = Describe("pull image failure", func() {
	var (
		f = framework.NewFramework(namespacePrefix)
	)

	DescribeTable(`Should fail with "ImagePullFailed" reason if failed to pull image`, func(url string, pullMethod cdiv1.RegistryPullMethod) {
		dv := utils.NewDataVolumeWithRegistryImport("failed-to-pull-image", "10Gi", "docker://"+url)
		if dv.Annotations == nil {
			dv.Annotations = make(map[string]string)
		}
		dv.Annotations[controller.AnnImmediateBinding] = "true"
		dv.Spec.Source.Registry.PullMethod = &pullMethod

		var err error
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify ImagePullFailed running condition")
		runningCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeRunning,
			Status:  v1.ConditionFalse,
			Message: common.ImagePullFailureText,
			Reason:  "ImagePullFailed",
		}
		utils.WaitForConditions(f, dv.Name, f.Namespace.Name, controllerSkipPVCCompleteTimeout, assertionPollInterval, runningCondition)
	},
		Entry("pull method = pod", "myregistry/myorg/myimage:wrongtag", cdiv1.RegistryPullPod),
		Entry("pull method = node", "myregistry/myorg/myimage:wrongtag", cdiv1.RegistryPullNode),
	)
})

var _ = Describe("Multi-arch image pull", func() {
	const (
		errMessageArchitectureAbsent = "Unable to process data: Unable to transfer source data to scratch space: " +
			"Failed to read registry image: Error retrieving image: choosing image instance: " +
			`no image found in image index for architecture "absent", variant "", OS "linux"`
		errImporterPodUnschedulable = "Importer pod cannot be scheduled"
	)
	var (
		f                         = framework.NewFramework(namespacePrefix)
		tinyCoreMultiarchRegistry = func() string { return fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs) }
		trustedRegistryURL        = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix) }
	)

	It("Should succeed to pull multi-arch image matching architecture with pull method Pod", func() {
		dv := utils.NewDataVolumeWithRegistryImport("multi-arch-pull", "100Mi", tinyCoreMultiarchRegistry())
		if dv.Annotations == nil {
			dv.Annotations = make(map[string]string)
		}
		pullMethod := cdiv1.RegistryPullPod
		dv.Annotations[controller.AnnImmediateBinding] = "true"
		dv.Spec.Source.Registry.PullMethod = &pullMethod
		dv.Spec.Source.Registry.Platform = &cdiv1.PlatformOptions{Architecture: "amd64"}

		cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		dv.Spec.Source.Registry.CertConfigMap = &cm

		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())

		phase := cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, phase, dv.Name)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should fail to pull multi-arch image with absent architecture with pull method Pod", func() {
		dv := utils.NewDataVolumeWithRegistryImport("multi-arch-pull", "100Mi", tinyCoreMultiarchRegistry())
		if dv.Annotations == nil {
			dv.Annotations = make(map[string]string)
		}
		dv.Annotations[controller.AnnImmediateBinding] = "true"
		pullMethod := cdiv1.RegistryPullPod
		dv.Spec.Source.Registry.PullMethod = &pullMethod
		dv.Spec.Source.Registry.Platform = &cdiv1.PlatformOptions{Architecture: "absent"}

		cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		dv.Spec.Source.Registry.CertConfigMap = &cm

		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("Verify datavolume condition")
		runningCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeRunning,
			Status:  v1.ConditionFalse,
			Message: errMessageArchitectureAbsent,
			Reason:  "Error",
		}
		utils.WaitForConditions(f, dv.Name, f.Namespace.Name, controllerSkipPVCCompleteTimeout, assertionPollInterval, runningCondition)
	})

	It("Should put correct node selector for multi-arch image architecture with pull method Node", func() {
		dv := utils.NewDataVolumeWithRegistryImport("multi-arch-pull", "100Mi", trustedRegistryURL())
		if dv.Annotations == nil {
			dv.Annotations = make(map[string]string)
		}
		pullMethod := cdiv1.RegistryPullNode
		dv.Annotations[controller.AnnImmediateBinding] = "true"
		dv.Spec.Source.Registry.PullMethod = &pullMethod
		dv.Spec.Source.Registry.Platform = &cdiv1.PlatformOptions{Architecture: "absent"}

		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("Verify datavolume condition")
		runningCondition := &cdiv1.DataVolumeCondition{
			Type:    cdiv1.DataVolumeRunning,
			Status:  v1.ConditionFalse,
			Message: errImporterPodUnschedulable,
			Reason:  "Unschedulable",
		}
		utils.WaitForConditions(f, dv.Name, f.Namespace.Name, controllerSkipPVCCompleteTimeout, assertionPollInterval, runningCondition)

		importer, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred())
		Expect(importer.Spec.NodeSelector).To(HaveKeyWithValue(v1.LabelArchStable, "absent"))
	})
})

func generateRegistryOnlySidecar() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "Sidecar",
			"metadata": map[string]interface{}{
				"name": "registry-only-sidecar",
			},
			"spec": map[string]interface{}{
				"outboundTrafficPolicy": map[string]interface{}{
					"mode": "REGISTRY_ONLY",
				},
			},
		},
	}
}
