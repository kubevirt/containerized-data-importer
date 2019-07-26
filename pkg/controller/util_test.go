package controller

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/util/cert"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"

	crdv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdifake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/keys"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

func TestController_pvcFromKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName, nil)

	//run the informers
	c, pvc, pod, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.pvcFromKey() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		key string
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

func TestController_objFromKey(t *testing.T) {
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName, nil)

	//run the informers
	c, pvc, pod, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.objFromKey() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		informer cache.SharedIndexInformer
		key      string
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

func Test_getVolumeMode(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}

	pvcVolumeModeBlock := createBlockPvc("testPVCVolumeModeBlock", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystem := createPvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystemDefault := createPvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)

	tests := []struct {
		name string
		args args
		want corev1.PersistentVolumeMode
	}{
		{
			name: "expected volumeMode to be Block",
			args: args{pvcVolumeModeBlock},
			want: corev1.PersistentVolumeBlock,
		},
		{
			name: "expected volumeMode to be Filesystem",
			args: args{pvcVolumeModeFilesystem},
			want: corev1.PersistentVolumeFilesystem,
		},
		{
			name: "expected volumeMode to be Filesystem",
			args: args{pvcVolumeModeFilesystemDefault},
			want: corev1.PersistentVolumeFilesystem,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVolumeMode(tt.args.pvc)
			if got != tt.want {
				t.Errorf("getVolumeMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getContentType(t *testing.T) {
	type args struct {
		pvc *v1.PersistentVolumeClaim
	}

	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcArchiveAnno := createPvc("testPVCArchiveAnno", "default", map[string]string{AnnContentType: string(cdiv1.DataVolumeArchive)}, nil)
	pvcKubevirtAnno := createPvc("testPVCKubevirtAnno", "default", map[string]string{AnnContentType: string(cdiv1.DataVolumeKubeVirt)}, nil)
	pvcInvalidValue := createPvc("testPVCInvalidValue", "default", map[string]string{AnnContentType: "iaminvalid"}, nil)

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "expected to kubevirt content type",
			args: args{pvcNoAnno},
			want: string(cdiv1.DataVolumeKubeVirt),
		},
		{
			name: "expected to find archive content type",
			args: args{pvcArchiveAnno},
			want: string(cdiv1.DataVolumeArchive),
		},
		{
			name: "expected to kubevirt content type",
			args: args{pvcKubevirtAnno},
			want: string(cdiv1.DataVolumeKubeVirt),
		},
		{
			name: "expected to find kubevirt with invalid anno",
			args: args{pvcInvalidValue},
			want: string(cdiv1.DataVolumeKubeVirt),
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

func Test_getCloneRequestPVCAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		wantOk     bool
		annKey     string
		annValue   string
		annErrType string
	}{
		{
			name:       "pvc without annotation should return error",
			wantOk:     false,
			annKey:     "",
			annValue:   "",
			annErrType: "missing",
		},
		{
			name:       "pvc with blank annotation should return error",
			wantOk:     false,
			annKey:     AnnCloneRequest,
			annValue:   "",
			annErrType: "empty",
		},
		{
			name:       "pvc with valid clone annotation",
			wantOk:     true,
			annKey:     AnnCloneRequest,
			annValue:   "default/pvc-name",
			annErrType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := createPvc("test", "default", map[string]string{tt.annKey: tt.annValue}, nil)
			ann, err := getCloneRequestPVCAnnotation(pvc)
			if !tt.wantOk && err == nil {
				t.Error("Got no error when expecting one")
			} else if tt.wantOk && err != nil {
				t.Errorf("Got error %+v when not expecting one", err)
			}
			if !tt.wantOk && err != nil {
				// Verify that the error contains what we are expecting.
				if !strings.Contains(err.Error(), tt.annErrType) {
					t.Errorf("Expecting error message to contain %s, but not found", tt.annErrType)
				}
			} else if tt.wantOk && err == nil {
				if ann != tt.annValue {
					t.Error("expected annotation did not match found annotation")
				}
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
			name:    "expect pod to be created for PVC with VolumeMode Filesystem",
			args:    args{k8sfake.NewSimpleClientset(pvc), "test/image", "-v=5", "Always", &importPodEnvVar{"", "", "", "", "1G", "", false}, pvc},
			want:    MakeImporterPodSpec("test/image", "-v=5", "Always", &importPodEnvVar{"", "", "", "", "1G", "", false}, pvc, nil),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateImporterPod(tt.args.client, tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.podEnvVar, tt.args.pvc, nil)
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
	// create PVC with VolumeMode: Filesystem
	pvc := createPvc("testPVC2", "default", nil, nil)
	// create POD
	pod := createPod(pvc, DataVolName, nil)

	// create PVC with VolumeMode: Block
	pvc1 := createBlockPvc("testPVC2", "default", nil, nil)
	// create POD
	pod1 := createPod(pvc1, DataVolName, nil)

	tests := []struct {
		name    string
		args    args
		wantPod *v1.Pod
	}{
		{
			name:    "expect pod to be created for PVC with VolumeMode: Filesystem",
			args:    args{"test/myimage", "5", "Always", &importPodEnvVar{"", "", SourceHTTP, string(cdiv1.DataVolumeKubeVirt), "1G", "", false}, pvc},
			wantPod: pod,
		},
		{
			name:    "expect pod to be created for PVC with VolumeMode: Block",
			args:    args{"test/myimage", "5", "Always", &importPodEnvVar{"", "", SourceHTTP, string(cdiv1.DataVolumeKubeVirt), "1G", "", false}, pvc1},
			wantPod: pod1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeImporterPodSpec(tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.podEnvVar, tt.args.pvc, nil)

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
			args: args{&importPodEnvVar{"myendpoint", "mysecret", SourceHTTP, string(cdiv1.DataVolumeKubeVirt), "1G", "", false}},
			want: createEnv(&importPodEnvVar{"myendpoint", "mysecret", SourceHTTP, string(cdiv1.DataVolumeKubeVirt), "1G", "", false}, mockUID),
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

func TestMakeCDIConfigSpec(t *testing.T) {
	type args struct {
		name string
	}
	config := createCDIConfigWithStorageClass("testConfig", "")

	tests := []struct {
		name          string
		args          args
		wantCDIConfig *cdiv1.CDIConfig
	}{
		{
			name:          "expect CDIConfig to be created",
			args:          args{"testConfig"},
			wantCDIConfig: config,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeEmptyCDIConfigSpec(tt.args.name)

			if !reflect.DeepEqual(got, tt.wantCDIConfig) {
				t.Errorf("MakeEmptyCDIConfigSpec() =\n%v\n, want\n%v", got, tt.wantCDIConfig)
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

func Test_getCertConfigMap(t *testing.T) {
	namespace := "default"
	configMapName := "foobar"

	testPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"cdi.kubevirt.io/storage.import.certConfigMap": configMapName,
			},
		},
	}

	testConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
	}

	type args struct {
		pvc    *corev1.PersistentVolumeClaim
		objs   []runtime.Object
		result string
	}

	for _, arg := range []args{
		{&corev1.PersistentVolumeClaim{}, nil, ""},
		{testPVC, []runtime.Object{testConfigMap}, configMapName},
		{testPVC, nil, configMapName},
	} {
		client := k8sfake.NewSimpleClientset(arg.objs...)

		result, err := getCertConfigMap(client, arg.pvc)

		if err != nil {
			t.Errorf("Enexpected error %+v", err)
		}

		if result != arg.result {
			t.Errorf("Expected %s got %s", arg.result, result)
		}
	}
}

func Test_getInsecureTLS(t *testing.T) {
	namespace := "cdi"
	configMapName := "cdi-insecure-registries"
	host := "myregistry"
	endpointNoPort := "docker://" + host
	hostWithPort := host + ":5000"
	endpointWithPort := "docker://" + hostWithPort

	type args struct {
		endpoint       string
		confiMapExists bool
		insecureHost   string
		result         bool
	}

	for _, arg := range []args{
		{endpointNoPort, true, host, true},
		{endpointWithPort, true, hostWithPort, true},
		{endpointNoPort, true, hostWithPort, false},
		{endpointWithPort, true, host, false},
		{endpointNoPort, false, "", false},
		{"", true, host, false},
	} {
		var objs []runtime.Object

		pvc := &corev1.PersistentVolumeClaim{}
		if arg.endpoint != "" {
			pvc.Annotations = map[string]string{
				"cdi.kubevirt.io/storage.import.endpoint": arg.endpoint,
			}
		}

		if arg.confiMapExists {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
				},
			}

			if arg.insecureHost != "" {
				cm.Data = map[string]string{
					"test-registry": arg.insecureHost,
				}
			}

			objs = append(objs, cm)
		}

		client := k8sfake.NewSimpleClientset(objs...)

		result, err := isInsecureTLS(client, pvc)

		if err != nil {
			t.Errorf("Enexpected error %+v", err)
		}

		if result != arg.result {
			t.Errorf("Expected %t got %t", arg.result, result)
		}
	}
}

func Test_GetScratchPvcStorageClassDefault(t *testing.T) {
	var objs []runtime.Object
	objs = append(objs, createStorageClass("test1", nil))
	objs = append(objs, createStorageClass("test2", nil))
	objs = append(objs, createStorageClass("test3", map[string]string{
		AnnDefaultStorageClass: "true",
	}))
	client := k8sfake.NewSimpleClientset(objs...)

	storageClassName := "test3"
	var cdiObjs []runtime.Object
	cdiObjs = append(cdiObjs, createCDIConfigWithStorageClass(common.ConfigName, storageClassName))
	cdiclient := cdifake.NewSimpleClientset(cdiObjs...)

	pvc := createPvc("test", "test", nil, nil)
	result := GetScratchPvcStorageClass(client, cdiclient, pvc)

	if result != storageClassName {
		t.Error("Storage class is not test3")
	}
}

func Test_GetScratchPvcStorageClassConfig(t *testing.T) {
	var objs []runtime.Object
	objs = append(objs, createStorageClass("test1", nil))
	objs = append(objs, createStorageClass("test2", nil))
	objs = append(objs, createStorageClass("test3", map[string]string{
		AnnDefaultStorageClass: "true",
	}))
	client := k8sfake.NewSimpleClientset(objs...)

	storageClassName := "test1"
	var cdiObjs []runtime.Object
	config := createCDIConfigWithStorageClass(common.ConfigName, storageClassName)
	config.Spec.ScratchSpaceStorageClass = &storageClassName
	cdiObjs = append(cdiObjs, config)
	cdiclient := cdifake.NewSimpleClientset(cdiObjs...)

	pvc := createPvc("test", "test", nil, nil)
	result := GetScratchPvcStorageClass(client, cdiclient, pvc)

	if result != storageClassName {
		t.Error("Storage class is not test1")
	}
}

func Test_GetScratchPvcStorageClassPvc(t *testing.T) {
	var objs []runtime.Object
	client := k8sfake.NewSimpleClientset(objs...)

	storageClass := "storageClass"
	var cdiObjs []runtime.Object
	cdiObjs = append(cdiObjs, createCDIConfigWithStorageClass(common.ConfigName, storageClass))
	cdiclient := cdifake.NewSimpleClientset(cdiObjs...)

	pvc := createPvcInStorageClass("test", "test", &storageClass, nil, nil)
	result := GetScratchPvcStorageClass(client, cdiclient, pvc)

	if result != storageClass {
		t.Error("Storage class is not storageClass")
	}
}

func Test_DecodePublicKey(t *testing.T) {
	bytes, err := cert.EncodePublicKeyPEM(&getAPIServerKey().PublicKey)
	if err != nil {
		t.Errorf("error encoding public key")
	}

	_, err = DecodePublicKey(bytes)
	if err != nil {
		t.Errorf("error decoding public key")
	}
}

func Test_TokenValidation(t *testing.T) {

	goodTokenData := func() *token.Payload {
		return &token.Payload{
			Operation: token.OperationClone,
			Name:      "source",
			Namespace: "sourcens",
			Resource: metav1.GroupVersionResource{
				Resource: "persistentvolumeclaims",
			},
			Params: map[string]string{
				"targetName":      "target",
				"targetNamespace": "targetns",
			},
		}
	}

	badOperation := goodTokenData()
	badOperation.Operation = token.OperationUpload

	badSourceName := goodTokenData()
	badSourceName.Name = "foo"

	badSourceNamespace := goodTokenData()
	badSourceNamespace.Namespace = "foo"

	badResource := goodTokenData()
	badResource.Resource.Resource = "foo"

	badTargetName := goodTokenData()
	badTargetName.Params["targetName"] = "foo"

	badTargetNamespace := goodTokenData()
	badTargetNamespace.Params["targetNamespace"] = "foo"

	missingParams := goodTokenData()
	missingParams.Params = nil

	g := token.NewGenerator(common.CloneTokenIssuer, getAPIServerKey(), 5*time.Minute)
	v := newCloneTokenValidator(&getAPIServerKey().PublicKey)

	payloads := []*token.Payload{
		goodTokenData(),
		badOperation,
		badSourceName,
		badSourceNamespace,
		badResource,
		badTargetName,
		badTargetNamespace,
		missingParams,
	}

	for _, p := range payloads {
		tokenString, err := g.Generate(p)
		if err != nil {
			panic("error generating token")
		}

		source := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "source",
				Namespace: "sourcens",
			},
		}

		target := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target",
				Namespace: "targetns",
				Annotations: map[string]string{
					AnnCloneToken: tokenString,
				},
			},
		}

		err = validateCloneToken(v, source, target)
		if err == nil && !reflect.DeepEqual(p, goodTokenData()) {
			t.Error("validation should have failed")
		} else if err != nil && reflect.DeepEqual(p, goodTokenData()) {
			t.Error("validation should have succeeded")
		}
	}
}

func createPod(pvc *v1.PersistentVolumeClaim, dvname string, scratchPvc *v1.PersistentVolumeClaim) *v1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s-", ImporterPodName, pvc.Name)

	blockOwnerDeletion := true
	isController := true

	volumes := []v1.Volume{
		{
			Name: DataVolName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	if scratchPvc != nil {
		volumes = append(volumes, v1.Volume{
			Name: ScratchVolName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: scratchPvc.Name,
					ReadOnly:  false,
				},
			},
		})
	}

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
					Args:            []string{"-v=5"},
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
			Volumes:       volumes,
		},
	}

	ep, _ := getEndpoint(pvc)
	source := getSource(pvc)
	contentType := getContentType(pvc)
	imageSize, _ := getRequestedImageSize(pvc)
	volumeMode := getVolumeMode(pvc)

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
		{
			Name:  InsecureTLSVar,
			Value: "false",
		},
	}
	pod.Spec.Containers[0].Env = env
	if volumeMode == v1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser: &[]int64{0}[0],
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = addVolumeMounts()
	}

	if scratchPvc != nil {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}

	return pod
}

func createBlockPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	pvcDef := createPvcInStorageClass(name, ns, nil, annotations, labels)
	volumeMode := v1.PersistentVolumeBlock
	pvcDef.Spec.VolumeMode = &volumeMode
	return pvcDef
}

func createPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return createPvcInStorageClass(name, ns, nil, annotations, labels)
}

func createPvcInStorageClass(name, ns string, storageClassName *string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
			UID:         types.UID(ns + "/" + name),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
				},
			},
			StorageClassName: storageClassName,
		},
	}
}

func createScratchPvc(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, storageClassName string) *v1.PersistentVolumeClaim {
	labels := map[string]string{
		"cdi-controller": pod.Name,
		"app":            "containerized-data-importer",
		LabelImportPvc:   pvc.Name,
	}

	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc.Name + "-scratch",
			Namespace: pvc.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       pod.Name,
					UID:        pod.GetUID(),
				},
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources:        pvc.Spec.Resources,
			StorageClassName: &storageClassName,
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

func createClonePvc(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return createClonePvcWithSize(sourceNamespace, sourceName, targetNamespace, targetName, annotations, labels, "1G")
}

func createClonePvcWithSize(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string, size string) *v1.PersistentVolumeClaim {
	tokenData := &token.Payload{
		Operation: token.OperationClone,
		Name:      sourceName,
		Namespace: sourceNamespace,
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
		Params: map[string]string{
			"targetNamespace": targetNamespace,
			"targetName":      targetName,
		},
	}

	g := token.NewGenerator(common.CloneTokenIssuer, getAPIServerKey(), 5*time.Minute)

	tokenString, err := g.Generate(tokenData)
	if err != nil {
		panic("error generating token")
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[AnnCloneRequest] = fmt.Sprintf("%s/%s", sourceNamespace, sourceName)
	annotations[AnnCloneToken] = tokenString

	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        targetName,
			Namespace:   targetNamespace,
			Annotations: annotations,
			Labels:      labels,
			UID:         "pvc-uid",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(size),
				},
			},
		},
	}
}

