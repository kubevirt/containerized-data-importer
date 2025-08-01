package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

var (
	utilLog = logf.Log.WithName("util-test")
)

var _ = Describe("getVolumeMode", func() {
	pvcVolumeModeBlock := createBlockPvc("testPVCVolumeModeBlock", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystem := CreatePvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)
	pvcVolumeModeFilesystemDefault := CreatePvc("testPVCVolumeModeFS", "default", map[string]string{AnnSource: SourceHTTP}, nil)

	DescribeTable("should", func(pvc *v1.PersistentVolumeClaim, expectedResult v1.PersistentVolumeMode) {
		result := GetVolumeMode(pvc)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("return block if pvc has block volume mode", pvcVolumeModeBlock, v1.PersistentVolumeBlock),
		Entry("return file system if pvc has filesystem mode", pvcVolumeModeFilesystem, v1.PersistentVolumeFilesystem),
		Entry("return file system if pvc has no mode defined", pvcVolumeModeFilesystemDefault, v1.PersistentVolumeFilesystem),
	)
})

var _ = Describe("CheckIfLabelExists", func() {
	pvc := CreatePvc("testPVC", "default", nil, map[string]string{common.CDILabelKey: common.CDILabelValue})
	pvcNoLbl := CreatePvc("testPVC2", "default", nil, nil)

	DescribeTable("should", func(pvc *v1.PersistentVolumeClaim, key, value string, expectedResult bool) {
		result := checkIfLabelExists(pvc, key, value)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("return true if label with value exists", pvc, common.CDILabelKey, common.CDILabelValue, true),
		Entry("return false if label with value does not exists", pvc, AnnCreatedBy, "yes", false),
		Entry("return false if label exists, but value doesn't match", pvc, common.CDILabelKey, "something", false),
		Entry("return false if pvc has no labels", pvcNoLbl, common.CDILabelKey, common.CDILabelValue, false),
		Entry("return false if pvc has no labels and check key and value are blank", pvcNoLbl, "", "", false),
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
		res, err := GetWorkloadNodePlacement(context.TODO(), client)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
	})

	It("Should return an err with > 1 CDI CR", func() {
		client := CreateClient(createCDIWithWorkload("cdi-test", "1111-1111"), createCDIWithWorkload("cdi-test2", "2222-2222"))
		res, err := GetWorkloadNodePlacement(context.TODO(), client)
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
	})

	It("Should return a node placement, with one active CDI CR one error", func() {
		errCR := createCDIWithWorkload("cdi-test2", "2222-2222")
		errCR.Status.Phase = sdkapi.PhaseError
		client := CreateClient(createCDIWithWorkload("cdi-test", "1111-1111"), errCR)
		res, err := GetWorkloadNodePlacement(context.TODO(), client)
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
		setAnnotationsFromPodWithPrefix(result, testPod, nil, AnnRunningCondition)
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
		setAnnotationsFromPodWithPrefix(result, testPod, nil, AnnRunningCondition)
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
		setAnnotationsFromPodWithPrefix(result, testPod, nil, AnnRunningCondition)
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
						Terminated: &v1.ContainerStateTerminated{},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, &common.TerminationMessage{PreallocationApplied: ptr.To(true)}, AnnRunningCondition)
		Expect(result[AnnPreallocationApplied]).To(Equal("true"))
	})

	It("Should set scratch space required status", func() {
		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, &common.TerminationMessage{ScratchSpaceRequired: ptr.To(true)}, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal(common.ScratchSpaceRequired))
		Expect(result[AnnRunningConditionReason]).To(Equal(ScratchSpaceRequiredReason))
		Expect(result[AnnRequiresScratch]).To(Equal("true"))
	})

	It("Should set image pull failure message and reason", func() {
		const errorIncludesImagePullText = `Unable to process data: ` + common.ImagePullFailureText + `: reading manifest wrong
in quay.io/myproject/myimage: manifest unknown`

		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: errorIncludesImagePullText,
							Reason:  common.GenericError,
						},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, nil, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal(errorIncludesImagePullText))
		Expect(result[AnnRunningConditionReason]).To(Equal(ImagePullFailedReason))
		Expect(result[AnnRequiresScratch]).To(BeEmpty())
	})

	It("Should set running reason as error for general errors", func() {
		const errorMessage = `just a fake error text to check in this test`

		result := make(map[string]string)
		testPod := CreateImporterTestPod(CreatePvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
		testPod.Status = v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: errorMessage,
							Reason:  common.GenericError,
						},
					},
				},
			},
		}
		setAnnotationsFromPodWithPrefix(result, testPod, nil, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal(errorMessage))
		Expect(result[AnnRunningConditionReason]).To(Equal(common.GenericError))
		Expect(result[AnnRequiresScratch]).To(BeEmpty())
	})
})

