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
	"fmt"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
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
	"kubevirt.io/containerized-data-importer/pkg/token"
)

var (
	dvCloneLog = logf.Log.WithName("datavolume-clone-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *CloneReconciler
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

		It("Should create a snapshot if cloning and the PVC doesn't exist, and the snapshot class can be found", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			expectedSnapshotClass := "snap-class"
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createCloneReconciler(sc, sp, dv, pvc, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			By("Verifying that snapshot now exists and phase is snapshot in progress")
			snap := &snapshotv1.VolumeSnapshot{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, snap)
			Expect(err).ToNot(HaveOccurred())
			Expect(snap.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.SnapshotForSmartCloneInProgress))
		})

		It("Should not recreate snpashot that was cleaned-up", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			expectedSnapshotClass := "snap-class"
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createCloneReconciler(sc, sp, dv, pvc, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			By("Verifying that snapshot now exists and phase is snapshot in progress")
			snap := &snapshotv1.VolumeSnapshot{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, snap)
			Expect(err).ToNot(HaveOccurred())
			Expect(snap.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.SnapshotForSmartCloneInProgress))

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("persistentvolumeclaims \"test-dv\" not found"))
			// Create smart clone PVC ourselves and delete snapshot (do smart clone controller's job)
			// Shouldn't see a recreated snapshot as it was legitimately cleaned up
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			controller := true
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			err = reconciler.client.Create(context.TODO(), targetPvc)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, targetPvc)
			Expect(err).ToNot(HaveOccurred())
			// Smart clone target PVC is done (bound), cleaning up snapshot
			err = reconciler.client.Delete(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, snap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("volumesnapshots.snapshot.storage.k8s.io \"test-dv\" not found"))
			// Reconcile and check it wasn't recreated
			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, snap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("volumesnapshots.snapshot.storage.k8s.io \"test-dv\" not found"))
		})

		It("Should do nothing when smart clone with namespace transfer and not target found", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", "test", &scName, nil, nil, corev1.ClaimBound)
			dv.Finalizers = append(dv.Finalizers, "cdi.kubevirt.io/dataVolumeFinalizer")
			dv.Spec.Source.PVC.Namespace = pvc.Namespace
			dv.Spec.Source.PVC.Name = pvc.Name
			dv.Status.Phase = cdiv1.NamespaceTransferInProgress
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("cdi-tmp-%s", dv.UID),
				},
			}
			expectedSnapshotClass := "snap-class"
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createCloneReconciler(sc, sp, dv, pvc, snapClass, ot, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			By("Verifying that phase is still NamespaceTransferInProgress")
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.NamespaceTransferInProgress))
		})

		DescribeTable("Should NOT create a snapshot if source PVC mounted", func(podFunc func(*cdiv1.DataVolume) *corev1.Pod) {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			expectedSnapshotClass := "snap-class"
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createCloneReconciler(sc, sp, dv, pvc, snapClass, podFunc(dv), createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
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
			reconciler.recorder = nil
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
		DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			scName := "testpvc"
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)

			r := createCloneReconciler(testDv, srcPvc, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r.updateStatusPhase, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent)
		},
			Entry("should switch to scheduled for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneScheduled, corev1.ClaimBound, corev1.PodPending, AnnCloneRequest, "Cloning from default/test into default/test-dv scheduled", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to clone in progress for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneInProgress, corev1.ClaimBound, corev1.PodRunning, AnnCloneRequest, "Cloning from default/test into default/test-dv in progress", AnnPriorityClassName, "p0-clone"),
			Entry("should stay the same for clone after pod fails", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneScheduled, corev1.ClaimBound, corev1.PodFailed, AnnCloneRequest, "Cloning from default/test into default/test-dv failed", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to failed on claim lost for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnCloneRequest, "PVC test-dv lost", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to succeeded for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnCloneRequest, "Successfully cloned from default/test into default/test-dv", AnnPriorityClassName, "p0-clone"),
		)
	})

	var _ = Describe("sourcePVCPopulated", func() {
		It("Should return true if source has no ownerRef", func() {
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			reconciler = createCloneReconciler(sourcePvc)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeTrue())
		})

		It("Should return false and error if source has an ownerRef, but it doesn't exist", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
			})
			reconciler = createCloneReconciler(sourcePvc)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeFalse())
		})

		It("Should return false if source has an ownerRef, but it is not succeeded", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourceDv := NewImportDataVolume("source-dv")
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "source-dv",
			})
			reconciler = createCloneReconciler(sourcePvc, sourceDv)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeFalse())
		})

		It("Should return true if source has an ownerRef, but it is succeeded", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourceDv := NewImportDataVolume("source-dv")
			sourceDv.Status.Phase = cdiv1.Succeeded
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "source-dv",
			})
			reconciler = createCloneReconciler(sourcePvc, sourceDv)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeTrue())
		})
	})

	var _ = Describe("Smart clone", func() {
		It("Should err, if no source pvc provided", func() {
			dv := NewImportDataVolume("test-dv")
			reconciler = createCloneReconciler(dv)
			possible, err := reconciler.advancedClonePossible(dv, dv.Spec.PVC)
			Expect(err).To(HaveOccurred())
			Expect(possible).To(BeFalse())
		})

		It("Should not return storage class, if no CSI CRDs exist", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "test"
			sc := CreateStorageClass(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			reconciler = createCloneReconciler(dv, sc)
			snapclass, err := reconciler.getSnapshotClassForSmartClone(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapclass).To(BeEmpty())
		})

		It("Should not return snapshot class, if source PVC doesn't exist", func() {
			dv := newCloneDataVolumeWithPVCNS("test-dv", "ns2")
			scName := "test"
			sc := CreateStorageClass(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			reconciler = createCloneReconciler(dv, sc, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			snapshotClass, err := reconciler.getSnapshotClassForSmartClone(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapshotClass).To(BeEmpty())
		})

		It("Should err, if source PVC doesn't exist", func() {
			dv := newCloneDataVolumeWithPVCNS("test-dv", "ns2")
			scName := "test"
			sc := CreateStorageClass(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			reconciler = createCloneReconciler(dv, sc, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			possible, err := reconciler.advancedClonePossible(dv, dv.Spec.PVC)
			Expect(err).To(HaveOccurred())
			Expect(possible).To(BeFalse())
		})

		It("Should not allow smart clone, if source PVC exist, but no storage class exists, and no storage class in PVC def", func() {
			dv := newCloneDataVolume("test-dv")
			pvc := CreatePvc("test", metav1.NamespaceDefault, nil, nil)
			reconciler = createCloneReconciler(dv, pvc)
			possible, err := reconciler.advancedClonePossible(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(possible).To(BeFalse())
		})

		It("Should not allow smart clone, if source SC and target SC do not match", func() {
			dv := newCloneDataVolume("test-dv")
			targetSc := "testsc"
			tsc := CreateStorageClass(targetSc, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			dv.Spec.PVC.StorageClassName = &targetSc
			sourceSc := "testsc2"
			ssc := CreateStorageClass(sourceSc, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &sourceSc, nil, nil, corev1.ClaimBound)
			reconciler = createCloneReconciler(ssc, tsc, dv, pvc)
			possible, err := reconciler.advancedClonePossible(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(possible).To(BeFalse())
		})

		It("Should not return snapshot class, if storage class does not exist", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			reconciler = createCloneReconciler(dv, pvc, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			snapclass, err := reconciler.getSnapshotClassForSmartClone(dv, dv.Spec.PVC)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to retrieve storage class"))
			Expect(snapclass).To(BeEmpty())
		})

		It("Should not return snapshot class, if storage class exists but snapshot class does not exist", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClass(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			})
			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			reconciler = createCloneReconciler(sc, dv, pvc)
			snapclass, err := reconciler.getSnapshotClassForSmartClone(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapclass).To(BeEmpty())
		})

		It("Should return snapshot class, everything is available", func() {
			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			dv.Spec.PVC.StorageClassName = &scName
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			expectedSnapshotClass := "snap-class"
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createCloneReconciler(sc, dv, pvc, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			snapclass, err := reconciler.getSnapshotClassForSmartClone(dv, dv.Spec.PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(snapclass).To(Equal(expectedSnapshotClass))
		})

		DescribeTable("Setting clone strategy affects the output of getGlobalCloneStrategyOverride", func(expectedCloneStrategy cdiv1.CDICloneStrategy) {
			dv := newCloneDataVolume("test-dv")
			reconciler = createCloneReconciler(dv)

			cr := &cdiv1.CDI{}
			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "cdi"}, cr)
			Expect(err).ToNot(HaveOccurred())

			cr.Spec.CloneStrategyOverride = &expectedCloneStrategy
			err = reconciler.client.Update(context.TODO(), cr)
			Expect(err).ToNot(HaveOccurred())

			cloneStrategy, err := reconciler.getGlobalCloneStrategyOverride()
			Expect(err).ToNot(HaveOccurred())
			Expect(*cloneStrategy).To(Equal(expectedCloneStrategy))
		},
			Entry("copy", cdiv1.CloneStrategyHostAssisted),
			Entry("snapshot", cdiv1.CloneStrategySnapshot),
		)

		DescribeTable("After smart clone", func(actualSize resource.Quantity, currentSize resource.Quantity, expectedDvPhase cdiv1.DataVolumePhase) {
			strategy := cdiv1.CloneStrategySnapshot
			controller := true

			dv := newCloneDataVolume("test-dv")
			scName := "testsc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			storageProfile := createStorageProfileWithCloneStrategy(scName,
				[]cdiv1.ClaimPropertySet{{AccessModes: accessMode, VolumeMode: &BlockMode}},
				&strategy)
			snapshotClassName := "snap-class"
			snapClass := createSnapshotClass(snapshotClassName, nil, "csi-plugin")

			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			targetPvc.Spec.Resources.Requests[corev1.ResourceStorage] = currentSize
			targetPvc.Status.Capacity[corev1.ResourceStorage] = actualSize
			targetPvc.SetAnnotations(make(map[string]string))
			targetPvc.GetAnnotations()[AnnCloneOf] = "true"

			reconciler = createCloneReconciler(dv, srcPvc, targetPvc, storageProfile, sc, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By(fmt.Sprintf("Verifying that dv phase is now in %s", expectedDvPhase))
			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(expectedDvPhase))

			By("Verifying that pvc request size as expected")
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1G")))

		},
			Entry("Should expand pvc when actual and current differ then the requested size", resource.MustParse("500M"), resource.MustParse("500M"), cdiv1.ExpansionInProgress),
			Entry("Should update request size when current size differ then the requested size and actual size is bigger then both", resource.MustParse("2G"), resource.MustParse("500M"), cdiv1.ExpansionInProgress),
			Entry("Should update request size when current size differ from requested size", resource.MustParse("1G"), resource.MustParse("500M"), cdiv1.ExpansionInProgress),
			Entry("Should complete clone in case all sizes match", resource.MustParse("1G"), resource.MustParse("1G"), cdiv1.Succeeded),
		)
	})

	var _ = Describe("CSI clone", func() {
		DescribeTable("Starting from Failed DV",
			func(targetPvcPhase corev1.PersistentVolumeClaimPhase, expectedDvPhase cdiv1.DataVolumePhase) {
				strategy := cdiv1.CloneStrategyCsiClone
				controller := true

				dv := newCloneDataVolume("test-dv")
				dv.Status.Phase = cdiv1.Failed

				scName := "testsc"
				srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
				targetPvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, targetPvcPhase)
				targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
					Kind:       "DataVolume",
					Controller: &controller,
					Name:       "test-dv",
					UID:        dv.UID,
				})
				sc := CreateStorageClassWithProvisioner(scName, map[string]string{
					AnnDefaultStorageClass: "true",
				}, map[string]string{}, "csi-plugin")

				accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
				storageProfile := createStorageProfileWithCloneStrategy(scName,
					[]cdiv1.ClaimPropertySet{{AccessModes: accessMode, VolumeMode: &BlockMode}},
					&strategy)

				reconciler = createCloneReconciler(dv, srcPvc, targetPvc, storageProfile, sc)

				By("Reconcile")
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Not(BeNil()))

				By(fmt.Sprintf("Verifying that phase is now in %s", expectedDvPhase))
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Status.Phase).To(Equal(expectedDvPhase))

			},
			Entry("Should be in progress, if source pvc is ClaimPending", corev1.ClaimPending, cdiv1.CSICloneInProgress),
			Entry("Should be failed, if source pvc is ClaimLost", corev1.ClaimLost, cdiv1.Failed),
			Entry("Should be Succeeded, if source pvc is ClaimBound", corev1.ClaimBound, cdiv1.Succeeded),
		)

		It("Should not panic if CSI Driver not available and no storage class on PVC spec", func() {
			strategy := cdiv1.CDICloneStrategy(cdiv1.CloneStrategyCsiClone)

			dv := newCloneDataVolume("test-dv")

			scName := "testsc"
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")

			accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
			storageProfile := createStorageProfileWithCloneStrategy(scName,
				[]cdiv1.ClaimPropertySet{{AccessModes: accessMode, VolumeMode: &BlockMode}},
				&strategy)

			reconciler := createCloneReconciler(dv, srcPvc, storageProfile, sc, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())

			By("Reconcile")
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

	})

	var _ = Describe("Clone without source", func() {
		scName := "testsc"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, map[string]string{}, "csi-plugin")

		It("Validate clone without source as feasible, but not done", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			reconciler = createCloneReconciler(dv, storageProfile, sc)

			done, err := reconciler.validateCloneAndSourcePVC(dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
		})

		It("Validate that clone without source completes after PVC is created", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			reconciler = createCloneReconciler(dv, storageProfile, sc)

			done, err := reconciler.validateCloneAndSourcePVC(dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())

			// We create the source PVC after creating the clone
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			done, err = reconciler.validateCloneAndSourcePVC(dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
		})

		It("Validate clone already populated without source completes", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			pvcAnnotations := make(map[string]string)
			pvcAnnotations[AnnPopulatedFor] = "test-dv"
			pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPopulatedFor] = "test-dv"
			reconciler = createCloneReconciler(dv, pvc, storageProfile, sc)

			//prePopulated := false
			//pvcPopulated := true
			result, err := reconciler.reconcileClone(reconciler.log, dv, pvc, dv.Spec.PVC.DeepCopy(), "")
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.ClaimName).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(dv.Annotations[AnnPrePopulated]).To(Equal("test-dv"))
			Expect(dv.Annotations[annCloneType]).To(BeEmpty())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		DescribeTable("Validation mechanism rejects or accepts the clone depending on the contentType combination",
			func(sourceContentType, targetContentType string, expectedResult bool) {
				dv := newCloneDataVolume("test-dv")
				dv.Spec.ContentType = cdiv1.DataVolumeContentType(targetContentType)
				storageProfile := createStorageProfile(scName, nil, FilesystemMode)
				reconciler = createCloneReconciler(dv, storageProfile, sc)

				done, err := reconciler.validateCloneAndSourcePVC(dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeFalse())

				// We create the source PVC after creating the clone
				pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, map[string]string{
					AnnContentType: sourceContentType}, nil, corev1.ClaimBound)
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())

				done, err = reconciler.validateCloneAndSourcePVC(dv)
				Expect(done).To(Equal(expectedResult))
				if expectedResult == false {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry("Archive in source and target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeArchive), true),
			Entry("Archive in source and KubeVirt in target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeKubeVirt), false),
			Entry("KubeVirt in source and archive in target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeArchive), false),
			Entry("KubeVirt in source and target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeKubeVirt), true),
			Entry("Empty (KubeVirt by default) in source and target", "", "", true),
			Entry("Empty (KubeVirt by default) in source and KubeVirt (explicit) in target", "", string(cdiv1.DataVolumeKubeVirt), true),
			Entry("KubeVirt (explicit) in source and empty (KubeVirt by default) in target", string(cdiv1.DataVolumeKubeVirt), "", true),
			Entry("Empty (kubeVirt by default) in source and archive in target", "", string(cdiv1.DataVolumeArchive), false),
			Entry("Archive in source and empty (KubeVirt by default) in target", string(cdiv1.DataVolumeArchive), "", false),
		)
	})

	var _ = Describe("Clone strategy", func() {
		var (
			hostAssisted = cdiv1.CloneStrategyHostAssisted
			snapshot     = cdiv1.CloneStrategySnapshot
			csiClone     = cdiv1.CloneStrategyCsiClone
		)

		DescribeTable("Setting clone strategy affects the output of getCloneStrategy",
			func(override, preferredCloneStrategy *cdiv1.CDICloneStrategy, expectedCloneStrategy cdiv1.CDICloneStrategy) {
				dv := newCloneDataVolume("test-dv")
				scName := "testsc"
				pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
				sc := CreateStorageClassWithProvisioner(scName, map[string]string{
					AnnDefaultStorageClass: "true",
				}, map[string]string{}, "csi-plugin")

				accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
				storageProfile := createStorageProfileWithCloneStrategy(scName,
					[]cdiv1.ClaimPropertySet{{AccessModes: accessMode, VolumeMode: &BlockMode}},
					preferredCloneStrategy)

				reconciler = createCloneReconciler(dv, pvc, storageProfile, sc)

				cr := &cdiv1.CDI{}
				err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "cdi"}, cr)
				Expect(err).ToNot(HaveOccurred())

				cr.Spec.CloneStrategyOverride = override
				err = reconciler.client.Update(context.TODO(), cr)
				Expect(err).ToNot(HaveOccurred())

				cloneStrategy, err := reconciler.getCloneStrategy(dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*cloneStrategy).To(Equal(expectedCloneStrategy))
			},
			Entry("override hostAssisted /host", &hostAssisted, &hostAssisted, cdiv1.CloneStrategyHostAssisted),
			Entry("override hostAssisted /snapshot", &hostAssisted, &snapshot, cdiv1.CloneStrategyHostAssisted),
			Entry("override hostAssisted /csiClone", &hostAssisted, &csiClone, cdiv1.CloneStrategyHostAssisted),
			Entry("override hostAssisted /nil", &hostAssisted, nil, cdiv1.CloneStrategyHostAssisted),

			Entry("override snapshot /host", &snapshot, &hostAssisted, cdiv1.CloneStrategySnapshot),
			Entry("override snapshot /snapshot", &snapshot, &snapshot, cdiv1.CloneStrategySnapshot),
			Entry("override snapshot /csiClone", &snapshot, &csiClone, cdiv1.CloneStrategySnapshot),
			Entry("override snapshot /nil", &snapshot, nil, cdiv1.CloneStrategySnapshot),

			Entry("preferred snapshot", nil, &snapshot, cdiv1.CloneStrategySnapshot),
			Entry("preferred hostassisted", nil, &hostAssisted, cdiv1.CloneStrategyHostAssisted),
			Entry("preferred csiClone", nil, &csiClone, cdiv1.CloneStrategyCsiClone),
			Entry("should default to snapshot", nil, nil, cdiv1.CloneStrategySnapshot),
		)
	})

	var _ = Describe("Clone with empty storage size", func() {
		scName := "testsc"
		accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, map[string]string{}, "csi-plugin")

		// detectCloneSize tests

		It("Size-detection fails when source PVC is not attainable", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			targetPvc := &corev1.PersistentVolumeClaim{}
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			reconciler := createCloneReconciler(dv, storageProfile, sc)
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(dv, targetPvc, pvcSpec, HostAssistedClone)
			Expect(err).To(HaveOccurred())
			Expect(done).To(BeFalse())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Size-detection fails when source PVC is not fully imported", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, HostAssistedClone)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, ImportPVCNotReady) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Size-detection fails when Pod is not ready", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, HostAssistedClone)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, SizeDetectionPodNotReady) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())

		})

		It("Size-detection fails when pod's termination message is invalid", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Prepare the size-detection Pod with the required information
			pod := reconciler.makeSizeDetectionPodSpec(pvc, dv)
			pod.Status.Phase = corev1.PodSucceeded
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			// Checks
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, HostAssistedClone)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrInvalidTermMsg))
			Expect(done).To(BeFalse())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pod)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			By("Checking error event recorded")
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("Size-detection pod failed due to"))
		})

		It("Should get the size from the size-detection pod", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Prepare the size-detection Pod with the required information
			pod := reconciler.makeSizeDetectionPodSpec(pvc, dv)
			pod.Status.Phase = corev1.PodSucceeded
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  "100", // Mock value
						},
					},
				},
			}
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			// Get the expected value
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			expectedSize, err := inflateSizeWithOverhead(reconciler.client, int64(100), pvcSpec)
			expectedSizeInt64, _ := expectedSize.AsInt64()

			// Checks
			done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, HostAssistedClone)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(dv.GetAnnotations()[AnnPermissiveClone]).To(Equal("true"))
			targetSize := pvcSpec.Resources.Requests[corev1.ResourceStorage]
			targetSizeInt64, _ := targetSize.AsInt64()
			Expect(targetSizeInt64).To(Equal(expectedSizeInt64))
		})

		It("Should get the size from the source PVC's annotations", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			// Prepare the source PVC with the required annotations
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnVirtualImageSize] = "100" // Mock value
			pvc.GetAnnotations()[AnnSourceCapacity] = string(pvc.Status.Capacity.Storage().String())
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Get the expected value
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
			Expect(err).ToNot(HaveOccurred())
			expectedSize, err := inflateSizeWithOverhead(reconciler.client, int64(100), pvcSpec)
			expectedSizeInt64, _ := expectedSize.AsInt64()

			// Checks
			done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, HostAssistedClone)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(dv.GetAnnotations()[AnnPermissiveClone]).To(Equal("true"))
			targetSize := pvcSpec.Resources.Requests[corev1.ResourceStorage]
			targetSizeInt64, _ := targetSize.AsInt64()
			Expect(targetSizeInt64).To(Equal(expectedSizeInt64))
		})

		DescribeTable("Should automatically collect the clone size from the source PVC's spec",
			func(cloneStrategy cdiv1.CDICloneStrategy, selectedCloneStrategy cloneStrategy, volumeMode corev1.PersistentVolumeMode) {
				dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
				storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
					{AccessModes: accessMode, VolumeMode: &volumeMode}}, &cloneStrategy)

				pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
				pvc.Spec.VolumeMode = &volumeMode
				reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

				pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv)
				Expect(err).ToNot(HaveOccurred())
				expectedSize := *pvc.Status.Capacity.Storage()
				done, err := reconciler.detectCloneSize(dv, pvc, pvcSpec, selectedCloneStrategy)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())
				Expect(pvc.Spec.Resources.Requests.Storage().Cmp(expectedSize)).To(Equal(0))
			},
			Entry("snapshot with empty size and 'Block' volume mode", cdiv1.CloneStrategySnapshot, SmartClone, BlockMode),
			Entry("csiClone with empty size and 'Block' volume mode", cdiv1.CloneStrategyCsiClone, CsiClone, BlockMode),
			Entry("hostAssited with empty size and 'Block' volume mode", cdiv1.CloneStrategyHostAssisted, HostAssistedClone, BlockMode),
			Entry("snapshot with empty size and 'Filesystem' volume mode", cdiv1.CloneStrategySnapshot, SmartClone, FilesystemMode),
			Entry("csiClone with empty size and 'Filesystem' volume mode", cdiv1.CloneStrategyCsiClone, CsiClone, FilesystemMode),
		)
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

