package tests_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Clone Populator tests", func() {
	const (
		sourceName     = "test-source"
		targetName     = "test-target"
		dataSourceName = "test-datasource"

		tmpSourcePVCforSnapshot = "tmp-source-pvc-test-target"
		snapshotAPIName         = "snapshot.storage.k8s.io"
	)

	var (
		defaultSize = resource.MustParse("1Gi")
		biggerSize  = resource.MustParse("2Gi")
	)

	f := framework.NewFramework("clone-populator-test")

	BeforeEach(func() {
		if utils.DefaultStorageClassCsiDriver == nil {
			Skip("No CSI driver found")
		}
	})

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

	createVolumeSnapshotSource := func(size string, storageClassName *string, volumeMode corev1.PersistentVolumeMode) *snapshotv1.VolumeSnapshot {
		snapSourceDv := utils.NewDataVolumeWithHTTPImport(sourceName, size, fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		snapSourceDv.Spec.PVC.VolumeMode = &volumeMode
		snapSourceDv.Spec.PVC.StorageClassName = storageClassName
		By(fmt.Sprintf("Create new datavolume %s which will be the source of the volumesnapshot", snapSourceDv.Name))
		snapSourceDv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, snapSourceDv)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(snapSourceDv)
		By("Waiting for import to be completed")
		err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, snapSourceDv.Name)
		Expect(err).ToNot(HaveOccurred())
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(snapSourceDv.Namespace).Get(context.TODO(), snapSourceDv.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		snapClass := f.GetSnapshotClass()
		snapshot := utils.NewVolumeSnapshot(sourceName, f.Namespace.Name, pvc.Name, &snapClass.Name)
		err = f.CrClient.Create(context.TODO(), snapshot)
		Expect(err).ToNot(HaveOccurred())

		snapshot = utils.WaitSnapshotReady(f.CrClient, snapshot)
		By("Snapshot ready, no need to keep PVC around")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		return snapshot
	}

	snapshotAPIGroup := snapshotAPIName
	createSnapshotDataSource := func() *cdiv1.VolumeCloneSource {
		vcs := &cdiv1.VolumeCloneSource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.Namespace.Name,
				Name:      dataSourceName,
			},
			Spec: cdiv1.VolumeCloneSourceSpec{
				Source: corev1.TypedLocalObjectReference{
					APIGroup: &snapshotAPIGroup,
					Kind:     "VolumeSnapshot",
					Name:     sourceName,
				},
			},
		}
		err := f.CrClient.Create(context.Background(), vcs)
		Expect(err).ToNot(HaveOccurred())
		return vcs
	}

	createTargetWithStrategy := func(sz resource.Quantity, vm corev1.PersistentVolumeMode, strategy, scName string) *corev1.PersistentVolumeClaim {
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
				StorageClassName: &scName,
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
		return createTargetWithStrategy(sz, vm, "", utils.DefaultStorageClass.GetName())
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

	Context("Clone from PVC", func() {
		It("should do filesystem to filesystem clone", func() {
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			createDataSource()
			target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
			target = waitSucceeded(target)
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		})

		It("should do filesystem to filesystem clone, source created after target", func() {
			createDataSource()
			target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			target = waitSucceeded(target)
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		})

		It("should do filesystem to filesystem clone, dataSource created after target", func() {
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
			createDataSource()
			target = waitSucceeded(target)
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		})

		It("should do filesystem to filesystem clone (bigger target)", func() {
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			createDataSource()
			target := createTarget(biggerSize, corev1.PersistentVolumeFilesystem)
			target = waitSucceeded(target)
			targetSize := target.Status.Capacity[corev1.ResourceStorage]
			Expect(targetSize.Cmp(biggerSize)).To(BeNumerically(">=", 0))
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		})

		It("should do block to filesystem clone", func() {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			source := createSource(defaultSize, corev1.PersistentVolumeBlock)
			createDataSource()
			target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
			target = waitSucceeded(target)
			targetSize := target.Status.Capacity[corev1.ResourceStorage]
			Expect(targetSize.Cmp(defaultSize)).To(BeNumerically(">", 0))
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		})

		It("should do filesystem to block clone", func() {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			createDataSource()
			target := createTarget(defaultSize, corev1.PersistentVolumeBlock)
			target = waitSucceeded(target)
			targetSize := target.Status.Capacity[corev1.ResourceStorage]
			Expect(targetSize.Cmp(defaultSize)).To(BeNumerically(">=", 0))
			sourceHash := getHash(source, 100000)
			targetHash := getHash(target, 100000)
			Expect(targetHash).To(Equal(sourceHash))
		})

		DescribeTable("should clone explicit types requested by user", func(cloneType string, canDo func() bool) {
			if canDo != nil && !canDo() {
				Skip(fmt.Sprintf("Clone type %s does not work without a capable storage class", cloneType))
			}
			source := createSource(defaultSize, corev1.PersistentVolumeFilesystem)
			createDataSource()
			target := createTargetWithStrategy(defaultSize, corev1.PersistentVolumeFilesystem, cloneType, utils.DefaultStorageClass.GetName())
			target = waitSucceeded(target)
			Expect(target.Annotations["cdi.kubevirt.io/cloneType"]).To(Equal(cloneType))
			sourceHash := getHash(source, 0)
			targetHash := getHash(target, 0)
			Expect(targetHash).To(Equal(sourceHash))
		},
			Entry("should do csi clone if possible", "csi-clone", f.IsCSIVolumeCloneStorageClassAvailable),
			Entry("should do snapshot clone if possible", "snapshot", f.IsSnapshotStorageClassAvailable),
			Entry("should do host assisted clone", "copy", nil),
		)
	})

	Context("Clone from Snapshot", func() {
		var (
			snapshot = &snapshotv1.VolumeSnapshot{}
		)

		BeforeEach(func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Clone from volumesnapshot does not work without snapshot capable storage")
			}
		})

		AfterEach(func() {
			By(fmt.Sprintf("[AfterEach] Removing snapshot %s/%s", snapshot.Namespace, snapshot.Name))
			Eventually(func() bool {
				err := f.CrClient.Delete(context.TODO(), snapshot)
				return err != nil && k8serrors.IsNotFound(err)
			}, time.Minute, time.Second).Should(BeTrue())
		})

		It("should do smart clone", func() {
			createSnapshotDataSource()
			snapshot = createVolumeSnapshotSource("1Gi", nil, corev1.PersistentVolumeFilesystem)
			By("Creating target PVC")
			target := createTarget(defaultSize, corev1.PersistentVolumeFilesystem)
			By("Waiting for population to be succeeded")
			target = waitSucceeded(target)
			path := utils.DefaultImagePath
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, target, path, utils.UploadFileMD5, utils.UploadFileSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		})

		Context("Fallback to host assisted", func() {
			var noExpansionStorageClass *storagev1.StorageClass

			BeforeEach(func() {
				allowVolumeExpansion := false
				disableVolumeExpansion := func(sc *storagev1.StorageClass) {
					sc.AllowVolumeExpansion = &allowVolumeExpansion
				}
				sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), utils.DefaultStorageClass.GetName(), metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				noExpansionStorageClass, err = f.CreateVariationOfStorageClass(sc, disableVolumeExpansion)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should do regular host assisted clone", func() {
				createSnapshotDataSource()
				snapshot = createVolumeSnapshotSource("1Gi", &noExpansionStorageClass.Name, corev1.PersistentVolumeFilesystem)
				By("Creating target PVC")
				target := createTargetWithStrategy(biggerSize, corev1.PersistentVolumeFilesystem, "", noExpansionStorageClass.Name)
				By("Waiting for population to be succeeded")
				target = waitSucceeded(target)
				path := utils.DefaultImagePath
				same, err := f.VerifyTargetPVCContentMD5(f.Namespace, target, path, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
				By("Check tmp source PVC is deleted")
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(snapshot.Namespace).Get(context.TODO(), tmpSourcePVCforSnapshot, metav1.GetOptions{})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})

			It("should finish the clone after creating the source snapshot", func() {
				By("Create the clone before the source snapshot")
				target := createTargetWithStrategy(biggerSize, corev1.PersistentVolumeFilesystem, "", noExpansionStorageClass.Name)
				By("Create VolumeCloneSource and source snapshot")
				createSnapshotDataSource()
				snapshot = createVolumeSnapshotSource("1Gi", &noExpansionStorageClass.Name, corev1.PersistentVolumeFilesystem)
				By("Waiting for population to be succeeded")
				target = waitSucceeded(target)
				path := utils.DefaultImagePath
				same, err := f.VerifyTargetPVCContentMD5(f.Namespace, target, path, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
				By("Check tmp source PVC is deleted")
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(snapshot.Namespace).Get(context.TODO(), tmpSourcePVCforSnapshot, metav1.GetOptions{})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})
