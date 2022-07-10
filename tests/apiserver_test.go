package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("cdi-apiserver tests", func() {
	var origSpec *cdiv1.CDIConfigSpec
	f := framework.NewFramework("cdi-apiserver-test", framework.Config{})

	Context("with apiserver", func() {

		It("should serve an openapi spec", func() {
			hostPort, cmd, err := startServicePortForward(f, "cdi-api")
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
			}()

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

		AfterEach(func() {
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

		It("should fail reaching server when TLS profile requires minimal TLS version higher than our client's", func() {
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				config.TLSSecurityProfile = &ocpconfigv1.TLSSecurityProfile{
					// Modern profile requires TLS 1.3
					// https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
					Type:   ocpconfigv1.TLSProfileModernType,
					Modern: &ocpconfigv1.ModernTLSProfile{},
				}
			})
			Expect(err).ToNot(HaveOccurred())

			hostPort, cmd, err := startServicePortForward(f, "cdi-api")
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
			}()
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

			Eventually(func() string {
				By("Should get TLS protocol version error")
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					return err.Error()
				}
				_, err = client.Do(req)
				if err != nil {
					return err.Error()
				}
				return ""
			}, 10*time.Second, 1*time.Second).Should(ContainSubstring("protocol version not supported"))
		})
	})
})