func createCloneBlockPvc(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	pvc := createClonePvc(sourceNamespace, sourceName, targetNamespace, targetName, annotations, labels)
	VolumeMode := v1.PersistentVolumeBlock
	pvc.Spec.VolumeMode = &VolumeMode
	return pvc
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
		{
			Name:  InsecureTLSVar,
			Value: strconv.FormatBool(podEnvVar.insecureTLS),
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
	cdiclient := cdifake.NewSimpleClientset()

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

	c := NewImportController(myclient, cdiclient, pvcInformer, podInformer, "test/image", "Always", "-v=5")
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

	key := &getAPIServerKey().PublicKey

	c := NewCloneController(myclient, pvcInformer, podInformer, "test/mycloneimage", "Always", "-v=5", key)
	return c, pvc, sourcePod, targetPod, nil
}

func createSourcePod(pvc *v1.PersistentVolumeClaim, pvcUID string) *v1.Pod {
	_, sourcePvcName := ParseSourcePvcAnnotation(pvc.GetAnnotations()[AnnCloneRequest], "/")
	podName := fmt.Sprintf("%s-%s-", common.ClonerSourcePodName, sourcePvcName)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
				AnnOwnerRef:  fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
			},
			Labels: map[string]string{
				CDILabelKey:       CDILabelValue, //filtered by the podInformer
				CDIComponentLabel: ClonerSourcePodName,
				// this label is used when searching for a pvc's cloner source pod.
				CloneUniqueID: pvcUID + "-source-pod",
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []v1.Container{
				{
					Name:            common.ClonerSourcePodName,
					Image:           "test/mycloneimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					Env: []v1.EnvVar{
						{
							Name:  "CLIENT_KEY",
							Value: string(testUploadServerClientKeySecret.Data["tls.key"]),
						},
						{
							Name:  "CLIENT_CERT",
							Value: string(testUploadServerClientKeySecret.Data["tls.crt"]),
						},
						{
							Name:  "SERVER_CA_CERT",
							Value: string(testUploadServerClientKeySecret.Data["ca.crt"]),
						},
						{
							Name:  "UPLOAD_URL",
							Value: GetUploadServerURL(pvc.Namespace, pvc.Name),
						},
						{
							Name:  common.OwnerUID,
							Value: pvcUID,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes: []v1.Volume{
				{
					Name: DataVolName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	var volumeMode v1.PersistentVolumeMode
	var addVars []v1.EnvVar

	if pvc.Spec.VolumeMode != nil {
		volumeMode = *pvc.Spec.VolumeMode
	} else {
		volumeMode = v1.PersistentVolumeFilesystem
	}

	if volumeMode == v1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		addVars = []v1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "block",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.ImporterWriteBlockPath,
			},
		}
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser: &[]int64{0}[0],
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
		addVars = []v1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "filesystem",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.ClonerMountPath,
			},
		}
	}

	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, addVars...)

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
	pod := createUploadClonePod(pvc)
	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		Name: ScratchVolName,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name + "-scratch",
				ReadOnly:  false,
			},
		},
	})
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
		Name:      ScratchVolName,
		MountPath: "/scratch",
	})
	return pod
}

