package controller

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/operator/certrotation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	clocktesting "k8s.io/utils/clock/testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
)

const testCertData = "test"

var secretNames = []string{
	"cdi-apiserver-signer", "cdi-apiserver-server-cert",
	"cdi-uploadproxy-signer", "cdi-uploadproxy-server-cert",
	"cdi-uploadserver-signer", "cdi-uploadserver-client-signer", "cdi-uploadserver-client-cert",
}

type fakeCertManager struct {
	client    client.Client
	namespace string
}

func (tcm *fakeCertManager) Sync(certs []cert.CertificateDefinition) error {
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
	return newCertManager(client, namespace, clocktesting.NewFakePassiveClock(time.Now()))
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

func getCertNotAfterAnno(client kubernetes.Interface, namespace, name string) string {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return s.Annotations[certrotation.CertificateNotAfterAnnotation]
}

func getCertConfigAnno(client kubernetes.Interface, namespace, name string) string {
	s, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	val, ok := s.Annotations["operator.cdi.kubevirt.io/certConfig"]
	Expect(ok).To(BeTrue())
	return val
}

func checkSecret(client kubernetes.Interface, namespace, name string, exists bool) {
	_, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if !exists {
		Expect(errors.IsNotFound(err)).To(BeTrue())
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

func checkCerts(client kubernetes.Interface, namespace string, exists bool) {
	for _, name := range secretNames {
		checkSecret(client, namespace, name, exists)
	}
}

func populateClientAndStoreWithSecrets(client kubernetes.Interface, cm *certManager, ns string) {
	for _, name := range secretNames {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		}
		Expect(
			cm.informers.InformersFor(ns).Core().V1().Secrets().Informer().GetStore().Add(secret),
		).To(Succeed())
		_, err := client.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}
}

func syncStoreWithLocalUpdateCalls(client *fake.Clientset, cm *certManager, ns string) {
	// This is not needed in production since the informer is going to react to the updates
	client.Fake.PrependReactor("update", "secrets", func(action testing.Action) (handled bool, obj runtime.Object, err error) {
		update, ok := action.(testing.UpdateAction)
		Expect(ok).To(BeTrue())
		secret, ok := update.GetObject().(*corev1.Secret)
		Expect(ok).To(BeTrue())
		Expect(
			cm.informers.InformersFor(ns).Core().V1().Secrets().Informer().GetStore().Update(secret),
		).To(Succeed())
		return false, secret, nil
	})
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

			ctx, cancel := context.WithCancel(context.Background())
			Expect(cm.(*certManager).Start(ctx)).To(Succeed())

			checkCerts(client, namespace, false)

			populateClientAndStoreWithSecrets(client, cm.(*certManager), namespace)
			syncStoreWithLocalUpdateCalls(client, cm.(*certManager), namespace)
			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)

			certs = cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			err = cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			checkCerts(client, namespace, true)

			cancel()
		})

		It("should update certs", func() {
			client := fake.NewSimpleClientset()
			cm := newCertManagerForTest(client, namespace)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			Expect(cm.(*certManager).Start(ctx)).To(Succeed())

			certs := cert.CreateCertificateDefinitions(&cert.FactoryArgs{Namespace: namespace})
			populateClientAndStoreWithSecrets(client, cm.(*certManager), namespace)
			syncStoreWithLocalUpdateCalls(client, cm.(*certManager), namespace)
			err := cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			apiCA := getCertNotAfterAnno(client, namespace, "cdi-apiserver-signer")
			apiServer := getCertNotAfterAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCA := getCertNotAfterAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServer := getCertNotAfterAnno(client, namespace, "cdi-uploadproxy-server-cert")
			uploadServerCA := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-signer")
			uploadClientCA := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-client-signer")
			uploadClient := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-client-cert")

			apiCAConfig := getCertConfigAnno(client, namespace, "cdi-apiserver-signer")
			apiServerConfig := getCertConfigAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCAConfig := getCertConfigAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServerConfig := getCertConfigAnno(client, namespace, "cdi-uploadproxy-server-cert")
			uploadServerCAConfig := getCertConfigAnno(client, namespace, "cdi-uploadserver-signer")
			uploadClientCAConfig := getCertConfigAnno(client, namespace, "cdi-uploadserver-client-signer")
			uploadClientConfig := getCertConfigAnno(client, namespace, "cdi-uploadserver-client-cert")

			args := &cert.FactoryArgs{
				Namespace:         namespace,
				SignerDuration:    pt(50 * time.Hour),
				SignerRenewBefore: pt(25 * time.Hour),
				ServerDuration:    pt(28 * time.Hour),
				ServerRenewBefore: pt(14 * time.Hour),
				ClientDuration:    pt(26 * time.Hour),
				ClientRenewBefore: pt(13 * time.Hour),
			}

			certs = cert.CreateCertificateDefinitions(args)
			err = cm.Sync(certs)
			Expect(err).ToNot(HaveOccurred())

			apiCA2 := getCertNotAfterAnno(client, namespace, "cdi-apiserver-signer")
			apiServer2 := getCertNotAfterAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCA2 := getCertNotAfterAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServer2 := getCertNotAfterAnno(client, namespace, "cdi-uploadproxy-server-cert")
			uploadServerCA2 := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-signer")
			uploadClientCA2 := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-client-signer")
			uploadClient2 := getCertNotAfterAnno(client, namespace, "cdi-uploadserver-client-cert")

			Expect(apiCA2).ToNot(Equal(apiCA))
			Expect(apiServer2).ToNot(Equal(apiServer))
			Expect(proxyCA2).ToNot(Equal(proxyCA))
			Expect(proxyServer2).ToNot(Equal(proxyServer))
			Expect(uploadServerCA2).ToNot(Equal(uploadServerCA))
			Expect(uploadClientCA2).ToNot(Equal(uploadClientCA))
			Expect(uploadClient2).ToNot(Equal(uploadClient))

			apiCAConfig2 := getCertConfigAnno(client, namespace, "cdi-apiserver-signer")
			apiServerConfig2 := getCertConfigAnno(client, namespace, "cdi-apiserver-server-cert")
			proxyCAConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadproxy-signer")
			proxyServerConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadproxy-server-cert")
			uploadServerCAConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadserver-signer")
			uploadClientCAConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadserver-client-signer")
			uploadClientConfig2 := getCertConfigAnno(client, namespace, "cdi-uploadserver-client-cert")

			Expect(apiCAConfig2).ToNot(Equal(apiCAConfig))
			Expect(apiServerConfig2).ToNot(Equal(apiServerConfig))
			Expect(proxyCAConfig2).ToNot(Equal(proxyCAConfig))
			Expect(proxyServerConfig2).ToNot(Equal(proxyServerConfig))
			Expect(uploadServerCAConfig2).ToNot(Equal(uploadServerCAConfig))
			Expect(uploadClientCAConfig2).ToNot(Equal(uploadClientCAConfig))
			Expect(uploadClientConfig2).ToNot(Equal(uploadClientConfig))

			scc := toSerializedCertConfig(50*time.Hour, 25*time.Hour)
			scc2 := toSerializedCertConfig(28*time.Hour, 14*time.Hour)
			scc3 := toSerializedCertConfig(26*time.Hour, 13*time.Hour)

			Expect(apiCAConfig2).To(Equal(scc))
			Expect(apiServerConfig2).To(Equal(scc2))
			Expect(proxyCAConfig2).To(Equal(scc))
			Expect(proxyServerConfig2).To(Equal(scc2))
			Expect(uploadServerCAConfig2).To(Equal(scc))
			Expect(uploadClientCAConfig2).To(Equal(scc))
			Expect(uploadClientConfig2).To(Equal(scc3))
		})
	})
})
