package tests

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	populatorGroupName  = "cdi.sample.populator"
	populatorAPIVersion = "v1alpha1"
	populatorKind       = "SamplePopulator"
	populatorResource   = "samplepopulators"
)

var _ = Describe("External population tests", func() {
	f := framework.NewFramework("population-func-test")

	BeforeEach(func() {})

	AfterEach(func() {})

	FIt("should provision storage with any volume data source", func() {
		By("Creating Sample Populator CR")
		samplePopulatorName := "sample-populator"
		fileName := fmt.Sprintf("example-%s.txt", f.Namespace.Name)
		expectedContent := fmt.Sprintf("Hello from namespace %s", f.Namespace.Name)
		sampleGVR := schema.GroupVersionResource{Group: populatorGroupName, Version: populatorAPIVersion, Resource: populatorResource}
		samplePopulatorCR := &unstructured.Unstructured{
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

		_, err := f.DynamicClient.Resource(sampleGVR).Namespace(f.Namespace.Name).Create(context.TODO(), samplePopulatorCR, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			err = f.DynamicClient.Resource(sampleGVR).Namespace(samplePopulatorCR.GetNamespace()).Delete(context.TODO(), samplePopulatorCR.GetName(), metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		}()

		storageSpec := &cdiv1.StorageSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Mi"),
				},
			},
		}

		dataVolume := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:        dataVolumeName,
				Annotations: map[string]string{},
			},
			Spec: cdiv1.DataVolumeSpec{
				Storage: storageSpec,
			},
		}

		apiGroup := populatorGroupName
		dataVolume.Spec.Storage.DataSourceRef = &corev1.TypedLocalObjectReference{
			APIGroup: &apiGroup,
			Kind:     populatorKind,
			Name:     samplePopulatorName,
		}

		controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")

		By(fmt.Sprintf("creating new datavolume %s", dataVolume.Name))
		dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		// verify PVC was created
		By("verifying pvc was created")
		pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, pvc.Namespace, corev1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying PVC's content")
		expectetHash := md5.Sum([]byte(expectedContent))
		expectedHashString := hex.EncodeToString(expectetHash[:])
		f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(controller.ExternalPopulationSucceeded))
		md5, err := f.GetMD5(f.Namespace, pvc, fileName, utils.MD5PrefixSize)
		Expect(err).ToNot(HaveOccurred())
		Expect(md5).To(Equal(expectedHashString))

		By("Cleaning up")
		err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			return k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})
})
