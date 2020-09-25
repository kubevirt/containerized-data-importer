package tests_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Alpha API tests", func() {
	f := framework.NewFramework("alpha-api-test", framework.Config{})

	Context("with v1alpha1 api", func() {

		It("[test_id:4944]should get CDI config", func() {
			config, err := f.CdiClient.CdiV1alpha1().CDIConfigs().Get(context.TODO(), "config", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(config).ToNot(BeNil())
		})

		It("[test_id:4945]should", func() {
			By("create a upload DataVolume")
			out, err := tests.RunKubectlCommand(f, "create", "-f", "manifests/dvAlphaUpload.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for DataVolume to be ready")
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.UploadReady, "upload")

			By("create a upload token")
			out, err = tests.RunKubectlCommand(f, "create", "-f", "manifests/tokenAlpha.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:4946]should clone across namespace with alpha", func() {
			By("create source PVC")
			pvcDef := utils.NewPVCDefinition("source-pvc", "1G", nil, nil)
			source, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(context.TODO(), pvcDef, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("create target namespace")
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			targetDef := createCloneDataVolume("target-dv", source)
			target, err := f.CdiClient.CdiV1alpha1().DataVolumes(targetNs.Name).Create(context.TODO(), targetDef, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("waiting for DataVolume to be complete")
			err = utils.WaitForDataVolumePhase(f.CdiClient, target.Namespace, cdiv1.Succeeded, target.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:4947]should not", func() {
			By("create a upload DataVolume")
			out, err := tests.RunKubectlCommand(f, "create", "-f", "manifests/dvAlphaMissingAccessModes.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(out).Should(ContainSubstring("at least 1 access mode is required"))
			Expect(err).To(HaveOccurred())
		})

		Context("with deletion blocked", func() {
			var originalStrategy *cdiv1.CDIUninstallStrategy

			BeforeEach(func() {
				strategy := cdiv1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist
				originalStrategy = updateUninstallStrategy(f.CdiClient, &strategy)
			})

			AfterEach(func() {
				updateUninstallStrategy(f.CdiClient, originalStrategy)
			})

			It("[test_id:4948]should block cdi delete", func() {
				By("create a upload DataVolume")
				out, err := tests.RunKubectlCommand(f, "create", "-f", "manifests/dvAlphaUpload.yaml", "-n", f.Namespace.Name)
				fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
				Expect(err).ToNot(HaveOccurred())

				By("waiting for DataVolume to be ready")
				err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.UploadReady, "upload")

				// tests listing alpha api
				alphaCDIs, err := f.CdiClient.CdiV1alpha1().CDIs().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(alphaCDIs.Items).Should(HaveLen(1))

				// will invoke webhook for alpha
				err = f.CdiClient.CdiV1alpha1().CDIs().Delete(context.TODO(), alphaCDIs.Items[0].Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("there are still DataVolumes present"))
			})

		})
	})
})

func createCloneDataVolume(name string, source *corev1.PersistentVolumeClaim) *cdiv1alpha1.DataVolume {
	dv := &cdiv1alpha1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: cdiv1alpha1.DataVolumeSpec{
			Source: cdiv1alpha1.DataVolumeSource{
				PVC: &cdiv1alpha1.DataVolumeSourcePVC{
					Namespace: source.Namespace,
					Name:      source.Name,
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes:      source.Spec.AccessModes,
				Resources:        source.Spec.Resources,
				VolumeMode:       source.Spec.VolumeMode,
				StorageClassName: source.Spec.StorageClassName,
			},
		},
	}

	return dv
}
