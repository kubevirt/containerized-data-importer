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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	storagev1 "k8s.io/api/storage/v1"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
	dvLog              = logf.Log.WithName("datavolume-controller-test")
)

var _ = Describe("Datavolume controller reconcile loop", func() {
	var (
		reconciler *DatavolumeReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should do nothing and return nil when no DV exists", func() {
		reconciler = createDatavolumeReconciler()
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).To(HaveOccurred())
		if !k8serrors.IsNotFound(err) {
			Fail("Error getting pvc")
		}
	})

	It("Should create a PVC on a valid import DV", func() {
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Name).To(Equal("test-dv"))
	})

	It("Should pass annotation from DV to created a PVC on a DV", func() {
		dv := newImportDataVolume("test-dv")
		dv.SetAnnotations(make(map[string]string))
		dv.GetAnnotations()["test-ann-1"] = "test-value-1"
		dv.GetAnnotations()["test-ann-2"] = "test-value-2"
		dv.GetAnnotations()[AnnSource] = "invalid phase should not copy"
		reconciler = createDatavolumeReconciler(dv)
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Name).To(Equal("test-dv"))
		Expect(pvc.GetAnnotations()).ToNot(BeNil())
		Expect(pvc.GetAnnotations()["test-ann-1"]).To(Equal("test-value-1"))
		Expect(pvc.GetAnnotations()["test-ann-2"]).To(Equal("test-value-2"))
		Expect(pvc.GetAnnotations()[AnnSource]).To(Equal(SourceHTTP))
	})

	It("Should pass annotation from DV with S3 source to created a PVC on a DV", func() {
		dv := newS3ImportDataVolume("test-dv")
		dv.SetAnnotations(make(map[string]string))
		dv.GetAnnotations()["test-ann-1"] = "test-value-1"
		dv.GetAnnotations()["test-ann-2"] = "test-value-2"
		dv.GetAnnotations()[AnnSource] = "invalid phase should not copy"
		reconciler = createDatavolumeReconciler(dv)
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Name).To(Equal("test-dv"))
		Expect(pvc.GetAnnotations()).ToNot(BeNil())
		Expect(pvc.GetAnnotations()["test-ann-1"]).To(Equal("test-value-1"))
		Expect(pvc.GetAnnotations()["test-ann-2"]).To(Equal("test-value-2"))
		Expect(pvc.GetAnnotations()[AnnSource]).To(Equal(SourceS3))
	})

	It("Should follow the phase of the created PVC", func() {
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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

		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(cdiv1.Pending))
	})

	It("Should follow the restarts of the PVC", func() {
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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

		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.RestartCount).To(Equal(int32(2)))
	})

	It("Should error if a PVC with same name already exists that is not owned by us", func() {
		reconciler = createDatavolumeReconciler(createPvc("test-dv", metav1.NamespaceDefault, map[string]string{}, nil), newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(HaveOccurred())
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Resource \"test-dv\" already exists and is not managed by DataVolume"))
	})

	It("Should add owner to pre populated PVC", func() {
		annotations := map[string]string{"cdi.kubevirt.io/storage.populatedFor": "test-dv"}
		pvc := createPvc("test-dv", metav1.NamespaceDefault, annotations, nil)
		pvc.Status.Phase = corev1.ClaimBound
		dv := newImportDataVolume("test-dv")
		reconciler = createDatavolumeReconciler(pvc, dv)
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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

	It("Should create a snapshot if cloning and the PVC doesn't exist, and the snapshot class can be found", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		sc := createStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, "csi-plugin")
		dv.Spec.PVC.StorageClassName = &scName
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
		expectedSnapshotClass := "snap-class"
		snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
		reconciler := createDatavolumeReconciler(sc, dv, pvc, snapClass)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that phase is now snapshot in progress")
		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(cdiv1.SnapshotForSmartCloneInProgress))
	})

	DescribeTable("Should NOT create a snapshot if source PVC mounted", func(podFunc func(*cdiv1.DataVolume) *corev1.Pod) {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		sc := createStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, "csi-plugin")
		dv.Spec.PVC.StorageClassName = &scName
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
		expectedSnapshotClass := "snap-class"
		snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
		reconciler := createDatavolumeReconciler(sc, dv, pvc, snapClass, podFunc(dv))
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		By("Checking events recorded")
		close(reconciler.recorder.(*record.FakeRecorder).Events)
		found := false
		for event := range reconciler.recorder.(*record.FakeRecorder).Events {
			if strings.Contains(event, "SmartCloneSourceInUse") {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	},
		Entry("read/write", func(dv *cdiv1.DataVolume) *corev1.Pod {
			return podUsingCloneSource(dv, false)
		}),
		Entry("read only", func(dv *cdiv1.DataVolume) *corev1.Pod {
			return podUsingCloneSource(dv, true)
		}),
	)
})

