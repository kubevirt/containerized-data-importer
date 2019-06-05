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
	k8sv1 "k8s.io/api/storage/v1"

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
		err := f.WaitTimeoutForPodReady(utils.UploadPodName(pvc), time.Second*90)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says running")
		found, err := utils.WaitPVCPodStatusRunning(f.K8sClient, pvc)
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
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
			fileSize, err := f.RunCommandAndCaptureOutput(pvc, "stat -c \"%s\" /pvc/disk.img")
			Expect(err).ToNot(HaveOccurred())
			Expect(fileSize).To(Equal("1073741824")) // 1G
		} else {
			By("Verify PVC empty")
			_, err = framework.VerifyPVCIsEmpty(f, pvc)
			Expect(err).ToNot(HaveOccurred())
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

var _ = Describe("Block PV upload Test", func() {
	var (
		pvc *v1.PersistentVolumeClaim
		err error

		uploadProxyURL string
		portForwardCmd *exec.Cmd

		pv           *v1.PersistentVolume
		storageClass *k8sv1.StorageClass
	)

	f := framework.NewFrameworkOrDie(namespacePrefix)

	BeforeEach(func() {
		err := f.ClearBlockPV()
		Expect(err).NotTo(HaveOccurred())

		pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "cdi-block-device", "kubevirt.io=cdi-block-device")
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get pod %q", f.CdiInstallNs+"/"+"cdi-block-device"))

		nodeName := pod.Spec.NodeName

		By(fmt.Sprintf("Creating storageClass for Block PV"))
		storageClass, err = f.CreateStorageClassFromDefinition(utils.NewStorageClassForBlockPVDefinition("manual"))
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("Creating Block PV"))
		pv, err = f.CreatePVFromDefinition(utils.NewBlockPVDefinition("local-volume", "500M", nil, "manual", nodeName))
		Expect(err).ToNot(HaveOccurred())

		By("Verify that PV's phase is Available")
		err = f.WaitTimeoutForPVReady(pv.Name, 60*time.Second)
		Expect(err).ToNot(HaveOccurred())

		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}

		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadBlockPVCDefinition())
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

		if pv != nil {
			By("Delete PV for block PV")
			err := utils.DeletePV(f.K8sClient, pv)
			Expect(err).ToNot(HaveOccurred())
		}

		if storageClass != nil {
			By("Delete storageClass for block PV")
			err = utils.DeleteStorageClass(f.K8sClient, storageClass)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	table.DescribeTable("should", func(validToken bool, expectedStatus int) {
		By("Verify that upload server POD running")
		err := f.WaitTimeoutForPodReady(utils.UploadPodName(pvc), time.Second*90)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says running")
		found, err := utils.WaitPVCPodStatusRunning(f.K8sClient, pvc)
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

			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultPvcMountPath, utils.UploadBlockDeviceMD5)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
			fileSize, err := f.RunCommandAndCaptureOutput(pvc, "lsblk -n -b -o SIZE /pvc")
			Expect(err).ToNot(HaveOccurred())
			Expect(fileSize).To(Equal("524288000")) // 500M
		} else {
			By("Verify PVC empty")
			_, err = framework.VerifyPVCIsEmpty(f, pvc)
			Expect(err).ToNot(HaveOccurred())
		}
	},
		table.Entry("[test_id:1368]succeed given a valid token", true, http.StatusOK),
		table.Entry("[posneg:negative][test_id:1369]fail given an invalid token", false, http.StatusUnauthorized),
	)
})
