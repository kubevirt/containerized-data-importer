package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

const (
	notAfter  = "auth.openshift.io/certificate-not-after"
	notBefore = "auth.openshift.io/certificate-not-before"
)

var _ = Describe("Cert rotation tests", func() {
	f := framework.NewFramework("certrotation-test")

	Context("with port forward", func() {
		DescribeTable("check secrets re read", func(serviceName, secretName string) {
			hostPort, cmd, err := startServicePortForward(f, serviceName)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
			}()

			var conn *tls.Conn

			Eventually(func() error {
				conn, err = tls.Dial("tcp", hostPort, &tls.Config{
					InsecureSkipVerify: true,
				})
				return err
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			oldExpire := conn.ConnectionState().PeerCertificates[0].NotAfter
			conn.Close()

			rotateCert(f, secretName)

			Eventually(func() error {
				conn, err = tls.Dial("tcp", hostPort, &tls.Config{
					InsecureSkipVerify: true,
				})

				if err != nil {
					return err
				}

				defer conn.Close()

				if conn.ConnectionState().PeerCertificates[0].NotAfter.After(oldExpire) {
					return nil
				}

				return fmt.Errorf("Expire time not updated")

			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

		}, Entry("[test_id:3925]apiserver", "cdi-api", "cdi-apiserver-server-cert"),
			Entry("[test_id:3926]uploadproxy", "cdi-uploadproxy", "cdi-uploadproxy-server-cert"))
	})

	DescribeTable("check secrets updated", func(secretName, configMapName string) {
		var oldBundle *corev1.ConfigMap
		oldSecret, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), secretName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		if configMapName != "" {
			oldBundle, err = f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Get(context.TODO(), configMapName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		rotateCert(f, secretName)

		Eventually(func() bool {
			updatedSecret, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), secretName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			return updatedSecret.Annotations[notAfter] != oldSecret.Annotations[notAfter] &&
				string(updatedSecret.Data["tls.cert"]) != string(oldSecret.Data["tls.crt"]) &&
				string(updatedSecret.Data["tls.key"]) != string(oldSecret.Data["tls.key"])

		}, 60*time.Second, 1*time.Second).Should(BeTrue())

		if configMapName != "" {
			Eventually(func() bool {
				updatedBundle, err := f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Get(context.TODO(), configMapName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				return updatedBundle.Data["ca-bundle.crt"] != oldBundle.Data["ca-bundle.crt"]

			}, 60*time.Second, 1*time.Second).Should(BeTrue())
		}

	}, // not supported
		//Entry("apiserver ca", "cdi-apiserver-signer", "cdi-apiserver-signer-bundle"),
		// done in test above
		//Entry("apiserver cert", "cdi-apiserver-server-cert", ""),
		Entry("[test_id:3927]uploadproxy ca", "cdi-uploadproxy-signer", "cdi-uploadproxy-signer-bundle"),
		//done in test above
		//Entry("uploadproxy cert", "cdi-uploadproxy-server-cert", ""),
		Entry("[test_id:3928]uploadserver ca", "cdi-uploadserver-signer", "cdi-uploadserver-signer-bundle"),
		Entry("[test_id:3929]uploadserver client ca", "cdi-uploadserver-client-signer", "cdi-uploadserver-client-signer-bundle"),
		Entry("[test_id:3930]uploadserver client cert", "cdi-uploadserver-client-cert", ""),
	)
})

func rotateCert(f *framework.Framework, secretName string) {
	secret, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), secretName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	nb, ok := secret.Annotations[notBefore]
	Expect(ok).To(BeTrue())

	notBefore, err := time.Parse(time.RFC3339, nb)
	Expect(err).ToNot(HaveOccurred())
	Expect(time.Now().Sub(notBefore).Seconds() > 0).To(BeTrue())

	newSecret := secret.DeepCopy()
	newSecret.Annotations[notAfter] = notBefore.Add(time.Second).Format(time.RFC3339)

	newSecret, err = f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
}

func startServicePortForward(f *framework.Framework, serviceName string) (string, *exec.Cmd, error) {
	lp := "18443"
	pm := lp + ":443"
	hostPort := "127.0.0.1:" + lp

	cmd := tests.CreateKubectlCommand(f, "-n", f.CdiInstallNs, "port-forward", "svc/"+serviceName, pm)
	err := cmd.Start()
	if err != nil {
		return "", nil, err
	}

	return hostPort, cmd, nil
}
