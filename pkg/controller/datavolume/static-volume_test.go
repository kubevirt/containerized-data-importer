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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("checkStaticVolume Tests", func() {
	const dvName = "static-check-dv"

	type testArgs struct {
		newDV         func() *cdiv1.DataVolume
		newReconciler func(...runtime.Object) (reconcile.Reconciler, client.Client)
	}

	addAnno := func(obj metav1.Object) {
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(make(map[string]string))
		}
		obj.GetAnnotations()[AnnCheckStaticVolume] = ""
	}

	newImportDV := func() *cdiv1.DataVolume {
		dv := NewImportDataVolume(dvName)
		addAnno(dv)
		return dv
	}

	importReconciler := func(objects ...runtime.Object) (reconcile.Reconciler, client.Client) {
		r := createImportReconciler(objects...)
		return r, r.client
	}

	newImportArgs := func() *testArgs {
		return &testArgs{
			newDV:         newImportDV,
			newReconciler: importReconciler,
		}
	}

	newUploadDV := func() *cdiv1.DataVolume {
		dv := newUploadDataVolume(dvName)
		addAnno(dv)
		return dv
	}

	uploadReconciler := func(objects ...runtime.Object) (reconcile.Reconciler, client.Client) {
		r := createUploadReconciler(objects...)
		return r, r.client
	}

	newUploadArgs := func() *testArgs {
		return &testArgs{
			newDV:         newUploadDV,
			newReconciler: uploadReconciler,
		}
	}

	newCloneDV := func() *cdiv1.DataVolume {
		dv := newCloneDataVolume("test-dv")
		addAnno(dv)
		return dv
	}

	cloneReconciler := func(objects ...runtime.Object) (reconcile.Reconciler, client.Client) {
		r := createCloneReconciler(objects...)
		return r, r.client
	}

	newCloneArgs := func() *testArgs {
		return &testArgs{
			newDV:         newCloneDV,
			newReconciler: cloneReconciler,
		}
	}

	newSnapshotCloneDV := func() *cdiv1.DataVolume {
		dv := newCloneFromSnapshotDataVolume("test-dv")
		addAnno(dv)
		return dv
	}

	snapshotCloneReconciler := func(objects ...runtime.Object) (reconcile.Reconciler, client.Client) {
		r := createSnapshotCloneReconciler(objects...)
		return r, r.client
	}

	newSnapshotCloneArgs := func() *testArgs {
		return &testArgs{
			newDV:         newSnapshotCloneDV,
			newReconciler: snapshotCloneReconciler,
		}
	}

	newPopulatorDV := func() *cdiv1.DataVolume {
		pvcDataSource := &corev1.TypedLocalObjectReference{
			Kind: "PersistentVolumeClaim",
			Name: "test",
		}
		dv := newPopulatorDataVolume("test-dv", pvcDataSource, nil)
		addAnno(dv)
		return dv
	}

	populatorReconciler := func(objects ...runtime.Object) (reconcile.Reconciler, client.Client) {
		r := createPopulatorReconciler(objects...)
		return r, r.client
	}

	newPopulatorArgs := func() *testArgs {
		return &testArgs{
			newDV:         newPopulatorDV,
			newReconciler: populatorReconciler,
		}
	}

	newAvailablePV := func(dv *cdiv1.DataVolume) *corev1.PersistentVolume {
		return &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv1",
			},
			Spec: corev1.PersistentVolumeSpec{
				ClaimRef: &corev1.ObjectReference{
					Name:      dv.Name,
					Namespace: dv.Namespace,
				},
			},
			Status: corev1.PersistentVolumeStatus{
				Phase: corev1.VolumeAvailable,
			},
		}
	}

	newPendingPVC := func(dv *cdiv1.DataVolume, pv *corev1.PersistentVolume) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dv.Namespace,
				Name:      dv.Name,
				Annotations: map[string]string{
					AnnPersistentVolumeList: fmt.Sprintf("[\"%s\"]", pv.Name),
				},
			},
		}
	}

	requestFunc := func(dv *cdiv1.DataVolume) reconcile.Request {
		return reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: dv.Namespace,
				Name:      dv.Name,
			},
		}
	}

	pvcFunc := func(c client.Client, dv *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dv.Namespace,
				Name:      dv.Name,
			},
		}
		err := c.Get(context.TODO(), client.ObjectKeyFromObject(pvc), pvc)
		return pvc, err
	}

	DescribeTable("should do nothing special if no PV exists", func(args *testArgs) {
		dv := args.newDV()
		reconciler, client := args.newReconciler(dv)
		_, err := reconciler.Reconcile(context.TODO(), requestFunc(dv))
		Expect(err).ToNot(HaveOccurred())

		pvc, err := pvcFunc(client, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Annotations).ToNot(HaveKey(AnnPersistentVolumeList))
	},
		Entry("with import DataVolume", newImportArgs()),
		Entry("with upload DataVolume", newUploadArgs()),
		Entry("with populator DataVolume", newPopulatorArgs()),
	)

	DescribeTable("should create PVC with persistentVolumeList annotation", func(args *testArgs) {
		dv := args.newDV()
		pv := newAvailablePV(dv)
		reconciler, client := args.newReconciler(dv, pv)
		_, err := reconciler.Reconcile(context.TODO(), requestFunc(dv))
		Expect(err).ToNot(HaveOccurred())

		pvc, err := pvcFunc(client, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Annotations[AnnPersistentVolumeList]).To(Equal(fmt.Sprintf("[\"%s\"]", pv.Name)))
	},
		Entry("with import DataVolume", newImportArgs()),
		Entry("with upload DataVolume", newUploadArgs()),
		Entry("with clone DataVolume", newCloneArgs()),
		Entry("with snapshot clone DataVolume", newSnapshotCloneArgs()),
		Entry("with populator DataVolume", newPopulatorArgs()),
	)

	DescribeTable("should do nothing if PVC not bound", func(args *testArgs) {
		dv := args.newDV()
		pv := newAvailablePV(dv)
		pvc := newPendingPVC(dv, pv)
		reconciler, client := args.newReconciler(dv, pv, pvc)
		_, err := reconciler.Reconcile(context.TODO(), requestFunc(dv))
		Expect(err).ToNot(HaveOccurred())

		pvc, err = pvcFunc(client, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Annotations[AnnPersistentVolumeList]).To(Equal(fmt.Sprintf("[\"%s\"]", pv.Name)))
		Expect(pvc.Annotations).ToNot(HaveKey(AnnPopulatedFor))
		Expect(pvc.Spec.VolumeName).To(BeEmpty())
	},
		Entry("with import DataVolume", newImportArgs()),
		Entry("with upload DataVolume", newUploadArgs()),
		Entry("with clone DataVolume", newCloneArgs()),
		Entry("with snapshot clone DataVolume", newSnapshotCloneArgs()),
		Entry("with populator DataVolume", newPopulatorArgs()),
	)

	DescribeTable("should remove persistentVolumeList and add populatedForAnnotation", func(args *testArgs) {
		dv := args.newDV()
		pv := newAvailablePV(dv)
		pvc := newPendingPVC(dv, pv)
		pvc.Spec.VolumeName = pv.Name
		pv.Status.Phase = corev1.VolumeBound
		reconciler, client := args.newReconciler(dv, pv, pvc)
		_, err := reconciler.Reconcile(context.TODO(), requestFunc(dv))
		Expect(err).ToNot(HaveOccurred())

		pvc, err = pvcFunc(client, dv)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc.Annotations).ToNot(HaveKey(AnnPersistentVolumeList))
		Expect(pvc.Annotations).To(HaveKey(AnnPopulatedFor))
		Expect(pvc.Spec.VolumeName).To(Equal(pv.Name))
	},
		Entry("with import DataVolume", newImportArgs()),
		Entry("with upload DataVolume", newUploadArgs()),
		Entry("with clone DataVolume", newCloneArgs()),
		Entry("with snapshot clone DataVolume", newSnapshotCloneArgs()),
		Entry("with populator DataVolume", newPopulatorArgs()),
	)

	DescribeTable("should delete PVC if it gets bound to unknown PV", func(args *testArgs) {
		dv := args.newDV()
		pv := newAvailablePV(dv)
		pvc := newPendingPVC(dv, pv)
		pvc.Spec.VolumeName = "foobar"
		pv.Status.Phase = corev1.VolumeBound
		reconciler, client := args.newReconciler(dv, pv, pvc)
		_, err := reconciler.Reconcile(context.TODO(), requestFunc(dv))
		Expect(err).To(HaveOccurred())

		_, err = pvcFunc(client, dv)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	},
		Entry("with import DataVolume", newImportArgs()),
		Entry("with upload DataVolume", newUploadArgs()),
		Entry("with clone DataVolume", newCloneArgs()),
		Entry("with snapshot clone DataVolume", newSnapshotCloneArgs()),
		Entry("with populator DataVolume", newPopulatorArgs()),
	)
})
