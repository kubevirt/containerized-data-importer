package controller

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	bootstrapapi "k8s.io/client-go/tools/bootstrap/token/api"
	"k8s.io/client-go/tools/cache"
	k8stesting "k8s.io/client-go/tools/cache/testing"
	"k8s.io/client-go/util/workqueue"

	. "kubevirt.io/containerized-data-importer/pkg/common"
	expectations "kubevirt.io/containerized-data-importer/pkg/expectations"
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	//create staging pvc and pod
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	podWithCdiAnno := createPod(pvcWithEndPointAnno, DataVolName)

	//run the informers
	c, _, pod, err := createImportController(pvcWithEndPointAnno, podWithCdiAnno, "default")
	if err != nil {
		t.Errorf("Controller.podFromKey() failed to initialize fake controller error = %v", err)
		return
	}

	type args struct {
		key interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.Pod
		wantErr bool
	}{
		{
			name:    "expect to get pod object from key",
			args:    args{fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)},
			want:    pod,
			wantErr: false,
		},
		{
			name:    "expect to not get pod object from key",
			args:    args{fmt.Sprintf("%s/%s", "myns", pod.Name)},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.objFromKey(tt.args.informer, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.objFromKey() error = %v, wantErr %v  myKey = %v", err, tt.wantErr, tt.args.key)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.objFromKey() = %v, want %v", got, tt.want)
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
			gotOk := checkPVC(tt.pvc)
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
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, map[string]string{AnnCreatedBy: "cdi"}, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}},
			want:    createPvc("testPVC1", "default", map[string]string{AnnCreatedBy: "cdi"}, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}),
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
			args:    args{k8sfake.NewSimpleClientset(pvcNoAnno), pvcNoAnno, nil, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}},
			want:    createPvc("testPVC1", "default", nil, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}),
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
	pvc := createPvc("testPVC", "default", nil, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE})
	pvcNoLbl := createPvc("testPVC2", "default", nil, nil)

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "pvc does have expected label and expected value",
			args: args{pvc, CDI_LABEL_KEY, CDI_LABEL_VALUE},
			want: true,
		},
		{
			name: "pvc does not have expected label",
			args: args{pvc, AnnCreatedBy, "yes"},
			want: false,
		},
		{
			name: "pvc does have expected label but does not have expected value",
			args: args{pvc, CDI_LABEL_KEY, "something"},
			want: false,
		},
		{
			name: "pvc does not have any labels",
			args: args{pvcNoLbl, CDI_LABEL_KEY, CDI_LABEL_VALUE},
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
		ep         string
		secretName string
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
			args:    args{k8sfake.NewSimpleClientset(pvc), "test/image", "-v=5", "Always", "", "", pvc},
			want:    MakeImporterPodSpec("test/image", "-v=5", "Always", "", "", pvc),
			wantErr: false,
		},
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
	// create PVC
	pvc := createPvc("testPVC2", "default", nil, nil)

	pod := createPod(pvc, DataVolName)

	tests := []struct {
		name    string
		args    args
		wantPod *v1.Pod
	}{
		{
			name:    "expect pod to be created",
			args:    args{"test/myimage", "5", "Always", "", "", pvc},
			wantPod: pod,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeImporterPodSpec(tt.args.image, tt.args.verbose, tt.args.pullPolicy, tt.args.ep, tt.args.secret, tt.args.pvc)

			if !reflect.DeepEqual(got, tt.wantPod) {
				t.Errorf("MakeImporterPodSpec() =\n%v\n, want\n%v", got, tt.wantPod)
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
		{
			name: "env should match",
			args: args{"myendpoint", "mysecret"},
			want: createEnv("myendpoint", "mysecret"),
		},
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
		{
			name: "use different key for map1 and map2 expect both maps to be returned",
			args: args{map[string]string{AnnImportPod: "mypod"}, map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}},
			want: map[string]string{AnnImportPod: "mypod", CDI_LABEL_KEY: CDI_LABEL_VALUE},
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

func createPodWithName(pvc *v1.PersistentVolumeClaim, dvname string) *v1.Pod {
	pod := createPod(pvc, dvname)
	pod.Name = fmt.Sprintf("%sgeneratedname", pod.GenerateName)
	return pod
}

func createPod(pvc *v1.PersistentVolumeClaim, dvname string) *v1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s-", IMPORTER_PODNAME, pvc.Name)

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
				CDI_LABEL_KEY: CDI_LABEL_VALUE,
				AnnImportPVC:  pvc.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
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
					Name:            IMPORTER_PODNAME,
					Image:           "test/myimage",
					ImagePullPolicy: v1.PullPolicy("Always"),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: IMPORTER_DATA_DIR,
						},
					},
					Args: []string{"-v=5"},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
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
	pod.Spec.Containers[0].Env = []v1.EnvVar{
		{
			Name:  IMPORTER_ENDPOINT,
			Value: ep,
		},
	}
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

func createEnv(endpoint, secret string) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  IMPORTER_ENDPOINT,
			Value: endpoint,
		},
	}
	if secret != "" {
		env = append(env, v1.EnvVar{
			Name: IMPORTER_ACCESS_KEY_ID,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
					},
					Key: KeyAccess,
				},
			},
		}, v1.EnvVar{
			Name: IMPORTER_SECRET_KEY,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
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

	c := &ImportController{
		clientset:       myclient,
		queue:           pvcQueue,
		pvcInformer:     pvcInformer.Informer(),
		podInformer:     podInformer.Informer(),
		importerImage:   "test/image",
		pullPolicy:      "Always",
		verbose:         "-v=5",
		podExpectations: expectations.NewUIDTrackingControllerExpectations(expectations.NewControllerExpectations()),
	}
	return c, pvc, pod, nil
}

