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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	storageProfileLog = logf.Log.WithName("storageprofile-controller-test")
	storageClassName  = "testSC"
)

var _ = Describe("Storage profile controller reconcile loop", func() {

	It("Should not requeue if storage class can not be found", func() {
		reconciler := createStorageProfileReconciler()
		res, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(res.Requeue).ToNot(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(0))
	})

	It("Should delete storage profile when corresponding storage class gets deleted", func() {
		storageClass := createStorageClass(storageClassName, map[string]string{AnnDefaultStorageClass: "true"})
		reconciler := createStorageProfileReconciler(storageClass)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		storageProfileList := &cdiv1.StorageProfileList{}
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(1))
		err = reconciler.client.Delete(context.TODO(), storageClass)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: storageClassName}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.List(context.TODO(), storageProfileList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(storageProfileList.Items)).To(Equal(0))
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
