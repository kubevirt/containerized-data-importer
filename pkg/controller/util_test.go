package controller

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
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
	"k8s.io/client-go/tools/record"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	ocpconfigv1 "github.com/openshift/api/config/v1"
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

var _ = Describe("resolveVolumeSize", func() {
	client := createClient()
	scName := "test"
	pvcSpec := &v1.PersistentVolumeClaimSpec{
		AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
			},
		},
		StorageClassName: &scName,
	}

	It("Should return empty volume size", func() {
		pvcSource := &cdiv1.DataVolumeSource{
			PVC: &cdiv1.DataVolumeSourcePVC{},
		}
		storageSpec := &cdiv1.StorageSpec{}
		dv := createDataVolumeWithStorageAPI("testDV", "testNamespace", pvcSource, storageSpec)
		requestedVolumeSize, err := resolveVolumeSize(client, dv.Spec, pvcSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(requestedVolumeSize.IsZero()).To(Equal(true))
	})

	It("Should return error after trying to create a DataVolume with empty storage size and http source", func() {
		httpSource := &cdiv1.DataVolumeSource{
			HTTP: &cdiv1.DataVolumeSourceHTTP{},
		}
		storageSpec := &cdiv1.StorageSpec{}
		dv := createDataVolumeWithStorageAPI("testDV", "testNamespace", httpSource, storageSpec)
		requestedVolumeSize, err := resolveVolumeSize(client, dv.Spec, pvcSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Datavolume Spec is not valid - missing storage size")))
		Expect(requestedVolumeSize).To(BeNil())
	})

	It("Should return the expected volume size (block volume mode)", func() {
		storageSpec := &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1G"),
				},
			},
		}
		volumeMode := corev1.PersistentVolumeBlock
		pvcSpec.VolumeMode = &volumeMode
		dv := createDataVolumeWithStorageAPI("testDV", "testNamespace", nil, storageSpec)
		requestedVolumeSize, err := resolveVolumeSize(client, dv.Spec, pvcSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(storageSpec.Resources.Requests.Storage().Value()).To(Equal(requestedVolumeSize.Value()))
	})

	It("Should return the expected volume size (filesystem volume mode)", func() {
		storageSpec := &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		volumeMode := corev1.PersistentVolumeFilesystem
		pvcSpec.VolumeMode = &volumeMode
		dv := createDataVolumeWithStorageAPI("testDV", "testNamespace", nil, storageSpec)
		requestedVolumeSize, err := resolveVolumeSize(client, dv.Spec, pvcSpec)
		Expect(err).ToNot(HaveOccurred())
		// Inflate expected size with overhead
		fsOverhead, err2 := GetFilesystemOverheadForStorageClass(client, dv.Spec.Storage.StorageClassName)
		Expect(err2).ToNot(HaveOccurred())
		fsOverheadFloat, _ := strconv.ParseFloat(string(fsOverhead), 64)
		requiredSpace := GetRequiredSpace(fsOverheadFloat, requestedVolumeSize.Value())
		expectedResult := resource.NewScaledQuantity(requiredSpace, 0)
		Expect(expectedResult.Value()).To(Equal(requestedVolumeSize.Value()))
	})
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

	It("Should return a node placement, with one active CDI CR one error", func() {
		errCR := createCDIWithWorkload("cdi-test2", "2222-2222")
		errCR.Status.Phase = sdkapi.PhaseError
		client := createClient(createCDIWithWorkload("cdi-test", "1111-1111"), errCR)
		res, err := GetWorkloadNodePlacement(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
	})
})

