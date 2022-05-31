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
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	cronLog  = logf.Log.WithName("data-import-cron-controller-test")
	cronName = "test-cron"
)

const (
	testRegistryURL = "docker://quay.io/kubevirt/junk"
	testTag         = ":12.34_56-7890"
	testDigest      = "sha256:68b44fc891f3fae6703d4b74bcc9b5f24df8d23f12e642805d1420cbe7a4be70"
	testDockerRef   = "quay.io/kubevirt/blabla@" + testDigest
	imageStreamName = "test-imagestream"
	imageStreamTag  = "test-imagestream-tag"
	tagWithNoItems  = "tag-with-no-items"
)

type possiblyErroringFakeCtrlRuntimeClient struct {
	client.Client
	shouldError bool
}

func (p *possiblyErroringFakeCtrlRuntimeClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	cron client.Object) error {
	if p.shouldError {
		return errors.New("Arbitrary unit test error that isn't NotFound")
	}
	return p.Client.Get(ctx, key, cron)
}

var _ = Describe("All DataImportCron Tests", func() {
	var _ = Describe("DataImportCron controller reconcile loop", func() {
		var (
			reconciler   *DataImportCronReconciler
			dsReconciler *DataSourceReconciler
			cron         *cdiv1.DataImportCron
			dataSource   *cdiv1.DataSource
			cronKey      = types.NamespacedName{Name: cronName, Namespace: metav1.NamespaceDefault}
			cronReq      = reconcile.Request{NamespacedName: cronKey}
			cronJobKey   = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: GetCronJobName(cron), Namespace: reconciler.cdiNamespace}
			}
			dataSourceKey = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: cron.Spec.ManagedDataSource, Namespace: metav1.NamespaceDefault}
			}
			dvKey = func(dvName string) types.NamespacedName {
				return types.NamespacedName{Name: dvName, Namespace: metav1.NamespaceDefault}
			}
		)

		// verifyConditions reconciles, gets DataImportCron and DataSource, and verifies their status conditions
		var verifyConditions = func(step string, isProgressing, isUpToDate, isReady bool, reasonProgressing, reasonUpToDate, reasonReady string) {
			By(step)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			cronCond := FindDataImportCronConditionByType(cron, cdiv1.DataImportCronProgressing)
			Expect(cronCond).ToNot(BeNil())
			verifyConditionState(string(cdiv1.DataImportCronProgressing), cronCond.ConditionState, isProgressing, reasonProgressing)
			cronCond = FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
			Expect(cronCond).ToNot(BeNil())
			verifyConditionState(string(cdiv1.DataImportCronUpToDate), cronCond.ConditionState, isUpToDate, reasonUpToDate)
			if dataSource != nil {
				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(len(imports)).ToNot(BeZero())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
				Expect(err).ToNot(HaveOccurred())
				dsReconciler = createDataSourceReconciler(dataSource, dv)
				dsReq := reconcile.Request{NamespacedName: dataSourceKey(cron)}
				_, err = dsReconciler.Reconcile(context.TODO(), dsReq)
				Expect(err).ToNot(HaveOccurred())

				err = dsReconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
				Expect(err).ToNot(HaveOccurred())
				dsCond := FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
				Expect(dsCond).ToNot(BeNil())
				verifyConditionState(string(cdiv1.DataSourceReady), dsCond.ConditionState, isReady, reasonReady)
			}
		}

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

		It("Should not cleanup if GET dataimportcron errors with something other than notfound", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)
			fakeErroringClient := &possiblyErroringFakeCtrlRuntimeClient{
				Client:      reconciler.client,
				shouldError: false,
			}
			reconciler.client = fakeErroringClient

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			// CronJob should not be cleaned up on arbitrary errors
			fakeErroringClient.shouldError = true
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Arbitrary unit test error"))

			// Stop artificial erroring so we could try grabbing the cronjob, which should still be there
			fakeErroringClient.shouldError = false
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create and delete CronJob if DataImportCron is created and deleted", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

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
			cron = newDataImportCron(cronName)
			dataSource = nil
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy
			reconciler = createDataImportCronReconciler(cron)
			verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "")

			if cron.Annotations == nil {
				cron.Annotations = make(map[string]string)
			}
			cron.Annotations[AnnSourceDesiredDigest] = testDigest
			err := reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			dataSource = &cdiv1.DataSource{}
			verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noPvc)

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

			dv.Status.Phase = cdiv1.ImportScheduled
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import scheduled", false, false, false, scheduled, inProgress, noPvc)

			dv.Status.Phase = cdiv1.ImportInProgress
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import in progress", true, false, false, inProgress, inProgress, noPvc)

			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready)

			sourcePVC := cdiv1.DataVolumeSourcePVC{
				Namespace: cron.Namespace,
				Name:      dvName,
			}
			Expect(dataSource.Spec.Source.PVC).ToNot(BeNil())
			Expect(*dataSource.Spec.Source.PVC).To(Equal(sourcePVC))
			Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
			Expect(*cron.Status.LastImportedPVC).To(Equal(sourcePVC))
			Expect(cron.Status.LastImportTimestamp).ToNot(BeNil())

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

		DescribeTable("Should fail when digest", func(digest, errorString string) {
			cron = newDataImportCron(cronName)
			cron.Annotations[AnnSourceDesiredDigest] = digest
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorString))
		},
			Entry("has no supported prefix", "sha234:012345678901", "Digest has no supported prefix"),
			Entry("is too short", "sha256:01234567890", "Digest is too short"),
		)

		DescribeTable("Should fail when ImageStream", func(taggedImageStreamName, errorString string) {
			cron = newDataImportCronWithImageStream(cronName, taggedImageStreamName)
			imageStream := newImageStream(imageStreamName)
			reconciler = createDataImportCronReconciler(cron, imageStream)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorString))
		},
			Entry("is not found", "no-such-image-stream", "not found"),
			Entry("tag is not found", imageStreamName+":"+"no-such-tag", "has no tag"),
			Entry("tag has no items", imageStreamName+":"+tagWithNoItems, "has no items"),
			Entry("has a colon but an empty tag", imageStreamName+":", "Illegal ImageStream name"),
			Entry("has an illegal format with two colons", imageStreamName+":"+imageStreamTag+":label", "Illegal ImageStream name"),
		)

		DescribeTable("Should start an import and update DataImportCron when ImageStream", func(taggedImageStreamName string, imageStreamTagsFromIndex int) {
			cron = newDataImportCronWithImageStream(cronName, taggedImageStreamName)
			imageStream := newImageStream(imageStreamName)
			imageStream.Status.Tags = imageStream.Status.Tags[imageStreamTagsFromIndex:]
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
			dv.Status.Phase = cdiv1.Succeeded
			dv.Status.Conditions = updateReadyCondition(dv.Status.Conditions, corev1.ConditionTrue, "", "")
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready)
			cond := findConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions)
			Expect(cond).ToNot(BeNil())
			condTime := cond.LastHeartbeatTime

			now := metav1.Now()
			cron.DeletionTimestamp = &now
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).To(HaveOccurred())

			// Should retain DataSource and DataVolume on DataImportCron deletion as default RetentionPolicy is RetainAll
			dataSource = &cdiv1.DataSource{}
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).ToNot(HaveOccurred())
			dvList := &cdiv1.DataVolumeList{}
			err = reconciler.client.List(context.TODO(), dvList, &client.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dvList.Items)).ToNot(Equal(0))

			// Test DV is reused
			time.Sleep(1 * time.Second)
			cron = newDataImportCronWithImageStream(cronName, taggedImageStreamName)
			reconciler = createDataImportCronReconciler(cron, imageStream, dv)
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			imports = cron.Status.CurrentImports
			Expect(len(imports)).ToNot(BeZero())
			dvName = imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())

			dv1 := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv1)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv1.Status.Phase).To(Equal(cdiv1.Succeeded))
			cond = findConditionByType(cdiv1.DataVolumeReady, dv1.Status.Conditions)
			Expect(cond).ToNot(BeNil())
			Expect(cond.LastHeartbeatTime.Time).To(BeTemporally(">", condTime.Time))
		},
			Entry("has tag", imageStreamName+":"+imageStreamTag, 0),
			Entry("has no tag", imageStreamName, 1),
		)
	})
})

