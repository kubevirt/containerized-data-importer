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

type ImportFixture struct {
	ControllerFixture
}

func newImportFixture(t *testing.T) *ImportFixture {
	f := &ImportFixture{
		ControllerFixture: *newControllerFixture(t),
	}
	return f
}

func (f *ImportFixture) newImportController() *ImportController {
	return &ImportController{
		Controller: *f.newController("test/myimage", "Always", "5"),
	}
}

func (f *ImportFixture) run(pvcName string) {
	f.runController(pvcName, true, false, false)
}

func (f *ImportFixture) runWithExpectation(pvcName string) {
	f.runController(pvcName, true, false, true)
}

func (f *ImportFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true, false)
}

func (f *ImportFixture) runController(pvcName string,
	startInformers bool,
	expectError bool,
	withCreateExpectation bool) {
	c := f.newImportController()
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
	expPvc.ObjectMeta.Labels = map[string]string{CDILabelKey: CDILabelValue}
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
	expPvc.ObjectMeta.Labels = map[string]string{CDILabelKey: CDILabelValue}
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
	pvc.ObjectMeta.Labels = map[string]string{CDILabelKey: CDILabelValue}

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

	controller := f.newImportController()

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
