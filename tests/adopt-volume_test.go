package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("PVC adoption tests", func() {
	f := framework.NewFramework("adoption-test")

	Context("with available PVC", func() {
		const (
			dvName = "testdv"
			dvSize = "1Gi"
		)

		var (
			storageClass *string
			volumeMode   *corev1.PersistentVolumeMode
		)

		importDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeWithHTTPImport(dvName, dvSize, fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		}

		uploadDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForUpload(dvName, dvSize)
		}

		cloneDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForImageCloning(dvName, dvSize, "default", "foo", storageClass, volumeMode)
		}

		snapshotCloneDef := func() *cdiv1.DataVolume {
			return utils.NewDataVolumeForSnapshotCloning(dvName, dvSize, "default", "foo", storageClass, volumeMode)
		}

		populatorDef := func() *cdiv1.DataVolume {
			dataSourceRef := &corev1.TypedObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: "doesnotmatter",
			}
			return utils.NewDataVolumeWithExternalPopulation(dvName, dvSize, *storageClass, *volumeMode, nil, dataSourceRef)
		}

		BeforeEach(func() {
			By("Creating source DV")
			// source here shouldn't matter
			dvDef := importDef()
			dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for source DV to succeed")
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())

			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			storageClass = pvc.Spec.StorageClassName
			volumeMode = pvc.Spec.VolumeMode

			By("Retaining PVC")
			Eventually(func() error {
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if len(pvc.OwnerReferences) > 0 {
					pvc.OwnerReferences = nil
					_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
					if err != nil {
						return err
					}
				}
				return nil
			}, timeout, pollingInterval).Should(Succeed())

			By("Deleting source DV")
			err = utils.DeleteDataVolume(f.CdiClient, dv.Namespace, dv.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("with annotation on PVC", Serial, func() {
			var disabledFeatureGate bool

			BeforeEach(func() {
				By("Disabling featuregate")
				alreadyEnabled, err := utils.DisableFeatureGate(f.CrClient, featuregates.DataVolumeClaimAdoption)
				Expect(err).ToNot(HaveOccurred())
				disabledFeatureGate = *alreadyEnabled
			})

			AfterEach(func() {
				if !disabledFeatureGate {
					return
				}
				By("Enabling featuregate")
				_, err := utils.EnableFeatureGate(f.CrClient, featuregates.DataVolumeClaimAdoption)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("it should get adopted", func(defFunc func() *cdiv1.DataVolume) {
				var pvcUID string
				By("Getting PVC UID")
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcUID = string(pvc.UID)

				By("Creating target DV")
				dvDef := defFunc()
				controller.AddAnnotation(dvDef, controller.AnnAllowClaimAdoption, "true")
				dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for target DV to succeed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 300*time.Second)
				Expect(err).ToNot(HaveOccurred())

				pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(pvc.UID)).To(Equal(pvcUID))
			},
				Entry("with import source", importDef),
				Entry("with upload source", uploadDef),
				Entry("with clone source", cloneDef),
				Entry("with snapshot clone source", snapshotCloneDef),
				Entry("with populator source", populatorDef),
			)
		})

		Context("with featuregate enabled", Serial, func() {
			var setFeatureGate bool

			BeforeEach(func() {
				By("Enabling featuregate")
				alreadyEnabled, err := utils.EnableFeatureGate(f.CrClient, featuregates.DataVolumeClaimAdoption)
				Expect(err).ToNot(HaveOccurred())
				setFeatureGate = !*alreadyEnabled
			})

			AfterEach(func() {
				if !setFeatureGate {
					return
				}
				By("Disabling featuregate")
				_, err := utils.DisableFeatureGate(f.CrClient, featuregates.DataVolumeClaimAdoption)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("it should get adopted", func(defFunc func() *cdiv1.DataVolume) {
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				pvcUID := string(pvc.UID)

				By("Creating target DV")
				dvDef := defFunc()
				dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for target DV to succeed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 300*time.Second)
				Expect(err).ToNot(HaveOccurred())

				pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(string(pvc.UID)).To(Equal(pvcUID))
			},
				Entry("with import source", importDef),
				Entry("with upload source", uploadDef),
				Entry("with clone source", cloneDef),
				Entry("with snapshot clone source", snapshotCloneDef),
				Entry("with populator source", populatorDef),
			)

			It("should not be adopted if annotation says not to", func() {
				By("Creating target DV")
				dvDef := importDef()
				controller.AddAnnotation(dvDef, controller.AnnAllowClaimAdoption, "false")
				_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dvDef)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("already exists"))
			})
		})
	})
})
