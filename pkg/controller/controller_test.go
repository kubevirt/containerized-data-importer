package controller

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestNewController(t *testing.T) {
	type args struct {
		client        kubernetes.Interface
		pvcInformer   cache.SharedIndexInformer
		podInformer   cache.SharedIndexInformer
		importerImage string
		pullPolicy    string
		verbose       string
	}
	tests := []struct {
		name string
		args args
		want *Controller
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewController(tt.args.client, tt.args.pvcInformer, tt.args.podInformer, tt.args.importerImage, tt.args.pullPolicy, tt.args.verbose); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewController() = %v, want %v", got, tt.want)
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
			c := &Controller{
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

func TestController_runPodWorkers(t *testing.T) {
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
	tests := []struct {
		name   string
		fields fields
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			c.runPodWorkers()
		})
	}
}

func TestController_runPVCWorkers(t *testing.T) {
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
	tests := []struct {
		name   string
		fields fields
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			c.runPVCWorkers()
		})
	}
}

func TestController_ProcessNextPodItem(t *testing.T) {
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
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if got := c.ProcessNextPodItem(); got != tt.want {
				t.Errorf("Controller.ProcessNextPodItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_processPodItem(t *testing.T) {
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
		pod *v1.Pod
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
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if err := c.processPodItem(tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("Controller.processPodItem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_ProcessNextPvcItem(t *testing.T) {
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
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if got := c.ProcessNextPvcItem(); got != tt.want {
				t.Errorf("Controller.ProcessNextPvcItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_processPvcItem(t *testing.T) {
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
		pvc *v1.PersistentVolumeClaim
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
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if err := c.processPvcItem(tt.args.pvc); (err != nil) != tt.wantErr {
				t.Errorf("Controller.processPvcItem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_forgetKey(t *testing.T) {
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
		key interface{}
		msg string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				clientset:     tt.fields.clientset,
				pvcQueue:      tt.fields.pvcQueue,
				podQueue:      tt.fields.podQueue,
				pvcInformer:   tt.fields.pvcInformer,
				podInformer:   tt.fields.podInformer,
				importerImage: tt.fields.importerImage,
				pullPolicy:    tt.fields.pullPolicy,
				verbose:       tt.fields.verbose,
			}
			if got := c.forgetKey(tt.args.key, tt.args.msg); got != tt.want {
				t.Errorf("Controller.forgetKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
