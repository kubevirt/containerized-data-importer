package controller

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	storagev1 "k8s.io/api/storage/v1"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdifake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
)

const (
	testURL         = "www.this.is.a.test.org"
	testRouteURL    = "cdi-uploadproxy.example.com"
	testServiceName = "cdi-proxyurl"
	testNamespace   = "cdi-test"
)

var (
	log = logf.Log.WithName("config-controller-test")
)

var _ = Describe("CDIConfig Controller reconcile loop", func() {
	It("Should not update if no changes happened", func() {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace))
		err := reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		_, err = reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		// CDIConfig generated, now reconcile again without changes.
		_, err = reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should set proxyURL to override if no ingress or route exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace))
		_, err := reconciler.Reconcile(reconcile.Request{})
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		override := "www.override-something.org.tt.test"
		cdiConfig.Spec.UploadProxyURLOverride = &override
		// Update the config object in the fake client go, would normally use an informer, but too much work
		reconciler.CdiClient = cdifake.NewSimpleClientset(cdiConfig)
		err = reconciler.Client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(override).To(Equal(*cdiConfig.Status.UploadProxyURL))
	})

	It("Should set proxyURL to override if ingress or route exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createConfigMap(operator.ConfigMapName, testNamespace),
			createIngressList(
				*createIngress("test-ingress", "test-ns", testServiceName, testURL),
			),
		)
		_, err := reconciler.Reconcile(reconcile.Request{})
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		override := "www.override-something.org.tt.test"
		cdiConfig.Spec.UploadProxyURLOverride = &override
		// Update the config object in the fake client go, would normally use an informer, but too much work
		reconciler.CdiClient = cdifake.NewSimpleClientset(cdiConfig)
		err = reconciler.Client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: reconciler.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(override).To(Equal(*cdiConfig.Status.UploadProxyURL))
	})
})

var _ = Describe("Controller ingress reconcile loop", func() {
	It("Should set uploadProxyUrl to nil if no Ingress exists", func() {
		reconciler, cdiConfig := createConfigReconciler()
		err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if ingress with correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress", "test-ns", testServiceName, testURL),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testURL))
	})

	It("Should not set uploadProxyUrl if ingress with incorrect serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress", "test-ns", "incorrect", testURL),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if multiple ingresses exist with one correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createIngressList(
			*createIngress("test-ingress1", "test-ns", "service1", "invalidurl"),
			*createIngress("test-ingress2", "test-ns", "service2", "invalidurl2"),
			*createIngress("test-ingress3", "test-ns", testServiceName, testURL),
			*createIngress("test-ingress4", "test-ns", "service3", "invalidurl3"),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileIngress(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testURL))
	})
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
			*createRoute("test-ingress", "test-ns", testServiceName),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testRouteURL))
	})

	It("Should not set uploadProxyUrl if ingress with incorrect serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress", "test-ns", "incorrect"),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.UploadProxyURL).To(BeNil())
	})

	It("Should set uploadProxyUrl correctly if multiple ingresses exist with one correct serviceName exists", func() {
		reconciler, cdiConfig := createConfigReconciler(createRouteList(
			*createRoute("test-ingress1", "test-ns", "service1"),
			*createRoute("test-ingress2", "test-ns", "service2"),
			*createRoute("test-ingress3", "test-ns", testServiceName),
			*createRoute("test-ingress4", "test-ns", "service3"),
		))
		reconciler.UploadProxyServiceName = testServiceName
		err := reconciler.reconcileRoute(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(*cdiConfig.Status.UploadProxyURL).To(Equal(testRouteURL))
	})
})

var _ = Describe("Controller storage class reconcile loop", func() {
	It("Should set the scratchspaceStorageClass to blank if there is no default sc", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*createStorageClass("test-default-sc", nil),
		))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal(""))
	})

	It("Should set the scratchspaceStorageClass to the default without override", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*createStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			},
			)))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})

	It("Should set the scratchspaceStorageClass to the default without override and multiple sc", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*createStorageClass("test-sc3", nil),
			*createStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*createStorageClass("test-sc", nil),
			*createStorageClass("test-sc2", nil),
		))
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})

	It("Should set the scratchspaceStorageClass to the override even with default", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*createStorageClass("test-sc3", nil),
			*createStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*createStorageClass("test-sc", nil),
			*createStorageClass("test-sc2", nil),
		))
		override := "test-sc"
		cdiConfig.Spec.ScratchSpaceStorageClass = &override
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal(override))
	})

	It("Should set the scratchspaceStorageClass to the default with invalid override", func() {
		reconciler, cdiConfig := createConfigReconciler(createStorageClassList(
			*createStorageClass("test-sc3", nil),
			*createStorageClass("test-default-sc", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
			*createStorageClass("test-sc", nil),
			*createStorageClass("test-sc2", nil),
		))
		override := "invalid"
		cdiConfig.Spec.ScratchSpaceStorageClass = &override
		err := reconciler.reconcileStorageClass(cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(cdiConfig.Status.ScratchSpaceStorageClass).To(Equal("test-default-sc"))
	})
})

func createConfigReconciler(objects ...runtime.Object) (*CDIConfigReconciler, *cdiv1.CDIConfig) {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
	objs = append(objs, cdiConfig)
	cdifakeclientset := cdifake.NewSimpleClientset(cdiConfig)
	k8sfakeclientset := k8sfake.NewSimpleClientset()
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	extensionsv1beta1.AddToScheme(s)
	routev1.AddToScheme(s)
	storagev1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &CDIConfigReconciler{
		Client:       cl,
		Scheme:       s,
		Log:          log,
		ConfigName:   "cdiconfig",
		CDINamespace: testNamespace,
		CdiClient:    cdifakeclientset,
		K8sClient:    k8sfakeclientset,
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
	route := &routev1.Route{
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
	return route
}

func createIngressList(ingresses ...extensionsv1beta1.Ingress) *extensionsv1beta1.IngressList {
	list := &extensionsv1beta1.IngressList{
		Items: []extensionsv1beta1.Ingress{},
	}
	list.Items = append(list.Items, ingresses...)
	return list
}

func createIngress(name, ns, service, url string) *extensionsv1beta1.Ingress {
	return &extensionsv1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: extensionsv1beta1.IngressSpec{
			Backend: &extensionsv1beta1.IngressBackend{
				ServiceName: service,
			},
			Rules: []extensionsv1beta1.IngressRule{
				{Host: url},
			},
		},
	}
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
