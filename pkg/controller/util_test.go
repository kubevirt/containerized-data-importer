package controller

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crdv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	utilLog = logf.Log.WithName("util-test")
)

var _ = Describe("check PVC", func() {
	pvcNoAnno := createPvc("testPvcNoAnno", "default", nil, nil)
	pvcWithEndPointAnno := createPvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pvcWithCloneRequestAnno := createPvc("testPvcWithCloneRequestAnno", "default", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, annotation string, expectedResult bool) {
		result := checkPVC(pvc, annotation, utilLog)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return false if no annotation provided", pvcNoAnno, AnnEndpoint, false),
		table.Entry("return true if annotation provided that matches test http", pvcWithEndPointAnno, AnnEndpoint, true),
		table.Entry("return true if annotation provided that matches test clone", pvcWithCloneRequestAnno, AnnCloneRequest, true),
	)
})

var _ = Describe("getRequestedImageSize", func() {
	It("Should return 1G if 1G provided", func() {
		result, err := getRequestedImageSize(createPvc("testPVC", "default", nil, nil))
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("1G"))
	})

	It("Should return error and blank if no size provided", func() {
		result, err := getRequestedImageSize(createPvcNoSize("testPVC", "default", nil, nil))
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("getVolumeMode", func() {
	pvcVolumeModeBlock := createBlockPvc("testPVCVolumeModeBlock", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystem := createPvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystemDefault := createPvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult corev1.PersistentVolumeMode) {
		result := getVolumeMode(pvc)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return block if pvc has block volume mode", pvcVolumeModeBlock, corev1.PersistentVolumeBlock),
		table.Entry("return file system if pvc has filesystem mode", pvcVolumeModeFilesystem, corev1.PersistentVolumeFilesystem),
		table.Entry("return file system if pvc has no mode defined", pvcVolumeModeFilesystemDefault, corev1.PersistentVolumeFilesystem),
	)
})

var _ = Describe("checkIfLabelExists", func() {
	pvc := createPvc("testPVC", "default", nil, map[string]string{CDILabelKey: CDILabelValue})
	pvcNoLbl := createPvc("testPVC2", "default", nil, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, key, value string, expectedResult bool) {
		result := checkIfLabelExists(pvc, key, value)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return true if label with value exists", pvc, CDILabelKey, CDILabelValue, true),
		table.Entry("return false if label with value does not exists", pvc, AnnCreatedBy, "yes", false),
		table.Entry("return false if label exists, but value doesn't match", pvc, CDILabelKey, "something", false),
		table.Entry("return false if pvc has no labels", pvcNoLbl, CDILabelKey, CDILabelValue, false),
		table.Entry("return false if pvc has no labels and check key and value are blank", pvcNoLbl, "", "", false),
	)
})

var _ = Describe("addToMap", func() {
	table.DescribeTable("should", func(m1, m2, expectedResult map[string]string) {
		result := addToMap(m1, m2)
		Expect(reflect.DeepEqual(result, expectedResult)).To(BeTrue())
	},
		table.Entry("use different key for map1 and map2 expect both maps to be returned", map[string]string{AnnImportPod: "mypod"}, map[string]string{CDILabelKey: CDILabelValue}, map[string]string{AnnImportPod: "mypod", CDILabelKey: CDILabelValue}),
		table.Entry("use same key for map1 and map2 expect map2 to be returned", map[string]string{AnnImportPod: "mypod"}, map[string]string{AnnImportPod: "map2pod"}, map[string]string{AnnImportPod: "map2pod"}),
		table.Entry("pass in empty map1 and map2 expect empty map", nil, nil, map[string]string{}),
	)
})

var _ = Describe("GetScratchPVCStorageClass", func() {
	It("Should return default storage class from status in CDIConfig", func() {
		storageClassName := "test3"
		client := createClient(createStorageClass("test1", nil), createStorageClass("test2", nil), createStorageClass("test3", map[string]string{
			AnnDefaultStorageClass: "true",
		}), createCDIConfigWithStorageClass(common.ConfigName, storageClassName))
		pvc := createPvc("test", "test", nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return default storage class from status in CDIConfig", func() {
		storageClassName := "test1"
		config := createCDIConfigWithStorageClass(common.ConfigName, storageClassName)
		config.Spec.ScratchSpaceStorageClass = &storageClassName
		client := createClient(createStorageClass("test1", nil), createStorageClass("test2", nil), createStorageClass("test3", map[string]string{
			AnnDefaultStorageClass: "true",
		}), config)
		pvc := createPvc("test", "test", nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return storage class from pvc", func() {
		storageClassName := "storageClass"
		client := createClient(createCDIConfigWithStorageClass(common.ConfigName, ""))
		pvc := createPvcInStorageClass("test", "test", &storageClassName, nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return blank if CDIConfig not there", func() {
		storageClassName := "storageClass"
		client := createClient()
		pvc := createPvcInStorageClass("test", "test", &storageClassName, nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(""))
	})
})

func createClient(objs ...runtime.Object) client.Client {
	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	// Create a fake client to mock API calls.
	return fake.NewFakeClientWithScheme(s, objs...)
}

var _ = Describe("DecodePublicKey", func() {
	It("Should decode an encoded key", func() {
		bytes, err := cert.EncodePublicKeyPEM(&getAPIServerKey().PublicKey)
		Expect(err).ToNot(HaveOccurred())
		_, err = DecodePublicKey(bytes)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should fail decoding invalid bytes", func() {
		bytes := []byte{}
		_, err := DecodePublicKey(bytes)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("setConditionFromPod", func() {
	It("Should follow pod container status, running", func() {
		result := make(map[string]string)
		testPod := createImporterTestPod(createPvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		}
		setConditionFromPod(result, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("true"))
		Expect(result[AnnRunningConditionMessage]).To(Equal(""))
	})

	It("Should follow pod container status, completed", func() {
		result := make(map[string]string)
		testPod := createImporterTestPod(createPvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: "The container completed",
						},
					},
				},
			},
		}
		setConditionFromPod(result, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("The container completed"))
	})

	It("Should follow pod container status, pending", func() {
		result := make(map[string]string)
		testPod := createImporterTestPod(createPvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{
							Message: "container is waiting",
						},
					},
				},
			},
		}
		setConditionFromPod(result, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("container is waiting"))
	})
})

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
			UID:         types.UID(ns + "-" + name),
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
	t := true
	labels := map[string]string{
		"cdi-controller": pod.Name,
		"app":            "containerized-data-importer",
		LabelImportPvc:   pvc.Name,
	}
	annotations := make(map[string]string)
	if len(pvc.GetAnnotations()) > 0 {
		for k, v := range pvc.GetAnnotations() {
			if strings.Contains(k, common.KubeVirtAnnKey) && !strings.Contains(k, common.CDIAnnKey) {
				annotations[k] = v
			}
		}
	}

	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvc.Name + "-scratch",
			Namespace:   pvc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               pod.Name,
					UID:                pod.GetUID(),
					Controller:         &t,
					BlockOwnerDeletion: &t,
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

func getPvcKey(pvc *corev1.PersistentVolumeClaim, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pvc)
	if err != nil {
		t.Errorf("Unexpected error getting key for pvc %v: %v", pvc.Name, err)
		return ""
	}
	return key
}

func createCDIConfig(name string) *cdiv1.CDIConfig {
	return createCDIConfigWithStorageClass(name, "")
}

func createCDIConfigWithStorageClass(name string, storageClass string) *cdiv1.CDIConfig {
	return &cdiv1.CDIConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDIConfig",
			APIVersion: "cdi.kubevirt.io/v1alpha1",
		},
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

func createDefaultPodResourceRequirements(limitCPUValue int64, limitMemoryValue int64, requestCPUValue int64, requestMemoryValue int64) *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    *resource.NewQuantity(limitCPUValue, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(limitMemoryValue, resource.DecimalSI)},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    *resource.NewQuantity(requestCPUValue, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(requestMemoryValue, resource.DecimalSI)},
	}
}
