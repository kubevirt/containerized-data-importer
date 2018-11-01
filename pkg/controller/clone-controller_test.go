package controller

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	. "kubevirt.io/containerized-data-importer/pkg/common"
)

type CloneFixture struct {
	ControllerFixture
}

func newCloneFixture(t *testing.T) *CloneFixture {
	f := &CloneFixture{
		ControllerFixture: *newControllerFixture(t),
	}
	return f
}

func (f *CloneFixture) newCloneController() *CloneController {
	return &CloneController{
		Controller: *f.newController("test/mycloneimage", "Always", "5"),
	}
}

func (f *CloneFixture) run(pvcName string) {
	f.runController(pvcName, true, false, false)
}

func (f *CloneFixture) runWithExpectation(pvcName string) {
	f.runController(pvcName, true, false, true)
}

func (f *CloneFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true, false)
}

func (f *CloneFixture) runController(pvcName string,
	startInformers bool,
	expectError bool,
	withCreateExpectation bool) {
	c := f.newCloneController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		go c.pvcInformer.Run(stopCh)
		go c.podInformer.Run(stopCh)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
	}

	if withCreateExpectation {
		c.raisePodCreate(pvcName)
	}
	err := c.syncPvc(pvcName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing pvc: %s: %v", pvcName, err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing pvc, got nil")
	}

	k8sActions := filterActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// Verifies basic pods creation when new PVC is discovered
func TestCreatesClonePods(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	id := string(pvc.GetUID())
	expSourcePod := createSourcePod(pvc, id)
	f.expectCreatePodAction(expSourcePod)
	expTargetPod := createTargetPod(pvc, id, "source-ns")
	f.expectCreatePodAction(expTargetPod)

	f.run(getPvcKey(pvc, t))
}

// Verifies pods creation is observed and pvc labels are set.
func TestCloneObservePod(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("trget-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	id := string(pvc.GetUID())

	sourcePod := createSourcePod(pvc, id)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodPending
	sourcePod.Namespace = "source-ns"

	targetPod := createTargetPod(pvc, id, sourcePod.Namespace)
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
	expPvc.ObjectMeta.Labels = map[string]string{CDILabelKey: CDILabelValue}
	expPvc.ObjectMeta.Annotations = map[string]string{AnnPodPhase: string(corev1.PodPending), AnnCloneRequest: "source-ns/golden-pvc"}

	f.expectUpdatePvcAction(expPvc)

	f.run(getPvcKey(pvc, t))
}

// Verifies pods status updates are reflected in PVC annotations
func TestClonePodStatusUpdating(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	id := string(pvc.GetUID())
	sourcePod := createSourcePod(pvc, id)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodRunning
	sourcePod.Namespace = "source-ns"

	targetPod := createTargetPod(pvc, id, sourcePod.Namespace)
	targetPod.Name = "madeup-target-name"
	targetPod.Status.Phase = corev1.PodRunning
	targetPod.Namespace = "target-ns"

	pvc.ObjectMeta.Annotations = map[string]string{AnnPodPhase: string(corev1.PodPending), AnnCloneRequest: "source-ns/golden-pvc"}
	pvc.ObjectMeta.Labels = map[string]string{CDILabelKey: CDILabelValue}

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, sourcePod)
	f.podLister = append(f.podLister, targetPod)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePod)
	f.kubeobjects = append(f.kubeobjects, targetPod)

	// expecting pvc's pod status annotation to be updated from pending => running
	expPvc := pvc.DeepCopy()
	expPvc.ObjectMeta.Annotations = map[string]string{AnnPodPhase: string(targetPod.Status.Phase), AnnCloneRequest: "source-ns/golden-pvc"}

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
	id := string(pvc.GetUID())
	sourcePod := createSourcePod(pvc, id)
	sourcePod.Name = "madeup-source-name"
	sourcePod.Status.Phase = corev1.PodPending
	sourcePod.Namespace, _ = ParseSourcePvcAnnotation(pvc.GetAnnotations()[AnnCloneRequest], "/")
	sourcePod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	targetPod := createTargetPod(pvc, id, sourcePod.Namespace)
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

	id := string(tests[0].pvc.GetUID())
	tests[0].sourcePod = createSourcePod(tests[0].pvc, id)
	tests[0].sourcePod.Namespace = "source-ns"
	tests[0].sourcePod.Name = fmt.Sprintf("fakesourcename%d", 1)

	tests[0].targetPod = createTargetPod(tests[0].pvc, id, tests[0].sourcePod.Namespace)
	tests[0].targetPod.Namespace = "target-ns"
	tests[0].sourcePod.Name = fmt.Sprintf("faketargetname%d", 1)

	f.pvcLister = append(f.pvcLister, tests[0].pvc)
	f.podLister = append(f.podLister, tests[0].sourcePod)
	f.podLister = append(f.podLister, tests[0].targetPod)

	f.kubeobjects = append(f.kubeobjects, tests[0].pvc)
	f.kubeobjects = append(f.kubeobjects, tests[0].sourcePod)
	f.kubeobjects = append(f.kubeobjects, tests[0].targetPod)

	tests[1].sourcePod = createSourcePod(tests[1].pvc, id)
	tests[1].sourcePod.Namespace = "source-ns"
	tests[1].sourcePod.Name = fmt.Sprintf("fakesourcename%d", 2)

	tests[1].targetPod = createTargetPod(tests[1].pvc, id, tests[1].sourcePod.Namespace)
	tests[1].targetPod.Namespace = "target2-ns"
	tests[1].sourcePod.Name = fmt.Sprintf("faketargetname%d", 2)

	f.pvcLister = append(f.pvcLister, tests[1].pvc)
	f.podLister = append(f.podLister, tests[1].sourcePod)
	f.podLister = append(f.podLister, tests[1].targetPod)

	f.kubeobjects = append(f.kubeobjects, tests[1].pvc)
	f.kubeobjects = append(f.kubeobjects, tests[1].sourcePod)
	f.kubeobjects = append(f.kubeobjects, tests[1].targetPod)

	controller := f.newCloneController()

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