var _ = Describe("Reconcile Datavolume status", func() {
	var (
		reconciler *DatavolumeReconciler
	)

	DescribeTable("if no pvc exists", func(current, expected cdiv1.DataVolumePhase) {
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		dv := &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		dv.Status.Phase = current
		err = reconciler.client.Update(context.TODO(), dv)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.reconcileDataVolumeStatus(dv, nil)
		Expect(err).ToNot(HaveOccurred())

		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(expected))
		Expect(len(dv.Status.Conditions)).To(Equal(3))
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
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
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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
		_, err = reconciler.reconcileDataVolumeStatus(dv, pvc)
		Expect(err).ToNot(HaveOccurred())
		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(cdiv1.Pending))
		Expect(len(dv.Status.Conditions)).To(Equal(3))
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
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
		reconciler = createDatavolumeReconciler(sc, newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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
		_, err = reconciler.reconcileDataVolumeStatus(dv, pvc)
		Expect(err).ToNot(HaveOccurred())
		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

		Expect(len(dv.Status.Conditions)).To(Equal(3))
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
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
		scDefault := createStorageClass("default_test_sc", map[string]string{
			AnnDefaultStorageClass: "true",
		})
		scWffc := createStorageClassWithBindingMode(scName, map[string]string{}, storagev1.VolumeBindingWaitForFirstConsumer)
		importDataVolume := newImportDataVolume("test-dv")
		importDataVolume.Spec.PVC.StorageClassName = &scName

		reconciler = createDatavolumeReconciler(scDefault, scWffc, importDataVolume)
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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
		_, err = reconciler.reconcileDataVolumeStatus(dv, pvc)
		Expect(err).ToNot(HaveOccurred())
		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(cdiv1.WaitForFirstConsumer))

		Expect(len(dv.Status.Conditions)).To(Equal(3))
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
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
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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
		_, err = reconciler.reconcileDataVolumeStatus(dv, pvc)
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
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
		Expect(boundCondition.Status).To(Equal(corev1.ConditionFalse))
		Expect(boundCondition.Message).To(Equal("PVC test-dv Pending"))
		readyCondition := findConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Message).To(Equal(""))
	})

	DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
		reconciler = createDatavolumeReconciler(testDv)
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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

		_, err = reconciler.reconcileDataVolumeStatus(dv, pvc)
		Expect(err).ToNot(HaveOccurred())

		dv = &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.Phase).To(Equal(expected))
		Expect(len(dv.Status.Conditions)).To(Equal(3))
		boundCondition := findConditionByType(cdiv1.DataVolumeBound, dv.Status.Conditions)
		Expect(boundCondition.Status).To(Equal(boundStatusByPVCPhase(pvcPhase)))
		Expect(boundCondition.Message).To(Equal(boundMessageByPVCPhase(pvcPhase, "test-dv")))
		readyCondition := findConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
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
	},
		Entry("should switch to bound for import", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.PVCBound, corev1.ClaimBound, corev1.PodPending, "invalid", "PVC test-dv Bound"),
		Entry("should switch to bound for import", newImportDataVolume("test-dv"), cdiv1.Unknown, cdiv1.PVCBound, corev1.ClaimBound, corev1.PodPending, "invalid", "PVC test-dv Bound"),
		Entry("should switch to scheduled for import", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodPending, AnnImportPod, "Import into test-dv scheduled"),
		Entry("should switch to inprogress for import", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportInProgress, corev1.ClaimBound, corev1.PodRunning, AnnImportPod, "Import into test-dv in progress"),
		Entry("should switch to failed for import", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimBound, corev1.PodFailed, AnnImportPod, "Failed to import into PVC test-dv"),
		Entry("should switch to failed on claim lost for impot", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnImportPod, "PVC test-dv lost"),
		Entry("should switch to succeeded for import", newImportDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnImportPod, "Successfully imported into PVC test-dv"),
		Entry("should switch to scheduled for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneScheduled, corev1.ClaimBound, corev1.PodPending, AnnCloneRequest, "Cloning from default/test into default/test-dv scheduled"),
		Entry("should switch to clone in progress for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneInProgress, corev1.ClaimBound, corev1.PodRunning, AnnCloneRequest, "Cloning from default/test into default/test-dv in progress"),
		Entry("should switch to failed for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimBound, corev1.PodFailed, AnnCloneRequest, "Cloning from default/test into default/test-dv failed"),
		Entry("should switch to failed on claim lost for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnCloneRequest, "PVC test-dv lost"),
		Entry("should switch to succeeded for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnCloneRequest, "Successfully cloned from default/test into default/test-dv"),
		Entry("should switch to scheduled for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadScheduled, corev1.ClaimBound, corev1.PodPending, AnnUploadRequest, "Upload into test-dv scheduled"),
		Entry("should switch to uploadready for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadReady, corev1.ClaimBound, corev1.PodRunning, AnnUploadRequest, "Upload into test-dv ready", AnnPodReady, "true"),
		Entry("should switch to failed for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimBound, corev1.PodFailed, AnnUploadRequest, "Upload into test-dv failed"),
		Entry("should switch to failed on claim lost for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnUploadRequest, "PVC test-dv lost"),
		Entry("should switch to succeeded for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnUploadRequest, "Successfully uploaded into test-dv"),
		Entry("should switch to scheduled for blank", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportScheduled, corev1.ClaimBound, corev1.PodPending, AnnImportPod, "Import into test-dv scheduled"),
		Entry("should switch to inprogress for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.ImportInProgress, corev1.ClaimBound, corev1.PodRunning, AnnImportPod, "Import into test-dv in progress"),
		Entry("should switch to failed for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimBound, corev1.PodFailed, AnnImportPod, "Failed to import into PVC test-dv"),
		Entry("should switch to failed on claim lost for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnImportPod, "PVC test-dv lost"),
		Entry("should switch to succeeded for blank", newBlankImageDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnImportPod, "Successfully imported into PVC test-dv"),
	)
})

var _ = Describe("sourcePVCPopulated", func() {
	var (
		reconciler *DatavolumeReconciler
	)

	It("Should return true if source has no ownerRef", func() {
		sourcePvc := createPvc("test", "default", nil, nil)
		targetDv := newCloneDataVolume("test-dv")
		reconciler = createDatavolumeReconciler(sourcePvc)
		res, err := reconciler.isSourcePVCPopulated(targetDv)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(BeTrue())
	})

	It("Should return false and error if source has an ownerRef, but it doesn't exist", func() {
		controller := true
		sourcePvc := createPvc("test", "default", nil, nil)
		targetDv := newCloneDataVolume("test-dv")
		sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
			Kind:       "DataVolume",
			Controller: &controller,
		})
		reconciler = createDatavolumeReconciler(sourcePvc)
		res, err := reconciler.isSourcePVCPopulated(targetDv)
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeFalse())
	})

	It("Should return false if source has an ownerRef, but it is not succeeded", func() {
		controller := true
		sourcePvc := createPvc("test", "default", nil, nil)
		targetDv := newCloneDataVolume("test-dv")
		sourceDv := newImportDataVolume("source-dv")
		sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
			Kind:       "DataVolume",
			Controller: &controller,
			Name:       "source-dv",
		})
		reconciler = createDatavolumeReconciler(sourcePvc, sourceDv)
		res, err := reconciler.isSourcePVCPopulated(targetDv)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(BeFalse())
	})

	It("Should return true if source has an ownerRef, but it is succeeded", func() {
		controller := true
		sourcePvc := createPvc("test", "default", nil, nil)
		targetDv := newCloneDataVolume("test-dv")
		sourceDv := newImportDataVolume("source-dv")
		sourceDv.Status.Phase = cdiv1.Succeeded
		sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
			Kind:       "DataVolume",
			Controller: &controller,
			Name:       "source-dv",
		})
		reconciler = createDatavolumeReconciler(sourcePvc, sourceDv)
		res, err := reconciler.isSourcePVCPopulated(targetDv)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(BeTrue())
	})
})

func podUsingCloneSource(dv *cdiv1.DataVolume, readOnly bool) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dv.Spec.Source.PVC.Namespace,
			Name:      dv.Spec.Source.PVC.Name + "-pod",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: dv.Spec.Source.PVC.Name,
							ReadOnly:  readOnly,
						},
					},
				},
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