func createUploadClonePod(pvc *v1.PersistentVolumeClaim) *v1.Pod {
	name := "cdi-upload-" + pvc.Name
	secretName := name + "-server-tls"
	requestImageSize, _ := getRequestedImageSize(pvc)

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
				MakePVCOwnerReference(pvc),
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
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
						{
							Name:  common.UploadImageSize,
							Value: requestImageSize,
						},
					},
					Args: []string{"-v=" + "5"},
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 8080,
								},
							},
						},
						InitialDelaySeconds: 2,
						PeriodSeconds:       5,
					},
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

func createCDIConfig(name string) *cdiv1.CDIConfig {
	return createCDIConfigWithStorageClass(name, "")
}

func createCDIConfigWithStorageClass(name string, storageClass string) *cdiv1.CDIConfig {
	return &cdiv1.CDIConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
		Status: cdiv1.CDIConfigStatus{
			ScratchSpaceStorageClass: storageClass,
		},
	}
}

func createStorageClass(name string, annotations map[string]string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
}

func createStorageClassWithProvisioner(name string, annotations map[string]string, provisioner string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		Provisioner: provisioner,
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
}
func createSnapshotClass(name string, annotations map[string]string, snapshotter string) *snapshotv1.VolumeSnapshotClass {
	return &snapshotv1.VolumeSnapshotClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumeSnapshotClass",
			APIVersion: snapshotv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Snapshotter: snapshotter,
	}
}

