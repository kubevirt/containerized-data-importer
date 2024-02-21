package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
)

// NewDataImportCron initializes a DataImportCron struct
func NewDataImportCron(name, size, schedule, dataSource string, importsToKeep int32, source cdiv1.DataVolumeSourceRegistry) *cdiv1.DataImportCron {
	return &cdiv1.DataImportCron{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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

// NewDataImportCronWithStorageSpec initializes a DataImportCron struct with storage defaults-inferring API
func NewDataImportCronWithStorageSpec(name, size, schedule, dataSource string, importsToKeep int32, source cdiv1.DataVolumeSourceRegistry) *cdiv1.DataImportCron {
	return &cdiv1.DataImportCron{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: cdiv1.DataImportCronSpec{
			Template: cdiv1.DataVolume{
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{
						Registry: &source,
					},
					Storage: &cdiv1.StorageSpec{
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

// VerifyCronJobCleanup verifies the DataImportCron cronjob was deleted
func VerifyCronJobCleanup(k8sClient *kubernetes.Clientset, cronJobNamespace, cronNamespace, cronName string) {
	gomega.Eventually(func() *batchv1.CronJob {
		cronJobs, err := k8sClient.BatchV1().CronJobs(cronJobNamespace).List(context.TODO(), metav1.ListOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		for _, cronJob := range cronJobs.Items {
			if cronJob.Labels[common.DataImportCronLabel] == controller.GetCronJobLabelValue(cronNamespace, cronName) {
				ginkgo.By(fmt.Sprintf("CronJob not deleted yet %s", cronJob.Name))
				return &cronJob
			}
		}
		return nil
	}, 3*time.Minute, 2*time.Second).Should(gomega.BeNil(), "Timeout waiting for CronJob deletion")
}
