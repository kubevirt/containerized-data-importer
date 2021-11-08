package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	dataImportCronTimeout = 90 * time.Second
)

var _ = Describe("DataImportCron", func() {
	const (
		scheduleEveryMinute = "* * * * *"
		scheduleOnceAYear   = "0 0 1 1 *"
	)
	var (
		f                   = framework.NewFramework(namespacePrefix)
		registryPullNode    = cdiv1.RegistryPullNode
		trustedRegistryURL  = func() string { return fmt.Sprintf(utils.TrustedRegistryURL, f.DockerPrefix, f.DockerTag) }
		externalRegistryURL = "docker://quay.io/kubevirt/fedora-cloud-registry-disk-demo:latest"
		cron                *cdiv1.DataImportCron
		err                 error
	)

	AfterEach(func() {
		if cron != nil {
			By("Delete cron")
			err = DeleteDataImportCron(f.CdiClient, cron.Namespace, cron.Name)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	table.DescribeTable("should", func(schedule string, repeat int) {
		var url string

		if utils.IsOpenshift(f.K8sClient) {
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

		cron = NewDataImportCron("cron-test", "5Gi", schedule, "datasource-test", cdiv1.DataVolumeSourceRegistry{URL: &url, PullMethod: &registryPullNode})
		By(fmt.Sprintf("Create new DataImportCron %s", url))
		cron, err = CreateDataImportCronFromDefinition(f.CdiClient, f.Namespace.Name, cron)
		Expect(err).ToNot(HaveOccurred())

		if schedule == scheduleEveryMinute {
			var cronjob *v1beta1.CronJob
			By("Verify cronjob was created")
			Eventually(func() bool {
				cronjob, err = f.K8sClient.BatchV1beta1().CronJobs(f.CdiInstallNs).Get(context.TODO(), controller.GetCronJobName(cron), metav1.GetOptions{})
				return err == nil && len(cronjob.Status.Active) > 0
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Find pod of job %s", cronjob.Status.Active[0].Name))
			Eventually(func() bool {
				_, err = utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cronjob.Status.Active[0].Name, "")
				return err == nil
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}

		var lastImportDv, currentImportDv string
		for i := 0; i < repeat; i++ {
			if repeat > 1 {
				// Emulate source update
				digest := []string{"sha256:0001112223334444", "sha256:0123456789abcdef", "sha256:aaabbbcccdddeeee"}[i]
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
				if cron.Status.CurrentImports == nil {
					return false
				}
				currentImportDv = cron.Status.CurrentImports[0].DataVolumeName
				return currentImportDv != "" && currentImportDv != lastImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			lastImportDv = currentImportDv

			By(fmt.Sprintf("Verify pvc was created %s", currentImportDv))
			_, err = utils.WaitForPVC(f.K8sClient, cron.Namespace, currentImportDv)
			Expect(err).ToNot(HaveOccurred())

			By("Wait for import completion")
			err = utils.WaitForDataVolumePhase(f.CdiClient, cron.Namespace, cdiv1.Succeeded, currentImportDv)
			Expect(err).ToNot(HaveOccurred(), "Datavolume not in phase succeeded in time")

			By("Verify datasource was updated")
			Eventually(func() bool {
				datasource, err := f.CdiClient.CdiV1beta1().DataSources(f.Namespace.Name).Get(context.TODO(), cron.Spec.ManagedDataSource, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return datasource.Spec.Source.PVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

			By("Verify cron LastImportedPVC updated")
			Eventually(func() bool {
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(cron.Status.LastImportedPVC).ToNot(Equal(nil))
				return cron.Status.LastImportedPVC.Name == currentImportDv
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}
		if repeat > 2 {
			Eventually(func() bool {
				dvList, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(dvList.Items) == 2
			}, dataImportCronTimeout, pollingInterval).Should(BeTrue())
		}
	},
		table.Entry("[test_id:7403] Should successfully import PVC from registry URL as scheduled", scheduleEveryMinute, 1),
		table.Entry("[test_id:7414] Should successfully import PVC from registry URL on source digest update", scheduleOnceAYear, 2),
		table.Entry("[test_id:7406] Should successfully garbage collect old PVCs when importing new ones", scheduleOnceAYear, 3),
	)
})

// NewDataImportCron initializes a DataImportCron struct
func NewDataImportCron(name, size, schedule, dataSource string, source cdiv1.DataVolumeSourceRegistry) *cdiv1.DataImportCron {
	garbageCollect := cdiv1.DataImportCronGarbageCollectOutdated
	var importsToKeep int32 = 2

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
		},
	}
}

// CreateDataImportCronFromDefinition is used by tests to create a testable DataImportCron
func CreateDataImportCronFromDefinition(clientSet *cdiclientset.Clientset, namespace string, def *cdiv1.DataImportCron) (*cdiv1.DataImportCron, error) {
	var dataImportCron *cdiv1.DataImportCron
	err := wait.PollImmediate(pollingInterval, dataImportCronTimeout, func() (bool, error) {
		var err error
		dataImportCron, err = clientSet.CdiV1beta1().DataImportCrons(namespace).Create(context.TODO(), def, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return dataImportCron, nil
}

// DeleteDataImportCron deletes the DataImportCron with the given name
func DeleteDataImportCron(clientSet *cdiclientset.Clientset, namespace, name string) error {
	return wait.PollImmediate(pollingInterval, dataImportCronTimeout, func() (bool, error) {
		err := clientSet.CdiV1beta1().DataImportCrons(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}
