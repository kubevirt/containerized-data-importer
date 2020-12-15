package controller

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/certrotation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
	cdicerts "kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
)

const testCertData = "test"

type fakeCertManager struct {
	client    client.Client
	namespace string
}

func (tcm *fakeCertManager) Sync(certs []cdicerts.CertificateDefinition) error {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: tcm.namespace, Name: "cdi-uploadproxy-signer-bundle"}
	err := tcm.client.Get(context.TODO(), key, cm)
	// should exist
	if err != nil {
		return err
	}
	cm.Data = map[string]string{
		"ca-bundle.crt": testCertData,
	}
	return tcm.client.Update(context.TODO(), cm)
}

// creating certs is really CPU intensive so mocking out a CertManager to just create what we need
func newFakeCertManager(crClient client.Client, namespace string) CertManager {
	return &fakeCertManager{client: crClient, namespace: namespace}
}

func newCertManagerForTest(client kubernetes.Interface, namespace string) CertManager {
	return newCertManager(client, namespace)
}

func toSerializedCertConfig(l, r time.Duration) string {
	scc := &serializedCertConfig{
		Lifetime: l.String(),
		Refresh:  r.String(),
	}

	bs, err := json.Marshal(scc)
	Expect(err).ToNot(HaveOccurred())
	return string(bs)
}

func getCertNotBefore(client kubernetes.Interface, namespace, name string) time.Time {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	val, ok := s.Annotations[certrotation.CertificateNotBeforeAnnotation]
	Expect(ok).To(BeTrue())
	t, err := time.Parse(time.RFC3339, val)
	Expect(err).ToNot(HaveOccurred())
	return t
}

func getCertConfigAnno(client kubernetes.Interface, namespace, name string) string {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	val, ok := s.Annotations["operator.cdi.kubevirt.io/certConfig"]
	Expect(ok).To(BeTrue())
	return val
}

func checkSecret(client kubernetes.Interface, namespace, name string, exists bool) {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if !exists {
		Expect(errors.IsNotFound(err)).To(BeTrue())
		return
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(s.Data["tls.crt"]).ShouldNot(BeEmpty())
	Expect(s.Data["tls.crt"]).ShouldNot(BeEmpty())
}

func checkConfigMap(client kubernetes.Interface, namespace, name string, exists bool) {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if !exists {
		Expect(errors.IsNotFound(err)).To(BeTrue())
		return
	}
	Expect(cm.Data["ca-bundle.crt"]).ShouldNot(BeEmpty())
}

func checkCerts(client kubernetes.Interface, namespace string, exists bool) {
	checkSecret(client, namespace, "cdi-apiserver-signer", exists)
	checkConfigMap(client, namespace, "cdi-apiserver-signer-bundle", exists)
	checkSecret(client, namespace, "cdi-apiserver-server-cert", exists)

	checkSecret(client, namespace, "cdi-uploadproxy-signer", exists)
	checkConfigMap(client, namespace, "cdi-uploadproxy-signer-bundle", exists)
	checkSecret(client, namespace, "cdi-uploadproxy-server-cert", exists)

	checkSecret(client, namespace, "cdi-uploadserver-signer", exists)
	checkConfigMap(client, namespace, "cdi-uploadserver-signer-bundle", exists)

	checkSecret(client, namespace, "cdi-uploadserver-client-signer", exists)
	checkConfigMap(client, namespace, "cdi-uploadserver-client-signer-bundle", exists)
	checkSecret(client, namespace, "cdi-uploadserver-client-cert", exists)
}

var _ = Describe("Cert rotation tests", func() {
	const namespace = "cdi"

	pt := func(d time.Duration) *time.Duration {
		return &d
	}

	Context("with clean slate", func() {
		It("should create everything", func() {
			client := fake.NewSimpleClientset()
			cm := newCertManagerForTest(client, namespace)

			ch := make(chan struct{})
			cm.(*certManager).Start(ch)

			checkCerts(client, namespace, false)

			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)

			certs = cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err = cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)

			close(ch)
		})

		It("should update certs", func() {
			client := fake.NewSimpleClientset()
			cm := newCertManagerForTest(client, namespace)

			ch := make(chan struct{})
			cm.(*certManager).Start(ch)

			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			apiCA := getCertNotBefore(client, namespace, "cdi-apiserver-signer")
			apiServer := getCertNotBefore(client, namespace, "cdi-apiserver-server-cert")
			proxyCA := getCertNotBefore(client, namespace, "cdi-uploadproxy-signer")
			proxyServer := getCertNotBefore(client, namespace, "cdi-uploadproxy-server-cert")

			apiCAConfig := getCertConfigAnno(client, namespace, "cdi-apiserver-signer")
			apiServerConfig := getCertConfigAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCAConfig := getCertConfigAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServerConfig := getCertConfigAnno(client, namespace, "cdi-uploadproxy-server-cert")

			n := time.Now()

			args := &cert.FactoryArgs{
				Namespace:      namespace,
				SignerValidity: pt(50 * time.Hour),
				SignerRefresh:  pt(25 * time.Hour),
				TargetValidity: pt(26 * time.Hour),
				TargetRefresh:  pt(13 * time.Hour),
			}

			certs = cert.CreateCertificateDefinitions(args)
			err = cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			apiCA2 := getCertNotBefore(client, namespace, "cdi-apiserver-signer")
			apiServer2 := getCertNotBefore(client, namespace, "cdi-apiserver-server-cert")
			proxyCA2 := getCertNotBefore(client, namespace, "cdi-uploadproxy-signer")
			proxyServer2 := getCertNotBefore(client, namespace, "cdi-uploadproxy-server-cert")

			Expect(apiCA2.After(n))
			Expect(apiServer2.After(n))
			Expect(proxyCA2.After(n))
			Expect(proxyServer2.After(n))

			Expect(apiCA2.After(apiCA))
			Expect(apiServer2.After(apiServer))
			Expect(proxyCA2.After(proxyCA))
			Expect(proxyServer2.After(proxyServer))

			apiCAConfig2 := getCertConfigAnno(client, namespace, "cdi-apiserver-signer")
			apiServerConfig2 := getCertConfigAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCAConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServerConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadproxy-server-cert")

			Expect(apiCAConfig2).ToNot(Equal(apiCAConfig))
			Expect(apiServerConfig2).ToNot(Equal(apiServerConfig))
			Expect(proxyCAConfig2).ToNot(Equal(proxyCAConfig))
			Expect(proxyServerConfig2).ToNot(Equal(proxyServerConfig))

			scc := toSerializedCertConfig(50*time.Hour, 25*time.Hour)
			scc2 := toSerializedCertConfig(26*time.Hour, 13*time.Hour)

			Expect(apiCAConfig2).To(Equal(scc))
			Expect(apiServerConfig2).To(Equal(scc2))
			Expect(proxyCAConfig2).To(Equal(scc))
			Expect(proxyServerConfig2).To(Equal(scc2))
		})
	})
})
