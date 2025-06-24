package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	dataImportCronTimeout         = 4 * time.Minute
	dataImportCronConvergeTimeout = 10 * time.Minute
	scheduleEveryMinute           = "* * * * *"
	scheduleOnceAYear             = "0 0 1 1 *"
	importsToKeep                 = 1
	emptySchedule                 = ""
	testKubevirtIoKey             = "test.kubevirt.io/test"
	testKubevirtIoValue           = "testvalue"
	// Digest must be 64 characters long
	errorDigest = "sha256:1234567890123456789012345678901234567890123456789012345678901234"
)

var _ = Describe("DataImportCron", Serial, func() {
	var (
		f                   = framework.NewFramework("dataimportcron-func-test")
		log                 = logf.Log.WithName("dataimportcron_test")
		dataSourceName      = "datasource-test"
		pollerPodName       = "poller"
		cronName            = "cron-test"
		cron                *cdiv1.DataImportCron
		reg                 *cdiv1.DataVolumeSourceRegistry
		err                 error
		ns                  string
		scName              string
		originalProfileSpec *cdiv1.StorageProfileSpec
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
		reg, err = getDataVolumeSourceRegistry(f)
		Expect(err).ToNot(HaveOccurred())

		scName = utils.DefaultStorageClass.GetName()
		By(fmt.Sprintf("Get original storage profile: %s", scName))

		spec, err := utils.GetStorageProfileSpec(f.CdiClient, scName)
		Expect(err).ToNot(HaveOccurred())
		originalProfileSpec = spec
	})

	AfterEach(func() {
		if err = utils.RemoveInsecureRegistry(f.CrClient, *reg.URL); err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to remove registry; %v", err)
		}
		err = utils.DeletePodByName(f.K8sClient, pollerPodName, f.CdiInstallNs, nil)
		Expect(err).ToNot(HaveOccurred())

		By("[AfterEach] Restore the profile")
		Expect(utils.UpdateStorageProfile(f.CrClient, scName, *originalProfileSpec)).Should(Succeed())

		By("[AfterEach] Delete the DataImportCron under test")
		// Delete the DataImportCron under test
		_ = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Delete(context.TODO(), cronName, metav1.DeleteOptions{})
		Eventually(func() bool {
			_, err := f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "DataImportCron was not deleted")

		By("[AfterEach] Wait for all DataImportCrons UpToDate")
		// Wait for all DataImportCrons to converge
		dataImportCrons := &cdiv1.DataImportCronList{}
		err = f.CrClient.List(context.TODO(), dataImportCrons, &client.ListOptions{Namespace: metav1.NamespaceAll})
		Expect(err).ToNot(HaveOccurred())
		for _, cronItem := range dataImportCrons.Items {
			Eventually(func() bool {
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(cronItem.Namespace).Get(context.TODO(), cronItem.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				condProgressing := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronProgressing)
				condUpToDate := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
				return condProgressing != nil && condProgressing.Status == corev1.ConditionFalse &&
					condUpToDate != nil && condUpToDate.Status == corev1.ConditionTrue
			}, dataImportCronConvergeTimeout, pollingInterval).Should(BeTrue(), "Timeout waiting for DataImportCron conditions %q", cronItem.Namespace+"/"+cronItem.Name)
		}
	})

	updateDigest := func(digest string) func(cron *cdiv1.DataImportCron) *cdiv1.DataImportCron {
		return func(cron *cdiv1.DataImportCron) *cdiv1.DataImportCron {
			cc.AddAnnotation(cron, controller.AnnSourceDesiredDigest, digest)
			return cron
		}
	}

	waitForDigest := func() {
		Eventually(func() string {
			cron, err := f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return cron.Annotations[controller.AnnSourceDesiredDigest]
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeEmpty(), "Desired digest is empty")
	}

	waitForConditions := func(statusProgressing, statusUpToDate corev1.ConditionStatus) {
		By(fmt.Sprintf("Wait for DataImportCron Progressing:%s, UpToDate:%s", statusProgressing, statusUpToDate))
		Eventually(func() bool {
			var err error
			cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			condProgressing := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronProgressing)
			condUpToDate := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
			return condProgressing != nil && condProgressing.Status == statusProgressing &&
				condUpToDate != nil && condUpToDate.Status == statusUpToDate
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "Timeout waiting for DataImportCron conditions")
	}

	configureStorageProfileResultingFormat := func(format cdiv1.DataImportCronSourceFormat) {
		By(fmt.Sprintf("configure storage profile %s", scName))
		newProfileSpec := originalProfileSpec.DeepCopy()
		newProfileSpec.DataImportCronSourceFormat = &format
		err := utils.UpdateStorageProfile(f.CrClient, scName, *newProfileSpec)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() *cdiv1.DataImportCronSourceFormat {
			profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), scName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return profile.Status.DataImportCronSourceFormat
		}, 15*time.Second, time.Second).Should(Equal(&format))
	}

	verifySourceReady := func(format cdiv1.DataImportCronSourceFormat, name string) metav1.Object {
		switch format {
		case cdiv1.DataImportCronSourceFormatPvc:
			By(fmt.Sprintf("Verify pvc was created %s", name))
			pvc, err := utils.WaitForPVC(f.K8sClient, ns, name)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for import completion")
			err = utils.WaitForDataVolumePhase(f, ns, cdiv1.Succeeded, name)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")
			return pvc
		case cdiv1.DataImportCronSourceFormatSnapshot:
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
			}
			snapshot = utils.WaitSnapshotReady(f.CrClient, snapshot)
			Expect(snapshot.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
			deleted, err := utils.WaitPVCDeleted(f.K8sClient, name, ns, 30*time.Second)
			if err != nil {
				// work around https://github.com/kubernetes-csi/external-snapshotter/issues/957
				// it does converge after the resync period of snapshot controller (15mins)
				cc.AddAnnotation(snapshot, "workaround", "triggersync")
				err = f.CrClient.Update(context.TODO(), snapshot)
				Expect(err).ToNot(HaveOccurred())
				// try again
				deleted, err = utils.WaitPVCDeleted(f.K8sClient, name, ns, 30*time.Second)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())
			// check pvc is not recreated
			Consistently(func() error {
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), name, metav1.GetOptions{})
				return err
			}, 5*time.Second, 1*time.Second).Should(
				SatisfyAll(HaveOccurred(), WithTransform(errors.IsNotFound, BeTrue())),
				"PVC should not have been recreated",
			)
			return snapshot
		}

		return nil
	}

	deleteSource := func(format cdiv1.DataImportCronSourceFormat, name string) {
		switch format {
		case cdiv1.DataImportCronSourceFormatPvc:
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			utils.CleanupDvPvcNoWait(f.K8sClient, f.CdiClient, f.Namespace.Name, name)
			deleted, err := f.WaitPVCDeletedByUID(pvc, time.Minute)
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())
		case cdiv1.DataImportCronSourceFormatSnapshot:
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
			}
			// Probably good to ensure deletion by UID here too but re-import is long enough
			// to not cause errors
			Eventually(func() bool {
				err := f.CrClient.Delete(context.TODO(), snapshot)
				return err != nil && errors.IsNotFound(err)
			}, time.Minute, time.Second).Should(BeTrue())
		}
	}

	getDataSourceName := func(format cdiv1.DataImportCronSourceFormat, ds *cdiv1.DataSource) string {
		var sourceName string

		switch format {
		case cdiv1.DataImportCronSourceFormatPvc:
			sourceName = ds.Spec.Source.PVC.Name
		case cdiv1.DataImportCronSourceFormatSnapshot:
			sourceName = ds.Spec.Source.Snapshot.Name
		}

		return sourceName
	}

	verifyRetention := func(format cdiv1.DataImportCronSourceFormat, name string) {
		By("Verify DataSource retention")
		_, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		Consistently(func() *metav1.Time {
			src := verifySourceReady(format, name)
			return src.GetDeletionTimestamp()
		}, 5*time.Second, time.Second).Should(BeNil())
	}

	verifyDeletion := func(format cdiv1.DataImportCronSourceFormat) {
		By("Verify DataSource deletion")
		Eventually(func() bool {
			_, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "DataSource was not deleted")

		By("Verify sources deleted")
		Eventually(func(g Gomega) bool {
			pvcs, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			return len(pvcs.Items) == 0
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "PVCs were not deleted")

		if format == cdiv1.DataImportCronSourceFormatSnapshot {
			snapshots := &snapshotv1.VolumeSnapshotList{}
			Eventually(func(g Gomega) bool {
				err := f.CrClient.List(context.TODO(), snapshots, &client.ListOptions{Namespace: ns})
				g.Expect(err).ToNot(HaveOccurred())
				return len(snapshots.Items) == 0
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "snapshots were not deleted")
		}
	}

	DescribeTable("should", func(retention, createErrorDv bool, repeat int, format cdiv1.DataImportCronSourceFormat) {
		if format == cdiv1.DataImportCronSourceFormatSnapshot && !f.IsSnapshotStorageClassAvailable() {
			Skip("Volumesnapshot support needed to test DataImportCron with Volumesnapshot sources")
		}

		configureStorageProfileResultingFormat(format)

		By(fmt.Sprintf("Create new DataImportCron %s, url %s", cronName, *reg.URL))
		cron = utils.NewDataImportCron(cronName, "5Gi", scheduleEveryMinute, dataSourceName, importsToKeep, *reg)

		garbageCollect := cdiv1.DataImportCronGarbageCollectNever
		cron.Spec.GarbageCollect = &garbageCollect

		if !retention {
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy
		}
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		if reg.PullMethod == nil || *reg.PullMethod == cdiv1.RegistryPullPod {
			By("Verify cronjob was created")
			Eventually(func() bool {
				_, err := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), controller.GetCronJobName(cron), metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				return true
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "cronjob was not created")
		}

		var lastImportDv, currentImportDv string
		for i := 0; i < repeat; i++ {
			By(fmt.Sprintf("Iter #%d", i))
			if i > 0 {
				if createErrorDv {
					By("Set desired digest to nonexisting one")

					//get and update!!!
					retryOnceOnErr(updateDataImportCron(f.CdiClient, ns, cronName, updateDigest(errorDigest))).Should(BeNil())

					By("Wait for CurrentImports update")
					Eventually(func() string {
						cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
						currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
						Expect(currentImportDv).ToNot(BeEmpty())
						return currentImportDv
					}, dataImportCronTimeout, pollingInterval).ShouldNot(Equal(lastImportDv), "Current import was not updated")
					lastImportDv = currentImportDv
				} else {
					By("Reset desired digest")
					retryOnceOnErr(updateDataImportCron(f.CdiClient, ns, cronName, updateDigest(""))).Should(BeNil())

					By(fmt.Sprintf("Delete last import %s, format: %s", currentImportDv, format))
					deleteSource(format, currentImportDv)
					lastImportDv = ""

					By("Wait for non-empty desired digest")
					waitForDigest()
				}
			}

			waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
			By("Verify CurrentImports update")
			currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
			Expect(currentImportDv).ToNot(BeEmpty())
			Expect(currentImportDv).ToNot(Equal(lastImportDv))
			lastImportDv = currentImportDv

			currentSource := verifySourceReady(format, currentImportDv)

			By("Verify DataSource was updated")
			var dataSource *cdiv1.DataSource
			Eventually(func(g Gomega) {
				dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				readyCond := controller.FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
				g.Expect(readyCond).ToNot(BeNil())
				g.Expect(readyCond.Status).To(Equal(corev1.ConditionTrue))
				g.Expect(getDataSourceName(format, dataSource)).To(Equal(currentImportDv))
				g.Expect(dataSource.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
			}, dataImportCronTimeout, pollingInterval).Should(Succeed(), "DataSource was not updated")

			By("Verify cron was updated")
			Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
			Expect(cron.Status.LastImportedPVC.Name).To(Equal(currentImportDv))

			By("Update DataSource to refer to a dummy name")
			retryOnceOnErr(
				updateDataSource(f.CdiClient, ns, cron.Spec.ManagedDataSource,
					func(dataSource *cdiv1.DataSource) *cdiv1.DataSource {
						switch format {
						case cdiv1.DataImportCronSourceFormatPvc:
							dataSource.Spec.Source.PVC.Name = "dummy"
						case cdiv1.DataImportCronSourceFormatSnapshot:
							dataSource.Spec.Source.Snapshot.Name = "dummy"
						}
						return dataSource
					},
				)).Should(BeNil())

			By("Verify name on DataSource was reconciled")
			Eventually(func() bool {
				dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return getDataSourceName(format, dataSource) == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "DataSource name was not reconciled")

			By("Delete DataSource")
			err = f.CdiClient.CdiV1beta1().DataSources(ns).Delete(context.TODO(), dataSourceName, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			By("Verify DataSource was re-created")
			Eventually(func() bool {
				ds, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				return err == nil && ds.UID != dataSource.UID
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "DataSource was not re-created")

			By("Delete last imported source")
			deleteSource(format, currentSource.GetName())
			By("Verify last imported source was re-created")
			recreatedSource := verifySourceReady(format, currentSource.GetName())
			Expect(recreatedSource.GetUID()).ToNot(Equal(currentSource.GetUID()), "Last imported source was not re-created")
		}

		lastImportedPVC := cron.Status.LastImportedPVC

		By("Delete cron")
		err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Delete(context.TODO(), cronName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		if retention {
			verifyRetention(format, lastImportedPVC.Name)
		} else {
			verifyDeletion(format)
		}
	},
		Entry("[test_id:7403] succeed importing initial PVC from registry URL", true, false, 1, cdiv1.DataImportCronSourceFormatPvc),
		Entry("[test_id:7414] succeed importing PVC from registry URL on source digest update", true, false, 2, cdiv1.DataImportCronSourceFormatPvc),
		Entry("[test_id:10031] succeed importing initially into a snapshot from registry URL", true, false, 1, cdiv1.DataImportCronSourceFormatSnapshot),
		Entry("[test_id:10032] succeed importing to a snapshot from registry URL on source digest update", true, false, 2, cdiv1.DataImportCronSourceFormatSnapshot),
		Entry("[test_id:8266] succeed deleting error DVs when importing new ones", false, true, 2, cdiv1.DataImportCronSourceFormatPvc),
	)

	It("[test_id:10040] Should get digest updated by external poller", func() {
		By("Create DataImportCron with only initial poller job")
		cron = utils.NewDataImportCron(cronName, "5Gi", scheduleOnceAYear, dataSourceName, importsToKeep, *reg)
		retentionPolicy := cdiv1.DataImportCronRetainNone
		cron.Spec.RetentionPolicy = &retentionPolicy
		cron, err := f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Wait for initial digest")
		waitForDigest()

		By("Set empty digest")
		retryOnceOnErr(updateDataImportCron(f.CdiClient, ns, cron.Name, updateDigest(""))).Should(BeNil())

		By("Create poller pod to update the DataImportCron digest")
		importerImage := f.GetEnvVarValue("IMPORTER_IMAGE")
		Expect(importerImage).ToNot(BeEmpty())

		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pollerPodName}}
		err = controller.InitPollerPodSpec(f.CrClient, cron, &pod.Spec, importerImage, corev1.PullIfNotPresent, log)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.CreatePod(f.K8sClient, f.CdiInstallNs, pod)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for digest set by external poller")
		waitForDigest()
	})

	It("[test_id:10360] Should allow an empty schedule to trigger an external update to the source", func() {
		configureStorageProfileResultingFormat(cdiv1.DataImportCronSourceFormatPvc)

		By("Create DataImportCron with empty schedule")
		cron = utils.NewDataImportCron(cronName, "5Gi", emptySchedule, dataSourceName, importsToKeep, *reg)
		retentionPolicy := cdiv1.DataImportCronRetainNone
		cron.Spec.RetentionPolicy = &retentionPolicy

		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Create poller pod to update the DataImportCron digest")
		importerImage := f.GetEnvVarValue("IMPORTER_IMAGE")
		Expect(importerImage).ToNot(BeEmpty())

		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pollerPodName}}
		err = controller.InitPollerPodSpec(f.CrClient, cron, &pod.Spec, importerImage, corev1.PullIfNotPresent, log)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.CreatePod(f.K8sClient, f.CdiInstallNs, pod)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for digest set by external poller")
		waitForDigest()

		waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
		By("Verify CurrentImports update")
		currentImportDv := cron.Status.CurrentImports[0].DataVolumeName
		Expect(currentImportDv).ToNot(BeEmpty())

		By(fmt.Sprintf("Verify pvc was created %s", currentImportDv))
		_, err = utils.WaitForPVC(f.K8sClient, ns, currentImportDv)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for import completion")
		err = utils.WaitForDataVolumePhase(f, ns, cdiv1.Succeeded, currentImportDv)
		Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

		By("Verify cronjob was not created")
		_, err = f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), controller.GetCronJobName(cron), metav1.GetOptions{})
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	DescribeTable("Succeed garbage collecting sources when importing new ones", func(format cdiv1.DataImportCronSourceFormat) {
		if format == cdiv1.DataImportCronSourceFormatSnapshot && !f.IsSnapshotStorageClassAvailable() {
			Skip("Volumesnapshot support needed to test DataImportCron with Volumesnapshot sources")
		}
		const oldDvName = "old-version-dv"

		configureStorageProfileResultingFormat(format)

		garbageSources := 3
		for i := 0; i < garbageSources; i++ {
			srcName := fmt.Sprintf("src-garbage-%d", i)
			By(fmt.Sprintf("Create %s", srcName))
			switch format {
			case cdiv1.DataImportCronSourceFormatPvc:
				pvc := utils.NewPVCDefinition(srcName, "1Gi",
					map[string]string{controller.AnnLastUseTime: time.Now().UTC().Format(time.RFC3339Nano)},
					map[string]string{common.DataImportCronLabel: cronName})
				f.CreateBoundPVCFromDefinition(pvc)
			case cdiv1.DataImportCronSourceFormatSnapshot:
				pvc := utils.NewPVCDefinition(srcName, "1Gi",
					map[string]string{controller.AnnLastUseTime: time.Now().UTC().Format(time.RFC3339Nano)},
					map[string]string{common.DataImportCronLabel: cronName})
				f.CreateBoundPVCFromDefinition(pvc)
				snapClass := f.GetSnapshotClass()
				snapshot := utils.NewVolumeSnapshot(srcName, ns, pvc.Name, &snapClass.Name)
				snapshot.SetAnnotations(map[string]string{controller.AnnLastUseTime: time.Now().UTC().Format(time.RFC3339Nano)})
				snapshot.SetLabels(map[string]string{common.DataImportCronLabel: cronName})
				err = f.CrClient.Create(context.TODO(), snapshot)
				Expect(err).ToNot(HaveOccurred())
				utils.WaitSnapshotReady(f.CrClient, snapshot)
				err = f.DeletePVC(pvc)
				Expect(err).ToNot(HaveOccurred())
				deleted, err := utils.WaitPVCDeleted(f.K8sClient, srcName, ns, 2*time.Minute)
				Expect(err).ToNot(HaveOccurred())
				Expect(deleted).To(BeTrue())
			}
		}

		switch format {
		case cdiv1.DataImportCronSourceFormatPvc:
			By(fmt.Sprintf("Create labeled DataVolume %s for old DVs garbage collection test", oldDvName))
			dv := utils.NewDataVolumeWithRegistryImport(oldDvName, "5Gi", "")
			dv.Spec.Source.Registry = reg
			dv.Labels = map[string]string{common.DataImportCronLabel: cronName}
			dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, ns, dv)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for import completion")
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)
			err = utils.WaitForDataVolumePhase(f, ns, cdiv1.Succeeded, dv.Name)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

			By(fmt.Sprintf("Verify PVC was created %s", dv.Name))
			pvc, err := utils.WaitForPVC(f.K8sClient, ns, dv.Name)
			Expect(err).ToNot(HaveOccurred())
			By(fmt.Sprintf("Verify DataImportCronLabel is passed to the PVC: %s", pvc.Labels[common.DataImportCronLabel]))
			Expect(pvc.Labels[common.DataImportCronLabel]).To(Equal(cronName))

			pvc.Labels[common.DataImportCronLabel] = ""
			By("Update DataImportCron label to be empty in the PVC")
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []corev1.PersistentVolumeClaim {
				pvcList, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return pvcList.Items
			}, dataImportCronTimeout, pollingInterval).Should(HaveLen(garbageSources + 1))
		case cdiv1.DataImportCronSourceFormatSnapshot:
			snapshots := &snapshotv1.VolumeSnapshotList{}
			err := f.CrClient.List(context.TODO(), snapshots, &client.ListOptions{Namespace: ns})
			Expect(err).ToNot(HaveOccurred())
			Expect(snapshots.Items).To(HaveLen(garbageSources))
		}

		By(fmt.Sprintf("Create new DataImportCron %s, url %s", cronName, *reg.URL))
		cron = utils.NewDataImportCron(cronName, "1Gi", scheduleEveryMinute, dataSourceName, importsToKeep, *reg)
		retentionPolicy := cdiv1.DataImportCronRetainNone
		cron.Spec.RetentionPolicy = &retentionPolicy

		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
		By("Verify CurrentImports update")
		currentImportDv := cron.Status.CurrentImports[0].DataVolumeName
		Expect(currentImportDv).ToNot(BeEmpty())

		currentSource := verifySourceReady(format, currentImportDv)

		By("Check garbage collection")
		switch format {
		case cdiv1.DataImportCronSourceFormatPvc:
			By("Check old DV garbage collection")
			Eventually(func() error {
				_, err := f.CdiClient.CdiV1beta1().DataVolumes(ns).Get(context.TODO(), oldDvName, metav1.GetOptions{})
				return err
			}, dataImportCronTimeout, pollingInterval).Should(Satisfy(errors.IsNotFound), "Garbage collection failed cleaning old DV")

			pvcList := &corev1.PersistentVolumeClaimList{}
			Eventually(func() int {
				pvcList, err = f.K8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(pvcList.Items)
			}, dataImportCronTimeout, pollingInterval).Should(Equal(importsToKeep), "Garbage collection failed cleaning old imports")

			By("Check last import PVC is timestamped and not garbage collected")
			found := false
			for _, pvc := range pvcList.Items {
				if pvc.UID == currentSource.GetUID() {
					lastUse := pvc.Annotations[controller.AnnLastUseTime]
					Expect(lastUse).ToNot(BeEmpty())
					ts, err := time.Parse(time.RFC3339Nano, lastUse)
					Expect(err).ToNot(HaveOccurred())
					Expect(ts).To(BeTemporally("<", time.Now()))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		case cdiv1.DataImportCronSourceFormatSnapshot:
			snapshots := &snapshotv1.VolumeSnapshotList{}
			Eventually(func(g Gomega) int {
				err := f.CrClient.List(context.TODO(), snapshots, &client.ListOptions{Namespace: ns})
				g.Expect(err).ToNot(HaveOccurred())
				return len(snapshots.Items)
			}, dataImportCronTimeout, pollingInterval).Should(Equal(importsToKeep), "Garbage collection failed cleaning old imports")

			By("Check last import snapshot is timestamped and not garbage collected")
			found := false
			for _, snap := range snapshots.Items {
				if snap.UID == currentSource.GetUID() {
					lastUse := snap.Annotations[controller.AnnLastUseTime]
					Expect(lastUse).ToNot(BeEmpty())
					ts, err := time.Parse(time.RFC3339Nano, lastUse)
					Expect(err).ToNot(HaveOccurred())
					Expect(ts).To(BeTemporally("<", time.Now()))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		}
	},
		Entry("[test_id:7406] with PVC & DV sources", cdiv1.DataImportCronSourceFormatPvc),
		Entry("[test_id:10033] with snapshot sources", cdiv1.DataImportCronSourceFormatSnapshot),
	)

	It("[test_id:8033] should delete jobs on deletion", func() {
		if reg.PullMethod != nil && *reg.PullMethod == cdiv1.RegistryPullNode {
			Skip("No cronjobs on pullMethod: node")
		}
		noSuchCM := "nosuch"
		reg.CertConfigMap = &noSuchCM
		cron = utils.NewDataImportCron("cron-test", "5Gi", scheduleEveryMinute, dataSourceName, importsToKeep, *reg)
		By("Create new DataImportCron")
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify initial job created")
		initialJobName := controller.GetInitialJobName(cron)
		Eventually(func() *batchv1.Job {
			job, _ := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), initialJobName, metav1.GetOptions{})
			return job
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil(), "initial job was not created")

		By("Verify initial job pod created")
		Eventually(func() *corev1.Pod {
			pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, initialJobName, "")
			return pod
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil(), "initial job pod was not created")

		By("Verify cronjob created and has active job")
		cronJobName := controller.GetCronJobName(cron)
		jobName := ""
		Eventually(func() string {
			cronjob, _ := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
			if cronjob != nil && len(cronjob.Status.Active) > 0 {
				jobName = cronjob.Status.Active[0].Name
			}
			return jobName
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeEmpty(), "cronjob has no active job")

		By("Verify cronjob first job created")
		Eventually(func() *batchv1.Job {
			job, _ := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), jobName, metav1.GetOptions{})
			return job
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil(), "cronjob first job was not created")

		By("Verify cronjob first job pod created")
		Eventually(func() *corev1.Pod {
			pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, jobName, "")
			return pod
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil(), "cronjob first job pod was not created")

		By("Delete cron")
		err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Delete(context.TODO(), cronName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify cronjob deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "cronjob was not deleted")

		By("Verify initial job deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), initialJobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "initial job was not deleted")

		By("Verify initial job pod deleted")
		Eventually(func() bool {
			_, err := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, initialJobName, "")
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "initial job pod was not deleted")

		By("Verify cronjob first job deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), jobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "cronjob first job was not deleted")

		By("Verify cronjob first job pod deleted")
		Eventually(func() bool {
			_, err := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, jobName, "")
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue(), "cronjob first job pod was not deleted")
	})

	Context("Change source format of existing DataImportCron", func() {
		It("[test_id:10034] Should allow switching back and forth from PVC to snapshot sources", func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Volumesnapshot support needed to test DataImportCron with Volumesnapshot sources")
			}
			size := "1Gi"

			configureStorageProfileResultingFormat(cdiv1.DataImportCronSourceFormatPvc)

			By(fmt.Sprintf("Create new DataImportCron %s, url %s", cronName, *reg.URL))
			cron = utils.NewDataImportCronWithStorageSpec(cronName, size, scheduleOnceAYear, dataSourceName, importsToKeep, *reg)
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy

			cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
			By("Verify CurrentImports update")
			currentImportDv := cron.Status.CurrentImports[0].DataVolumeName
			Expect(currentImportDv).ToNot(BeEmpty())

			_ = verifySourceReady(cdiv1.DataImportCronSourceFormatPvc, currentImportDv)
			snapshots := &snapshotv1.VolumeSnapshotList{}
			err = f.CrClient.List(context.TODO(), snapshots, &client.ListOptions{Namespace: ns})
			Expect(err).ToNot(HaveOccurred())
			Expect(snapshots.Items).To(BeEmpty())
			// Ensure existing PVC clones from this source don't mess up future ones
			cloneDV := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv-from-pvc", size, ns, currentImportDv, nil, nil)
			cloneDV, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(cloneDV)
			err = utils.WaitForDataVolumePhase(f, cloneDV.Namespace, cdiv1.Succeeded, cloneDV.Name)

			// Now simulate an upgrade, where a new CDI version has identified
			// more storage types that scale better with snapshots
			configureStorageProfileResultingFormat(cdiv1.DataImportCronSourceFormatSnapshot)
			// Check snapshot now exists and PVC is gone
			currentSource := verifySourceReady(cdiv1.DataImportCronSourceFormatSnapshot, currentImportDv)
			waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
			// DataSource is updated to point to a snapshot
			dataSource, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			readyCond := controller.FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
			Expect(readyCond.Status).To(Equal(corev1.ConditionTrue))
			expectedSource := cdiv1.DataSourceSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Name:      currentSource.GetName(),
					Namespace: currentSource.GetNamespace(),
				},
			}
			Expect(dataSource.Spec.Source).To(Equal(expectedSource))
			// Verify content
			targetDV := utils.NewDataVolumeWithSourceRefAndStorageAPI("target-dv-from-snap", &size, dataSource.Namespace, dataSource.Name)
			By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
			targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, ns, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

			By("Wait for clone DV Succeeded phase")
			err = utils.WaitForDataVolumePhase(f, targetDataVolume.Namespace, cdiv1.Succeeded, targetDataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			By("Verify MD5")
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetDataVolume.Namespace).Get(context.TODO(), targetDataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			path := utils.DefaultImagePath
			volumeMode := pvc.Spec.VolumeMode
			if volumeMode != nil && *volumeMode == corev1.PersistentVolumeBlock {
				path = utils.DefaultPvcMountPath
			}
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, path, utils.UploadFileMD5, utils.UploadFileSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())

			By("Switch back from snap to PVC source")
			configureStorageProfileResultingFormat(cdiv1.DataImportCronSourceFormatPvc)
			currentSource = verifySourceReady(cdiv1.DataImportCronSourceFormatPvc, currentImportDv)
			dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			expectedSource = cdiv1.DataSourceSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      currentSource.GetName(),
					Namespace: currentSource.GetNamespace(),
				},
			}
			Expect(dataSource.Spec.Source).To(Equal(expectedSource))
		})
	})
})