func createCloneController(pvcSpec *v1.PersistentVolumeClaim, podSpec *v1.Pod, ns string) (*CloneController, *v1.PersistentVolumeClaim, *v1.Pod, error) {
	//Set up environment
	myclient := k8sfake.NewSimpleClientset()
	pvcSource := k8stesting.NewFakePVCControllerSource()
	podSource := k8stesting.NewFakeControllerSource()

	//create staging pvc and pod
	pvc, err := myclient.CoreV1().PersistentVolumeClaims(ns).Create(pvcSpec)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pvc error = %v", err)
	}
	pvcSource.Add(pvc)

	pod, err := myclient.CoreV1().Pods(ns).Create(podSpec)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pod error = %v", err)
	}
	podSource.Add(pod)

	// create informers and queue
	pvcInformer := cache.NewSharedIndexInformer(pvcSource, pvc, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podInformer := cache.NewSharedIndexInformer(podSource, pod, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	pvcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	podQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	pvcQueue.Add(pvc)
	podQueue.Add(pod)

	//run the informers
	stop := make(chan struct{})
	go pvcInformer.Run(stop)
	go podInformer.Run(stop)
	cache.WaitForCacheSync(stop, podInformer.HasSynced)
	cache.WaitForCacheSync(stop, pvcInformer.HasSynced)
	defer close(stop)

	c := &CloneController{
		clientset:   myclient,
		pvcQueue:    pvcQueue,
		podQueue:    podQueue,
		pvcInformer: pvcInformer,
		podInformer: podInformer,
		cloneImage:  CLONER_DEFAULT_IMAGE,
		pullPolicy:  "Always",
		verbose:     "-v=5",
	}
	return c, pvc, pod, nil
}

func createImportControllerMultiObject(pvcSpecs []*v1.PersistentVolumeClaim, podSpecs []*v1.Pod, nspaces []string) (*ImportController, []*v1.PersistentVolumeClaim, []*v1.Pod, error) {
	//Set up environment
	myclient := k8sfake.NewSimpleClientset()
	pvcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var pvcs []*v1.PersistentVolumeClaim
	var pods []*v1.Pod

	k8sI := kubeinformers.NewSharedInformerFactory(myclient, noResyncPeriodFunc())

	pvcInformer := k8sI.Core().V1().PersistentVolumeClaims()
	podInformer := k8sI.Core().V1().Pods()

	//create staging pvc and pod
	for i, v := range pvcSpecs {
		pvc, err := myclient.CoreV1().PersistentVolumeClaims(v.Namespace).Create(v)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pvc index = %v value = %v error = %v", i, v, err)
		}
		k8sI.Core().V1().PersistentVolumeClaims().Informer().GetIndexer().Add(pvc)
		pvcQueue.Add(pvc)
		pvcs = append(pvcs, pvc)
	}

	for i, v := range podSpecs {
		pod, err := myclient.CoreV1().Pods(nspaces[i]).Create(v)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("createImportController: failed to initialize and create pod index = %v value = %v error = %v, Pod Name = %v, Length Specs = %v", i, v, err, v.Name, len(podSpecs))
		}
		k8sI.Core().V1().Pods().Informer().GetIndexer().Add(pod)
		pods = append(pods, pod)
	}

	//run the informers
	stop := make(chan struct{})
	go pvcInformer.Informer().Run(stop)
	cache.WaitForCacheSync(stop, pvcInformer.Informer().HasSynced)
	go podInformer.Informer().Run(stop)
	cache.WaitForCacheSync(stop, podInformer.Informer().HasSynced)
	defer close(stop)

	c := &ImportController{
		clientset:       myclient,
		queue:           pvcQueue,
		pvcInformer:     pvcInformer.Informer(),
		podInformer:     podInformer.Informer(),
		importerImage:   "test/image",
		pullPolicy:      "Always",
		verbose:         "-v=5",
		podExpectations: expectations.NewUIDTrackingControllerExpectations(expectations.NewControllerExpectations()),
	}
	return c, pvcs, pods, nil
}

