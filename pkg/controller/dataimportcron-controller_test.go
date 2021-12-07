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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"

	v1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	cronLog         = logf.Log.WithName("data-import-cron-controller-test")
	cronName        = "test-cron"
	imageStreamName = "test-imagestream"
	testRegistryURL = "docker://quay.io/kubevirt/junk"
)

const (
	testDigest    = "sha256:68b44fc891f3fae6703d4b74bcc9b5f24df8d23f12e642805d1420cbe7a4be70"
	testDockerRef = "quay.io/kubevirt/blabla@" + testDigest
)

var _ = Describe("All DataImportCron Tests", func() {
	var _ = Describe("DataImportCron controller reconcile loop", func() {
		var (
			reconciler *DataImportCronReconciler
			cronKey    = types.NamespacedName{Name: cronName, Namespace: metav1.NamespaceDefault}
			cronReq    = reconcile.Request{NamespacedName: cronKey}
			cronJobKey = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: GetCronJobName(cron), Namespace: reconciler.cdiNamespace}
			}
			dataSourceKey = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: cron.Spec.ManagedDataSource, Namespace: metav1.NamespaceDefault}
			}
			dvKey = func(dvName string) types.NamespacedName {
				return types.NamespacedName{Name: dvName, Namespace: metav1.NamespaceDefault}
			}
		)
		AfterEach(func() {
			if reconciler != nil {
				close(reconciler.recorder.(*record.FakeRecorder).Events)
				reconciler = nil
			}
		})

		It("Should do nothing and return nil when no DataImportCron exists", func() {
			reconciler = createDataImportCronReconciler()
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create and delete CronJob if DataImportCron is created and deleted", func() {
			cron := newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &v1beta1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Finalizers).ToNot(BeNil())
			Expect(cron.Finalizers[0]).To(Equal(dataImportCronFinalizer))

			now := metav1.Now()
			cron.DeletionTimestamp = &now
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).To(HaveOccurred())
		})

		It("Should create DataVolume on AnnSourceDesiredDigest annotation update, and update DataImportCron and DataSource on DataVolume Succeeded", func() {
			cron := newDataImportCron(cronName)
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			cron.Annotations[AnnSourceDesiredDigest] = testDigest
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeNil())
			Expect(len(imports)).ToNot(BeZero())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())
			digest := imports[0].Digest
			Expect(digest).To(Equal(testDigest))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Status.LastExecutionTimestamp).ToNot(BeNil())

			dv.Status.Phase = cdiv1.Succeeded
			dv.Status.Conditions = updateCondition(dv.Status.Conditions, cdiv1.DataVolumeReady, corev1.ConditionTrue, "", "")
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			dataSource := &cdiv1.DataSource{}
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).ToNot(HaveOccurred())

			sourcePVC := cdiv1.DataVolumeSourcePVC{
				Namespace: cron.Namespace,
				Name:      dvName,
			}
			Expect(dataSource.Spec.Source.PVC).ToNot(BeNil())
			Expect(*dataSource.Spec.Source.PVC).To(Equal(sourcePVC))

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
			Expect(*cron.Status.LastImportedPVC).To(Equal(sourcePVC))
			Expect(cron.Status.LastImportTimestamp).ToNot(BeNil())

			cronCond := FindDataImportCronConditionByType(cron, cdiv1.DataImportCronProgressing)
			Expect(cronCond).ToNot(BeNil())
			Expect(cronCond.Status).To(Equal(corev1.ConditionFalse))
			cronCond = FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
			Expect(cronCond).ToNot(BeNil())
			Expect(cronCond.Status).To(Equal(corev1.ConditionTrue))
			dsCond := FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
			Expect(dsCond).ToNot(BeNil())
			Expect(dsCond.Status).To(Equal(corev1.ConditionTrue))

			now := metav1.Now()
			cron.DeletionTimestamp = &now
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			// Should delete DataSource and DataVolume on DataImportCron deletion as RetentionPolicy is RetainNone
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).To(HaveOccurred())
			dvList := &cdiv1.DataVolumeList{}
			err = reconciler.client.List(context.TODO(), dvList, &client.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dvList.Items)).To(Equal(0))
		})

		It("Should update AnnNextCronTime annotation on a valid DataImportCron with ImageStream, and start an import and update DataImportCron when AnnNextCronTime annotation is updated to now", func() {
			cron := newDataImportCronWithImageStream(cronName)
			imageStream := newImageStream(imageStreamName)
			reconciler = createDataImportCronReconciler(cron, imageStream)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Annotations[AnnNextCronTime]).ToNot(BeEmpty())

			timestamp := time.Now().Format(time.RFC3339)
			cron.Annotations[AnnNextCronTime] = timestamp
			err = reconciler.client.Update(context.TODO(), cron)

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			Expect(cron.Annotations[AnnNextCronTime]).ToNot(Equal(timestamp))

			digest := cron.Annotations[AnnSourceDesiredDigest]
			Expect(digest).To(Equal(testDigest))
			dockerRef := cron.Annotations[AnnImageStreamDockerRef]
			Expect(dockerRef).To(Equal(testDockerRef))

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeNil())
			Expect(len(imports)).ToNot(BeZero())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())
			digest = imports[0].Digest
			Expect(digest).To(Equal(testDigest))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(*dv.Spec.Source.Registry.URL).To(Equal("docker://" + testDockerRef))

			now := metav1.Now()
			cron.DeletionTimestamp = &now
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			// Should retain DataSource and DataVolume on DataImportCron deletion as default RetentionPolicy is RetainAll
			dataSource := &cdiv1.DataSource{}
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).ToNot(HaveOccurred())
			dvList := &cdiv1.DataVolumeList{}
			err = reconciler.client.List(context.TODO(), dvList, &client.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dvList.Items)).ToNot(Equal(0))
		})
	})
})

