package controller

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

type ControllerFixture struct {
	t *testing.T

	kubeclient *k8sfake.Clientset

	// Objects to put in the store.
	pvcLister []*corev1.PersistentVolumeClaim
	podLister []*corev1.Pod

	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action

	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
}

func newControllerFixture(t *testing.T) *ControllerFixture {
	f := &ControllerFixture{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	return f
}

func (f *ControllerFixture) newController(image, pullPolicy, verbose string) *Controller {

	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	podFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	pvcFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	podInformer := podFactory.Core().V1().Pods()
	pvcInformer := pvcFactory.Core().V1().PersistentVolumeClaims()

	c := NewController(f.kubeclient,
		pvcInformer,
		podInformer,
		image,
		pullPolicy,
		verbose)

	for _, pod := range f.podLister {
		c.podInformer.GetIndexer().Add(pod)
	}

	for _, pvc := range f.pvcLister {
		c.pvcInformer.GetIndexer().Add(pvc)
	}

	return c
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
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

// filterActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "persistentvolumeclaims") ||
				action.Matches("watch", "persistentvolumeclaims") ||
				action.Matches("list", "pods") ||
				action.Matches("watch", "pods")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *ControllerFixture) expectCreatePodAction(d *corev1.Pod) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "pods", Version: "v1"}, d.Namespace, d))
}

func (f *ControllerFixture) expectUpdatePvcAction(d *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, d.Namespace, d))
}
