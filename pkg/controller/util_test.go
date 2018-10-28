package controller

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	bootstrapapi "k8s.io/client-go/tools/bootstrap/token/api"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	. "kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/keys"
)

func TestController_pvcFromKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName)

	//run the informers
	c, pvc, pod, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.pvcFromKey() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		key interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		{
			name:    "expect to get pvc object from key",
			args:    args{fmt.Sprintf("%s/%s", pod.Namespace, pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
		{
			name:    "expect to not get pvc object from key",
			args:    args{fmt.Sprintf("%s/%s", "myns", pvc.Name)},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists, err := c.pvcFromKey(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.pvcFromKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.pvcFromKey() = %v, want %v", got, tt.want)
			}
			if tt.want == nil && exists {
				t.Errorf("Controller.pvcFromKey() expected key not to exist")
			}
		})
	}
}

func TestCloneController_pvcFromKey(t *testing.T) {

	//create staging pvc and pods
	pvcWithCloneRequestAnno := createPvc("target-pvc", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	id := string(pvcWithCloneRequestAnno.GetUID())
	sourcePod := createSourcePod(pvcWithCloneRequestAnno, id)
	targetPod := createTargetPod(pvcWithCloneRequestAnno, id, sourcePod.Namespace)

	//run the informers
	c, pvc, sourcePod, targetPod, err := createCloneController(pvcWithCloneRequestAnno, sourcePod, targetPod, "target-ns", "sourceNs")
	if err != nil {
		t.Errorf("Controller.pvcFromKey() failed to initialize fake clone controller error = %v", err)
		return
	}

	type args struct {
		key interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		{
			name:    "expect to get pvc object from key",
			args:    args{fmt.Sprintf("%s/%s", sourcePod.GetAnnotations()[AnnTargetPodNamespace], pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
		{
			name:    "expect to not get pvc object from key",
			args:    args{fmt.Sprintf("%s/%s", "myns", pvc.Name)},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "expect to get pvc object from key",
			args:    args{fmt.Sprintf("%s/%s", targetPod.Namespace, pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists, err := c.pvcFromKey(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.pvcFromKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.pvcFromKey() = %v, want %v", got, tt.want)
			}
			if tt.want == nil && exists {
				t.Errorf("Controller.pvcFromKey() expected key not to exist")
			}
		})
	}
}

func TestController_objFromKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName)

	//run the informers
	c, pvc, pod, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.objFromKey() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		informer cache.SharedIndexInformer
		key      interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:    "expect to get object key",
			args:    args{c.pvcInformer, fmt.Sprintf("%s/%s", pod.Namespace, pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
		{
			name:    "expect to not get object key",
			args:    args{c.pvcInformer, fmt.Sprintf("%s/%s", "myns", pvc.Name)},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists, err := c.objFromKey(tt.args.informer, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.objFromKey() error = %v, wantErr %v  myKey = %v", err, tt.wantErr, tt.args.key)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.objFromKey() = %v, want %v", got, tt.want)
			}
			if tt.want == nil && exists {
				t.Errorf("Controller.pvcFromKey() expected key not to exist")
			}
		})
	}
}

func TestCloneController_objFromKey(t *testing.T) {
	//create staging pvc and pods
	pvcWithCloneRequestAnno := createPvc("testPvcWithCloneRequestAnno", "target-ns", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	id := string(pvcWithCloneRequestAnno.GetUID())
	sourcePod := createSourcePod(pvcWithCloneRequestAnno, id)
	targetPod := createTargetPod(pvcWithCloneRequestAnno, id, sourcePod.Namespace)

	//run the informers
	c, pvc, sourcePod, targetPod, err := createCloneController(pvcWithCloneRequestAnno, sourcePod, targetPod, "target-ns", "source-ns")
	if err != nil {
		t.Errorf("Controller.objFromKey() failed to initialize fake clone controller error = %v", err)
		return
	}

	type args struct {
		informer cache.SharedIndexInformer
		key      interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:    "expect to get object key",
			args:    args{c.pvcInformer, fmt.Sprintf("%s/%s", sourcePod.GetAnnotations()[AnnTargetPodNamespace], pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
		{
			name:    "expect to not get object key",
			args:    args{c.pvcInformer, fmt.Sprintf("%s/%s", "myns", pvc.Name)},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "expect to get object key",
			args:    args{c.pvcInformer, fmt.Sprintf("%s/%s", targetPod.Namespace, pvc.Name)},
			want:    pvc,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists, err := c.objFromKey(tt.args.informer, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.objFromKey() error = %v, wantErr %v  myKey = %v", err, tt.wantErr, tt.args.key)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.objFromKey() = %v, want %v", got, tt.want)
			}
			if tt.want == nil && exists {
				t.Errorf("Controller.pvcFromKey() expected key not to exist")
			}
		})
	}
}

func Test_checkPVC(t *testing.T) {
	//Create base pvcs and secrets
	pvcNoAnno := createPvc("testPvcNoAnno", "default", nil, nil)
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)

	tests := []struct {
		name   string
		wantOk bool
		pvc    *v1.PersistentVolumeClaim
	}{
		{
			name:   "pvc does not have endpoint annotation or pod annotation",
			wantOk: false,
			pvc:    pvcNoAnno,
		},
		{
			name:   "pvc does have endpoint annotation and does not have a pod annotation",
			wantOk: true,
			pvc:    pvcWithEndPointAnno,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk := checkPVC(tt.pvc, AnnEndpoint)
			if gotOk != tt.wantOk {
				t.Errorf("checkPVC() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}

}

func Test_checkClonePVC(t *testing.T) {
	//Create base pvcs and secrets
	pvcNoAnno := createPvc("testPvcNoAnno", "default", nil, nil)
	pvcWithCloneRequestAnno := createPvc("testPvcWithCloneRequestAnno", "default", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)
	tests := []struct {
		name   string
		wantOk bool
		pvc    *v1.PersistentVolumeClaim
	}{
		{
			name:   "pvc does not have cloneRequest annotation or pod annotation",
			wantOk: false,
			pvc:    pvcNoAnno,
		},
		{
			name:   "pvc does have CloneRequest annotation and does not have a pod annotation",
			wantOk: true,
			pvc:    pvcWithCloneRequestAnno,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk := checkPVC(tt.pvc, AnnCloneRequest)
			if gotOk != tt.wantOk {
				t.Errorf("checkPVC() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}

}

func Test_getEndpoint(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}
	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pvcNoValue := createPvc("testPVCNoValue", "default", map[string]string{AnnEndpoint: ""}, nil)

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "expected to find endpoint annotation",
			args:    args{pvcWithAnno},
			want:    "http://test",
			wantErr: false,
		},
		{
			name:    "expected to not find endpoint annotation",
			args:    args{pvcNoAnno},
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing endpoint value",
			args:    args{pvcNoValue},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := getEndpoint(tt.args.pvc)
			if got != tt.want {
				t.Errorf("getEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSource(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}

	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcNoneAnno := createPvc("testPVCNoneAnno", "default", map[string]string{AnnSource: SourceNone}, nil)
	pvcGlanceAnno := createPvc("testPVCNoneAnno", "default", map[string]string{AnnSource: SourceGlance}, nil)
	pvcInvalidValue := createPvc("testPVCInvalidValue", "default", map[string]string{AnnSource: "iaminvalid"}, nil)
	pvcRegistryAnno := createPvc("testPVCRegistryAnno", "default", map[string]string{AnnSource: SourceRegistry}, nil)

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "expected to find none anno",
			args: args{pvcNoneAnno},
			want: SourceNone,
		},
		{
			name: "expected to find http with invalid value",
			args: args{pvcInvalidValue},
			want: SourceHTTP,
		},
		{
			name: "expected to find http with no anno",
			args: args{pvcNoAnno},
			want: SourceHTTP,
		},
		{
			name: "expected to find glance with glance anno",
			args: args{pvcGlanceAnno},
			want: SourceGlance,
		},
		{
			name: "expected to find registry with registry anno",
			args: args{pvcRegistryAnno},
			want: SourceRegistry,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSource(tt.args.pvc)
			if got != tt.want {
				t.Errorf("getSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getContentType(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}

	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcArchiveAnno := createPvc("testPVCArchiveAnno", "default", map[string]string{AnnContentType: ContentTypeArchive}, nil)
	pvcKubevirtAnno := createPvc("testPVCKubevirtAnno", "default", map[string]string{AnnContentType: ContentTypeKubevirt}, nil)
	pvcInvalidValue := createPvc("testPVCInvalidValue", "default", map[string]string{AnnContentType: "iaminvalid"}, nil)

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "expected to kubevirt content type",
			args: args{pvcNoAnno},
			want: ContentTypeKubevirt,
		},
		{
			name: "expected to find archive content type",
			args: args{pvcArchiveAnno},
			want: ContentTypeArchive,
		},
		{
			name: "expected to kubevirt content type",
			args: args{pvcKubevirtAnno},
			want: ContentTypeKubevirt,
		},
		{
			name: "expected to find kubevirt with invalid anno",
			args: args{pvcInvalidValue},
			want: ContentTypeKubevirt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContentType(tt.args.pvc)
			if got != tt.want {
				t.Errorf("getSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getImageSize(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "expected to get size 1G",
			args:    args{createPvc("testPVC", "default", nil, nil)},
			want:    "1G",
			wantErr: false,
		},
		{
			name:    "expected to get error, because of missing size",
			args:    args{createPvcNoSize("testPVC", "default", nil, nil)},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRequestedImageSize(tt.args.pvc)
			if err != nil && !tt.wantErr {
				t.Errorf("Error retrieving adjusted image size, when not expecting error: %s", err.Error())
			}
			if got != tt.want {
				t.Errorf("getSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSecretName(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		pvc    *v1.PersistentVolumeClaim
	}
	//Create base pvcs and secrets
	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnSecret: "mysecret"}, nil)
	testSecret1 := createSecret("mysecret", "default", "mysecretkey", "mysecretstring", map[string]string{AnnSecret: "mysecret"})
	testSecret2 := createSecret("mysecret2", "default", "mysecretkey2", "mysecretstring2", map[string]string{AnnSecret: "mysecret2"})

	// set test env
	myclient := k8sfake.NewSimpleClientset(pvcNoAnno, pvcWithAnno, testSecret1, testSecret2)

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "expected to find secret",
			args:    args{myclient, pvcWithAnno},
			want:    "mysecret",
			wantErr: false,
		},
		{
			name:    "expected to not find secret",
			args:    args{myclient, pvcNoAnno},
			want:    "",
			wantErr: false,
		},
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
	//Create base pvc
	pvcNoAnno := createPvc("testPVC1", "default", nil, nil)

	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		{
			name:    "pvc is updated with annotation and label",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, map[string]string{AnnCreatedBy: "cdi"}, map[string]string{CDILabelKey: CDILabelValue}},
			want:    createPvc("testPVC1", "default", map[string]string{AnnCreatedBy: "cdi"}, map[string]string{CDILabelKey: CDILabelValue}),
			wantErr: false,
		},
		{
			name:    "pvc is updated with annotation",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, map[string]string{AnnCreatedBy: "cdi"}, nil},
			want:    createPvc("testPVC1", "default", map[string]string{AnnCreatedBy: "cdi"}, nil),
			wantErr: false,
		},
		{
			name:    "pvc is updated with label",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, nil, map[string]string{CDILabelKey: CDILabelValue}},
			want:    createPvc("testPVC1", "default", nil, map[string]string{CDILabelKey: CDILabelValue}),
			wantErr: false,
		},
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

	//Create base pvc
	pvcNoAnno := createPvc("testPVC1", "default", nil, nil)

	tests := []struct {
		name    string
		args    args
		want    *v1.PersistentVolumeClaim
		wantErr bool
	}{
		{
			name:    "pvc is updated with new annotation",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, AnnCreatedBy, "cdi"},
			want:    createPvc("testPVC1", "default", map[string]string{AnnCreatedBy: "cdi"}, nil),
			wantErr: false,
		},
		{
			name:    "pvc is updated with new annotation - empty value",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, AnnCreatedBy, ""},
			want:    createPvc("testPVC1", "default", map[string]string{AnnCreatedBy: ""}, nil),
			wantErr: false,
		},
		// TODO: Do we want to allow an Empty Key??
		{
			name:    "pvc is not updated with new annotation - empty key",
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, "", "cdi"},
			want:    createPvc("testPVC1", "default", map[string]string{"": "cdi"}, nil),
			wantErr: false,
		},
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

	//create PVCs
	pvc := createPvc("testPVC", "default", map[string]string{AnnPodPhase: "Running"}, nil)
	pvcNoAnno := createPvc("testPVC2", "default", nil, nil)
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "pvc does have expected annotation key and value",
			args: args{pvc, AnnPodPhase, "Running"},
			want: true,
		},
		{
			name: "pvc does not have expected annotation key and value",
			args: args{pvc, AnnEndpoint, "http://test"},
			want: false,
		},
		{
			name: "pvc does have expected key but not value",
			args: args{pvc, AnnPodPhase, "Pending"},
			want: false,
		},
		{
			name: "pvc does not have any annotations",
			args: args{pvcNoAnno, AnnPodPhase, "Running"},
			want: false,
		},
		{
			name: "pvc does have expected annotations but pass in empty strings",
			args: args{pvc, "", ""},
			want: false,
		},
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
	//create PVCs
	pvc := createPvc("testPVC", "default", nil, map[string]string{CDILabelKey: CDILabelValue})
	pvcNoLbl := createPvc("testPVC2", "default", nil, nil)

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "pvc does have expected label and expected value",
			args: args{pvc, CDILabelKey, CDILabelValue},
			want: true,
		},
		{
			name: "pvc does not have expected label",
			args: args{pvc, AnnCreatedBy, "yes"},
			want: false,
		},
		{
			name: "pvc does have expected label but does not have expected value",
			args: args{pvc, CDILabelKey, "something"},
			want: false,
		},
		{
			name: "pvc does not have any labels",
			args: args{pvcNoLbl, CDILabelKey, CDILabelValue},
			want: false,
		},
		{
			name: "pvc does have labels but pass in empty search strings",
			args: args{pvc, "", ""},
			want: false,
		},
	}
	// checkIfLabelExists expects both label to be present and correct value to match
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
		podEnvVar  *importPodEnvVar
		pvc        *v1.PersistentVolumeClaim
	}

	// create PVC
	pvc := createPvc("testPVC2", "", nil, nil)

	tests := []struct {
		name    string
		args    args
		want    *v1.Pod
		wantErr bool
	}{
		{
			name:    "expect pod to be created",
			args:    args{k8sfake.NewSimpleClientset(pvc), "test/image", "-v=5", "Always", &importPodEnvVar{"", "", "", "", "1G"}, pvc},
			want:    MakeImporterPodSpec("test/image", "-v=5", "Always", &importPodEnvVar{"", "", "", "", "1G"}, pvc),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateImporterPod(tt.args.client, tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.podEnvVar, tt.args.pvc)
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
		podEnvVar  *importPodEnvVar
		pvc        *v1.PersistentVolumeClaim
	}
	// create PVC
	pvc := createPvc("testPVC2", "default", nil, nil)
	// create POD
	pod := createPod(pvc, DataVolName)

	tests := []struct {
		name    string
		args    args
		wantPod *v1.Pod
	}{
		{
			name:    "expect pod to be created",
			args:    args{"test/myimage", "5", "Always", &importPodEnvVar{"", "", SourceHTTP, ContentTypeKubevirt, "1G"}, pvc},
			wantPod: pod,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeImporterPodSpec(tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.podEnvVar, tt.args.pvc)

			if !reflect.DeepEqual(got, tt.wantPod) {
				t.Errorf("MakeImporterPodSpec() =\n%v\n, want\n%v", got, tt.wantPod)
			}

		})
	}
}

func Test_makeEnv(t *testing.T) {
	const mockUID = "1111-1111-1111-1111"

	type args struct {
		podEnvVar *importPodEnvVar
	}

	tests := []struct {
		name string
		args args
		want []v1.EnvVar
	}{
		{
			name: "env should match",
			args: args{&importPodEnvVar{"myendpoint", "mysecret", SourceHTTP, ContentTypeKubevirt, "1G"}},
			want: createEnv(&importPodEnvVar{"myendpoint", "mysecret", SourceHTTP, ContentTypeKubevirt, "1G"}, mockUID),
		},
	}
	for _, tt := range tests {
		{
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := makeEnv(tt.args.podEnvVar, mockUID); !reflect.DeepEqual(got, tt.want) {
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
		{
			name: "use different key for map1 and map2 expect both maps to be returned",
			args: args{map[string]string{AnnImportPod: "mypod"}, map[string]string{CDILabelKey: CDILabelValue}},
			want: map[string]string{AnnImportPod: "mypod", CDILabelKey: CDILabelValue},
		},
		{
			name: "use same key for map1 and map2 expect map2 to be returned",
			args: args{map[string]string{AnnImportPod: "mypod"}, map[string]string{AnnImportPod: "map2pod"}},
			want: map[string]string{AnnImportPod: "map2pod"},
		},
		{
			name: "pass in empty map1 and map2 expect empty map",
			args: args{nil, nil},
			want: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addToMap(tt.args.m1, tt.args.m2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createPod(pvc *v1.PersistentVolumeClaim, dvname string) *v1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s-", ImporterPodName, pvc.Name)

	blockOwnerDeletion := true
	isController := true

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				CDILabelKey:       CDILabelValue,
				CDIComponentLabel: ImporterPodName,
				LabelImportPvc:    pvc.Name,
				PrometheusLabel:   "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            ImporterPodName,
					Image:           "test/myimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: ImporterDataDir,
						},
					},
					Args: []string{"-v=5"},
					Ports: []v1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      v1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes: []v1.Volume{
				{
					Name: dvname,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	ep, _ := getEndpoint(pvc)
	source := getSource(pvc)
	contentType := getContentType(pvc)
	imageSize, _ := getRequestedImageSize(pvc)

	env := []v1.EnvVar{
		{
			Name:  ImporterSource,
			Value: source,
		},
		{
			Name:  ImporterEndpoint,
			Value: ep,
		},
		{
			Name:  ImporterContentType,
			Value: contentType,
		},
		{
			Name:  ImporterImageSize,
			Value: imageSize,
		},
		{
			Name:  OwnerUID,
			Value: string(pvc.UID),
		},
	}
	pod.Spec.Containers[0].Env = env
	return pod
}

func createPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
				},
			},
		},
	}
}

func createPvcNoSize(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
		},
	}
}

func createClonePvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
			UID:         "pvc-uid",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
				},
			},
		},
	}
}

