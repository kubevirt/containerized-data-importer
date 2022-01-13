package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("DataSource", func() {
	const (
		ds1Name  = "ds1"
		ds2Name  = "ds2"
		pvc1Name = "pvc1"
		pvc2Name = "pvc2"
	)

	f := framework.NewFramework("datasource-func-test")

	newDataSource := func(dsName string) *cdiv1.DataSource {
		return &cdiv1.DataSource{
			TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dsName,
				Namespace: f.Namespace.Name,
			},
		}
	}

	waitForReadyCondition := func(ds *cdiv1.DataSource, status corev1.ConditionStatus, reason string) *cdiv1.DataSource {
		By(fmt.Sprintf("wait for DataSource %s ready condition: %s, %s", ds.Name, status, reason))
		Eventually(func() bool {
			var err error
			ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Get(context.TODO(), ds.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			cond := controller.FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
			if cond != nil {
				By(fmt.Sprintf("condition state: %s, %s", cond.Status, cond.Reason))
			}
			return cond != nil && cond.Status == status && cond.Reason == reason
		}, 60*time.Second, pollingInterval).Should(BeTrue())
		return ds
	}

	testUrl := func() string { return fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs) }
	createDv := func(pvcName, url string) {
		By(fmt.Sprintf("creating DataVolume %s %s", pvcName, url))
		dv := utils.NewDataVolumeWithHTTPImport(pvcName, "1Gi", url)
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())
		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)
	}

	It("[test_id:8041]status conditions should be updated on pvc create/update/delete", func() {
		By("creating datasource")
		ds := newDataSource(ds1Name)
		ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NoPvc")

		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: f.Namespace.Name, Name: pvc1Name}
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

		createDv(pvc1Name, testUrl())
		ds = waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")

		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, pvc1Name)
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

		ds.Spec.Source.PVC = nil
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NoPvc")
	})

	createDs := func(dsName, pvcName string) *cdiv1.DataSource {
		By(fmt.Sprintf("creating DataSource %s -> %s", dsName, pvcName))
		ds := newDataSource(dsName)
		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: f.Namespace.Name, Name: pvcName}
		ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		return waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")
	}

	updateDsPvc := func(ds *cdiv1.DataSource, pvcName string) {
		By(fmt.Sprintf("updating DataSource %s -> %s", ds.Name, pvcName))
		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: f.Namespace.Name, Name: pvcName}
		ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	It("[test_id:8067]status conditions should be updated when several DataSources refer the same pvc", func() {
		createDv(pvc1Name, testUrl())
		ds1 := createDs(ds1Name, pvc1Name)
		ds2 := createDs(ds2Name, pvc1Name)

		ds1 = waitForReadyCondition(ds1, corev1.ConditionTrue, "Ready")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionTrue, "Ready")

		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, pvc1Name)
		Expect(err).ToNot(HaveOccurred())
		ds1 = waitForReadyCondition(ds1, corev1.ConditionFalse, "NotFound")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionFalse, "NotFound")

		createDv(pvc2Name, testUrl()+"bad")
		updateDsPvc(ds1, pvc2Name)
		updateDsPvc(ds2, pvc2Name)
		ds1 = waitForReadyCondition(ds1, corev1.ConditionFalse, "ImportInProgress")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionFalse, "ImportInProgress")

		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, pvc2Name)
		Expect(err).ToNot(HaveOccurred())
		ds1 = waitForReadyCondition(ds1, corev1.ConditionFalse, "NotFound")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionFalse, "NotFound")
	})
})
