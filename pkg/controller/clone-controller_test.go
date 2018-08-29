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

type cloneFixture struct {
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

func newCloneFixture(t *testing.T) *cloneFixture {
	f := &cloneFixture{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	return f
}

func (f *cloneFixture) newController() *CloneController {

	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	podFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	pvcFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	podInformer := podFactory.Core().V1().Pods()
	pvcInformer := pvcFactory.Core().V1().PersistentVolumeClaims()

	c := NewCloneController(f.kubeclient,
		pvcInformer,
		podInformer,
		"test/mycloneimage",
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

func (f *cloneFixture) run(pvcName string) {
	f.runController(pvcName, true, false, false)
}

func (f *cloneFixture) runWithExpectation(pvcName string) {
	f.runController(pvcName, true, false, true)
}

func (f *cloneFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true, false)
}

func (f *cloneFixture) runController(pvcName string,
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

	k8sActions := filterCloneActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkCloneAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkCloneAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkCloneAction(expected, actual core.Action, t *testing.T) {
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

// filterCloneActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterCloneActions(actions []core.Action) []core.Action {
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

func (f *cloneFixture) expectCreatePodAction(d *corev1.Pod) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "pods", Version: "v1"}, d.Namespace, d))
}

func (f *cloneFixture) expectUpdatePvcAction(d *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, d.Namespace, d))
}

// Verifies basic pods creation when new PVC is discovered
func TestCreatesClonePods(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	pvcUid := string(pvc.GetUID())
	expSourcePod := createSourcePod(pvc, DataVolName, pvcUid)
	f.expectCreatePodAction(expSourcePod)
	expTargetPod := createTargetPod(pvc, DataVolName, pvcUid, "source-ns")
	f.expectCreatePodAction(expTargetPod)

	f.run(getPvcKey(pvc, t))
}

// Verifies pods creation is observed and pvc labels are set.
func TestCloneObservePod(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("trget-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	pvcUid := string(pvc.GetUID())

	sourcePod := createSourcePod(pvc, DataVolName, pvcUid)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodPending
	sourcePod.Namespace = "source-ns"

	targetPod := createTargetPod(pvc, DataVolName, pvcUid, sourcePod.Namespace)
	targetPod.Name = "madeup-target-name"
	targetPod.Status.Phase = corev1.PodPending
	targetPod.Namespace = "target-ns"

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, sourcePod)
	f.podLister = append(f.podLister, targetPod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePod)
	f.kubeobjects = append(f.kubeobjects, targetPod)

	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Labels = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}
	expPvc.ObjectMeta.Annotations = map[string]string{AnnClonePodPhase: string(corev1.PodPending), AnnCloneRequest: "source-ns/golden-pvc"}

	f.expectUpdatePvcAction(expPvc)

	f.run(getPvcKey(pvc, t))
}

// Verifies pods status updates are reflected in PVC annotations
func TestClonePodStatusUpdating(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	pvcUid := string(pvc.GetUID())
	sourcePod := createSourcePod(pvc, DataVolName, pvcUid)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodRunning
	sourcePod.Namespace = "source-ns"

	targetPod := createTargetPod(pvc, DataVolName, pvcUid, sourcePod.Namespace)
	targetPod.Name = "madeup-target-name"
	targetPod.Status.Phase = corev1.PodRunning
	targetPod.Namespace = "target-ns"

	pvc.ObjectMeta.Annotations = map[string]string{AnnClonePodPhase: string(corev1.PodPending), AnnCloneRequest: "source-ns/golden-pvc"}
	pvc.ObjectMeta.Labels = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, sourcePod)
	f.podLister = append(f.podLister, targetPod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePod)
	f.kubeobjects = append(f.kubeobjects, targetPod)

	// expecting pvc's pod status annotation to be updated from pending => running
	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Annotations = map[string]string{AnnClonePodPhase: string(targetPod.Status.Phase), AnnCloneRequest: "source-ns/golden-pvc"}

	f.expectUpdatePvcAction(expPvc)

	f.run(getPvcKey(pvc, t))
}