func createDataImportCronReconciler(objects ...runtime.Object) *DataImportCronReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	objs := []runtime.Object{}
	objs = append(objs, cdiConfig)
	objs = append(objs, objects...)

	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	imagev1.AddToScheme(s)

	cl := fake.NewFakeClientWithScheme(s, objs...)
	rec := record.NewFakeRecorder(1)
	r := &DataImportCronReconciler{
		client:         cl,
		uncachedClient: cl,
		scheme:         s,
		log:            cronLog,
		recorder:       rec,
	}
	return r
}

func newDataImportCronWithImageStream(name string) *cdiv1.DataImportCron {
	cron := newDataImportCron(name)
	cron.Spec.Template.Spec.Source.Registry.ImageStream = &imageStreamName
	cron.Spec.Template.Spec.Source.Registry.URL = nil
	return cron
}

func newImageStream(name string) *imagev1.ImageStream {
	return &imagev1.ImageStream{
		TypeMeta: metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Items: []imagev1.TagEvent{
						{
							Image:                testDigest,
							DockerImageReference: testDockerRef,
						},
					},
				},
			},
		},
	}
}

func newDataImportCron(name string) *cdiv1.DataImportCron {
	garbageCollect := cdiv1.DataImportCronGarbageCollectOutdated
	registryPullNodesource := cdiv1.RegistryPullNode
	importsToKeep := int32(2)

	return &cdiv1.DataImportCron{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   metav1.NamespaceDefault,
			UID:         types.UID(metav1.NamespaceDefault + "-" + name),
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataImportCronSpec{
			Template: cdiv1.DataVolume{
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{
						Registry: &cdiv1.DataVolumeSourceRegistry{
							URL:        &testRegistryURL,
							PullMethod: &registryPullNodesource,
						},
					},
					PVC: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				},
			},
			Schedule:          "* * * * *",
			ManagedDataSource: "test-datasource",
			GarbageCollect:    &garbageCollect,
			ImportsToKeep:     &importsToKeep,
		},
	}
}
