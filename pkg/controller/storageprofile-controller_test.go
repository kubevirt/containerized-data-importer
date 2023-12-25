/*
Copyright 2021 The CDI Authors.

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
	"reflect"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
)

const (
	storageClassName  = "testSC"
	snapshotClassName = "testSnapClass"
	cephProvisioner   = "rook-ceph.rbd.csi.ceph.com"
)

var (
	storageProfileLog = logf.Log.WithName("storageprofile-controller-test")
	lsoLabels         = map[string]string{
		"local.storage.openshift.io/owner-name":      "local",
		"local.storage.openshift.io/owner-namespace": "openshift-local-storage",
	}
)

var _ = Describe("Storage profile controller reconcile loop", func() {
	var reconciler *StorageProfileReconciler

	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should not requeue if storage class can not be found", func() {
		reconciler = createStorageProfileReconciler()
		res, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(res.Requeue).ToNot(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(BeEmpty())
	})

	It("Should give correct results and not panic with isIncomplete", func() {
		storageProfile := cdiv1.StorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Spec: cdiv1.StorageProfileSpec{
				ClaimPropertySets: []cdiv1.ClaimPropertySet{
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}},
				},
			},
		}
		incomplete := isIncomplete(storageProfile.Status.ClaimPropertySets)
		Expect(incomplete).To(BeTrue())
		storageProfile = cdiv1.StorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Spec: cdiv1.StorageProfileSpec{
				ClaimPropertySets: []cdiv1.ClaimPropertySet{
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}},
				},
			},
			Status: cdiv1.StorageProfileStatus{},
		}
		incomplete = isIncomplete(storageProfile.Status.ClaimPropertySets)
		Expect(incomplete).To(BeTrue())
		storageProfile = cdiv1.StorageProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Spec: cdiv1.StorageProfileSpec{
				ClaimPropertySets: []cdiv1.ClaimPropertySet{
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &BlockMode},
				},
			},
			Status: cdiv1.StorageProfileStatus{
				ClaimPropertySets: []cdiv1.ClaimPropertySet{
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &BlockMode},
				},
			},
		}
		incomplete = isIncomplete(storageProfile.Status.ClaimPropertySets)
		Expect(incomplete).To(BeFalse())
	})

	It("Should delete storage profile when corresponding storage class gets deleted", func() {
		storageClass := CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"})
		reconciler = createStorageProfileReconciler(storageClass)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		err = reconciler.client.Delete(context.TODO(), storageClass)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(BeEmpty())
	})

	It("Should create storage profile without claim property set for storage class not in capabilitiesByProvisionerKey map", func() {
		reconciler = createStorageProfileReconciler(CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).To(BeEmpty())
	})

	It("Should create storage profile with default claim property set for storage class", func() {
		scProvisioner := "rook-ceph.rbd.csi.ceph.com"
		reconciler = createStorageProfileReconciler(CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, scProvisioner))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))

		claimPropertySets := []cdiv1.ClaimPropertySet{}
		capabilities := storagecapabilities.CapabilitiesByProvisionerKey[scProvisioner]
		for i := range capabilities {
			claimPropertySet := cdiv1.ClaimPropertySet{
				AccessModes: []v1.PersistentVolumeAccessMode{capabilities[i].AccessMode},
				VolumeMode:  &capabilities[i].VolumeMode,
			}
			claimPropertySets = append(claimPropertySets, claimPropertySet)
		}
		Expect(sp.Status.ClaimPropertySets).To(Equal(claimPropertySets))
	})

	It("Should find storage capabilities for no-provisioner LSO storage class", func() {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, lsoLabels, "kubernetes.io/no-provisioner")
		pv := CreatePv("my-pv", storageClassName)

		reconciler = createStorageProfileReconciler(storageClass, pv)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).ToNot(BeEmpty())
	})

	It("Should not have storage capabilities for no-provisioner LSO storage class if there are no PVs for it", func() {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, lsoLabels, "kubernetes.io/no-provisioner")

		reconciler = createStorageProfileReconciler(storageClass)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).To(BeEmpty())
	})

	It("Should update storage profile with editted claim property sets", func() {
		reconciler = createStorageProfileReconciler(CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).To(BeEmpty())

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &BlockMode},
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
		}
		sp.Spec.ClaimPropertySets = claimPropertySets
		err = reconciler.client.Update(context.TODO(), sp.DeepCopy())
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList = &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		updatedSp := storageProfileList.Items[0]
		Expect(*updatedSp.Status.StorageClass).To(Equal(storageClassName))
		Expect(updatedSp.Spec.ClaimPropertySets).To(Equal(claimPropertySets))
		Expect(updatedSp.Status.ClaimPropertySets).To(Equal(claimPropertySets))
	})

	It("Should update storage profile with labels when the value changes", func() {
		reconciler = createStorageProfileReconciler(CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(sp.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &BlockMode},
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &FilesystemMode},
		}
		sp.Spec.ClaimPropertySets = claimPropertySets
		reconciler.installerLabels[common.AppKubernetesPartOfLabel] = "newtesting"
		err = reconciler.client.Update(context.TODO(), sp.DeepCopy())
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList = &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		updatedSp := storageProfileList.Items[0]
		Expect(updatedSp.Labels[common.AppKubernetesPartOfLabel]).To(Equal("newtesting"))
	})

	It("Should error when updating storage profile with missing access modes", func() {
		reconciler = createStorageProfileReconciler(CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).To(BeEmpty())

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &BlockMode},
			{VolumeMode: &FilesystemMode},
		}
		sp.Spec.ClaimPropertySets = claimPropertySets
		err = reconciler.client.Update(context.TODO(), sp.DeepCopy())
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("must provide access mode for volume mode: %s", FilesystemMode)))
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		updatedSp := storageProfileList.Items[0]
		Expect(*updatedSp.Status.StorageClass).To(Equal(storageClassName))
		Expect(updatedSp.Status.ClaimPropertySets).To(BeEmpty())
		Expect(updatedSp.Spec.ClaimPropertySets).To(Equal(claimPropertySets))
	})

	DescribeTable("should create clone strategy", func(cloneStrategy cdiv1.CDICloneStrategy) {
		storageClass := CreateStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"})

		storageClass.Annotations["cdi.kubevirt.io/clone-strategy"] = string(cloneStrategy)
		reconciler = createStorageProfileReconciler(storageClass)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))

		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(*sp.Status.CloneStrategy).To(Equal(cloneStrategy))
	},
		Entry("None", cdiv1.CloneStrategyHostAssisted),
		Entry("Snapshot", cdiv1.CloneStrategySnapshot),
		Entry("Clone", cdiv1.CloneStrategyCsiClone),
	)

	It("Should succeed when updating storage profile with specific SnapshotClass", func() {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, nil, nil, cephProvisioner)
		reconciler = createStorageProfileReconciler(storageClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())

		for i := 0; i < 3; i++ {
			snapClass := createSnapshotClass(fmt.Sprintf("snapclass-%d", i), nil, cephProvisioner)
			err := reconciler.client.Create(context.TODO(), snapClass)
			Expect(err).ToNot(HaveOccurred())
		}

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		sp := &cdiv1.StorageProfile{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, sp, &client.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		snapName := "snapclass-1"
		sp.Spec.SnapshotClass = &snapName
		err = reconciler.client.Update(context.TODO(), sp, &client.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, sp, &client.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(sp.Status.SnapshotClass).ToNot(BeNil())
		Expect(*sp.Status.SnapshotClass).To(Equal(snapName))
	})

	It("Should error when updating storage profile with non-existing SnapshotClass", func() {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, nil, nil, cephProvisioner)
		reconciler = createStorageProfileReconciler(storageClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())

		snapClass := createSnapshotClass(snapshotClassName, nil, cephProvisioner)
		err := reconciler.client.Create(context.TODO(), snapClass)
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		sp := &cdiv1.StorageProfile{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, sp, &client.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		noSuchSnapshotClassName := "no-such-snapshotclass"
		sp.Spec.SnapshotClass = &noSuchSnapshotClassName
		err = reconciler.client.Update(context.TODO(), sp, &client.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("\"%s\" not found", noSuchSnapshotClassName)))
	})

	It("Should error when updating storage profile with existing SnapshotClass with mismatching driver", func() {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, nil, nil, cephProvisioner)
		reconciler = createStorageProfileReconciler(storageClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())

		snapClass := createSnapshotClass(snapshotClassName, nil, "no-such-provisoner")
		err := reconciler.client.Create(context.TODO(), snapClass)
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		sp := &cdiv1.StorageProfile{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, sp, &client.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		sp.Spec.SnapshotClass = pointer.String(snapshotClassName)
		err = reconciler.client.Update(context.TODO(), sp, &client.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		Expect(sp.Status.SnapshotClass).To(BeNil())
	})

	DescribeTable("should set advised source format for dataimportcrons", func(provisioner string, expectedFormat cdiv1.DataImportCronSourceFormat, deploySnapClass bool) {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, provisioner)
		reconciler = createStorageProfileReconciler(storageClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		if deploySnapClass {
			snapClass := createSnapshotClass(snapshotClassName, nil, provisioner)
			err := reconciler.client.Create(context.TODO(), snapClass)
			Expect(err).ToNot(HaveOccurred())
		}
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))

		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(*sp.Status.DataImportCronSourceFormat).To(Equal(expectedFormat))
	},
		Entry("provisioners where snapshot source is more appropriate", "rook-ceph.rbd.csi.ceph.com", cdiv1.DataImportCronSourceFormatSnapshot, true),
		Entry("provisioners where snapshot source is more appropriate but lack volumesnapclass", "rook-ceph.rbd.csi.ceph.com", cdiv1.DataImportCronSourceFormatPvc, false),
		Entry("provisioners where there is no known preferred format", "format.unknown.provisioner.csi.com", cdiv1.DataImportCronSourceFormatPvc, false),
	)

	DescribeTable("should set cloneStrategy", func(provisioner string, expectedCloneStrategy cdiv1.CDICloneStrategy, deploySnapClass bool) {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, provisioner)
		reconciler = createStorageProfileReconciler(storageClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
		if deploySnapClass {
			snapClass := createSnapshotClass(snapshotClassName, nil, provisioner)
			err := reconciler.client.Create(context.TODO(), snapClass)
			Expect(err).ToNot(HaveOccurred())
		}
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())

		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})

		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))

		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(*sp.Status.CloneStrategy).To(Equal(expectedCloneStrategy))
	},
		Entry("provisioner with volumesnapshotclass and no known advised strategy", "strategy.unknown.provisioner.csi.com", cdiv1.CloneStrategySnapshot, true),
		Entry("provisioner without volumesnapshotclass and no known advised strategy", "strategy.unknown.provisioner.csi.com", cdiv1.CloneStrategyHostAssisted, false),
		Entry("provisioner that is known to prefer csi clone", "csi-powermax.dellemc.com", cdiv1.CloneStrategyCsiClone, false),
	)

	DescribeTable("Should set the StorageProfileStatus metric correctly", func(provisioner string, count int) {
		storageClass := CreateStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, provisioner)
		reconciler = createStorageProfileReconciler(storageClass)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(storageProfileList.Items).To(HaveLen(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(sp.Status.ClaimPropertySets).To(BeEmpty())

		labels := createLabels(storageClassName, provisioner, false, true, false, false, false)
		Expect(int(metrics.GetStorageProfileStatus(labels))).To(Equal(count))
	},
		Entry("Noobaa (not supported)", storagecapabilities.ProvisionerNoobaa, 0),
		Entry("Unknown provisioner", "unknown-provisioner", 1),
	)
})

func createStorageProfileReconciler(objects ...runtime.Object) *StorageProfileReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, MakeEmptyCDICR())
	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		// ScratchSpaceStorageClass: storageClassName,
	}

	objs = append(objs, cdiConfig)
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

	rec := record.NewFakeRecorder(10)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &StorageProfileReconciler{
		client:         cl,
		uncachedClient: cl,
		scheme:         s,
		log:            storageProfileLog,
		recorder:       rec,
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "testing",
			common.AppKubernetesVersionLabel: "v0.0.0-tests",
		},
	}
	return r
}

func CreatePv(name string, storageClassName string) *v1.PersistentVolume {
	volumeMode := v1.PersistentVolumeFilesystem
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(name),
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
			},
			StorageClassName: storageClassName,
			VolumeMode:       &volumeMode,
		},
	}
	return pv
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
