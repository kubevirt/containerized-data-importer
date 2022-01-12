package tests_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = FDescribe("DataSource", func() {
	const dsName = "ds-test"
	const pvcName = "pvc-test"
	f := framework.NewFramework("datasource-func-test")

	newDataSource := func() *cdiv1.DataSource {
		return &cdiv1.DataSource{
			TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dsName,
				Namespace: f.Namespace.Name,
			},
		}
	}

	waitForReadyCondition := func(status corev1.ConditionStatus, reason string) (ds *cdiv1.DataSource) {
		By(fmt.Sprintf("wait for ready condition: %s, %s", status, reason))
		Eventually(func() bool {
			var err error
			ds, err = f.CdiClient.CdiV1beta1().DataSources(f.Namespace.Name).Get(context.TODO(), dsName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			cond := controller.FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
			if cond != nil {
				By(fmt.Sprintf("condition state: %s, %s", cond.Status, cond.Reason))
			}
			return cond != nil && cond.Status == status && cond.Reason == reason
		}, timeout, pollingInterval).Should(BeTrue())
		return
	}

	It("[test_id:8041]status conditions should be updated on pvc create/update/delete", func() {
		By("creating datasource")
		ds := newDataSource()
		ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(corev1.ConditionFalse, "NoPvc")

		dv := utils.NewDataVolumeWithHTTPImport(pvcName, "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: f.Namespace.Name, Name: pvcName}
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(corev1.ConditionFalse, "NotFound")

		By("creating datavolume")
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())
		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)
		ds = waitForReadyCondition(corev1.ConditionTrue, "Ready")

		err = utils.DeleteDataVolume(f.CdiClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(corev1.ConditionFalse, "NotFound")

		ds.Spec.Source.PVC = nil
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(corev1.ConditionFalse, "NoPvc")
	})
})
