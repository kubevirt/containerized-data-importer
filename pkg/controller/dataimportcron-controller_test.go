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
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/google/uuid"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	imagev1 "github.com/openshift/api/image/v1"
	secv1 "github.com/openshift/api/security/v1"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
)

var (
	cronLog        = logf.Log.WithName("data-import-cron-controller-test")
	cronName       = "test-cron"
	httpProxy      = "test-http-proxy"
	httpsProxy     = "test-https-proxy"
	noProxy        = "test-no-proxy"
	trustedCAProxy = "test-trusted-ca-proxy"
)

const (
	testRegistryURL = "docker://quay.io/kubevirt/junk"
	testTag         = ":12.34_56-7890"
	testDigest      = "sha256:68b44fc891f3fae6703d4b74bcc9b5f24df8d23f12e642805d1420cbe7a4be70"
	testDockerRef   = "quay.io/kubevirt/blabla@" + testDigest
	dataSourceName  = "test-datasource"
	imageStreamName = "test-imagestream"
	imageStreamTag  = "test-imagestream-tag"
	tagWithNoItems  = "tag-with-no-items"
	defaultSchedule = "* * * * *"
	emptySchedule   = ""
)

type possiblyErroringFakeCtrlRuntimeClient struct {
	client.Client
	shouldError bool
}