func createSecret(name, ns, accessKey, secretKey string, labels map[string]string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			bootstrapapi.BootstrapTokenIDKey:           []byte(accessKey),
			bootstrapapi.BootstrapTokenSecretKey:       []byte(secretKey),
			bootstrapapi.BootstrapTokenUsageSigningKey: []byte("true"),
		},
	}
}

func createEnv(podEnvVar *importPodEnvVar, uid string) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  ImporterSource,
			Value: podEnvVar.source,
		},
		{
			Name:  ImporterEndpoint,
			Value: podEnvVar.ep,
		},
		{
			Name:  ImporterContentType,
			Value: podEnvVar.contentType,
		},
		{
			Name:  ImporterImageSize,
			Value: podEnvVar.imageSize,
		},
		{
			Name:  OwnerUID,
			Value: string(uid),
		},
	}
	if podEnvVar.secretName != "" {
		env = append(env, v1.EnvVar{
			Name: ImporterAccessKeyID,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: KeyAccess,
				},
			},
		}, v1.EnvVar{
			Name: ImporterSecretKey,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: KeySecret,
				},
			},
		})
	}
	return env
}

func createImportController(pvcSpec *v1.PersistentVolumeClaim, podSpec *v1.Pod, ns string) (*ImportController, *v1.PersistentVolumeClaim, *v1.Pod, error) {
	//Set up environment
	myclient := k8sfake.NewSimpleClientset()

	//create staging pvc and pod
	pvc, err := myclient.CoreV1().PersistentVolumeClaims(ns).Create(pvcSpec)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pvc error = %v", err)
	}

	pod, err := myclient.CoreV1().Pods(ns).Create(podSpec)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pod error = %v", err)
	}

	// create informers and queue
	k8sI := kubeinformers.NewSharedInformerFactory(myclient, noResyncPeriodFunc())

	pvcInformer := k8sI.Core().V1().PersistentVolumeClaims()
	podInformer := k8sI.Core().V1().Pods()

	pvcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	pvcQueue.Add(pvc)

	k8sI.Core().V1().PersistentVolumeClaims().Informer().GetIndexer().Add(pvc)
	k8sI.Core().V1().Pods().Informer().GetIndexer().Add(pod)

	//run the informers
	stop := make(chan struct{})
	go pvcInformer.Informer().Run(stop)
	go podInformer.Informer().Run(stop)
	cache.WaitForCacheSync(stop, podInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stop, pvcInformer.Informer().HasSynced)
	defer close(stop)

	c := NewImportController(myclient, pvcInformer, podInformer, "test/image", "Always", "-v=5")
	return c, pvc, pod, nil
}

