package controller

import (
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	routefake "github.com/openshift/client-go/route/clientset/versioned/fake"
	routeinformers "github.com/openshift/client-go/route/informers/externalversions"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
	informers "kubevirt.io/containerized-data-importer/pkg/client/informers/externalversions"
)

type configFixture struct {
	t *testing.T

	client      *fake.Clientset
	kubeclient  *k8sfake.Clientset
	routeClient *routefake.Clientset

	// Objects to put in the store.
	configLister  []*cdiv1.CDIConfig
	ingressLister []*extensionsv1beta1.Ingress
	routeLister   []*routev1.Route

	// Actions expected to happen on the client.
	kubeactions  []core.Action
	actions      []core.Action
	routeactions []core.Action

	// Objects from here preloaded into NewSimpleFake.
	kubeobjects  []runtime.Object
	objects      []runtime.Object
	routeobjects []runtime.Object
}

func newConfigFixture(t *testing.T) *configFixture {
	f := &configFixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

func getConfigKey(config *cdiv1.CDIConfig, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(config)
	if err != nil {
		t.Errorf("Unexpected error getting key for config %v: %v", config.Name, err)
		return ""
	}
	return key
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
				{Host: "cdi-uploadproxy.example.com"},
			},
		},
	}
	return route
}

func createIngress(name, ns, service, url string, labels map[string]string) *extensionsv1beta1.Ingress {
	return &extensionsv1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
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

func (f *configFixture) newController() (*ConfigController, informers.SharedInformerFactory, routeinformers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	f.routeClient = routefake.NewSimpleClientset(f.routeobjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	r := routeinformers.NewSharedInformerFactory(f.routeClient, noResyncPeriodFunc())

	for _, conf := range f.configLister {
		i.Cdi().V1alpha1().CDIConfigs().Informer().GetIndexer().Add(conf)
	}

	for _, ing := range f.ingressLister {
		k8sI.Extensions().V1beta1().Ingresses().Informer().GetIndexer().Add(ing)
	}

	for _, route := range f.routeLister {
		r.Route().V1().Routes().Informer().GetIndexer().Add(route)
	}

	c := NewConfigController(f.kubeclient,
		f.client,
		k8sI.Extensions().V1beta1().Ingresses(),
		r.Route().V1().Routes(),
		i.Cdi().V1alpha1().CDIConfigs(),
		"cdi-uploadproxy",
		"testConfig",
		"Always",
		"5")

	c.ingressesSynced = alwaysReady
	c.routesSynced = alwaysReady
	c.configsSynced = alwaysReady

	return c, i, r, k8sI
}

func (f *configFixture) run(configName string) {
	f.runController(configName, true, false)
}

func (f *configFixture) runController(configName string, startInformers bool, expectError bool) {
	c, i, r, k8sI := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
		r.Start(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncHandler(configName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkConfigAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	routeactions := filterInformerActions(f.routeClient.Actions())
	for i, action := range routeactions {
		if len(f.routeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(routeactions)-len(f.routeactions), routeactions[i:])
			break
		}

		expectedAction := f.routeactions[i]
		checkConfigAction(expectedAction, action, f.t)
	}

	if len(f.routeactions) > len(routeactions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.routeactions)-len(routeactions), f.routeactions[len(routeactions):])
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkConfigAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}

}

func filterConfigInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "ingresses") ||
				action.Matches("watch", "ingresses") ||
				action.Matches("list", "routes") ||
				action.Matches("watch", "routes") ||
				action.Matches("list", "cdiconfigs") ||
				action.Matches("watch", "cdiconfigs")) {
			continue
		}
		ret = append(ret, action)
	}
	return ret
}

func checkConfigAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, expPatch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch))
		}
	}
}

func (f *configFixture) expectUpdateConfigAction(config *cdiv1.CDIConfig) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Resource: "cdiconfigs", Version: "v1alpha1"}, config.Namespace, config)
	f.actions = append(f.actions, action)
}

func TestCreatesCDIConfig(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	f.run(getConfigKey(config, t))
}

func TestCDIConfigStatusChanged(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")
	url := "www.example.com"
	config.Spec.UploadProxyURLOverride = &url

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	result := config.DeepCopy()
	result.Status.UploadProxyURL = &url

	f.expectUpdateConfigAction(result)

	f.run(getConfigKey(config, t))
}

func TestCreatesRoute(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	route := createRoute("testRoute", "default", "cdi-uploadproxy")

	f.routeLister = append(f.routeLister, route)

	url := route.Status.Ingress[0].Host

	result := config.DeepCopy()
	result.Status.UploadProxyURL = &url

	f.expectUpdateConfigAction(result)

	f.run(getConfigKey(config, t))
}

func TestCreatesRouteOverrideExists(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")
	newURL := "www.override.com"
	config.Spec.UploadProxyURLOverride = &newURL

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	route := createRoute("testRoute", "default", "cdi-uploadproxy")

	f.routeLister = append(f.routeLister, route)

	result := config.DeepCopy()
	result.Status.UploadProxyURL = &newURL

	f.expectUpdateConfigAction(result)

	f.run(getConfigKey(config, t))
}

func TestCreatesRouteDifferentService(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	route := createRoute("testRoute", "default", "other-service")

	f.routeLister = append(f.routeLister, route)

	f.run(getConfigKey(config, t))
}
func TestCreatesIngress(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	url := "www.example.com"
	ing := createIngress("ing", "default", "cdi-uploadproxy", url, map[string]string{"app": ""})

	f.ingressLister = append(f.ingressLister, ing)
	f.kubeobjects = append(f.kubeobjects, ing)

	result := config.DeepCopy()
	result.Status.UploadProxyURL = &url
	f.expectUpdateConfigAction(result)

	f.run(getConfigKey(config, t))
}

func TestCreatesIngressOverrideExists(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")
	newURL := "www.override.com"
	config.Spec.UploadProxyURLOverride = &newURL

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	url := "www.example.com"
	ing := createIngress("ing", "default", "cdi-uploadproxy", url, map[string]string{"app": ""})

	f.ingressLister = append(f.ingressLister, ing)
	f.kubeobjects = append(f.kubeobjects, ing)

	result := config.DeepCopy()
	result.Status.UploadProxyURL = &newURL
	f.expectUpdateConfigAction(result)

	f.run(getConfigKey(config, t))
}

func TestCreatesIngressDifferentService(t *testing.T) {
	f := newConfigFixture(t)
	config := createCDIConfig("testConfig", "default")

	f.configLister = append(f.configLister, config)
	f.objects = append(f.objects, config)

	url := "www.example.com"
	ing := createIngress("ing", "default", "other-service", url, map[string]string{"app": ""})

	f.ingressLister = append(f.ingressLister, ing)
	f.kubeobjects = append(f.kubeobjects, ing)

	f.run(getConfigKey(config, t))
}
