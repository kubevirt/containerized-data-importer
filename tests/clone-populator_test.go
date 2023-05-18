package tests_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Clone Populator tests", func() {
	const (
		sourceName     = "test-source"
		targetName     = "test-target"
		dataSourceName = "test-datasource"
	)

	var (
		defaultSize = resource.MustParse("1Gi")
		biggerSize  = resource.MustParse("2Gi")
	)

	f := framework.NewFramework("clone-populator-test")

	createSource := func(sz resource.Quantity, vm corev1.PersistentVolumeMode) *corev1.PersistentVolumeClaim {
		dataVolume := utils.NewDataVolumeWithHTTPImport(sourceName, sz.String(), fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		dataVolume.Spec.PVC.VolumeMode = &vm
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
		Expect(utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 180*time.Second)).To(Succeed())
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return pvc
	}

	createDataSource := func() *cdiv1.VolumeCloneSource {
		vcs := &cdiv1.VolumeCloneSource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.Namespace.Name,
				Name:      dataSourceName,
			},
			Spec: cdiv1.VolumeCloneSourceSpec{
				Source: corev1.TypedLocalObjectReference{
					Kind: "PersistentVolumeClaim",
					Name: sourceName,
				},
			},
		}
		err := f.CrClient.Create(context.Background(), vcs)
		Expect(err).ToNot(HaveOccurred())
		return vcs
	}

	createTargetWithStrategy := func(sz resource.Quantity, vm corev1.PersistentVolumeMode, strategy string) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.Namespace.Name,
				Name:      targetName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				DataSourceRef: &corev1.TypedObjectReference{
					APIGroup: &cdiv1.SchemeGroupVersion.Group,
					Kind:     "VolumeCloneSource",
					Name:     dataSourceName,
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: sz,
					},
				},
			},
		}
		pvc.Spec.VolumeMode = &vm
		if strategy != "" {
			pvc.Annotations = map[string]string{
				"cdi.kubevirt.io/cloneType": strategy,
			}
		}
		err := f.CrClient.Create(context.Background(), pvc)
		Expect(err).ToNot(HaveOccurred())
		f.ForceSchedulingIfWaitForFirstConsumerPopulationPVC(pvc)
		result := &corev1.PersistentVolumeClaim{}
		err = f.CrClient.Get(context.Background(), client.ObjectKeyFromObject(pvc), result)
		Expect(err).ToNot(HaveOccurred())
		return result
	}

	createTarget := func(sz resource.Quantity, vm corev1.PersistentVolumeMode) *corev1.PersistentVolumeClaim {
		return createTargetWithStrategy(sz, vm, "")
	}

	waitSucceeded := func(target *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
		var result *corev1.PersistentVolumeClaim
		Eventually(func() string {
			pvc := &corev1.PersistentVolumeClaim{}
			err := f.CrClient.Get(context.Background(), client.ObjectKeyFromObject(target), pvc)
			Expect(err).ToNot(HaveOccurred())
			result = pvc
			return pvc.Annotations["cdi.kubevirt.io/clonePhase"]
		}, timeout, pollingInterval).Should(Equal("Succeeded"))
		Expect(result.Annotations["cdi.kubevirt.io/storage.populator.progress"]).To(Equal("100.0%"))
		return result
	}

	getHash := func(pvc *corev1.PersistentVolumeClaim, numBytes int64) string {
		path := utils.DefaultPvcMountPath
		if pvc.Spec.VolumeMode == nil || *pvc.Spec.VolumeMode == corev1.PersistentVolumeFilesystem {
			path = filepath.Join(path, "disk.img")
		}
		hash, err := f.GetMD5(f.Namespace, pvc, path, numBytes)
		Expect(err).ToNot(HaveOccurred())
		err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())
		_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, 270*time.Second)
		Expect(err).ToNot(HaveOccurred())
		return hash
	}

	BeforeEach(func() {
		if utils.DefaultStorageClassCsiDriver == nil {
			Skip("No CSI driver found")
		}
	})

	It("should should do filesystem to filesystem clone", func() {
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		createDataSource()
		target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do filesystem to filesystem clone, source created after target", func() {
		createDataSource()
		target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do filesystem to filesystem clone, dataSource created after target", func() {
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
		createDataSource()
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do filesystem to filesystem clone (bigger target)", func() {
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		createDataSource()
		target := createTarget(biggerSize, corev1.PersistentVolumeFilesystem)
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		targetSize := target.Status.Capacity[corev1.ResourceStorage]
		Expect(targetSize.Cmp(biggerSize)).To(BeNumerically(">=", 0))
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do block to filesystem clone", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		source := createSource(defaultSize, corev1.PersistentVolumeBlock)
		createDataSource()
		target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		targetSize := target.Status.Capacity[corev1.ResourceStorage]
		Expect(targetSize.Cmp(defaultSize)).To(BeNumerically(">", 0))
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do filesystem to block clone", func() {
		if !f.IsBlockVolumeStorageClassAvailable() {
			Skip("Storage Class for block volume is not available")
		}
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		createDataSource()
		target := createTarget(defaultSize, corev1.PersistentVolumeBlock)
		sourceHash := getHash(source, 100000)
		target = waitSucceeded(target)
		targetSize := target.Status.Capacity[corev1.ResourceStorage]
		Expect(targetSize.Cmp(defaultSize)).To(BeNumerically(">=", 0))
		targetHash := getHash(target, 100000)
		Expect(targetHash).To(Equal(sourceHash))
	})

	It("should should do csi clone if possible", func() {
		if !f.IsCSIVolumeCloneStorageClassAvailable() {
			Skip("CSI Clone does not work without a capable storage class")
		}
		source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
		createDataSource()
		target := createTargetWithStrategy(defaultSize, corev1.PersistentVolumeFilesystem, "csi-clone")
		sourceHash := getHash(source, 0)
		target = waitSucceeded(target)
		Expect(target.Annotations["cdi.kubevirt.io/cloneType"]).To(Equal("csi-clone"))
		targetHash := getHash(target, 0)
		Expect(targetHash).To(Equal(sourceHash))
	})
})
