package tests_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/framework"
)

var _ = Describe("cdi-apiserver tests", func() {
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
})