var _ = Describe("untagURL", func() {
	It("should Remove tag from URL", func() {
		testDigestedURL := testRegistryURL + "@" + testDigest
		Expect(untagDigestedDockerURL(testRegistryURL + testTag + "@" + testDigest)).To(Equal(testDigestedURL))
		Expect(untagDigestedDockerURL(testDigestedURL)).To(Equal(testDigestedURL))
		Expect(untagDigestedDockerURL(testRegistryURL)).To(Equal(testRegistryURL))
	})
})

func createDataImportCronReconciler(objects ...runtime.Object) *DataImportCronReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	crd := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "dataimportcrons.cdi.kubevirt.io"}}
	objs := []runtime.Object{cdiConfig, crd}
	objs = append(objs, objects...)

	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	imagev1.AddToScheme(s)
	extv1.AddToScheme(s)

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

func newDataImportCronWithImageStream(dataImportCronName, taggedImageStreamName string) *cdiv1.DataImportCron {
	cron := newDataImportCron(dataImportCronName)
	cron.Spec.Template.Spec.Source.Registry.ImageStream = &taggedImageStreamName
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
					Tag: tagWithNoItems,
				},
				{
					Tag: imageStreamTag,
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
	url := testRegistryURL + testTag

	return &cdiv1.DataImportCron{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   metav1.NamespaceDefault,
			UID:         types.UID(uuid.NewString()),
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataImportCronSpec{
			Template: cdiv1.DataVolume{
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{
						Registry: &cdiv1.DataVolumeSourceRegistry{
							URL:        &url,
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

func verifyConditionState(condType string, condState cdiv1.ConditionState, desired bool, desiredReason string) {
	By(fmt.Sprintf("Verify condition %s is %v, %s", condType, desired, desiredReason))
	desiredStatus := corev1.ConditionFalse
	if desired {
		desiredStatus = corev1.ConditionTrue
	}
	Expect(condState.Status).To(Equal(desiredStatus))
	Expect(condState.Reason).To(Equal(desiredReason))
}