func getDataVolumeSourceRegistry(f *framework.Framework) (*cdiv1.DataVolumeSourceRegistry, error) {
	reg := &cdiv1.DataVolumeSourceRegistry{}
	var (
		pullMethod cdiv1.RegistryPullMethod
		url        string
	)
	if utils.IsOpenshift(f.K8sClient) {
		url = fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs)
		pullMethod = cdiv1.RegistryPullPod
	} else {
		url = fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix)
		pullMethod = cdiv1.RegistryPullNode
	}
	reg.URL = &url
	reg.PullMethod = &pullMethod
	if err := utils.AddInsecureRegistry(f.CrClient, url); err != nil {
		return nil, err
	}
	return reg, nil
}

func updateDataImportCron(clientSet *cdiclientset.Clientset, namespace string, cronName string,
	update func(cron *cdiv1.DataImportCron) *cdiv1.DataImportCron) func() error {
	return func() error {
		cron, err := clientSet.CdiV1beta1().DataImportCrons(namespace).Get(context.TODO(), cronName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		cron = update(cron)

		_, err = clientSet.CdiV1beta1().DataImportCrons(namespace).Update(context.TODO(), cron, metav1.UpdateOptions{})
		return err
	}
}

func updateDataSource(clientSet *cdiclientset.Clientset, namespace string, dataSourceName string,
	update func(dataSource *cdiv1.DataSource) *cdiv1.DataSource) func() error {
	return func() error {
		dataSource, err := clientSet.CdiV1beta1().DataSources(namespace).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		dataSource = update(dataSource)

		_, err = clientSet.CdiV1beta1().DataSources(namespace).Update(context.TODO(), dataSource, metav1.UpdateOptions{})
		return err
	}
}

func retryOnceOnErr(f func() error) Assertion {
	err := f()
	if err != nil {
		err = f()
	}

	return Expect(err)
}
