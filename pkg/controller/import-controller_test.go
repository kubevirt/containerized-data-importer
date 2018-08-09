package controller

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	. "kubevirt.io/containerized-data-importer/pkg/common"
)

type importFixture struct {
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

func newImportFixture(t *testing.T) *importFixture {
	f := &importFixture{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	return f
}

func (f *importFixture) newController() *ImportController {

	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	podFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	pvcFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	podInformer := podFactory.Core().V1().Pods()
	pvcInformer := pvcFactory.Core().V1().PersistentVolumeClaims()

	c := NewImportController(f.kubeclient,
		pvcInformer,
		podInformer,
		"test/myimage",
		"Always",
		"5")

	for _, pod := range f.podLister {
		c.podInformer.GetIndexer().Add(pod)
	}

	for _, pvc := range f.pvcLister {
		c.pvcInformer.GetIndexer().Add(pvc)
	}

	return c
}

func (f *importFixture) run(pvcName string) {
	f.runController(pvcName, true, false, false)
}

func (f *importFixture) runWithExpectation(pvcName string) {
	f.runController(pvcName, true, false, true)
}

func (f *importFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true, false)
}

func (f *importFixture) runController(pvcName string,
	startInformers bool,
	expectError bool,
	withCreateExpectation bool) {
	c := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		go c.pvcInformer.Run(stopCh)
		go c.podInformer.Run(stopCh)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
	}

	if withCreateExpectation {
		c.expectPodCreate(pvcName)
	}
	err := c.syncPvc(pvcName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing pvc: %s: %v", pvcName, err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing pvc, got nil")
	}

	k8sActions := filterImportActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkImportAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkImportAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkImportAction(expected, actual core.Action, t *testing.T) {
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

// filterImportActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterImportActions(actions []core.Action) []core.Action {
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

func (f *importFixture) expectCreatePodAction(d *corev1.Pod) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "pods", Version: "v1"}, d.Namespace, d))
}

func (f *importFixture) expectUpdatePvcAction(d *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, d.Namespace, d))
}

// Verifies basic pod creation when new PVC is discovered
func TestCreatesImportPod(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	expPod := createPod(pvc, DataVolName)

	f.expectCreatePodAction(expPod)

	f.run(getPvcKey(pvc, t))
}

// Verifies pod creation does not occur when waiting for expectation.
func TestImportPodCreationExpectation(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Labels = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}
	f.expectUpdatePvcAction(expPvc)

	f.runWithExpectation(getPvcKey(pvc, t))
}

// Verifies pod creation is observed and pvc labels are set.
func TestImportObservePod(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pod := createPod(pvc, DataVolName)
	pod.Name = "madeup-name"
	pod.Status.Phase = corev1.PodPending
	pod.Namespace = pvc.Namespace

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, pod)

	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Labels = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}
	expPvc.ObjectMeta.Annotations = map[string]string{AnnImportPod: pod.Name, AnnPodPhase: string(corev1.PodPending), AnnEndpoint: "http://test"}

	f.expectUpdatePvcAction(expPvc)

	f.run(getPvcKey(pvc, t))
}

// Verifies pod status updates are reflected in PVC annotations
func TestImportPodStatusUpdating(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	pod := createPod(pvc, DataVolName)
	pod.Name = "madeup-name"
	pod.Status.Phase = corev1.PodRunning
	pod.Namespace = pvc.Namespace

	pvc.ObjectMeta.Annotations = map[string]string{AnnImportPod: pod.Name, AnnPodPhase: string(corev1.PodPending), AnnEndpoint: "http://test"}
	pvc.ObjectMeta.Labels = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, pod)

	// expecting pvc's pod status annotation to be updated from pending => running
	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Annotations = map[string]string{AnnImportPod: pod.Name, AnnPodPhase: string(pod.Status.Phase), AnnEndpoint: "http://test"}

	f.expectUpdatePvcAction(expPvc)

	f.run(getPvcKey(pvc, t))
}

func TestImportFindPodInCacheUpdating(t *testing.T) {

	f := newImportFixture(t)

	tests := []struct {
		pvc *corev1.PersistentVolumeClaim
		pod *corev1.Pod
	}{
		{
			pvc: createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil),
		},
		{
			pvc: createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test"}, nil),
		},
		{
			pvc: createPvc("testPvc3", "default", map[string]string{AnnEndpoint: "http://test"}, nil),
		},
	}

	for idx, test := range tests {
		test.pod = createPod(test.pvc, DataVolName)
		test.pod.Namespace = test.pvc.Namespace
		test.pod.Name = fmt.Sprintf("fakename%d", idx)

		f.pvcLister = append(f.pvcLister, test.pvc)
		f.podLister = append(f.podLister, test.pod)

		f.kubeobjects = append(f.kubeobjects, test.pvc)
		f.kubeobjects = append(f.kubeobjects, test.pod)
		tests[idx] = test
	}

	controller := f.newController()

	stopCh := make(chan struct{})
	defer close(stopCh)
	go controller.pvcInformer.Run(stopCh)
	go controller.podInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, controller.pvcInformer.HasSynced)
	cache.WaitForCacheSync(stopCh, controller.pvcInformer.HasSynced)

	for _, test := range tests {
		foundPod, err := controller.findImportPodFromCache(test.pvc)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if foundPod == nil {
			t.Errorf("didn't find pod for pvc %v", test.pvc)
		}
		if !reflect.DeepEqual(foundPod, test.pod) {
			t.Errorf("wrong pod found.\nfound %v\nwant %v", foundPod, test.pod)
		}
	}

}

// verifies no work is done on pvcs without our annotations
func TestImportIgnorePVC(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", nil, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.runWithExpectation(getPvcKey(pvc, t))
}

// verify error if ownership doesn't match
func TestImportOwnership(t *testing.T) {
	f := newImportFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pod := createPod(pvc, DataVolName)
	pod.Name = "madeup-name"
	pod.Status.Phase = corev1.PodPending
	pod.Namespace = pvc.Namespace
	pod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.runExpectError(getPvcKey(pvc, t))
}
