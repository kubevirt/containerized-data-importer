package datavolume

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	ocpconfigv1 "github.com/openshift/api/config/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var _ = Describe("renderPvcSpecVolumeSize", func() {
	client := createClient()
	volumeSize := resource.MustParse("1G")
	scName := "test"

	It("Should return empty volume size on clone PVC with empty storage size", func() {
		pvcSpec := &corev1.PersistentVolumeClaimSpec{}
		err := renderPvcSpecVolumeSize(client, pvcSpec, true, nil)
		Expect(err).ToNot(HaveOccurred())
		requestedVolumeSize, found := pvcSpec.Resources.Requests[corev1.ResourceStorage]
		Expect(found).To(BeTrue())
		Expect(requestedVolumeSize.IsZero()).To(BeTrue())
	})

	It("Should return error on non-clone PVC with empty storage size", func() {
		pvcSpec := &corev1.PersistentVolumeClaimSpec{}
		err := renderPvcSpecVolumeSize(client, pvcSpec, false, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("PVC Spec is not valid - missing storage size"))
		_, found := pvcSpec.Resources.Requests[corev1.ResourceStorage]
		Expect(found).To(BeFalse())
	})

	It("Should return error on PVC with storage size smaller than 1MiB", func() {
		volumeMode := corev1.PersistentVolumeBlock
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("9"),
				},
			},
		}
		err := renderPvcSpecVolumeSize(client, pvcSpec, false, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("PVC Spec is not valid - storage size should be at least 1MiB"))
	})

	It("Should return the same volume size (block volume mode)", func() {
		volumeMode := corev1.PersistentVolumeBlock
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: volumeSize,
				},
			},
		}
		err := renderPvcSpecVolumeSize(client, pvcSpec, false, nil)
		Expect(err).ToNot(HaveOccurred())

		requestedVolumeSize, found := pvcSpec.Resources.Requests[corev1.ResourceStorage]
		Expect(found).To(BeTrue())
		Expect(requestedVolumeSize.Value()).To(Equal(volumeSize.Value()))
	})

	It("Should return the inflated volume size (filesystem volume mode)", func() {
		volumeMode := corev1.PersistentVolumeFilesystem
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: volumeSize,
				},
			},
		}
		err := renderPvcSpecVolumeSize(client, pvcSpec, false, nil)
		Expect(err).ToNot(HaveOccurred())
		requestedVolumeSize, found := pvcSpec.Resources.Requests[corev1.ResourceStorage]
		Expect(found).To(BeTrue())

		// Inflate expected size with overhead
		fsOverhead, err := GetFilesystemOverheadForStorageClass(context.TODO(), client, pvcSpec.StorageClassName)
		Expect(err).ToNot(HaveOccurred())

		fsOverheadFloat, _ := strconv.ParseFloat(string(fsOverhead), 64)
		requiredSpace := util.GetRequiredSpace(fsOverheadFloat, volumeSize.Value())
		expectedResult := resource.NewScaledQuantity(requiredSpace, 0)

		Expect(requestedVolumeSize.Value()).To(BeNumerically(">", volumeSize.Value()))
		Expect(requestedVolumeSize.Value()).To(Equal(expectedResult.Value()))
	})

	DescribeTable("Should return", func(storageSize, minSupportedSize, expectedSize string) {
		sp := &cdiv1.StorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
				Annotations: map[string]string{
					AnnMinimumSupportedPVCSize: minSupportedSize,
				},
			},
		}
		client = createClient(sp)

		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		}
		err := renderPvcSpecVolumeSize(client, pvcSpec, false, nil)
		Expect(err).ToNot(HaveOccurred())
		requestedSize, found := pvcSpec.Resources.Requests[corev1.ResourceStorage]
		Expect(found).To(BeTrue())
		expected := resource.MustParse(expectedSize)
		Expect(requestedSize.Value()).To(Equal(expected.Value()))
	},
		Entry("increased volume size if smaller than minimal", "1Gi", "4Gi", "4Gi"),
		Entry("original volume size if larger than minimal", "5Gi", "4Gi", "5Gi"),
		Entry("original volume size if no minimal size defined", "1Gi", "", "1Gi"),
		Entry("original volume size if wrong minimal size defined", "1Gi", "bla", "1Gi"),
	)
})