func createCloneController(pvcSpec *v1.PersistentVolumeClaim, sourcePodSpec *v1.Pod, targetPodSpec *v1.Pod, targetNs, sourceNs string) (*CloneController, *v1.PersistentVolumeClaim, *v1.Pod, *v1.Pod, error) {
	//Set up environment
	myclient := k8sfake.NewSimpleClientset()

	//create staging pvc and pods
	pvc, err := myclient.CoreV1().PersistentVolumeClaims(targetNs).Create(pvcSpec)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pvc error = %v", err)
	}

	sourcePod, err := myclient.CoreV1().Pods(sourceNs).Create(sourcePodSpec)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("createCloneController: failed to initialize and create source pod error = %v", err)
	}
	targetPod, err := myclient.CoreV1().Pods(targetNs).Create(targetPodSpec)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("createCloneController: failed to initialize and create target pod error = %v", err)
	}

	// create informers and queue
	k8sI := kubeinformers.NewSharedInformerFactory(myclient, noResyncPeriodFunc())

	pvcInformer := k8sI.Core().V1().PersistentVolumeClaims()
	podInformer := k8sI.Core().V1().Pods()

	pvcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	pvcQueue.Add(pvc)

	k8sI.Core().V1().PersistentVolumeClaims().Informer().GetIndexer().Add(pvc)
	k8sI.Core().V1().Pods().Informer().GetIndexer().Add(sourcePod)
	k8sI.Core().V1().Pods().Informer().GetIndexer().Add(targetPod)

	//run the informers
	stop := make(chan struct{})
	go pvcInformer.Informer().Run(stop)
	go podInformer.Informer().Run(stop)
	cache.WaitForCacheSync(stop, podInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stop, pvcInformer.Informer().HasSynced)
	defer close(stop)

	c := NewCloneController(myclient, pvcInformer, podInformer, "test/mycloneimage", "Always", "-v=5")
	return c, pvc, sourcePod, targetPod, nil
}

