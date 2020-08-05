package controller

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

func checkSecret(client kubernetes.Interface, namespace, name string, exists bool) {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if !exists {
		Expect(errors.IsNotFound(err)).To(BeTrue())
		return
	}
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
	Context("with clean slate", func() {
		client := fake.NewSimpleClientset()
		cm := newCertManagerForTest(client, namespace)

		It("should create everything", func() {
			checkCerts(client, namespace, false)

			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)
		})

		It("should not do anything", func() {
			checkCerts(client, namespace, true)

			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)
		})
	})
})
