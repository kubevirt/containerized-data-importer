package controller

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

var (
	utilLog = logf.Log.WithName("util-test")
)

var _ = Describe("getVolumeMode", func() {
	pvcVolumeModeBlock := createBlockPvc("testPVCVolumeModeBlock", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystem := CreatePvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystemDefault := CreatePvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult corev1.PersistentVolumeMode) {
		result := GetVolumeMode(pvc)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return block if pvc has block volume mode", pvcVolumeModeBlock, corev1.PersistentVolumeBlock),
		table.Entry("return file system if pvc has filesystem mode", pvcVolumeModeFilesystem, corev1.PersistentVolumeFilesystem),
		table.Entry("return file system if pvc has no mode defined", pvcVolumeModeFilesystemDefault, corev1.PersistentVolumeFilesystem),
	)
})

var _ = Describe("CheckIfLabelExists", func() {
	pvc := CreatePvc("testPVC", "default", nil, map[string]string{common.CDILabelKey: common.CDILabelValue})
	pvcNoLbl := CreatePvc("testPVC2", "default", nil, nil)

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

var _ = Describe("GetScratchPVCStorageClass", func() {
	It("Should return default storage class from status in CDIConfig", func() {
		storageClassName := "test3"
		client := CreateClient(CreateStorageClass("test1", nil), CreateStorageClass("test2", nil), CreateStorageClass("test3", map[string]string{
			AnnDefaultStorageClass: "true",
		}), createCDIConfigWithStorageClass(common.ConfigName, storageClassName))
		pvc := CreatePvc("test", "test", nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return default storage class from status in CDIConfig", func() {
		storageClassName := "test1"
		config := createCDIConfigWithStorageClass(common.ConfigName, storageClassName)
		config.Spec.ScratchSpaceStorageClass = &storageClassName
		client := CreateClient(CreateStorageClass("test1", nil), CreateStorageClass("test2", nil), CreateStorageClass("test3", map[string]string{
			AnnDefaultStorageClass: "true",
		}), config)
		pvc := CreatePvc("test", "test", nil, nil)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return storage class from pvc", func() {
		storageClassName := "storageClass"
		client := CreateClient(createCDIConfigWithStorageClass(common.ConfigName, ""))
		pvc := CreatePvcInStorageClass("test", "test", &storageClassName, nil, nil, v1.ClaimBound)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(storageClassName))
	})

	It("Should return blank if CDIConfig not there", func() {
		storageClassName := "storageClass"
		client := CreateClient()
		pvc := CreatePvcInStorageClass("test", "test", &storageClassName, nil, nil, v1.ClaimBound)
		Expect(GetScratchPvcStorageClass(client, pvc)).To(Equal(""))
	})
})

var _ = Describe("GetWorkloadNodePlacement", func() {
	It("Should return a node placement, with one CDI CR", func() {
		client := CreateClient(createCDIWithWorkload("cdi-test", "1111-1111"))
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
	})

	It("Should return an err with > 1 CDI CR", func() {
		client := CreateClient(createCDIWithWorkload("cdi-test", "1111-1111"), createCDIWithWorkload("cdi-test2", "2222-2222"))
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
	})

	It("Should return a node placement, with one active CDI CR one error", func() {
		errCR := createCDIWithWorkload("cdi-test2", "2222-2222")
		errCR.Status.Phase = sdkapi.PhaseError
		client := CreateClient(createCDIWithWorkload("cdi-test", "1111-1111"), errCR)
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
	})
})

var _ = Describe("DecodePublicKey", func() {
	It("Should decode an encoded key", func() {
		bytes, err := cert.EncodePublicKeyPEM(&GetAPIServerKey().PublicKey)
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

var _ = Describe("setAnnotationsFromPod", func() {
	It("Should follow pod container status, running", func() {
		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("true"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Pod is running"))
	})

	It("Should follow pod container status, completed", func() {
		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
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
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("The container completed"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Completed"))
	})

	It("Should follow pod container status, pending", func() {
		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
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
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("container is waiting"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Pending"))
	})

	It("Should set preallocation status", func() {
		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: "container completed, " + common.PreallocationApplied,
							Reason:  "Completed",
						},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
		Expect(result[AnnPreallocationApplied]).To(Equal("true"))
	})
})

var _ = Describe("GetPreallocation", func() {
	It("Should return preallocation for DataVolume if specified", func() {
		client := CreateClient()
		dv := createDataVolumeWithPreallocation("test-dv", "test-ns", true)
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeTrue())

		dv = createDataVolumeWithPreallocation("test-dv", "test-ns", false)
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())

		// global: true, data volume overrides to false
		client = CreateClient(createCDIConfigWithGlobalPreallocation(true))
		dv = createDataVolumeWithStorageClassPreallocation("test-dv", "test-ns", "test-class", false)
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})

	It("Should return global preallocation setting if not defined in DV or SC", func() {
		client := CreateClient(createCDIConfigWithGlobalPreallocation(true))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeTrue())

		client = CreateClient(createCDIConfigWithGlobalPreallocation(false))
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})

	It("Should be false when niether DV nor Config defines preallocation", func() {
		client := CreateClient(createCDIConfig("test"))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})
})

var _ = Describe("ValidateClone", func() {
	sourcePvc := CreatePvc("testPVC", "default", map[string]string{}, nil)
	blockVM := corev1.PersistentVolumeBlock
	fsVM := corev1.PersistentVolumeFilesystem

	It("Should reject the clone if source and target have different content types", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		dvSpec := &cdiv1.DataVolumeSpec{ContentType: cdiv1.DataVolumeArchive}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(
			fmt.Sprintf("Source contentType (%s) and target contentType (%s) do not match", cdiv1.DataVolumeKubeVirt, cdiv1.DataVolumeArchive)))
	})

	It("Should reject the clone if the target has an incompatible size and the source PVC is using block volumeMode (Storage API)", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		sourcePvc.Spec.VolumeMode = &blockVM
		storageSpec := &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Mi"), // Less than the source's one (1Gi)
				},
			},
		}
		dvSpec := &cdiv1.DataVolumeSpec{Storage: storageSpec}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("target resources requests storage size is smaller than the source"))
	})

	It("Should validate the clone when source PVC is using fs volumeMode, even if the target has an incompatible size (Storage API)", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		sourcePvc.Spec.VolumeMode = &fsVM
		storageSpec := &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Mi"), // Less than the source's one (1Gi)
				},
			},
		}
		dvSpec := &cdiv1.DataVolumeSpec{Storage: storageSpec}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should validate the clone if target's size is empty, even when the source uses block volumeMode (Storage API)", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		sourcePvc.Spec.VolumeMode = &blockVM
		storageSpec := &cdiv1.StorageSpec{}
		dvSpec := &cdiv1.DataVolumeSpec{Storage: storageSpec}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should reject the clone when the target has an incompatible size (PVC API)", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Mi"), // Less than the source's one (1Gi)
				},
			},
		}
		dvSpec := &cdiv1.DataVolumeSpec{PVC: pvcSpec}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("target resources requests storage size is smaller than the source"))

	})

	It("Should validate the clone when both sizes are compatible (PVC API)", func() {
		sourcePvc.Annotations[AnnContentType] = string(cdiv1.DataVolumeKubeVirt)
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"), // Same as the source's
				},
			},
		}
		dvSpec := &cdiv1.DataVolumeSpec{PVC: pvcSpec}

		err := ValidateClone(sourcePvc, dvSpec)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("HandleFailedPod", func() {
	pvc := CreatePvc("test-pvc", "test-ns", nil, nil)
	podName := "test-pod"

	table.DescribeTable("Should record an event with pertinent information if pod fails due to", func(errMsg, reason string) {
		// Create a mock reconciler to record the events
		cl := CreateClient(pvc)
		rec := record.NewFakeRecorder(10)
		err := HandleFailedPod(errors.New(errMsg), podName, pvc, rec, cl)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(errMsg))

		By("Checking error event recorded")
		msg := fmt.Sprintf(MessageErrStartingPod, podName)
		event := <-rec.Events
		Expect(event).To(ContainSubstring(msg))
		Expect(event).To(ContainSubstring(reason))
	},
		table.Entry("generic error", "test error", ErrStartingPod),
		table.Entry("quota error", "exceeded quota:", ErrExceededQuota),
	)

	It("Should return a different error if the PVC isn't able to update, but still record the event", func() {
		// Create a mock reconciler to record the events
		cl := CreateClient()
		rec := record.NewFakeRecorder(10)
		err := HandleFailedPod(errors.New("test error"), podName, pvc, rec, cl)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("persistentvolumeclaims \"test-pvc\" not found"))

		By("Checking error event recorded")
		msg := fmt.Sprintf(MessageErrStartingPod, podName)
		event := <-rec.Events
		Expect(event).To(ContainSubstring(msg))
		Expect(event).To(ContainSubstring(ErrStartingPod))
	})
})

