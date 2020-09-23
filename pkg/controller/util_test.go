package controller

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
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
	pvc := createPvc("testPVC", "default", nil, map[string]string{common.CDILabelKey: common.CDILabelValue})
	pvcNoLbl := createPvc("testPVC2", "default", nil, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, key, value string, expectedResult bool) {
		result := checkIfLabelExists(pvc, key, value)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return true if label with value exists", pvc, common.CDILabelKey, common.CDILabelValue, true),
		table.Entry("return false if label with value does not exists", pvc, AnnCreatedBy, "yes", false),
		table.Entry("return false if label exists, but value doesn't match", pvc, common.CDILabelKey, "something", false),
		table.Entry("return false if pvc has no labels", pvcNoLbl, common.CDILabelKey, common.CDILabelValue, false),
		table.Entry("return false if pvc has no labels and check key and value are blank", pvcNoLbl, "", "", false),
	)
})

var _ = Describe("addToMap", func() {
	table.DescribeTable("should", func(m1, m2, expectedResult map[string]string) {
		result := addToMap(m1, m2)
		Expect(reflect.DeepEqual(result, expectedResult)).To(BeTrue())
	},
		table.Entry("use different key for map1 and map2 expect both maps to be returned",
			map[string]string{AnnImportPod: "mypod"}, map[string]string{common.CDILabelKey: common.CDILabelValue}, map[string]string{AnnImportPod: "mypod", common.CDILabelKey: common.CDILabelValue}),
		table.Entry("use same key for map1 and map2 expect map2 to be returned",
			map[string]string{AnnImportPod: "mypod"}, map[string]string{AnnImportPod: "map2pod"}, map[string]string{AnnImportPod: "map2pod"}),
		table.Entry("pass in empty map1 and map2 expect empty map",
			nil, nil, map[string]string{}),
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
		pvc := createPvcInStorageClass("test", "test", &storageClassName, nil, nil, v1.ClaimBound)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return blank if CDIConfig not there", func() {
		storageClassName := "storageClass"
		client := createClient()
		pvc := createPvcInStorageClass("test", "test", &storageClassName, nil, nil, v1.ClaimBound)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(""))
	})
})

var _ = Describe("GetWorkloadNodePlacement", func() {
	It("Should return a node placement, with one CDI CR", func() {
		client := createClient(createCDIWithWorkload("cdi-test", "1111-1111"))
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
	})

	It("Should return an err with > 1 CDI CR", func() {
		client := createClient(createCDIWithWorkload("cdi-test", "1111-1111"), createCDIWithWorkload("cdi-test2", "2222-2222"))
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
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
		setConditionFromPodWithPrefix(result, AnnRunningCondition, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("true"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Pod is running"))
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
							Reason:  "Completed",
						},
					},
				},
			},
		}
		setConditionFromPodWithPrefix(result, AnnRunningCondition, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("The container completed"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Completed"))
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
							Reason:  "Pending",
						},
					},
				},
			},
		}
		setConditionFromPodWithPrefix(result, AnnRunningCondition, testPod)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("container is waiting"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Pending"))
	})
})

func createBlockPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	pvcDef := createPvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimBound)
	volumeMode := v1.PersistentVolumeBlock
	pvcDef.Spec.VolumeMode = &volumeMode
	return pvcDef
}

func createPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return createPvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimBound)
}

func createPendingPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return createPvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimPending)
}

func createPvcInStorageClass(name, ns string, storageClassName *string, annotations, labels map[string]string, phase v1.PersistentVolumeClaimPhase) *v1.PersistentVolumeClaim {
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
		Status: v1.PersistentVolumeClaimStatus{
			Phase: phase,
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
			APIVersion: "cdi.kubevirt.io/v1beta1",
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

func createStorageClassWithBindingMode(name string, annotations map[string]string, bindingMode storagev1.VolumeBindingMode) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		VolumeBindingMode: &bindingMode,
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
		Driver: snapshotter,
	}
}

func createVolumeSnapshotContentCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshotcontents"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.ClusterScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshotContent{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
			},
		},
	}
}

func createVolumeSnapshotClassCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshotclasses"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.ClusterScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshotClass{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
			},
		},
	}
}

func createVolumeSnapshotCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshots"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.NamespaceScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshot{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
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

func podUsingPVC(pvc *corev1.PersistentVolumeClaim, readOnly bool) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pvc.Namespace,
			Name:      pvc.Name + "-pod",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  readOnly,
						},
					},
				},
			},
		},
	}
}

func createCDIWithWorkload(name, uid string) *cdiv1.CDI {
	return &cdiv1.CDI{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(uid),
		},
		Spec: cdiv1.CDISpec{
			Workloads: api.NodePlacement{},
		},
	}
}
