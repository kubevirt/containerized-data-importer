package tests_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	syncUploadPath  = "/v1beta1/upload"
	asyncUploadPath = "/v1beta1/upload-async"

	syncFormPath  = "/v1beta1/upload-form"
	asyncFormPath = "/v1beta1/upload-form-async"

	alphaSyncUploadPath  = "/v1alpha1/upload"
	alphaAsyncUploadPath = "/v1alpha1/upload-async"
)

type uploadFunc func(string, string, int) error

type uploadFileNameRequestCreator func(string, string) (*http.Request, error)

var _ = Describe("[rfe_id:138][crit:high][vendor:cnv-qe@redhat.com][level:component]Upload tests", func() {

	var (
		pvc *v1.PersistentVolumeClaim
		err error

		uploadProxyURL string
		portForwardCmd *exec.Cmd
	)

	f := framework.NewFramework("upload-func-test")

	BeforeEach(func() {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		uploadProxyURL = findProxyURLCdiConfig(f)
		if uploadProxyURL == "" {
			By("Set up port forwarding")
			uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
			Expect(err).ToNot(HaveOccurred())
		}
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

	DescribeTable("should", func(uploader uploadFunc, validToken bool, expectedStatus int) {
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
		Eventually(func() error {
			return uploader(uploadProxyURL, token, expectedStatus)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

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
				Expect(f.GetDiskGroup(f.Namespace, pvc, false)).To(Equal("107"))
			}
			By("Verifying permissions are 660")
			Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")
		} else {
			uploadPod, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, utils.UploadPodName(pvc), common.CDILabelSelector)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get uploader pod %q", f.Namespace.Name+"/"+utils.UploadPodName(pvc)))

			pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			delete(pvc.Annotations, controller.AnnUploadRequest)
			pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				_, err = f.K8sClient.CoreV1().Pods(uploadPod.Namespace).Get(context.TODO(), uploadPod.Name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					return true
				}
				Expect(err).ToNot(HaveOccurred())
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			By("Verify PVC empty")
			_, err = framework.VerifyPVCIsEmpty(f, pvc, "")
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("[test_id:1368]succeed given a valid token", uploadImage, true, http.StatusOK),
		Entry("[test_id:5078]succeed given a valid token (async)", uploadImageAsync, true, http.StatusOK),
		Entry("[test_id:5079]succeed given a valid token (alpha)", uploadImageAlpha, true, http.StatusOK),
		Entry("[test_id:5080]succeed given a valid token (async alpha)", uploadImageAsyncAlpha, true, http.StatusOK),
		Entry("[test_id:5081]succeed given a valid token (form)", uploadForm, true, http.StatusOK),
		Entry("[test_id:5082]succeed given a valid token (form async)", uploadFormAsync, true, http.StatusOK),
		Entry("[posneg:negative][test_id:1369]fail given an invalid token", uploadImage, false, http.StatusUnauthorized),
	)

	It("[test_id:4988]Verify upload to the same pvc fails", func() {
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
		Eventually(func() error {
			return uploadImage(uploadProxyURL, token, http.StatusOK)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

		By("Verify PVC status annotation says succeeded")
		found, err = utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		By("Verifying permissions are 660")
		Expect(f.VerifyPermissions(f.Namespace, pvc)).To(BeTrue(), "Permissions on disk image are not 660")

		By("Try upload again")
		err = uploadImage(uploadProxyURL, token, http.StatusServiceUnavailable)
		Expect(err).ToNot(HaveOccurred())

	})

	DescribeTable("Verify validation error message on async upload if virtual size > pvc size", func(filename string) {
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
		Eventually(func() error {
			return uploadFileNameToPath(binaryRequestFunc, filename, uploadProxyURL, asyncUploadPath, token, http.StatusBadRequest)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")
	},
		Entry("[test_id:4989]fail given a large virtual size QCOW2 file", utils.UploadFileLargeVirtualDiskQcow),
		Entry("fail given a large physical size QCOW2 file", utils.UploadFileLargePhysicalDiskQcow),
	)

	DescribeTable("[posneg:negative][test_id:2330]Verify failure on sync upload if virtual size > pvc size", func(filename string) {
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
		err = uploadFileNameToPath(binaryRequestFunc, filename, uploadProxyURL, syncUploadPath, token, http.StatusOK)
		Expect(err).To(HaveOccurred())

		uploadPod, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, utils.UploadPodName(pvc), common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get uploader pod %q", f.Namespace.Name+"/"+utils.UploadPodName(pvc)))

		By("Verify size error in logs")
		Eventually(func() bool {
			log, _ := tests.RunKubectlCommand(f, "logs", uploadPod.Name, "-n", uploadPod.Namespace)
			if strings.Contains(log, "is larger than available size") {
				return true
			}
			if strings.Contains(log, "no space left on device") {
				return true
			}
			if strings.Contains(log, "qemu-img execution failed") {
				return true
			}
			By("Failed to find error messages about a too large image in log:")
			By(log)
			return false
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	},
		Entry("fail given a large virtual size RAW XZ file", utils.UploadFileLargeVirtualDiskXz),
		Entry("fail given a large virtual size QCOW2 file", utils.UploadFileLargeVirtualDiskQcow),
		Entry("fail given a large physical size RAW XZ file", utils.UploadFileLargePhysicalDiskXz),
		Entry("fail given a large physical size QCOW2 file", utils.UploadFileLargePhysicalDiskQcow),
	)
})

var TestFakeError = errors.New("TestFakeError")

// LimitThenErrorReader returns a Reader that reads from r
// but stops with FakeError after n bytes.
// Based on io.LimitReader
func LimitThenErrorReader(r io.Reader, n int64) io.Reader { return &limitThenErrorReader{r, n} }

// A limitThenErrorReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns ERR when N <= 0.
type limitThenErrorReader struct {
	r io.Reader // underlying reader
	n int64     // max bytes remaining
}

func (l *limitThenErrorReader) Read(p []byte) (n int, err error) {
	if l.n <= 0 {
		return 0, TestFakeError // EOF
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err = l.r.Read(p)
	l.n -= int64(n)
	return
}

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

func formRequestFunc(url, fileName string) (*http.Request, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)

	req, err := http.NewRequest("POST", url, pipeReader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", multipartWriter.FormDataContentType())

	go func() {
		defer GinkgoRecover()
		defer f.Close()
		defer pipeWriter.Close()

		formFile, err := multipartWriter.CreateFormFile("file", utils.UploadFile)
		Expect(err).ToNot(HaveOccurred())

		_, err = io.Copy(formFile, f)
		Expect(err).ToNot(HaveOccurred())

		err = multipartWriter.Close()
		Expect(err).ToNot(HaveOccurred())
	}()

	return req, nil
}

func binaryRequestFunc(url, fileName string) (*http.Request, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, f)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")

	return req, nil
}

func testBadRequestFunc(url, fileName string) (*http.Request, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	lr := LimitThenErrorReader(f, 2048)
	req, err := http.NewRequest("POST", url, lr)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/octet-stream")

	return req, nil
}

func uploadImage(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(binaryRequestFunc, utils.UploadFile, portForwardURL, syncUploadPath, token, expectedStatus)
}

func uploadImageAsync(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(binaryRequestFunc, utils.UploadFile, portForwardURL, asyncUploadPath, token, expectedStatus)
}

func uploadImageAlpha(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(binaryRequestFunc, utils.UploadFile, portForwardURL, alphaSyncUploadPath, token, expectedStatus)
}

func uploadImageAsyncAlpha(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(binaryRequestFunc, utils.UploadFile, portForwardURL, alphaAsyncUploadPath, token, expectedStatus)
}

func uploadForm(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(formRequestFunc, utils.UploadFile, portForwardURL, syncFormPath, token, expectedStatus)
}

func uploadFormAsync(portForwardURL, token string, expectedStatus int) error {
	return uploadFileNameToPath(formRequestFunc, utils.UploadFile, portForwardURL, syncFormPath, token, expectedStatus)
}

func uploadFileNameToPath(requestFunc uploadFileNameRequestCreator, fileName, portForwardURL, path, token string, expectedStatus int) error {
	url := portForwardURL + path

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := requestFunc(url, fileName)
	if err != nil {
		return err
	}
	defer req.Body.Close()

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Origin", "foo.bar.com")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("Unexpected return value %d expected %d, Response: %s", resp.StatusCode, expectedStatus, resp.Body)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		return fmt.Errorf("Auth response header missing")
	}

	return nil
}

func findProxyURLCdiConfig(f *framework.Framework) string {
	config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	if config.Status.UploadProxyURL != nil {
		return fmt.Sprintf("https://%s", *config.Status.UploadProxyURL)
	}
	return ""
}

var _ = Describe("Block PV upload Test", func() {
	var (
		pvc *v1.PersistentVolumeClaim
		err error

		uploadProxyURL string
		portForwardCmd *exec.Cmd
	)

	f := framework.NewFramework(namespacePrefix)

	BeforeEach(func() {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}

		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadBlockPVCDefinition(f.BlockSCName))
		Expect(err).ToNot(HaveOccurred())

		uploadProxyURL = findProxyURLCdiConfig(f)
		if uploadProxyURL == "" {
			By("Set up port forwarding")
			uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
			Expect(err).ToNot(HaveOccurred())
		}
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
		Eventually(func() error {
			return uploadImage(uploadProxyURL, token, expectedStatus)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

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

var _ = Describe("CDIConfig manipulation upload tests", func() {
	f := framework.NewFramework(namespacePrefix)
	var (
		origSpec       *cdiv1.CDIConfigSpec
		pvc            *v1.PersistentVolumeClaim
		portForwardCmd *exec.Cmd
		uploadProxyURL string
	)

	BeforeEach(func() {
		By("Capturing original CDIConfig state")
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		origSpec = config.Spec.DeepCopy()
		if pvc != nil {
			By("Making sure no pvc exists")
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}

		uploadProxyURL = findProxyURLCdiConfig(f)
		if uploadProxyURL == "" {
			By("Set up port forwarding")
			uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	AfterEach(func() {
		By("Restoring CDIConfig to original state")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			origSpec.DeepCopyInto(config)
		})
		Eventually(func() bool {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return apiequality.Semantic.DeepEqual(config.Spec, *origSpec)
		}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not properly restored to original value")

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

		By("Waiting for PVC to be deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
			return k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	It("[test_id:4990]Should create upload pod in namespace with quota", func() {
		err := f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
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

	It("[test_id:4991]Should fail to create upload pod in namespace with quota, when pods have higher requirements", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(2), int64(1024*1024*1024), int64(2), int64(1*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify Quota was exceeded in logs")
		matchString := fmt.Sprintf("pods \\\"cdi-upload-upload-test\\\" is forbidden: exceeded quota: test-quota, requested")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return log
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
	})

	It("[test_id:4992]Should fail to create upload pod in namespace with quota, and recover when quota fixed", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(0), int64(512*1024*1024), int64(2), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(256*1024*1024), int64(2), int64(256*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify Quota was exceeded in logs")
		matchString := fmt.Sprintf("pods \\\"cdi-upload-upload-test\\\" is forbidden: exceeded quota: test-quota, requested")
		Eventually(func() string {
			log, err := tests.RunKubectlCommand(f, "logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
			Expect(err).NotTo(HaveOccurred())
			return log
		}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))
		By("Updating the quota to be enough")
		err = f.UpdateQuotaInNs(int64(2), int64(512*1024*1024), int64(2), int64(1024*1024*1024))
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

	It("[test_id:4993]Should create upload pod in namespace with quota and pods limits are low enough", func() {
		err := f.UpdateCdiConfigResourceLimits(int64(0), int64(0), int64(1), int64(512*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
		Expect(err).ToNot(HaveOccurred())
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
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

	DescribeTable("Async upload with filesystem overhead", func(expectedStatus int, globalOverhead, scOverhead string) {
		defaultSCName := utils.DefaultStorageClass.GetName()
		testedFilesystemOverhead := &cdiv1.FilesystemOverhead{}
		if globalOverhead != "" {
			testedFilesystemOverhead.Global = cdiv1.Percent(globalOverhead)
		}
		if scOverhead != "" {
			testedFilesystemOverhead.StorageClass = map[string]cdiv1.Percent{defaultSCName: cdiv1.Percent(scOverhead)}
		}
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

		By("Creating PVC with upload target annotation")
		pvc, err = f.CreateBoundPVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC annotation says ready")
		found, err := utils.WaitPVCPodStatusReady(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		Eventually(func() error {
			return uploadImageAsync(uploadProxyURL, token, expectedStatus)
		}, timeout, pollingInterval).Should(BeNil(), "uploadImageAsync should return nil, even if not ready")
	},
		Entry("[test_id:4548] Succeed with low global overhead", http.StatusOK, "0.1", ""),
		Entry("[test_id:4672][posneg:negative] Fail with high global overhead", http.StatusBadRequest, "0.99", ""),
		Entry("[test_id:4673] Succeed with low per-storageclass overhead (despite high global overhead)", http.StatusOK, "0.99", "0.1"),
		Entry("[test_id:4714][posneg:negative] Fail with high per-storageclass overhead (despite low global overhead)", http.StatusBadRequest, "0.1", "0.99"),
	)
})

var _ = Describe("[rfe_id:138][crit:high][vendor:cnv-qe@redhat.com][level:component] Upload tests", func() {
	f := framework.NewFramework("upload-func-test")

	var (
		pvc        *v1.PersistentVolumeClaim
		dataVolume *cdiv1.DataVolume
		err        error

		uploadProxyURL string
		portForwardCmd *exec.Cmd
		errAsString    = func(e error) string { return e.Error() }
	)

	BeforeEach(func() {
		uploadProxyURL = findProxyURLCdiConfig(f)
		if uploadProxyURL == "" {
			By("Set up port forwarding")
			uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	AfterEach(func() {
		By("Stop port forwarding")
		if portForwardCmd != nil {
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}

		By("Delete upload DV")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	It("[test_id:3993] Upload image to data volume and verify retry count", func() {
		dvName := "upload-dv"
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeForUpload(dvName, "100Mi")
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		pvc = utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		Eventually(func() error {
			return uploadImage(uploadProxyURL, token, http.StatusOK)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

		phase = cdiv1.Succeeded
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
		}, timeout, pollingInterval).Should(BeNumerically("==", 0))

		By("Verify the number of retries on the datavolume")
		Eventually(func() int32 {
			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			restarts := dv.Status.RestartCount
			return restarts
		}, timeout, pollingInterval).Should(BeNumerically("==", 0))

	})

	It("[test_id:3997] Upload image to data volume - kill container and verify retry count", func() {
		dvName := "upload-dv"
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeForUpload(dvName, "100Mi")
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		pvc = utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Kill upload pod to force error")
		// exit code 137 = 128 + 9, it means parent process issued kill -9, in our case it is not a problem
		_, _, err = f.ExecShellInPod(utils.UploadPodName(pvc), f.Namespace.Name, "kill 1")
		Expect(err).To(Or(
			BeNil(),
			WithTransform(errAsString, ContainSubstring("137"))))

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

	DescribeTable("Upload datavolume creates correct scratch space, pod and service names", func(dvName string) {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))

		dv := utils.NewDataVolumeForUpload(dvName, "1Gi")
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())
		pvc = utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		Eventually(func() error {
			return uploadImage(uploadProxyURL, token, http.StatusOK)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

		phase = cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("[test_id:4273] with short DataVolume name", "import-long-name-dv"),
		Entry("[test_id:4274] with long DataVolume name", "import-long-name-dv-"+
			"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-"+
			"123456789-123456789-123456789-1234567890"),
		Entry("[test_id:4275] with long DataVolume name including special chars '.'",
			"import-long-name-dv."+
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-"+
				"123456789-123456789-123456789-1234567890"),
	)

	It("[test_id:1985] Upload datavolume should succeed on retry after failure", func() {
		shortDvName := "upload-after-fail-1985"
		By(fmt.Sprintf("Creating new datavolume %s", shortDvName))

		By("Create DV")
		dv := utils.NewDataVolumeForUpload(shortDvName, "1Gi")
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		pvc = utils.PersistentVolumeClaimFromDataVolume(dataVolume)
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload - expecting failure")
		err = uploadFileNameToPath(testBadRequestFunc, utils.UploadFile, uploadProxyURL, syncUploadPath, token, http.StatusOK)
		Expect(err).To(HaveOccurred())

		phase = cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Retry Upload")
		Eventually(func() error {
			return uploadFileNameToPath(binaryRequestFunc, utils.UploadFile, uploadProxyURL, syncUploadPath, token, http.StatusOK)
		}, timeout, pollingInterval).Should(BeNil(), "uploadFileNameToPath should return nil, even if not ready")

		phase = cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says succeeded")
		found, err := utils.WaitPVCPodStatusSucceeded(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5100kbytes, 100000)
		Expect(err).ToNot(HaveOccurred())
		Expect(same).To(BeTrue(), "MD5 does not match")
	})
})

var _ = Describe("Preallocation", func() {
	f := framework.NewFramework(namespacePrefix)
	dvName := "upload-dv"

	var (
		dataVolume     *cdiv1.DataVolume
		err            error
		uploadProxyURL string
		portForwardCmd *exec.Cmd
	)

	BeforeEach(func() {
		uploadProxyURL = findProxyURLCdiConfig(f)
		if uploadProxyURL == "" {
			By("Set up port forwarding")
			uploadProxyURL, portForwardCmd, err = startUploadProxyPortForward(f)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	AfterEach(func() {
		if portForwardCmd != nil {
			By("Delete port forward")
			err = portForwardCmd.Process.Kill()
			Expect(err).ToNot(HaveOccurred())
			portForwardCmd.Wait()
			portForwardCmd = nil
		}

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
	})

	It("Uploader should add preallocation when requested", func() {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeForUpload(dvName, "100Mi")
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		pvc := utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, http.StatusOK)
		Expect(err).ToNot(HaveOccurred())

		phase = cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Or(Equal("true"), Equal("skipped")))
	})

	It("Uploader should not add preallocation when preallocation=false", func() {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeForUpload(dvName, "100Mi")
		preallocation := false
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		pvc := utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		Eventually(func() error {
			return uploadImage(uploadProxyURL, token, http.StatusOK)
		}, timeout, pollingInterval).Should(BeNil(), "Upload should eventually succeed, even if initially pod is not ready")

		phase = cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).ShouldNot(Equal("true"))
	})

	DescribeTable("Each upload path include preallocation/conversion", func(uploader uploadFunc) {
		By(fmt.Sprintf("Creating new datavolume %s", dvName))
		dv := utils.NewDataVolumeForUpload(dvName, "100Mi")
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		pvc := utils.PersistentVolumeClaimFromDataVolume(dataVolume)

		By("verifying pvc was created, force bind if needed")
		pvc, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, pvc.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

		phase := cdiv1.UploadReady
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		if err != nil {
			dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			if dverr != nil {
				Fail(fmt.Sprintf("datavolume %s phase %s", dv.Name, dv.Status.Phase))
			}
		}
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		By("Do upload")
		err = uploadImage(uploadProxyURL, token, http.StatusOK)
		Expect(err).ToNot(HaveOccurred())

		phase = cdiv1.Succeeded
		By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(phase)))
		err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, phase, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		pvc, err = utils.FindPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.GetAnnotations()[controller.AnnPreallocationApplied]).Should(Or(Equal("true"), Equal("skipped")))
	},
		Entry("sync", uploadImage),
		Entry("async", uploadImageAsync),
		Entry("form sync", uploadForm),
		Entry("form async", uploadFormAsync),
	)
})
