package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	cdiDeploymentPodPrefix  = "cdi-deployment-"
	cdiAPIServerPodPrefix   = "cdi-apiserver-"
	cdiUploadProxyPodPrefix = "cdi-uploadproxy-"

	pollingInterval = 2 * time.Second
	timeout         = 360 * time.Second
)

var _ = Describe("cdi-apiserver tests", Serial, func() {
	var origSpec *cdiv1.CDIConfigSpec
	f := framework.NewFramework("cdi-apiserver-test")

	Context("with apiserver", func() {
		var cmd *exec.Cmd
		AfterEach(func() {
			afterCMD(cmd)
		})

		It("should serve an openapi spec", func() {
			var (
				err      error
				hostPort string
			)
			hostPort, cmd, err = startServicePortForward(f, "cdi-api")
			Expect(err).ToNot(HaveOccurred())

			url := fmt.Sprintf("https://%s/openapi/v2", hostPort)

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}

			Eventually(func() error {
				req, err := http.NewRequest("GET", url, nil)
				Expect(err).ToNot(HaveOccurred())

				resp, err := client.Do(req)
				if err != nil {
					return err
				}

				Expect(resp.StatusCode).To(Equal(200))

				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

	Context("with TLS profile configured", func() {

		BeforeEach(func() {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			origSpec = config.Spec.DeepCopy()
		})

		var cmd *exec.Cmd

		AfterEach(func() {
			afterCMD(cmd)

			By("Restoring CDIConfig to original state")
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				origSpec.DeepCopyInto(config)
			})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return apiequality.Semantic.DeepEqual(config.Spec, *origSpec)
			}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not properly restored to original value")

			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return !apiequality.Semantic.DeepEqual(config.Status, cdiv1.CDIConfigStatus{})
			}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig status not restored by config controller")
		})

		It("[test_id:9062]should fail reaching server when TLS profile requires minimal TLS version higher than our client's", func() {
			Expect(utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				config.TLSSecurityProfile = &cdiv1.TLSSecurityProfile{
					// Modern profile requires TLS 1.3
					// https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
					Type:   cdiv1.TLSProfileModernType,
					Modern: &cdiv1.ModernTLSProfile{},
				}
			})).To(Succeed())

			var (
				err      error
				hostPort string
			)
			hostPort, cmd, err = startServicePortForward(f, "cdi-api")
			Expect(err).ToNot(HaveOccurred())

			url := fmt.Sprintf("https://%s/healthz", hostPort)
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						MinVersion:         tls.VersionTLS12,
						MaxVersion:         tls.VersionTLS12,
					},
				},
			}
			requestFunc := func() string {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					return err.Error()
				}
				resp, err := client.Do(req)
				if err != nil {
					return err.Error()
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Sprintf("Unexpected status code %d", resp.StatusCode)
				}
				return "success"
			}
			Eventually(requestFunc, 10*time.Second, 1*time.Second).Should(ContainSubstring("protocol version not supported"))

			// Change to intermediate, which is fine with 1.2, expect success
			err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				config.TLSSecurityProfile = &cdiv1.TLSSecurityProfile{
					// Intermediate profile requires TLS 1.2
					// https://wiki.mozilla.org/Security/Server_Side_TLS#Intermediate_compatibility_.28recommended.29
					Type:         cdiv1.TLSProfileIntermediateType,
					Intermediate: &cdiv1.IntermediateTLSProfile{},
				}
			})
			Expect(err).ToNot(HaveOccurred())
			Eventually(requestFunc, 10*time.Second, 1*time.Second).Should(Equal("success"))
		})
	})
})