var _ = Describe("addLabelsFromTerminationMessage", func() {
	It("should add labels from termMsg", func() {
		labels := make(map[string]string, 0)
		termMsg := &common.TerminationMessage{
			Labels: map[string]string{
				"test": "test",
			},
		}

		newLabels := addLabelsFromTerminationMessage(labels, termMsg)

		Expect(labels).To(BeEmpty())
		for k, v := range termMsg.Labels {
			Expect(newLabels).To(HaveKeyWithValue(k, v))
		}
	})

	It("should handle nil labels", func() {
		termMsg := &common.TerminationMessage{
			Labels: map[string]string{
				"test": "test",
			},
		}

		newLabels := addLabelsFromTerminationMessage(nil, termMsg)
		for k, v := range termMsg.Labels {
			Expect(newLabels).To(HaveKeyWithValue(k, v))
		}
	})

	It("should not overwrite existing labels from termMsg", func() {
		const testKeyExisting = "test"
		const testValueExisting = "existing"

		labels := map[string]string{
			testKeyExisting: testValueExisting,
		}
		termMsg := &common.TerminationMessage{
			Labels: map[string]string{
				testKeyExisting: "somethingelse",
			},
		}

		newLabels := addLabelsFromTerminationMessage(labels, termMsg)
		Expect(newLabels).To(HaveKeyWithValue(testKeyExisting, testValueExisting))
	})

	It("should handle nil termMsg", func() {
		labels := map[string]string{
			"test": "test",
		}

		newLabels := addLabelsFromTerminationMessage(labels, nil)
		Expect(newLabels).To(Equal(labels))
	})
})

var _ = Describe("GetPreallocation", func() {
	It("Should return preallocation for DataVolume if specified", func() {
		client := CreateClient()
		dv := createDataVolumeWithPreallocation("test-dv", "test-ns", true)
		preallocation := GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeTrue())

		dv = createDataVolumeWithPreallocation("test-dv", "test-ns", false)
		preallocation = GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeFalse())

		// global: true, data volume overrides to false
		client = CreateClient(createCDIConfigWithGlobalPreallocation(true))
		dv = createDataVolumeWithStorageClassPreallocation("test-dv", "test-ns", "test-class", false)
		preallocation = GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeFalse())
	})

	It("Should return global preallocation setting if not defined in DV or SC", func() {
		client := CreateClient(createCDIConfigWithGlobalPreallocation(true))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeTrue())

		client = CreateClient(createCDIConfigWithGlobalPreallocation(false))
		preallocation = GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeFalse())
	})

	It("Should be false when niether DV nor Config defines preallocation", func() {
		client := CreateClient(createCDIConfig("test"))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(context.Background(), client, dv.Spec.Preallocation)
		Expect(preallocation).To(BeFalse())
	})
})