func (p *possiblyErroringFakeCtrlRuntimeClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	cron client.Object,
	opts ...client.GetOption) error {
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
			pollerPodKey = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: getPollerPodName(cron), Namespace: metav1.NamespaceDefault}
			}
			dataSourceKey = func(cron *cdiv1.DataImportCron) types.NamespacedName {
				return types.NamespacedName{Name: cron.Spec.ManagedDataSource, Namespace: metav1.NamespaceDefault}
			}
			dvKey = func(dvName string) types.NamespacedName {
				return types.NamespacedName{Name: dvName, Namespace: metav1.NamespaceDefault}
			}
		)

		// verifyConditions reconciles, gets DataImportCron and DataSource, and verifies their status conditions
		var verifyConditions = func(step string, isProgressing, isUpToDate, isReady bool, reasonProgressing, reasonUpToDate, reasonReady string, sourceObj client.Object) {
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
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				if err == nil {
					err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
					Expect(err).ToNot(HaveOccurred())
					dsReconciler = createDataSourceReconciler(dataSource, dv)
					dsReq := reconcile.Request{NamespacedName: dataSourceKey(cron)}
					_, err = dsReconciler.Reconcile(context.TODO(), dsReq)
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())
					err = reconciler.client.Get(context.TODO(), dvKey(dvName), sourceObj)
					Expect(err).ToNot(HaveOccurred())
					err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
					Expect(err).ToNot(HaveOccurred())
					dsReconciler = createDataSourceReconciler(dataSource, sourceObj)
					dsReq := reconcile.Request{NamespacedName: dataSourceKey(cron)}
					_, err = dsReconciler.Reconcile(context.TODO(), dsReq)
					Expect(err).ToNot(HaveOccurred())
				}

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
			cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
			cdiConfig.Status.ImportProxy = &cdiv1.ImportProxy{
				HTTPProxy:      &httpProxy,
				HTTPSProxy:     &httpsProxy,
				NoProxy:        &noProxy,
				TrustedCAProxy: &trustedCAProxy,
			}

			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconcilerWithoutConfig(cron, cdiConfig)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())
			containers := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(cronjob.Spec.JobTemplate.Spec.Template.GetAnnotations()[secv1.RequiredSCCAnnotation]).To(Equal(common.RestrictedSCCName))

			env := containers[0].Env
			Expect(getEnvVar(env, common.ImportProxyHTTP)).To(Equal(httpProxy))
			Expect(getEnvVar(env, common.ImportProxyHTTPS)).To(Equal(httpsProxy))
			Expect(getEnvVar(env, common.ImportProxyNoProxy)).To(Equal(noProxy))

			volMounts := containers[0].VolumeMounts
			Expect(volMounts).To(HaveLen(1))
			Expect(volMounts[0]).To(Equal(corev1.VolumeMount{
				Name:      ProxyCertVolName,
				MountPath: common.ImporterProxyCertDir,
			}))

			volumes := cronjob.Spec.JobTemplate.Spec.Template.Spec.Volumes
			Expect(volumes).To(HaveLen(1))
			Expect(volumes[0]).To(Equal(corev1.Volume{
				Name: ProxyCertVolName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: trustedCAProxy,
						},
					},
				},
			}))

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Delete(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).To(HaveOccurred())
		})

		It("Should delete DataImportCron-orphan CronJob", func() {
			reconciler = createDataImportCronReconciler()

			cron = newDataImportCron(cronName)
			err := reconciler.client.Create(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Delete(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Should update CronJob DataImportCron labels", func() {
			reconciler = createDataImportCronReconciler()

			cron = newDataImportCron(cronName)
			err := reconciler.client.Create(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			cronjob.Labels = nil
			err = reconciler.client.Update(context.TODO(), cronjob)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			cronNsLabel, ok := cronjob.Labels[common.DataImportCronNsLabel]
			Expect(ok).To(BeTrue())
			Expect(cronNsLabel).To(Equal(cron.Namespace))

			cronLabel, ok := cronjob.Labels[common.DataImportCronLabel]
			Expect(ok).To(BeTrue())
			Expect(cronLabel).To(Equal(cron.Name))
		})

		It("Should verify CronJob container env variables are empty and no extra volume is set when proxy is not configured", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			Expect(cronjob.Spec.SuccessfulJobsHistoryLimit).To(Equal(ptr.To[int32](1)))
			Expect(cronjob.Spec.FailedJobsHistoryLimit).To(Equal(ptr.To[int32](1)))

			jobTemplateSpec := cronjob.Spec.JobTemplate.Spec
			Expect(jobTemplateSpec.TTLSecondsAfterFinished).To(Equal(ptr.To[int32](10)))

			jobPodTemplateSpec := jobTemplateSpec.Template.Spec
			containers := jobPodTemplateSpec.Containers
			Expect(containers).To(HaveLen(1))

			env := containers[0].Env
			Expect(getEnvVar(env, common.ImportProxyHTTP)).To(BeEmpty())
			Expect(getEnvVar(env, common.ImportProxyHTTPS)).To(BeEmpty())
			Expect(getEnvVar(env, common.ImportProxyNoProxy)).To(BeEmpty())

			Expect(containers[0].VolumeMounts).To(BeEmpty())
			Expect(jobPodTemplateSpec.Volumes).To(BeEmpty())
		})

		It("Should verify CronJob container terminationMessagePolicy is correctly set", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())

			containers := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].TerminationMessagePolicy).To(Equal(corev1.TerminationMessageFallbackToLogsOnError))
		})

		It("Should update CronJob on reconcile", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)

			verifyCronJobContainerImage := func(image string) {
				reconciler.image = image
				_, err := reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())

				cronjob := &batchv1.CronJob{}
				err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
				Expect(err).ToNot(HaveOccurred())

				containers := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
				Expect(containers).ToNot(BeEmpty())
				Expect(containers[0].Image).To(Equal(image))
			}

			verifyCronJobContainerImage("old-image")
			verifyCronJobContainerImage("new-image")
		})

		It("Should update CronJob Pod workload NodePlacement on reconcile", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())
			spec := &cronjob.Spec.JobTemplate.Spec.Template.Spec
			spec.NodeSelector = map[string]string{"some": "thing"}
			spec.Tolerations = []corev1.Toleration{{Key: "another", Value: "value"}}
			err = reconciler.client.Update(context.TODO(), cronjob)
			Expect(err).ToNot(HaveOccurred())

			workloads := updateCdiWithTestNodePlacement(reconciler.client)

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(err).ToNot(HaveOccurred())
			podSpec := cronjob.Spec.JobTemplate.Spec.Template.Spec
			Expect(podSpec.Affinity).To(Equal(workloads.Affinity))
			Expect(podSpec.NodeSelector).To(Equal(workloads.NodeSelector))
			Expect(podSpec.Tolerations).To(Equal(workloads.Tolerations))
		})

		It("Should set initial Job Pod workload NodePlacement on reconcile", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)

			workloads := updateCdiWithTestNodePlacement(reconciler.client)

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			job := &batchv1.Job{}
			jobKey := types.NamespacedName{Name: GetInitialJobName(cron), Namespace: reconciler.cdiNamespace}
			err = reconciler.client.Get(context.TODO(), jobKey, job)
			Expect(err).ToNot(HaveOccurred())
			podSpec := job.Spec.Template.Spec
			Expect(podSpec.Affinity).To(Equal(workloads.Affinity))
			Expect(podSpec.NodeSelector).To(Equal(workloads.NodeSelector))
			Expect(podSpec.Tolerations).To(Equal(workloads.Tolerations))
			Expect(job.Spec.Template.GetAnnotations()[secv1.RequiredSCCAnnotation]).To(Equal(common.RestrictedSCCName))
		})

		It("Should not modify new CronJob on initCronJob", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			cronJob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronJob)
			Expect(err).ToNot(HaveOccurred())

			cronJobCopy := cronJob.DeepCopy()
			err = reconciler.initCronJob(cron, cronJobCopy)
			Expect(err).ToNot(HaveOccurred())

			Expect(cronJob).To(Equal(cronJobCopy))
		})

		DescribeTable("Should create DataVolume on AnnSourceDesiredDigest annotation update, and update DataImportCron and DataSource on DataVolume Succeeded", func(schedule, errorString string) {
			cron = newDataImportCron(cronName)
			cron.Spec.Schedule = schedule
			dataSource = nil
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy
			reconciler = createDataImportCronReconciler(cron)
			verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &corev1.PersistentVolumeClaim{})

			cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
			err := reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			dataSource = &cdiv1.DataSource{}
			verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &corev1.PersistentVolumeClaim{})

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeNil())
			Expect(imports).ToNot(BeEmpty())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())
			digest := imports[0].Digest
			Expect(digest).To(Equal(testDigest))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
			Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))

			if cron.Spec.Schedule == emptySchedule {
				cronjob := &batchv1.CronJob{}
				err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			}

			dv.Status.Phase = cdiv1.ImportScheduled
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import scheduled", false, false, false, scheduled, inProgress, noSource, &corev1.PersistentVolumeClaim{})
			// Verify DIC outdated metric is 1 only with pending=true
			verifyDataImportCronOutdatedMetric(cron, true, 1)
			verifyDataImportCronOutdatedMetric(cron, false, 0)

			dv.Status.Phase = cdiv1.ImportInProgress
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import in progress", true, false, false, inProgress, inProgress, noSource, &corev1.PersistentVolumeClaim{})

			dv.Status.Phase = cdiv1.Succeeded
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := cc.CreatePvc(dv.Name, dv.Namespace, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &corev1.PersistentVolumeClaim{})
			// Verify DIC outdated metric is 0 with either pending=true/false
			verifyDataImportCronOutdatedMetric(cron, true, 0)
			verifyDataImportCronOutdatedMetric(cron, false, 0)

			sourcePVC := cdiv1.DataVolumeSourcePVC{
				Namespace: cron.Namespace,
				Name:      dvName,
			}
			Expect(dataSource.Spec.Source.PVC).ToNot(BeNil())
			Expect(*dataSource.Spec.Source.PVC).To(Equal(sourcePVC))
			Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
			Expect(*cron.Status.LastImportedPVC).To(Equal(sourcePVC))
			Expect(cron.Status.LastImportTimestamp).ToNot(BeNil())

			err = reconciler.client.Delete(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			// Should delete DataSource and DataVolume on DataImportCron deletion as RetentionPolicy is RetainNone
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).To(HaveOccurred())
			dvList := &cdiv1.DataVolumeList{}
			err = reconciler.client.List(context.TODO(), dvList, &client.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(dvList.Items).To(BeEmpty())
		},
			Entry("default schedule", defaultSchedule, "should succeed with a default schedule"),
			Entry("empty schedule", emptySchedule, "should succeed with an empty schedule"),
		)

		It("Should create a poller Pod, and upon its termination update the DataImportCron DesiredDigest according to the container status ImageID", func() {
			cron = newDataImportCron(cronName)
			cron.Spec.Template.Spec.Source.Registry.PullMethod = ptr.To(cdiv1.RegistryPullNode)
			reconciler = createDataImportCronReconciler(cron)

			res, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{RequeueAfter: 3 * time.Second}))

			cronjob := &batchv1.CronJob{}
			err = reconciler.client.Get(context.TODO(), cronJobKey(cron), cronjob)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			pollerPod := &corev1.Pod{}
			err = reconciler.client.Get(context.TODO(), pollerPodKey(cron), pollerPod)
			Expect(err).ToNot(HaveOccurred())
			Expect(pollerPod.Spec.Containers).To(HaveLen(1))
			Expect(pollerPod.Status.ContainerStatuses).To(BeEmpty())

			pollerPod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
					ImageID: testDockerRef,
				},
			}
			err = reconciler.client.Status().Update(context.TODO(), pollerPod)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), pollerPodKey(cron), pollerPod)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Annotations[AnnSourceDesiredDigest]).To(Equal(testDigest))
		})

		It("Should recreate DataVolume if the last import was deleted", func() {
			cron = newDataImportCron(cronName)
			cron.Annotations[AnnSourceDesiredDigest] = testDigest
			reconciler = createDataImportCronReconciler(cron)

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeEmpty())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Delete(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("Should set the DataImportCronOutdated metric correctly", func(isPending bool) {
			cron = newDataImportCron(cronName)
			cron.Annotations[AnnSourceDesiredDigest] = testDigest

			reconciler = createDataImportCronReconciler(cron)

			if !isPending {
				sc := cc.CreateStorageClass(storageClassName, map[string]string{cc.AnnDefaultStorageClass: "true"})
				err := reconciler.client.Create(context.TODO(), sc)
				Expect(err).ToNot(HaveOccurred())

				sp := &cdiv1.StorageProfile{}
				sp.Name = storageClassName
				err = reconciler.client.Create(context.TODO(), sp)
				Expect(err).ToNot(HaveOccurred())
			}

			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeEmpty())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			// Verify DIC outdated metric is 1 only with correct pending label
			verifyDataImportCronOutdatedMetric(cron, isPending, 1)
			verifyDataImportCronOutdatedMetric(cron, !isPending, 0)
		},
			Entry("with DataVolume pending for default storage class", true),
			Entry("with default storage class", false),
		)

		It("Should not create DV if PVC exists on DesiredDigest update; Should update DIC and DAS, and GC LRU PVCs", func() {
			const nPVCs = 3
			var (
				digests [nPVCs]string
				pvcs    [nPVCs]*corev1.PersistentVolumeClaim
			)

			cron = newDataImportCron(cronName)
			dataSource = nil
			reconciler = createDataImportCronReconciler(cron)
			verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &corev1.PersistentVolumeClaim{})

			for i := 0; i < nPVCs; i++ {
				digest := strings.Repeat(strconv.Itoa(i), 12)
				digests[i] = "sha256:" + digest
				pvcs[i] = cc.CreatePvc(dataSourceName+"-"+digest, cron.Namespace, nil, nil)
				err := reconciler.client.Create(context.TODO(), pvcs[i])
				Expect(err).ToNot(HaveOccurred())
			}

			pvc := &corev1.PersistentVolumeClaim{}
			lastTs := ""
			verifyDigestUpdate := func(idx int) {
				cc.AddAnnotation(cron, AnnSourceDesiredDigest, digests[idx])
				err := reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}

				_, err = reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &corev1.PersistentVolumeClaim{})

				imports := cron.Status.CurrentImports
				Expect(imports).To(HaveLen(1))
				Expect(imports[0].Digest).To(Equal(digests[idx]))
				dvName := imports[0].DataVolumeName
				Expect(dvName).To(Equal(pvcs[idx].Name))

				sourcePVC := cdiv1.DataVolumeSourcePVC{
					Namespace: cron.Namespace,
					Name:      dvName,
				}
				Expect(dataSource.Spec.Source.PVC).ToNot(BeNil())
				Expect(*dataSource.Spec.Source.PVC).To(Equal(sourcePVC))
				Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
				Expect(*cron.Status.LastImportedPVC).To(Equal(sourcePVC))
				Expect(cron.Status.LastImportTimestamp).ToNot(BeNil())

				By("Verifying current pvc LastUseTime is later than the previous one")
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), pvc)
				Expect(err).ToNot(HaveOccurred())
				ts := pvc.Annotations[AnnLastUseTime]
				Expect(ts).ToNot(BeEmpty())
				Expect(strings.Compare(ts, lastTs)).Should(Equal(1))
				lastTs = ts
			}

			verifyDigestUpdate(0)
			verifyDigestUpdate(1)
			verifyDigestUpdate(0)
			verifyDigestUpdate(2)

			By("Verifying pvc1 was garbage collected")
			err := reconciler.client.Get(context.TODO(), dvKey(pvcs[1].Name), pvc)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			verifyDigestUpdate(0)

			By("Re-create pvc1")
			pvcs[1].ResourceVersion = ""
			err = reconciler.client.Create(context.TODO(), pvcs[1])
			Expect(err).ToNot(HaveOccurred())
			verifyDigestUpdate(1)

			By("Verifying pvc2 was garbage collected")
			err = reconciler.client.Get(context.TODO(), dvKey(pvcs[2].Name), pvc)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Should reconcile only if DataSource is not labeled by another existing DIC", func() {
			cron = newDataImportCron(cronName)
			reconciler = createDataImportCronReconciler(cron)

			shouldReconcile, err := reconciler.shouldReconcileCron(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReconcile).To(BeTrue())

			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			dataSource = &cdiv1.DataSource{}
			err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
			Expect(err).ToNot(HaveOccurred())
			Expect(dataSource.Labels[common.DataImportCronLabel]).To(Equal(cron.Name))

			cron1 := newDataImportCron(cronName + "1")
			shouldReconcile, err = reconciler.shouldReconcileCron(context.TODO(), cron1)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReconcile).To(BeFalse())
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring(fmt.Sprintf(MessageDataSourceAlreadyManaged, dataSource.Name, cron.Name)))

			dataSource.Labels[common.DataImportCronLabel] = "nosuchdic"
			err = reconciler.client.Update(context.TODO(), dataSource)
			Expect(err).ToNot(HaveOccurred())
			shouldReconcile, err = reconciler.shouldReconcileCron(context.TODO(), cron1)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReconcile).To(BeTrue())

			dataSource.Labels[common.DataImportCronLabel] = ""
			err = reconciler.client.Update(context.TODO(), dataSource)
			Expect(err).ToNot(HaveOccurred())
			shouldReconcile, err = reconciler.shouldReconcileCron(context.TODO(), cron1)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldReconcile).To(BeTrue())
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
			res, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.RequeueAfter.Seconds()).To(And(BeNumerically(">", 0), BeNumerically("<=", 60)))

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())
			Expect(cron.Annotations[AnnNextCronTime]).ToNot(BeEmpty())

			timestamp := time.Now().Format(time.RFC3339)
			cron.Annotations[AnnNextCronTime] = timestamp
			err = reconciler.client.Update(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

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
			Expect(imports).ToNot(BeEmpty())
			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())
			digest = imports[0].Digest
			Expect(digest).To(Equal(testDigest))

			dv := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(*dv.Spec.Source.Registry.URL).To(Equal("docker://" + testDockerRef))
			Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))
			dv.Status.Phase = cdiv1.Succeeded
			dv.Status.Conditions = dvc.UpdateReadyCondition(dv.Status.Conditions, corev1.ConditionTrue, "", "")
			err = reconciler.client.Update(context.TODO(), dv)
			Expect(err).ToNot(HaveOccurred())

			pvc := cc.CreatePvc(dv.Name, dv.Namespace, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &corev1.PersistentVolumeClaim{})

			err = reconciler.client.Delete(context.TODO(), cron)
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
			Expect(dvList.Items).ToNot(BeEmpty())

			// Test DV is reused
			time.Sleep(1 * time.Second)
			cron = newDataImportCronWithImageStream(cronName, taggedImageStreamName)
			reconciler = createDataImportCronReconciler(cron, imageStream, dv)
			_, err = reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.client.Get(context.TODO(), cronKey, cron)
			Expect(err).ToNot(HaveOccurred())

			imports = cron.Status.CurrentImports
			Expect(imports).ToNot(BeEmpty())
			dvName = imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())

			dv1 := &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv1)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv1.Status.Phase).To(Equal(cdiv1.Succeeded))
		},
			Entry("has tag", imageStreamName+":"+imageStreamTag, 0),
			Entry("has no tag", imageStreamName, 1),
		)

		It("Should fail with non-existing source PVC", func() {
			cron := newDataImportCron(cronName)
			cron.Spec.Template.Spec.Source = &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name: "no-such-pvc",
				},
			}
			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("Should succeed with existing source PVC", func() {
			pvc := newPVC("test-pvc")
			cron := newDataImportCron(cronName)
			cron.Spec.Template.Spec.Source = &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      pvc.Name,
					Namespace: pvc.Namespace,
				},
			}
			reconciler = createDataImportCronReconciler(cron, pvc)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should succeed garbage collecting old version DVs", func() {
			cron = newDataImportCron(cronName)
			importsToKeep := int32(1)
			cron.Spec.ImportsToKeep = &importsToKeep
			reconciler = createDataImportCronReconciler(cron)

			// Labeled DV and unlabeled PVC
			dv1 := cc.NewImportDataVolume("test-dv1")
			dv1.Labels = map[string]string{common.DataImportCronLabel: cronName}
			err := reconciler.client.Create(context.TODO(), dv1)
			Expect(err).ToNot(HaveOccurred())

			pvc1 := cc.CreatePvc(dv1.Name, dv1.Namespace, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc1)
			Expect(err).ToNot(HaveOccurred())

			// Labeled DV and PVC
			dv2 := cc.NewImportDataVolume("test-dv2")
			dv2.Labels = map[string]string{common.DataImportCronLabel: cronName}
			err = reconciler.client.Create(context.TODO(), dv2)
			Expect(err).ToNot(HaveOccurred())

			pvc2 := cc.CreatePvc(dv2.Name, dv2.Namespace, nil, nil)
			pvc2.Labels = map[string]string{common.DataImportCronLabel: cronName}
			err = reconciler.client.Create(context.TODO(), pvc2)
			Expect(err).ToNot(HaveOccurred())

			// Unlabeled DV and PVC
			dv3 := cc.NewImportDataVolume("test-dv3")
			err = reconciler.client.Create(context.TODO(), dv3)
			Expect(err).ToNot(HaveOccurred())

			pvc3 := cc.CreatePvc(dv3.Name, dv3.Namespace, nil, nil)
			err = reconciler.client.Create(context.TODO(), pvc3)
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.garbageCollectOldImports(context.TODO(), cron)
			Expect(err).ToNot(HaveOccurred())

			// Ensure the old version DV is deleted (labeled DV and unlabeled PVC).
			// The labeled PVC will not be deleted here, as there is no relevant controller.
			err = reconciler.client.Get(context.TODO(), dvKey(dv1.Name), dv1)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())

			// Ensure the new version DV is not deleted (labeled DV and labeled PVC).
			err = reconciler.client.Get(context.TODO(), dvKey(dv2.Name), dv2)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), dvKey(pvc2.Name), pvc2)
			Expect(err).ToNot(HaveOccurred())

			// Ensure unrelated DVs and PVCs are not deleted (unlabeled DV and PVC)
			err = reconciler.client.Get(context.TODO(), dvKey(dv3.Name), dv3)
			Expect(err).ToNot(HaveOccurred())
			err = reconciler.client.Get(context.TODO(), dvKey(pvc3.Name), pvc3)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should pass through metadata to DataVolume", func() {
			cron = newDataImportCron(cronName)
			cron.Annotations[AnnSourceDesiredDigest] = testDigest

			cron.Labels = map[string]string{}
			for _, defaultInstanceTypeLabel := range cc.DefaultInstanceTypeLabels {
				cron.Labels[defaultInstanceTypeLabel] = defaultInstanceTypeLabel
			}
			cron.Labels[cc.LabelDynamicCredentialSupport] = "true"

			reconciler = createDataImportCronReconciler(cron)
			_, err := reconciler.Reconcile(context.TODO(), cronReq)
			Expect(err).ToNot(HaveOccurred())

			Expect(reconciler.client.Get(context.TODO(), cronKey, cron)).ToNot(HaveOccurred())

			imports := cron.Status.CurrentImports
			Expect(imports).ToNot(BeNil())
			Expect(imports).ToNot(BeEmpty())

			dvName := imports[0].DataVolumeName
			Expect(dvName).ToNot(BeEmpty())

			dv := &cdiv1.DataVolume{}
			Expect(reconciler.client.Get(context.TODO(), dvKey(dvName), dv)).To(Succeed())

			for _, defaultInstanceTypeLabel := range cc.DefaultInstanceTypeLabels {
				Expect(dv.Labels).To(HaveKeyWithValue(defaultInstanceTypeLabel, defaultInstanceTypeLabel))
			}
			Expect(dv.Labels).To(HaveKeyWithValue(cc.LabelDynamicCredentialSupport, "true"))
		})

		var (
			dicLabels = map[string]string{common.DataImportCronLabel: cronName}
			oldScName = "old-sc"
			newScName = "new-sc"
		)

		DescribeTable("should delete DIC-created PVCs Pending for old default/virt default storage class",
			func(dvScName *string, pvcScName string, labels map[string]string, phase corev1.PersistentVolumeClaimPhase, shouldDelete bool) {
				cron := newDataImportCron(cronName)
				cron.Annotations[AnnSourceDesiredDigest] = testDigest
				cron.Spec.Template.Spec.Storage.StorageClassName = dvScName

				sc := cc.CreateStorageClass(storageClassName, map[string]string{cc.AnnDefaultVirtStorageClass: "true"})
				sp := &cdiv1.StorageProfile{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}}

				reconciler := createDataImportCronReconciler(cron, sc, sp)

				_, err := reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Get(context.TODO(), cronKey, cron)
				Expect(err).ToNot(HaveOccurred())

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())

				pvc := cc.CreatePvcInStorageClass(dv.Name, dv.Namespace, &pvcScName, nil, labels, phase)
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())

				_, err = reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())

				err = reconciler.client.Get(context.TODO(), client.ObjectKeyFromObject(pvc), pvc)
				if shouldDelete {
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry("DV with default SC, Pending PVC with DIC-label and outdated SC - is deleted", nil, oldScName, dicLabels, corev1.ClaimPending, true),
			Entry("DV with default SC, Pending PVC without DIC-label and outdated SC - is not deleted", nil, oldScName, nil, corev1.ClaimPending, false),
			Entry("DV with default SC, Pending PVC with DIC-label and default SC - is not deleted", nil, storageClassName, dicLabels, corev1.ClaimPending, false),
			Entry("DV with default SC, Bound PVC with DIC-label and outdated SC - is not deleted", nil, oldScName, dicLabels, corev1.ClaimBound, false),
			Entry("DV with default SC, Bound PVC without DIC-label and outdated SC - is not deleted", nil, oldScName, nil, corev1.ClaimBound, false),
			Entry("DV with specific SC, Pending PVC with DIC-label and outdated SC - is not deleted", &newScName, oldScName, nil, corev1.ClaimPending, false),
		)

		Context("Snapshot source format", func() {
			snapFormat := cdiv1.DataImportCronSourceFormatSnapshot
			var sc *storagev1.StorageClass

			BeforeEach(func() {
				sc = cc.CreateStorageClass(storageClassName, map[string]string{cc.AnnDefaultStorageClass: "true"})
				sp := &cdiv1.StorageProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: storageClassName,
					},
					Status: cdiv1.StorageProfileStatus{
						DataImportCronSourceFormat: &snapFormat,
						ClaimPropertySets: []cdiv1.ClaimPropertySet{
							{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								VolumeMode:  ptr.To[corev1.PersistentVolumeMode]("dummyfromsp"),
							},
						},
					},
				}
				reconciler = createDataImportCronReconciler(sc, sp, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
				storageProfile := &cdiv1.StorageProfile{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}}
				err := reconciler.client.Get(context.TODO(), client.ObjectKeyFromObject(storageProfile), storageProfile)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("Should handle sources in the event of storage class change", func(shouldCleanupOldScSource bool) {
				cron = newDataImportCron(cronName)
				dataSource = nil
				dvName := "test-datasource-68b44fc891f3"
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				cron.Status = cdiv1.DataImportCronStatus{
					CurrentImports: []cdiv1.ImportStatus{
						{
							DataVolumeName: dvName,
							Digest:         testDigest,
						},
					},
				}
				cc.AddAnnotation(cron, AnnStorageClass, sc.Name)
				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err := reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())

				dataSource = &cdiv1.DataSource{}
				snap := &snapshotv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dvName,
						Namespace: metav1.NamespaceDefault,
						Annotations: map[string]string{
							cc.AnnSourceVolumeMode: "dummy",
						},
					},
					Spec: snapshotv1.VolumeSnapshotSpec{
						Source: snapshotv1.VolumeSnapshotSource{
							PersistentVolumeClaimName: &dvName,
						},
					},
					Status: &snapshotv1.VolumeSnapshotStatus{
						ReadyToUse: ptr.To[bool](true),
					},
				}
				err = reconciler.client.Create(context.TODO(), snap)
				Expect(err).ToNot(HaveOccurred())

				verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &snapshotv1.VolumeSnapshot{})

				// Trigger desired storage class by spawning virt default sc variation
				virtSc := cc.CreateStorageClass(sc.Name+"-virt", map[string]string{cc.AnnDefaultVirtStorageClass: strconv.FormatBool(shouldCleanupOldScSource)})
				virtSp := &cdiv1.StorageProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: virtSc.Name,
					},
					Status: cdiv1.StorageProfileStatus{
						DataImportCronSourceFormat: &snapFormat,
					},
				}
				err = reconciler.client.Create(context.TODO(), virtSc)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Create(context.TODO(), virtSp)
				Expect(err).ToNot(HaveOccurred())
				if shouldCleanupOldScSource {
					// simulate first reconcile deleting old sources and requeueing
					_, err := reconciler.Reconcile(context.TODO(), cronReq)
					Expect(err).ToNot(HaveOccurred())
					verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, "NotFound", &snapshotv1.VolumeSnapshot{})
					err = reconciler.client.Get(context.TODO(), cronKey, cron)
					Expect(err).ToNot(HaveOccurred())
					Expect(cron.Annotations[AnnStorageClass]).To(Equal(virtSc.Name))
				} else {
					verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &snapshotv1.VolumeSnapshot{})
					err = reconciler.client.Get(context.TODO(), cronKey, cron)
					Expect(err).ToNot(HaveOccurred())
					Expect(cron.Annotations[AnnStorageClass]).To(Equal(sc.Name))
				}

				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				if !shouldCleanupOldScSource {
					Expect(err).ToNot(HaveOccurred())
					return
				}
				// Snapshot created by old storage class is gone
				Expect(err).To(And(HaveOccurred(), Satisfy(k8serrors.IsNotFound)))
				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))
			},
				Entry("source deleted when appropriate", true),
				Entry("source not deleted when not appropriate", false),
			)

			It("Should proceed to at least creating a PVC when no default storage class", func() {
				// Simulate an environment without default storage class
				sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}}
				sp := &cdiv1.StorageProfile{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}}
				err := reconciler.client.Delete(context.TODO(), sc)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Delete(context.TODO(), sp)
				Expect(err).ToNot(HaveOccurred())

				cron = newDataImportCron(cronName)
				dataSource = nil
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				err = reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &corev1.PersistentVolumeClaim{})

				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err = reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}
				verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &corev1.PersistentVolumeClaim{})

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())
				digest := imports[0].Digest
				Expect(digest).To(Equal(testDigest))

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))
			})

			It("Should create snapshot, and update DataImportCron once its ready to use", func() {
				cron = newDataImportCron(cronName)
				dataSource = nil
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				err := reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &snapshotv1.VolumeSnapshot{})

				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err = reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}
				verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &snapshotv1.VolumeSnapshot{})

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())
				digest := imports[0].Digest
				Expect(digest).To(Equal(testDigest))

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))

				pvc := cc.CreatePvc(dv.Name, dv.Namespace, nil, map[string]string{
					testKubevirtIoKey: testKubevirtIoValue,
				})
				pvc.Spec.VolumeMode = ptr.To[corev1.PersistentVolumeMode]("dummy")
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())
				// DV GCed after hitting succeeded
				err = reconciler.client.Delete(context.TODO(), dv)
				Expect(err).ToNot(HaveOccurred())
				// Reconcile that gets snapshot created
				verifyConditions("Snap creation reconcile", false, false, false, noImport, outdated, "SnapshotNotReady", &snapshotv1.VolumeSnapshot{})
				// Reconcile so created snapshot can be fetched
				verifyConditions("Snap creation triggered reconcile", false, false, false, noImport, inProgress, "SnapshotNotReady", &snapshotv1.VolumeSnapshot{})
				// Make snap ready so we reach UpToDate
				snap := &snapshotv1.VolumeSnapshot{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				Expect(err).ToNot(HaveOccurred())
				Expect(snap.Annotations[cc.AnnSourceVolumeMode]).To(Equal("dummy"))
				snap.Status = &snapshotv1.VolumeSnapshotStatus{
					ReadyToUse: ptr.To[bool](true),
				}
				err = reconciler.client.Update(context.TODO(), snap)
				Expect(err).ToNot(HaveOccurred())

				verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &snapshotv1.VolumeSnapshot{})

				sourcePVC := cdiv1.DataVolumeSourcePVC{
					Namespace: cron.Namespace,
					Name:      dvName,
				}
				expectedSource := cdiv1.DataSourceSource{
					Snapshot: &cdiv1.DataVolumeSourceSnapshot{
						Namespace: cron.Namespace,
						Name:      dvName,
					},
				}
				Expect(dataSource.Spec.Source).To(Equal(expectedSource))
				Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
				Expect(*cron.Status.LastImportedPVC).To(Equal(sourcePVC))
				Expect(cron.Status.LastImportTimestamp).ToNot(BeNil())
				// PVCs not around anymore, they are not needed, we are using a snapshot source
				pvcList := &corev1.PersistentVolumeClaimList{}
				err = reconciler.client.List(context.TODO(), pvcList, &client.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(pvcList.Items).To(BeEmpty())
				snap = &snapshotv1.VolumeSnapshot{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				Expect(err).ToNot(HaveOccurred())
				Expect(*snap.Status.ReadyToUse).To(BeTrue())
				Expect(*snap.Spec.Source.PersistentVolumeClaimName).To(Equal(dvName))
				// Expect labels to be copied from the source PVC
				Expect(snap.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))

				err = reconciler.client.Delete(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				_, err = reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())

				// Should delete DataSource and DataVolume on DataImportCron deletion as RetentionPolicy is RetainNone
				err = reconciler.client.Get(context.TODO(), dataSourceKey(cron), dataSource)
				Expect(err).To(HaveOccurred())
				dvList := &cdiv1.DataVolumeList{}
				err = reconciler.client.List(context.TODO(), dvList, &client.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(dvList.Items).To(BeEmpty())
				pvcList = &corev1.PersistentVolumeClaimList{}
				err = reconciler.client.List(context.TODO(), pvcList, &client.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(pvcList.Items).To(BeEmpty())
				snapList := &snapshotv1.VolumeSnapshotList{}
				err = reconciler.client.List(context.TODO(), snapList, &client.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(snapList.Items).To(BeEmpty())
			})

			It("Should respect default virt storage class by creating a snapshot using it's corresponding volumesnapclass", func() {
				virtSc := cc.CreateStorageClass(storageClassName+"-virt", map[string]string{cc.AnnDefaultVirtStorageClass: "true"})
				virtSp := &cdiv1.StorageProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: virtSc.Name,
					},
					Status: cdiv1.StorageProfileStatus{
						DataImportCronSourceFormat: &snapFormat,
					},
				}
				snapClass := &snapshotv1.VolumeSnapshotClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "VolumeSnapshotClass",
						APIVersion: snapshotv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: virtSc.Name + "-snapclass",
					},
					Driver: virtSc.Provisioner,
				}
				err := reconciler.client.Create(context.TODO(), virtSc)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Create(context.TODO(), virtSp)
				Expect(err).ToNot(HaveOccurred())
				err = reconciler.client.Create(context.TODO(), snapClass)
				Expect(err).ToNot(HaveOccurred())

				cron = newDataImportCron(cronName)
				dataSource = nil
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				cron.Spec.Template.Spec.PVC = nil
				cron.Spec.Template.Spec.Storage = &cdiv1.StorageSpec{}
				err = reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &snapshotv1.VolumeSnapshot{})

				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err = reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}
				verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &snapshotv1.VolumeSnapshot{})

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())
				digest := imports[0].Digest
				Expect(digest).To(Equal(testDigest))

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))

				pvc := cc.CreatePvc(dv.Name, dv.Namespace, nil, nil)
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())
				// DV GCed after hitting succeeded
				err = reconciler.client.Delete(context.TODO(), dv)
				Expect(err).ToNot(HaveOccurred())
				// Reconcile that gets snapshot created
				verifyConditions("Snap creation reconcile", false, false, false, noImport, outdated, "SnapshotNotReady", &snapshotv1.VolumeSnapshot{})
				// Reconcile so created snapshot can be fetched
				verifyConditions("Snap creation triggered reconcile", false, false, false, noImport, inProgress, "SnapshotNotReady", &snapshotv1.VolumeSnapshot{})
				// Make snap ready so we reach UpToDate
				snap := &snapshotv1.VolumeSnapshot{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				Expect(err).ToNot(HaveOccurred())
				Expect(snap.Spec.VolumeSnapshotClassName).To(HaveValue(Equal(snapClass.Name)))
			})

			It("Should not create snapshot from old storage class PVCs", func() {
				cron = newDataImportCron(cronName)
				dataSource = nil
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				err := reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &snapshotv1.VolumeSnapshot{})

				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err = reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}
				verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &snapshotv1.VolumeSnapshot{})

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())
				digest := imports[0].Digest
				Expect(digest).To(Equal(testDigest))

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))

				prevSc := "previous-storage-class"
				pvc := cc.CreatePvcInStorageClass(dv.Name, dv.Namespace, &prevSc, nil, nil, corev1.ClaimBound)
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())
				// DV GCed after hitting succeeded
				err = reconciler.client.Delete(context.TODO(), dv)
				Expect(err).ToNot(HaveOccurred())

				_, err = reconciler.Reconcile(context.TODO(), cronReq)
				Expect(err).ToNot(HaveOccurred())
				snap := &snapshotv1.VolumeSnapshot{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})

			It("Should set snapshot source volume mode annotation on carried-over-upgrade snapshot", func() {
				cron = newDataImportCron(cronName)
				dataSource = nil
				retentionPolicy := cdiv1.DataImportCronRetainNone
				cron.Spec.RetentionPolicy = &retentionPolicy
				err := reconciler.client.Create(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				verifyConditions("Before DesiredDigest is set", false, false, false, noImport, noDigest, "", &snapshotv1.VolumeSnapshot{})

				cc.AddAnnotation(cron, AnnSourceDesiredDigest, testDigest)
				err = reconciler.client.Update(context.TODO(), cron)
				Expect(err).ToNot(HaveOccurred())
				dataSource = &cdiv1.DataSource{}
				verifyConditions("After DesiredDigest is set", false, false, false, noImport, outdated, noSource, &snapshotv1.VolumeSnapshot{})

				imports := cron.Status.CurrentImports
				Expect(imports).ToNot(BeNil())
				Expect(imports).ToNot(BeEmpty())
				dvName := imports[0].DataVolumeName
				Expect(dvName).ToNot(BeEmpty())
				digest := imports[0].Digest
				Expect(digest).To(Equal(testDigest))

				dv := &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(*dv.Spec.Source.Registry.URL).To(Equal(testRegistryURL + "@" + testDigest))
				Expect(dv.Annotations[cc.AnnImmediateBinding]).To(Equal("true"))

				// DV GCed after hitting succeeded
				err = reconciler.client.Delete(context.TODO(), dv)
				Expect(err).ToNot(HaveOccurred())
				pvc := cc.CreatePvc(dv.Name, dv.Namespace, nil, nil)
				// Snap already exists, without the source volume mode annotation
				snap := &snapshotv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvc.Name,
						Namespace: metav1.NamespaceDefault,
					},
					Spec: snapshotv1.VolumeSnapshotSpec{
						Source: snapshotv1.VolumeSnapshotSource{
							PersistentVolumeClaimName: &pvc.Name,
						},
					},
					Status: &snapshotv1.VolumeSnapshotStatus{
						ReadyToUse: ptr.To[bool](true),
					},
				}
				err = reconciler.client.Create(context.TODO(), snap)
				Expect(err).ToNot(HaveOccurred())

				verifyConditions("Import succeeded", false, true, true, noImport, upToDate, ready, &snapshotv1.VolumeSnapshot{})

				snap = &snapshotv1.VolumeSnapshot{}
				err = reconciler.client.Get(context.TODO(), dvKey(dvName), snap)
				Expect(err).ToNot(HaveOccurred())
				Expect(*snap.Status.ReadyToUse).To(BeTrue())
				Expect(*snap.Spec.Source.PersistentVolumeClaimName).To(Equal(dvName))
				Expect(snap.Annotations[cc.AnnSourceVolumeMode]).To(Equal("dummyfromsp"))
			})
		})
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
	cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
	objs := []runtime.Object{cdiConfig}
	objs = append(objs, objects...)
	return createDataImportCronReconcilerWithoutConfig(objs...)
}

