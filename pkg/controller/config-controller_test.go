/*
Copyright 2020 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

const (
	testURL         = "www.this.is.a.test.org"
	testRouteURL    = "cdi-uploadproxy.example.com"
	testServiceName = "cdi-proxyurl"
	testNamespace   = "cdi-test"
)

var (
	configLog = logf.Log.WithName("config-controller-test")
)

var _ = Describe("CDIConfig Controller reconcile loop", func() {
	It("Should not update if no changes happened", func() {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace))
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		// CDIConfig generated, now reconcile again without changes.
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should update labels on CDIConfig when the ones on CR change", func() {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace))
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		// CDIConfig generated
		reconciler.installerLabels[common.AppKubernetesPartOfLabel] = "new"
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Labels[common.AppKubernetesPartOfLabel]).To(Equal("new"))
	})

	DescribeTable("Should set proxyURL to override if no ingress or route exists", func(authority bool) {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		override := "www.override-something.org.tt.test"
		cdi, err := GetActiveCDI(context.TODO(), reconciler.client)
		Expect(err).ToNot(HaveOccurred())
		cdi.Spec.Config = &cdiv1.CDIConfigSpec{
			UploadProxyURLOverride: &override,
		}
		if !authority {
			delete(cdi.Annotations, "cdi.kubevirt.io/configAuthority")
		}
		err = reconciler.client.Update(context.TODO(), cdi)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		if authority {
			Expect(override).To(Equal(*cdiConfig.Status.UploadProxyURL))
		} else {
			Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
		}
	},
		Entry("as authority", true),
		Entry("not authority", false),
	)

	DescribeTable("Should set proxyURL to override if ingress or route exists", func(authority bool) {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace),
			createIngressList(
				*createIngress("test-ingress", testNamespace, testServiceName, testURL),
			),
		)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		override := "www.override-something.org.tt.test"
		cdi, err := GetActiveCDI(context.TODO(), reconciler.client)
		Expect(err).ToNot(HaveOccurred())
		cdi.Spec.Config = &cdiv1.CDIConfigSpec{
			UploadProxyURLOverride: &override,
		}
		if !authority {
			delete(cdi.Annotations, "cdi.kubevirt.io/configAuthority")
		}
		err = reconciler.client.Update(context.TODO(), cdi)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		if authority {
			Expect(override).To(Equal(*cdiConfig.Status.UploadProxyURL))
		} else {
			Expect(cdiConfig.Status.UploadProxyURL).ToNot(BeNil())
			Expect(override).ToNot(Equal(*cdiConfig.Status.UploadProxyURL))
		}
	},
		Entry("as authority", true),
		Entry("not authority", false),
	)

	It("Should set the Ingress UploadProxyCA to the right value", func() {
		// Create the reconciler such that the ingressupload proxy URL is set
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress", testNamespace, testServiceName),
		))
		reconciler.uploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).NotTo(BeNil())

		// Put the certificate in a ConfigMap
		root, certificate := generateCertificate(*cdiConfig.Status.UploadProxyURL)

		err = reconciler.client.Create(context.TODO(), &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rootCertificateConfigMap,
				Namespace: reconciler.cdiNamespace,
			},
			Data: map[string]string{"ca.crt": fmt.Sprintf("%s\n%s", root, certificate)},
		})
		Expect(err).ToNot(HaveOccurred())

		// Reconcile
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())

		// Ensure the upload proxy CA is set
		var c cdiv1.CDIConfig
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, &c)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Status.UploadProxyCA).ToNot(BeNil())

		// Ensure the certificate is valid (cannot compare directly due to the certificate chain being rebuilt)
		certs, err := cert.ParseCertsPEM([]byte(*c.Status.UploadProxyCA))
		Expect(err).ToNot(HaveOccurred())
		Expect(certs).NotTo(BeEmpty(), "Expected at least one certificate in the chain")
		Expect(certs[0].VerifyHostname(*cdiConfig.Status.UploadProxyURL)).ToNot(HaveOccurred(), "Certificate does not match the upload proxy URL")
	})

	It("Should set the Route UploadProxyCA to the right value", func() {
		const secretName = "test-secret-route-ca"

		ingress := createIngress("test-ingress", testNamespace, testServiceName, testURL)
		ingress.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{testURL},
			SecretName: secretName,
		}}

		// Create the reconciler such that the ingressupload proxy URL is set
		reconciler, _ := createConfigReconciler(createIngressList(*ingress))

		// Put the certificate in the secret referenced by the ingress
		root, certificate := generateCertificate(testURL)
		err := reconciler.client.Create(context.TODO(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"tls.crt": []byte(fmt.Sprintf("%s\n%s", root, certificate)),
			},
		})
		Expect(err).ToNot(HaveOccurred())

		// Reconcile
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())

		// Ensure the upload proxy CA is set
		var c cdiv1.CDIConfig
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, &c)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Status.UploadProxyCA).ToNot(BeNil())

		// Ensure the certificate is valid (cannot compare directly due to the certificate chain being rebuilt)
		certs, err := cert.ParseCertsPEM([]byte(*c.Status.UploadProxyCA))
		Expect(err).ToNot(HaveOccurred())
		Expect(certs).NotTo(BeEmpty(), "Expected at least one certificate in the chain")
		Expect(certs[0].VerifyHostname(testURL)).ToNot(HaveOccurred(), "Certificate does not match the upload proxy URL")
	})
})

var _ = Describe("Controller ingress reconcile loop", func() {
	It("Should set uploadProxyUrl to nil if no Ingress exists", func() {
		reconciler, cdiConfig := createConfigReconciler()
		ing, err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
		Expect(ing).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if ingress with correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress", testNamespace, testServiceName, testURL),
		))
		reconciler.uploadProxyServiceName = testServiceName
		ing, err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testURL))
		Expect(ing).ToNot(BeNil())
	})

	It("Should not set uploadProxyUrl if ingress with incorrect serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress", testNamespace, "incorrect", testURL),
		))
		reconciler.uploadProxyServiceName = testServiceName
		ing, err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
		Expect(ing).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if multiple ingresses exist with one correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress1", testNamespace, "service1", "invalidurl"),
			*createIngress("test-ingress2", testNamespace, "service2", "invalidurl2"),
			*createIngress("test-ingress3", testNamespace, testServiceName, testURL),
			*createIngress("test-ingress4", testNamespace, "service3", "invalidurl3"),
		))
		reconciler.uploadProxyServiceName = testServiceName
		ing, err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testURL))
		Expect(ing).ToNot(BeNil())
	})

	DescribeTable("Should not set proxyURL if invalid ingress exists", func(createIngress func(name, ns, service, url string) *networkingv1.Ingress) {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress", testNamespace, "service", testURL),
		))
		reconciler.uploadProxyServiceName = testServiceName
		ing, err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig).ToNot(BeNil())
		Expect(cdiConfig.Status).ToNot(BeNil())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
		Expect(ing).To(BeNil())
	},
		Entry("No default backend", createNoDefaultBackendIngress),
		Entry("No service", createNoServiceIngress),
		Entry("0 rules", createNoRulesIngress),
	)
})

var _ = Describe("Controller route reconcile loop", func() {
	It("Should set uploadProxyUrl to nil if no Route exists", func() {
		reconciler, cdiConfig := createConfigReconciler()
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if route with correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress", testNamespace, testServiceName),
		))
		reconciler.uploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testRouteURL))
	})

	It("Should not set uploadProxyUrl if ingress with incorrect serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress", testNamespace, "incorrect"),
		))
		reconciler.uploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if multiple ingresses exist with one correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress1", testNamespace, "service1"),
			*createRoute("test-ingress2", testNamespace, "service2"),
			*createRoute("test-ingress3", testNamespace, testServiceName),
			*createRoute("test-ingress4", testNamespace, "service3"),
		))
		reconciler.uploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testRouteURL))
	})
})

var _ = Describe("Controller storage class reconcile loop", func() {
	It("Should set the scratchspaceStorageClass to blank if there is no default sc", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*CreateStorageClass("test-default-sc", nil),
		))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal(""))
	})

	It("Should set the scratchspaceStorageClass to the default without override", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*CreateStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			},
			)))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})

	It("Should set the scratchspaceStorageClass to the default without override and multiple sc", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*CreateStorageClass("test-sc3", nil),
			*CreateStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*CreateStorageClass("test-sc", nil),
			*CreateStorageClass("test-sc2", nil),
		))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})

	It("Should set the scratchspaceStorageClass to the override even with default", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*CreateStorageClass("test-sc3", nil),
			*CreateStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*CreateStorageClass("test-sc", nil),
			*CreateStorageClass("test-sc2", nil),
		))
		override := "test-sc"
		cdiConfig.Spec.ScratchSpaceStorageClass = &override
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal(override))
	})

	It("Should set the scratchspaceStorageClass to the default with invalid override", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*CreateStorageClass("test-sc3", nil),
			*CreateStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*CreateStorageClass("test-sc", nil),
			*CreateStorageClass("test-sc2", nil),
		))
		override := "invalid"
		cdiConfig.Spec.ScratchSpaceStorageClass = &override
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})
})

var _ = Describe("Controller ImportProxy reconcile loop", func() {
	It("Should set ImportProxy to nil if no proxy configuration for import proxy exists", func() {
		reconciler, cdiConfig := createConfigReconciler()
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ImportProxy).To(BeNil())
	})

	var proxyHTTPURL = "http://user:pswd@www.myproxy.com"
	var proxyHTTPSURL = "https://user:pswd@www.myproxy.com"
	var noProxyDomains = ".myproxy.com,.noproxy.com"
	var trustedCAProxy = "user-ca-bundle"

	DescribeTable("Should set ImportProxy correctly if ClusterWideProxy with correct URLs exists", func(proxyHTTPURL string, proxyHTTPSURL string, noProxyDomains string, trustedCAName string, expect string, endpType string) {
		trustedCAProxyConfigMap := createConfigMap(trustedCAProxy, ClusterWideProxyConfigMapNameSpace)
		trustedCAProxyConfigMap.Data = map[string]string{ClusterWideProxyConfigMapKey: "fake_cert"}
		reconciler, cdiConfig := createConfigReconciler(createClusterWideProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAProxy), trustedCAProxyConfigMap)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		switch endpType {
		case common.ImportProxyHTTP:
			Expect(*cdiConfig.Status.ImportProxy.HTTPProxy).To(Equal(expect))
		case common.ImportProxyHTTPS:
			Expect(*cdiConfig.Status.ImportProxy.HTTPSProxy).To(Equal(expect))
		case common.ImportProxyNoProxy:
			Expect(*cdiConfig.Status.ImportProxy.NoProxy).To(Equal(expect))
		case common.ImportProxyConfigMapName:
			Expect(*cdiConfig.Status.ImportProxy.TrustedCAProxy).To(Equal(expect))
		default:
		}
	},
		Entry("successfully get http proxy url", proxyHTTPURL, "", "", "", proxyHTTPURL, common.ImportProxyHTTP),
		Entry("successfully get https proxy url", "", proxyHTTPSURL, "", "", proxyHTTPSURL, common.ImportProxyHTTPS),
		Entry("successfully get the list of hostnames and/or CIDRs that proxy should not be used", "", "", noProxyDomains, "", noProxyDomains, common.ImportProxyNoProxy),
		Entry("successfully get ConfiMap CA name", "", "", "", trustedCAProxy, trustedCAProxy, trustedCAProxy),
	)

	It("Should error and record event if TrustedCAProxy ConfigMap is missing", func() {
		reconciler, _ := createConfigReconciler(createClusterWideProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAProxy))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring(errResourceDoesntExist))
	})

	It("Should not change the CDIConfig when updating the ClusterWideProxy if the CDIConfig proxy information already exist", func() {
		reconciler, cdiConfig := createConfigReconciler()
		By("updating the CDIConfig with proxy information")
		cdiConfig.Spec.ImportProxy = createImportProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAProxy)
		err := reconciler.reconcileImportProxy(cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		By("creating cluster wide proxy")
		proxy := createClusterWideProxy("http", "https", "noproxy", "ca")
		err = reconciler.uncachedClient.Create(context.TODO(), proxy)
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.reconcileImportProxy(cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		Expect(proxyHTTPURL).To(Equal(*cdiConfig.Status.ImportProxy.HTTPProxy))
		Expect(proxyHTTPSURL).To(Equal(*cdiConfig.Status.ImportProxy.HTTPSProxy))
		Expect(noProxyDomains).To(Equal(*cdiConfig.Status.ImportProxy.NoProxy))
		Expect(trustedCAProxy).To(Equal(*cdiConfig.Status.ImportProxy.TrustedCAProxy))
	})

	It("Should create a new ConfigMap if ClusterWideProxy contains CA certificates name of an exiting ConfigMap in Openshift namespace", func() {
		certificate := "ca-test"
		reconciler, cdiConfig := createConfigReconciler(createClusterWideProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, ClusterWideProxyConfigMapName),
			createClusterWideProxyCAConfigMap(certificate))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: reconciler.configName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		test := &corev1.ConfigMap{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: ClusterWideProxyConfigMapName, Namespace: ClusterWideProxyConfigMapNameSpace}, test)
		Expect(err).ToNot(HaveOccurred())

		configMap := &corev1.ConfigMap{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: ClusterWideProxyConfigMapName, Namespace: reconciler.cdiNamespace}, configMap)
		Expect(err).ToNot(HaveOccurred())
		Expect(configMap.Labels[common.AppKubernetesComponentLabel]).To(Equal("storage"))

		Expect(configMap.Data[common.ImportProxyConfigMapKey]).To(Equal(certificate))
	})
})

var _ = Describe("Controller create CDI config", func() {
	It("Should return existing cdi config", func() {
		reconciler, cdiConfig := createConfigReconciler()
		resConfig, err := reconciler.createCDIConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(resConfig).ToNot(BeNil())
		Expect(cdiConfig).ToNot(BeNil())
		Expect(*resConfig).To(Equal(*cdiConfig))
	})

	It("Should create a new CDIConfig if not found and configmap exists", func() {
		reconciler, cdiConfig := createConfigReconciler()
		Expect(cdiConfig.Name).To(Equal("cdiconfig"))

		// Make sure no cdi config object exists
		err := reconciler.client.Delete(context.TODO(), cdiConfig)
		Expect(err).To(Not(HaveOccurred()))

		owner := true
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cdi-config",
				Namespace: "cdi",
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller: &owner,
						Name:       "testowner",
					},
				},
			},
		}
		err = reconciler.client.Create(context.TODO(), configMap)
		Expect(err).To(Not(HaveOccurred()))

		reconciler.configName = "testconfig"
		resConfig, err := reconciler.createCDIConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(resConfig.Name).To(Equal("testconfig"))
		Expect(resConfig.ObjectMeta.OwnerReferences[0].Name).To(Equal("testowner"))
		Expect(resConfig.Labels[common.AppKubernetesPartOfLabel]).To(Equal(""))
	})
})

var _ = Describe("getUrlFromIngress", func() {
	//TODO: Once we get newer version of client-go, we need to switch to networking ingress.
	It("Should return the url if backend service matches", func() {
		ingress := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{Name: testServiceName},
				},
				Rules: []networkingv1.IngressRule{
					{Host: testURL},
				},
			},
		}
		resURL := getURLFromIngress(ingress, testServiceName)
		Expect(resURL).To(Equal(testURL))
	})

	It("Should return blank if backend service does not match", func() {
		ingress := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{Name: testServiceName},
				},
				Rules: []networkingv1.IngressRule{
					{Host: testURL},
				},
			},
		}
		resURL := getURLFromIngress(ingress, "somethingelse")
		Expect(resURL).To(Equal(""))
	})

	It("Should return the url if first rule backend service matches", func() {
		ingress := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: testServiceName},
										},
									},
								},
							},
						},
						Host: testURL,
					},
				},
			},
		}
		resURL := getURLFromIngress(ingress, testServiceName)
		Expect(resURL).To(Equal(testURL))
	})

	It("Should return the url if any rule backend servicename matches", func() {
		ingress := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: "service1"},
										},
									},
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: "service2"},
										},
									},
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: testServiceName},
										},
									},
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: "service4"},
										},
									},
								},
							},
						},
						Host: testURL,
					},
				},
			},
		}
		resURL := getURLFromIngress(ingress, testServiceName)
		Expect(resURL).To(Equal(testURL))
	})

	It("Should return blank if no http rule exists", func() {
		ingress := &networkingv1.Ingress{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "networking/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{},
					},
				},
			},
		}
		resURL := getURLFromIngress(ingress, testServiceName)
		Expect(resURL).To(Equal(""))
	})
})

var _ = Describe("getUrlFromRoute", func() {
	It("Should return the URL if the route spec to name matches and status ingress is correct", func() {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: testServiceName,
				},
			},
			Status: routev1.RouteStatus{
				Ingress: []routev1.RouteIngress{
					{Host: testRouteURL},
				},
			},
		}
		resURL := getURLFromRoute(route, testServiceName)
		Expect(resURL).To(Equal(testRouteURL))
	})

	It("Should return blank if the route spec to name does not match", func() {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "doesntmatch",
				},
			},
			Status: routev1.RouteStatus{
				Ingress: []routev1.RouteIngress{
					{Host: testRouteURL},
				},
			},
		}
		resURL := getURLFromRoute(route, testServiceName)
		Expect(resURL).To(Equal(""))
	})

	It("Should return blank if the route spec to name matches and status ingress not there", func() {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: testServiceName,
				},
			},
			Status: routev1.RouteStatus{},
		}
		resURL := getURLFromRoute(route, testServiceName)
		Expect(resURL).To(Equal(""))
	})
})

var _ = Describe("Controller default pod resource requirements reconcile loop", func() {
	var (
		testValueCPULimit   = "10"
		testValueCPURequest = "4"
		testValueMemLimit   = "10M"
		testValueMemRequest = "4M"
	)

	It("Should set the defaultPodResourceRequirements to the override value", func() {
		defaultResourceRequirements := createDefaultPodResourceRequirements("1", "2", "3000M", "4000M")

		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err := reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(defaultResourceRequirements))
	})

	It("Should set the defaultPodResourceRequirements to the default if ResourceRequirements.Limits and ResourceRequirements.Requests are nil", func() {
		defaultResourceRequirements := &corev1.ResourceRequirements{
			Limits:   nil,
			Requests: nil,
		}

		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err := reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(createDefaultPodResourceRequirements("", "", "", "")))
	})

	It("Should set the defaultPodResourceRequirements to the default if all fields are null except ResourceRequirements.Limits.cpu", func() {
		var err error
		defaultResourceRequirements := &corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{},
			Requests: nil,
		}
		defaultResourceRequirements.Limits[corev1.ResourceCPU], err = resource.ParseQuantity(testValueCPULimit)
		Expect(err).ToNot(HaveOccurred())
		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err = reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(createDefaultPodResourceRequirements(testValueCPULimit, "", "", "")))
	})

	It("Should set the defaultPodResourceRequirements to the default if all fields are null except ResourceRequirements.Limits.memory", func() {
		var err error
		defaultResourceRequirements := &corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{},
			Requests: nil,
		}

		defaultResourceRequirements.Limits[corev1.ResourceMemory], err = resource.ParseQuantity(testValueMemLimit)
		Expect(err).ToNot(HaveOccurred())

		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err = reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(createDefaultPodResourceRequirements("", testValueMemLimit, "", "")))
	})

	It("Should set the defaultPodResourceRequirements to the default if all fields are null except ResourceRequirements.Requests.cpu", func() {
		var err error
		defaultResourceRequirements := &corev1.ResourceRequirements{
			Limits:   nil,
			Requests: corev1.ResourceList{},
		}

		defaultResourceRequirements.Requests[corev1.ResourceCPU], err = resource.ParseQuantity(testValueCPURequest)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(defaultResourceRequirements)

		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err = reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(createDefaultPodResourceRequirements("", "", testValueCPURequest, "")))
	})

	It("Should set the defaultPodResourceRequirements to the default if all fields are null except ResourceRequirements.Requests.memory", func() {
		var err error
		defaultResourceRequirements := &corev1.ResourceRequirements{
			Limits:   nil,
			Requests: corev1.ResourceList{},
		}

		defaultResourceRequirements.Requests[corev1.ResourceMemory], err = resource.ParseQuantity(testValueMemRequest)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(defaultResourceRequirements)

		reconciler, cdiConfig := createConfigReconciler()
		cdiConfig.Spec.PodResourceRequirements = defaultResourceRequirements

		err = reconciler.reconcileDefaultPodResourceRequirements(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.DefaultPodResourceRequirements).To(Equal(createDefaultPodResourceRequirements("", "", "", testValueMemRequest)))
	})
})

var _ = Describe("getClusterWideProxy", func() {
	var proxyHTTPURL = "http://user:pswd@www.myproxy.com"
	var proxyHTTPSURL = "https://user:pswd@www.myproxy.com"
	var noProxyDomains = ".noproxy.com"
	var trustedCAName = "user-ca-bundle"

	It("Should return a not empty cluster wide proxy obj", func() {
		client := CreateClient(createClusterWideProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAName))
		proxy, err := getClusterWideProxy(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).ToNot(BeNil())

		By("should return a proxy https url")
		Expect(proxyHTTPSURL).To(Equal(proxy.Status.HTTPSProxy))

		By("should return a proxy http url")
		Expect(proxyHTTPURL).To(Equal(proxy.Status.HTTPProxy))

		By("should return a noProxy list of domains")
		Expect(noProxyDomains).To(Equal(proxy.Status.NoProxy))

		By("should return a CA ConfigMap name")
		Expect(trustedCAName).To(Equal(proxy.Spec.TrustedCA.Name))
	})

	It("Should return a nil cluster wide proxy obj", func() {
		client := CreateClient()
		proxy, err := getClusterWideProxy(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(BeEquivalentTo(&ocpconfigv1.Proxy{}))
	})
})

var _ = Describe("GetImportProxyConfig", func() {
	var proxyHTTPURL = "http://user:pswd@www.myproxy.com"
	var proxyHTTPSURL = "https://user:pswd@www.myproxy.com"
	var noProxyDomains = ".noproxy.com"
	var trustedCAName = "user-ca-bundle"

	It("should return valid proxy information from a CDIConfig with importer proxy configured", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAName)
		field, _ := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect(proxyHTTPURL).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS)
		Expect(proxyHTTPSURL).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy)
		Expect(noProxyDomains).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName)
		Expect(trustedCAName).To(Equal(field))
	})

	It("should return blank proxy information from a CDIConfig with importer proxy not configured", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy("", "", "", "")
		field, _ := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName)
		Expect("").To(Equal(field))
	})

	It("should return error if the requested field does not exist", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy("", "", "", "")
		_, err := GetImportProxyConfig(cdiConfig, "nonExistingField")
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("CDIConfig ImportProxy does not have the field: %s", "nonExistingField")))
	})

	It("should return error if the ImportProxy field is nil", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = nil
		_, err := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect(err.Error()).To(ContainSubstring("failed to get field, the CDIConfig ImportProxy is nil"))
	})
})

func createConfigReconciler(objects ...runtime.Object) (*CDIConfigReconciler, *cdiv1.CDIConfig) {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
	objs = append(objs, cdiConfig)
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	schemeInitFuncs := []func(*runtime.Scheme) error{
		cdiv1.AddToScheme,
		networkingv1.AddToScheme,
		routev1.Install,
		storagev1.AddToScheme,
		ocpconfigv1.Install,
	}
	for _, f := range schemeInitFuncs {
		if err := f(s); err != nil {
			panic(fmt.Errorf("failed to initiate the scheme %w", err))
		}
	}

	cdi := &cdiv1.CDI{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdi",
			Annotations: map[string]string{
				"cdi.kubevirt.io/configAuthority": "",
			},
		},
	}

	objs = append(objs, cdi)

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &CDIConfigReconciler{
		client:                 cl,
		uncachedClient:         cl,
		scheme:                 s,
		log:                    configLog,
		recorder:               record.NewFakeRecorder(1),
		configName:             "cdiconfig",
		cdiNamespace:           testNamespace,
		uploadProxyServiceName: testServiceName,
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "",
			common.AppKubernetesVersionLabel: "",
		},
	}
	return r, cdiConfig
}

func createStorageClassList(storageClasses ...storagev1.StorageClass) *storagev1.StorageClassList {
	list := &storagev1.StorageClassList{
		Items: []storagev1.StorageClass{},
	}
	list.Items = append(list.Items, storageClasses...)
	return list
}

func createRouteList(routes ...routev1.Route) *routev1.RouteList {
	list := &routev1.RouteList{
		Items: []routev1.Route{},
	}
	list.Items = append(list.Items, routes...)
	return list
}

func createRoute(name, ns, service string) *routev1.Route {
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: service,
			},
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{Host: testRouteURL},
			},
		},
	}
}

func createIngressList(ingresses ...networkingv1.Ingress) *networkingv1.IngressList {
	list := &networkingv1.IngressList{
		Items: []networkingv1.Ingress{},
	}
	list.Items = append(list.Items, ingresses...)
	return list
}

func createIngress(name, ns, service, url string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{Name: service},
			},
			Rules: []networkingv1.IngressRule{
				{Host: url},
			},
		},
	}
}

func createNoDefaultBackendIngress(name, ns, service, url string) *networkingv1.Ingress {
	res := createIngress(name, ns, service, url)
	res.Spec.DefaultBackend = nil
	return res
}

func createNoServiceIngress(name, ns, service, url string) *networkingv1.Ingress {
	res := createIngress(name, ns, service, url)
	res.Spec.DefaultBackend.Service = nil
	return res
}

func createNoRulesIngress(name, ns, service, url string) *networkingv1.Ingress {
	res := createIngress(name, ns, service, url)
	res.Spec.Rules = []networkingv1.IngressRule{}
	return res
}

func createConfigMap(name, namespace string) *corev1.ConfigMap {
	isController := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel: "cdi.kubevirt.io",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "some owner",
					Controller: &isController,
				},
			},
		},
	}
	return cm
}

func createDefaultPodResourceRequirements(limitCPUValue string, limitMemoryValue string, requestCPUValue string, requestMemoryValue string) *corev1.ResourceRequirements {
	if limitCPUValue == "" {
		limitCPUValue = defaultCPULimit
	}
	cpuLimit, err := resource.ParseQuantity(limitCPUValue)
	Expect(err).ToNot(HaveOccurred())
	if limitMemoryValue == "" {
		limitMemoryValue = defaultMemLimit
	}
	memLimit, err := resource.ParseQuantity(limitMemoryValue)
	Expect(err).ToNot(HaveOccurred())
	if requestCPUValue == "" {
		requestCPUValue = defaultCPURequest
	}
	cpuRequest, err := resource.ParseQuantity(requestCPUValue)
	Expect(err).ToNot(HaveOccurred())
	if requestMemoryValue == "" {
		requestMemoryValue = defaultMemRequest
	}
	memRequest, err := resource.ParseQuantity(requestMemoryValue)
	Expect(err).ToNot(HaveOccurred())
	return &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memLimit},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memRequest},
	}
}

func createClusterWideProxy(HTTPProxy string, HTTPSProxy string, noProxy string, trustedCAName string) *ocpconfigv1.Proxy {
	proxy := &ocpconfigv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterWideProxyName,
			UID:  types.UID(ClusterWideProxyAPIKind + "-" + ClusterWideProxyName),
		},
		Spec: ocpconfigv1.ProxySpec{
			HTTPProxy:          HTTPProxy,
			HTTPSProxy:         HTTPSProxy,
			NoProxy:            noProxy,
			ReadinessEndpoints: []string{},
			TrustedCA: ocpconfigv1.ConfigMapNameReference{
				Name: trustedCAName,
			},
		},
		Status: ocpconfigv1.ProxyStatus{
			HTTPProxy:  HTTPProxy,
			HTTPSProxy: HTTPSProxy,
			NoProxy:    noProxy,
		},
	}
	return proxy
}

func createClusterWideProxyCAConfigMap(certBytes string) *corev1.ConfigMap {
	configMap := &v1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: ClusterWideProxyConfigMapName, Namespace: ClusterWideProxyConfigMapNameSpace},
		Immutable:  new(bool),
		Data:       map[string]string{ClusterWideProxyConfigMapKey: certBytes},
		BinaryData: map[string][]byte{},
	}
	return configMap
}

func createImportProxy(http, https, noproxy, ca string) *cdiv1.ImportProxy {
	p := &cdiv1.ImportProxy{
		HTTPProxy:      &http,
		HTTPSProxy:     &https,
		NoProxy:        &noproxy,
		TrustedCAProxy: &ca,
	}
	return p
}

func generateCertificate(hostnames ...string) (caCert string, signedCert string) {
	// Generate a self-signed CA
	key, err := cert.NewPrivateKey()
	Expect(err).ToNot(HaveOccurred())

	ca, err := cert.NewSelfSignedCACert(cert.Config{CommonName: "www.test-root-ca.com"}, key)
	Expect(err).ToNot(HaveOccurred())
	ca.DNSNames = append(ca.DNSNames, ca.Subject.CommonName)

	// Create a certificate signed by the CA
	// Hide the hostnames inside the altnames for more robust testing
	signed, err := cert.NewSignedCert(cert.Config{
		CommonName: "www.example.com",
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		AltNames: cert.AltNames{
			DNSNames: hostnames,
		},
	}, key, ca, key)
	Expect(err).ToNot(HaveOccurred())

	// Get rid of trailing newlines
	caCert = strings.TrimSpace(string(cert.EncodeCertPEM(ca)))
	signedCert = strings.TrimSpace(string(cert.EncodeCertPEM(signed)))
	return caCert, signedCert
}
