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
	"kubevirt.io/containerized-data-importer/pkg/common"
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
	importsToKeep int32 = 1
)

var _ = Describe("DataImportCron", func() {
	var (
		f              = framework.NewFramework(namespacePrefix)
		dataSourceName = "datasource-test"
		cronName       = "cron-test"
		dvName         = "dv-garbage"
		cron           *cdiv1.DataImportCron
		ns             string
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
	})

	table.DescribeTable("should", func(garbageCollection, retention, createErrorDv bool, repeat int) {
		reg, err := getDataVolumeSourceRegistry(f)
		Expect(err).To(BeNil())
		defer utils.RemoveInsecureRegistry(f.CrClient, *reg.URL)

		By(fmt.Sprintf("Create labeled DatVolume %s for garbage collection test", dvName))
		dv := utils.NewDataVolumeWithRegistryImport(dvName, "5Gi", "")
		dv.Spec.Source.Registry = reg
		dv.Labels = map[string]string{common.DataImportCronLabel: cronName}
		_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, ns, dv)
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("Create new DataImportCron %s, url %s", cronName, *reg.URL))
		cron = NewDataImportCron(cronName, "5Gi", scheduleEveryMinute, dataSourceName, *reg)
		if !garbageCollection {
			garbageCollect := cdiv1.DataImportCronGarbageCollectNever
			cron.Spec.GarbageCollect = &garbageCollect
		}
		if !retention {
			retentionPolicy := cdiv1.DataImportCronRetainNone
			cron.Spec.RetentionPolicy = &retentionPolicy
		}
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify cronjob was created")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), controller.GetCronJobName(cron), metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false
			}
			Expect(err).ToNot(HaveOccurred())
			return true
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

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
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}

		var lastImportDv, currentImportDv string
		for i := 0; i < repeat; i++ {
			By(fmt.Sprintf("Iter #%d", i))
			if i > 0 {
				if createErrorDv {
					By("Set desired digest to nonexisting one")
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					cron.Annotations[controller.AnnSourceDesiredDigest] = "sha256:12345678900987654321"
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Update(context.TODO(), cron, metav1.UpdateOptions{})
					Expect(err).ToNot(HaveOccurred())

					By("Wait for CurrentImports update")
					Eventually(func() bool {
						cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
						currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
						Expect(currentImportDv).ToNot(BeEmpty())
						return currentImportDv != lastImportDv
					}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
					lastImportDv = currentImportDv
				} else {
					By("Reset desired digest")
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					cron.Annotations[controller.AnnSourceDesiredDigest] = ""
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Update(context.TODO(), cron, metav1.UpdateOptions{})
					Expect(err).ToNot(HaveOccurred())

					By("Delete last import DV")
					err = f.CdiClient.CdiV1beta1().DataVolumes(ns).Delete(context.TODO(), currentImportDv, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
					lastImportDv = ""

					By("Wait for non-empty desired digest")
					Eventually(func() bool {
						cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())
						return cron.Annotations[controller.AnnSourceDesiredDigest] != ""
					}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
				}
			}

			waitForConditions(corev1.ConditionFalse, corev1.ConditionTrue)
			By("Verify CurrentImports update")
			currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
			Expect(currentImportDv).ToNot(BeEmpty())
			Expect(currentImportDv).ToNot(Equal(lastImportDv))
			lastImportDv = currentImportDv

			By(fmt.Sprintf("Verify pvc was created %s", currentImportDv))
			currentPvc, err := utils.WaitForPVC(f.K8sClient, ns, currentImportDv)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for import completion")
			err = utils.WaitForDataVolumePhase(f.CdiClient, ns, cdiv1.Succeeded, currentImportDv)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

			By("Verify datasource was updated")
			var dataSource *cdiv1.DataSource
			Eventually(func() bool {
				dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				readyCond := controller.FindDataSourceConditionByType(dataSource, cdiv1.DataSourceReady)
				return readyCond != nil && readyCond.Status == corev1.ConditionTrue &&
					dataSource.Spec.Source.PVC != nil && dataSource.Spec.Source.PVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Verify cron was updated")
			Expect(cron.Status.LastImportedPVC).ToNot(BeNil())
			Expect(cron.Status.LastImportedPVC.Name).To(Equal(currentImportDv))

			By("Update DataSource pvc with dummy name")
			dataSource.Spec.Source.PVC.Name = "dummy"
			dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Update(context.TODO(), dataSource, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Verify DataSource pvc name was reconciled")
			Eventually(func() bool {
				dataSource, err = f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				Expect(err).To(BeNil())
				return dataSource.Spec.Source.PVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Delete DataSource")
			err = f.CdiClient.CdiV1beta1().DataSources(ns).Delete(context.TODO(), dataSourceName, metav1.DeleteOptions{})
			Expect(err).To(BeNil())
			By("Verify DataSource was re-created")
			Eventually(func() bool {
				ds, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				return err == nil && ds.UID != dataSource.UID
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Delete last imported PVC")
			err = f.DeletePVC(currentPvc)
			Expect(err).To(BeNil())
			By("Verify last imported PVC was re-created")
			Eventually(func() bool {
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), currentPvc.Name, metav1.GetOptions{})
				return err == nil && pvc.UID != currentPvc.UID
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Wait for import completion")
			err = utils.WaitForDataVolumePhase(f.CdiClient, ns, cdiv1.Succeeded, currentImportDv)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")
		}
		By("Check garbage collection")
		Eventually(func() bool {
			dvList, err := f.CdiClient.CdiV1beta1().DataVolumes(ns).List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			if !garbageCollection {
				return len(dvList.Items) == 2
			}
			return len(dvList.Items) == int(importsToKeep)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

		lastImportedPVC := cron.Status.LastImportedPVC

		By("Delete cron")
		err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Delete(context.TODO(), cronName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		if retention {
			By("Verify DataSource retention")
			_, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
			Expect(err).To(BeNil())

			By("Verify last PVC retention")
			_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), lastImportedPVC.Name, metav1.GetOptions{})
			Expect(err).To(BeNil())
		} else {
			By("Verify DataSource deletion")
			Eventually(func() bool {
				_, err := f.CdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataSourceName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Verify PVCs deletion")
			Eventually(func() bool {
				pvcs, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(pvcs.Items) == 0
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}
	},
		table.Entry("[test_id:7403] succeed importing initial PVC from registry URL", false, true, false, 1),
		table.Entry("[test_id:7414] succeed importing PVC from registry URL on source digest update", false, true, false, 2),
		table.Entry("[test_id:7406] succeed garbage collecting old PVCs when importing new ones", true, false, false, 2),
		table.Entry("[test_id:8266] succeed deleting error DVs when importing new ones", true, false, true, 2),
	)

	It("[test_id:8033] should delete jobs on deletion", func() {
		reg, err := getDataVolumeSourceRegistry(f)
		Expect(err).To(BeNil())
		noSuchCM := "nosuch"
		reg.CertConfigMap = &noSuchCM
		cron = NewDataImportCron("cron-test", "5Gi", scheduleEveryMinute, dataSourceName, *reg)
		By("Create new DataImportCron")
		cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), cron, metav1.CreateOptions{})
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
			cronjob, _ := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
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
		err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Delete(context.TODO(), cronName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verify cronjob deleted")
		Eventually(func() bool {
			_, err := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

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
func NewDataImportCron(name, size, schedule, dataSource string, source cdiv1.DataVolumeSourceRegistry) *cdiv1.DataImportCron {
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
			ImportsToKeep:     &importsToKeep,
		},
	}
}

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
