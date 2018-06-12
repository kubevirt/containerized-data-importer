package controller

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestController_pvcFromKey(t *testing.T) {
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
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1.PersistentVolumeClaim
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
			got, err := c.pvcFromKey(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.pvcFromKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.pvcFromKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_podFromKey(t *testing.T) {
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
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1.Pod
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
			got, err := c.podFromKey(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.podFromKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.podFromKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_objFromKey(t *testing.T) {
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
		informer cache.SharedIndexInformer
		key      interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
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
			got, err := c.objFromKey(tt.args.informer, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.objFromKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.objFromKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkPVC(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    *v1.PersistentVolumeClaim
		get    bool
	}
	tests := []struct {
		name       string
		args       args
		wantOk     bool
		wantNewPvc *v1.PersistentVolumeClaim
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk, gotNewPvc, err := checkPVC(tt.args.client, tt.args.pvc, tt.args.get)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkPVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOk != tt.wantOk {
				t.Errorf("checkPVC() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
			if !reflect.DeepEqual(gotNewPvc, tt.wantNewPvc) {
				t.Errorf("checkPVC() gotNewPvc = %v, want %v", gotNewPvc, tt.wantNewPvc)
			}
		})
	}
}

func Test_getEndpoint(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getEndpoint(tt.args.pvc)
			if (err != nil) != tt.wantErr {
				t.Errorf("getEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSecretName(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSecretName(tt.args.client, tt.args.pvc)
			if (err != nil) != tt.wantErr {
				t.Errorf("getSecretName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getSecretName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updatePVC(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    *v1.PersistentVolumeClaim
		anno   map[string]string
		label  map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updatePVC(tt.args.client, tt.args.pvc, tt.args.anno, tt.args.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("updatePVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updatePVC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setPVCAnnotation(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    *v1.PersistentVolumeClaim
		key    string
		val    string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setPVCAnnotation(tt.args.client, tt.args.pvc, tt.args.key, tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("setPVCAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setPVCAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkIfAnnoExists(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
		key string
		val string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkIfAnnoExists(tt.args.pvc, tt.args.key, tt.args.val); got != tt.want {
				t.Errorf("checkIfAnnoExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkIfLabelExists(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
		lbl string
		val string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkIfLabelExists(tt.args.pvc, tt.args.lbl, tt.args.val); got != tt.want {
				t.Errorf("checkIfLabelExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateImporterPod(t *testing.T) {
	type args struct {
		client     kubernetes.Interface
		image      string
		verbose    string
		pullPolicy string
		ep         string
		secretName string
		pvc        *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Pod
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateImporterPod(tt.args.client, tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.ep, tt.args.secretName, tt.args.pvc)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateImporterPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateImporterPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeImporterPodSpec(t *testing.T) {
	type args struct {
		image      string
		verbose    string
		pullPolicy string
		ep         string
		secret     string
		pvc        *v1.PersistentVolumeClaim
	}
	tests := []struct {
		name string
		args args
		want *v1.Pod
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeImporterPodSpec(tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.ep, tt.args.secret, tt.args.pvc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeImporterPodSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_makeEnv(t *testing.T) {
	type args struct {
		endpoint string
		secret   string
	}
	tests := []struct {
		name string
		args args
		want []v1.EnvVar
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeEnv(tt.args.endpoint, tt.args.secret); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_addToMap(t *testing.T) {
	type args struct {
		m1 map[string]string
		m2 map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addToMap(tt.args.m1, tt.args.m2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