var _ = Describe("check PVC", func() {
	pvcNoAnno := CreatePvc("testPvcNoAnno", "default", nil, nil)
	pvcWithEndPointAnno := CreatePvc("testPvcWithEndPointAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pvcWithCloneRequestAnno := CreatePvc("testPvcWithCloneRequestAnno", "default", map[string]string{AnnCloneRequest: "source-ns/golden-pvc"}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, annotation string, expectedResult bool) {
		result := checkPVC(pvc, annotation, utilLog)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("return false if no annotation provided", pvcNoAnno, AnnEndpoint, false),
		table.Entry("return true if annotation provided that matches test http", pvcWithEndPointAnno, AnnEndpoint, true),
		table.Entry("return true if annotation provided that matches test clone", pvcWithCloneRequestAnno, AnnCloneRequest, true),
	)
})

func addOwnerToDV(dv *cdiv1.DataVolume, ownerName string) {
	dv.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "VirtualMachine",
			Name:       ownerName,
		},
	}
}

func createDataVolumeWithStorageClass(name, ns, storageClassName string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{},
			PVC: &corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
			},
		},
	}
}

func createDataVolume(name, ns string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{},
		},
	}
}

func createDataVolumeWithPreallocation(name, ns string, preallocation bool) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source:        &cdiv1.DataVolumeSource{},
			Preallocation: &preallocation,
		},
	}
}

func createDataVolumeWithStorageClassPreallocation(name, ns, storageClassName string, preallocation bool) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source:        &cdiv1.DataVolumeSource{},
			Preallocation: &preallocation,
			PVC: &corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
			},
		},
	}
}

func createScratchPvc(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, storageClassName string) *v1.PersistentVolumeClaim {
	t := true
	labels := map[string]string{
		"cdi-controller": pod.Name,
		"app":            "containerized-data-importer",
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

func createCDIConfigWithGlobalPreallocation(globalPreallocation bool) *cdiv1.CDIConfig {
	return &cdiv1.CDIConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDIConfig",
			APIVersion: "cdi.kubevirt.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ConfigName,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
		Status: cdiv1.CDIConfigStatus{
			Preallocation: globalPreallocation,
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
		Status: cdiv1.CDIStatus{
			Status: sdkapi.Status{
				Phase: sdkapi.PhaseDeployed,
			},
		},
	}
}
