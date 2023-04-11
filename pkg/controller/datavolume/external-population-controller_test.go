/*
Copyright 2022 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datavolume

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

var (
	dvPopulatorLog = logf.Log.WithName("datavolume-external-population-controller-test")
)

const (
	samplePopulatorName = "sample-populator"
	populatorGroupName  = "cdi.sample.populator"
	populatorKind       = "SamplePopulator"

	snapshotAPIName = "snapshot.storage.k8s.io"
)

var _ = Describe("All external-population tests", func() {
	var (
		reconciler *PopulatorReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	// Environment requirements
	scName := "testsc"
	sc := CreateStorageClassWithProvisioner(scName, map[string]string{
		AnnDefaultStorageClass: "true",
	}, map[string]string{}, "csi-plugin")
	csiDriver := &storagev1.CSIDriver{}
	csiDriver.Name = "csi-plugin"
	accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
	storageProfile := createStorageProfile(scName, accessMode, BlockMode)
	controller := true

	var _ = Describe("Using populator", func() {
		// Sample populator's DataSourceRef
		apiGroup := populatorGroupName
		dataSourceRef := &corev1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     populatorKind,
			Name:     samplePopulatorName,
		}

		AfterEach(func() {
			if reconciler != nil && reconciler.recorder != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
			}
		})

		It("Should succeed if PVC is bound using an external populator", func() {
			dv := newPopulatorDataVolume("test-dv", nil, dataSourceRef)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.DataSourceRef = dv.Spec.PVC.DataSourceRef

			reconciler = createPopulatorReconciler(dv, targetPvc, storageProfile, sc, csiDriver)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV is succeeded")
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, ExternalPopulationSucceeded) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Should not succeed if PVC has no DataSorceRef field (no AnyVolumeDataSource feature gate)", func() {
			dv := newPopulatorDataVolume("test-dv", nil, dataSourceRef)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})

			reconciler = createPopulatorReconciler(dv, targetPvc, storageProfile, sc, csiDriver)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV phase is unchanged")
			oldDVPhase := dv.Status.Phase
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(oldDVPhase))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, NoAnyVolumeDataSource) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Should not succeed if no CSI drivers are available", func() {
			dv := newPopulatorDataVolume("test-dv", nil, dataSourceRef)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.DataSourceRef = dv.Spec.PVC.DataSourceRef

			reconciler = createPopulatorReconciler(dv, targetPvc, storageProfile, sc)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV phase is unchanged")
			oldDVPhase := dv.Status.Phase
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(oldDVPhase))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, NoCSIDriverForExternalPopulation) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})
	})

	var _ = Describe("Legacy population", func() {
		// DataSources for PVC and Snapshot
		pvcDataSource := &corev1.TypedLocalObjectReference{
			Kind: "PersistentVolumeClaim",
			Name: "test",
		}
		snapshotAPIGroup := snapshotAPIName
		snapshotDataSource := &corev1.TypedLocalObjectReference{
			APIGroup: &snapshotAPIGroup,
			Kind:     "VolumeSnapshot",
			Name:     "test",
		}

		It("Should not panic if CSI Driver not available", func() {
			dv := newPopulatorDataVolume("test-dv", pvcDataSource, nil)
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.DataSource = dv.Spec.PVC.DataSource

			reconciler = createPopulatorReconciler(dv, srcPvc, targetPvc, storageProfile, sc)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV phase is unchanged")
			oldDVPhase := dv.Status.Phase
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(oldDVPhase))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, NoCSIDriverForExternalPopulation) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Should succeed by using PVC CSI clone as population method", func() {
			dv := newPopulatorDataVolume("test-dv", pvcDataSource, nil)
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.DataSource = dv.Spec.PVC.DataSource

			reconciler = createPopulatorReconciler(dv, srcPvc, targetPvc, storageProfile, sc, csiDriver)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV is succeeded")
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, ExternalPopulationSucceeded) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Succeeds using Snapshot as population method, even without CSI drivers", func() {
			dv := newPopulatorDataVolume("test-dv", snapshotDataSource, nil)
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.DataSource = dv.Spec.PVC.DataSource

			reconciler = createPopulatorReconciler(dv, srcPvc, targetPvc, storageProfile, sc)

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Verifying that DV is succeeded")
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, ExternalPopulationSucceeded) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})
	})
})

func createPopulatorReconciler(objects ...runtime.Object) *PopulatorReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createPopulatorReconcilerWithoutConfig(objs...)
}

func createPopulatorReconcilerWithoutConfig(objects ...runtime.Object) *PopulatorReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)
	extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &PopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvPopulatorLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
		},
	}
	r.Reconciler = r
	return r
}

func newPopulatorDataVolume(name string, dataSource *corev1.TypedLocalObjectReference, dataSourceRef *corev1.TypedObjectReference) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   metav1.NamespaceDefault,
			Annotations: map[string]string{},
			UID:         types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			PVC: &corev1.PersistentVolumeClaimSpec{
				DataSource:    dataSource,
				DataSourceRef: dataSourceRef,
				AccessModes:   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			},
		},
	}
}
