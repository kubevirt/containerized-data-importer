package tests_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo/extensions/table"

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

	table.DescribeTable("should", func(validToken bool, expectedStatus int) {

		By("Verify that upload server POD running")
		err := f.WaitTimeoutForPodReady(utils.UploadPodName(pvc), time.Second*20)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says running")
		found, err := utils.WaitPVCUploadPodStatusRunning(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		err = f.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		if !validToken {
			token = "abc"
		}

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, expectedStatus)
		Expect(err).ToNot(HaveOccurred())

		if !validToken {
			By("Get an error while verifying content")
			_, err = f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
			Expect(err).To(HaveOccurred())
		} else {
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(utils.UploadPodName(pvc), metav1.GetOptions{})
				if err != nil && k8serrors.IsNotFound(err) {
					return false
				}
				return true
			}, 90*time.Second, 2*time.Second).Should(BeFalse())
			By("Verify content")
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
			fileSize, err := f.RunCommandAndCaptureOutput(pvc, "stat -c \"%s\" /pvc/disk.img")
			Expect(err).ToNot(HaveOccurred())
			Expect(fileSize).To(Equal("1073741824")) // 1G
		}
	},
		table.Entry("[test_id:1368]succeed given a valid token", true, http.StatusOK),
		table.Entry("[posneg:negative][test_id:1369]fail given an invalid token", false, http.StatusUnauthorized),
	)
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

	f, err := os.Open("./images/cirros-qcow2.img")
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
