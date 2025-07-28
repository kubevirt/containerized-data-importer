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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	dsName       = "test-datasource"
	pvcName      = "test-pvc"
	snapshotName = "test-snapshot"

	testKubevirtIoKey               = "test.kubevirt.io/test"
	testKubevirtIoValue             = "testvalue"
	testInstancetypeKubevirtIoKey   = "instancetype.kubevirt.io/default-preference"
	testInstancetypeKubevirtIoValue = "testpreference"
	testKubevirtIoKeyExisting       = "test.kubevirt.io/existing"
	testKubevirtIoNewValueExisting  = "newvalue"
)

var _ = Describe("All DataSource Tests", func() {
	var _ = Describe("DataSource controller reconcile loop", func() {
		// verifyConditions reconciles, gets DataSource, and verifies its status conditions
		It("Should do nothing and return nil when no DataSource exists", func() {
			reconciler := createDataSourceReconciler()
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dsName, Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should update Ready condition when DataSource has no source", func() {
			ds := createDataSource(dsName)
			reconciler := createDataSourceReconciler(ds)
			verifyConditions("No source", false, noSource, ds, reconciler)
		})

		It("Should update Ready condition when DataSource has source pvc", func() {
			ds := createDataSource(dsName)
			ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}
			reconciler := createDataSourceReconciler(ds)
			verifyConditions("Source DV does not exist", false, NotFound, ds, reconciler)

			dv := NewImportDataVolume(pvcName)
			err := reconciler.client.Create(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			dv.Status.Phase = cdiv1.ImportInProgress
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV ImportInProgress", false, string(dv.Status.Phase), ds, reconciler)

			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV Succeeded", true, ready, ds, reconciler)

			err = reconciler.client.Delete(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV Deleted", false, NotFound, ds, reconciler)

			pvc := CreatePvc(pvcName, metav1.NamespaceDefault, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source PVC exists, but no DV", true, ready, ds, reconciler)

			err = reconciler.client.Delete(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source PVC Deleted", false, NotFound, ds, reconciler)
		})

		It("Should update Ready condition when DataSource has source snapshot", func() {
			ds := createDataSource(dsName)
			ds.Spec.Source.Snapshot = &cdiv1.DataVolumeSourceSnapshot{Namespace: metav1.NamespaceDefault, Name: snapshotName}
			reconciler := createDataSourceReconciler(ds)
			verifyConditions("Source snapshot does not exist", false, NotFound, ds, reconciler)

			snap := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: snapshotv1.VolumeSnapshotSpec{},
				Status: &snapshotv1.VolumeSnapshotStatus{
					ReadyToUse: ptr.To[bool](false),
				},
			}
			err := reconciler.client.Create(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source snapshot not ready", false, "SnapshotNotReady", ds, reconciler)

			snap.Status.ReadyToUse = ptr.To[bool](true)
			err = reconciler.client.Update(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source snapshot ready", true, ready, ds, reconciler)

			err = reconciler.client.Delete(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source snapshot Deleted", false, NotFound, ds, reconciler)
		})

		DescribeTable("Should copy labels to DataSource", func(source cdiv1.DataSourceSource, createSource func(reconciler *DataSourceReconciler) error) {
			ds := createDataSource(dsName)
			ds.Spec.Source = source
			reconciler := createDataSourceReconciler(ds)
			verifyConditions("Source does not exist", false, NotFound, ds, reconciler)

			Expect(createSource(reconciler)).To(Succeed())
			verifyConditions("DataSource is ready to be consumed", true, ready, ds, reconciler)

			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dsName, Namespace: metav1.NamespaceDefault}, ds)
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
			Expect(ds.Labels).To(HaveKeyWithValue(testInstancetypeKubevirtIoKey, testInstancetypeKubevirtIoValue))
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKeyExisting, testKubevirtIoNewValueExisting))
		},
			Entry("from DataVolume",
				cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}},
				func(reconciler *DataSourceReconciler) error {
					dv := NewImportDataVolume(pvcName)
					dv.Status.Phase = cdiv1.Succeeded
					dv.Labels = map[string]string{
						testKubevirtIoKey:             testKubevirtIoValue,
						testInstancetypeKubevirtIoKey: testInstancetypeKubevirtIoValue,
						testKubevirtIoKeyExisting:     testKubevirtIoNewValueExisting,
					}
					return reconciler.client.Create(context.TODO(), dv)
				},
			),
			Entry("from PersistentVolumeClaim",
				cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}},
				func(reconciler *DataSourceReconciler) error {
					pvc := CreatePvc(pvcName, metav1.NamespaceDefault, nil, map[string]string{
						testKubevirtIoKey:             testKubevirtIoValue,
						testInstancetypeKubevirtIoKey: testInstancetypeKubevirtIoValue,
						testKubevirtIoKeyExisting:     testKubevirtIoNewValueExisting,
					})
					return reconciler.client.Create(context.TODO(), pvc)
				},
			),
			Entry("from VolumeSnapshot",
				cdiv1.DataSourceSource{Snapshot: &cdiv1.DataVolumeSourceSnapshot{Namespace: metav1.NamespaceDefault, Name: snapshotName}},
				func(reconciler *DataSourceReconciler) error {
					snap := &snapshotv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      snapshotName,
							Namespace: metav1.NamespaceDefault,
							Labels: map[string]string{
								testKubevirtIoKey:             testKubevirtIoValue,
								testInstancetypeKubevirtIoKey: testInstancetypeKubevirtIoValue,
								testKubevirtIoKeyExisting:     testKubevirtIoNewValueExisting,
							},
						},
						Spec: snapshotv1.VolumeSnapshotSpec{},
						Status: &snapshotv1.VolumeSnapshotStatus{
							ReadyToUse: ptr.To[bool](true),
						},
					}
					return reconciler.client.Create(context.TODO(), snap)
				},
			),
		)

		Describe("DataSource pointers", func() {
			It("DataSource pointer should resolve reference DataSource and reach ready to be consumed state", func() {
				ds := createDataSource(dsName)
				ds.Spec.Source = cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}}
				dsPointer := createDataSource(dsName + "-pointer")
				dsPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: metav1.NamespaceDefault, Name: ds.Name}}
				reconciler := createDataSourceReconciler(ds, dsPointer)
				verifyConditions("Source does not exist", false, NotFound, ds, reconciler)

				pvc := CreatePvc(pvcName, metav1.NamespaceDefault, nil, nil)
				Expect(reconciler.client.Create(context.TODO(), pvc)).To(Succeed())
				verifyConditions("DataSource is ready to be consumed", true, ready, ds, reconciler)

				dsPointerKey := types.NamespacedName{Name: dsPointer.Name, Namespace: metav1.NamespaceDefault}
				verifyConditions("DataSource is ready to be consumed", true, ready, dsPointer, reconciler)
				Expect(reconciler.client.Get(context.TODO(), dsPointerKey, dsPointer)).To(Succeed())
				Expect(dsPointer.Spec.Source.DataSource.Name).To(Equal(ds.Name))
				Expect(dsPointer.Status.Source).To(Equal(ds.Spec.Source))
			})
			It("DataSource pointer should fail to resolve non-existing DataSource", func() {
				dsPointer := createDataSource(dsName + "-pointer")
				dsPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: metav1.NamespaceDefault, Name: "non-existent"}}
				reconciler := createDataSourceReconciler(dsPointer)
				dsPointerKey := types.NamespacedName{Name: dsPointer.Name, Namespace: metav1.NamespaceDefault}
				verifyConditions("Source does not exist", false, NotFound, dsPointer, reconciler)
				Expect(reconciler.client.Get(context.TODO(), dsPointerKey, dsPointer)).To(Succeed())
				Expect(dsPointer.Spec.Source.DataSource.Name).To(Equal("non-existent"))
				Expect(dsPointer.Status.Source.DataSource).To(BeNil())
			})
			It("DataSource pointer should fail to resolve reference with depth exceeding 1", func() {
				ds := createDataSource(dsName)
				ds.Spec.Source = cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}}
				dsPointer := createDataSource(dsName + "-pointer")
				dsPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: metav1.NamespaceDefault, Name: ds.Name}}
				pointerToPointer := createDataSource(dsName + "-pointer-depth-2")
				pointerToPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: metav1.NamespaceDefault, Name: dsPointer.Name}}
				reconciler := createDataSourceReconciler(ds, dsPointer, pointerToPointer)
				dsPointerKey := types.NamespacedName{Name: dsPointer.Name, Namespace: metav1.NamespaceDefault}
				verifyConditions("Pointer to pointer max reference depth reached", false, maxReferenceDepthReached, pointerToPointer, reconciler)
				Expect(reconciler.client.Get(context.TODO(), dsPointerKey, dsPointer)).To(Succeed())
				Expect(dsPointer.Status.Source.DataSource).To(BeNil())
			})
			It("DataSource pointer should fail to resolve self-reference", func() {
				dsPointer := createDataSource(dsName + "-pointer")
				dsPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: dsPointer.Namespace, Name: dsPointer.Name}}
				reconciler := createDataSourceReconciler(dsPointer)
				dsPointerKey := types.NamespacedName{Name: dsPointer.Name, Namespace: metav1.NamespaceDefault}
				verifyConditions("DataSource self-reference", false, selfReference, dsPointer, reconciler)
				Expect(reconciler.client.Get(context.TODO(), dsPointerKey, dsPointer)).To(Succeed())
				Expect(dsPointer.Spec.Source.DataSource.Name).To(Equal(dsPointer.Name))
				Expect(dsPointer.Status.Source.DataSource).To(BeNil())
			})
			It("DataSource pointer should fail to resolve cross-namespace DataSource", func() {
				dsPointer := createDataSource(dsName + "-pointer")
				dsPointer.Spec.Source = cdiv1.DataSourceSource{DataSource: &cdiv1.DataSourceRefSourceDataSource{Namespace: "differentNamespace", Name: dsPointer.Name}}
				reconciler := createDataSourceReconciler(dsPointer)
				dsPointerKey := types.NamespacedName{Name: dsPointer.Name, Namespace: metav1.NamespaceDefault}
				verifyConditions("DataSource cross-namespace reference", false, crossNamespaceReference, dsPointer, reconciler)
				Expect(reconciler.client.Get(context.TODO(), dsPointerKey, dsPointer)).To(Succeed())
				Expect(dsPointer.Spec.Source.DataSource.Name).To(Equal(dsPointer.Name))
				Expect(dsPointer.Status.Source.DataSource).To(BeNil())
			})
		})

	})
})

func createDataSourceReconciler(objects ...runtime.Object) *DataSourceReconciler {
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objects...).Build()
	r := &DataSourceReconciler{
		client: cl,
		scheme: s,
		log:    cronLog,
	}
	return r
}

func createDataSource(name string) *cdiv1.DataSource {
	return &cdiv1.DataSource{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				testKubevirtIoKeyExisting: "existing",
			},
		},
	}
}

func verifyConditions(step string, isReady bool, reasonReady string, ds *cdiv1.DataSource, reconciler *DataSourceReconciler) {
	By(step)
	key := types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}
	req := reconcile.Request{NamespacedName: key}
	_, err := reconciler.Reconcile(context.TODO(), req)
	Expect(err).ToNot(HaveOccurred())
	err = reconciler.client.Get(context.TODO(), key, ds)
	Expect(err).ToNot(HaveOccurred())
	dsCond := FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
	Expect(dsCond).ToNot(BeNil())
	verifyConditionState(string(cdiv1.DataSourceReady), dsCond.ConditionState, isReady, reasonReady)
}