func createCloneControllerMultiObject(pvcSpecs []*v1.PersistentVolumeClaim, podSpecs []*v1.Pod, nspaces []string) (*CloneController, []*v1.PersistentVolumeClaim, []*v1.Pod, error) {
	//Set up environment
	myclient := k8sfake.NewSimpleClientset()
	pvcSource := k8stesting.NewFakePVCControllerSource()
	podSource := k8stesting.NewFakeControllerSource()
	pvcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	podQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var pvcInformer cache.SharedIndexInformer
	var podInformer cache.SharedIndexInformer
	var pvcs []*v1.PersistentVolumeClaim
	var pods []*v1.Pod

	//create staging pvc and pod
	for i, v := range pvcSpecs {
		pvc, err := myclient.CoreV1().PersistentVolumeClaims(v.Namespace).Create(v)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("createCloneController: failed to initialize and create pvc index = %v value = %v error = %v", i, v, err)
		}
		pvcSource.Add(pvc)
		pvcInformer = cache.NewSharedIndexInformer(pvcSource, pvc, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		pvcQueue.Add(pvc)
		pvcs = append(pvcs, pvc)
	}

	for i, v := range podSpecs {
		pod, err := myclient.CoreV1().Pods(nspaces[i]).Create(v)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("createCloneController: failed to initialize and create pod index = %v value = %v error = %v, Pod Name = %v, Length Specs = %v", i, v, err, v.Name, len(podSpecs))
		}
		podSource.Add(pod)
		podInformer = cache.NewSharedIndexInformer(podSource, pod, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		podQueue.Add(pod)
		pods = append(pods, pod)
	}

	//run the informers
	stop := make(chan struct{})
	if pvcInformer != nil {
		go pvcInformer.Run(stop)
		cache.WaitForCacheSync(stop, pvcInformer.HasSynced)
	}
	if podInformer != nil {
		go podInformer.Run(stop)
		cache.WaitForCacheSync(stop, podInformer.HasSynced)
	}
	defer close(stop)

	c := &CloneController{
		clientset:   myclient,
		pvcQueue:    pvcQueue,
		podQueue:    podQueue,
		pvcInformer: pvcInformer,
		podInformer: podInformer,
		cloneImage:  CLONER_DEFAULT_IMAGE,
		pullPolicy:  "Always",
		verbose:     "-v=5",
	}
	return c, pvcs, pods, nil
}