func createVolumeSnapshotContentCrd() *apiextensionsv1beta1.CustomResourceDefinition {
	return &apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: apiextensionsv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crdv1.VolumeSnapshotContentResourcePlural + "." + crdv1.GroupName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   crdv1.GroupName,
			Version: crdv1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.ClusterScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: crdv1.VolumeSnapshotContentResourcePlural,
				Kind:   reflect.TypeOf(crdv1.VolumeSnapshotContent{}).Name(),
			},
		},
	}
}

func createVolumeSnapshotClassCrd() *apiextensionsv1beta1.CustomResourceDefinition {
	return &apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: apiextensionsv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crdv1.VolumeSnapshotClassResourcePlural + "." + crdv1.GroupName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   crdv1.GroupName,
			Version: crdv1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.ClusterScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: crdv1.VolumeSnapshotClassResourcePlural,
				Kind:   reflect.TypeOf(crdv1.VolumeSnapshotClass{}).Name(),
			},
		},
	}
}

func createVolumeSnapshotCrd() *apiextensionsv1beta1.CustomResourceDefinition {
	return &apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: apiextensionsv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crdv1.VolumeSnapshotResourcePlural + "." + crdv1.GroupName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   crdv1.GroupName,
			Version: crdv1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: crdv1.VolumeSnapshotResourcePlural,
				Kind:   reflect.TypeOf(crdv1.VolumeSnapshot{}).Name(),
			},
		},
	}
}