var _ = Describe("HandleFailedPod", func() {
	pvc := CreatePvc("test-pvc", "test-ns", nil, nil)
	podName := "test-pod"

	DescribeTable("Should record an event with pertinent information if pod fails due to", func(errMsg, reason string) {
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
		Entry("generic error", "test error", ErrStartingPod),
		Entry("quota error", "exceeded quota:", ErrExceededQuota),
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

	DescribeTable("should", func(pvc *v1.PersistentVolumeClaim, annotation string, expectedResult bool) {
		result := checkPVC(pvc, annotation, utilLog)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("return false if no annotation provided", pvcNoAnno, AnnEndpoint, false),
		Entry("return true if annotation provided that matches test http", pvcWithEndPointAnno, AnnEndpoint, true),
		Entry("return true if annotation provided that matches test clone", pvcWithCloneRequestAnno, AnnCloneRequest, true),
	)
})

var _ = Describe("createScratchPersistentVolumeClaim", func() {
	DescribeTable("Should create a scratch PVC of the correct size, taking fs overhead into account", func(scratchOverhead, scOverhead cdiv1.Percent, expectedValue int64) {
		cdiConfig := createCDIConfigWithStorageClass(common.ConfigName, scratchStorageClassName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: "0.05",
			StorageClass: map[string]cdiv1.Percent{
				scratchStorageClassName: scratchOverhead,
				storageClassName:        scOverhead,
			},
		}
		cl := CreateClient(cdiConfig, CreateStorageClass(scratchStorageClassName, nil), CreateStorageClass(storageClassName, nil))
		rec := record.NewFakeRecorder(10)
		By("Create a 1Gi pvc")
		testPvc := CreatePvcInStorageClass("testPvc", "default", ptr.To[string](storageClassName), nil, nil, v1.ClaimBound)
		testPvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse("1Gi")
		name := "test-scratchspace-pvc"
		pod := &v1.Pod{}
		res, err := createScratchPersistentVolumeClaim(cl, testPvc, pod, name, scratchStorageClassName, nil, rec)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
		Expect(res.Spec.Resources).ToNot(BeNil())
		Expect(res.Spec.Resources.Requests.Storage()).ToNot(BeNil())
		scratchPVCSize := *res.Spec.Resources.Requests.Storage()
		Expect(scratchPVCSize.Value()).To(Equal(expectedValue * 1024 * 1024))
	},
		Entry("same scratch and storage class overhead", cdiv1.Percent("0.03"), cdiv1.Percent("0.03"), int64(1024)),
		Entry("scratch  > storage class overhead", cdiv1.Percent("0.1"), cdiv1.Percent("0.03"), int64(1094)),
		Entry("scratch  < storage class overhead", cdiv1.Percent("0.03"), cdiv1.Percent("0.1"), int64(958)),
	)

	It("Should calculate the correct size for a scratch PVC from a block volume", func() {
		cdiConfig := createCDIConfigWithStorageClass(common.ConfigName, scratchStorageClassName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: "0.05",
		}
		cl := CreateClient(cdiConfig)
		rec := record.NewFakeRecorder(10)
		By("Create a 1Gi pvc")
		testPvc := CreatePvcInStorageClass("testPvc", "default", ptr.To[string](storageClassName), nil, nil, v1.ClaimBound)
		testPvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse("1Gi")
		testPvc.Spec.VolumeMode = ptr.To[v1.PersistentVolumeMode](v1.PersistentVolumeBlock)
		name := "test-scratchspace-pvc"
		pod := &v1.Pod{}
		res, err := createScratchPersistentVolumeClaim(cl, testPvc, pod, name, scratchStorageClassName, nil, rec)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
		Expect(res.Spec.Resources).ToNot(BeNil())
		Expect(res.Spec.Resources.Requests.Storage()).ToNot(BeNil())
		scratchPVCSize := *res.Spec.Resources.Requests.Storage()
		Expect(scratchPVCSize.Value()).To(Equal(int64(1076 * 1024 * 1024)))
	})

	It("Should add skip velero backup label", func() {
		cdiConfig := createCDIConfigWithStorageClass(common.ConfigName, scratchStorageClassName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: "0.05",
		}
		cl := CreateClient(cdiConfig)
		rec := record.NewFakeRecorder(10)
		By("Create a 1Gi pvc")
		testPvc := CreatePvcInStorageClass("testPvc", "default", ptr.To[string](storageClassName), nil, nil, v1.ClaimBound)
		testPvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse("1Gi")
		testPvc.Spec.VolumeMode = ptr.To[v1.PersistentVolumeMode](v1.PersistentVolumeBlock)
		name := "test-scratchspace-pvc"
		pod := &v1.Pod{}
		scratchPVC, err := createScratchPersistentVolumeClaim(cl, testPvc, pod, name, scratchStorageClassName, nil, rec)
		Expect(err).ToNot(HaveOccurred())
		Expect(scratchPVC).ToNot(BeNil())
		Expect(scratchPVC.GetLabels()[LabelExcludeFromVeleroBackup]).To(Equal("true"))
	})
})

func createDataVolumeWithStorageClass(name, ns, storageClassName string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{},
			PVC: &v1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
			},
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
			PVC: &v1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
			},
		},
	}
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
			Workloads: sdkapi.NodePlacement{},
		},
		Status: cdiv1.CDIStatus{
			Status: sdkapi.Status{
				Phase: sdkapi.PhaseDeployed,
			},
		},
	}
}
