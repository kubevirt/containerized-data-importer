package tests_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Alpha API tests", func() {
	f := framework.NewFrameworkOrDie("alpha-api-test")

	Context("with v1alpha1 api", func() {

		It("should", func() {
			By("create a upload DataVolume")
			out, err := tests.RunKubectlCommand(f, "create", "-f", "manifests/dvAlphaUpload.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for DataVolume to be ready")
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.UploadReady, "upload")

			By("create a upload token")
			out, err = tests.RunKubectlCommand(f, "create", "-f", "manifests/tokenAlpha.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("and kubectl proxy", func() {
			var proxyCmd *exec.Cmd
			var proxyUrl string

			BeforeEach(func() {
				proxyUrl, proxyCmd = startKubeProxy(f)
			})

			AfterEach(func() {
				if proxyCmd != nil {
					proxyCmd.Process.Kill()
					proxyCmd.Wait()
				}
			})

			getResource := func(path string) {
				resp, err := http.Get(proxyUrl + path)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			}

			It("should", func() {
				By("get CDIConfig")
				getResource("/apis/cdi.kubevirt.io/v1alpha1/cdiconfigs/config")

				By("get CDI")
				getResource("/apis/cdi.kubevirt.io/v1alpha1/cdis/cdi")
			})
		})
	})
})

func startKubeProxy(f *framework.Framework) (string, *exec.Cmd) {
	port := "18443"
	url := "http://127.0.0.1:" + port

	cmd := tests.CreateKubectlCommand(f, "proxy", "--port="+port)
	err := cmd.Start()
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() error {
		resp, err := http.Get(url + "/apis")
		if err != nil {
			return err
		}
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		return nil
	}, 30*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

	return url, cmd
}