func createClient(objs ...runtime.Object) client.Client {
	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	// Register other types with the runtime scheme.
	ocpconfigv1.AddToScheme(s)
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

var _ = Describe("setAnnotationsFromPod", func() {
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
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
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
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
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
		setAnnotationsFromPodWithPrefix(result, testPod, AnnRunningCondition)
		Expect(result[AnnRunningCondition]).To(Equal("false"))
		Expect(result[AnnRunningConditionMessage]).To(Equal("container is waiting"))
		Expect(result[AnnRunningConditionReason]).To(Equal("Pending"))
	})

	It("Should set preallocation status", func() {
		result := make(map[string]string)
		testPod := createImporterTestPod(createPvc("test", metav1.NamespaceDefault, nil, nil), "test", nil)
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
		client := createClient()
		dv := createDataVolumeWithPreallocation("test-dv", "test-ns", true)
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeTrue())

		dv = createDataVolumeWithPreallocation("test-dv", "test-ns", false)
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())

		// global: true, data volume overrides to false
		client = createClient(createCDIConfigWithGlobalPreallocation(true))
		dv = createDataVolumeWithStorageClassPreallocation("test-dv", "test-ns", "test-class", false)
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})

	It("Should return global preallocation setting if not defined in DV or SC", func() {
		client := createClient(createCDIConfigWithGlobalPreallocation(true))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeTrue())

		client = createClient(createCDIConfigWithGlobalPreallocation(false))
		preallocation = GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})

	It("Should be false when niether DV nor Config defines preallocation", func() {
		client := createClient(createCDIConfig("test"))
		dv := createDataVolumeWithStorageClass("test-dv", "test-ns", "test-class")
		preallocation := GetPreallocation(client, dv)
		Expect(preallocation).To(BeFalse())
	})
})

var _ = Describe("GetDefaultStorageClass", func() {
	It("Should return the default storage class name", func() {
		client := createClient(
			createStorageClass("test-storage-class-1", nil),
			createStorageClass("test-storage-class-2", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
		)
		sc, _ := GetDefaultStorageClass(client)
		Expect(sc.Name).To(Equal("test-storage-class-2"))
	})

	It("Should return nil if there's not default storage class", func() {
		client := createClient(
			createStorageClass("test-storage-class-1", nil),
			createStorageClass("test-storage-class-2", nil),
		)
		sc, _ := GetDefaultStorageClass(client)
		Expect(sc).To(BeNil())
	})
})

var _ = Describe("GetClusterWideProxy", func() {
	var proxyHTTPURL = "http://user:pswd@www.myproxy.com"
	var proxyHTTPSURL = "https://user:pswd@www.myproxy.com"
	var noProxyDomains = ".noproxy.com"
	var trustedCAName = "user-ca-bundle"

	It("Should return a not empty cluster wide proxy obj", func() {
		client := createClient(createClusterWideProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAName))
		proxy, err := GetClusterWideProxy(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).ToNot(BeNil())

		By("should return a proxy https url")
		Expect(proxyHTTPSURL).To(Equal(proxy.Status.HTTPSProxy))

		By("should return a proxy http url")
		Expect(proxyHTTPURL).To(Equal(proxy.Status.HTTPProxy))

		By("should return a noProxy list of domains")
		Expect(noProxyDomains).To(Equal(proxy.Status.NoProxy))

		By("should return a CA ConfigMap name")
		Expect(trustedCAName).To(Equal(proxy.Spec.TrustedCA.Name))
	})

	It("Should return a nil cluster wide proxy obj", func() {
		client := createClient()
		proxy, err := GetClusterWideProxy(client)
		Expect(err).ToNot(HaveOccurred())
		Expect(proxy).To(BeEquivalentTo(&ocpconfigv1.Proxy{}))
	})
})

var _ = Describe("GetImportProxyConfig", func() {
	var proxyHTTPURL = "http://user:pswd@www.myproxy.com"
	var proxyHTTPSURL = "https://user:pswd@www.myproxy.com"
	var noProxyDomains = ".noproxy.com"
	var trustedCAName = "user-ca-bundle"

	It("should return valid proxy information from a CDIConfig with importer proxy configured", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy(proxyHTTPURL, proxyHTTPSURL, noProxyDomains, trustedCAName)
		field, _ := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect(proxyHTTPURL).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS)
		Expect(proxyHTTPSURL).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy)
		Expect(noProxyDomains).To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName)
		Expect(trustedCAName).To(Equal(field))
	})

	It("should return blank proxy information from a CDIConfig with importer proxy not configured", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy("", "", "", "")
		field, _ := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy)
		Expect("").To(Equal(field))
		field, _ = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName)
		Expect("").To(Equal(field))
	})

	It("should return error if the requested field does not exist", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = createImportProxy("", "", "", "")
		_, err := GetImportProxyConfig(cdiConfig, "nonExistingField")
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("CDIConfig ImportProxy does not have the field: %s\n", "nonExistingField")))
	})

	It("should return error if the ImportProxy field is nil", func() {
		cdiConfig := MakeEmptyCDIConfigSpec("cdiconfig")
		cdiConfig.Status.ImportProxy = nil
		_, err := GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP)
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to get field, the CDIConfig ImportProxy is nil\n")))
	})
})