func createSourcePod(pvc *v1.PersistentVolumeClaim, id string) *v1.Pod {
	_, sourcePvcName := ParseSourcePvcAnnotation(pvc.GetAnnotations()[AnnCloneRequest], "/")
	// source pod name contains the pvc name
	podName := fmt.Sprintf("%s-", ClonerSourcePodName)
	blockOwnerDeletion := true
	isController := true
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy:          "yes",
				AnnTargetPodNamespace: pvc.Namespace,
			},
			Labels: map[string]string{
				CDILabelKey:       CDILabelValue, //filtered by the podInformer
				CDIComponentLabel: ClonerSourcePodName,
				CloningLabelKey:   CloningLabelValue + "-" + id, //used by podAffity
				// this label is used when searching for a pvc's cloner source pod.
				CloneUniqueID: pvc.Name + "-source-pod",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            ClonerSourcePodName,
					Image:           "test/mycloneimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					SecurityContext: &v1.SecurityContext{
						Privileged: &[]bool{true}[0],
						RunAsUser:  &[]int64{0}[0],
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      ImagePathName,
							MountPath: ClonerImagePath,
						},
						{
							Name:      socketPathName,
							MountPath: ClonerSocketPath + "/" + id,
						},
					},
					Args: []string{"source", id},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: ImagePathName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
							ReadOnly:  false,
						},
					},
				},
				{
					Name: socketPathName,
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: ClonerSocketPath + "/" + id,
						},
					},
				},
			},
		},
	}
	return pod
}

