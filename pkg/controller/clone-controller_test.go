package controller

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	k8stesting "k8s.io/client-go/tools/cache/testing"
	. "kubevirt.io/containerized-data-importer/pkg/common"
)

const CLONER_DEFAULT_IMAGE = "kubevirt/cdi-cloner:latest"

func TestNewCloneController(t *testing.T) {
	type args struct {
		client      kubernetes.Interface
		pvcInformer cache.SharedIndexInformer
		podInformer cache.SharedIndexInformer
		cloneImage  string
		pullPolicy  string
		verbose     string
	}
	// Set up environment
	myclient := k8sfake.NewSimpleClientset()
	pvcSource := k8stesting.NewFakePVCControllerSource()
	podSource := k8stesting.NewFakeControllerSource()

	// create informers
	pvcInformer := cache.NewSharedIndexInformer(pvcSource, nil, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podInformer := cache.NewSharedIndexInformer(podSource, nil, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	tests := []struct {
		name string
		args args
		want *CloneController
	}{
		{
			name: "expect controller to be created with expected informers",
			args: args{myclient, pvcInformer, podInformer, CLONER_DEFAULT_IMAGE, "Always", "-v=5"},
			want: &CloneController{myclient, nil, nil, pvcInformer, podInformer, "test/image", "Always", "-v=5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCloneController(tt.args.client, tt.args.pvcInformer, tt.args.podInformer, tt.args.cloneImage, tt.args.pullPolicy, tt.args.verbose)
			if got == nil {
				t.Errorf("NewCloneController() not created - %v", got)
			}
			if !reflect.DeepEqual(got.podInformer, tt.want.podInformer) {
				t.Errorf("NewCloneController() podInformer = %v, want %v", got.podInformer, tt.want.podInformer)
			}
			if !reflect.DeepEqual(got.pvcInformer, tt.want.pvcInformer) {
				t.Errorf("NewCloneController() pvcInformer = %v, want %v", got.pvcInformer, tt.want.pvcInformer)
			}
			// queues are generated from the controller
			if got.pvcQueue == nil {
				t.Errorf("NewCloneController() pvcQueue was not generated properly = %v", got.pvcQueue)
			}
			if got.podQueue == nil {
				t.Errorf("NewCloneController() podQueue was not generated properly = %v", got.podQueue)
			}
		})
	}
}

func TestCloneController_ProcessNextPodItem(t *testing.T) {
	//create pod and pvc
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPodWithName(pvcWithEndPointAnno, DataVolName)

	//create and run the informers
	c, _, _, err := createCloneController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("CloneController.ProcessNextPodItem() failed to initialize fake controller error = %v", err)
		return
	}
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "successfully process pod",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.ProcessNextPodItem(); got != tt.want {
				t.Errorf("CloneController.ProcessNextPodItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneController_processPodItem(t *testing.T) {
	//create pod and pvc
	const stageCount = 3

	//Generate Staging Specs
	// dictates what NS the spec is actually created in
	stageNs := make([]string, stageCount)
	stageNs[0] = "default"
	stageNs[1] = "default"
	stageNs[2] = "test"

	// create pvc specs
	stagePvcs := make([]*v1.PersistentVolumeClaim, stageCount)
	stagePvcs[0] = createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test", AnnCloneRequest: "default/golden-pvc"}, nil)
	stagePvcs[1] = createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	stagePvcs[2] = createPvc("testPvc3", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	// create pod specs with pvc reference
	stagePods := make([]*v1.Pod, stageCount)
	stagePods[0] = createPodWithName(stagePvcs[0], ImagePathName)
	stagePods[1] = createPodWithName(stagePvcs[1], "myvolume")
	stagePods[2] = createPodWithName(stagePvcs[2], ImagePathName)

	//create and run the informers passing back v1 processed pods
	c, _, pods, err := createCloneControllerMultiObject(stagePvcs, stagePods, stageNs)
	if err != nil {
		t.Errorf("Controller.ProcessPodItem() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		pod *v1.Pod
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "successfully process pod",
			args:    args{pods[0]},
			wantErr: false,
		},
		{
			name:    "expect error when processing pod with no image-path volume",
			args:    args{pods[1]},
			wantErr: true,
		},
		{
			name:    "expect error when proessing pvc with different NS than expected pod",
			args:    args{pods[2]},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.processPodItem(tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("Controller.processPodItem() error = %v, wantErr %v, Pod Name = %v, Pod ClaimRef = %v", err, tt.wantErr, tt.args.pod.Name, tt.args.pod.Spec.Volumes)
			}
		})
	}
}

func TestCloneController_ProcessNextPvcItem(t *testing.T) {
	//create pod and pvc
	const stageCount = 4

	//Generate Staging Specs
	// dictates what NS the spec is actually created in
	stageNs := make([]string, stageCount)
	stageNs[0] = "default"
	stageNs[1] = "default"
	stageNs[2] = "default"
	stageNs[3] = "default"

	// create pvc specs
	stagePvcs := make([]*v1.PersistentVolumeClaim, stageCount)
	stagePvcs[0] = createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	stagePvcs[1] = createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test", AnnImportPod: "cdi-importer-pod"}, nil)
	stagePvcs[2] = createPvc("testPvc3", "default", nil, nil)
	stagePvcs[3] = createPvc("testPvc4", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	// create pod specs with pvc reference
	stagePods := make([]*v1.Pod, stageCount)
	stagePods[0] = createPodWithName(stagePvcs[0], DataVolName)
	stagePods[1] = createPodWithName(stagePvcs[1], DataVolName)
	stagePods[2] = createPodWithName(stagePvcs[2], DataVolName)
	stagePods[3] = createPodWithName(stagePvcs[3], DataVolName)

	//create and run queues and informers
	c, _, _, err := createCloneControllerMultiObject(stagePvcs, stagePods, stageNs)
	if err != nil {
		t.Errorf("Controller.ProcessNextPvcItem() failed to initialize fake controller error = %v", err)
		return
	}

	tests := []struct {
		name string
		want bool
	}{
		{
			name: "expect successful processing of queue",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.ProcessNextPvcItem(); got != tt.want {
				t.Errorf("Controller.ProcessNextPvcItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneController_processPvcItem(t *testing.T) {
	const (
		stageCount = 4
		srcNs      = "test"
		srcPvc     = "testPvcSource"
	)
	srcCloneRequestValue := srcNs + "/" + srcPvc

	//Generate Staging Specs
	// create pvc specs
	stagePvcs := make([]*v1.PersistentVolumeClaim, stageCount)
	stagePvcs[0] = createPvc(srcPvc, srcNs, map[string]string{}, nil)
	stagePvcs[1] = createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: srcCloneRequestValue}, nil)
	stagePvcs[2] = createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test", AnnCloningPods: "cdi-cloner-pod", AnnCloneRequest: "default/testPvcSource"}, nil)
	stagePvcs[3] = createPvc("testPvc3", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	//create and run queues and informers
	c, _, _, err := createCloneControllerMultiObject(stagePvcs, nil, nil)
	if err != nil {
		t.Errorf("Controller.ProcessPvcItem() failed to initialize fake controller error = %v", err)
		return
	}
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name          string
		args          args
		podSourceName string
		podTargetName string
		wantErr       bool
		wantFind      bool
	}{
		{
			name:          "process pvc and expect clone pods to be created",
			args:          args{stagePvcs[1]},
			podSourceName: fmt.Sprintf("%s-", CLONER_SOURCE_PODNAME),
			podTargetName: fmt.Sprintf("%s-", CLONER_TARGET_PODNAME),
			wantErr:       false,
			wantFind:      true,
		},
		{
			name:          "process pvc and do not create cloner pods because AnnCloningPods annotation exists",
			args:          args{stagePvcs[2]},
			podSourceName: fmt.Sprintf("%s-", CLONER_SOURCE_PODNAME),
			podTargetName: fmt.Sprintf("%s-", CLONER_TARGET_PODNAME),
			wantErr:       false,
			wantFind:      false,
		},
		{
			name:          "should not process pvc as it does not contain correct cloner request annotation",
			args:          args{stagePvcs[3]},
			podSourceName: fmt.Sprintf("%s-", CLONER_SOURCE_PODNAME),
			podTargetName: fmt.Sprintf("%s-", CLONER_TARGET_PODNAME),
			wantErr:       true,
			wantFind:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.processPvcItem(tt.args.pvc)
			if err == nil && tt.wantErr {
				t.Errorf("Controller.processPvcItem() expected error but did not get one %v", tt.wantErr)
			} else if err != nil && tt.wantErr {
				// good case
			} else {
				// retrieve the pods from respective namespaces
				gotSource, err := c.clientset.CoreV1().Pods(srcNs).List(metav1.ListOptions{})
				if err != nil {
					// could not get pods
					t.Errorf("Controller.processPvcItem() could not retrieve pods from API")
					return
				}
				gotTarget, err := c.clientset.CoreV1().Pods(tt.args.pvc.Namespace).List(metav1.ListOptions{})
				if err != nil {
					// could not get pods
					t.Errorf("Controller.processPvcItem() could not retrieve pods from API")
					return
				}
				// expect 2 pods to be created - 1 in test namespace and 1 in default namespace
				if len(gotSource.Items) != 1 && len(gotTarget.Items) != 1 && tt.wantFind {
					t.Errorf("Controller.processPvcItem() did not find both the source and target pods, pod counts = %v - %v in namespaces %s - %s", len(gotSource.Items), len(gotTarget.Items), srcNs, tt.args.pvc.Namespace)
					return
				}
				if gotSource.Items[0].GenerateName != tt.podSourceName && tt.wantFind {
					t.Errorf("Controller.processPvcItem() could not retrieve Source Pod %v", gotSource.Items[0].GenerateName)
				}
				if gotTarget.Items[0].GenerateName != tt.podTargetName && tt.wantFind {
					t.Errorf("Controller.processPvcItem() could not retrieve Target Pod %v", gotTarget.Items[0].GenerateName)
				}
			}
		})
	}
}

func TestCloneController_forgetKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPodWithName(pvcWithEndPointAnno, DataVolName)

	//run the informers
	c, _, _, err := createCloneController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.forgetKey() failed to initialize fake controller error = %v", err)
		return
	}
	key, shutdown := c.pvcQueue.Get()
	if shutdown {
		t.Errorf("Controller.forgetKey() failed to retrieve key from pvcQueue")
		return
	}
	defer c.pvcQueue.Done(key)

	type args struct {
		key interface{}
		msg string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "successfully forget key",
			args: args{key, "test of forgetKey func"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.forgetKey(tt.args.key, tt.args.msg); got != tt.want {
				t.Errorf("Controller.forgetKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
