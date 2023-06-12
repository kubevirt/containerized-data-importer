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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	dvUploadLog = logf.Log.WithName("datavolume-upload-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *UploadReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	It("Should create volumeUploadSource if should use cdi populator", func() {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}
		dv := newUploadDataVolume("test-dv")
		dv.Spec.ContentType = cdiv1.DataVolumeArchive
		preallocation := true
		dv.Spec.Preallocation = &preallocation
		reconciler = createUploadReconciler(dv, sc, csiDriver)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))

		uploadSource := &cdiv1.VolumeUploadSource{}
		uploadSourceName := volumeUploadSourceName(dv)
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadSourceName, Namespace: metav1.NamespaceDefault}, uploadSource)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadSource.Spec.ContentType).To(Equal(dv.Spec.ContentType))
		Expect(uploadSource.Spec.Preallocation).To(Equal(dv.Spec.Preallocation))
		Expect(uploadSource.OwnerReferences).To(HaveLen(1))
		or := uploadSource.OwnerReferences[0]
		Expect(or.UID).To(Equal(dv.UID))
	})

	It("Should delete volumeUploadSource if dv succeeded and we use cdi populator", func() {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}
		dv := newUploadDataVolume("test-dv")
		uploadSourceName := volumeUploadSourceName(dv)
		uploadSource := &cdiv1.VolumeUploadSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uploadSourceName,
				Namespace: dv.Namespace,
			},
		}
		reconciler = createUploadReconciler(dv, sc, csiDriver, uploadSource)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))

		dv.Status.Phase = cdiv1.Succeeded
		err = reconciler.client.Update(context.TODO(), dv)
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		deletedUploadSource := &cdiv1.VolumeUploadSource{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadSourceName, Namespace: dv.Namespace}, deletedUploadSource)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("Should fail if dv source not upload when use populators", func() {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}
		dv := newUploadDataVolume("test-dv")
		dv.Spec.Source.Upload = nil
		reconciler = createUploadReconciler(dv, sc, csiDriver)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no source set for upload datavolume"))
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("Should create a PVC with volumeUploadSource when use populators", func() {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}
		dv := newUploadDataVolume("test-dv")
		reconciler = createUploadReconciler(dv, sc, csiDriver)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Name).To(Equal("test-dv"))
		Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
		Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
		Expect(pvc.Spec.DataSourceRef).ToNot(BeNil())
		uploadSourceName := volumeUploadSourceName(dv)
		Expect(pvc.Spec.DataSourceRef.Name).To(Equal(uploadSourceName))
		Expect(pvc.Spec.DataSourceRef.Kind).To(Equal(cdiv1.VolumeUploadSourceRef))
	})

	It("Should always report NA progress for upload population", func() {
		scName := "testSC"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		csiDriver := &storagev1.CSIDriver{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-plugin",
			},
		}
		dv := newUploadDataVolume("test-dv")
		reconciler = createUploadReconciler(dv, sc, csiDriver)
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

		// updating the annotation to make sure we dont update the progress
		// in reality the annotation should never be on upload pvcs
		AddAnnotation(pvc, AnnPopulatorProgress, "13.45%")
		err = reconciler.client.Update(context.TODO(), pvc)
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(dv.Status.Progress)).To(Equal("N/A"))
	})

	var _ = Describe("Reconcile Datavolume status", func() {
		DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			scName := "testpvc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)

			r := createUploadReconciler(testDv, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent, extraAnnotations...)
		},
			Entry("should switch to scheduled for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadScheduled, corev1.ClaimBound, corev1.PodPending, AnnUploadRequest, "Upload into test-dv scheduled", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to uploadready for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadReady, corev1.ClaimBound, corev1.PodRunning, AnnUploadRequest, "Upload into test-dv ready", AnnPodReady, "true", AnnPriorityClassName, "p0-upload"),
			Entry("should stay the same for upload after pod fails", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadScheduled, corev1.ClaimBound, corev1.PodFailed, AnnUploadRequest, "Upload into test-dv failed", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to failed on claim lost for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnUploadRequest, "PVC test-dv lost", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to succeeded for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnUploadRequest, "Successfully uploaded into test-dv", AnnPriorityClassName, "p0-upload"),
		)

		It("Should set DV phase to UploadScheduled if use populators wffc storage class after scheduled node", func() {
			scName := "pvc_sc_wffc"
			bindingMode := storagev1.VolumeBindingWaitForFirstConsumer
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			sc.VolumeBindingMode = &bindingMode
			csiDriver := &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csi-plugin",
				},
			}
			uploadDataVolume := newUploadDataVolume("test-dv")
			uploadDataVolume.Spec.PVC.StorageClassName = &scName

			reconciler = createUploadReconciler(sc, csiDriver, uploadDataVolume)
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Name).To(Equal("test-dv"))
			pvc.Status.Phase = corev1.ClaimPending
			AddAnnotation(pvc, AnnSelectedNode, "node01")
			err = reconciler.client.Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.updateStatus(getReconcileRequest(uploadDataVolume), nil, reconciler)
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.UploadScheduled))

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
	})
})

func createUploadReconciler(objects ...runtime.Object) *UploadReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createUploadReconcilerWithoutConfig(objs...)
}

func createUploadReconcilerWithoutConfig(objects ...runtime.Object) *UploadReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

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
	r := &UploadReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvUploadLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
		},
	}
	return r
}
