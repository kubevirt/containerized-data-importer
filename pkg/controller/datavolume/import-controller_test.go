/*
Copyright 2020 The CDI Authors.

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

const (
	testStorageClass = "test-sc"
)

var (
	dvImportLog = logf.Log.WithName("datavolume-import-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *ImportReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	var _ = Describe("Datavolume controller reconcile loop", func() {
		AfterEach(func() {
			if reconciler != nil && reconciler.recorder != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
			}
		})
		It("Should do nothing and return nil when no DV exists", func() {
			reconciler = createImportReconciler()
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).To(HaveOccurred())
			if !errors.IsNotFound(err) {
				Fail("Error getting pvc")
			}
		})

		DescribeTable("Should return nil when storage spec has no AccessModes and", func(scName *string) {
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: scName,
			}

			reconciler = createImportReconciler(importDataVolume)
			res, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{}))

			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring(MessageErrStorageClassNotFound))
		},
			Entry("no StorageClassName, and no default storage class set", nil),
			Entry("non-existing StorageClassName", ptr.To[string]("nosuch")),
		)

		It("Should create volumeImportSource if should use cdi populator", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			dv := NewImportDataVolume("test-dv")
			dv.Spec.ContentType = cdiv1.DataVolumeArchive
			preallocation := true
			dv.Spec.Preallocation = &preallocation
			reconciler = createImportReconciler(dv, sc, csiDriver)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))

			importSource := &cdiv1.VolumeImportSource{}
			importSourceName := volumeImportSourceName(dv)
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: importSourceName, Namespace: metav1.NamespaceDefault}, importSource)
			Expect(err).ToNot(HaveOccurred())
			Expect(importSource.Spec.Source).ToNot(BeNil())
			Expect(importSource.Spec.ContentType).To(Equal(dv.Spec.ContentType))
			Expect(importSource.Spec.Preallocation).To(Equal(dv.Spec.Preallocation))
			Expect(importSource.OwnerReferences).To(HaveLen(1))
			or := importSource.OwnerReferences[0]
			Expect(or.UID).To(Equal(dv.UID))
		})

		It("Should delete volumeImportSource if dv succeeded and we use cdi populator", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			dv := NewImportDataVolume("test-dv")
			importSourceName := volumeImportSourceName(dv)
			importSource := &cdiv1.VolumeImportSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      importSourceName,
					Namespace: dv.Namespace,
				},
			}
			reconciler = createImportReconciler(dv, sc, csiDriver, importSource)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())

			pvc.Annotations[AnnPodPhase] = string(corev1.PodSucceeded)
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Status().Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			deletedImportSource := &cdiv1.VolumeImportSource{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: importSourceName, Namespace: dv.Namespace}, deletedImportSource)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("Should create a PVC on a valid import DV", func() {
			dv := NewImportDataVolume("test-dv")
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
			val, ok := pvc.Annotations[AnnCreatedForDataVolume]
			Expect(ok).To(BeTrue())
			Expect(val).To(Equal(string(dv.UID)))
		})

		It("Should create a PVC on a valid import DV without delayed annotation then add on success", func() {
			dv := NewImportDataVolume("test-dv")
			AddAnnotation(dv, "foo", "bar")
			AddAnnotation(dv, AnnPopulatedFor, "true")
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Annotations["foo"]).To(Equal("bar"))
			Expect(pvc.Annotations).ToNot(HaveKey(AnnPopulatedFor))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Status().Update(context.Background(), dv)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Annotations["foo"]).To(Equal("bar"))
			Expect(pvc.Annotations[AnnPopulatedFor]).To(Equal("true"))
		})

		It("Should fail if dv source not import when use populators", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			dv := NewImportDataVolume("test-dv")
			dv.Spec.Source.HTTP = nil
			reconciler = createImportReconciler(dv, sc, csiDriver)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no source set for import datavolume"))
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("Should create a PVC with volumeImportSource when use populators", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			dv := NewImportDataVolume("test-dv")
			reconciler = createImportReconcilerWFFCDisabled(dv, sc, csiDriver)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
			Expect(pvc.Spec.DataSourceRef).ToNot(BeNil())
			importSourceName := volumeImportSourceName(dv)
			Expect(pvc.Spec.DataSourceRef.Name).To(Equal(importSourceName))
			Expect(pvc.Spec.DataSourceRef.Kind).To(Equal(cdiv1.VolumeImportSourceRef))
			Expect(pvc.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))
			_, annExists := pvc.Annotations[AnnImmediateBinding]
			Expect(annExists).To(BeTrue())
		})

		It("Should report import population progress when use populators", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			dv := NewImportDataVolume("test-dv")
			reconciler = createImportReconciler(dv, sc, csiDriver)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(dv.Status.Progress)).To(Equal("N/A"))

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))

			AddAnnotation(pvc, AnnPopulatorProgress, "13.45%")
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Progress).To(BeEquivalentTo("13.45%"))
		})

		It("Should pass labels from DV to PVC", func() {
			dv := NewImportDataVolume("test-dv")
			dv.Labels = map[string]string{}
			for _, defaultInstanceTypeLabel := range DefaultInstanceTypeLabels {
				dv.Labels[defaultInstanceTypeLabel] = defaultInstanceTypeLabel
			}
			dv.Labels[LabelDynamicCredentialSupport] = "true"

			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())

			Expect(pvc.Name).To(Equal("test-dv"))
			for _, defaultInstanceTypeLabel := range DefaultInstanceTypeLabels {
				Expect(pvc.Labels).To(HaveKeyWithValue(defaultInstanceTypeLabel, defaultInstanceTypeLabel))
			}
			Expect(pvc.Labels).To(HaveKeyWithValue(LabelDynamicCredentialSupport, "true"))
		})

		It("Should set params on a PVC from import DV.PVC", func() {
			importDataVolume := NewImportDataVolume("test-dv")
			importDataVolume.Spec.PVC.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			importDataVolume.Spec.PVC.VolumeMode = &BlockMode

			defaultStorageClass := CreateStorageClass("defaultSc", map[string]string{AnnDefaultStorageClass: "true"})
			reconciler = createImportReconciler(defaultStorageClass, importDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(pvc.Spec.StorageClassName).To(BeNil())
			Expect(pvc.Spec.VolumeMode).ToNot(BeNil())
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
		})

		It("Should explicitly set computed storageClassName on a PVC, when not provided in dv", func() {
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			// spec with accessMode/VolumeMode so storageprofile is not needed
			importDataVolume.Spec.Storage = createStorageSpec()
			defaultStorageClass := CreateStorageClass("defaultSc", map[string]string{AnnDefaultStorageClass: "true"})
			reconciler = createImportReconciler(defaultStorageClass, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Spec.StorageClassName).To(HaveValue(Equal("defaultSc")))
		})

		It("Should set params on a PVC from import DV.Storage", func() {
			// spec with accessMode/VolumeMode so storageprofile is not needed
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = createStorageSpec()
			defaultStorageClass := CreateStorageClass("defaultSc", map[string]string{AnnDefaultStorageClass: "true"})
			reconciler = createImportReconciler(defaultStorageClass, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(pvc.Spec.VolumeMode).ToNot(BeNil())
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
			Expect(pvc.Spec.StorageClassName).To(HaveValue(Equal("defaultSc")))
		})

		It("Should fail on missing size, without storageClass", func() {
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			// spec with accessMode/VolumeMode so storageprofile is not needed
			importDataVolume.Spec.Storage = createStorageSpec()
			importDataVolume.Spec.Storage.Resources = corev1.VolumeResourceRequirements{}
			defaultStorageClass := CreateStorageClass("defaultSc", map[string]string{AnnDefaultStorageClass: "true"})
			reconciler = createImportReconciler(defaultStorageClass, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing storage size"))
		})

		It("Should fail on missing size, with StorageClass", func() {
			storageClassName := "defaultSc"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			// spec with accessMode/VolumeMode so storageprofile is not needed
			importDataVolume.Spec.Storage = createStorageSpec()
			importDataVolume.Spec.Storage.Resources = corev1.VolumeResourceRequirements{}
			importDataVolume.Spec.Storage.StorageClassName = &storageClassName
			defaultStorageClass := CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"})
			reconciler = createImportReconciler(defaultStorageClass, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing storage size"))
		})

		DescribeTable("Should set params on a PVC from storageProfile when import DV has no accessMode and no volume mode", func(contentType cdiv1.DataVolumeContentType) {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.ContentType = contentType
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			storageClass := CreateStorageClass(scName, nil)
			claimPropertySets := []cdiv1.ClaimPropertySet{
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
			}
			storageProfile := createStorageProfileWithClaimPropertySets(scName, claimPropertySets)

			reconciler = createImportReconciler(storageClass, storageProfile, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			if contentType == cdiv1.DataVolumeKubeVirt {
				Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadOnlyMany))
				Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
			} else {
				Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
				Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
			}
		},

			Entry("Kubevirt contentType", cdiv1.DataVolumeKubeVirt),
			Entry("Archive contentType", cdiv1.DataVolumeArchive),
		)

		It("Should fail if DV with archive content type has volume mode block", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				VolumeMode:       &BlockMode,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			storageClass := CreateStorageClass(scName, nil)
			claimPropertySets := []cdiv1.ClaimPropertySet{
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
			}
			storageProfile := createStorageProfileWithClaimPropertySets(scName, claimPropertySets)

			reconciler = createImportReconciler(storageClass, storageProfile, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ContentType Archive cannot have block volumeMode"))
			By("Checking error event recorded")
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("ContentType Archive cannot have block volumeMode"))
		})

		It("Should set on a PVC matching access mode from storageProfile to the DV given volume mode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				VolumeMode:       &FilesystemMode,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			storageClass := CreateStorageClass(scName, nil)

			claimPropertySets := []cdiv1.ClaimPropertySet{
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
			}
			storageProfile := createStorageProfileWithClaimPropertySets(scName, claimPropertySets)

			reconciler = createImportReconciler(storageClass, storageProfile, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set on a PVC matching access mode from storageProfile to the DV given contentType archive", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			importDataVolume.Spec.ContentType = cdiv1.DataVolumeArchive

			storageClass := CreateStorageClass(scName, nil)

			// First is RWX / block, but because of the contentType DataVolumeArchive, the volumeMode should be fs,
			// and the matched accessMode is RWO
			claimPropertySets := []cdiv1.ClaimPropertySet{
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
			}
			storageProfile := createStorageProfileWithClaimPropertySets(scName, claimPropertySets)
			reconciler = createImportReconciler(storageClass, storageProfile, importDataVolume)

			// actual test
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set on a PVC matching volume mode from storageProfile to the given DV access mode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			storageClass := CreateStorageClass(scName, nil)

			claimPropertySets := []cdiv1.ClaimPropertySet{
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, VolumeMode: &BlockMode},
				{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
			}
			storageProfile := createStorageProfileWithClaimPropertySets(scName, claimPropertySets)

			reconciler = createImportReconciler(storageClass, storageProfile, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set params on a PVC from correct storageProfile when import DV has no accessMode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}
			storageClass := CreateStorageClass(scName, nil)
			storageProfile := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			defaultStorageClass := CreateStorageClass("defaultSc", map[string]string{AnnDefaultStorageClass: "true"})
			defaultStorageProfile := createStorageProfile("defaultSc", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, FilesystemMode)

			reconciler = createImportReconciler(defaultStorageClass, storageClass, storageProfile, defaultStorageProfile, importDataVolume)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadOnlyMany))
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
		})

		It("Should set params on a PVC from default storageProfile when import DV has no storageClass and no accessMode", func() {
			cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
			cdiConfig.Status = cdiv1.CDIConfigStatus{
				ScratchSpaceStorageClass: testStorageClass,
				FilesystemOverhead: &cdiv1.FilesystemOverhead{
					Global: cdiv1.Percent("0.5"),
				},
			}

			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			}

			storageClass := CreateStorageClass(scName, map[string]string{AnnDefaultStorageClass: "true"})
			storageProfile := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			anotherStorageProfile := createStorageProfile("anotherSp", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, FilesystemMode)

			reconciler = createImportReconcilerWithoutConfig(
				storageClass,
				storageProfile,
				anotherStorageProfile,
				importDataVolume,
				cdiConfig)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			Expect(pvc.Spec.AccessModes).To(HaveLen(1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadOnlyMany))
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
			expectedSize := resource.MustParse("1G")
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
		})

		It("Should pass annotations and labels from DV to created PVC", func() {
			dv := NewImportDataVolume("test-dv")
			dv.SetAnnotations(make(map[string]string))
			dv.GetAnnotations()["test-ann-1"] = "test-value-1"
			dv.GetAnnotations()["test-ann-2"] = "test-value-2"
			dv.GetAnnotations()[AnnSource] = "invalid phase should not copy"
			dv.GetAnnotations()[AnnPodNetwork] = "data-network"
			dv.GetAnnotations()[AnnPodSidecarInjectionIstio] = "false"
			dv.GetAnnotations()[AnnPodSidecarInjectionLinkerd] = "false"
			dv.SetLabels(make(map[string]string))
			dv.GetLabels()["test"] = "test-label"
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.GetAnnotations()).ToNot(BeNil())
			Expect(pvc.GetAnnotations()["test-ann-1"]).To(Equal("test-value-1"))
			Expect(pvc.GetAnnotations()["test-ann-2"]).To(Equal("test-value-2"))
			Expect(pvc.GetAnnotations()[AnnSource]).To(Equal(SourceHTTP))
			Expect(pvc.GetAnnotations()[AnnPodNetwork]).To(Equal("data-network"))
			Expect(pvc.GetAnnotations()[AnnPodSidecarInjectionIstio]).To(Equal("false"))
			Expect(pvc.GetAnnotations()[AnnPodSidecarInjectionLinkerd]).To(Equal("false"))
			Expect(pvc.GetAnnotations()[AnnPriorityClassName]).To(Equal("p0"))
			Expect(pvc.Labels["test"]).To(Equal("test-label"))
		})

		It("Should pass annotation from DV with S3 source to created a PVC on a DV", func() {
			dv := newS3ImportDataVolume("test-dv")
			dv.SetAnnotations(make(map[string]string))
			dv.GetAnnotations()["test-ann-1"] = "test-value-1"
			dv.GetAnnotations()["test-ann-2"] = "test-value-2"
			dv.GetAnnotations()[AnnSource] = "invalid phase should not copy"
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.GetAnnotations()).ToNot(BeNil())
			Expect(pvc.GetAnnotations()["test-ann-1"]).To(Equal("test-value-1"))
			Expect(pvc.GetAnnotations()["test-ann-2"]).To(Equal("test-value-2"))
			Expect(pvc.GetAnnotations()[AnnSource]).To(Equal(SourceS3))
			Expect(pvc.GetAnnotations()[AnnPriorityClassName]).To(Equal("p0-s3"))
		})

		It("Should follow the phase of the created PVC", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(BeEquivalentTo(""))

			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Pending))
		})

		It("Should follow the restarts of the PVC", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.RestartCount).To(Equal(int32(0)))

			pvc.Annotations[AnnPodRestarts] = "2"
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.RestartCount).To(Equal(int32(2)))
		})

		It("Should error if a PVC with same name already exists that is not owned by us", func() {
			reconciler = createImportReconciler(CreatePvc("test-dv", metav1.NamespaceDefault, map[string]string{}, nil), NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(HaveOccurred())
			By("Checking error event recorded")
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("Resource \"test-dv\" already exists and is not managed by DataVolume"))
		})

		It("Should add owner to pre populated PVC", func() {
			annotations := map[string]string{"cdi.kubevirt.io/storage.populatedFor": "test-dv"}
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, annotations, nil)
			pvc.Status.Phase = corev1.ClaimBound
			dv := NewImportDataVolume("test-dv")
			reconciler = createImportReconciler(pvc, dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			or := pvc.OwnerReferences[0]
			Expect(or.UID).To(Equal(dv.UID))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Annotations["cdi.kubevirt.io/storage.prePopulated"]).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(string(dv.Status.Progress)).To(Equal("N/A"))
		})

		It("Should adopt a PVC (with annotation)", func() {
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, nil, nil)
			pvc.Status.Phase = corev1.ClaimBound
			dv := NewImportDataVolume("test-dv")
			AddAnnotation(dv, AnnAllowClaimAdoption, "true")
			reconciler = createImportReconciler(pvc, dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			or := pvc.OwnerReferences[0]
			Expect(or.UID).To(Equal(dv.UID))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(string(dv.Status.Progress)).To(Equal("N/A"))
			_, ok := pvc.Annotations[AnnCreatedForDataVolume]
			Expect(ok).To(BeFalse())
		})

		It("Should adopt a unbound PVC (with annotation)", func() {
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, nil, nil)
			pvc.Spec.VolumeName = ""
			pvc.Status.Phase = corev1.ClaimPending
			dv := NewImportDataVolume("test-dv")
			AddAnnotation(dv, AnnAllowClaimAdoption, "true")
			reconciler = createImportReconciler(pvc, dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			or := pvc.OwnerReferences[0]
			Expect(or.UID).To(Equal(dv.UID))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(string(dv.Status.Progress)).To(Equal("N/A"))
			_, ok := pvc.Annotations[AnnCreatedForDataVolume]
			Expect(ok).To(BeFalse())
		})

		It("Should adopt a PVC (with featuregate)", func() {
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, nil, nil)
			pvc.Status.Phase = corev1.ClaimBound
			dv := NewImportDataVolume("test-dv")
			featureGates := []string{featuregates.DataVolumeClaimAdoption, featuregates.HonorWaitForFirstConsumer}
			reconciler = createImportReconcilerWithFeatureGates(featureGates, pvc, dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			or := pvc.OwnerReferences[0]
			Expect(or.UID).To(Equal(dv.UID))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(string(dv.Status.Progress)).To(Equal("N/A"))
			_, ok := pvc.Annotations[AnnCreatedForDataVolume]
			Expect(ok).To(BeFalse())
		})

		It("Should set multistage migration annotations on a newly created PVC", func() {
			dv := NewImportDataVolume("test-dv")
			dv.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{
					Previous: "previous",
					Current:  "current",
				},
			}

			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.GetAnnotations()).ToNot(BeNil())
			Expect(pvc.GetAnnotations()[AnnPreviousCheckpoint]).To(Equal("previous"))
			Expect(pvc.GetAnnotations()[AnnCurrentCheckpoint]).To(Equal("current"))
			Expect(pvc.GetAnnotations()[AnnFinalCheckpoint]).To(Equal("false"))
		})

		It("Should set multistage migration annotations on an existing PVC if they're not set", func() {
			annotations := map[string]string{AnnPopulatedFor: "test-dv"}
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, annotations, nil)
			pvc.Status.Phase = corev1.ClaimBound

			dv := NewImportDataVolume("test-dv")
			dv.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{
					Previous: "previous",
					Current:  "current",
				},
			}
			dv.Spec.FinalCheckpoint = true

			reconciler = createImportReconciler(dv, pvc)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.GetAnnotations()).ToNot(BeNil())
			Expect(pvc.GetAnnotations()[AnnPreviousCheckpoint]).To(Equal("previous"))
			Expect(pvc.GetAnnotations()[AnnCurrentCheckpoint]).To(Equal("current"))
			Expect(pvc.GetAnnotations()[AnnFinalCheckpoint]).To(Equal("true"))
		})

		It("Should not set multistage migration annotations on an existing PVC if they're already set", func() {
			annotations := map[string]string{
				AnnPopulatedFor:       "test-dv",
				AnnPreviousCheckpoint: "oldPrevious",
				AnnCurrentCheckpoint:  "oldCurrent",
				AnnFinalCheckpoint:    "true",
			}
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, annotations, nil)
			pvc.Status.Phase = corev1.ClaimBound

			dv := NewImportDataVolume("test-dv")
			dv.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{
					Previous: "newPrevious",
					Current:  "newCurrent",
				},
			}
			dv.Spec.FinalCheckpoint = false

			reconciler = createImportReconciler(dv, pvc)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.GetAnnotations()).ToNot(BeNil())
			Expect(pvc.GetAnnotations()[AnnPreviousCheckpoint]).To(Equal("oldPrevious"))
			Expect(pvc.GetAnnotations()[AnnCurrentCheckpoint]).To(Equal("oldCurrent"))
			Expect(pvc.GetAnnotations()[AnnFinalCheckpoint]).To(Equal("true"))
		})

		DescribeTable("After successful checkpoint copy", func(finalCheckpoint bool, modifyAnnotations func(annotations map[string]string), validate func(pv *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume)) {
			annotations := map[string]string{
				AnnPreviousCheckpoint: "previous",
				AnnCurrentCheckpoint:  "current",
				AnnFinalCheckpoint:    strconv.FormatBool(finalCheckpoint),
				AnnPodPhase:           string(cdiv1.Succeeded),
				AnnCurrentPodID:       "12345678",
			}
			annotations[AnnCheckpointsCopied+"."+"first"] = "12345"
			annotations[AnnCheckpointsCopied+"."+"second"] = "123456"
			annotations[AnnCheckpointsCopied+"."+"previous"] = "1234567"
			annotations[AnnCheckpointsCopied+"."+"current"] = "12345678"
			if modifyAnnotations != nil {
				modifyAnnotations(annotations)
			}
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, annotations, nil)
			pvc.Status.Phase = corev1.ClaimBound

			dv := NewImportDataVolume("test-dv")
			dv.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{
					Previous: "",
					Current:  "first",
				},
				{
					Previous: "first",
					Current:  "second",
				},
				{
					Previous: "second",
					Current:  "previous",
				},
				{
					Previous: "previous",
					Current:  "current",
				},
			}
			dv.Spec.FinalCheckpoint = finalCheckpoint

			pvc.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: "cdi.kubevirt.io/v1beta1",
					Kind:       "DataVolume",
					Name:       dv.Name,
					UID:        dv.UID,
					Controller: ptr.To(true),
				},
			}

			reconciler = createImportReconciler(dv, pvc)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			newPvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, newPvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(newPvc.Name).To(Equal("test-dv"))

			newDv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, newDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(newDv.Name).To(Equal("test-dv"))
			validate(newPvc, newDv)
		},
			Entry("should move to 'Paused' if non-final checkpoint", false, nil, func(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
				Expect(dv.Status.Phase).To(Equal(cdiv1.Paused))
			}),
			Entry("should move to 'Succeeded' if final checkpoint", true, nil, func(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
				// Extra reconcile to move from final Paused to Succeeded
				reconciler = createImportReconciler(dv, pvc)
				_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				newDv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, newDv)
				Expect(err).ToNot(HaveOccurred())
				Expect(newDv.Name).To(Equal("test-dv"))
				Expect(newDv.Status.Phase).To(Equal(cdiv1.Succeeded))
			}),
			Entry("should clear multistage migration annotations after copying the final checkpoint", true, nil, func(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
				_, ok := pvc.GetAnnotations()[AnnCurrentCheckpoint]
				Expect(ok).To(BeFalse())
				_, ok = pvc.GetAnnotations()[AnnPreviousCheckpoint]
				Expect(ok).To(BeFalse())
				_, ok = pvc.GetAnnotations()[AnnFinalCheckpoint]
				Expect(ok).To(BeFalse())
				_, ok = pvc.GetAnnotations()[AnnCurrentPodID]
				Expect(ok).To(BeFalse())
				_, ok = pvc.GetAnnotations()[AnnCheckpointsCopied+".current"]
				Expect(ok).To(BeFalse())
			}),
			Entry("should add a final 'done' annotation for overall multi-stage import", true, nil, func(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
				Expect(pvc.GetAnnotations()[AnnMultiStageImportDone]).To(Equal("true"))
			}),
			Entry("should advance exactly one checkpoint after one delta copy", false, func(annotations map[string]string) {
				delete(annotations, AnnCheckpointsCopied+"."+"previous")
				delete(annotations, AnnCheckpointsCopied+"."+"current")
				annotations[AnnCurrentCheckpoint] = "previous"
				annotations[AnnCurrentPodID] = "1234567"
			}, func(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
				Expect(pvc.GetAnnotations()[AnnCurrentCheckpoint]).To(Equal("current"))
			}),
		)

		It("Should get VDDK info annotations from PVC", func() {
			dv := NewImportDataVolume("test-dv")
			annotations := map[string]string{
				AnnVddkHostConnection: "esx1.test",
				AnnVddkVersion:        "1.3.4",
				AnnSource:             SourceVDDK,
				AnnPopulatedFor:       "test-dv",
			}
			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, annotations, nil)

			reconciler = createImportReconciler(dv, pvc)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			newDv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, newDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(newDv.GetAnnotations()[AnnVddkHostConnection]).To(Equal("esx1.test"))
			Expect(newDv.GetAnnotations()[AnnVddkVersion]).To(Equal("1.3.4"))
		})

		It("Should add VDDK image URL to PVC", func() {
			dv := newVDDKDataVolume("test-dv")
			dv.Spec.Source.VDDK.InitImageURL = "test://image"
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc).ToNot(BeNil())
			Expect(pvc.GetAnnotations()[AnnVddkInitImageURL]).To(Equal("test://image"))
		})

		It("Should copy extra VDDK args to PVC", func() {
			dv := newVDDKDataVolume("test-dv")
			dv.Spec.Source.VDDK.ExtraArgs = "vddk-extra-args"
			reconciler = createImportReconciler(dv)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc).ToNot(BeNil())
			Expect(pvc.GetAnnotations()[AnnVddkExtraArgs]).To(Equal("vddk-extra-args"))
		})
	})

	var _ = Describe("Reconcile Datavolume status", func() {
		DescribeTable("if no pvc exists", func(current, expected cdiv1.DataVolumePhase) {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			dv.Status.Phase = current
			err = reconciler.client.Status().Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Delete(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(expected))
			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("No PVC found"))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "No PVC found") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		},
			Entry("should become pending", cdiv1.PhaseUnset, cdiv1.Pending),
			Entry("should remain pending", cdiv1.Pending, cdiv1.Pending),
			Entry("should remain inprogress", cdiv1.ImportInProgress, cdiv1.ImportInProgress),
		)

		It("Should switch to pending if PVC phase is pending", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Pending))
			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "PVC test-dv Pending") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should set DV phase to WaitForFirstConsumer if storage class is WFFC", func() {
			scName := "default_test_sc"
			sc := createStorageClassWithBindingMode(scName,
				map[string]string{
					AnnDefaultStorageClass: "true",
				},
				storagev1.VolumeBindingWaitForFirstConsumer)
			reconciler = createImportReconciler(sc, NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "PVC test-dv Pending") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should set DV phase to WaitForFirstConsumer if storage class on PVC is WFFC", func() {
			scName := "pvc_sc_wffc"
			scDefault := CreateStorageClass("default_test_sc", map[string]string{
				AnnDefaultStorageClass: "true",
			})
			scWffc := createStorageClassWithBindingMode(scName, map[string]string{}, storagev1.VolumeBindingWaitForFirstConsumer)
			importDataVolume := NewImportDataVolume("test-dv")
			importDataVolume.Spec.PVC.StorageClassName = &scName

			reconciler = createImportReconciler(scDefault, scWffc, importDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "PVC test-dv Pending") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})
		It("Should set DV phase to pendingPopulation if use populators with storage class WFFC", func() {
			scName := "pvc_sc_wffc"
			bindingMode := storagev1.VolumeBindingWaitForFirstConsumer
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			sc.VolumeBindingMode = &bindingMode
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			importDataVolume := NewImportDataVolume("test-dv")
			importDataVolume.Spec.PVC.StorageClassName = &scName

			reconciler = createImportReconciler(sc, csiDriver, importDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.PendingPopulation))

			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "PVC test-dv Pending") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should set DV phase to ImportScheduled if use populators wffc storage class after scheduled node", func() {
			scName := "pvc_sc_wffc"
			bindingMode := storagev1.VolumeBindingWaitForFirstConsumer
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			sc.VolumeBindingMode = &bindingMode
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			importDataVolume := NewImportDataVolume("test-dv")
			importDataVolume.Spec.PVC.StorageClassName = &scName

			reconciler = createImportReconciler(sc, csiDriver, importDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			AddAnnotation(pvc, AnnSelectedNode, "node01")
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			// Creating a valid PVC Prime
			pvcPrime := &corev1.PersistentVolumeClaim{}
			pvcPrime.Name = populators.PVCPrimeName(pvc)
			pvcPrime.Namespace = metav1.NamespaceDefault
			pvcPrime.Status.Phase = corev1.ClaimBound
			pvcPrime.SetAnnotations(make(map[string]string))
			pvcPrime.GetAnnotations()[AnnImportPod] = "something"
			err = reconciler.client.Create(context.TODO(), pvcPrime)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.updateStatus(getReconcileRequest(importDataVolume), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.ImportScheduled))

			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "PVC test-dv Pending") {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should not update DV phase when PVC Prime is unbound", func() {
			scName := "testSC"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			importDataVolume := NewImportDataVolume("test-dv")
			importDataVolume.Spec.PVC.StorageClassName = &scName

			reconciler = createImportReconciler(sc, csiDriver, importDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			// Get original DV phase
			dvPhase := dv.Status.Phase

			// Create PVC Prime
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			pvcPrime := &corev1.PersistentVolumeClaim{}
			pvcPrime.Name = populators.PVCPrimeName(pvc)
			pvcPrime.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Create(context.TODO(), pvcPrime)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.updateStatus(getReconcileRequest(importDataVolume), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(dvPhase))
		})

		It("Should switch to succeeded if PVC phase is pending, but pod phase is succeeded", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			By("Checking error event recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			foundSuccess := false
			foundPending := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "Successfully imported into PVC test-dv") {
					foundSuccess = true
				}
				if strings.Contains(event, "PVC test-dv Pending") {
					foundPending = true
				}
			}
			Expect(foundSuccess).To(BeTrue())
			Expect(foundPending).To(BeTrue())
			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			readyCondition := FindConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
			Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
			Expect(readyCondition.Message).To(Equal(""))
		})

		It("Should switch to paused if pod phase is succeeded but a checkpoint is set", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnCurrentCheckpoint] = "current"
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			pvc.Status.Phase = corev1.ClaimPending
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Paused))
			By("Checking error event recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			foundPaused := false
			foundPending := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, "Multistage import into PVC test-dv is paused") {
					foundPaused = true
				}
				if strings.Contains(event, "PVC test-dv Pending") {
					foundPending = true
				}
			}
			Expect(foundPaused).To(BeTrue())
			Expect(foundPending).To(BeTrue())
			Expect(dv.Status.Conditions).To(HaveLen(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			readyCondition := FindConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
			Expect(readyCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(readyCondition.Message).To(Equal(""))
		})

		DescribeTable("DV phase", func(testDv client.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			// First we test the non-populator flow
			scName := "testpvc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)
			r := createImportReconciler(testDv, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent, extraAnnotations...)

			// Test the populator flow, it should match
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			// Creating a valid PVC Prime
			pvcPrime := &corev1.PersistentVolumeClaim{}
			pvcPrime.Name = "prime-"
			pvcPrime.Namespace = metav1.NamespaceDefault
			pvcPrime.Status.Phase = corev1.ClaimBound
			pvcPrime.SetAnnotations(make(map[string]string))
			pvcPrime.GetAnnotations()[ann] = "something"
			pvcPrime.GetAnnotations()[AnnPodPhase] = string(podPhase)
			for i := 0; i < len(extraAnnotations); i += 2 {
				pvcPrime.GetAnnotations()[extraAnnotations[i]] = extraAnnotations[i+1]
			}
			r = createImportReconciler(testDv, sc, storageProfile, pvcPrime, csiDriver)
			dvPhaseTest(r.ReconcilerBase, r, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent, extraAnnotations...)
		},
			Entry("should switch to bound for import", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.PVCBound, corev1.ClaimBound, corev1.PodPending, "invalid", "PVC test-dv Bound", AnnPriorityClassName, "p0"),
			Entry("should switch to bound for import", NewImportDataVolume("test-dv"), cdiv1.Unknown, cdiv1.PVCBound, corev1.ClaimBound, corev1.PodPending, "invalid", "PVC test-dv Bound", AnnPriorityClassName, "p0"),
			Entry("should switch to scheduled for import", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodPending, AnnImportPod, "Import into test-dv scheduled", AnnPriorityClassName, "p0"),
			Entry("should switch to inprogress for import", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportInProgress, corev1.ClaimBound, corev1.PodRunning, AnnImportPod, "Import into test-dv in progress", AnnPriorityClassName, "p0"),
			Entry("should stay the same for import after pod fails", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodFailed, AnnImportPod, "Failed to import into PVC test-dv", AnnPriorityClassName, "p0"),
			Entry("should switch to failed on claim lost for impot", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnImportPod, "PVC test-dv lost", AnnPriorityClassName, "p0"),
			Entry("should switch to succeeded for import", NewImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnImportPod, "Successfully imported into PVC test-dv", AnnPriorityClassName, "p0"),
			Entry("should switch to scheduled for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodPending, AnnImportPod, "Import into test-dv scheduled", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to inprogress for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportInProgress, corev1.ClaimBound, corev1.PodRunning, AnnImportPod, "Import into test-dv in progress"),
			Entry("should stay the same for blank after pod fails", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodFailed, AnnImportPod, "Failed to import into PVC test-dv"),
			Entry("should switch to failed on claim lost for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnImportPod, "PVC test-dv lost"),
			Entry("should switch to succeeded for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnImportPod, "Successfully imported into PVC test-dv"),
		)
	})
	var _ = Describe("Get Pod from PVC", func() {
		var (
			pvc *corev1.PersistentVolumeClaim
		)
		BeforeEach(func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc = &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return error if no pods can be found", func() {
			_, err := GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
		})

		It("Should return pod if pods can be found based on owner ref", func() {
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			foundPod, err := GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundPod.Name).To(Equal(pod.Name))
		})

		It("Should return pod if pods can be found based on cloneid", func() {
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			pod.GetLabels()[CloneUniqueID] = string(pvc.GetUID()) + "-source-pod"
			pod.OwnerReferences = nil
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			foundPod, err := GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundPod.Name).To(Equal(pod.Name))
		})

		It("Should return error if pods can be found but cloneid doesn't match", func() {
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			pod.GetLabels()[CloneUniqueID] = string(pvc.GetUID()) + "-source-p"
			pod.OwnerReferences = nil
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			_, err = GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
		})

		It("Should ignore completed pods from a multi-stage migration, when retainAfterCompletion is set", func() {
			pvc.Annotations[AnnCurrentCheckpoint] = "test-checkpoint"
			pvc.Annotations[AnnPodRetainAfterCompletion] = "true"
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			pod.Status.Phase = corev1.PodSucceeded
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			foundPod, err := GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).To(HaveOccurred())
			Expect(foundPod).To(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
		})

		It("Should not ignore completed pods from a multi-stage migration, when retainAfterCompletion is not set", func() {
			pvc.Annotations[AnnCurrentCheckpoint] = "test-checkpoint"
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			pod.Status.Phase = corev1.PodSucceeded
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			foundPod, err := GetPodFromPvc(reconciler.client, metav1.NamespaceDefault, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundPod).ToNot(BeNil())
		})
	})

	var _ = Describe("Update Progress from pod", func() {
		var (
			pvc *corev1.PersistentVolumeClaim
			pod *corev1.Pod
			dv  *cdiv1.DataVolume
		)

		BeforeEach(func() {
			pvc = CreatePvc("test", metav1.NamespaceDefault, nil, nil)
			pod = CreateImporterTestPod(pvc, "test", nil)
			dv = NewImportDataVolume("test")
		})

		It("Should return error, if no metrics port in pod", func() {
			pod.Spec.Containers[0].Ports = nil
			err := updateProgressUsingPod(dv, pod)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Metrics port not found in pod"))
		})

		It("Should not error, if no endpoint exists", func() {
			pod.Spec.Containers[0].Ports[0].ContainerPort = 12345
			pod.Status.PodIP = "127.0.0.1"
			err := updateProgressUsingPod(dv, pod)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should properly update progress if http endpoint returns matching data", func() {
			dv.SetUID("b856691e-1038-11e9-a5ab-525500d15501")
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, "kubevirt_cdi_import_progress_total{ownerUID=\"%v\"} 13.45", dv.GetUID()) // ignore error here
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.ParseInt(ep.Port(), 10, 32)
			Expect(err).ToNot(HaveOccurred())
			pod.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			pod.Status.PodIP = ep.Hostname()
			err = updateProgressUsingPod(dv, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Progress).To(BeEquivalentTo("13.45%"))
		})

		It("Should not change update progress if http endpoint returns no matching data", func() {
			dv.SetUID("b856691e-1038-11e9-a5ab-525500d15501")
			dv.Status.Progress = cdiv1.DataVolumeProgress("2.3%")
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(fmt.Sprintf("import_progress{ownerUID=\"%v\"} 13.45", "b856691e-1038-11e9-a5ab-55500d15501")))
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.ParseInt(ep.Port(), 10, 32)
			Expect(err).ToNot(HaveOccurred())
			pod.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			pod.Status.PodIP = ep.Hostname()
			err = updateProgressUsingPod(dv, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Progress).To(BeEquivalentTo("2.3%"))
		})
	})

	var _ = Describe("shouldUseCDIPopulator", func() {
		scName := "test"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}

		DescribeTable("Should return expected result if has annotation", func(annotation, value string, expected bool) {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{},
			}
			storageSpec := &cdiv1.StorageSpec{}
			dv := createDataVolumeWithStorageAPI("test-dv", metav1.NamespaceDefault, httpSource, storageSpec)
			AddAnnotation(dv, annotation, value)

			reconciler = createImportReconciler(sc, csiDriver)
			syncState := dvSyncState{
				dvMutated: dv,
				pvcSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName,
				},
			}
			usePopulator, err := reconciler.shouldUseCDIPopulator(&syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(usePopulator).To(Equal(expected))
		},
			Entry("AnnUsePopulator=true return true", AnnUsePopulator, "true", true),
			Entry("AnnUsePopulator=false return false", AnnUsePopulator, "false", false),
			Entry("AnnPodRetainAfterCompletion return true", AnnPodRetainAfterCompletion, "true", true),
		)

		It("Should return true if storage class has wffc bindingMode and honorWaitForFirstConsumer feature gate is disabled", func() {
			sc := createStorageClassWithBindingMode(scName,
				map[string]string{
					AnnDefaultStorageClass: "true",
				},
				storagev1.VolumeBindingWaitForFirstConsumer)
			sc.Provisioner = "csi-plugin"
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{},
			}
			storageSpec := &cdiv1.StorageSpec{}
			dv := createDataVolumeWithStorageAPI("test-dv", metav1.NamespaceDefault, httpSource, storageSpec)

			reconciler = createImportReconcilerWFFCDisabled(sc, csiDriver)
			syncState := dvSyncState{
				dvMutated: dv,
				pvcSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName,
				},
			}
			usePopulator, err := reconciler.shouldUseCDIPopulator(&syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(usePopulator).To(BeTrue())
		})

		It("Should return false if storage class doesnt exist", func() {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{},
			}
			storageSpec := &cdiv1.StorageSpec{}
			dv := createDataVolumeWithStorageAPI("test-dv", metav1.NamespaceDefault, httpSource, storageSpec)

			reconciler = createImportReconciler()
			syncState := dvSyncState{
				dvMutated: dv,
				pvcSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName,
				},
			}
			usePopulator, err := reconciler.shouldUseCDIPopulator(&syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(usePopulator).To(BeFalse())
		})

		It("Should return false if storage class doesnt have csi driver", func() {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{},
			}
			storageSpec := &cdiv1.StorageSpec{}
			dv := createDataVolumeWithStorageAPI("test-dv", metav1.NamespaceDefault, httpSource, storageSpec)

			reconciler = createImportReconciler(sc)
			syncState := dvSyncState{
				dvMutated: dv,
				pvcSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName,
				},
			}
			usePopulator, err := reconciler.shouldUseCDIPopulator(&syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(usePopulator).To(BeFalse())
		})

		It("Should return true if storage class has csi driver", func() {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{},
			}
			storageSpec := &cdiv1.StorageSpec{}
			dv := createDataVolumeWithStorageAPI("test-dv", metav1.NamespaceDefault, httpSource, storageSpec)

			reconciler = createImportReconciler(sc, csiDriver)
			syncState := dvSyncState{
				dvMutated: dv,
				pvcSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName,
				},
			}
			usePopulator, err := reconciler.shouldUseCDIPopulator(&syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(usePopulator).To(BeTrue())
		})
	})

})

func dvPhaseTest(reconciler ReconcilerBase, dvc dvController, testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
	_, err := dvc.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
	Expect(err).ToNot(HaveOccurred())
	dv := &cdiv1.DataVolume{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
	Expect(err).ToNot(HaveOccurred())
	dv.Status.Phase = current
	err = reconciler.client.Status().Update(context.TODO(), dv)
	Expect(err).ToNot(HaveOccurred())

	pvc := &corev1.PersistentVolumeClaim{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
	Expect(err).ToNot(HaveOccurred())
	Expect(pvc.Name).To(Equal("test-dv"))
	pvc.SetAnnotations(make(map[string]string))
	pvc.GetAnnotations()[ann] = "something"
	pvc.GetAnnotations()[AnnPodPhase] = string(podPhase)
	for i := 0; i < len(extraAnnotations); i += 2 {
		pvc.GetAnnotations()[extraAnnotations[i]] = extraAnnotations[i+1]
	}
	err = reconciler.client.Update(context.TODO(), pvc)
	Expect(err).ToNot(HaveOccurred())
	pvc.Status.Phase = pvcPhase
	err = reconciler.client.Status().Update(context.TODO(), pvc)
	Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.updateStatus(getReconcileRequest(dv), nil, dvc)
	Expect(err).ToNot(HaveOccurred())

	dv = &cdiv1.DataVolume{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
	Expect(err).ToNot(HaveOccurred())
	Expect(dv.Status.Phase).To(Equal(expected))
	Expect(dv.Status.Conditions).To(HaveLen(3))
	boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
	Expect(boundCondition.Status).To(Equal(boundStatusByPVCPhase(pvcPhase)))
	Expect(boundCondition.Message).To(Equal(boundMessageByPVCPhase(pvcPhase, "test-dv")))
	readyCondition := FindConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
	Expect(readyCondition.Status).To(Equal(readyStatusByPhase(expected)))
	Expect(readyCondition.Message).To(Equal(""))
	By("Checking events recorded")
	close(reconciler.recorder.(*record.FakeRecorder).Events)
	found := false
	for event := range reconciler.recorder.(*record.FakeRecorder).Events {
		By(event)
		if strings.Contains(event, expectedEvent) {
			found = true
		}
	}
	Expect(found).To(BeTrue())
}

func createStorageSpec() *cdiv1.StorageSpec {
	return &cdiv1.StorageSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		VolumeMode:  &BlockMode,
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1G"),
			},
		},
	}
}

func boundStatusByPVCPhase(pvcPhase corev1.PersistentVolumeClaimPhase) corev1.ConditionStatus {
	if pvcPhase == corev1.ClaimBound {
		return corev1.ConditionTrue
	} else if pvcPhase == corev1.ClaimPending {
		return corev1.ConditionFalse
	} else if pvcPhase == corev1.ClaimLost {
		return corev1.ConditionFalse
	}
	return corev1.ConditionUnknown
}

func boundMessageByPVCPhase(pvcPhase corev1.PersistentVolumeClaimPhase, pvcName string) string {
	switch pvcPhase {
	case corev1.ClaimBound:
		return fmt.Sprintf("PVC %s Bound", pvcName)
	case corev1.ClaimPending:
		return fmt.Sprintf("PVC %s Pending", pvcName)
	case corev1.ClaimLost:
		return "Claim Lost"
	default:
		return "No PVC found"
	}
}

func readyStatusByPhase(phase cdiv1.DataVolumePhase) corev1.ConditionStatus {
	switch phase {
	case cdiv1.Succeeded:
		return corev1.ConditionTrue
	case cdiv1.Unknown:
		return corev1.ConditionUnknown
	default:
		return corev1.ConditionFalse
	}
}

func createImportReconcilerWFFCDisabled(objects ...client.Object) *ImportReconciler {
	return createImportReconcilerWithFeatureGates(nil, objects...)
}
func createImportReconciler(objects ...client.Object) *ImportReconciler {
	return createImportReconcilerWithFeatureGates([]string{featuregates.HonorWaitForFirstConsumer}, objects...)
}

func createImportReconcilerWithFeatureGates(featureGates []string, objects ...client.Object) *ImportReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = featureGates

	objs := []client.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createImportReconcilerWithoutConfig(objs...)
}

func createImportReconcilerWithoutConfig(objects ...client.Object) *ImportReconciler {
	objs := []client.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ImportReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvImportLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
			shouldUpdateProgress: true,
		},
	}
	return r
}

func newImportDataVolumeWithPvc(name string, pvc *corev1.PersistentVolumeClaimSpec) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://example.com/data",
				},
			},
			PVC: pvc,
		},
	}
}

func newS3ImportDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				S3: &cdiv1.DataVolumeSourceS3{
					URL: "http://example.com/data",
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
			PriorityClassName: "p0-s3",
		},
	}
}

func newUploadDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Upload: &cdiv1.DataVolumeSourceUpload{},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
			PriorityClassName: "p0-upload",
		},
	}
}

func newBlankImageDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		},
	}
}

func newVDDKDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				VDDK: &cdiv1.DataVolumeSourceVDDK{},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
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