var _ = Describe("validateContentTypes", func() {
	getContentType := func(contentType string) cdiv1.DataVolumeContentType {
		if contentType == "" {
			return cdiv1.DataVolumeKubeVirt
		}
		return cdiv1.DataVolumeContentType(contentType)
	}

	table.DescribeTable("should return", func(sourceContentType, targetContentType string, expectedResult bool) {
		sourcePvc := createPvc("testPVC", "default", map[string]string{AnnContentType: sourceContentType}, nil)
		dvSpec := &cdiv1.DataVolumeSpec{}
		dvSpec.ContentType = cdiv1.DataVolumeContentType(targetContentType)

		validated, sourceContent, targetContent := validateContentTypes(sourcePvc, dvSpec)
		Expect(validated).To(Equal(expectedResult))
		Expect(sourceContent).To(Equal(getContentType(sourceContentType)))
		Expect(targetContent).To(Equal(getContentType(targetContentType)))
	},
		table.Entry("true when using archive in source and target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeArchive), true),
		table.Entry("false when using archive in source and KubeVirt in target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeKubeVirt), false),
		table.Entry("false when using KubeVirt in source and archive in target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeArchive), false),
		table.Entry("true when using KubeVirt in source and target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeKubeVirt), true),
		table.Entry("true when using default in source and target", "", "", true),
		table.Entry("true when using default in source and KubeVirt (explicit) in target", "", string(cdiv1.DataVolumeKubeVirt), true),
		table.Entry("true when using KubeVirt (explicit) in source and default in target", string(cdiv1.DataVolumeKubeVirt), "", true),
		table.Entry("false when using default in source and archive in target", "", string(cdiv1.DataVolumeArchive), false),
		table.Entry("false when using archive in source and default in target", string(cdiv1.DataVolumeArchive), "", false),
	)
})

var _ = Describe("ValidateClone", func() {
	sourcePvc := createPvc("testPVC", "default", map[string]string{}, nil)
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

var _ = Describe("handleFailedPod", func() {
	pvc := createPvc("test-pvc", "test-ns", nil, nil)
	podName := "test-pod"

	table.DescribeTable("Should record an event with pertinent information if pod fails due to", func(errMsg, reason string) {
		// Create a mock reconciler to record the events
		cl := createClient(pvc)
		rec := record.NewFakeRecorder(10)
		r := &DatavolumeReconciler{
			client:   cl,
			recorder: rec,
		}

		err := handleFailedPod(errors.New(errMsg), podName, pvc, r.recorder, r.client)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(errMsg))

		By("Checking error event recorded")
		msg := fmt.Sprintf(MessageErrStartingPod, podName)
		event := <-r.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring(msg))
		Expect(event).To(ContainSubstring(reason))
	},
		table.Entry("generic error", "test error", ErrStartingPod),
		table.Entry("quota error", "exceeded quota:", ErrExceededQuota),
	)

	It("Should return a different error if the PVC isn't able to update, but still record the event", func() {
		// Create a mock reconciler to record the events
		cl := createClient()
		rec := record.NewFakeRecorder(10)
		r := &DatavolumeReconciler{
			client:   cl,
			recorder: rec,
		}

		err := handleFailedPod(errors.New("test error"), podName, pvc, r.recorder, r.client)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("persistentvolumeclaims \"test-pvc\" not found"))

		By("Checking error event recorded")
		msg := fmt.Sprintf(MessageErrStartingPod, podName)
		event := <-r.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring(msg))
		Expect(event).To(ContainSubstring(ErrStartingPod))
	})
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

func createDataVolumeWithStorageAPI(name, ns string, source *cdiv1.DataVolumeSource, storageSpec *cdiv1.StorageSpec) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source:  source,
			Storage: storageSpec,
		},
	}
}

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
	pvc := &v1.PersistentVolumeClaim{
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
	pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
	return pvc
}

func CreatePv(name string, storageClassName string) *v1.PersistentVolume {
	volumeMode := v1.PersistentVolumeFilesystem
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(name),
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
			},
			StorageClassName: storageClassName,
			VolumeMode:       &volumeMode,
		},
	}
	return pv
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

