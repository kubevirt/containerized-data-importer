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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

const (
	testStorageClass = "test-sc"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
	dvImportLog        = logf.Log.WithName("datavolume-import-controller-test")
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
			if !k8serrors.IsNotFound(err) {
				Fail("Error getting pvc")
			}
		})

		It("Should create a PVC on a valid import DV", func() {
			reconciler = createImportReconciler(NewImportDataVolume("test-dv"))
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
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
			Expect(pvc.Spec.StorageClassName).ToNot(Equal("defaultSc"))
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
			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(pvc.Spec.VolumeMode).ToNot(BeNil())
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
			Expect(pvc.Spec.StorageClassName).ToNot(Equal("defaultSc"))
		})

		It("Should fail on missing size, without storageClass", func() {
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			// spec with accessMode/VolumeMode so storageprofile is not needed
			importDataVolume.Spec.Storage = createStorageSpec()
			importDataVolume.Spec.Storage.Resources = corev1.ResourceRequirements{}
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
			importDataVolume.Spec.Storage.Resources = corev1.ResourceRequirements{}
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
				Resources: corev1.ResourceRequirements{
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
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
				Resources: corev1.ResourceRequirements{
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
			Expect(err.Error()).To(ContainSubstring("DataVolume with ContentType Archive cannot have block volumeMode"))
			By("Checking error event recorded")
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("DataVolume with ContentType Archive cannot have block volumeMode"))
		})

		It("Should set on a PVC matching access mode from storageProfile to the DV given volume mode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				VolumeMode:       &FilesystemMode,
				Resources: corev1.ResourceRequirements{
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set on a PVC matching access mode from storageProfile to the DV given contentType archive", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				Resources: corev1.ResourceRequirements{
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
			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set on a PVC matching volume mode from storageProfile to the given DV access mode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadWriteOnce))
			Expect(*pvc.Spec.VolumeMode).To(Equal(FilesystemMode))
		})

		It("Should set params on a PVC from correct storageProfile when import DV has no accessMode", func() {
			scName := "testStorageClass"
			importDataVolume := newImportDataVolumeWithPvc("test-dv", nil)
			importDataVolume.Spec.Storage = &cdiv1.StorageSpec{
				StorageClassName: &scName,
				Resources: corev1.ResourceRequirements{
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
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
				Resources: corev1.ResourceRequirements{
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

			Expect(len(pvc.Spec.AccessModes)).To(BeNumerically("==", 1))
			Expect(pvc.Spec.AccessModes[0]).To(Equal(corev1.ReadOnlyMany))
			Expect(*pvc.Spec.VolumeMode).To(Equal(BlockMode))
			expectedSize := resource.MustParse("1G")
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize.Value()))
		})

		It("Should pass annotation from DV to created a PVC on a DV", func() {
			dv := NewImportDataVolume("test-dv")
			dv.SetAnnotations(make(map[string]string))
			dv.GetAnnotations()["test-ann-1"] = "test-value-1"
			dv.GetAnnotations()["test-ann-2"] = "test-value-2"
			dv.GetAnnotations()[AnnSource] = "invalid phase should not copy"
			dv.GetAnnotations()[AnnPodNetwork] = "data-network"
			dv.GetAnnotations()[AnnPodSidecarInjection] = "false"
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
			Expect(pvc.GetAnnotations()[AnnPodSidecarInjection]).To(Equal("false"))
			Expect(pvc.GetAnnotations()[AnnPriorityClassName]).To(Equal("p0"))
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
			err = reconciler.client.Update(context.TODO(), pvc)
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
				AnnPopulatedFor:       "test-dv",
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
				Expect(ok).To(Equal(false))
				_, ok = pvc.GetAnnotations()[AnnPreviousCheckpoint]
				Expect(ok).To(Equal(false))
				_, ok = pvc.GetAnnotations()[AnnFinalCheckpoint]
				Expect(ok).To(Equal(false))
				_, ok = pvc.GetAnnotations()[AnnCurrentPodID]
				Expect(ok).To(Equal(false))
				_, ok = pvc.GetAnnotations()[AnnCheckpointsCopied+".current"]
				Expect(ok).To(Equal(false))
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
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, nil, nil, reconciler.updateStatusPhase)
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(expected))
			Expect(len(dv.Status.Conditions)).To(Equal(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionUnknown))
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
			Entry("should remain unset", cdiv1.PhaseUnset, cdiv1.PhaseUnset),
			Entry("should remain pending", cdiv1.Pending, cdiv1.Pending),
			Entry("should remain snapshotforsmartcloninginprogress", cdiv1.SnapshotForSmartCloneInProgress, cdiv1.SnapshotForSmartCloneInProgress),
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
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, reconciler.updateStatusPhase)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.Pending))
			Expect(len(dv.Status.Conditions)).To(Equal(3))
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
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, reconciler.updateStatusPhase)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

			Expect(len(dv.Status.Conditions)).To(Equal(3))
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
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, reconciler.updateStatusPhase)
			Expect(err).ToNot(HaveOccurred())
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

			Expect(len(dv.Status.Conditions)).To(Equal(3))
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
			pvc.Status.Phase = corev1.ClaimPending
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, reconciler.updateStatusPhase)
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
			Expect(len(dv.Status.Conditions)).To(Equal(3))
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
			pvc.Status.Phase = corev1.ClaimPending
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnCurrentCheckpoint] = "current"
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, reconciler.updateStatusPhase)
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
			Expect(len(dv.Status.Conditions)).To(Equal(3))
			boundCondition := FindConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
			Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
			readyCondition := FindConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
			Expect(readyCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(readyCondition.Message).To(Equal(""))
		})

		DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			scName := "testpvc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)

			r := createImportReconciler(testDv, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r.updateStatusPhase, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent, extraAnnotations...)
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
			_, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
		})

		It("Should return pod if pods can be found based on owner ref", func() {
			pod := CreateImporterTestPod(pvc, "test-dv", nil)
			pod.SetLabels(make(map[string]string))
			pod.GetLabels()[common.PrometheusLabelKey] = common.PrometheusLabelValue
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())
			foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
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
			foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
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
			_, err = reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
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
			foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
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
			foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc)
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
				w.Write([]byte(fmt.Sprintf("import_progress{ownerUID=\"%v\"} 13.45", dv.GetUID())))
				w.WriteHeader(200)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.Atoi(ep.Port())
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
				w.Write([]byte(fmt.Sprintf("import_progress{ownerUID=\"%v\"} 13.45", "b856691e-1038-11e9-a5ab-55500d15501")))
				w.WriteHeader(200)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.Atoi(ep.Port())
			Expect(err).ToNot(HaveOccurred())
			pod.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			pod.Status.PodIP = ep.Hostname()
			err = updateProgressUsingPod(dv, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Progress).To(BeEquivalentTo("2.3%"))
		})
	})

	const (
		Mi              = int64(1024 * 1024)
		Gi              = 1024 * Mi
		noOverhead      = float64(0)
		defaultOverhead = float64(0.055)
		largeOverhead   = float64(0.75)
	)
	DescribeTable("GetRequiredSpace should return properly enlarged sizes,", func(imageSize int64, overhead float64) {
		for testedSize := int64(imageSize - 1024); testedSize < imageSize+1024; testedSize++ {
			alignedImageSpace := imageSize
			if testedSize > imageSize {
				alignedImageSpace = imageSize + Mi
			}

			// TEST
			actualRequiredSpace := GetRequiredSpace(overhead, testedSize)

			// ASSERT results
			// check that the resulting space includes overhead over the `aligned image size`
			overheadSpace := actualRequiredSpace - alignedImageSpace
			actualOverhead := float64(overheadSpace) / float64(actualRequiredSpace)

			Expect(actualOverhead).To(BeNumerically("~", overhead, 0.01))
		}
	},
		Entry("1Mi virtual size, 0 overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, noOverhead),
		Entry("1Mi virtual size, default overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, defaultOverhead),
		Entry("1Mi virtual size, large overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, largeOverhead),
		Entry("40Mi virtual size, 0 overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, noOverhead),
		Entry("40Mi virtual size, default overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, defaultOverhead),
		Entry("40Mi virtual size, large overhead to be 40Mi if <= 40Mi and 41Mi if > 40Mi", 40*Mi, largeOverhead),
		Entry("1Gi virtual size, 0 overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, noOverhead),
		Entry("1Gi virtual size, default overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, defaultOverhead),
		Entry("1Gi virtual size, large overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, largeOverhead),
		Entry("40Gi virtual size, 0 overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, noOverhead),
		Entry("40Gi virtual size, default overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, defaultOverhead),
		Entry("40Gi virtual size, large overhead to be 40Gi if <= 40Gi and 41Gi if > 40Gi", 40*Gi, largeOverhead),
	)

	Describe("DataVolume garbage collection", func() {
		It("updatePvcOwnerRefs should correctly update PVC owner refs", func() {
			ref := func(uid string) metav1.OwnerReference {
				return metav1.OwnerReference{UID: types.UID(uid)}
			}
			dv := NewImportDataVolume("test-dv")
			vmOwnerRef := metav1.OwnerReference{Kind: "VirtualMachine", Name: "test-vm", UID: "test-vm-uid"}
			dv.OwnerReferences = append(dv.OwnerReferences, ref("3"), vmOwnerRef, ref("2"), ref("1"))

			pvc := CreatePvc("test-dv", metav1.NamespaceDefault, nil, nil)
			dvOwnerRef := metav1.OwnerReference{Kind: "DataVolume", Name: "test-dv", UID: dv.UID}
			pvc.OwnerReferences = append(pvc.OwnerReferences, ref("1"), dvOwnerRef, ref("2"))

			updatePvcOwnerRefs(pvc, dv)
			Expect(pvc.OwnerReferences).To(HaveLen(4))
			Expect(pvc.OwnerReferences).To(Equal([]metav1.OwnerReference{ref("1"), ref("2"), ref("3"), vmOwnerRef}))
		})
	})

})

func dvPhaseTest(reconciler ReconcilerBase, updateStatusPhase updateStatusPhaseFunc, testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
	Expect(err).ToNot(HaveOccurred())
	dv := &cdiv1.DataVolume{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
	Expect(err).ToNot(HaveOccurred())
	dv.Status.Phase = current
	err = reconciler.client.Update(context.TODO(), dv)
	Expect(err).ToNot(HaveOccurred())

	pvc := &corev1.PersistentVolumeClaim{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
	Expect(err).ToNot(HaveOccurred())
	Expect(pvc.Name).To(Equal("test-dv"))
	pvc.Status.Phase = pvcPhase
	pvc.SetAnnotations(make(map[string]string))
	pvc.GetAnnotations()[ann] = "something"
	pvc.GetAnnotations()[AnnPodPhase] = string(podPhase)
	for i := 0; i < len(extraAnnotations); i += 2 {
		pvc.GetAnnotations()[extraAnnotations[i]] = extraAnnotations[i+1]
	}

	_, err = reconciler.reconcileDataVolumeStatus(dv, pvc, nil, updateStatusPhase)
	Expect(err).ToNot(HaveOccurred())

	dv = &cdiv1.DataVolume{}
	err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
	Expect(err).ToNot(HaveOccurred())
	Expect(dv.Status.Phase).To(Equal(expected))
	Expect(len(dv.Status.Conditions)).To(Equal(3))
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
		Resources: corev1.ResourceRequirements{
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

func createImportReconciler(objects ...runtime.Object) *ImportReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createImportReconcilerWithoutConfig(objs...)
}

func createImportReconcilerWithoutConfig(objects ...runtime.Object) *ImportReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)
	extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

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
		},
	}
	r.Reconciler = r
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
