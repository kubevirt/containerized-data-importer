package datavolume

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"

	ocpconfigv1 "github.com/openshift/api/config/v1"
)

var _ = Describe("resolveVolumeSize", func() {
	client := createClient()
	scName := "test"
	pvcSpec := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1G"),
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
		Expect(requestedVolumeSize.IsZero()).To(BeTrue())
	})

	It("Should return error after trying to create a DataVolume with empty storage size and http source", func() {
		httpSource := &cdiv1.DataVolumeSource{
			HTTP: &cdiv1.DataVolumeSourceHTTP{},
		}
		storageSpec := &cdiv1.StorageSpec{}
		dv := createDataVolumeWithStorageAPI("testDV", "testNamespace", httpSource, storageSpec)
		requestedVolumeSize, err := resolveVolumeSize(client, dv.Spec, pvcSpec)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Datavolume Spec is not valid - missing storage size"))
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
		fsOverhead, err2 := GetFilesystemOverheadForStorageClass(context.TODO(), client, dv.Spec.Storage.StorageClassName)
		Expect(err2).ToNot(HaveOccurred())
		fsOverheadFloat, _ := strconv.ParseFloat(string(fsOverhead), 64)
		requiredSpace := GetRequiredSpace(fsOverheadFloat, requestedVolumeSize.Value())
		expectedResult := resource.NewScaledQuantity(requiredSpace, 0)
		Expect(expectedResult.Value()).To(Equal(requestedVolumeSize.Value()))
	})
})

var _ = Describe("updateDataVolumeDefaultInstancetypeLabels", func() {

	const (
		namespace            = "namespace"
		sourcePVCName        = "sourcePVC"
		sourceDataSourceName = "sourceDataSource"
	)

	var (
		fakeClient                     client.Client
		dataVolumeWithSourcePVC        cdiv1.DataVolume
		dataVolumeWithSourceDataSource cdiv1.DataVolume
	)

	defaultInstancetypeLabelMap := map[string]string{
		LabelDefaultInstancetype:     LabelDefaultInstancetype,
		LabelDefaultInstancetypeKind: LabelDefaultInstancetypeKind,
		LabelDefaultPreference:       LabelDefaultPreference,
		LabelDefaultPreferenceKind:   LabelDefaultPreferenceKind,
	}

	BeforeEach(func() {
		dataVolumeWithSourcePVC = cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					PVC: &cdiv1.DataVolumeSourcePVC{
						Name:      sourcePVCName,
						Namespace: namespace,
					},
				},
			},
		}

		dataVolumeWithSourceDataSource = cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "datasource-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				SourceRef: &cdiv1.DataVolumeSourceRef{
					Kind:      cdiv1.DataVolumeDataSource,
					Name:      sourceDataSourceName,
					Namespace: ptr.To[string](namespace),
				},
			},
		}
		fakeClient = createClient(
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourcePVCName,
					Namespace: namespace,
					Labels:    defaultInstancetypeLabelMap,
				},
			},
			&cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceDataSourceName,
					Namespace: namespace,
					Labels:    defaultInstancetypeLabelMap,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						PVC: &cdiv1.DataVolumeSourcePVC{
							Name: sourcePVCName,
						},
					},
				},
			},
		)
	})

	DescribeTable("should update DataVolume with labels from source using", func(dataVolume *cdiv1.DataVolume) {
		syncState := &dvSyncState{
			dvMutated: dataVolume,
		}
		Expect(syncState.dvMutated.Labels).To(BeEmpty())
		Expect(updateDataVolumeDefaultInstancetypeLabels(fakeClient, syncState)).To(Succeed())
		Expect(syncState.dvMutated.Labels).ToNot(BeEmpty())
		for k, v := range defaultInstancetypeLabelMap {
			Expect(syncState.dvMutated.Labels).To(HaveKeyWithValue(k, v))
		}
	},
		Entry("PVC", &dataVolumeWithSourcePVC),
		Entry("dataSource", &dataVolumeWithSourceDataSource),
	)

	DescribeTable("should not update DataVolume with labels from source if already present using", func(dataVolume *cdiv1.DataVolume) {
		const customDefaultInstancetype = "customDefaultInstancetype"
		dv := dataVolume
		dv.Labels = map[string]string{
			LabelDefaultInstancetype: customDefaultInstancetype,
		}
		syncState := &dvSyncState{
			dvMutated: dv,
		}

		Expect(updateDataVolumeDefaultInstancetypeLabels(fakeClient, syncState)).To(Succeed())
		Expect(syncState.dvMutated.Labels).To(HaveLen(1))
		Expect(syncState.dvMutated.Labels).To(HaveKeyWithValue(LabelDefaultInstancetype, customDefaultInstancetype))
	},
		Entry("PVC", &dataVolumeWithSourcePVC),
		Entry("DataSource", &dataVolumeWithSourceDataSource),
	)

	DescribeTable("should ignore IsNotFound errors", func(dataVolume *cdiv1.DataVolume) {
		syncState := &dvSyncState{
			dvMutated: dataVolume,
		}
		Expect(updateDataVolumeDefaultInstancetypeLabels(fakeClient, syncState)).To(Succeed())
	},
		Entry("PVC", &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					PVC: &cdiv1.DataVolumeSourcePVC{
						Name:      "unknown",
						Namespace: namespace,
					},
				},
			},
		}),
		Entry("DataSource", &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "datasource-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				SourceRef: &cdiv1.DataVolumeSourceRef{
					Kind:      cdiv1.DataVolumeDataSource,
					Name:      "unknown",
					Namespace: ptr.To[string](namespace),
				},
			},
		}),
	)

	DescribeTable("should return all non IsNotFound errors", func(dataVolume *cdiv1.DataVolume) {
		err := updateDataVolumeDefaultInstancetypeLabels(
			fakeClientWithGetServiceUnavailableErr{
				fakeClient,
			},
			&dvSyncState{
				dvMutated: dataVolume,
			},
		)
		Expect(errors.IsServiceUnavailable(err)).To(BeTrue())
	},
		Entry("PVC", &dataVolumeWithSourcePVC),
		Entry("DataSource", &dataVolumeWithSourceDataSource),
	)
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

func createClient(objs ...client.Object) client.Client {
	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	// Register other types with the runtime scheme.
	_ = ocpconfigv1.Install(s)
	// Create a fake client to mock API calls.
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

type fakeClientWithGetServiceUnavailableErr struct {
	client.Client
}

func (c fakeClientWithGetServiceUnavailableErr) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return errors.NewServiceUnavailable("error")
}
