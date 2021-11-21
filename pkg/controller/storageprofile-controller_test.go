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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
)

var (
	storageProfileLog = logf.Log.WithName("storageprofile-controller-test")
	storageClassName  = "testSC"
)

var _ = Describe("Storage profile controller reconcile loop", func() {

	It("Should return error if storage profile can not be found", func() {
		reconciler := createStorageProfileReconciler()
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("storageclasses.storage.k8s.io \"%s\" not found", storageClassName)))
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(0))
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
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &blockMode},
				},
			},
			Status: cdiv1.StorageProfileStatus{
				ClaimPropertySets: []cdiv1.ClaimPropertySet{
					{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &blockMode},
				},
			},
		}
		incomplete = isIncomplete(storageProfile.Status.ClaimPropertySets)
		Expect(incomplete).To(BeFalse())
	})

	It("Should create storage profile without claim property set for storage class not in capabilitiesByProvisionerKey map", func() {
		reconciler := createStorageProfileReconciler(createStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(len(sp.Status.ClaimPropertySets)).To(Equal(0))
	})

	It("Should create storage profile with default claim property set for storage class", func() {
		scProvisioner := "rook-ceph.rbd.csi.ceph.com"
		reconciler := createStorageProfileReconciler(createStorageClassWithProvisioner(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}, scProvisioner))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
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

	It("Should update storage profile with editted claim property sets", func() {
		reconciler := createStorageProfileReconciler(createStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(len(sp.Status.ClaimPropertySets)).To(Equal(0))

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &blockMode},
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &filesystemMode},
		}
		sp.Spec.ClaimPropertySets = claimPropertySets
		err = reconciler.client.Update(context.TODO(), sp.DeepCopy())
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList = &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		updatedSp := storageProfileList.Items[0]
		Expect(*updatedSp.Status.StorageClass).To(Equal(storageClassName))
		Expect(updatedSp.Spec.ClaimPropertySets).To(Equal(claimPropertySets))
		Expect(updatedSp.Status.ClaimPropertySets).To(Equal(claimPropertySets))
	})

	It("Should update storage profile with labels when the value changes", func() {
		reconciler := createStorageProfileReconciler(createStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		sp := storageProfileList.Items[0]
		Expect(sp.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &blockMode},
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, VolumeMode: &filesystemMode},
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
		Expect(len(storageProfileList.Items)).To(Equal(1))
		updatedSp := storageProfileList.Items[0]
		Expect(updatedSp.Labels[common.AppKubernetesPartOfLabel]).To(Equal("newtesting"))
	})

	It("Should error when updating storage profile with missing access modes", func() {
		reconciler := createStorageProfileReconciler(createStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"}))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		sp := storageProfileList.Items[0]
		Expect(*sp.Status.StorageClass).To(Equal(storageClassName))
		Expect(len(sp.Status.ClaimPropertySets)).To(Equal(0))

		claimPropertySets := []cdiv1.ClaimPropertySet{
			{AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany}, VolumeMode: &blockMode},
			{VolumeMode: &filesystemMode},
		}
		sp.Spec.ClaimPropertySets = claimPropertySets
		err = reconciler.client.Update(context.TODO(), sp.DeepCopy())
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("must provide access mode for volume mode: %s", filesystemMode)))
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		updatedSp := storageProfileList.Items[0]
		Expect(*updatedSp.Status.StorageClass).To(Equal(storageClassName))
		Expect(len(updatedSp.Status.ClaimPropertySets)).To(Equal(0))
		Expect(updatedSp.Spec.ClaimPropertySets).To(Equal(claimPropertySets))
	})
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
	cdiv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &StorageProfileReconciler{
		client:         cl,
		uncachedClient: cl,
		scheme:         s,
		log:            storageProfileLog,
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "testing",
			common.AppKubernetesVersionLabel: "v0.0.0-tests",
		},
	}
	return r
}
