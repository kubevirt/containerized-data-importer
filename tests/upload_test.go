package tests_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("[rfe_id:138][crit:high][vendor:cnv-qe@redhat.com][level:component]Upload tests", func() {

	var (
		pvc *v1.PersistentVolumeClaim
		err error

		uploadProxyURL string
		portForwardCmd *exec.Cmd
	)

	f := framework.NewFrameworkOrDie("upload-func-test")

	BeforeEach(func() {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Set up port forwarding")
		uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("Stop port forwarding")
		if portForwardCmd != nil {
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}

		By("Delete upload PVC")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	DescribeTable("should", func(validToken bool, expectedStatus int) {
		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		var token string
		if validToken {
			By("Get an upload token")
			token, err = utils.RequestUploadToken(f.CdiClient, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())
		} else {
			token = "abc"
		}

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, expectedStatus)
		Expect(err).ToNot(HaveOccurred())

		if validToken {
			By("Verify PVC status annotation says succeeded")
			found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			By("Verify content")
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5100kbytes, 100000)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
			By("Verifying the image is sparse")
			Expect(f.VerifySparse(f.Namespace, pvc)).To(BeTrue())
			if utils.DefaultStorageCSI {
				// CSI storage class, it should respect fsGroup
				By("Checking that disk image group is qemu")
				Expect(f.GetDiskGroup(f.Namespace, pvc)).To(Equal("107"))
			}
		} else {
			uploader, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, utils.UploadPodName(pvc), common.CDILabelSelector)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get uploader pod %q", f.Namespace.Name+"/"+utils.UploadPodName(pvc)))
			By("Verifying PVC is empty")
			By(fmt.Sprintf("uploader.Spec.NodeName %q", uploader.Spec.NodeName))
			By("Verify PVC empty")
			_, err = framework.VerifyPVCIsEmpty(f, pvc, uploader.Spec.NodeName)
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("[test_id:1368]succeed given a valid token", true, http.StatusOK),
		Entry("[posneg:negative][test_id:1369]fail given an invalid token", false, http.StatusUnauthorized),
	)
	It("Verify upload to the same pvc fails", func() {
		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		var token string
		By("Get an upload token")
		token, err = utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, http.StatusOK)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says succeeded")
		found, err = utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Try upload again")
		err = uploadImage(uploadProxyURL, token, http.StatusServiceUnavailable)
		Expect(err).ToNot(HaveOccurred())

	})
})

func startUploadProxyPortForward(f *framework.Framework) (string, *exec.Cmd, error) {
	lp := "18443"
	pm := lp + ":443"
	url := "https://127.0.0.1:" + lp

	cmd := tests.CreateKubectlCommand(f, "-n", f.CdiInstallNs, "port-forward", "svc/cdi-uploadproxy", pm)
	err := cmd.Start()
	if err != nil {
		return "", nil, err
	}

	return url, cmd, nil
}

func uploadImage(portForwardURL, token string, expectedStatus int) error {
	url := portForwardURL + "/v1alpha1/upload"

	f, err := os.Open(utils.UploadFile)
	if err != nil {
		return err
	}
	defer f.Close()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, f)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("Unexpected return value %d expected %d", resp.StatusCode, expectedStatus)
	}

	return nil
}

var _ = Describe("Block PV upload Test", func() {
	var (
		pvc *v1.PersistentVolumeClaim
		err error

		uploadProxyURL string
		portForwardCmd *exec.Cmd
	)

	f := framework.NewFrameworkOrDie(namespacePrefix)

	BeforeEach(func() {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}

		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadBlockPVCDefinition(f.BlockSCName))
		Expect(err).ToNot(HaveOccurred())

		By("Set up port forwarding")
		uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("Stop port forwarding")
		if portForwardCmd != nil {
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}

		By("Delete upload PVC")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	DescribeTable("should", func(validToken bool, expectedStatus int) {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}

		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		var token string
		if validToken {
			By("Get an upload token")
			token, err = utils.RequestUploadToken(f.CdiClient, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())
		} else {
			token = "abc"
		}

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, expectedStatus)
		Expect(err).ToNot(HaveOccurred())

		if validToken {
			By("Verify PVC status annotation says succeeded")
			found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultPvcMountPath, utils.UploadFileMD5, utils.UploadFileSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		} else {
			// TODO framework.VerifyPVCIsEmpty doesn't make sense for block devices
			//By("Verify PVC empty")
			//_, err = framework.VerifyPVCIsEmpty(f, pvc)
			//Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("[test_id:1368]succeed given a valid token (block)", true, http.StatusOK),
		Entry("[posneg:negative][test_id:1369]fail given an invalid token (block)", false, http.StatusUnauthorized),
	)
})

var _ = Describe("Namespace with quota", func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)
	var (
		orgConfig      *v1.ResourceRequirements
		pvc            *v1.PersistentVolumeClaim
		portForwardCmd *exec.Cmd
	)

	BeforeEach(func() {
		By("Capturing original CDIConfig state")
		config, err := f.CdiClient.CdiV1alpha1().CDIConfigs().Get(common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		orgConfig = config.Status.DefaultPodResourceRequirements
		if pvc != nil {
			By("Making sure no pvc exists")
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}
	})

	AfterEach(func() {
		By("Restoring CDIConfig to original state")
		reqCpu, _ := orgConfig.Requests.Cpu().AsInt64()
		reqMem, _ := orgConfig.Requests.Memory().AsInt64()
		limCpu, _ := orgConfig.Limits.Cpu().AsInt64()
		limMem, _ := orgConfig.Limits.Memory().AsInt64()
		err := f.UpdateCdiConfigResourceLimits(reqCpu, reqMem, limCpu, limMem)
		Expect(err).ToNot(HaveOccurred())
		By("Stop port forwarding")
		if portForwardCmd != nil {
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}

		By("Delete upload PVC")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	It("Should create upload pod in namespace with quota", func() {
		err := f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())
	})

	It("Should fail to create upload pod in namespace with quota, when pods have higher requirements", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(2), int64(1024*1024*1024), int64(2), int64(1*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify Quota was exceeded in logs")
		matchString := fmt.Sprintf("pods \\\"cdi-upload-upload-test\\\" is forbidden: exceeded quota: test-quota, requested")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return log
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
	})

	It("Should fail to create upload pod in namespace with quota, and recover when quota fixed", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(2), int64(1024*1024*1024), int64(2), int64(1*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify Quota was exceeded in logs")
		matchString := fmt.Sprintf("pods \\\"cdi-upload-upload-test\\\" is forbidden: exceeded quota: test-quota, requested")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return log
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
		By("Updating the quota to be enough")
		err = f.UpdateQuotaInNs(int64(2), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())
	})

	It("Should create upload pod in namespace with quota and pods limits are low enough", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(0), int64(0), int64(1), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())
	})
})
