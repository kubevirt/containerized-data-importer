/*
Copyright 2018 The CDI Authors.

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
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	cdifake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

const (
	uploadRequestAnnotation = "cdi.kubevirt.io/storage.upload.target"
	podPhaseAnnotation      = "cdi.kubevirt.io/storage.pod.phase"
	podReadyAnnotation      = "cdi.kubevirt.io/storage.pod.ready"
	cloneRequestAnnotation  = "k8s.io/CloneRequest"
)

type uploadFixture struct {
	t *testing.T

	kubeclient *k8sfake.Clientset
	cdiclient  *cdifake.Clientset

	// Objects to put in the store.
	pvcLister     []*corev1.PersistentVolumeClaim
	podLister     []*corev1.Pod
	serviceLister []*corev1.Service

	// Actions expected to happen on the client.
	kubeactions []core.Action

	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	cdiobjects  []runtime.Object

	expectedSecretNamespace                   string
	expectedSecretGets, expectedSecretCreates int
}

func newUploadFixture(t *testing.T) *uploadFixture {
	f := &uploadFixture{}
	f.t = t
	f.kubeobjects = []runtime.Object{}
	f.cdiobjects = []runtime.Object{}
	return f
}

func (f *uploadFixture) newController() (*UploadController, kubeinformers.SharedInformerFactory) {
	serverCAKeypair, err := triple.NewCA("serverca")
	if err != nil {
		f.t.Errorf("Error creating CA cert")
	}

	clientCAKeypair, err := triple.NewCA("clientca")
	if err != nil {
		f.t.Errorf("Error creating CA cert")
	}

	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	f.cdiclient = cdifake.NewSimpleClientset(f.cdiobjects...)
	i := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	pvcInformer := i.Core().V1().PersistentVolumeClaims()
	podInformer := i.Core().V1().Pods()
	serviceInformer := i.Core().V1().Services()

	c := NewUploadController(f.kubeclient,
		f.cdiclient,
		pvcInformer,
		podInformer,
		serviceInformer,
		"test/myimage",
		"cdi-uploadproxy",
		"Always",
		"5")

	c.serverCAKeyPair = serverCAKeypair
	c.clientCAKeyPair = clientCAKeypair

	c.pvcsSynced = alwaysReady
	c.podsSynced = alwaysReady
	c.servicesSynced = alwaysReady

	for _, pvc := range f.pvcLister {
		c.pvcInformer.GetIndexer().Add(pvc)
	}

	for _, pod := range f.podLister {
		c.podInformer.GetIndexer().Add(pod)
	}

	for _, service := range f.serviceLister {
		c.serviceInformer.GetIndexer().Add(service)
	}

	return c, i
}

func (f *uploadFixture) run(pvcName string) {
	f.runController(pvcName, true, false)
}

func (f *uploadFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true)
}

func (f *uploadFixture) runController(pvcName string, startInformers bool, expectError bool) {
	c, i := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		i.Start(stopCh)
	}

	err := c.syncHandler(pvcName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing foo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing foo, got nil")
	}

	actualSecretGets := findSecretGetActions(f.expectedSecretNamespace, f.kubeclient.Actions())
	if len(actualSecretGets) != f.expectedSecretGets {
		f.t.Errorf("Unexpected secret get counts %d %d", f.expectedSecretGets, len(actualSecretGets))
	}

	actualSecretCreates := findSecretCreateActions(f.expectedSecretNamespace, f.kubeclient.Actions())
	if len(actualSecretCreates) != f.expectedSecretCreates {
		f.t.Errorf("Unexpected secret create counts %d %d", f.expectedSecretCreates, len(actualSecretCreates))
	}

	k8sActions := filterSecretGetAndCreateActions(f.expectedSecretNamespace,
		filterUploadInformerActions(f.kubeclient.Actions()))
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

func (f *uploadFixture) expectCreatePodAction(p *corev1.Pod) {
	f.kubeactions = append(f.kubeactions,
		core.NewCreateAction(schema.GroupVersionResource{Resource: "pods", Version: "v1"}, p.Namespace, p))
}

func (f *uploadFixture) expectDeletePodAction(p *corev1.Pod) {
	f.kubeactions = append(f.kubeactions,
		core.NewDeleteAction(schema.GroupVersionResource{Resource: "pods", Version: "v1"}, p.Namespace, p.Name))
}

func (f *uploadFixture) expectCreateServiceAction(s *corev1.Service) {
	f.kubeactions = append(f.kubeactions,
		core.NewCreateAction(schema.GroupVersionResource{Resource: "services", Version: "v1"}, s.Namespace, s))
}

func (f *uploadFixture) expectDeleteServiceAction(s *corev1.Service) {
	f.kubeactions = append(f.kubeactions,
		core.NewDeleteAction(schema.GroupVersionResource{Resource: "services", Version: "v1"}, s.Namespace, s.Name))
}

func (f *uploadFixture) expectCreatePvcAction(pvc *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions,
		core.NewCreateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, pvc.Namespace, pvc))
}

func (f *uploadFixture) expectUpdatePvcAction(pvc *corev1.PersistentVolumeClaim) {
	f.kubeactions = append(f.kubeactions,
		core.NewUpdateAction(schema.GroupVersionResource{Resource: "persistentvolumeclaims", Version: "v1"}, pvc.Namespace, pvc))
}

// really should be expect keystore actions but they are the only secrets created for now
func (f *uploadFixture) expectSecretActions(namespace string, gets, creates int) {
	f.expectedSecretNamespace = namespace
	f.expectedSecretGets = gets
	f.expectedSecretCreates = creates
}

func filterUploadInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "persistentvolumeclaims") ||
				action.Matches("watch", "persistentvolumeclaims") ||
				action.Matches("list", "pods") ||
				action.Matches("watch", "pods") ||
				action.Matches("list", "services") ||
				action.Matches("watch", "services")) {
			continue
		}
		ret = append(ret, action)
	}
	return ret
}

func findSecretCreateActions(filterNamespace string, actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if action.GetNamespace() == filterNamespace && action.Matches("create", "secrets") {
			ret = append(ret, action)
		}
	}
	return ret
}

func findSecretGetActions(filterNamespace string, actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if action.GetNamespace() == filterNamespace && action.Matches("get", "secrets") {
			ret = append(ret, action)
		}
	}
	return ret
}

func filterSecretGetAndCreateActions(filterNamespace string, actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if action.GetNamespace() == filterNamespace && (action.Matches("get", "secrets") || action.Matches("create", "secrets")) {
			continue
		}
		ret = append(ret, action)
	}
	return ret
}

func TestCreatesUploadPodAndService(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{uploadRequestAnnotation: ""}, nil)
	pod := createUploadPod(pvc)
	pod.Namespace = ""
	scratchPvc := createScratchPvc(pvc, pod, storageClassName)
	f.expectCreatePodAction(pod)

	f.podLister = append(f.podLister, pod)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pod, pvc)
	cdiConfig := createCDIConfig(common.ConfigName)
	f.cdiobjects = append(f.cdiobjects, cdiConfig)

	f.expectCreatePvcAction(scratchPvc)
	f.expectSecretActions("default", 1, 1)

	service := createUploadService(pvc)
	service.Namespace = ""
	f.expectCreateServiceAction(service)

	pvcUpdate := pvc.DeepCopy()
	pvcUpdate.Annotations[podPhaseAnnotation] = string(pod.Status.Phase)
	pvcUpdate.Annotations[podReadyAnnotation] = "false"
	f.expectUpdatePvcAction(pvcUpdate)

	f.run(getPvcKey(pvc, t))
}

func TestCreatesCloneTargetPodAndService(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{cloneRequestAnnotation: "default/sourcePvc"}, nil)
	source := createPvcInStorageClass("sourcePvc", "default", &storageClassName, nil, nil)
	clientName := fmt.Sprintf("%s/%s-%s/%s", source.Namespace, source.Name, pvc.Namespace, pvc.Name)
	pod := createUploadClonePod(pvc, clientName)
	pod.Namespace = ""
	f.expectCreatePodAction(pod)

	f.podLister = append(f.podLister, pod)
	f.pvcLister = append(f.pvcLister, pvc, source)
	f.kubeobjects = append(f.kubeobjects, pod, pvc, source)
	cdiConfig := createCDIConfig(common.ConfigName)
	f.cdiobjects = append(f.cdiobjects, cdiConfig)

	f.expectSecretActions("default", 1, 1)

	service := createUploadService(pvc)
	service.Namespace = ""
	f.expectCreateServiceAction(service)

	pvcUpdate := pvc.DeepCopy()
	pvcUpdate.Annotations[AnnUploadClientName] = clientName
	pvcUpdate.Annotations[podPhaseAnnotation] = string(pod.Status.Phase)
	pvcUpdate.Annotations[podReadyAnnotation] = "false"
	f.expectUpdatePvcAction(pvcUpdate)

	f.run(getPvcKey(pvc, t))
}

func TestCloneFailNoSource(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{cloneRequestAnnotation: "default/sourcePvc"}, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.runExpectError(getPvcKey(pvc, t))
}

func TestCloneAndUploadAnnotation(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	annotations := map[string]string{
		uploadRequestAnnotation: "",
		cloneRequestAnnotation:  "default/sourcePvc",
	}
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, annotations, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.run(getPvcKey(pvc, t))
}

func TestCloneValidationFails(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{cloneRequestAnnotation: "default/sourcePvc"}, nil)
	source := createPvcInStorageClass("sourcePvc", "default", &storageClassName, nil, nil)
	vm := corev1.PersistentVolumeBlock
	source.Spec.VolumeMode = &vm

	f.pvcLister = append(f.pvcLister, pvc, source)
	f.kubeobjects = append(f.kubeobjects, pvc, source)

	f.run(getPvcKey(pvc, t))
}

func TestUpdatePodPhase(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{uploadRequestAnnotation: ""}, nil)
	pod := createUploadPod(pvc)
	service := createUploadService(pvc)
	scratchPvc := createScratchPvc(pvc, pod, storageClassName)

	pod.Status.Phase = corev1.PodRunning
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Ready: true,
		},
	}

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.serviceLister = append(f.serviceLister, service)
	f.kubeobjects = append(f.kubeobjects, service)
	cdiConfig := createCDIConfig(common.ConfigName)
	f.cdiobjects = append(f.cdiobjects, cdiConfig)

	updatedPVC := pvc.DeepCopy()
	updatedPVC.Annotations[podPhaseAnnotation] = string(corev1.PodRunning)
	updatedPVC.Annotations[podReadyAnnotation] = "true"

	f.expectCreatePvcAction(scratchPvc)
	f.expectUpdatePvcAction(updatedPVC)

	f.run(getPvcKey(pvc, t))
}

func TestUploadComplete(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{uploadRequestAnnotation: "", podPhaseAnnotation: "Running"}, nil)
	pod := createUploadPod(pvc)
	scratchPvc := createScratchPvc(pvc, pod, storageClassName)
	service := createUploadService(pvc)

	pod.Status.Phase = corev1.PodSucceeded
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Ready: false,
		},
	}

	f.pvcLister = append(f.pvcLister, pvc)
	f.pvcLister = append(f.pvcLister, scratchPvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, scratchPvc)

	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.serviceLister = append(f.serviceLister, service)
	f.kubeobjects = append(f.kubeobjects, service)

	updatedPVC := pvc.DeepCopy()
	updatedPVC.Annotations[podPhaseAnnotation] = string(corev1.PodSucceeded)
	updatedPVC.Annotations[podReadyAnnotation] = "false"

	f.expectUpdatePvcAction(updatedPVC)
	f.run(getPvcKey(pvc, t))

	f.pvcLister = nil
	f.kubeobjects = nil
	f.kubeactions = nil
	f.pvcLister = append(f.pvcLister, scratchPvc, updatedPVC)
	f.kubeobjects = append(f.kubeobjects, scratchPvc, updatedPVC, pod, service)
	f.expectDeleteServiceAction(service)
	f.expectDeletePodAction(pod)
	f.run(getPvcKey(pvc, t))
}

func TestSucceededDoNothing(t *testing.T) {
	f := newUploadFixture(t)
	storageClassName := "test"
	pvc := createPvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{uploadRequestAnnotation: "", podPhaseAnnotation: "Succeeded"}, nil)
	pod := createUploadPod(pvc)
	scratchPvc := createScratchPvc(pvc, pod, storageClassName)

	f.pvcLister = append(f.pvcLister, pvc)
	f.pvcLister = append(f.pvcLister, scratchPvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.kubeobjects = append(f.kubeobjects, scratchPvc)

	f.run(getPvcKey(pvc, t))
}

func TestPVCNolongerExists(t *testing.T) {
	f := newUploadFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{uploadRequestAnnotation: "", podPhaseAnnotation: "Succeeded"}, nil)

	f.run(getPvcKey(pvc, t))
}

func TestDeletesUploadPodAndServiceWhenAnnotationRemoved(t *testing.T) {
	f := newUploadFixture(t)
	pvc := createPvc("testPvc1", "default", nil, nil)
	pod := createUploadPod(pvc)
	service := createUploadService(pvc)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.serviceLister = append(f.serviceLister, service)
	f.kubeobjects = append(f.kubeobjects, service)

	f.expectDeleteServiceAction(service)
	f.expectDeletePodAction(pod)

	f.run(getPvcKey(pvc, t))
}

func TestDeletesUploadPodAndServiceWhenPVCDeleted(t *testing.T) {
	f := newUploadFixture(t)
	pvc := createPvc("testPvc1", "default", map[string]string{uploadRequestAnnotation: "", podPhaseAnnotation: "Running"}, nil)
	pvc.SetDeletionTimestamp(&metav1.Time{Time: time.Now().UTC().Add(-5 * time.Second)})
	pod := createUploadPod(pvc)
	service := createUploadService(pvc)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pod)

	f.serviceLister = append(f.serviceLister, service)
	f.kubeobjects = append(f.kubeobjects, service)

	f.expectDeleteServiceAction(service)
	f.expectDeletePodAction(pod)

	f.run(getPvcKey(pvc, t))
}

func TestShouldCreateCerts(t *testing.T) {
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	controller := &UploadController{client: client, uploadProxyServiceName: "cdi-uploadproxy"}

	err := controller.initCerts()
	if err != nil {
		t.Errorf("init certs failed %+v", err)
	}

	filteredActions := findSecretCreateActions(util.GetNamespace(), client.Actions())
	if len(filteredActions) != 5 {
		t.Errorf("Expected 5 certs, got %d", len(filteredActions))
	}
}

func TestUploadPossible(t *testing.T) {
	type args struct {
		annotations map[string]string
		expectErr   bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			"PVC is ready for upload",
			args{
				map[string]string{"cdi.kubevirt.io/storage.upload.target": "",
					"cdi.kubevirt.io/storage.pod.phase": "Running",
				},
				false,
			},
		},
		{
			"PVC missing target annotation",
			args{
				map[string]string{},
				true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := createPvc("testPvc1", "default", tt.args.annotations, nil)
			err := UploadPossibleForPVC(pvc)
			if (err == nil && tt.args.expectErr) || (err != nil && !tt.args.expectErr) {
				t.Errorf("Unexpected result expectErr=%t, err=%+v", tt.args.expectErr, err)
			}
		})
	}
}

func TestResourceName(t *testing.T) {
	resourceName := GetUploadResourceName("testPvc1")
	if resourceName != "cdi-upload-testPvc1" {
		t.Errorf("Unexpected resource name %s", resourceName)
	}
}