func createTargetPod(pvc *v1.PersistentVolumeClaim, id, podAffinityNamespace string) *v1.Pod {
	// target pod name contains the pvc name
	podName := fmt.Sprintf("%s-", ClonerTargetPodName)
	blockOwnerDeletion := true
	isController := true
	ownerUID := pvc.UID
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy:          "yes",
				AnnTargetPodNamespace: pvc.Namespace,
			},
			Labels: map[string]string{
				CDILabelKey:       CDILabelValue, //filtered by the podInformer
				CDIComponentLabel: ClonerTargetPodName,
				// this label is used when searching for a pvc's cloner target pod.
				CloneUniqueID:   pvc.Name + "-target-pod",
				PrometheusLabel: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.PodSpec{
			Affinity: &v1.Affinity{
				PodAffinity: &v1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      CloningLabelKey,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{CloningLabelValue + "-" + id},
									},
								},
							},
							Namespaces:  []string{podAffinityNamespace}, //the scheduler looks for the namespace of the source pod
							TopologyKey: CloningTopologyKey,
						},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:            ClonerTargetPodName,
					Image:           "test/mycloneimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					SecurityContext: &v1.SecurityContext{
						Privileged: &[]bool{true}[0],
						RunAsUser:  &[]int64{0}[0],
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      ImagePathName,
							MountPath: ClonerImagePath,
						},
						{
							Name:      socketPathName,
							MountPath: ClonerSocketPath + "/" + id,
						},
					},
					Args: []string{"target", id},
					Ports: []v1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      v1.ProtocolTCP,
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  OwnerUID,
							Value: string(ownerUID),
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: ImagePathName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  false,
						},
					},
				},
				{
					Name: socketPathName,
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: ClonerSocketPath + "/" + id,
						},
					},
				},
			},
		},
	}
	return pod
}