var _ = Describe("Smart clone", func() {
	It("Should not return storage class, if no source pvc provided", func() {
		dv := newImportDataVolume("test-dv")
		reconciler := createDatavolumeReconciler(dv)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no source PVC provided"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if no CSI CRDs exist", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "test"
		sc := createStorageClass(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		reconciler := createDatavolumeReconciler(dv, sc)
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("CSI snapshot CRDs not found"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if source PVC doesn't exist", func() {
		dv := newCloneDataVolumeWithPVCNS("test-dv", "ns2")
		scName := "test"
		sc := createStorageClass(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		reconciler := createDatavolumeReconciler(dv, sc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source PVC not found"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if source PVC exist, but no storage class exists, and no storage class in PVC def", func() {
		dv := newCloneDataVolume("test-dv")
		pvc := createPvc("test", metav1.NamespaceDefault, nil, nil)
		reconciler := createDatavolumeReconciler(dv, pvc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Target PVC storage class not found"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if source SC and target SC do not match", func() {
		dv := newCloneDataVolume("test-dv")
		targetSc := "testsc"
		tsc := createStorageClass(targetSc, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		dv.Spec.PVC.StorageClassName = &targetSc
		sourceSc := "testsc2"
		ssc := createStorageClass(sourceSc, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &sourceSc, nil, nil, corev1.ClaimBound)
		reconciler := createDatavolumeReconciler(ssc, tsc, dv, pvc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source PVC and target PVC belong to different storage classes"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if source NS and target NS do not match", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		sc := createStorageClass(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		dv.Spec.PVC.StorageClassName = &scName
		dv.Spec.Source.PVC.Namespace = "other-ns"
		pvc := createPvcInStorageClass("test", "other-ns", &scName, nil, nil, corev1.ClaimBound)
		reconciler := createDatavolumeReconciler(sc, dv, pvc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source PVC and target PVC belong to different namespaces"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if storage class does not exist", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		dv.Spec.PVC.StorageClassName = &scName
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
		reconciler := createDatavolumeReconciler(dv, pvc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to retrieve storage class"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should not return storage class, if storage class does not exist", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		sc := createStorageClass(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		})
		dv.Spec.PVC.StorageClassName = &scName
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
		reconciler := createDatavolumeReconciler(sc, dv, pvc)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not match snapshotter with storage class, falling back to host assisted clone"))
		Expect(snapclass).To(BeEmpty())
	})

	It("Should return snapshot class, everything is available", func() {
		dv := newCloneDataVolume("test-dv")
		scName := "testsc"
		sc := createStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, "csi-plugin")
		dv.Spec.PVC.StorageClassName = &scName
		pvc := createPvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
		expectedSnapshotClass := "snap-class"
		snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
		reconciler := createDatavolumeReconciler(sc, dv, pvc, snapClass)
		reconciler.extClientSet = extfake.NewSimpleClientset(createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		snapclass, err := reconciler.getSnapshotClassForSmartClone(dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(snapclass).To(Equal(expectedSnapshotClass))
	})
})

var _ = Describe("Get Pod from PVC", func() {
	var (
		reconciler *DatavolumeReconciler
		pvc        *corev1.PersistentVolumeClaim
	)
	BeforeEach(func() {
		reconciler = createDatavolumeReconciler(newImportDataVolume("test-dv"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvc = &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return error if no pods can be found", func() {
		_, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc.GetUID())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
	})

	It("Should return pod if pods can be found based on owner ref", func() {
		pod := createImporterTestPod(pvc, "test-dv", nil)
		pod.SetLabels(make(map[string]string))
		pod.GetLabels()[common.PrometheusLabel] = ""
		err := reconciler.client.Create(context.TODO(), pod)
		Expect(err).ToNot(HaveOccurred())
		foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc.GetUID())
		Expect(err).ToNot(HaveOccurred())
		Expect(foundPod.Name).To(Equal(pod.Name))
	})

	It("Should return pod if pods can be found based on cloneid", func() {
		pod := createImporterTestPod(pvc, "test-dv", nil)
		pod.SetLabels(make(map[string]string))
		pod.GetLabels()[common.PrometheusLabel] = ""
		pod.GetLabels()[CloneUniqueID] = string(pvc.GetUID()) + "-source-pod"
		pod.OwnerReferences = nil
		err := reconciler.client.Create(context.TODO(), pod)
		Expect(err).ToNot(HaveOccurred())
		foundPod, err := reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc.GetUID())
		Expect(err).ToNot(HaveOccurred())
		Expect(foundPod.Name).To(Equal(pod.Name))
	})

	It("Should return error if pods can be found but cloneid doesn't match", func() {
		pod := createImporterTestPod(pvc, "test-dv", nil)
		pod.SetLabels(make(map[string]string))
		pod.GetLabels()[common.PrometheusLabel] = ""
		pod.GetLabels()[CloneUniqueID] = string(pvc.GetUID()) + "-source-p"
		pod.OwnerReferences = nil
		err := reconciler.client.Create(context.TODO(), pod)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.getPodFromPvc(metav1.NamespaceDefault, pvc.GetUID())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Unable to find pod owned by UID: %s, in namespace: %s", string(pvc.GetUID()), metav1.NamespaceDefault)))
	})
})

var _ = Describe("Update Progress from pod", func() {
	var (
		pvc *corev1.PersistentVolumeClaim
		pod *corev1.Pod
		dv  *cdiv1.DataVolume
	)

	BeforeEach(func() {
		pvc = createPvc("test", metav1.NamespaceDefault, nil, nil)
		pod = createImporterTestPod(pvc, "test", nil)
		dv = newImportDataVolume("test")
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

func createDatavolumeReconciler(objects ...runtime.Object) *DatavolumeReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}
	objs = append(objs, cdiConfig)
	extfakeclientset := extfake.NewSimpleClientset()

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(10)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &DatavolumeReconciler{
		client:       cl,
		scheme:       s,
		log:          dvLog,
		recorder:     rec,
		extClientSet: extfakeclientset,
		featureGates: featuregates.NewFeatureGates(cl),
	}
	return r
}

func newImportDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://example.com/data",
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
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
			Source: cdiv1.DataVolumeSource{
				S3: &cdiv1.DataVolumeSourceS3{
					URL: "http://example.com/data",
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
		},
	}
}

func newCloneDataVolume(name string) *cdiv1.DataVolume {
	return newCloneDataVolumeWithPVCNS(name, "default")
}

func newCloneDataVolumeWithPVCNS(name string, pvcNamespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Annotations: map[string]string{
				AnnCloneToken: "foobar",
			},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: pvcNamespace,
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
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
			Source: cdiv1.DataVolumeSource{
				Upload: &cdiv1.DataVolumeSourceUpload{},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
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
			Source: cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{},
		},
	}
}
