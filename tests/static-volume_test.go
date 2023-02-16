package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("checkStaticVolume tests", func() {
	f := framework.NewFramework("transfer-test")

	Context("with available PV", func() {
		const (
			dvName = "testdv"
			dvSize = "1Gi"
		)

		var (
			pvName         string
			pvStorageClass *string
			pvMode         *corev1.PersistentVolumeMode
		)

		importDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport(dvName, dvSize, fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		}

		uploadDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dvName, dvSize)
		}

		cloneDef := func() *cdiv1.DataVolume {
			// TODO change validation so this is not necessary
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			return utils.NewDataVolumeForImageCloning(dvName, dvSize, targetNs.Name, "foo", pvStorageClass, pvMode)
		}

		snapshotCloneDef := func() *cdiv1.DataVolume {
			// TODO change validation so this is not necessary
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			return utils.NewDataVolumeForSnapshotCloning(dvName, dvSize, targetNs.Name, "foo", pvStorageClass, pvMode)
		}

		BeforeEach(func() {
			By("Creating source DV")
			// source here shouldn't matter
			dvDef := importDef()
			controller.AddAnnotation(dvDef, controller.AnnDeleteAfterCompletion, "false")
			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for source DV to succeed")
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())

			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			pvName = pvc.Spec.VolumeName
			pvStorageClass = pvc.Spec.StorageClassName
			pvMode = pvc.Spec.VolumeMode

			By("Retaining PV")
			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Deleting source DV")
			err = utils.DeleteDataVolume(f.CdiClient, dv.Namespace, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Making PV available")
			Eventually(func() bool {
				pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(pv.Spec.ClaimRef.Namespace).To(Equal(dv.Namespace))
				Expect(pv.Spec.ClaimRef.Name).To(Equal(dv.Name))
				if pv.Status.Phase == corev1.VolumeAvailable {
					return true
				}
				pv.Spec.ClaimRef.ResourceVersion = ""
				pv.Spec.ClaimRef.UID = ""
				_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				return false
			}, timeout, pollingInterval).Should(BeTrue())
		})

		AfterEach(func() {
			if pvName == "" {
				return
			}
			err := f.K8sClient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvName, metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("should handle static allocated DataVolume", func(defFunc func() *cdiv1.DataVolume) {
			By("Creating target DV")
			dvDef := defFunc()
			controller.AddAnnotation(dvDef, controller.AnnCheckStaticVolume, "")
			controller.AddAnnotation(dvDef, controller.AnnDeleteAfterCompletion, "false")
			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for target DV to succeed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())

			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.VolumeName).To(Equal(pvName))
		},
			Entry("with import source", importDef),
			Entry("with upload source", uploadDef),
			Entry("with clone source", cloneDef),
			Entry("with snapshot clone source", snapshotCloneDef),
		)
	})
})
