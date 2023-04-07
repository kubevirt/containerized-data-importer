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

	"k8s.io/apimachinery/pkg/api/resource"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
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

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	scLog = logf.Log.WithName("smart-clone-controller-test")
)

var _ = Describe("All smart clone tests", func() {
	var _ = Describe("Smart-clone reconcile functions", func() {
		table.DescribeTable("snapshot", func(annotation string, expectSuccess bool) {
			annotations := make(map[string]string)
			if annotation != "" {
				annotations[annotation] = ""
			}
			val := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
			}
			Expect(shouldReconcileSnapshot(val)).To(Equal(expectSuccess))
		},
			table.Entry("should reconcile if annotation exists", AnnSmartCloneRequest, true),
			table.Entry("should not reconcile if annotation does not exist", "", false),
		)

		table.DescribeTable("pvc", func(key, value string, expectSuccess bool) {
			annotations := make(map[string]string)
			if key != "" {
				annotations[key] = value
			}
			val := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
			}
			Expect(shouldReconcilePvc(val)).To(Equal(expectSuccess))
		},
			table.Entry("should reconcile if annotation exists, and is true", AnnSmartCloneRequest, "true", true),
			table.Entry("should not reconcile if annotation exists, and is false", AnnSmartCloneRequest, "false", false),
			table.Entry("should not reconcile if annotation doesn't exist", "", "true", false),
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
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
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

		It("Should return nil if PVC not bound", func() {
			reconciler := createSmartCloneReconciler()
			pvc := createPVCWithSnapshotSource("test-dv", "invalid")
			pvc.Status.Phase = corev1.ClaimPending
			_, err := reconciler.reconcilePvc(reconciler.log, pvc)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should error with malformed annotation", func() {
			reconciler := createSmartCloneReconciler()
			pvc := createPVCWithSnapshotSource("test-dv", "invalid")
			pvc.Annotations["cdi.kubevirt.io/smartCloneSnapshot"] = "foo/bar/baz"
			_, err := reconciler.reconcilePvc(reconciler.log, pvc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected key format"))
		})

		It("Should add cloneOf annotation and delete snapshot", func() {
			pvc := createPVCWithSnapshotSource("test-dv", "invalid")
			snapshot := createSnapshotVolume("invalid", pvc.Namespace, nil)
			reconciler := createSmartCloneReconciler(pvc, snapshot)

			_, err := reconciler.reconcilePvc(reconciler.log, pvc)
			Expect(err).ToNot(HaveOccurred())

			pvc2 := &corev1.PersistentVolumeClaim{}
			nn := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
			err = reconciler.client.Get(context.TODO(), nn, pvc2)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc2.Annotations["k8s.io/CloneOf"]).To(Equal("true"))

			nn = types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Name}
			err = reconciler.client.Get(context.TODO(), nn, &snapshotv1.VolumeSnapshot{})
			Expect(err).To(HaveOccurred())
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

		It("Okay if no matching DV can be found", func() {
			reconciler := createSmartCloneReconciler()
			_, err := reconciler.reconcileSnapshot(reconciler.log, createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should do nothing if snapshot deleted", func() {
			reconciler := createSmartCloneReconciler()
			snapshot := createSnapshotVolume("test-dv", metav1.NamespaceDefault, nil)
			ts := metav1.Now()
			snapshot.DeletionTimestamp = &ts
			_, err := reconciler.reconcileSnapshot(reconciler.log, snapshot)
			Expect(err).ToNot(HaveOccurred())

			nn := types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Name}
			err = reconciler.client.Get(context.TODO(), nn, &corev1.PersistentVolumeClaim{})
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Should delete snapshot if DataVolume deleted", func() {
			dv := newCloneDataVolume("test-dv")
			ts := metav1.Now()
			dv.DeletionTimestamp = &ts
			snapshot := createSnapshotVolume("invalid", dv.Namespace, nil)
			setAnnOwnedByDataVolume(snapshot, dv)

			reconciler := createSmartCloneReconciler(dv, snapshot)
			_, err := reconciler.reconcileSnapshot(reconciler.log, snapshot)
			Expect(err).ToNot(HaveOccurred())

			nn := types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Name}
			err = reconciler.client.Get(context.TODO(), nn, &snapshotv1.VolumeSnapshot{})
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Should return nil if snapshot not ready", func() {
			dv := newCloneDataVolume("test-dv")
			snapshot := createSnapshotVolume("invalid", dv.Namespace, nil)
			snapshot.Status = &snapshotv1.VolumeSnapshotStatus{
				ReadyToUse: &[]bool{false}[0],
			}
			setAnnOwnedByDataVolume(snapshot, dv)

			reconciler := createSmartCloneReconciler(dv, snapshot)
			_, err := reconciler.reconcileSnapshot(reconciler.log, snapshot)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create PVC if snapshot ready", func() {
			dv := newCloneDataVolume("test-dv")
			q, _ := resource.ParseQuantity("500Mi")
			// Set annotation on DV which we can verify on PVC later
			dv.GetAnnotations()["test"] = "test-value"
			snapshot := createSnapshotVolume(dv.Name, dv.Namespace, nil)
			snapshot.Spec.Source = snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &[]string{"source"}[0],
			}
			snapshot.Status = &snapshotv1.VolumeSnapshotStatus{
				ReadyToUse:  &[]bool{true}[0],
				RestoreSize: &q,
			}
			setAnnOwnedByDataVolume(snapshot, dv)

			reconciler := createSmartCloneReconciler(dv, snapshot)
			_, err := reconciler.reconcileSnapshot(reconciler.log, snapshot)
			Expect(err).ToNot(HaveOccurred())

			pvc := &corev1.PersistentVolumeClaim{}
			nn := types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}
			err = reconciler.client.Get(context.TODO(), nn, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Labels[common.AppKubernetesVersionLabel]).To(Equal("v0.0.0-tests"))
			Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
			// Verify PVC's annotation
			Expect(pvc.GetAnnotations()["test"]).To(Equal("test-value"))
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("Creating PVC for smart-clone is in progress"))
		})
	})

	createSnapshotWithRestoreSize := func(size int64) *snapshotv1.VolumeSnapshot {
		snapshot := createSnapshotVolume("snapshot", "default", nil)
		snapshot.Status.RestoreSize = resource.NewQuantity(size, resource.BinarySI)
		return snapshot
	}

	createCloneDataVolumeWithRequestSizePvc := func(size int64) *cdiv1.DataVolume {
		dv := createCloneDataVolume("testDv", "default", "snapshot", "default")
		sizeQuantity := resource.NewQuantity(size, resource.BinarySI)
		dv.Spec.PVC = &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *sizeQuantity,
				},
			},
		}
		return dv
	}

	createCloneDataVolumeWithRequestSizeStorage := func(size int64) *cdiv1.DataVolume {
		dv := createCloneDataVolume("testDv", "default", "snapshot", "default")
		sizeQuantity := resource.NewQuantity(size, resource.BinarySI)
		dv.Spec.Storage = &cdiv1.StorageSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *sizeQuantity,
				},
			},
		}
		return dv
	}

	table.DescribeTable("newPvcFromSnapshot should return proper size", func(dv *cdiv1.DataVolume, snapshot *snapshotv1.VolumeSnapshot, expectedSize int64, expectedError error) {
		targetPvcSpec := &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		}
		pvc, err := newPvcFromSnapshot(dv, "targetPvc", snapshot, targetPvcSpec)
		if expectedError == nil {
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc).ToNot(BeNil())
			Expect(pvc.Spec.Resources.Requests.Storage().Value()).To(Equal(expectedSize))
		} else {
			Expect(err).To(Equal(expectedError))
		}
	},
		table.Entry("with nil restoreSize", createCloneDataVolume("testDv", "default", "snapshot", "default"), createSnapshotVolume("snapshot", "default", nil), int64(0), fmt.Errorf("snapshot has no RestoreSize")),
		table.Entry("with 0 restoreSize, and no dv size", createCloneDataVolume("testDv", "default", "snapshot", "default"), createSnapshotWithRestoreSize(int64(0)), int64(0), fmt.Errorf("unable to determine restore size of PVC")),
		table.Entry("with negative restoreSize, and no dv size", createCloneDataVolume("testDv", "default", "snapshot", "default"), createSnapshotWithRestoreSize(int64(-1024)), int64(0), fmt.Errorf("snapshot has no RestoreSize")),
		table.Entry("with 0 restoreSize, and set dv pvc size", createCloneDataVolumeWithRequestSizePvc(int64(1024)), createSnapshotWithRestoreSize(int64(0)), int64(1024), nil),
		table.Entry("with negative restoreSize, and set dv pvc size", createCloneDataVolumeWithRequestSizePvc(int64(10240)), createSnapshotWithRestoreSize(int64(-10240)), int64(0), fmt.Errorf("snapshot has no RestoreSize")),
		table.Entry("with postive restoreSize, and set larger dv pvc size", createCloneDataVolumeWithRequestSizePvc(int64(102400)), createSnapshotWithRestoreSize(int64(1024)), int64(1024), nil),
		table.Entry("with 0 restoreSize, and set dv size storage", createCloneDataVolumeWithRequestSizeStorage(int64(2048)), createSnapshotWithRestoreSize(int64(0)), int64(2048), nil),
		table.Entry("with negative restoreSize, and set dv size storage", createCloneDataVolumeWithRequestSizeStorage(int64(20480)), createSnapshotWithRestoreSize(int64(-20480)), int64(0), fmt.Errorf("snapshot has no RestoreSize")),
		table.Entry("with postive restoreSize, and set larger dv size storage", createCloneDataVolumeWithRequestSizeStorage(int64(204800)), createSnapshotWithRestoreSize(int64(2048)), int64(2048), nil),
	)
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
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "testing",
			common.AppKubernetesVersionLabel: "v0.0.0-tests",
		},
	}
	return r
}

func createPVCWithSnapshotSource(name, snapshotName string) *corev1.PersistentVolumeClaim {
	pvc := createPvc(name, metav1.NamespaceDefault, map[string]string{}, nil)
	pvc.Annotations = map[string]string{
		"cdi.kubevirt.io/smartCloneSnapshot": metav1.NamespaceDefault + "/" + snapshotName,
	}
	pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
		Name:     snapshotName,
		Kind:     "VolumeSnapshot",
		APIGroup: &snapshotv1.SchemeGroupVersion.Group,
	}
	pvc.Status.Phase = corev1.ClaimBound
	return pvc
}

func createSnapshotVolume(name, namespace string, owner *metav1.OwnerReference) *snapshotv1.VolumeSnapshot {
	var ownerRefs []metav1.OwnerReference
	if owner != nil {
		ownerRefs = append(ownerRefs, *owner)
	}
	return &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
		Status: &snapshotv1.VolumeSnapshotStatus{},
	}
}

func createCloneDataVolume(name, namespace, pvcName, pvcNamespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      pvcName,
					Namespace: pvcNamespace,
				},
			},
		},
	}
}
