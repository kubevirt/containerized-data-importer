package tests_test

import (
	"context"
	"fmt"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("DataSource", func() {
	const (
		ds1Name   = "ds1"
		ds2Name   = "ds2"
		pvc1Name  = "pvc1"
		pvc2Name  = "pvc2"
		snap1Name = "snap1"
		snap2Name = "snap2"
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
		}, timeout, pollingInterval).Should(BeTrue())
		return ds
	}

	testURL := func() string { return fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs) }
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
		By("Create DataSource with no source PVC")
		ds := newDataSource(ds1Name)
		ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NoSource")

		By("Update DataSource source PVC to nonexisting one")
		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: f.Namespace.Name, Name: pvc1Name}
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

		By("Create clone DV with SourceRef pointing the DataSource")
		dv := utils.NewDataVolumeWithSourceRef("clone-dv", "1Gi", ds.Namespace, ds.Name)
		dv.Annotations[cc.AnnImmediateBinding] = "true"
		Expect(dv).ToNot(BeNil())
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("Verify DV conditions")
		utils.WaitForConditions(f, dv.Name, dv.Namespace, time.Minute, pollingInterval,
			&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeBound, Status: corev1.ConditionUnknown, Message: "The source pvc pvc1 doesn't exist", Reason: dvc.CloneWithoutSource},
			&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeReady, Status: corev1.ConditionFalse, Reason: dvc.CloneWithoutSource},
			&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeRunning, Status: corev1.ConditionFalse})
		f.ExpectEvent(dv.Namespace).Should(ContainSubstring(dvc.CloneWithoutSource))

		By("Create import DV so the missing DataSource source PVC will be ready")
		createDv(pvc1Name, testURL())
		ds = waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")

		By("Wait for the clone DV success")
		err = utils.WaitForDataVolumePhase(f, dv.Namespace, cdiv1.Succeeded, dv.Name)
		Expect(err).ToNot(HaveOccurred())

		deleteDvPvc(f, pvc1Name)
		ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

		ds.Spec.Source.PVC = nil
		ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		_ = waitForReadyCondition(ds, corev1.ConditionFalse, "NoSource")
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
		ds.Spec.Source.PVC = &cdiv1.DataVolumeSourcePVC{Namespace: "", Name: pvcName}
		_, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	It("[test_id:8067]status conditions should be updated when several DataSources refer the same pvc", func() {
		createDv(pvc1Name, testURL())
		ds1 := createDs(ds1Name, pvc1Name)
		ds2 := createDs(ds2Name, pvc1Name)

		ds1 = waitForReadyCondition(ds1, corev1.ConditionTrue, "Ready")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionTrue, "Ready")

		deleteDvPvc(f, pvc1Name)
		ds1 = waitForReadyCondition(ds1, corev1.ConditionFalse, "NotFound")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionFalse, "NotFound")

		createDv(pvc2Name, testURL()+"bad")
		updateDsPvc(ds1, pvc2Name)
		updateDsPvc(ds2, pvc2Name)
		ds1 = waitForReadyCondition(ds1, corev1.ConditionFalse, "ImportInProgress")
		ds2 = waitForReadyCondition(ds2, corev1.ConditionFalse, "ImportInProgress")

		deleteDvPvc(f, pvc2Name)
		_ = waitForReadyCondition(ds1, corev1.ConditionFalse, "NotFound")
		_ = waitForReadyCondition(ds2, corev1.ConditionFalse, "NotFound")
	})

	It("status conditions timestamp should be updated when DataSource referred pvc is updated, although condition status does not change", func() {
		createDv(pvc1Name, testURL())
		ds := createDs(ds1Name, pvc1Name)
		ds = waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")
		cond := controller.FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
		Expect(cond).ToNot(BeNil())
		ts := cond.LastTransitionTime

		createDv(pvc2Name, testURL())
		err := utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, pvc2Name)
		Expect(err).ToNot(HaveOccurred())
		updateDsPvc(ds, pvc2Name)

		Eventually(func() metav1.Time {
			ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Get(context.TODO(), ds.Name, metav1.GetOptions{})
			Expect(ds.Spec.Source.PVC.Name).To(Equal(pvc2Name))
			cond = controller.FindDataSourceConditionByType(ds, cdiv1.DataSourceReady)
			Expect(cond).ToNot(BeNil())
			Expect(cond.Status).To(Equal(corev1.ConditionTrue))
			return cond.LastTransitionTime
		}, 60*time.Second, pollingInterval).ShouldNot(Equal(ts))
	})

	Context("snapshot source", func() {
		createSnap := func(name string) *snapshotv1.VolumeSnapshot {
			pvcDef := utils.NewPVCDefinition("snap-source-pvc", "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(context.TODO(), pvcDef, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			snapClass := f.GetSnapshotClass()
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: pvc.Namespace,
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvc.Name,
					},
					VolumeSnapshotClassName: &snapClass.Name,
				},
			}
			err = f.CrClient.Create(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())

			return snapshot
		}

		createSnapDs := func(dsName, snapName string) *cdiv1.DataSource {
			By(fmt.Sprintf("creating DataSource %s -> %s", dsName, snapName))
			ds := newDataSource(dsName)
			ds.Spec.Source.Snapshot = &cdiv1.DataVolumeSourceSnapshot{Namespace: f.Namespace.Name, Name: snapName}
			ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			return waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")
		}

		BeforeEach(func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Clone from volumesnapshot does not work without snapshot capable storage")
			}
		})

		It("[test_id:9762] status conditions should be updated on snapshot create/update/delete", func() {
			By("Create DataSource with no source")
			ds := newDataSource(ds1Name)
			ds, err := f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NoSource")

			By("Update DataSource source snapshot to nonexisting one")
			ds.Spec.Source.Snapshot = &cdiv1.DataVolumeSourceSnapshot{Namespace: f.Namespace.Name, Name: snap1Name}
			ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

			By("Create clone DV with SourceRef pointing the DataSource")
			dv := utils.NewDataVolumeWithSourceRef("clone-dv", "1Gi", ds.Namespace, ds.Name)
			dv.Annotations[cc.AnnImmediateBinding] = "true"
			Expect(dv).ToNot(BeNil())
			dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())

			By("Verify DV conditions")
			utils.WaitForConditions(f, dv.Name, dv.Namespace, time.Minute, pollingInterval,
				&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeBound, Status: corev1.ConditionUnknown, Message: "The source snapshot snap1 doesn't exist", Reason: dvc.CloneWithoutSource},
				&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeReady, Status: corev1.ConditionFalse, Reason: dvc.CloneWithoutSource},
				&cdiv1.DataVolumeCondition{Type: cdiv1.DataVolumeRunning, Status: corev1.ConditionFalse})
			f.ExpectEvent(dv.Namespace).Should(ContainSubstring(dvc.CloneWithoutSource))

			By("Create snapshot so the DataSource will be ready")
			snapshot := createSnap(snap1Name)
			ds = waitForReadyCondition(ds, corev1.ConditionTrue, "Ready")

			By("Wait for the clone DV success")
			err = utils.WaitForDataVolumePhase(f, dv.Namespace, cdiv1.Succeeded, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			err = f.CrClient.Delete(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())
			ds = waitForReadyCondition(ds, corev1.ConditionFalse, "NotFound")

			ds.Spec.Source.Snapshot = nil
			ds, err = f.CdiClient.CdiV1beta1().DataSources(ds.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			_ = waitForReadyCondition(ds, corev1.ConditionFalse, "NoSource")
		})

		It("[test_id:9763] status conditions should be updated when several DataSources refer the same snapshot", func() {
			snapshot := createSnap(snap1Name)
			ds1 := createSnapDs(ds1Name, snap1Name)
			ds2 := createSnapDs(ds2Name, snap1Name)

			ds1 = waitForReadyCondition(ds1, corev1.ConditionTrue, "Ready")
			ds2 = waitForReadyCondition(ds2, corev1.ConditionTrue, "Ready")

			err := f.CrClient.Delete(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())
			_ = waitForReadyCondition(ds1, corev1.ConditionFalse, "NotFound")
			_ = waitForReadyCondition(ds2, corev1.ConditionFalse, "NotFound")
		})
	})
})

func deleteDvPvc(f *framework.Framework, pvcName string) {
	utils.CleanupDvPvc(f.K8sClient, f.CdiClient, f.Namespace.Name, pvcName)
}
