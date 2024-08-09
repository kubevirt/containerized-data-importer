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
		var (
			reconciler *DataSourceReconciler
			ds         *cdiv1.DataSource
			dsKey      = types.NamespacedName{Name: dsName, Namespace: metav1.NamespaceDefault}
			dsReq      = reconcile.Request{NamespacedName: dsKey}
		)

		// verifyConditions reconciles, gets DataSource, and verifies its status conditions
		var verifyConditions = func(step string, isReady bool, reasonReady string) {
			By(step)
			_, err := reconciler.Reconcile(context.TODO(), dsReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), dsKey, ds)
			Expect(err).ToNot(HaveOccurred())
			dsCond := FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
			Expect(dsCond).ToNot(BeNil())
			verifyConditionState(string(cdiv1.DataSourceReady), dsCond.ConditionState, isReady, reasonReady)
		}

		It("Should do nothing and return nil when no DataSource exists", func() {
			reconciler = createDataSourceReconciler()
			_, err := reconciler.Reconcile(context.TODO(), dsReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should update Ready condition when DataSource has no source", func() {
			ds = createDataSource()
			reconciler = createDataSourceReconciler(ds)
			verifyConditions("No source", false, noSource)
		})

		It("Should update Ready condition when DataSource has source pvc", func() {
			ds = createDataSource()
			ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}
			reconciler = createDataSourceReconciler(ds)
			verifyConditions("Source DV does not exist", false, NotFound)

			dv := NewImportDataVolume(pvcName)
			err := reconciler.client.Create(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			dv.Status.Phase = cdiv1.ImportInProgress
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV ImportInProgress", false, string(dv.Status.Phase))

			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV Succeeded", true, ready)

			err = reconciler.client.Delete(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source DV Deleted", false, NotFound)

			pvc := CreatePvc(pvcName, metav1.NamespaceDefault, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source PVC exists, but no DV", true, ready)

			err = reconciler.client.Delete(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source PVC Deleted", false, NotFound)
		})

		It("Should update Ready condition when DataSource has source snapshot", func() {
			ds = createDataSource()
			ds.Spec.Source.Snapshot = &cdiv1.DataVolumeSourceSnapshot{Namespace: metav1.NamespaceDefault, Name: snapshotName}
			reconciler = createDataSourceReconciler(ds)
			verifyConditions("Source snapshot does not exist", false, NotFound)

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
			verifyConditions("Source snapshot not ready", false, "SnapshotNotReady")

			snap.Status.ReadyToUse = ptr.To[bool](true)
			err = reconciler.client.Update(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source snapshot ready", true, ready)

			err = reconciler.client.Delete(context.TODO(), snap)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Source snapshot Deleted", false, NotFound)
		})

		DescribeTable("Should copy labels to DataSource", func(source cdiv1.DataSourceSource, createSource func() error) {
			ds = createDataSource()
			ds.Spec.Source = source
			reconciler = createDataSourceReconciler(ds)
			verifyConditions("Source does not exist", false, NotFound)

			Expect(createSource()).To(Succeed())
			verifyConditions("DataSource is ready to be consumed", true, ready)

			err := reconciler.client.Get(context.TODO(), dsKey, ds)
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
			Expect(ds.Labels).To(HaveKeyWithValue(testInstancetypeKubevirtIoKey, testInstancetypeKubevirtIoValue))
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKeyExisting, testKubevirtIoNewValueExisting))
		},
			Entry("from DataVolume",
				cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: metav1.NamespaceDefault, Name: pvcName}},
				func() error {
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
				func() error {
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
				func() error {
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

func createDataSource() *cdiv1.DataSource {
	return &cdiv1.DataSource{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				testKubevirtIoKeyExisting: "existing",
			},
		},
	}
}
