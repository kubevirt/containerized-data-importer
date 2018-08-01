/*
Copyright 2017 The Kubernetes Authors.

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
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/cdicontroller/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
	informers "kubevirt.io/containerized-data-importer/pkg/client/informers/externalversions"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset

	// Objects to put in the store.
	dataVolumeLister []*cdiv1.DataVolume
	pvcLister        []*corev1.PersistentVolumeClaim

	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action

	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

func newDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://example.com/data",
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
		},
	}
}

func (f *fixture) newController() (*DataVolumeController, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	for _, f := range f.dataVolumeLister {
		i.Cdi().V1alpha1().DataVolumes().Informer().GetIndexer().Add(f)
	}

	for _, d := range f.pvcLister {
		k8sI.Core().V1().PersistentVolumeClaims().Informer().GetIndexer().Add(d)
	}

	c := NewDataVolumeController(f.kubeclient,
		f.client,
		k8sI.Core().V1().PersistentVolumeClaims(),
		i.Cdi().V1alpha1().DataVolumes())

	c.dataVolumesSynced = alwaysReady
	c.pvcsSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	return c, i, k8sI
}

func (f *fixture) run(dataVolumeName string) {
	f.runController(dataVolumeName, true, false)
}

func (f *fixture) runExpectError(dataVolumeName string) {
	f.runController(dataVolumeName, true, true)
}

func (f *fixture) runController(dataVolumeName string, startInformers bool, expectError bool) {
	c, i, k8sI := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
		k8sI.Start(stopCh)
	}

	err := c.syncHandler(dataVolumeName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing dataVolume: %s: %v", dataVolumeName, err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing dataVolume, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
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

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "dataVolumes") ||
				action.Matches("watch", "dataVolumes") ||
				action.Matches("list", "persistentvolumeclaims") ||
				action.Matches("watch", "persistentvolumeclaims")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectCreatePersistentVolumeClaimAction(d *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, d.Namespace, d))
}

func (f *fixture) expectUpdatePersistentVolumeClaimAction(d *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, d.Namespace, d))
}

func (f *fixture) expectUpdateDataVolumeStatusAction(dataVolume *cdiv1.DataVolume) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Resource: "dataVolumes", Version: "v1alpha1"}, dataVolume.Namespace, dataVolume)
	// TODO: Until #38113 is merged, we can't use Subresource
	//action.Subresource = "status"
	f.actions = append(f.actions, action)
}

func getKey(dataVolume *cdiv1.DataVolume, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(dataVolume)
	if err != nil {
		t.Errorf("Unexpected error getting key for dataVolume %v: %v", dataVolume.Name, err)
		return ""
	}
	return key
}

func TestCreatesPersistentVolumeClaim(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)

	expPersistentVolumeClaim, _ := newPersistentVolumeClaim(dataVolume)

	f.expectCreatePersistentVolumeClaimAction(expPersistentVolumeClaim)

	f.run(getKey(dataVolume, t))
}

func TestDoNothing(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.PVCBound
	pvc.Status.Phase = corev1.ClaimBound

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.run(getKey(dataVolume, t))
}

func TestNotControlledByUs(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	d, _ := newPersistentVolumeClaim(dataVolume)

	d.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, d)
	f.kubeobjects = append(f.kubeobjects, d)

	f.runExpectError(getKey(dataVolume, t))
}

func TestDetectPVCBound(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimBound

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.PVCBound
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}

func TestImportScheduled(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimBound
	pvc.Annotations[AnnImportPod] = "somepod"

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.ImportScheduled
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}

func TestImportInProgress(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimBound
	pvc.Annotations[AnnImportPod] = "somepod"
	pvc.Annotations[AnnPodPhase] = "Running"

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.ImportInProgress
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}

func TestImportSucceeded(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimBound
	pvc.Annotations[AnnImportPod] = "somepod"
	pvc.Annotations[AnnPodPhase] = "Succeeded"

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.Succeeded
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}

func TestImportPodFailed(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimBound
	pvc.Annotations[AnnImportPod] = "somepod"
	pvc.Annotations[AnnPodPhase] = "Failed"

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.Failed
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}

func TestImportClaimLost(t *testing.T) {
	f := newFixture(t)
	dataVolume := newDataVolume("test")
	pvc, _ := newPersistentVolumeClaim(dataVolume)

	dataVolume.Status.Phase = cdiv1.Pending
	pvc.Status.Phase = corev1.ClaimLost

	f.dataVolumeLister = append(f.dataVolumeLister, dataVolume)
	f.objects = append(f.objects, dataVolume)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	result := dataVolume.DeepCopy()
	result.Status.Phase = cdiv1.Failed
	f.expectUpdateDataVolumeStatusAction(result)
	f.run(getKey(dataVolume, t))
}
