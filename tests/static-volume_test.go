package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
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
			sourceMD5      string
		)

		importDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport(dvName, dvSize, fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		}

		uploadDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dvName, dvSize)
		}

		cloneDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForImageCloning(dvName, dvSize, "baz", "foo", pvStorageClass, pvMode)
		}

		snapshotCloneDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForSnapshotCloning(dvName, dvSize, "baz", "foo", pvStorageClass, pvMode)
		}

		populatorDef := func() *cdiv1.DataVolume {
			dataSourceRef := &corev1.TypedObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: "doesnotmatter",
			}
			return utils.NewDataVolumeWithExternalPopulation(dvName, dvSize, *pvStorageClass, *pvMode, nil, dataSourceRef)
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

			By("Getting source MD5")
			md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultImagePath, 0)
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			sourceMD5 = md5

			By("Retaining PV")
			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

			Eventually(func() error {
				_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
				// We shouldn't make the test fail if there's a conflict with the update request.
				// These errors are usually transient and should be fixed in subsequent retries.
				return err
			}, timeout, pollingInterval).Should(Succeed())

			By("Deleting source DV")
			err = utils.DeleteDataVolume(f.CdiClient, dv.Namespace, dv.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Making PV available")
			Eventually(func(g Gomega) bool {
				pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(pv.Spec.ClaimRef.Namespace).To(Equal(dv.Namespace))
				g.Expect(pv.Spec.ClaimRef.Name).To(Equal(dv.Name))
				if pv.Status.Phase == corev1.VolumeAvailable {
					return true
				}
				pv.Spec.ClaimRef.ResourceVersion = ""
				pv.Spec.ClaimRef.UID = ""
				_, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), pv, metav1.UpdateOptions{})
				// We shouldn't make the test fail if there's a conflict with the update request.
				// These errors are usually transient and should be fixed in subsequent retries.
				g.Expect(err).ToNot(HaveOccurred())
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

			pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pv.CreationTimestamp.Before(&pvc.CreationTimestamp)).To(BeTrue())

			By("Verify content")
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, sourceMD5, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		},
			Entry("with import source", importDef),
			Entry("with upload source", uploadDef),
			Entry("with clone source", cloneDef),
			Entry("with snapshot clone source", snapshotCloneDef),
			Entry("with populator source", populatorDef),
		)
	})
})
