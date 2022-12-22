package datavolume

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"

	ocpconfigv1 "github.com/openshift/api/config/v1"
)

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

func createClient(objs ...runtime.Object) client.Client {
	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	// Register other types with the runtime scheme.
	ocpconfigv1.AddToScheme(s)
	// Create a fake client to mock API calls.
	return fake.NewFakeClientWithScheme(s, objs...)
}