func createCloneReconciler(objects ...runtime.Object) *CloneReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createCloneReconcilerWithoutConfig(objs...)
}

func createCloneReconcilerWithoutConfig(objects ...runtime.Object) *CloneReconciler {
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

	sccs := &fakeControllerStarter{}

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &CloneReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvCloneLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
		},
		tokenValidator: &FakeValidator{Match: "foobar"},
		tokenGenerator: &FakeGenerator{token: "foobar"},
		sccs:           sccs,
	}
	r.Reconciler = r
	return r
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
			UID: types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: pvcNamespace,
				},
			},
			PriorityClassName: "p0-clone",
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			},
		},
	}
}

func newCloneDataVolumeWithEmptyStorage(name string, pvcNamespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Annotations: map[string]string{
				AnnCloneToken: "foobar",
			},
			UID: types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: pvcNamespace,
				},
			},
			PriorityClassName: "p0-clone",
			Storage:           &cdiv1.StorageSpec{},
		},
	}
}

type fakeControllerStarter struct{}

func (f *fakeControllerStarter) Start(ctx context.Context) error {
	return nil
}

func (f *fakeControllerStarter) StartController() {
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

type FakeGenerator struct {
	token string
}

func (g *FakeGenerator) Generate(*token.Payload) (string, error) {
	return g.token, nil
}