var _ = Describe("renderPvcSpec", func() {
	block := corev1.PersistentVolumeBlock
	filesystem := corev1.PersistentVolumeFilesystem
	rwo := corev1.ReadWriteOnce
	rwx := corev1.ReadWriteMany

	DescribeTable("Rendering pvcSpec based on storageProfile should", func(
		storageVolumeMode *corev1.PersistentVolumeMode, storageAccessMode *corev1.PersistentVolumeAccessMode,
		expectedVolumeMode *corev1.PersistentVolumeMode, expectedAccessMode *corev1.PersistentVolumeAccessMode,
		expectedError *string) {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, nil, nil, "")
		sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{rwo}, block)
		cdiconfig := &cdiv1.CDIConfig{ObjectMeta: metav1.ObjectMeta{Name: "config"}}
		client := createClient(sc, sp, cdiconfig)

		storageSpec := &cdiv1.StorageSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		if storageVolumeMode != nil {
			storageSpec.VolumeMode = storageVolumeMode
		}
		if storageAccessMode != nil {
			storageSpec.AccessModes = []corev1.PersistentVolumeAccessMode{*storageAccessMode}
		}
		dv := createDataVolumeWithStorageAPI("testDV", metav1.NamespaceDefault, &cdiv1.DataVolumeSource{}, storageSpec)

		pvcSpec, err := renderPvcSpec(client, nil, logr.Logger{}, dv, nil)
		if expectedError != nil {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(*expectedError))
			Expect(pvcSpec).To(BeNil())
			return
		}

		Expect(err).ToNot(HaveOccurred())
		if expectedVolumeMode != nil {
			Expect(pvcSpec.VolumeMode).ToNot(BeNil())
			Expect(*pvcSpec.VolumeMode).To(Equal(*expectedVolumeMode))
		} else {
			Expect(pvcSpec.VolumeMode).To(BeNil())
		}
		if expectedAccessMode != nil {
			Expect(pvcSpec.AccessModes).ToNot(BeNil())
			Expect(pvcSpec.AccessModes[0]).To(Equal(*expectedAccessMode))
		} else {
			Expect(pvcSpec.AccessModes).To(BeNil())
		}
	},
		Entry("set default volumeMode and accessMode if not passed", nil, nil, &block, &rwo, nil),
		Entry("set a matching accessMode for the volumeMode", &block, nil, &block, &rwo, nil),
		Entry("set a matching volumeMode for the accessMode", nil, &rwo, &block, &rwo, nil),
		Entry("fail when volumeMode has no matching accessMode", &filesystem, nil, nil, nil, ptr.To("no matching accessMode specified in StorageProfile testSC")),
		Entry("fallback to k8s default when accessMode has no matching volumeMode", nil, &rwx, nil, &rwx, nil),
		Entry("use the passed volumeMode and accessMode even if not in storageProfile", &filesystem, &rwx, &filesystem, &rwx, nil),
	)
})

var _ = Describe("updateDataVolumeDefaultInstancetypeLabels", func() {

	const (
		namespace            = "namespace"
		sourcePVCName        = "sourcePVC"
		sourceSnapshotName   = "sourceSnapshot"
		sourceRegistryName   = "sourceRegistry"
		sourceDataSourceName = "sourceDataSource"
	)

	var (
		fakeClient                     client.Client
		dataVolumeWithSourcePVC        cdiv1.DataVolume
		dataVolumeWithSourceSnapshot   cdiv1.DataVolume
		dataVolumeWithSourceRegistry   cdiv1.DataVolume
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

		dataVolumeWithSourceSnapshot = cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "snapshot-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Snapshot: &cdiv1.DataVolumeSourceSnapshot{
						Name:      sourceSnapshotName,
						Namespace: namespace,
					},
				},
			},
		}

		dataVolumeWithSourceRegistry = cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sourceRegistryName,
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Registry: &cdiv1.DataVolumeSourceRegistry{
						URL: ptr.To("docker://testurl"),
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
			&snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceSnapshotName,
					Namespace: namespace,
					Labels:    defaultInstancetypeLabelMap,
				},
			},
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceRegistryName,
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
		Entry("Snapshot", &dataVolumeWithSourceSnapshot),
		Entry("Registry", &dataVolumeWithSourceRegistry),
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
		Entry("Snapshot", &dataVolumeWithSourceSnapshot),
		Entry("Registry", &dataVolumeWithSourceRegistry),
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
		Entry("Snapshot", &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "snapshot-datavolume",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Snapshot: &cdiv1.DataVolumeSourceSnapshot{
						Name:      "unknown",
						Namespace: namespace,
					},
				},
			},
		}),
		Entry("Registry", &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-datavolume-unknown",
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Registry: &cdiv1.DataVolumeSourceRegistry{
						URL: ptr.To("docker://uknown"),
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
		Entry("Snapshot", &dataVolumeWithSourceSnapshot),
		Entry("Registry", &dataVolumeWithSourceRegistry),
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
	_ = snapshotv1.AddToScheme(s)
	// Create a fake client to mock API calls.
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

type fakeClientWithGetServiceUnavailableErr struct {
	client.Client
}

func (c fakeClientWithGetServiceUnavailableErr) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return errors.NewServiceUnavailable("error")
}
