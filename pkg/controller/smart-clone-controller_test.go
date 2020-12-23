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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	scLog = logf.Log.WithName("smart-clone-controller-test")
)
var _ = Describe("Smart-clone reconcile functions", func() {
	table.DescribeTable("snapshot", func(annotation string, ready, expectSuccess bool) {
		annotations := make(map[string]string)
		if annotation != "" {
			annotations[annotation] = ""
		}
		val := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
			Status: &snapshotv1.VolumeSnapshotStatus{
				ReadyToUse: &ready,
			},
		}
		Expect(shouldReconcileSnapshot(val)).To(Equal(expectSuccess))
	},
		table.Entry("should reconcile if both annotation exists and ready", AnnSmartCloneRequest, true, true),
		table.Entry("should not reconcile if annotation exists and not ready", AnnSmartCloneRequest, false, false),
		table.Entry("should not reconcile if annotation does not exist and ready", "", true, false),
		table.Entry("should not reconcile if annotation does not exist and not ready", "", false, false),
	)

	table.DescribeTable("pvc", func(key, value string, phase corev1.PersistentVolumeClaimPhase, expectSuccess bool) {
		annotations := make(map[string]string)
		if key != "" {
			annotations[key] = value
		}
		val := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: annotations,
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: phase,
			},
		}
		Expect(shouldReconcilePvc(val)).To(Equal(expectSuccess))
	},
		table.Entry("should reconcile if annotation exists, and is true, and phase is bound", AnnSmartCloneRequest, "true", corev1.ClaimBound, true),
		table.Entry("should not reconcile if annotation exists, and is false, and phase is bound", AnnSmartCloneRequest, "false", corev1.ClaimBound, false),
		table.Entry("should not reconcile if annotation doesn't exist, and phase is bound", "", "true", corev1.ClaimBound, false),
		table.Entry("should not reconcile if annotation exists, and is true, and phase is lost", AnnSmartCloneRequest, "true", corev1.ClaimLost, false),
	)
})

var _ = Describe("Smart-clone controller reconcile loop", func() {
	var (
		reconciler *SmartCloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("should return nil if no pvc or snapshot can be found", func() {
		reconciler := createSmartCloneReconciler()
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should return error if a pvc with no datavolume is passed", func() {
		By("Creating PVC with snapshot source that doesn't match a DV")
		// This should call reconcilePVC, and since the datasource doesn't match it will return an error.
		reconciler := createSmartCloneReconciler(createPVCWithSnapshotSource("test-dv", "invalid"))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(HaveOccurred())
	})

	It("should return an error if a snapshot with no matching dv is passed", func() {
		reconciler := createSmartCloneReconciler(createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Smart-clone controller reconcilePVC loop", func() {
	var (
		reconciler *SmartCloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should error if no matching DV can be found", func() {
		reconciler := createSmartCloneReconciler()
		_, err := reconciler.reconcilePvc(reconciler.log, createPVCWithSnapshotSource("test-dv", "invalid"))
		Expect(err).To(HaveOccurred())
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	})

	It("Should update the DV to success and no snapshot should exist after reconcile", func() {
		reconciler := createSmartCloneReconciler(newCloneDataVolume("test-dv"))
		_, err := reconciler.reconcilePvc(reconciler.log, createPVCWithSnapshotSource("test-dv", "test-dv"))
		Expect(err).ToNot(HaveOccurred())
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Successfully cloned from default/test into default/test-dv"))
		snapshot := &snapshotv1.VolumeSnapshot{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, snapshot)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	})

	It("Should update the DV to success and no snapshot should exist after reconcile, if snapshot existed", func() {
		reconciler := createSmartCloneReconciler(newCloneDataVolume("test-dv"), createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil))
		_, err := reconciler.reconcilePvc(reconciler.log, createPVCWithSnapshotSource("test-dv", "test-dv"))
		Expect(err).ToNot(HaveOccurred())
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Successfully cloned from default/test into default/test-dv"))
		snapshot := &snapshotv1.VolumeSnapshot{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, snapshot)
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	})
})

var _ = Describe("Smart-clone controller reconcileSnapshot loop", func() {
	var (
		reconciler *SmartCloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should error if no matching DV can be found", func() {
		reconciler := createSmartCloneReconciler()
		_, err := reconciler.reconcileSnapshot(reconciler.log, createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil))
		Expect(err).To(HaveOccurred())
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	})

	It("Should error if snapshot has no owner reference, and dv exists", func() {
		reconciler := createSmartCloneReconciler(newCloneDataVolume("test-dv"))
		_, err := reconciler.reconcileSnapshot(reconciler.log, createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error creating new pvc from snapshot object, snapshot has no owner"))
		By("Checking that the DV phase has been marked in progress")
		datavolume := &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, datavolume)
		Expect(err).ToNot(HaveOccurred())
		Expect(datavolume.Status.Phase).To(Equal(cdiv1.SmartClonePVCInProgress))
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Creating PVC for smart-clone is in progress"))
	})

	It("Should create a new if snapshot has an owner reference, and dv exists", func() {
		controller := true
		reconciler := createSmartCloneReconciler(newCloneDataVolume("test-dv"))
		_, err := reconciler.reconcileSnapshot(reconciler.log, createSnapshotVolume("test-dv", metav1.NamespaceDefault, &metav1.OwnerReference{
			Controller: &controller,
		}))
		Expect(err).ToNot(HaveOccurred())
		By("Checking that the DV phase has been marked in progress")
		datavolume := &cdiv1.DataVolume{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, datavolume)
		Expect(err).ToNot(HaveOccurred())
		Expect(datavolume.Status.Phase).To(Equal(cdiv1.SmartClonePVCInProgress))
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Creating PVC for smart-clone is in progress"))
		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Spec.DataSource).ToNot(BeNil())
	})
})

func createSmartCloneReconciler(objects ...runtime.Object) *SmartCloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)

	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(1)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &SmartCloneReconciler{
		client:   cl,
		scheme:   s,
		log:      scLog,
		recorder: rec,
	}
	return r
}

func createPVCWithSnapshotSource(name, snapshotName string) *corev1.PersistentVolumeClaim {
	pvc := createPvc(name, metav1.NamespaceDefault, map[string]string{}, nil)
	pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
		Name:     snapshotName,
		Kind:     "VolumeSnapshot",
		APIGroup: &snapshotv1.SchemeGroupVersion.Group,
	}
	pvc.Status.Phase = corev1.ClaimBound
	return pvc
}

func createSnapshotVolume(name, namespace string, owner *metav1.OwnerReference) *snapshotv1.VolumeSnapshot {
	ownerRefs := make([]metav1.OwnerReference, 0)
	if owner != nil {
		ownerRefs = append(ownerRefs, *owner)
	}
	return &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
	}
}
