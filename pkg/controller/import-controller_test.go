package controller

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	k8stesting "k8s.io/client-go/tools/cache/testing"
	"k8s.io/client-go/util/workqueue"
)

func TestNewImportController(t *testing.T) {
	type args struct {
		client        kubernetes.Interface
		pvcInformer   cache.SharedIndexInformer
		podInformer   cache.SharedIndexInformer
		importerImage string
		pullPolicy    string
		verbose       string
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
		want *ImportController
	}{
		{
			name: "expect controller to be created with expected informers",
			args: args{myclient, pvcInformer, podInformer, "test/image", "Always", "-v=5"},
			want: &ImportController{myclient, nil, nil, pvcInformer, podInformer, "test/image", "Always", "-v=5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewImportController(tt.args.client, tt.args.pvcInformer, tt.args.podInformer, tt.args.importerImage, tt.args.pullPolicy, tt.args.verbose)
			if got == nil {
				t.Errorf("NewImportController() not created - %v", got)
			}
			if !reflect.DeepEqual(got.podInformer, tt.want.podInformer) {
				t.Errorf("NewImportController() podInformer = %v, want %v", got.podInformer, tt.want.podInformer)
			}
			if !reflect.DeepEqual(got.pvcInformer, tt.want.pvcInformer) {
				t.Errorf("NewImportController() pvcInformer = %v, want %v", got.pvcInformer, tt.want.pvcInformer)
			}
			// queues are generated from the controller
			if got.pvcQueue == nil {
				t.Errorf("NewImportController() pvcQueue was not generated properly = %v", got.pvcQueue)
			}
			if got.podQueue == nil {
				t.Errorf("NewImportController() podQueue was not generated properly = %v", got.podQueue)
			}
		})
	}
}

func TestController_Run(t *testing.T) {
	type fields struct {
		clientset     kubernetes.Interface
		pvcQueue      workqueue.RateLimitingInterface
		podQueue      workqueue.RateLimitingInterface
		pvcInformer   cache.SharedIndexInformer
		podInformer   cache.SharedIndexInformer
		importerImage string
		pullPolicy    string
		verbose       string
	}
	type args struct {
		threadiness int
		stopCh      <-chan struct{}
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ImportController{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if err := c.Run(tt.args.threadiness, tt.args.stopCh); (err != nil) != tt.wantErr {
				t.Errorf("Controller.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_ProcessNextPodItem(t *testing.T) {
	//create pod and pvc
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName)

	//create and run the informers
	c, _, _, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.ProcessNextPodItem() failed to initialize fake controller error = %v", err)
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
				t.Errorf("Controller.ProcessNextPodItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_processPodItem(t *testing.T) {
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
	stagePvcs[0] = createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	stagePvcs[1] = createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	stagePvcs[2] = createPvc("testPvc3", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	// create pod specs with pvc reference
	stagePods := make([]*v1.Pod, stageCount)
	stagePods[0] = createPod(stagePvcs[0], DataVolName)
	stagePods[1] = createPod(stagePvcs[1], "myvolume")
	stagePods[2] = createPod(stagePvcs[2], DataVolName)

	//create and run the informers passing back v1 processed pods
	c, _, pods, err := createImportControllerMultiObject(stagePvcs, stagePods, stageNs)
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
			name:    "expect error when processing pod with no cdi-data-volume",
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

func TestController_ProcessNextPvcItem(t *testing.T) {
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
	stagePods[0] = createPod(stagePvcs[0], DataVolName)
	stagePods[1] = createPod(stagePvcs[1], DataVolName)
	stagePods[2] = createPod(stagePvcs[2], DataVolName)
	stagePods[3] = createPod(stagePvcs[3], DataVolName)

	//create and run queues and informers
	c, _, _, err := createImportControllerMultiObject(stagePvcs, stagePods, stageNs)
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

func TestController_processPvcItem(t *testing.T) {
	//create pod and pvc
	const stageCount = 2

	//Generate Staging Specs
	// create pvc specs
	stagePvcs := make([]*v1.PersistentVolumeClaim, stageCount)
	stagePvcs[0] = createPvc("testPvc1", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	stagePvcs[1] = createPvc("testPvc2", "default", map[string]string{AnnEndpoint: "http://test", AnnImportPod: "cdi-importer-pod"}, nil)

	//create and run queues and informers
	c, _, _, err := createImportControllerMultiObject(stagePvcs, nil, nil)
	if err != nil {
		t.Errorf("Controller.ProcessPvcItem() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		pvc *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name     string
		args     args
		podName  string
		wantErr  bool
		wantFind bool
	}{
		{
			name:     "process pvc and expect importer pod to be created",
			args:     args{stagePvcs[0]},
			podName:  fmt.Sprintf("importer-%s-", stagePvcs[0].Name),
			wantErr:  false,
			wantFind: true,
		},
		{
			name:     "process pvc and do not create importer pod",
			args:     args{stagePvcs[1]},
			podName:  fmt.Sprintf("importer-%s-", stagePvcs[1].Name),
			wantErr:  false,
			wantFind: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.processPvcItem(tt.args.pvc); (err != nil) != tt.wantErr {
				t.Errorf("Controller.processPvcItem() error = %v, wantErr %v", err, tt.wantErr)
			}
			//find importer pod
			found := false
			got, err := c.clientset.CoreV1().Pods(tt.args.pvc.Namespace).List(metav1.ListOptions{})
			if (len(got.Items) == 0 || err != nil) && !tt.wantErr {
				t.Errorf("Controller.processPvcItem() could not find any pods or got error = %v", err)
			} else {
				for _, p := range got.Items {
					if p.GenerateName == tt.podName {
						found = true
					}
				}
				if found && !tt.wantFind {
					t.Errorf("Controller.processPvcItem() found pod %v but expected not to find pod", tt.podName)
				}
				if !found && tt.wantFind {
					t.Errorf("Controller.processPvcItem() did not find pod %v and expected to find pod", tt.podName)
				}
			}
		})
	}
}

func TestController_forgetKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName)

	//run the informers
	c, _, _, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
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