func getPvcKey(pvc *corev1.PersistentVolumeClaim, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pvc)
	if err != nil {
		t.Errorf("Unexpected error getting key for pvc %v: %v", pvc.Name, err)
		return ""
	}
	return key
}

func createUploadPod(pvc *v1.PersistentVolumeClaim) *v1.Pod {
	name := "cdi-upload-" + pvc.Name
	secretName := name + "-server-tls"

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				"app":             "containerized-data-importer",
				"cdi.kubevirt.io": "cdi-upload-server",
				"service":         name,
			},
			OwnerReferences: []metav1.OwnerReference{
				MakeOwnerReference(pvc),
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "cdi-upload-server",
					Image:           "test/myimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: "/data",
						},
					},
					Env: []v1.EnvVar{
						{
							Name: "TLS_KEY",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSKeyFile,
								},
							},
						},
						{
							Name: "TLS_CERT",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSCertFile,
								},
							},
						},
						{
							Name: "CLIENT_CERT",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSCAFile,
								},
							},
						},
					},
					Args: []string{"-v=" + "5"},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes: []v1.Volume{
				{
					Name: DataVolName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
	return pod
}

func createUploadService(pvc *v1.PersistentVolumeClaim) *v1.Service {
	name := "cdi-upload-" + pvc.Name
	blockOwnerDeletion := true
	isController := true
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				"app":             "containerized-data-importer",
				"cdi.kubevirt.io": "cdi-upload-server",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8443,
					},
				},
			},
			Selector: map[string]string{
				"service": name,
			},
		},
	}
	return service
}
