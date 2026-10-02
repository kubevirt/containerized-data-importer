package main

import (
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/util/cert"

	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

var _ = Describe("Prometheus Endpoint", func() {
	It("Should start prometheus endpoint with pre-generated certs", func() {
		certsDirectory, err := os.MkdirTemp("", "certsdir")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(certsDirectory)

		certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("test", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path.Join(certsDirectory, "tls.crt"), certBytes, 0600)).To(Succeed())
		Expect(os.WriteFile(path.Join(certsDirectory, "tls.key"), keyBytes, 0600)).To(Succeed())

		Expect(prometheusutil.StartPrometheusEndpointNoCertGeneration(certsDirectory)).To(Succeed())
	})

	It("Should fail if certs are missing", func() {
		certsDirectory, err := os.MkdirTemp("", "certsdir")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(certsDirectory)

		err = prometheusutil.StartPrometheusEndpointNoCertGeneration(certsDirectory)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cert file"))
		Expect(err.Error()).To(ContainSubstring("does not exist"))
	})
})