func createStorageClass(name string, annotations map[string]string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
}
func createStorageProfile(name string,
	accessModes []v1.PersistentVolumeAccessMode,
	volumeMode v1.PersistentVolumeMode) *cdiv1.StorageProfile {
	claimPropertySets := []cdiv1.ClaimPropertySet{{
		AccessModes: accessModes,
		VolumeMode:  &volumeMode,
	}}
	return createStorageProfileWithClaimPropertySets(name, claimPropertySets)
}

func createStorageProfileWithClaimPropertySets(name string,
	claimPropertySets []cdiv1.ClaimPropertySet) *cdiv1.StorageProfile {
	return createStorageProfileWithCloneStrategy(name, claimPropertySets, nil)
}

func createStorageProfileWithCloneStrategy(name string,
	claimPropertySets []cdiv1.ClaimPropertySet,
	cloneStrategy *cdiv1.CDICloneStrategy) *cdiv1.StorageProfile {

	return &cdiv1.StorageProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: cdiv1.StorageProfileStatus{
			StorageClass:      &name,
			ClaimPropertySets: claimPropertySets,
			CloneStrategy:     cloneStrategy,
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

func createStorageClassWithProvisioner(name string, annotations, labels map[string]string, provisioner string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		Provisioner: provisioner,
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
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

func createDefaultPodResourceRequirements(limitCPUValue string, limitMemoryValue string, requestCPUValue string, requestMemoryValue string) *corev1.ResourceRequirements {
	if limitCPUValue == "" {
		limitCPUValue = defaultCPULimit
	}
	cpuLimit, err := resource.ParseQuantity(limitCPUValue)
	Expect(err).ToNot(HaveOccurred())
	if limitMemoryValue == "" {
		limitMemoryValue = defaultMemLimit
	}
	memLimit, err := resource.ParseQuantity(limitMemoryValue)
	Expect(err).ToNot(HaveOccurred())
	if requestCPUValue == "" {
		requestCPUValue = defaultCPURequest
	}
	cpuRequest, err := resource.ParseQuantity(requestCPUValue)
	Expect(err).ToNot(HaveOccurred())
	if requestMemoryValue == "" {
		requestMemoryValue = defaultMemRequest
	}
	memRequest, err := resource.ParseQuantity(requestMemoryValue)
	Expect(err).ToNot(HaveOccurred())
	return &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memLimit},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memRequest},
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
		Status: cdiv1.CDIStatus{
			Status: sdkapi.Status{
				Phase: sdkapi.PhaseDeployed,
			},
		},
	}
}

func createClusterWideProxy(HTTPProxy string, HTTPSProxy string, noProxy string, trustedCAName string) *ocpconfigv1.Proxy {
	proxy := &ocpconfigv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterWideProxyName,
			UID:  types.UID(ClusterWideProxyAPIKind + "-" + ClusterWideProxyName),
		},
		Spec: ocpconfigv1.ProxySpec{
			HTTPProxy:          HTTPProxy,
			HTTPSProxy:         HTTPSProxy,
			NoProxy:            noProxy,
			ReadinessEndpoints: []string{},
			TrustedCA: ocpconfigv1.ConfigMapNameReference{
				Name: trustedCAName,
			},
		},
		Status: ocpconfigv1.ProxyStatus{
			HTTPProxy:  HTTPProxy,
			HTTPSProxy: HTTPSProxy,
			NoProxy:    noProxy,
		},
	}
	return proxy
}

func createClusterWideProxyCAConfigMap(certBytes string) *corev1.ConfigMap {
	configMap := &v1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: ClusterWideProxyConfigMapName, Namespace: ClusterWideProxyConfigMapNameSpace},
		Immutable:  new(bool),
		Data:       map[string]string{ClusterWideProxyConfigMapKey: string(certBytes)},
		BinaryData: map[string][]byte{},
	}
	return configMap
}
