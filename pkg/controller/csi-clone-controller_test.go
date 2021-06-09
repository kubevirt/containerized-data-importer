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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	csiLog = logf.Log.WithName("csi-clone-controller-test")
)

var _ = Describe("All csi clone tests", func() {
	var _ = Describe("csi-clone reconcile functions", func() {
		table.DescribeTable("pvc", func(key, value string, hasOwner bool, expectSuccess bool) {

			pvc := createPvcOwnedByAnyDv(key, value, hasOwner)
			Expect(shouldReconcileCSIClonePvc(pvc)).To(Equal(expectSuccess))
		},
			table.Entry("should reconcile if annotation exists, and is true", AnnCSICloneRequest, "true", true, true),
			table.Entry("should not reconcile if not owned by DV", AnnCSICloneRequest, "true", false, false),
			table.Entry("should not reconcile if annotation exists, and is false", AnnCSICloneRequest, "false", true, false),
			table.Entry("should not reconcile if annotation doesn't exist", "", "true", true, false),
		)
	})

	var _ = Describe("Csi-clone controller reconcile loop", func() {
		var (
			reconciler *CSICloneReconciler
		)
		AfterEach(func() {
			if reconciler != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
				reconciler = nil
			}
		})

		It("should return nil if no pvc can be found", func() {
			reconciler := createCsiCloneReconciler()
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	var _ = Describe("Csi-clone controller reconcilePVC loop", func() {
		var (
			reconciler *CSICloneReconciler
		)
		AfterEach(func() {
			if reconciler != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
				reconciler = nil
			}
		})

		It("Should return nil if PVC not bound", func() {
			reconciler := createCsiCloneReconciler()
			pvc := createCsiClonePvc()
			pvc.Status.Phase = corev1.ClaimPending
			_, err := reconciler.reconcilePvc(reconciler.log, pvc)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should add cloneOf annotation", func() {
			pvc := createCsiClonePvc()
			reconciler := createCsiCloneReconciler(pvc)

			_, err := reconciler.reconcilePvc(reconciler.log, pvc)
			Expect(err).ToNot(HaveOccurred())

			pvc2 := &corev1.PersistentVolumeClaim{}
			nn := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
			err = reconciler.client.Get(context.TODO(), nn, pvc2)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc2.Annotations["k8s.io/CloneOf"]).To(Equal("true"))
		})
	})
})

func createCsiClonePvc() *corev1.PersistentVolumeClaim {
	return createPvcOwnedByAnyDv(AnnCSICloneRequest, "true", true)
}

func createPvcOwnedByAnyDv(annoKey string, annoValue string, hasOwner bool) *corev1.PersistentVolumeClaim {
	annotations := make(map[string]string)
	if annoKey != "" {
		annotations[annoKey] = annoValue
	}
	controller := true
	ownerReferences := []metav1.OwnerReference{}
	if hasOwner {
		ownerReferences = []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "DataVolume",
				Name:       "someDvName",
				Controller: &controller,
			},
		}
	}

	var val = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testPvc",
			Namespace:       metav1.NamespaceDefault,
			Annotations:     annotations,
			OwnerReferences: ownerReferences,
		},
	}
	val.Status.Phase = corev1.ClaimBound

	return val
}

func createCsiCloneReconciler(objects ...runtime.Object) *CSICloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(1)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &CSICloneReconciler{
		client:   cl,
		scheme:   s,
		log:      scLog,
		recorder: rec,
	}
	return r
}
