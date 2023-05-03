package tests

import (
	"context"
	"crypto/md5"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

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
	snapshotAPIName     = "snapshot.storage.k8s.io"
)

var _ = Describe("Population tests", func() {
	f := framework.NewFramework("population-func-test")

	var (
		fileName          string
		expectedContent   string
		samplePopulatorCR *unstructured.Unstructured
	)

	sampleGVR := schema.GroupVersionResource{Group: populatorGroupName, Version: populatorAPIVersion, Resource: populatorResource}
	apiGroup := populatorGroupName
	dummyAPIGroup := "dummy.populator.io"
	dataSourceRef := &corev1.TypedLocalObjectReference{
		APIGroup: &apiGroup,
		Kind:     populatorKind,
		Name:     samplePopulatorName,
	}
	dummySourceRef := &corev1.TypedLocalObjectReference{
		APIGroup: &dummyAPIGroup,
		Kind:     "Dummy",
		Name:     "dummyname",
	}

	// If the AnyVolumeDataSource feature gate is disabled, Kubernetes drops the contents of the dataSourceRef field.
	// We can then determine if the feature is enabled or not by checking that field after creating a PVC.
	isAnyVolumeDataSourceEnabled := func() bool {
		pvc := utils.NewPVCDefinition("test", "10Mi", nil, nil)
		pvc.Spec.DataSourceRef = dummySourceRef
		pvc, err := f.CreatePVCFromDefinition(pvc)
		Expect(err).ToNot(HaveOccurred())
		enabled := pvc.Spec.DataSourceRef != nil
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())
		deleted, err := utils.WaitPVCDeleted(f.K8sClient, pvc.Name, pvc.Namespace, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
		return enabled
	}

	getSnapshotClassName := func() string {
		storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		scs := &snapshotv1.VolumeSnapshotClassList{}
		err = f.CrClient.List(context.TODO(), scs)
		Expect(err).ToNot(HaveOccurred())
		for _, snapshotClass := range scs.Items {
			if snapshotClass.Driver == storageclass.Provisioner {
				return snapshotClass.Name
			}
		}
		return ""
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

	getNonCSIStorage := func() (string, bool) {
		localSCName := "local"
		// We first check if the default storage class lacks CSI drivers
		if utils.DefaultStorageClassCsiDriver == nil {
			return utils.DefaultStorageClass.GetName(), true
		}
		// If it doesn't, we attempt to get the local storage class
		_, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), localSCName, metav1.GetOptions{})
		if err == nil {
			return localSCName, true
		}
		return "", false
	}

	Context("External populator", func() {
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
			dataVolume := utils.NewDataVolumeWithExternalPopulationAndStorageSpec(dataVolumeName, "100Mi", f.CsiCloneSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock), nil, dataSourceRef)
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

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
			dataVolume := utils.NewDataVolumeWithExternalPopulationAndStorageSpec(dataVolumeName, "100Mi", f.CsiCloneSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock), nil, dataSourceRef)
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
			By("Checking if non-CSI storage class is available")
			scName, available := getNonCSIStorage()
			if !available {
				Skip("No storage class to run without CSI drivers, cannot run test")
			}

			By(fmt.Sprintf("Creating new datavolume %s", dataVolumeName))
			dataVolume := utils.NewDataVolumeWithExternalPopulationAndStorageSpec(dataVolumeName, "100Mi", scName, corev1.PersistentVolumeMode(corev1.PersistentVolumeFilesystem), nil, dataSourceRef)
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
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

	Context("Legacy population", func() {
		It("Should perform a CSI PVC clone by manually populating the DataSource field", func() {
			if !f.IsCSIVolumeCloneStorageClassAvailable() {
				Skip("No CSI drivers available - Population not supported")
			}

			By("Creating source PVC")
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "80Mi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc := f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			dataSource := &corev1.TypedLocalObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: sourcePvc.Name,
			}

			By(fmt.Sprintf("Creating target datavolume %s", dataVolumeName))
			dataVolume := utils.NewDataVolumeWithExternalPopulationAndStorageSpec(dataVolumeName, "100Mi", f.CsiCloneSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeFilesystem), dataSource, nil)
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Verifying pvc was created")
			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetPvc.Namespace, corev1.ClaimBound, targetPvc.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying PVC's content")
			f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(dvc.ExternalPopulationSucceeded))
			sourcemd5, err := f.GetMD5(f.Namespace, sourcePvc, filepath.Join(testBaseDir, testFile), 0)
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			targetmd5, err := f.GetMD5(f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourcemd5).To(Equal(targetmd5))
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			err = f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
			}, timeout, pollingInterval).Should(BeTrue())
		})

		It("Should perform a Volume Snapshot clone through the DataSource field", func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Snapshot not possible")
			}

			By("Creating source PVC")
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "80Mi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc := f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)

			By("Creating Snapshot")
			snapshotClassName := getSnapshotClassName()
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "snapshot-" + pvcDef.Name,
					Namespace: pvcDef.Namespace,
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvcDef.Name,
					},
					VolumeSnapshotClassName: &snapshotClassName,
				},
			}
			snapshotAPIGroup := snapshotAPIName
			dataSource := &corev1.TypedLocalObjectReference{
				APIGroup: &snapshotAPIGroup,
				Kind:     "VolumeSnapshot",
				Name:     snapshot.Name,
			}
			err := f.CrClient.Create(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for Snapshot to be ready to use")
			Eventually(func() bool {
				err := f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(snapshot), snapshot)
				Expect(err).ToNot(HaveOccurred())
				return snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse
			}, timeout, pollingInterval).Should(BeTrue())

			By(fmt.Sprintf("Creating target datavolume %s", dataVolumeName))
			// PVC API because some provisioners only allow exact match between source size and restore size
			dataVolume := utils.NewDataVolumeWithExternalPopulation(dataVolumeName, snapshot.Status.RestoreSize.String(), f.SnapshotSCName, corev1.PersistentVolumeMode(corev1.PersistentVolumeFilesystem), dataSource, nil)
			controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Verifying pvc was created")
			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetPvc.Namespace, corev1.ClaimBound, targetPvc.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying PVC's content")
			f.ExpectEvent(dataVolume.Namespace).Should(ContainSubstring(dvc.ExternalPopulationSucceeded))
			sourcemd5, err := f.GetMD5(f.Namespace, sourcePvc, filepath.Join(testBaseDir, testFile), 0)
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
			targetmd5, err := f.GetMD5(f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourcemd5).To(Equal(targetmd5))
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Cleaning up")
			err = f.DeletePVC(sourcePvc)
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			err = f.CrClient.Delete(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				return k8serrors.IsNotFound(err)
			}, timeout, pollingInterval).Should(BeTrue())
			Eventually(func() bool {
				err := f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(snapshot), snapshot)
				return k8serrors.IsNotFound(err)
			}, timeout, pollingInterval).Should(BeTrue())
		})
	})
})