// verifies no work is done on pvcs without our annotations
func TestCloneIgnorePVC(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("target-pvc", "target-ns", nil, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.runWithExpectation(getPvcKey(pvc, t))
}

// verify error if ownership doesn't match
func TestCloneOwnership(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	pvcUid := string(pvc.GetUID())
	sourcePod := createSourcePod(pvc, DataVolName, pvcUid)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodPending
	sourcePod.Namespace, _ = ParseSourcePvcAnnotation(pvc.GetAnnotations()[AnnCloneRequest], "/")
	sourcePod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	targetPod := createTargetPod(pvc, DataVolName, pvcUid, sourcePod.Namespace)
	targetPod.Status.Phase = corev1.PodPending
	targetPod.Namespace = pvc.Namespace
	targetPod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, sourcePod)
	f.podLister = append(f.podLister, targetPod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePod)
	f.kubeobjects = append(f.kubeobjects, targetPod)

	f.runExpectError(getPvcKey(pvc, t))
}

func TestCloneFindPodsInCacheUpdating(t *testing.T) {

	f := newCloneFixture(t)

	tests := []struct {
		pvc       *corev1.PersistentVolumeClaim
		sourcePod *corev1.Pod
		targetPod *corev1.Pod
	}{
		{
			pvc: createPvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil),
		},
		{
			pvc: createPvc("target-pvc2", "target2-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil),
		},
	}

	pvcUid := string(tests[0].pvc.GetUID())
	tests[0].sourcePod = createSourcePod(tests[0].pvc, DataVolName, pvcUid)
	tests[0].sourcePod.Namespace = "source-ns"
	tests[0].sourcePod.Name = fmt.Sprintf("fakesourcename%d", 1)

	tests[0].targetPod = createTargetPod(tests[0].pvc, DataVolName, pvcUid, tests[0].sourcePod.Namespace)
	tests[0].targetPod.Namespace = "target-ns"
	tests[0].sourcePod.Name = fmt.Sprintf("faketargetname%d", 1)

	f.pvcLister = append(f.pvcLister, tests[0].pvc)
	f.podLister = append(f.podLister, tests[0].sourcePod)
	f.podLister = append(f.podLister, tests[0].targetPod)

	f.kubeobjects = append(f.kubeobjects, tests[0].pvc)
	f.kubeobjects = append(f.kubeobjects, tests[0].sourcePod)
	f.kubeobjects = append(f.kubeobjects, tests[0].targetPod)

	tests[1].sourcePod = createSourcePod(tests[1].pvc, DataVolName, pvcUid)
	tests[1].sourcePod.Namespace = "source-ns"
	tests[1].sourcePod.Name = fmt.Sprintf("fakesourcename%d", 2)

	tests[1].targetPod = createTargetPod(tests[1].pvc, DataVolName, pvcUid, tests[1].sourcePod.Namespace)
	tests[1].targetPod.Namespace = "target2-ns"
	tests[1].sourcePod.Name = fmt.Sprintf("faketargetname%d", 2)

	f.pvcLister = append(f.pvcLister, tests[1].pvc)
	f.podLister = append(f.podLister, tests[1].sourcePod)
	f.podLister = append(f.podLister, tests[1].targetPod)

	f.kubeobjects = append(f.kubeobjects, tests[1].pvc)
	f.kubeobjects = append(f.kubeobjects, tests[1].sourcePod)
	f.kubeobjects = append(f.kubeobjects, tests[1].targetPod)

	controller := f.newController()

	stopCh := make(chan struct{})
	defer close(stopCh)
	go controller.pvcInformer.Run(stopCh)
	go controller.podInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, controller.pvcInformer.HasSynced)
	cache.WaitForCacheSync(stopCh, controller.pvcInformer.HasSynced)

	for _, test := range tests {
		foundSourcePod, foundTargetPod, err := controller.findClonePodsFromCache(test.pvc)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if foundSourcePod == nil {
			t.Errorf("didn't find source pod for pvc %v", test.pvc)
		}
		if !reflect.DeepEqual(foundSourcePod, test.sourcePod) {
			t.Errorf("wrong source pod found.\nfound %v\nwant %v", foundSourcePod, test.sourcePod)
		}
		if foundTargetPod == nil {
			t.Errorf("didn't find target pod for pvc %v", test.pvc)
		}
		if !reflect.DeepEqual(foundSourcePod, test.sourcePod) {
			t.Errorf("wrong source pod found.\nfound %v\nwant %v", foundSourcePod, test.sourcePod)
		}
	}
}
