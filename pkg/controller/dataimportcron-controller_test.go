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
)

var (
	cronLog = logf.Log.WithName("data-import-cron-controller-test")
)

var _ = Describe("All DataImportCron Tests", func() {
	var _ = Describe("DataImportCron controller reconcile loop", func() {
		var (
			reconciler *DataImportCronReconciler
		)
		AfterEach(func() {
			if reconciler != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
				reconciler = nil
			}
		})

		It("Should do nothing and return nil when no DataImportCron exists", func() {
			reconciler = createDataImportCronReconciler()
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-cron", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			cron := &cdiv1.DataImportCron{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-cron", Namespace: metav1.NamespaceDefault}, cron)
			Expect(err).To(HaveOccurred())
			if !k8serrors.IsNotFound(err) {
				Fail("Error getting DataImportCron")
			}
		})
	})
})

func createDataImportCronReconciler(objects ...runtime.Object) *DataImportCronReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	cl := fake.NewFakeClientWithScheme(s, objs...)
	rec := record.NewFakeRecorder(1)
	r := &DataImportCronReconciler{
		client:   cl,
		scheme:   s,
		log:      cronLog,
		recorder: rec,
	}
	return r
}
