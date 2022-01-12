package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	dataImportCronTimeout = 4 * time.Minute
	scheduleEveryMinute   = "* * * * *"
	scheduleOnceAYear     = "0 0 1 1 *"
)

var (
	importsToKeep       int32 = 1
	registryPullNode          = cdiv1.RegistryPullNode
	externalRegistryURL       = "docker://quay.io/kubevirt/cirros-container-disk-demo"
)

var _ = Describe("DataImportCron", func() {
	var (
		f                  = framework.NewFramework(namespacePrefix)
		trustedRegistryURL = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix) }
		dataSourceName     = "datasource-test"
		cron               *cdiv1.DataImportCron
		err                error
	)

	table.DescribeTable("should", func(schedule string, retentionPolicy cdiv1.DataImportCronRetentionPolicy, repeat int, checkGarbageCollection bool) {
		var url string

		if repeat > 1 || utils.IsOpenshift(f.K8sClient) {
			url = externalRegistryURL
		} else {
			url = trustedRegistryURL()
			err = utils.AddInsecureRegistry(f.CrClient, url)
			Expect(err).To(BeNil())

			hasInsecReg, err := utils.HasInsecureRegistry(f.CrClient, url)
			Expect(err).ToNot(HaveOccurred())
			Expect(hasInsecReg).To(BeTrue())
			defer utils.RemoveInsecureRegistry(f.CrClient, url)
		}

		cron = NewDataImportCron("cron-test", "5Gi", schedule, dataSourceName, cdiv1.DataVolumeSourceRegistry{URL: &url, PullMethod: &registryPullNode}, retentionPolicy)
		By(fmt.Sprintf("Create new DataImportCron %s", url))
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		if schedule == scheduleEveryMinute {
			By("Verify cronjob was created")
			Eventually(func() bool {
				_, err = f.K8sClient.BatchV1beta1().CronJobs(f.CdiInstallNs).Get(context.TODO(), controller.GetCronJobName(cron), metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				return err == nil
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}

		var lastImportDv, currentImportDv string
		for i := 0; i < repeat; i++ {
			if i > 0 {
				// Emulate source update using digests from https://quay.io/repository/kubevirt/cirros-container-disk-demo?tab=tags
				digest := []string{
					"sha256:68b44fc891f3fae6703d4b74bcc9b5f24df8d23f12e642805d1420cbe7a4be70",
					"sha256:90e064fca2f47eabce210d218a45ba48cc7105b027d3f39761f242506cad15d6",
				}[i%2]

				By(fmt.Sprintf("Update source desired digest to %s", digest))
				Eventually(func() bool {
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if cron.Annotations == nil {
						cron.Annotations = make(map[string]string)
					}
					cron.Annotations[controller.AnnSourceDesiredDigest] = digest
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Update(context.TODO(), cron, metav1.UpdateOptions{})
					return err == nil
				}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
			}
			By("Wait for CurrentImports DataVolumeName update")
			Eventually(func() bool {
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if len(cron.Status.CurrentImports) == 0 {
					return false
				}
				currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
				return currentImportDv != "" && currentImportDv != lastImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			lastImportDv = currentImportDv

			By(fmt.Sprintf("Verify pvc was created %s", currentImportDv))
			currentPvc, err := utils.WaitForPVC(f.K8sClient, cron.Namespace, currentImportDv)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for import completion")
			err = utils.WaitForDataVolumePhase(f.CdiClient, cron.Namespace, cdiv1.Succeeded, currentImportDv)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

			By("Verify datasource was updated")
			var dataSource *cdiv1.DataSource
			Eventually(func() bool {
				dataSource, err = f.CdiClient.CdiV1beta1().DataSources(f.Namespace.Name).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				readyCond := controller.FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
				return readyCond != nil && readyCond.Status == corev1.ConditionTrue &&
					dataSource.Spec.Source.PVC != nil && dataSource.Spec.Source.PVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Verify cron was updated")
			Eventually(func() bool {
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progressCond := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronProgressing)
				upToDateCond := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
				return progressCond != nil && progressCond.Status == corev1.ConditionFalse &&
					upToDateCond != nil && upToDateCond.Status == corev1.ConditionTrue &&
					cron.Status.LastImportedPVC != nil && cron.Status.LastImportedPVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Delete DataSource")
			err = f.CdiClient.CdiV1beta1().DataSources(dataSource.Namespace).Delete(context.TODO(), dataSource.Name, metav1.DeleteOptions{})
			Expect(err).To(BeNil())
			By("Verify DataSource was re-created")
			Eventually(func() bool {
				ds, err := f.CdiClient.CdiV1beta1().DataSources(dataSource.Namespace).Get(context.TODO(), dataSource.Name, metav1.GetOptions{})
				return err == nil && ds.UID != dataSource.UID
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Delete last imported PVC")
			err = f.DeletePVC(currentPvc)
			Expect(err).To(BeNil())
			By("Verify last imported PVC was re-created")
			Eventually(func() bool {
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(currentPvc.Namespace).Get(context.TODO(), currentPvc.Name, metav1.GetOptions{})
				return err == nil && pvc.UID != currentPvc.UID
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}
		if checkGarbageCollection {
			Eventually(func() bool {
				dvList, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(dvList.Items) == int(importsToKeep)
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}

		lastImportedPVC := cron.Status.LastImportedPVC
		retention := cron.Spec.RetentionPolicy

		By("Delete cron")
		err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Delete(context.TODO(), cron.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		if retention != nil && *retention == cdiv1.DataImportCronRetainNone {
			By("Verify DataSource deletion")
			Eventually(func() bool {
				_, err := f.CdiClient.CdiV1beta1().DataSources(f.Namespace.Name).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Verify PVCs deletion")
			Eventually(func() bool {
				pvcs, err := f.K8sClient.CoreV1().PersistentVolumeClaims(lastImportedPVC.Namespace).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(pvcs.Items) == 0
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		} else {
			By("Verify DataSource retention")
			_, err := f.CdiClient.CdiV1beta1().DataSources(f.Namespace.Name).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
			Expect(err).To(BeNil())

			By("Verify last PVC retention")
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(lastImportedPVC.Namespace).Get(context.TODO(), lastImportedPVC.Name, metav1.GetOptions{})
			Expect(err).To(BeNil())
		}
	},
		table.Entry("[test_id:7403] Should successfully import PVC from registry URL as scheduled", scheduleEveryMinute, cdiv1.DataImportCronRetainAll, 1, false),
		table.Entry("[test_id:7414] Should successfully import PVC from registry URL on source digest update", scheduleOnceAYear, cdiv1.DataImportCronRetainAll, 2, false),
		table.Entry("[test_id:7406] Should successfully garbage collect old PVCs when importing new ones", scheduleOnceAYear, cdiv1.DataImportCronRetainNone, 2, true),
	)

	It("[test_id:8033] should delete jobs on deletion", func() {
		url := trustedRegistryURL()
		noSuchCM := "nosuch"
		cron = NewDataImportCron("cron-test", "5Gi", scheduleEveryMinute, dataSourceName, cdiv1.DataVolumeSourceRegistry{URL: &url, PullMethod: &registryPullNode, CertConfigMap: &noSuchCM}, cdiv1.DataImportCronRetainAll)
		By("Create new DataImportCron")
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify initial job created")
		initialJobName := controller.GetInitialJobName(cron)
		Eventually(func() *batchv1.Job {
			job, _ := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), initialJobName, metav1.GetOptions{})
			return job
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil())

		By("Verify initial job pod created")
		Eventually(func() *corev1.Pod {
			pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, initialJobName, "")
			return pod
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil())

		By("Verify cronjob created and has active job")
		cronJobName := controller.GetCronJobName(cron)
		jobName := ""
		Eventually(func() string {
			cronjob, _ := f.K8sClient.BatchV1beta1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
			if cronjob != nil && len(cronjob.Status.Active) > 0 {
				jobName = cronjob.Status.Active[0].Name
			}
			return jobName
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeEmpty())

		By("Verify cronjob first job created")
		Eventually(func() *batchv1.Job {
			job, _ := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), jobName, metav1.GetOptions{})
			return job
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil())

		By("Verify cronjob first job pod created")
		Eventually(func() *corev1.Pod {
			pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, jobName, "")
			return pod
		}, dataImportCronTimeout, pollingInterval).ShouldNot(BeNil())

		By("Delete cron")
		err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Delete(context.TODO(), cron.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify initial job deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), initialJobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

		By("Verify initial job pod deleted")
		Eventually(func() bool {
			_, err := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, initialJobName, "")
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

		By("Verify cronjob first job deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), jobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

		By("Verify cronjob first job pod deleted")
		Eventually(func() bool {
			_, err := utils.FindPodByPrefixOnce(f.K8sClient, f.CdiInstallNs, jobName, "")
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
	})
})

// NewDataImportCron initializes a DataImportCron struct
func NewDataImportCron(name, size, schedule, dataSource string, source cdiv1.DataVolumeSourceRegistry, retentionPolicy cdiv1.DataImportCronRetentionPolicy) *cdiv1.DataImportCron {
	garbageCollect := cdiv1.DataImportCronGarbageCollectOutdated

	return &cdiv1.DataImportCron{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				controller.AnnImmediateBinding: "true",
			},
		},
		Spec: cdiv1.DataImportCronSpec{
			Template: cdiv1.DataVolume{
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{
						Registry: &source,
					},
					PVC: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(size),
							},
						},
					},
				},
			},
			Schedule:          schedule,
			ManagedDataSource: dataSource,
			GarbageCollect:    &garbageCollect,
			ImportsToKeep:     &importsToKeep,
			RetentionPolicy:   &retentionPolicy,
		},
	}
}