func createDataImportCronReconcilerWithoutConfig(objects ...runtime.Object) *DataImportCronReconciler {
	crd := &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "dataimportcrons.cdi.kubevirt.io"}}
	objs := []runtime.Object{crd, cc.MakeEmptyCDICR()}
	objs = append(objs, objects...)

	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = imagev1.Install(s)
	_ = extv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	rec := record.NewFakeRecorder(10)
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

func newPVC(name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
	}
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
							URL: ptr.To(testRegistryURL + testTag),
						},
					},
					Storage: &cdiv1.StorageSpec{},
				},
			},
			Schedule:          defaultSchedule,
			ManagedDataSource: dataSourceName,
			GarbageCollect:    ptr.To(cdiv1.DataImportCronGarbageCollectOutdated),
			ImportsToKeep:     ptr.To[int32](2),
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

func verifyDataImportCronOutdatedMetric(cron *cdiv1.DataImportCron, isPending bool, expectedValue int) {
	labels := prometheus.Labels{
		metrics.PrometheusCronNsLabel:      cron.Namespace,
		metrics.PrometheusCronNameLabel:    cron.Name,
		metrics.PrometheusCronPendingLabel: strconv.FormatBool(isPending),
	}
	Expect(int(metrics.GetDataImportCronOutdated(labels))).To(Equal(expectedValue))
}

func getEnvVar(env []corev1.EnvVar, name string) string {
	for _, envVar := range env {
		if envVar.Name == name {
			return envVar.Value
		}
	}
	return ""
}
