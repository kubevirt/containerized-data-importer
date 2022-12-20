package tests

import (
	"context"
	"crypto/md5"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	samplePopulatorName = "sample-populator"
	populatorGroupName  = "cdi.sample.populator"
	populatorAPIVersion = "v1alpha1"
	populatorKind       = "SamplePopulator"
	populatorResource   = "samplepopulators"
	localSCName         = "local"
)

var _ = Describe("External population tests", func() {
	f := framework.NewFramework("population-func-test")

	var (
		fileName          string
		expectedContent   string
		samplePopulatorCR *unstructured.Unstructured
	)

	sampleGVR := schema.GroupVersionResource{Group: populatorGroupName, Version: populatorAPIVersion, Resource: populatorResource}
	apiGroup := populatorGroupName
	dataSourceRef := &corev1.TypedLocalObjectReference{
		APIGroup: &apiGroup,
		Kind:     populatorKind,
		Name:     samplePopulatorName,
	}

	// If the AnyVolumeDataSource feature gate is disabled, Kubernetes drops the contents of the dataSourceRef field.
	// We can then determine if the feature is enabled or not by checking that field after creating a PVC.
	isAnyVolumeDataSourceEnabled := func() bool {
		pvc := utils.NewPVCDefinition("test", "10Mi", nil, nil)
		pvc.Spec.DataSourceRef = dataSourceRef
		pvc, err := f.CreatePVCFromDefinition(pvc)
		Expect(err).ToNot(HaveOccurred())
		enabled := pvc.Spec.DataSourceRef != nil
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		return enabled
	}

	deploySamplePopulator := func() error {
		By("Creating Sample Populator CR")
		fileName = fmt.Sprintf("example-%s.txt", f.Namespace.Name)
		expectedContent = fmt.Sprintf("Hello from namespace %s", f.Namespace.Name)
		samplePopulatorCR = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       populatorKind,
				"apiVersion": populatorGroupName + "/" + populatorAPIVersion,
				"metadata": map[string]interface{}{
					"name":      samplePopulatorName,
					"namespace": f.Namespace.Name,
				},
				"spec": map[string]interface{}{
					"fileName":     fileName,
					"fileContents": expectedContent,
				},
			},
		}

		_, err := f.DynamicClient.Resource(sampleGVR).Namespace(f.Namespace.Name).Create(
			context.TODO(), samplePopulatorCR, metav1.CreateOptions{})
		return err
	}

	BeforeEach(func() {
		err := deploySamplePopulator()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := f.DynamicClient.Resource(sampleGVR).Namespace(samplePopulatorCR.GetNamespace()).Delete(context.TODO(), samplePopulatorCR.GetName(), metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("Should provision storage with any volume data source", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("No CSI drivers available - Population not supported")
		}
		if !isAnyVolumeDataSourceEnabled() {
			Skip("No AnyVolumeDataSource feature gate")
		}

		By(fmt.Sprintf("Creating new datavolume %s", dataVolumeName))
		dataVolume := utils.NewDataVolumeWithExternalPopulation(dataVolumeName, "100Mi", f.CsiCloneSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock), dataSourceRef)
		controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, corev1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying PVC's content")
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(dvc.ExternalPopulationSucceeded))
		expectetHash := []byte(expectedContent)
		expectedHashString := fmt.Sprintf("%x", md5.Sum(expectetHash))
		md5, err := f.GetMD5(f.Namespace, pvc, utils.DefaultPvcMountPath, int64(len(expectedContent)))
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(expectedHashString))

		By("Delete verifier pod")
		err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Cleaning up")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			return k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should not populate PVC when AnyVolumeDataSource is disabled", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("No CSI drivers available - Population not supported")
		}
		if isAnyVolumeDataSourceEnabled() {
			Skip("AnyVolumeDataSource is enabled - Population will succeed")
		}

		By(fmt.Sprintf("Creating new datavolume %s", dataVolumeName))
		dataVolume := utils.NewDataVolumeWithExternalPopulation(dataVolumeName, "100Mi", f.CsiCloneSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock), dataSourceRef)
		controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)
		// We check the expected event
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(dvc.NoAnyVolumeDataSource))

		By("Cleaning up")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			return k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should not populate PVC when CSI drivers are not available", func() {
		By("Checking if local storage class is available")
		sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), localSCName, metav1.GetOptions{})
		if err != nil {
			Skip("No local storage class to run without CSI drivers, cannot run test")
		}

		By(fmt.Sprintf("Creating new datavolume %s", dataVolumeName))
		dataVolume := utils.NewDataVolumeWithExternalPopulation(dataVolumeName, "100Mi", sc.Name, corev1.PersistentVolumeMode(corev1.PersistentVolumeFilesystem), dataSourceRef)
		controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)
		// We check the expected event
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(dvc.NoCSIDriverForExternalPopulation))

		By("Cleaning up")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			return k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})
})
