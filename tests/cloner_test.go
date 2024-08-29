package tests

import (
	"context"
	"crypto/rsa"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/clone"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	namespacePrefix                  = "cloner"
	sourcePodFillerName              = "fill-source"
	sourcePVCName                    = "source-pvc"
	sizeDetectionPodPrefix           = "size-detection-"
	fillData                         = "123456789012345678901234567890123456789012345678901234567890"
	fillDataFSMD5sum                 = "fabc176de7eb1b6ca90b3aa4c7e035f3"
	testBaseDir                      = utils.DefaultPvcMountPath
	testFile                         = "/disk.img"
	fillCommand                      = "echo \"" + fillData + "\" >> " + testBaseDir
	blockFillCommand                 = "dd if=/dev/urandom bs=4096 of=" + testBaseDir + " || echo this is fine"
	assertionPollInterval            = 2 * time.Second
	cloneCompleteTimeout             = 270 * time.Second
	verifyPodDeletedTimeout          = 270 * time.Second
	controllerSkipPVCCompleteTimeout = 270 * time.Second
	crossVolumeModeCloneMD5NumBytes  = 100000
	hostAssistedCloneSource          = "cdi.kubevirt.io/hostAssistedSourcePodCloneSource"
)

var _ = Describe("all clone tests", func() {
	var _ = Describe("[rfe_id:1277][crit:high][vendor:cnv-qe@redhat.com][level:component]Cloner Test Suite", Serial, func() {
		f := framework.NewFramework(namespacePrefix)
		tinyCoreIsoURL := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }

		var originalProfileSpec *cdiv1.StorageProfileSpec
		var cloneStorageClassName string
		var sourcePvc *v1.PersistentVolumeClaim
		var targetPvc *v1.PersistentVolumeClaim
		var origSpec *cdiv1.CDIConfigSpec

		BeforeEach(func() {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			origSpec = config.Spec.DeepCopy()
		})

		AfterEach(func() {
			// Needs better cleanup, in the tests that actually change the overhead
			By("Restoring CDIConfig to original state")
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				origSpec.DeepCopyInto(config)
			})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return reflect.DeepEqual(config.Spec, *origSpec)
			}, 30*time.Second, time.Second).Should(BeTrue())

			if sourcePvc != nil {
				By("[AfterEach] Clean up source PVC")
				err := f.DeletePVC(sourcePvc)
				Expect(err).ToNot(HaveOccurred())
			}
			if targetPvc != nil {
				By("[AfterEach] Clean up target PVC")
				err := f.DeletePVC(targetPvc)
				Expect(err).ToNot(HaveOccurred())
			}

		})

		It("[test_id:6693]Should clone imported data and retain transfer pods after completion", func() {
			if utils.DefaultStorageClassCsiDriver != nil {
				Skip("Cannot test host-assisted cloning")
			}

			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
			By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", pvc)
			targetDV.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
			targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

			By("Wait for target datavolume phase Succeeded")
			Expect(utils.WaitForDataVolumePhaseWithTimeout(f, targetDataVolume.Namespace, cdiv1.Succeeded, targetDV.Name, cloneCompleteTimeout)).Should(Succeed())

			target, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetDataVolume.Namespace).Get(context.TODO(), targetDataVolume.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			f.ExpectCloneFallback(target, dvc.NoPopulator, dvc.NoPopulatorMessage)

			By("Find cloner source pod after completion")
			cloner, err := utils.FindPodBySuffixOnce(f.K8sClient, targetDataVolume.Namespace, common.ClonerSourcePodNameSuffix, common.CDILabelSelector)
			Expect(err).ToNot(HaveOccurred())
			Expect(cloner.DeletionTimestamp).To(BeNil())
			// The Pod should be associated with the host-assisted clone source
			Expect(cloner.GetLabels()[hostAssistedCloneSource]).To(Equal(string(pvc.GetUID())))

			By("Find upload pod after completion")
			uploader, err := utils.FindPodByPrefixOnce(f.K8sClient, targetDataVolume.Namespace, "cdi-upload-", common.CDILabelSelector)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploader.DeletionTimestamp).To(BeNil())
		})

		Context("DataVolume Garbage Collection", func() {
			var (
				ns       string
				err      error
				config   *cdiv1.CDIConfig
				origSpec *cdiv1.CDIConfigSpec
			)

			BeforeEach(func() {
				ns = f.Namespace.Name
				config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				origSpec = config.Spec.DeepCopy()
			})

			AfterEach(func() {
				By("Restoring CDIConfig to original state")
				err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
					origSpec.DeepCopyInto(config)
				})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return reflect.DeepEqual(config.Spec, *origSpec)
				}, timeout, pollingInterval).Should(BeTrue())
			})

			verifyGC := func(dvName string) {
				VerifyGC(f, dvName, ns, false, nil)
			}
			verifyDisabledGC := func(dvName string) {
				VerifyDisabledGC(f, dvName, ns)
			}
			enableGcAndAnnotateLegacyDv := func(dvName string) {
				EnableGcAndAnnotateLegacyDv(f, dvName, ns)
			}

			DescribeTable("Should", func(ttl int, verifyGCFunc, additionalTestFunc func(dvName string)) {
				SetConfigTTL(f, ttl)

				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, ns, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				verifyGCFunc(dataVolume.Name)

				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", pvc)
				delete(targetDV.Annotations, controller.AnnDeleteAfterCompletion)
				By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, ns, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target datavolume phase Succeeded")
				Expect(utils.WaitForDataVolumePhaseWithTimeout(f, targetDataVolume.Namespace, cdiv1.Succeeded, targetDV.Name, cloneCompleteTimeout)).Should(Succeed())
				verifyGCFunc(targetDV.Name)

				if additionalTestFunc != nil {
					additionalTestFunc(dataVolume.Name)
					additionalTestFunc(targetDV.Name)
				}
			},
				Entry("[test_id:8565] garbage collect dvs after completion when TTL is 0", 0, verifyGC, nil),
				Entry("[test_id:8569] Add DeleteAfterCompletion annotation to a legacy DV", -1, verifyDisabledGC, enableGcAndAnnotateLegacyDv),
			)
		})

		ClonerBehavior := func(storageClass string, cloneType string) {

			DescribeTable("[test_id:1354]Should clone data within same namespace", func(targetSize string) {
				By(storageClass)
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				pvcDef.Namespace = f.Namespace.Name
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				doFileBasedCloneTest(f, pvcDef, f.Namespace, "target-dv", targetSize)
			},
				Entry("with same target size", "1Gi"),
				Entry("with larger target", "2Gi"),
			)

			It("[test_id:4953]Should clone imported data within same namespace and preserve fsGroup", func() {
				diskImagePath := filepath.Join(testBaseDir, testFile)
				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())

				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				// Create targetPvc in new NS.
				targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", pvc)
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				fmt.Fprintf(GinkgoWriter, "INFO: wait for target DV phase Succeeded: %s\n", targetPvc.Name)
				Expect(utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDV.Name, 3*90*time.Second)).Should(Succeed())
				sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, pvc, true)
				fmt.Fprintf(GinkgoWriter, "INFO: %s\n", sourcePvcDiskGroup)
				Expect(err).ToNot(HaveOccurred())

				By("verifying pvc content")
				sourceMD5, err := f.GetMD5(f.Namespace, pvc, diskImagePath, 0)
				Expect(err).ToNot(HaveOccurred())

				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				completeClone(f, f.Namespace, targetPvc, diskImagePath, sourceMD5, sourcePvcDiskGroup)
			})

			It("[test_id:6784]Should clone imported data from SourceRef PVC DataSource", func() {
				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDS := utils.NewPvcDataSource("test-datasource", pvc.Namespace, pvc.Name, pvc.Namespace)
				By(fmt.Sprintf("Create new datasource %s", targetDS.Name))
				targetDataSource, err := f.CdiClient.CdiV1beta1().DataSources(pvc.Namespace).Create(context.TODO(), targetDS, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeWithSourceRef("target-dv", "1Gi", targetDataSource.Namespace, targetDataSource.Name)
				By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target datavolume phase Succeeded")
				Expect(utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDV.Name, 3*90*time.Second)).To(Succeed())
			})

			DescribeTable("[test_id:1355]Should clone data across different namespaces", func(targetSize string) {
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				pvcDef.Namespace = f.Namespace.Name

				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
					framework.NsPrefixLabel: f.NsPrefix,
				})
				Expect(err).NotTo(HaveOccurred())

				f.AddNamespaceToDelete(targetNs)
				doFileBasedCloneTest(f, pvcDef, targetNs, "target-dv", targetSize)
			},
				Entry("with same target size", "1Gi"),
				Entry("with bigger target size", "2Gi"),
			)

			It("[test_id:4954]Should clone data across different namespaces when source initially in use", func() {
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				pvcDef.Namespace = f.Namespace.Name
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
					framework.NsPrefixLabel: f.NsPrefix,
				})
				Expect(err).NotTo(HaveOccurred())
				f.AddNamespaceToDelete(targetNs)
				doInUseCloneTest(f, pvcDef, targetNs, "target-dv")
			})

			It("[test_id:1356]Should not clone anything when CloneOf annotation exists", func() {
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				cloneOfAnnoExistenceTest(f, f.Namespace.Name)
			})

			It("[posneg:negative][test_id:3617]Should clone across nodes when multiple local filesystem volumes exist,", func() {
				if utils.DefaultStorageClassCsiDriver != nil {
					Skip("this test is only relevant for non CSI local storage")
				}
				// Get nodes, need at least 2
				nodeList, err := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				if len(nodeList.Items) < 2 {
					Skip("Need at least 2 nodes to copy across nodes")
				}
				nodeMap := make(map[string]bool)
				for _, node := range nodeList.Items {
					if ok := nodeMap[node.Name]; !ok {
						nodeMap[node.Name] = true
					}
				}
				// Find PVs and identify local storage, the PVs should already exist.
				pvList, err := f.K8sClient.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				var sourcePV, targetPV *v1.PersistentVolume
				var storageClassName string
				// Verify we have PVs to at least 2 nodes.
				pvNodeNames := make(map[string]int)
				for _, pv := range pvList.Items {
					if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil || len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms) == 0 || (pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode == v1.PersistentVolumeBlock) {
						// Not a local volume filesystem PV
						continue
					}
					pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
					if pv.Spec.ClaimRef == nil {
						// PV is available and not claimed.
						if val, ok := pvNodeNames[pvNode]; !ok {
							pvNodeNames[pvNode] = 1
						} else {
							pvNodeNames[pvNode] = val + 1
						}
					}
				}
				if len(pvNodeNames) < 2 {
					Skip("Need filesystem PVs on at least 2 nodes to test")
				}

				// Find the source and target PVs so we can label them.
				for _, pv := range pvList.Items {
					if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil ||
						len(pv.Spec.NodeAffinity.Required.NodeSelectorTerms) == 0 || pv.Status.Phase != v1.VolumeAvailable {
						// Not an available local volume PV
						continue
					}
					if sourcePV == nil {
						if pv.Spec.StorageClassName != "" {
							storageClassName = pv.Spec.StorageClassName
						}
						pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
						if ok, val := nodeMap[pvNode]; ok && val {
							nodeMap[pvNode] = false
							By("Labeling PV " + pv.Name + " as source")
							Eventually(func() error {
								sourcePV, err = f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pv.Name, metav1.GetOptions{})
								Expect(err).ToNot(HaveOccurred())
								if sourcePV.GetLabels() == nil {
									sourcePV.SetLabels(make(map[string]string))
								}
								sourcePV.GetLabels()["source-pv"] = "yes"
								// We shouldn't make the test fail if there's a conflict with the update request.
								// These errors are usually transient and should be fixed in subsequent retries.
								sourcePV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), sourcePV, metav1.UpdateOptions{})
								return err
							}, timeout, pollingInterval).Should(Succeed())
						}
					} else if targetPV == nil {
						pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
						if ok, val := nodeMap[pvNode]; ok && val {
							nodeMap[pvNode] = false

							By("Labeling PV " + pv.Name + " as target")
							Eventually(func() error {
								targetPV, err = f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pv.Name, metav1.GetOptions{})
								Expect(err).ToNot(HaveOccurred())
								if targetPV.GetLabels() == nil {
									targetPV.SetLabels(make(map[string]string))
								}
								targetPV.GetLabels()["target-pv"] = "yes"
								// We shouldn't make the test fail if there's a conflict with the update request.
								// These errors are usually transient and should be fixed in subsequent retries.
								targetPV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), targetPV, metav1.UpdateOptions{})
								return err
							}, timeout, pollingInterval).Should(Succeed())
							break
						}
					}
				}
				Expect(sourcePV).ToNot(BeNil())
				Expect(targetPV).ToNot(BeNil())
				// Source and target PVs have been annotated, now create PVCs with label selectors.
				sourceSelector := make(map[string]string)
				sourceSelector["source-pv"] = "yes"
				sourcePVCDef := utils.NewPVCDefinitionWithSelector(sourcePVCName, "1Gi", storageClassName, sourceSelector, nil, nil)
				sourcePVCDef.Namespace = f.Namespace.Name
				sourcePVC := f.CreateAndPopulateSourcePVC(sourcePVCDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				sourcePVC, err = f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), sourcePVC.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(sourcePVC.Spec.VolumeName).To(Equal(sourcePV.Name))

				targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", sourcePVCDef)
				targetDV.Spec.PVC.StorageClassName = &storageClassName
				targetLabelSelector := metav1.LabelSelector{
					MatchLabels: map[string]string{
						"target-pv": "yes",
					},
				}
				targetDV.Spec.PVC.Selector = &targetLabelSelector
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				Eventually(func() bool {
					targetPvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					return targetPvc.Spec.VolumeName == targetPV.Name
				}, 60, 1).Should(BeTrue())

				fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
				Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)).Should(Succeed())
				completeClone(f, f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, "")
			})

			DescribeTable("Should clone data from filesystem to block", func(preallocate bool) {
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csi-clone" || cloneType == "snapshot" {
					Skip("csi-clone only works for the same volumeMode")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeCloneToBlockPV("target-dv", "1Gi", sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)
				if preallocate {
					targetDV.Spec.Preallocation = ptr.To[bool](true)
				}
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target PVC Bound phase")
				Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)).To(Succeed())
				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Source file system pvc md5summing")
				diskImagePath := filepath.Join(testBaseDir, testFile)
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Target block pvc md5summing")
				targetMD5, err := f.GetMD5(f.Namespace, targetPvc, testBaseDir, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			},
				Entry("[test_id:5569]regular target", false),
				Entry("[test_id:XXXX]preallocated target", true),
			)

			DescribeTable("[test_id:5570]Should clone data from block to filesystem", func(desiredPreallocation bool) {
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csi-clone" {
					Skip("csi-clone only works for the same volumeMode")
				}
				if !desiredPreallocation && cloneType != "copy" {
					Skip("Sparse is only guaranteed for copy")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImportToBlockPV(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs), f.BlockSCName)
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				volumeMode := v1.PersistentVolumeFilesystem
				targetDV := utils.NewDataVolumeForImageCloning("target-dv", "1.2Gi", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				targetDV.Spec.Preallocation = &desiredPreallocation
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Wait for target PVC Bound phase")
				Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)).To(Succeed())
				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Source block pvc md5summing")
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, testBaseDir, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Target file system pvc md5summing")
				diskImagePath := filepath.Join(testBaseDir, testFile)
				targetMD5, err := f.GetMD5(f.Namespace, targetPvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())

				// preallocation settings only respected for copy
				if cloneType == "copy" {
					preallocated, err := f.VerifyImagePreallocated(f.Namespace, targetPvc)
					Expect(err).ToNot(HaveOccurred())
					Expect(preallocated).To(Equal(desiredPreallocation))
				}
			},
				Entry("with preallocation", true),
				Entry("without preallocation", false),
			)

			It("bz:2079781 Should clone data from filesystem to block, when using storage API ", func() {
				SetFilesystemOverhead(f, "0.50", "0.50")
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csi-clone" || cloneType == "snapshot" {
					Skip("csi-clone only works for the same volumeMode")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "2Gi", fmt.Sprintf(utils.LargeVirtualDiskQcow, f.CdiInstallNs))
				controller.AddAnnotation(dataVolume, controller.AnnDeleteAfterCompletion, "false")
				filesystem := v1.PersistentVolumeFilesystem
				dataVolume.Spec.Storage.VolumeMode = &filesystem

				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeCloneToBlockPVStorageAPI("target-dv", "2Gi", sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)

				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target PVC Bound phase")
				Expect(
					utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name),
				).To(Succeed())
				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Source file system pvc md5summing")
				diskImagePath := filepath.Join(testBaseDir, testFile)
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Target block pvc md5summing")
				targetMD5, err := f.GetMD5(f.Namespace, targetPvc, testBaseDir, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should clone data from fs to fs while using calculated storage size", func() {
				// should clone from fs to fs using the same size in spec.storage.size
				// source pvc might be bigger than the size, but the clone should work
				// as the actual data is the same
				volumeMode := v1.PersistentVolumeFilesystem

				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume.Spec.Storage.VolumeMode = &volumeMode
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv", "1Gi", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(targetDV, controller.AnnDeleteAfterCompletion, "false")
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target PVC Bound phase")
				err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
				Expect(err).ToNot(HaveOccurred())
				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Source file system pvc md5summing")
				diskImagePath := filepath.Join(testBaseDir, testFile)
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Target file system pvc md5summing")
				targetMD5, err := f.GetMD5(f.Namespace, targetPvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[rfe_id:1126][crit:High][vendor:cnv-qe@redhat.com][level:component] Should fail with Event when cloning into a smaller sized data volume", func() {
				By("Creating a source from a real image")
				sourceDv := utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL())
				filesystem := v1.PersistentVolumeFilesystem
				sourceDv.Spec.PVC.VolumeMode = &filesystem
				sourceDv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)

				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

				By("Waiting for import to be completed")
				Expect(
					utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second),
				).Should(Succeed())

				By("Cloning from the source DataVolume to under sized target")
				targetDv := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv", "100Mi",
					f.Namespace.Name,
					sourceDv.Name,
					sourceDv.Spec.PVC.StorageClassName,
					sourceDv.Spec.PVC.VolumeMode)

				targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)

				f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrIncompatiblePVC))
			})

			It("should handle a pre populated PVC during clone", func() {
				By(fmt.Sprintf("initializing target PVC %s", dataVolumeName))
				targetPodFillerName := fmt.Sprintf("%s-filler-pod", dataVolumeName)
				annotations := map[string]string{controller.AnnPopulatedFor: dataVolumeName}
				targetPvcDef := utils.NewPVCDefinition(dataVolumeName, "1G", annotations, nil)
				targetPvc = f.CreateAndPopulateSourcePVC(targetPvcDef, targetPodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)

				By(fmt.Sprintf("initializing source PVC %s witg different data", dataVolumeName))
				alternativeFillData := "987654321"
				alternativeFillCommand := "echo \"" + alternativeFillData + "\" >> " + testBaseDir
				sourcePodFillerName := fmt.Sprintf("%s-filler-pod", "sourcepvcempty")
				srcPvcDef := utils.NewPVCDefinition("sourcepvcempty", "1G", nil, nil)
				sourcePvc := f.CreateAndPopulateSourcePVC(srcPvcDef, sourcePodFillerName, alternativeFillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)

				dataVolume := utils.NewDataVolumeForImageCloning(dataVolumeName, "1G",
					sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Spec.StorageClassName, sourcePvc.Spec.VolumeMode)
				Expect(dataVolume).ToNot(BeNil())

				By(fmt.Sprintf("creating new populated datavolume %s", dataVolume.Name))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					pvcName := dv.Annotations["cdi.kubevirt.io/storage.prePopulated"]
					return pvcName == targetPvcDef.Name &&
						dv.Status.Phase == cdiv1.Succeeded &&
						string(dv.Status.Progress) == "N/A"
				}, timeout, pollingInterval).Should(BeTrue(), "DV Should succeed with storage.prePopulated==pvcName")

				By("Verify no clone - the contents of prepopulated volume did not change")
				md5Match, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum)
				Expect(err).ToNot(HaveOccurred())
				Expect(md5Match).To(BeTrue())
			})

			DescribeTable("Should clone with empty volume size without using size-detection pod",
				func(sourceVolumeMode, targetVolumeMode v1.PersistentVolumeMode, sourceRef bool) {
					// When cloning without defining the target's storage size, the source's size can be attainable
					// by different means depending on the clone type and the volume mode used.
					// Either if "block" is used as volume mode or smart/csi cloning is used as clone strategy,
					// the value is simply extracted from the original PVC's spec.

					var sourceSCName string
					var targetSCName string
					targetDiskImagePath := filepath.Join(testBaseDir, testFile)
					sourceDiskImagePath := filepath.Join(testBaseDir, testFile)

					if cloneType == "copy" && sourceVolumeMode == v1.PersistentVolumeFilesystem {
						Skip("Clone strategy and volume mode combination requires of size-detection pod")
					}

					if sourceVolumeMode == v1.PersistentVolumeBlock {
						if !f.IsBlockVolumeStorageClassAvailable() {
							Skip("Storage Class for block volume is not available")
						}
						sourceSCName = f.BlockSCName
						sourceDiskImagePath = testBaseDir
					}

					if targetVolumeMode == v1.PersistentVolumeBlock {
						if !f.IsBlockVolumeStorageClassAvailable() {
							Skip("Storage Class for block volume is not available")
						}
						targetSCName = f.BlockSCName
						targetDiskImagePath = testBaseDir
					}

					// Create the source DV
					dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
					dataVolume.Spec.Storage.VolumeMode = &sourceVolumeMode
					if sourceSCName != "" {
						dataVolume.Spec.Storage.StorageClassName = &sourceSCName
					}

					dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
					sourcePvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					By("Wait for source DV Succeeded phase")
					err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolumeName, cloneCompleteTimeout)
					Expect(err).ToNot(HaveOccurred())
					var ds *cdiv1.DataSource
					if sourceRef {
						ds = utils.NewPvcDataSource("test-datasource", sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Namespace)
						By(fmt.Sprintf("Create new datasource %s", ds.Name))
						ds, err = f.CdiClient.CdiV1beta1().DataSources(sourcePvc.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{})
						Expect(err).ToNot(HaveOccurred())
					}

					// We attempt to create the sizeless DV
					targetDV := utils.NewDataVolumeForCloningWithEmptySize("target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &targetVolumeMode)
					if sourceRef {
						targetDV = utils.NewDataVolumeWithSourceRefAndStorageAPI("target-dv", nil, ds.Namespace, ds.Name)
						targetDV.Spec.Storage.VolumeMode = &targetVolumeMode
					}
					if targetSCName != "" {
						targetDV.Spec.Storage.StorageClassName = &targetSCName
					}

					controller.AddAnnotation(targetDV, controller.AnnDeleteAfterCompletion, "false")
					targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
					Expect(err).ToNot(HaveOccurred())
					targetPvc, err = utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

					By("Wait for target DV Succeeded phase")
					err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
					Expect(err).ToNot(HaveOccurred())

					By("Source file system pvc md5summing")
					sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, sourceDiskImagePath, crossVolumeModeCloneMD5NumBytes)
					Expect(err).ToNot(HaveOccurred())

					By("Deleting verifier pod")
					err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())
					_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
					Expect(err).ToNot(HaveOccurred())

					By("Target file system pvc md5summing")
					targetMD5, err := f.GetMD5(f.Namespace, targetPvc, targetDiskImagePath, crossVolumeModeCloneMD5NumBytes)
					Expect(err).ToNot(HaveOccurred())

					By("Deleting verifier pod")
					err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())

					By("Checksum comparison")
					Expect(sourceMD5).To(Equal(targetMD5))
				},
				Entry("[test_id:8492]Block to block (empty storage size)", v1.PersistentVolumeBlock, v1.PersistentVolumeBlock, false),
				Entry("[test_id:8491]Block to filesystem (empty storage size)", v1.PersistentVolumeBlock, v1.PersistentVolumeFilesystem, false),
				Entry("[test_id:8490]Filesystem to filesystem(empty storage size)", v1.PersistentVolumeFilesystem, v1.PersistentVolumeFilesystem, false),
				Entry("[test_id:8490]Filesystem to filesystem(empty storage size) with sourceRef", v1.PersistentVolumeFilesystem, v1.PersistentVolumeFilesystem, true),
			)

			Context("WaitForFirstConsumer with advanced cloning methods", func() {
				var wffcStorageClass *storagev1.StorageClass

				BeforeEach(func() {
					if cloneType != "csi-clone" && cloneType != "snapshot" {
						Skip("relevant for csi/smart clones only")
					}

					sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), utils.DefaultStorageClass.GetName(), metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode == storagev1.VolumeBindingImmediate {
						sc, err = f.CreateWFFCVariationOfStorageClass(sc)
						Expect(err).ToNot(HaveOccurred())
						wffcStorageClass = sc
						Eventually(func() bool {
							_, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), wffcStorageClass.Name, metav1.GetOptions{})
							return err == nil
						}, time.Minute, time.Second).Should(BeTrue())
						spec, err := utils.GetStorageProfileSpec(f.CdiClient, wffcStorageClass.Name)
						Expect(err).ToNot(HaveOccurred())
						if cloneType == "csi-clone" {
							Expect(utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, wffcStorageClass.Name, spec, cdiv1.CloneStrategyCsiClone)).Should(Succeed())
						} else if cloneType == "snapshot" {
							Expect(utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, wffcStorageClass.Name, spec, cdiv1.CloneStrategySnapshot)).Should(Succeed())
						}
					}
				})

				It("should report correct status for smart/CSI clones", func() {
					volumeMode := v1.PersistentVolumeFilesystem

					dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
					dataVolume.Spec.Storage.VolumeMode = &volumeMode
					if wffcStorageClass != nil {
						dataVolume.Spec.Storage.StorageClassName = &wffcStorageClass.Name
					}
					dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
					dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
					Expect(err).ToNot(HaveOccurred())
					By("Waiting for import to be completed")
					err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					targetDV := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv", "1Gi", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
					if wffcStorageClass != nil {
						targetDV.Spec.Storage.StorageClassName = &wffcStorageClass.Name
					}
					targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
					Expect(err).ToNot(HaveOccurred())
					targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					By("Ensure WFFC is reported to reflect the situation correctly")
					err = utils.WaitForDataVolumePhase(f, targetDataVolume.Namespace, cdiv1.PendingPopulation, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())

					// Force bind to ensure integrity after first consumer
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)
					By("Wait for target PVC Bound phase")
					err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
					Expect(err).ToNot(HaveOccurred())
					By("Wait for target DV Succeeded phase")
					err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())

					By("Verify content")
					same, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
					Expect(err).ToNot(HaveOccurred())
					Expect(same).To(BeTrue())
					By("Deleting verifier pod")
					err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should succeed smart/CSI clones with immediate bind requested", func() {
					volumeMode := v1.PersistentVolumeFilesystem

					dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
					dataVolume.Spec.Storage.VolumeMode = &volumeMode
					if wffcStorageClass != nil {
						dataVolume.Spec.Storage.StorageClassName = &wffcStorageClass.Name
					}
					dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
					dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
					Expect(err).ToNot(HaveOccurred())
					By("Waiting for import to be completed")
					err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					targetDV := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv", "1Gi", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
					if wffcStorageClass != nil {
						targetDV.Spec.Storage.StorageClassName = &wffcStorageClass.Name
					}
					targetDV.Annotations[controller.AnnImmediateBinding] = "true"
					targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
					Expect(err).ToNot(HaveOccurred())
					targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())

					By("Wait for target DV Succeeded phase")
					err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, targetDataVolume.Name)
					Expect(err).ToNot(HaveOccurred())
					if targetPvc.Spec.DataSourceRef != nil && targetPvc.Spec.DataSourceRef.Kind == cdiv1.VolumeCloneSourceRef {
						Expect(targetPvc.Annotations[controller.AnnCloneType]).To(Equal(cloneType))
					} else {
						Expect(targetPvc.Annotations[controller.AnnCloneRequest]).To(Equal(fmt.Sprintf("%s/%s", sourcePvc.Namespace, sourcePvc.Name)))
						Expect(targetPvc.Spec.DataSource).To(BeNil())
						Expect(targetPvc.Spec.DataSourceRef).To(BeNil())
					}

					By("Verify content")
					same, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
					Expect(err).ToNot(HaveOccurred())
					Expect(same).To(BeTrue())
					By("Deleting verifier pod")
					err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("Validate Data Volume should clone multiple clones in parallel", func() {
				tinyCoreIsoURL := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }

				var (
					sourceDv  *cdiv1.DataVolume
					targetDvs []*cdiv1.DataVolume
					err       error
				)

				AfterEach(func() {
					targetDvs = append(targetDvs, sourceDv)
					for _, dv := range targetDvs {
						cleanDv(f, dv)
						if dv != nil && dv.Status.Phase == cdiv1.Succeeded {
							validateCloneType(f, dv)
						}
					}
					targetDvs = nil
				})

				getClonerPodName := func(pvc *corev1.PersistentVolumeClaim) string {
					usedPvc := pvc
					if usesPopulator, _ := dvc.CheckPVCUsingPopulators(pvc); usesPopulator {
						pvcPrime, err := utils.WaitForPVC(f.K8sClient, pvc.Namespace, fmt.Sprintf("tmp-pvc-%s", string(pvc.UID)))
						Expect(err).ToNot(HaveOccurred())
						usedPvc = pvcPrime
					}
					return controller.CreateCloneSourcePodName(usedPvc)
				}

				It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel", func() {
					const NumOfClones int = 3

					By("Creating a source from a real image")
					sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL())
					sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

					By("Waiting for import to be completed")
					Expect(
						utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second),
					).Should(Succeed())

					By("Calculating the md5sum of the source data volume")
					md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+utils.DefaultImagePath, true)
					Expect(err).ToNot(HaveOccurred())
					_, _ = fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

					err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), "execute-command", metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
					_, err = utils.WaitPodDeleted(f.K8sClient, "execute-command", f.Namespace.Name, verifyPodDeletedTimeout)
					Expect(err).ToNot(HaveOccurred())

					// By not waiting for completion, we will start 3 transfers in parallel
					By("Cloning #NumOfClones times in parallel")
					for i := 1; i <= NumOfClones; i++ {
						By("Cloning from the source DataVolume to target" + strconv.Itoa(i))
						targetDv := utils.NewDataVolumeForImageCloning("target-dv"+strconv.Itoa(i), "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
						targetDv.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
						targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
						Expect(err).ToNot(HaveOccurred())
						f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)
						targetDvs = append(targetDvs, targetDv)
					}

					podsNodeName := make(map[string]bool)
					for _, dv := range targetDvs {
						By("Waiting for clone to be completed")
						err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
						Expect(err).ToNot(HaveOccurred())
					}

					if cloneType == "copy" {
						// Make sure we don't have high number of restart counts on source pods
						for _, dv := range targetDvs {
							pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
							Expect(err).ToNot(HaveOccurred())
							clonerPodName := getClonerPodName(pvc)
							cloner, err := f.K8sClient.CoreV1().Pods(dv.Namespace).Get(context.TODO(), clonerPodName, metav1.GetOptions{})
							Expect(err).ToNot(HaveOccurred())
							restartCount := cloner.Status.ContainerStatuses[0].RestartCount
							fmt.Fprintf(GinkgoWriter, "INFO: restart count on clone source pod %s: %d\n", clonerPodName, restartCount)
							// TODO remove the comment when the issue in #2550 is fixed
							// Expect(restartCount).To(BeNumerically("<", 2))
						}
					}

					for _, dv := range targetDvs {
						By("Verifying MD5 sum matches")
						matchFile := filepath.Join(testBaseDir, "disk.img")
						Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), matchFile, md5sum[:32])).To(BeTrue())

						pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())

						if cloneSourcePod := pvc.Annotations[controller.AnnCloneSourcePod]; cloneSourcePod != "" {
							By(fmt.Sprintf("Getting pod %s/%s", dv.Namespace, cloneSourcePod))
							pod, err := f.K8sClient.CoreV1().Pods(dv.Namespace).Get(context.TODO(), cloneSourcePod, metav1.GetOptions{})
							Expect(err).ToNot(HaveOccurred())
							podsNodeName[pod.Spec.NodeName] = true
						}

						By("Deleting verifier pod")
						err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), utils.VerifierPodName, metav1.DeleteOptions{})
						Expect(err).ToNot(HaveOccurred())
						_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
						Expect(err).ToNot(HaveOccurred())
					}

					// All pods should be in the same node except when the map is empty in smart clone
					if cloneType == "network" {
						Expect(podsNodeName).To(HaveLen(1))
					}
				})

				It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel for block devices", func() {
					const NumOfClones int = 3

					if !f.IsBlockVolumeStorageClassAvailable() {
						Skip("Storage Class for block volume is not available")
					}
					By("Creating a source from a real image")
					sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL(), f.BlockSCName)
					sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

					By("Waiting for import to be completed")
					Expect(utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)).To(Succeed())

					By("Calculating the md5sum of the source data volume")
					md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir, true)
					Expect(err).ToNot(HaveOccurred())
					_, _ = fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

					// By not waiting for completion, we will start 3 transfers in parallell
					By("Cloning #NumOfClones times in parallel")
					for i := 1; i <= NumOfClones; i++ {
						By("Cloning from the source DataVolume to target" + strconv.Itoa(i))
						targetDv := utils.NewDataVolumeForImageCloning("target-dv"+strconv.Itoa(i), "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
						targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
						Expect(err).ToNot(HaveOccurred())
						f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)
						targetDvs = append(targetDvs, targetDv)
					}

					for _, dv := range targetDvs {
						By("Waiting for clone to be completed")
						err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
						Expect(err).ToNot(HaveOccurred())
					}

					for _, dv := range targetDvs {
						By("Verifying MD5 sum matches")
						Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), testBaseDir, md5sum[:32])).To(BeTrue())
						By("Deleting verifier pod")
						err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
						Expect(err).ToNot(HaveOccurred())
					}

				})
			})
		}

		Context("HostAssisted Clone", func() {
			BeforeEach(func() {
				cloneStorageClassName = utils.DefaultStorageClass.GetName()
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))

				spec, err := utils.GetStorageProfileSpec(f.CdiClient, cloneStorageClassName)
				Expect(err).ToNot(HaveOccurred())
				originalProfileSpec = spec

				By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
				Expect(
					utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyHostAssisted),
				).Should(Succeed())

			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				Expect(utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)).Should(Succeed())
			})
			ClonerBehavior(cloneStorageClassName, "copy")
		})

		Context("SmartClone", func() {
			BeforeEach(func() {
				if !f.IsSnapshotStorageClassAvailable() {
					Skip("SmartClone does not work without SnapshotStorageClass")
				}
				cloneStorageClassName = f.SnapshotSCName
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))

				spec, err := utils.GetStorageProfileSpec(f.CdiClient, cloneStorageClassName)
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprintf("Got original storage profile: %v", spec))
				originalProfileSpec = spec

				By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
				Expect(
					utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategySnapshot),
				).Should(Succeed())
			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				Expect(utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)).Should(Succeed())
			})
			ClonerBehavior(cloneStorageClassName, "snapshot")
		})

		Context("[rfe_id:4219]CSI Clone", func() {
			BeforeEach(func() {
				if !f.IsCSIVolumeCloneStorageClassAvailable() {
					Skip("CSI Clone does not work without a capable storage class")
				}
				cloneStorageClassName = f.CsiCloneSCName
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))

				spec, err := utils.GetStorageProfileSpec(f.CdiClient, cloneStorageClassName)
				Expect(err).ToNot(HaveOccurred())
				originalProfileSpec = spec

				By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
				Expect(
					utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyCsiClone),
				).Should(Succeed())
			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				Expect(utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)).Should(Succeed())
			})
			ClonerBehavior(cloneStorageClassName, "csi-clone")
		})

		// The size-detection pod is only used in cloning when three requirements are met:
		// 	1. The clone manifest is created without defining a storage size.
		//	2. 'Filesystem' is used as volume mode.
		//	3. 'HostAssisted' is used as clone strategy.
		Context("Clone with empty size using the size-detection pod", func() {
			diskImagePath := filepath.Join(testBaseDir, testFile)
			volumeMode := v1.PersistentVolumeFilesystem

			deleteAndWaitForVerifierPod := func() {
				By("Deleting verifier pod")
				err := utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}

			deleteAndWaitForSizeDetectionPod := func() {
				By("Deleting size-detection pod")
				pod, err := utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
				Expect(err).ToNot(HaveOccurred())
				err = utils.DeletePod(f.K8sClient, pod, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, pod.Name, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}

			compareCloneWithSource := func(sourcePvc, targetPvc *v1.PersistentVolumeClaim, sourceImgPath, targetImgPath string) {
				By("Source file system pvc md5summing")
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, sourceImgPath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				deleteAndWaitForVerifierPod()

				By("Target file system pvc md5summing")
				targetMD5, err := f.GetMD5(f.Namespace, targetPvc, targetImgPath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				deleteAndWaitForVerifierPod()
			}

			BeforeEach(func() {
				cloneStorageClassName = utils.DefaultStorageClass.GetName()
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))

				spec, err := utils.GetStorageProfileSpec(f.CdiClient, cloneStorageClassName)
				Expect(err).ToNot(HaveOccurred())
				originalProfileSpec = spec

				By(fmt.Sprintf("configure storage profile %s", cloneStorageClassName))
				Expect(
					utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyHostAssisted),
				).To(Succeed())

			})

			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				Expect(utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)).To(Succeed())
			})

			DescribeTable("Should clone with different overheads in target and source", func(sourceOverHead, targetOverHead string) {
				SetFilesystemOverhead(f, sourceOverHead, sourceOverHead)
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "200Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume.Spec.Storage.VolumeMode = &volumeMode
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolumeName, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				SetFilesystemOverhead(f, targetOverHead, targetOverHead)
				targetDV := utils.NewDataVolumeForCloningWithEmptySize("target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(targetDV, controller.AnnDeleteAfterCompletion, "false")
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target PVC Bound phase")
				err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
				Expect(err).ToNot(HaveOccurred())
				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, targetPvc, diskImagePath, diskImagePath)
				// Check if the PermissiveClone annotation exists in target PVC
				By("Check expected annotations")
				// TODO: work on this with next PR to remove the annotation
				_, available := targetPvc.Annotations[controller.AnnPermissiveClone]
				Expect(available).To(BeTrue())
			},
				Entry("[test_id:8666]Smaller overhead in source than in target", "0.50", "0.30"),
				Entry("[test_id:8665]Bigger overhead in source than in target", "0.30", "0.50"),
			)

			It("[test_id:8498]Should only use size-detection pod when cloning a PVC for the first time", func() {
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "200Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume.Spec.Storage.VolumeMode = &volumeMode
				controller.AddAnnotation(dataVolume, controller.AnnPodRetainAfterCompletion, "true")
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Wait for source DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolumeName, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// We attempt to create the sizeless clone
				targetDataVolume := utils.NewDataVolumeForCloningWithEmptySize("target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(targetDataVolume, controller.AnnDeleteAfterCompletion, "false")
				targetDataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDataVolume)
				Expect(err).ToNot(HaveOccurred())

				// We verify that the size-detection pod is created the first time
				By("Verify size-detection pod is created")
				Eventually(func() *v1.Pod {
					pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
					return pod
				}, time.Minute, time.Second).ShouldNot(BeNil(), "Creating size-detection pod")

				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, targetPvc, diskImagePath, diskImagePath)
				deleteAndWaitForSizeDetectionPod()

				// We attempt to create the second, sizeless clone
				secondTargetDV := utils.NewDataVolumeForCloningWithEmptySize("second-target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(secondTargetDV, controller.AnnDeleteAfterCompletion, "false")
				secondTargetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, secondTargetDV)
				Expect(err).ToNot(HaveOccurred())

				// We verify that the size-detection pod is not created anymore
				By("Verify size-detection pod is not created")
				Consistently(func() *v1.Pod {
					pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
					return pod
				}, time.Second*30, time.Second).Should(BeNil(), "Verify size-detection pod is not created anymore")

				secondTargetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, secondTargetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(secondTargetDataVolume)

				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "second-target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, secondTargetPvc, diskImagePath, diskImagePath)
			})

			It("[test_id:8762]Should use size-detection pod when cloning if the source PVC has changed its original capacity", func() {
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume.Spec.Storage.VolumeMode = &volumeMode
				controller.AddAnnotation(dataVolume, controller.AnnPodRetainAfterCompletion, "true")
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Wait for source DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolumeName, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// We attempt to create the sizeless clone
				targetDV := utils.NewDataVolumeForCloningWithEmptySize("target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(targetDV, controller.AnnDeleteAfterCompletion, "false")
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())

				// We verify that the size-detection pod is created the first time
				By("Verify size-detection pod is created")
				Eventually(func() *v1.Pod {
					pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
					return pod
				}, time.Minute*2, time.Second).ShouldNot(BeNil(), "Creating size-detection pod")

				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, targetPvc, diskImagePath, diskImagePath)
				deleteAndWaitForSizeDetectionPod()

				// Since modifying the original PVC's capacity would require restarting several pods,
				// we just modify the 'AnnSourceCapacity' to mock that behavior
				By("Modify source PVC's capacity")
				Eventually(func() error {
					sourcePvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourcePvc.Namespace).Get(context.TODO(), sourcePvc.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					sourcePvc.Annotations[dvc.AnnSourceCapacity] = "400Mi"
					// We shouldn't make the test fail if there's a conflict with the update request.
					// These errors are usually transient and should be fixed in subsequent retries.
					_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourcePvc.Namespace).Update(context.TODO(), sourcePvc, metav1.UpdateOptions{})
					return err
				}, timeout, pollingInterval).Should(Succeed())

				// We attempt to create the second, sizeless clone
				By("Create second clone")
				secondTargetDV := utils.NewDataVolumeForCloningWithEmptySize("second-target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(secondTargetDV, controller.AnnDeleteAfterCompletion, "false")
				secondTargetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, secondTargetDV)
				Expect(err).ToNot(HaveOccurred())

				// We verify that the size-detection pod needs to be created again
				By("Verify second size-detection pod is created")
				Eventually(func() *v1.Pod {
					pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
					return pod
				}, time.Minute*2, time.Second).ShouldNot(BeNil(), "Creating size-detection pod")

				secondTargetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, secondTargetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(secondTargetDataVolume)

				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "second-target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, secondTargetPvc, diskImagePath, diskImagePath)
			})

			It("Should clone using size-detection pod across namespaces", func() {
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "200Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume.Spec.Storage.VolumeMode = &volumeMode
				controller.AddAnnotation(dataVolume, controller.AnnPodRetainAfterCompletion, "true")
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Wait for source DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dataVolumeName, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// We create the target namespace
				targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
					framework.NsPrefixLabel: f.NsPrefix,
				})
				Expect(err).NotTo(HaveOccurred())
				f.AddNamespaceToDelete(targetNs)

				// We attempt to create the sizeless clone
				targetDataVolume := utils.NewDataVolumeForCloningWithEmptySize("target-dv", f.Namespace.Name, sourcePvc.Name, nil, &volumeMode)
				controller.AddAnnotation(targetDataVolume, controller.AnnDeleteAfterCompletion, "false")
				targetDataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDataVolume)
				Expect(err).ToNot(HaveOccurred())

				// We verify that the size-detection pod is created
				By("Verify size-detection pod is created")
				Eventually(func() *v1.Pod {
					pod, _ := utils.FindPodByPrefixOnce(f.K8sClient, f.Namespace.Name, sizeDetectionPodPrefix, "")
					return pod
				}, time.Minute, time.Second).ShouldNot(BeNil(), "Creating size-detection pod")

				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, targetDataVolume.Namespace, cdiv1.Succeeded, "target-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				By("Source file system pvc md5summing")
				sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				deleteAndWaitForVerifierPod()

				By("Target file system pvc md5summing")
				targetMD5, err := f.GetMD5(targetNs, targetPvc, diskImagePath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				Expect(sourceMD5).To(Equal(targetMD5))
				deleteAndWaitForVerifierPod()

				deleteAndWaitForSizeDetectionPod()
			})
		})

		Context("CloneStrategy on storageclass annotation", Serial, func() {
			cloneType := cdiv1.CloneStrategyCsiClone
			var originalStrategy *cdiv1.CDICloneStrategy

			BeforeEach(func() {
				if !f.IsCSIVolumeCloneStorageClassAvailable() {
					cloneStorageClassName = ""
					Skip("CSI Volume Clone is not applicable")
				}
				cloneStorageClassName = f.CsiCloneSCName
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))
				storageProfile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				originalStrategy = storageProfile.Status.CloneStrategy
				Expect(storageProfile.Spec.CloneStrategy).To(BeNil())

				storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if storageclass.GetAnnotations() == nil {
					storageclass.SetAnnotations(make(map[string]string))
				}

				storageclass.Annotations["cdi.kubevirt.io/clone-strategy"] = string(cloneType)
				_, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), storageclass, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() cdiv1.CDICloneStrategy {
					storageProfile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if storageProfile.Status.CloneStrategy == nil {
						return ""
					}
					return *storageProfile.Status.CloneStrategy
				}, time.Minute, time.Second).Should(Equal(cloneType))

			})

			AfterEach(func() {
				if cloneStorageClassName == "" {
					return
				}
				By("[AfterEach] Restore the storage class - remove annotation ")
				storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if storageclass.GetAnnotations() != nil {
					delete(storageclass.Annotations, "cdi.kubevirt.io/clone-strategy")
				}
				_, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), storageclass, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() *cdiv1.CDICloneStrategy {
					storageProfile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return storageProfile.Status.CloneStrategy
				}, time.Minute, time.Second).Should(Equal(originalStrategy))
				cloneStorageClassName = ""
			})

			It("Should clone  with correct strategy from storageclass annotation ", func() {
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				pvcDef.Spec.StorageClassName = &cloneStorageClassName
				pvcDef.Namespace = f.Namespace.Name

				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)

				targetDV := utils.NewCloningDataVolume("target-pvc", "1Gi", sourcePvc)

				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				Expect(utils.GetCloneType(f.CdiClient, dataVolume)).To(Equal("csi-clone"))
			})
		})

		Context("Clone without a source PVC", func() {
			diskImagePath := filepath.Join(testBaseDir, testFile)
			fsVM := v1.PersistentVolumeFilesystem
			blockVM := v1.PersistentVolumeBlock

			deleteAndWaitForVerifierPod := func() {
				By("Deleting verifier pod")
				err := utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}

			getAndHandleMD5 := func(pvc *v1.PersistentVolumeClaim, imgPath string) string {
				defer deleteAndWaitForVerifierPod()
				md5, err := f.GetMD5(f.Namespace, pvc, imgPath, crossVolumeModeCloneMD5NumBytes)
				Expect(err).ToNot(HaveOccurred())
				return md5
			}

			compareCloneWithSource := func(sourcePvc, targetPvc *v1.PersistentVolumeClaim, sourceImgPath, targetImgPath string) {
				By("Source file system pvc md5summing")
				sourceMD5 := getAndHandleMD5(sourcePvc, sourceImgPath)
				By("Target file system pvc md5summing")
				targetMD5 := getAndHandleMD5(targetPvc, targetImgPath)
				Expect(sourceMD5).To(Equal(targetMD5))
			}

			It("Should finish the clone after creating the source PVC", func() {
				By("Create the clone before the source PVC")
				cloneDV := utils.NewDataVolumeForImageCloningAndStorageSpec("clone-dv", "1Gi", f.Namespace.Name, dataVolumeName, nil, &fsVM)
				cloneDV, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
				Expect(err).ToNot(HaveOccurred())
				// Check if the NoSourceClone annotation exists in target PVC
				By("Check the expected event")
				f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(dvc.CloneWithoutSource))

				By("Create source PVC")
				sourceDV := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				sourceDV.Spec.Storage.VolumeMode = &fsVM
				sourceDV, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDV)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(sourceDV.Namespace).Get(context.TODO(), sourceDV.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				clonePvc, err := utils.WaitForPVC(f.K8sClient, cloneDV.Namespace, cloneDV.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(cloneDV)

				By("Wait for clone PVC Bound phase")
				err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, cloneDV.Name)
				Expect(err).ToNot(HaveOccurred())
				By("Wait for clone DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, "clone-dv", cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())

				// Compare the two clones to see if they have the same hash
				compareCloneWithSource(sourcePvc, clonePvc, diskImagePath, diskImagePath)
			})

			It("Should reject the clone after creating the source PVC if the validation fails ", func() {
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}

				By("Create the clone before the source PVC")
				cloneDV := utils.NewDataVolumeForImageCloningAndStorageSpec("clone-dv", "1Mi", f.Namespace.Name, dataVolumeName, &f.BlockSCName, &blockVM)
				_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
				Expect(err).ToNot(HaveOccurred())
				// Check if the NoSourceClone annotation exists in target PVC
				By("Check the expected event")
				f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(dvc.CloneWithoutSource))

				By("Create source PVC")
				// We use a larger size in the source PVC so the validation fails
				sourceDV := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				sourceDV.Spec.Storage.VolumeMode = &blockVM
				sourceDV, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDV)
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourceDV.Namespace).Get(context.TODO(), sourceDV.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("The clone should fail")
				f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(dvc.CloneValidationFailed))
			})

			// TODO: check if this test is a duplicate of It("should handle a pre populated PVC during clone", func()
			It("Should not clone when PopulatedFor annotation exists", func() {
				targetName := "target" + rand.String(12)

				By(fmt.Sprintf("Creating target pvc: %s/%s", f.Namespace.Name, targetName))
				f.CreateBoundPVCFromDefinition(
					utils.NewPVCDefinition(targetName, "1Gi", map[string]string{controller.AnnPopulatedFor: targetName}, nil))
				cloneDV := utils.NewDataVolumeForImageCloningAndStorageSpec(targetName, "1Gi", f.Namespace.Name, "non-existing-source", nil, &fsVM)
				controller.AddAnnotation(cloneDV, controller.AnnDeleteAfterCompletion, "false")
				_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
				Expect(err).ToNot(HaveOccurred())
				By("Wait for clone DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetName, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), targetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				_, ok := dv.Annotations["cdi.kubevirt.io/cloneType"]
				Expect(ok).To(BeFalse())
			})

		})
	})

	var _ = Describe("Validate creating multiple clones of same source Data Volume", func() {
		f := framework.NewFramework(namespacePrefix)
		tinyCoreIsoURL := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }

		var (
			sourceDv, targetDv1, targetDv2, targetDv3 *cdiv1.DataVolume
			err                                       error
		)

		AfterEach(func() {
			dvs := []*cdiv1.DataVolume{sourceDv, targetDv1, targetDv2, targetDv3}
			for _, dv := range dvs {
				if dv != nil && dv.Status.Phase == cdiv1.Succeeded {
					validateCloneType(f, dv)
				}
				cleanDv(f, dv)
			}
		})

		It("[rfe_id:1277][test_id:1891][crit:High][vendor:cnv-qe@redhat.com][level:component]Should allow multiple clones from a single source datavolume", func() {
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL())
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			Expect(
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second),
			).To(Succeed())

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+utils.DefaultImagePath, true)
			Expect(err).ToNot(HaveOccurred())
			_, _ = fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

			err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), "execute-command", metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitPodDeleted(f.K8sClient, "execute-command", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())

			By("Cloning from the source DataVolume to target1")
			targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			By("Cloning from the source DataVolume to target2")
			targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			By("Cloning from the target1 DataVolume to target3")
			targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, targetDv1.Name, targetDv1.Spec.PVC.StorageClassName, targetDv1.Spec.PVC.VolumeMode)
			dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}

			for _, dv := range dvs {
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				By("Waiting for clone to be completed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
				Expect(err).ToNot(HaveOccurred())

				matchFile := filepath.Join(testBaseDir, "disk.img")
				Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), matchFile, md5sum[:32])).To(BeTrue())
				err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), utils.VerifierPodName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("[rfe_id:1277][test_id:1891][crit:High][vendor:cnv-qe@redhat.com][level:component]Should allow multiple clones from a single source datavolume in block volume mode", func() {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL(), f.BlockSCName)
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			Expect(
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second),
			).To(Succeed())
			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir, true)
			retry := 0
			for err != nil && retry < 10 {
				retry++
				md5sum, err = f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir, true)
			}
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])
			err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), "execute-command", metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitPodDeleted(f.K8sClient, "execute-command", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())

			By("Cloning from the source DataVolume to target1")
			targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			By("Cloning from the source DataVolume to target2")
			targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			By("Cloning from the target1 DataVolume to target3")
			targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, targetDv1.Name, targetDv1.Spec.PVC.StorageClassName, targetDv1.Spec.PVC.VolumeMode)
			dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}

			for _, dv := range dvs {
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

				By("Waiting for clone to be completed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
				Expect(err).ToNot(HaveOccurred())
				Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), testBaseDir, md5sum[:32])).To(BeTrue())
				err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), utils.VerifierPodName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	var _ = Describe("With nfs and larger target capacity", func() {
		f := framework.NewFramework(namespacePrefix)
		var (
			bigPV *v1.PersistentVolume
			bigDV *cdiv1.DataVolume
		)

		AfterEach(func() {
			if bigDV != nil {
				err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, bigDV.Name)
				Expect(err).ToNot(HaveOccurred())
			}

			if bigPV != nil {
				err := utils.WaitTimeoutForPVDeleted(f.K8sClient, bigPV, 30*time.Second)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should successfully clone", func() {
			if !utils.IsNfs() {
				Skip("NFS specific test; fine with both static NFS and CSI because no CSI clone")
			}

			By("Creating a source from a real image")
			sourceDv := utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
			sourceDv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			Expect(
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second),
			).To(Succeed())
			if utils.IsStaticNfsWithInternalClusterServer() {
				pvDef := framework.NfsPvDef(1, framework.ExtraNfsDiskPrefix, utils.NfsService.Spec.ClusterIP, framework.BiggerNfsPvSize)
				pv, err := utils.CreatePVFromDefinition(f.K8sClient, pvDef)
				Expect(err).ToNot(HaveOccurred())
				bigPV = pv
			}

			targetDv := utils.NewDataVolumeForImageCloning("target-dv", framework.BiggerNfsPvSize, f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
			Expect(err).ToNot(HaveOccurred())
			bigDV = targetDv
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)

			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())

			By("Verify target is bigger")
			srcPVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(sourceDv.Namespace).Get(context.TODO(), sourceDv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			targetPVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetDv.Namespace).Get(context.TODO(), targetDv.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			srcCapacity := srcPVC.Status.Capacity.Storage()
			Expect(srcCapacity).ToNot(BeNil())
			targetCapacity := targetPVC.Status.Capacity.Storage()
			Expect(targetCapacity).ToNot(BeNil())
			Expect(srcCapacity.Cmp(*targetCapacity)).To(Equal(-1))

			By("Verify content")
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, targetPVC, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		})
	})

	var _ = Describe("Block PV Cloner Test", func() {
		f := framework.NewFramework(namespacePrefix)

		DescribeTable("[test_id:4955]Should clone data across namespaces", func(targetSize string) {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			sourceSize := "500M"
			ss := resource.MustParse(sourceSize)
			pvcDef := utils.NewBlockPVCDefinition(sourcePVCName, sourceSize, nil, nil, f.BlockSCName)
			sourcePvc := f.CreateAndPopulateSourcePVC(pvcDef, "fill-source-block-pod", blockFillCommand)
			sourceMD5, err := f.GetMD5(f.Namespace, sourcePvc, testBaseDir, ss.Value())
			Expect(err).ToNot(HaveOccurred())

			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			targetDV := utils.NewDataVolumeCloneToBlockPV("target-dv", targetSize, sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)
			controller.AddAnnotation(targetDV, controller.AnnDeleteAfterCompletion, "false")
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("INFO: wait for PVC claim phase: %s\n", targetPvc.Name))
			// skipping error check because in some cases (e.g. on ceph), the PVC is never on "bound" phase, or it is
			// for a very short period that we'll probably' miss
			_ = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)

			Expect(
				utils.WaitForDataVolumePhaseWithTimeout(f, targetNs.Name, cdiv1.Succeeded, "target-dv", 3*90*time.Second),
			).Should(Succeed())
			Expect(f.VerifyTargetPVCContentMD5(targetNs, targetPvc, testBaseDir, sourceMD5, ss.Value())).To(BeTrue())
			By("Deleting verifier pod")
			Expect(utils.DeleteVerifierPod(f.K8sClient, targetNs.Name)).Should(Succeed())

			validateCloneType(f, dataVolume)

			targetPvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(targetPvc.Namespace).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			es := resource.MustParse(targetSize)
			Expect(es.Cmp(*targetPvc.Status.Capacity.Storage())).To(BeNumerically("<=", 0))
		},
			Entry("with same target size", "500M"),
			Entry("with bigger target", "1Gi"),
		)
	})

	var _ = Describe("Namespace with quota", Serial, func() {
		f := framework.NewFramework(namespacePrefix)
		var (
			orgConfig *v1.ResourceRequirements
			sourcePvc *v1.PersistentVolumeClaim
			targetPvc *v1.PersistentVolumeClaim
		)

		BeforeEach(func() {
			By("Capturing original CDIConfig state")
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			orgConfig = config.Spec.PodResourceRequirements.DeepCopy()
		})

		AfterEach(func() {
			By("Restoring CDIConfig to original state")
			err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
				config.PodResourceRequirements = orgConfig
			})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return reflect.DeepEqual(config.Spec.PodResourceRequirements, orgConfig)
			}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not properly restored to original value")

			if sourcePvc != nil {
				By("[AfterEach] Clean up source PVC")
				err := f.DeletePVC(sourcePvc)
				Expect(err).ToNot(HaveOccurred())
			}
			if targetPvc != nil {
				By("[AfterEach] Clean up target PVC")
				err := f.DeletePVC(targetPvc)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("[test_id:4956]Should create clone in namespace with quota", func() {
			err := f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
				}
			}

			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			doFileBasedCloneTest(f, pvcDef, f.Namespace, "target-dv")
		})

		It("[test_id:4957]Should fail to clone in namespace with quota when pods have higher requirements", func() {
			err := f.UpdateCdiConfigResourceLimits(int64(2), int64(1024*1024*1024), int64(2), int64(1*1024*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
				}
			}

			By("Populating source PVC")
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			// Create targetPvc in new NS.
			By("Creating new DV")
			targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", pvcDef)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: "Error starting pod",
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, f.Namespace.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring("Error starting pod"))
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrExceededQuota))
		})

		It("[test_id:4958]Should fail to clone in namespace with quota when pods have higher requirements, then succeed when quota increased", func() {
			err := f.UpdateCdiConfigResourceLimits(int64(0), int64(256*1024*1024), int64(0), int64(256*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			err = f.CreateQuotaInNs(int64(1), int64(128*1024*1024), int64(2), int64(128*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
				}
			}

			By("Populating source PVC")
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			// Create targetPvc in new NS.
			By("Creating new DV")
			targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", pvcDef)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: "Error starting pod",
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, f.Namespace.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring("Error starting pod"))
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrExceededQuota))

			Expect(f.UpdateQuotaInNs(int64(1), int64(512*1024*1024), int64(4), int64(512*1024*1024))).To(Succeed())
			Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetDV.Name)).To(Succeed())
			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, sourcePvc, true)
			Expect(err).ToNot(HaveOccurred())
			completeClone(f, f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, sourcePvcDiskGroup)
		})

		It("[test_id:4959]Should create clone in namespace with quota when pods requirements are low enough", func() {
			err := f.UpdateCdiConfigResourceLimits(int64(0), int64(0), int64(1), int64(512*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			err = f.CreateQuotaInNs(int64(1), int64(1024*1024*1024), int64(2), int64(2*1024*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
				}
			}

			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			doFileBasedCloneTest(f, pvcDef, f.Namespace, "target-dv")
		})

		It("[test_id:4960]Should fail clone data across namespaces, if source namespace doesn't have enough quota", func() {
			err := f.UpdateCdiConfigResourceLimits(int64(0), int64(512*1024*1024), int64(1), int64(512*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			err = f.CreateQuotaInNs(int64(1), int64(256*1024*1024), int64(2), int64(256*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "500M", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			targetDV := utils.NewDataVolumeForImageCloning("target-dv", "500M", sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Spec.StorageClassName, sourcePvc.Spec.VolumeMode)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "copy" {
				Skip("only valid for copy clone")
			}

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: "Error starting pod",
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, targetNs.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring("Error starting pod"))
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring(controller.ErrExceededQuota))
		})

		It("[test_id:9076]Should fail clone data across namespaces, if target namespace doesn't have enough quota", func() {
			err := f.UpdateCdiConfigResourceLimits(int64(0), int64(512*1024*1024), int64(1), int64(512*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "500M", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			err = f.CreateQuotaInSpecifiedNs(targetNs.Name, (1), int64(256*1024*1024), int64(2), int64(256*1024*1024))
			Expect(err).NotTo(HaveOccurred())

			targetDV := utils.NewDataVolumeForImageCloning("target-dv", "500M", sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Spec.StorageClassName, sourcePvc.Spec.VolumeMode)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())

			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			if targetPvc.Spec.DataSourceRef != nil {
				Skip("only valid for non csi clone")
			}

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "copy" {
				Skip("only valid for copy clone")
			}

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: "Error starting pod",
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, targetNs.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring("Error starting pod"))
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring(controller.ErrExceededQuota))
		})
	})

	var _ = Describe("[rfe_id:1277][crit:high][vendor:cnv-qe@redhat.com][level:component]Cloner Test Suite", func() {
		f := framework.NewFramework(namespacePrefix)

		var sourcePvc *v1.PersistentVolumeClaim
		var targetPvc *v1.PersistentVolumeClaim
		var errAsString = func(e error) string { return e.Error() }

		AfterEach(func() {
			if sourcePvc != nil {
				By("[AfterEach] Clean up source PVC")
				err := f.DeletePVC(sourcePvc)
				Expect(err).ToNot(HaveOccurred())
			}
			if targetPvc != nil {
				By("[AfterEach] Clean up target PVC")
				err := f.DeletePVC(targetPvc)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("[test_id:3999] Create a data volume and then clone it and verify retry count", Serial, func() {
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			targetDvName := "target-dv"
			doFileBasedCloneTest(f, pvcDef, targetNs, targetDvName)

			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			cloneType := utils.GetCloneType(f.CdiClient, dv)
			if cloneType != "copy" {
				Skip("only valid for copy clone")
			}

			By("Verify retry annotation on PVC")
			targetPvc, err := utils.WaitForPVC(f.K8sClient, targetNs.Name, targetDvName)
			Expect(err).ToNot(HaveOccurred())
			restartsValue, status, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodRestarts)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(BeTrue())
			Expect(restartsValue).To(Equal("0"))

			By("Verify the number of retries on the datavolume")
			Expect(dv.Status.RestartCount).To(BeNumerically("==", 0))
		})

		It("[test_id:4000] Create a data volume and then clone it while killing the container and verify retry count", func() {
			By("Prepare source PVC")
			sourceDV := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
			sourceDV, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDV)
			Expect(err).ToNot(HaveOccurred())

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDV)
			sourcePvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourceDV.Namespace).Get(context.TODO(), sourceDV.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create clone DV")
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)
			targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", sourcePvc)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())

			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			if targetPvc.Spec.DataSourceRef != nil {
				// Skipping with csi because force bind early causes to succeed very quickly
				// cannot catch pod
				Skip("only for non csi-clone")
			}

			f.ForceBindIfWaitForFirstConsumer(targetPvc)

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "copy" {
				Skip("only valid for copy clone")
			}

			fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
			Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)).To(Succeed())

			By("Wait for upload pod")
			err = utils.WaitTimeoutForPodReadyPollPeriod(f.K8sClient, utils.UploadPodName(targetPvc), targetNs.Name, utils.PodWaitIntervalFast, utils.PodWaitForTime)
			Expect(err).ToNot(HaveOccurred())

			By("Kill upload pod to force error")
			// exit code 137 = 128 + 9, it means parent process issued kill -9, in our case it is not a problem
			_, _, err = f.ExecShellInPod(utils.UploadPodName(targetPvc), targetNs.Name, "kill 1")
			Expect(err).To(Or(
				Not(HaveOccurred()),
				WithTransform(errAsString, ContainSubstring("137"))))

			By("Verify retry annotation on PVC")
			Eventually(func() int {
				restarts, status, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodRestarts)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(BeTrue())
				i, err := strconv.Atoi(restarts)
				Expect(err).ToNot(HaveOccurred())
				return i
			}, timeout, pollingInterval).Should(BeNumerically(">=", 1))

			By("Verify the number of retries on the datavolume")
			Eventually(func() int32 {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				restarts := dv.Status.RestartCount
				return restarts
			}, timeout, pollingInterval).Should(BeNumerically(">=", 1))

		})

		It("[test_id:4276] Clone datavolume with short name", Serial, func() {
			shortDvName := "import-long-name-dv"

			By(fmt.Sprintf("Create PVC %s", shortDvName))
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetDvName := shortDvName
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			doFileBasedCloneTest(f, pvcDef, targetNs, targetDvName)

			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			cloneType := utils.GetCloneType(f.CdiClient, dv)
			if cloneType == "copy" {
				By("Verify retry annotation on PVC")
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetNs.Name, targetDvName)
				Expect(err).ToNot(HaveOccurred())
				restartsValue, status, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodRestarts)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(BeTrue())
				Expect(restartsValue).To(Equal("0"))

				By("Verify the number of retries on the datavolume")
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(dv.Status.RestartCount).To(BeNumerically("==", 0))
			}
		})

		It("[test_id:4277] Clone datavolume with long name", Serial, func() {
			// 20 chars + 100ch + 40chars
			dvName160Characters := "import-long-name-dv-" +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
				"123456789-123456789-123456789-1234567890"

			By(fmt.Sprintf("Create PVC %s", dvName160Characters))
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetDvName := dvName160Characters
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			doFileBasedCloneTest(f, pvcDef, targetNs, targetDvName)

			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			cloneType := utils.GetCloneType(f.CdiClient, dv)
			if cloneType == "copy" {
				By("Verify retry annotation on PVC")
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetNs.Name, targetDvName)
				Expect(err).ToNot(HaveOccurred())
				restartsValue, status, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodRestarts)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(BeTrue())
				Expect(restartsValue).To(Equal("0"))

				By("Verify the number of retries on the datavolume")
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(dv.Status.RestartCount).To(BeNumerically("==", 0))
			}
		})

		It("[test_id:4278] Clone datavolume with long name including special character '.'", Serial, func() {
			// 20 chars + 100ch + 40chars
			dvName160Characters := "import-long-name-dv." +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
				"123456789-123456789-123456789-1234567890"

			By(fmt.Sprintf("Create PVC %s", dvName160Characters))
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			targetDvName := dvName160Characters
			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).NotTo(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			doFileBasedCloneTest(f, pvcDef, targetNs, targetDvName)

			dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			cloneType := utils.GetCloneType(f.CdiClient, dv)
			if cloneType == "copy" {
				By("Verify retry annotation on PVC")
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetNs.Name, targetDvName)
				Expect(err).ToNot(HaveOccurred())
				restartsValue, status, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnPodRestarts)
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(BeTrue())
				Expect(restartsValue).To(Equal("0"))

				By("Verify the number of retries on the datavolume")
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetDvName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(dv.Status.RestartCount).To(BeNumerically("==", 0))
			}
		})

		It("should recreate and reclone target pvc if it was deleted", func() {
			pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
			pvcDef.Namespace = f.Namespace.Name
			sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
			dvName := "target-dv"
			doFileBasedCloneTest(f, pvcDef, f.Namespace, dvName, "1Gi")

			targetPVC, err := f.FindPVC(dvName)
			Expect(err).ToNot(HaveOccurred())

			By("Delete target PVC")
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, dvName)
			Expect(err).ToNot(HaveOccurred())

			deleted, err := f.WaitPVCDeletedByUID(targetPVC, time.Minute)
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())

			targetPVC, err = utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dvName)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(targetPVC)

			By("Verify target PVC is bound again")
			err = utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, dvName)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	var _ = Describe("Preallocation", Serial, func() {
		f := framework.NewFramework(namespacePrefix)

		var sourcePvc *v1.PersistentVolumeClaim
		var targetPvc *v1.PersistentVolumeClaim
		var cdiCr cdiv1.CDI
		var cdiCrSpec *cdiv1.CDISpec

		BeforeEach(func() {
			By("[BeforeEach] Saving CDI CR spec")
			crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(crList.Items).To(HaveLen(1))

			cdiCrSpec = crList.Items[0].Spec.DeepCopy()
			cdiCr = crList.Items[0]

			By("[BeforeEach] Forcing Host Assisted cloning")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			cdiCr.Spec.CloneStrategyOverride = &cloneStrategy
			_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), &cdiCr, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(utils.WaitForCDICrCloneStrategy(f.CdiClient, cloneStrategy)).To(Succeed())
		})

		AfterEach(func() {
			if sourcePvc != nil {
				By("[AfterEach] Clean up source PVC")
				Expect(f.DeletePVC(sourcePvc)).To(Succeed())
			}
			if targetPvc != nil {
				By("[AfterEach] Clean up target PVC")
				Expect(f.DeletePVC(targetPvc)).To(Succeed())
			}

			By("[AfterEach] Restoring CDI CR spec to original state")
			crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(crList.Items).To(HaveLen(1))

			newCdiCr := crList.Items[0]
			newCdiCr.Spec = *cdiCrSpec
			_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), &newCdiCr, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			if cdiCrSpec.CloneStrategyOverride == nil {
				err = utils.WaitForCDICrCloneStrategyNil(f.CdiClient)
			} else {
				err = utils.WaitForCDICrCloneStrategy(f.CdiClient, *cdiCrSpec.CloneStrategyOverride)
			}
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should preallocate data on target PVC", func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
			sourcePvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			targetDV := utils.NewCloningDataVolume("target-dv", "1Gi", sourcePvc)
			preallocation := true
			targetDV.Spec.Preallocation = &preallocation
			targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

			targetPvc, err = utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			annValue, preallocationAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetDataVolume.Namespace, targetPvc, controller.AnnPreallocationRequested)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Failed to wait for PVC annotation: %v", err)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(preallocationAnnotationFound).To(BeTrue())
			Expect(annValue).To(Equal("true"))

			fmt.Fprintf(GinkgoWriter, "INFO: wait for target DV phase Succeeded: %s\n", targetPvc.Name)

			annValue, preallocationAnnotationFound, err = utils.WaitForPVCAnnotation(f.K8sClient, targetDataVolume.Namespace, targetPvc, controller.AnnPreallocationApplied)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Failed to wait for PVC annotation: %v", err)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(preallocationAnnotationFound).To(BeTrue())
			Expect(annValue).To(Equal("true"))
		})
	})

	var _ = Describe("[rfe_id:9453] Clone from volumesnapshot source", func() {
		f := framework.NewFramework(namespacePrefix)

		var snapshot *snapshotv1.VolumeSnapshot
		var targetNamespace *v1.Namespace

		createSnapshot := func(size string, storageClassName *string, volumeMode v1.PersistentVolumeMode) {
			snapSourceDv := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
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
			snapshot = utils.NewVolumeSnapshot("snap-"+snapSourceDv.Name, f.Namespace.Name, pvc.Name, &snapClass.Name)
			err = f.CrClient.Create(context.TODO(), snapshot)
			Expect(err).ToNot(HaveOccurred())

			snapshot = utils.WaitSnapshotReady(f.CrClient, snapshot)
			By("Snapshot ready, no need to keep PVC around")
			utils.CleanupDvPvc(f.K8sClient, f.CdiClient, f.Namespace.Name, pvc.Name)
		}

		BeforeEach(func() {
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Clone from volumesnapshot does not work without snapshot capable storage")
			}
		})

		AfterEach(func() {
			if snapshot != nil {
				By(fmt.Sprintf("[AfterEach] Removing snapshot %s/%s", snapshot.Namespace, snapshot.Name))
				Eventually(func() bool {
					err := f.CrClient.Delete(context.TODO(), snapshot)
					return err != nil && k8serrors.IsNotFound(err)
				}, time.Minute, time.Second).Should(BeTrue())
				snapshot = nil
			}

			if targetNamespace != nil {
				err := f.K8sClient.CoreV1().Namespaces().Delete(context.TODO(), targetNamespace.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				targetNamespace = nil
			}
		})

		DescribeTable("Should successfully clone without falling back to host assisted", func(volumeMode v1.PersistentVolumeMode, repeat int, crossNamespace bool) {
			var i int
			var err error

			defaultSc := utils.DefaultStorageClass.GetName()
			if crossNamespace && f.IsBindingModeWaitForFirstConsumer(&defaultSc) {
				Skip("only host assisted is applicable with WFFC cross namespace")
			}
			if !f.IsSnapshotStorageClassAvailable() {
				Skip("Clone from volumesnapshot does not work without snapshot capable storage")
			}
			if volumeMode == v1.PersistentVolumeBlock && !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}

			targetNs := f.Namespace
			if crossNamespace {
				targetNamespace, err = f.CreateNamespace("cdi-cross-ns-snapshot-clone-test", nil)
				Expect(err).ToNot(HaveOccurred())
				targetNs = targetNamespace
			}
			size := "1Gi"
			createSnapshot(size, &f.SnapshotSCName, volumeMode)

			for i = 0; i < repeat; i++ {
				dataVolume := utils.NewDataVolumeForSnapshotCloningAndStorageSpec(fmt.Sprintf("clone-from-snap-%d", i), size, snapshot.Namespace, snapshot.Name, &f.SnapshotSCName, &volumeMode)
				dataVolume.Labels = map[string]string{"test-label-1": "test-label-value-1"}
				dataVolume.Annotations = map[string]string{"test-annotation-1": "test-annotation-value-1"}
				By(fmt.Sprintf("Create new datavolume %s which will clone from volumesnapshot", dataVolume.Name))
				dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
			}

			for i = 0; i < repeat; i++ {
				By("Waiting for clone to be completed")
				dvName := fmt.Sprintf("clone-from-snap-%d", i)
				err = utils.WaitForDataVolumePhase(f, targetNs.Name, cdiv1.Succeeded, dvName)
				Expect(err).ToNot(HaveOccurred())
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				_, ok := pvc.Annotations[controller.AnnCloneRequest]
				Expect(ok).To(BeFalse())
				Expect(pvc.Spec.DataSource.Kind).To(Equal("VolumeCloneSource"))
				Expect(pvc.Spec.DataSourceRef.Kind).To(Equal("VolumeCloneSource"))
				// All labels and annotations passed
				Expect(pvc.Labels["test-label-1"]).To(Equal("test-label-value-1"))
				Expect(pvc.Annotations["test-annotation-1"]).To(Equal("test-annotation-value-1"))
			}

			By("Verify MD5 on one of the DVs")
			lastDvName := fmt.Sprintf("clone-from-snap-%d", i-1)
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), lastDvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			path := utils.DefaultImagePath
			if volumeMode == v1.PersistentVolumeBlock {
				path = utils.DefaultPvcMountPath
			}
			same, err := f.VerifyTargetPVCContentMD5(targetNs, pvc, path, utils.UploadFileMD5, utils.UploadFileSize)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		},
			Entry("[test_id:9703] with filesystem single clone", v1.PersistentVolumeFilesystem, 1, true),
			Entry("[test_id:9708] with filesystem multiple clones", v1.PersistentVolumeFilesystem, 5, false),
			Entry("[test_id:9709] with block single clone", v1.PersistentVolumeBlock, 1, false),
			Entry("[test_id:9710] with block multiple clones", v1.PersistentVolumeBlock, 5, false),
		)

		Context("Fallback to host assisted", func() {
			var noExpansionStorageClass *storagev1.StorageClass

			BeforeEach(func() {
				allowVolumeExpansion := false
				disableVolumeExpansion := func(sc *storagev1.StorageClass) {
					sc.AllowVolumeExpansion = &allowVolumeExpansion
				}
				sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), utils.DefaultStorageClass.GetName(), metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				noExpansionStorageClass, err = f.CreateNonDefaultVariationOfStorageClass(sc, disableVolumeExpansion)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("Should successfully clone using host assisted", func(volumeMode v1.PersistentVolumeMode, repeat int, crossNamespace bool) {
				var i int
				var err error

				if volumeMode == v1.PersistentVolumeBlock && !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				targetNs := f.Namespace
				if crossNamespace {
					targetNamespace, err = f.CreateNamespace("cdi-cross-ns-snapshot-clone-test", nil)
					Expect(err).ToNot(HaveOccurred())
					targetNs = targetNamespace
				}
				snapSourceSize := "1Gi"
				// Make sure expansion required, that's how we achieve fallback to host assisted
				targetDvSize := "2Gi"
				createSnapshot(snapSourceSize, &noExpansionStorageClass.Name, volumeMode)

				for i = 0; i < repeat; i++ {
					dataVolume := utils.NewDataVolumeForSnapshotCloningAndStorageSpec(fmt.Sprintf("clone-from-snap-%d", i), targetDvSize, snapshot.Namespace, snapshot.Name, &noExpansionStorageClass.Name, &volumeMode)
					By(fmt.Sprintf("Create new datavolume %s which will clone from volumesnapshot", dataVolume.Name))
					dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, dataVolume)
					Expect(err).ToNot(HaveOccurred())
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				}

				for i = 0; i < repeat; i++ {
					dvName := fmt.Sprintf("clone-from-snap-%d", i)
					By("Waiting for clone to be completed")
					err = utils.WaitForDataVolumePhase(f, targetNs.Name, cdiv1.Succeeded, dvName)
					Expect(err).ToNot(HaveOccurred())
					By("Check host assisted clone is taking place")
					pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					f.ExpectCloneFallback(pvc, clone.NoVolumeExpansion, clone.MessageNoVolumeExpansion)

					// non csi
					if pvc.Spec.DataSourceRef == nil {
						suffix := "-host-assisted-source-pvc"
						Expect(pvc.Annotations[controller.AnnCloneRequest]).To(HaveSuffix(suffix))
						Expect(pvc.Spec.DataSource).To(BeNil())
					} else {
						dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())
						cloneType := utils.GetCloneType(f.CdiClient, dv)
						Expect(cloneType).To(Equal(string(cdiv1.CloneStrategyHostAssisted)))
					}
				}

				By("Verify MD5 on one of the DVs")
				lastDvName := fmt.Sprintf("clone-from-snap-%d", i-1)
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), lastDvName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				path := utils.DefaultImagePath
				if volumeMode == v1.PersistentVolumeBlock {
					path = utils.DefaultPvcMountPath
				}
				same, err := f.VerifyTargetPVCContentMD5(targetNs, pvc, path, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
			},
				Entry("[test_id:9714] with filesystem single clone", v1.PersistentVolumeFilesystem, 1, true),
				Entry("[test_id:9715] with filesystem multiple clones", v1.PersistentVolumeFilesystem, 5, false),
				Entry("[test_id:9716] with block single clone", v1.PersistentVolumeBlock, 1, false),
				Entry("[test_id:9717] with block multiple clones", v1.PersistentVolumeBlock, 5, false),
			)
		})

		Context("Immediate bind requested", func() {
			var wffcStorageClass *storagev1.StorageClass

			BeforeEach(func() {
				sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), utils.DefaultStorageClass.GetName(), metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				wffcStorageClass = sc
				if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode == storagev1.VolumeBindingImmediate {
					sc, err = f.CreateWFFCVariationOfStorageClass(sc)
					Expect(err).ToNot(HaveOccurred())
					wffcStorageClass = sc
					Eventually(func() error {
						_, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), wffcStorageClass.Name, metav1.GetOptions{})
						return err
					}, time.Minute, time.Second).Should(Succeed())
				}
			})

			It("should succeed if immediate bind requested", func() {
				var err error

				size := "1Gi"
				volumeMode := v1.PersistentVolumeFilesystem
				dvName := "clone-from-snap"
				createSnapshot(size, &wffcStorageClass.Name, volumeMode)

				dataVolume := utils.NewDataVolumeForSnapshotCloningAndStorageSpec(dvName, size, snapshot.Namespace, snapshot.Name, &wffcStorageClass.Name, &volumeMode)
				By(fmt.Sprintf("Create new datavolume %s which will clone from volumesnapshot", dataVolume.Name))
				dataVolume.Annotations[controller.AnnImmediateBinding] = "true"
				dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, snapshot.Namespace, dataVolume)
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for clone to be completed")
				Expect(utils.WaitForDataVolumePhase(f, dataVolume.Namespace, cdiv1.Succeeded, dataVolume.Name)).To(Succeed())
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Verify content")
				same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
			})
		})

		Context("Clone without a source snapshot", func() {
			It("[test_id:9718] Should finish the clone after creating the source snapshot", func() {
				if !f.IsSnapshotStorageClassAvailable() {
					Skip("Clone from volumesnapshot does not work without snapshot capable storage")
				}
				size := "1Gi"
				volumeMode := v1.PersistentVolumeFilesystem
				By("Create the clone before the source snapshot")
				cloneDV := utils.NewDataVolumeForSnapshotCloningAndStorageSpec("clone-from-snap", size, f.Namespace.Name, "snap-"+dataVolumeName, &f.SnapshotSCName, &volumeMode)
				cloneDV, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
				Expect(err).ToNot(HaveOccurred())
				// Check if the NoSourceClone annotation exists in target PVC
				// By("Check the expected event")
				// f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(dvc.CloneWithoutSource))

				By("Create source snapshot")
				createSnapshot(size, &f.SnapshotSCName, volumeMode)

				clonePvc, err := utils.WaitForPVC(f.K8sClient, cloneDV.Namespace, cloneDV.Name)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(cloneDV)

				By("Wait for clone PVC Bound phase")
				Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, cloneDV.Name)).To(Succeed())
				By("Wait for clone DV Succeeded phase")
				Expect(
					utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, cloneDV.Name, cloneCompleteTimeout),
				).To(Succeed())

				By("Verify MD5")
				path := utils.DefaultImagePath
				same, err := f.VerifyTargetPVCContentMD5(f.Namespace, clonePvc, path, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
			})
		})

		Context("sourceRef support", func() {
			DescribeTable("[test_id:9758] Should clone data from SourceRef snapshot DataSource", func(sizeless bool) {
				if !f.IsSnapshotStorageClassAvailable() {
					Skip("Clone from volumesnapshot does not work without snapshot capable storage")
				}

				size := "1Gi"
				volumeMode := v1.PersistentVolumeFilesystem
				createSnapshot(size, &f.SnapshotSCName, volumeMode)

				targetDS := utils.NewSnapshotDataSource("test-datasource", snapshot.Namespace, snapshot.Name, snapshot.Namespace)
				By(fmt.Sprintf("Create new datasource %s", targetDS.Name))
				targetDataSource, err := f.CdiClient.CdiV1beta1().DataSources(snapshot.Namespace).Create(context.TODO(), targetDS, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				var targetSizePtr *string
				if !sizeless {
					targetSizePtr = &size
				}
				targetDV := utils.NewDataVolumeWithSourceRefAndStorageAPI("target-dv", targetSizePtr, targetDataSource.Namespace, targetDataSource.Name)
				targetDV.Spec.Storage.StorageClassName = &f.SnapshotSCName
				targetDV.Spec.Storage.VolumeMode = &volumeMode
				By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for clone DV Succeeded phase")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDV.Name, cloneCompleteTimeout)
				Expect(err).ToNot(HaveOccurred())
				By("Verify MD5")
				path := utils.DefaultImagePath
				same, err := f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDV), path, utils.UploadFileMD5, utils.UploadFileSize)
				Expect(err).ToNot(HaveOccurred())
				Expect(same).To(BeTrue())
			},
				Entry("with size specified on target", false),
				Entry("with size omitted on target", true),
			)
		})
	})
})

func doFileBasedCloneTest(f *framework.Framework, srcPVCDef *v1.PersistentVolumeClaim, targetNs *v1.Namespace, targetDv string, targetSize ...string) {
	if len(targetSize) == 0 {
		targetSize = []string{"1Gi"}
	}
	// Create targetPvc in new NS.
	targetDV := utils.NewCloningDataVolume(targetDv, targetSize[0], srcPVCDef)
	if targetDV.GetLabels() == nil {
		targetDV.SetLabels(make(map[string]string))
	}
	if targetDV.GetAnnotations() == nil {
		targetDV.SetAnnotations(make(map[string]string))
	}
	targetDV.Labels["test-label-1"] = "test-label-key-1"
	targetDV.Annotations["test-annotation-1"] = "test-annotation-key-1"

	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
	Expect(err).ToNot(HaveOccurred())

	targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
	Expect(err).ToNot(HaveOccurred())
	f.ForceBindIfWaitForFirstConsumer(targetPvc)

	_, _ = fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)).To(Succeed())
	sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, srcPVCDef, true)
	_, _ = fmt.Fprintf(GinkgoWriter, "INFO: %s\n", sourcePvcDiskGroup)
	Expect(err).ToNot(HaveOccurred())

	completeClone(f, targetNs, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, sourcePvcDiskGroup)

	targetPvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(targetPvc.Namespace).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	es := resource.MustParse(targetSize[0])
	Expect(es.Cmp(*targetPvc.Status.Capacity.Storage())).To(BeNumerically("<=", 0))

	// All labels and annotations passed
	Expect(targetPvc.Labels["test-label-1"]).To(Equal("test-label-key-1"))
	Expect(targetPvc.Annotations["test-annotation-1"]).To(Equal("test-annotation-key-1"))

	if targetNs.Name != f.Namespace.Name {
		dataVolume, err = f.CdiClient.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dataVolume.Annotations).To(HaveKey(controller.AnnExtendedCloneToken))
	}
}

func doInUseCloneTest(f *framework.Framework, srcPVCDef *v1.PersistentVolumeClaim, targetNs *v1.Namespace, targetDv string) {
	pod, err := f.CreateExecutorPodWithPVC("temp-pod", f.Namespace.Name, srcPVCDef, false)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() bool {
		pod, err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return pod.Status.Phase == v1.PodRunning
	}, 90*time.Second, 2*time.Second).Should(BeTrue())

	// Create targetPvc in new NS.
	targetDV := utils.NewCloningDataVolume(targetDv, "1Gi", srcPVCDef)
	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
	Expect(err).ToNot(HaveOccurred())

	var targetPvc *v1.PersistentVolumeClaim
	targetPvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
	Expect(err).ToNot(HaveOccurred())
	f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

	f.ExpectEvent(targetNs.Name).Should(ContainSubstring(controller.CloneSourceInUse))
	err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())

	_, _ = fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	Expect(utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)).Should(Succeed())
	sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, srcPVCDef, true)
	Expect(err).ToNot(HaveOccurred())
	_, _ = fmt.Fprintf(GinkgoWriter, "INFO: %s\n", sourcePvcDiskGroup)

	completeClone(f, targetNs, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, sourcePvcDiskGroup)
}

func completeClone(f *framework.Framework, targetNs *v1.Namespace, targetPvc *v1.PersistentVolumeClaim, filePath, expectedMD5, sourcePvcDiskGroup string) {
	if _, ok := targetPvc.Annotations[controller.AnnCloneRequest]; ok {
		By("Verify the clone annotation is on the target PVC")
		_, cloneAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnCloneOf)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Failed to wait for PVC annotation: %v", err)
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(cloneAnnotationFound).To(BeTrue())
	}

	By("Verify the clone status is success on the target datavolume")
	err := utils.WaitForDataVolumePhase(f, targetNs.Name, cdiv1.Succeeded, targetPvc.Name)
	Expect(err).ToNot(HaveOccurred())

	By("Verify the content")
	md5Match, err := f.VerifyTargetPVCContentMD5(targetNs, targetPvc, filePath, expectedMD5)
	Expect(err).ToNot(HaveOccurred())
	Expect(md5Match).To(BeTrue())

	if utils.DefaultStorageCSIRespectsFsGroup && sourcePvcDiskGroup != "" {
		// CSI storage class, it should respect fsGroup
		By("Checking that disk image group is qemu")
		Expect(f.GetDiskGroup(targetNs, targetPvc, false)).To(Equal(sourcePvcDiskGroup))
	}
	By("Verifying permissions are 660")
	Expect(f.VerifyPermissions(targetNs, targetPvc)).To(BeTrue(), "Permissions on disk image are not 660")
	By("Deleting verifier pod")
	err = utils.DeleteVerifierPod(f.K8sClient, targetNs.Name)
	Expect(err).ToNot(HaveOccurred())

	dv, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	validateCloneType(f, dv)

	sns := dv.Spec.Source.PVC.Namespace
	if sns == "" {
		sns = dv.Namespace
	}

	switch utils.GetCloneType(f.CdiClient, dv) {
	case "snapshot":
		snapshots := &snapshotv1.VolumeSnapshotList{}
		err = f.CrClient.List(context.TODO(), snapshots, &client.ListOptions{Namespace: sns})
		Expect(err).ToNot(HaveOccurred())
		for _, s := range snapshots.Items {
			Expect(s.DeletionTimestamp).ToNot(BeNil())
		}
		fallthrough
	case "csi-clone":
		if sns != dv.Namespace {
			tmpName := fmt.Sprintf("cdi-tmp-%s", dv.UID)
			tmpPvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(sns).Get(context.TODO(), tmpName, metav1.GetOptions{})
			if err != nil {
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			} else {
				Expect(tmpPvc.DeletionTimestamp).ToNot(BeNil())
			}

			Eventually(func() []string {
				tmp, err := f.CdiClient.CdiV1beta1().DataVolumes(targetNs.Name).Get(context.TODO(), dv.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return tmp.Finalizers
			}, 90*time.Second, 2*time.Second).Should(BeEmpty())

			Eventually(func() bool {
				ot, err := f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), tmpName, metav1.GetOptions{})
				if err != nil {
					Expect(k8serrors.IsNotFound(err)).To(BeTrue())
					return true
				}
				return ot.DeletionTimestamp != nil
			}, 90*time.Second, 2*time.Second).Should(BeTrue())
		}
	case "copy":
		if sns != dv.Namespace {
			s, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), "cdi-api-signing-key", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			bytes, ok := s.Data["id_rsa.pub"]
			Expect(ok).To(BeTrue())
			objs, err := cert.ParsePublicKeysPEM(bytes)
			Expect(err).ToNot(HaveOccurred())
			Expect(objs).To(HaveLen(1))
			v := token.NewValidator("cdi-deployment", objs[0].(*rsa.PublicKey), time.Minute)

			By("checking long token added")
			Eventually(func(g Gomega) bool {
				pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
				g.Expect(err).ToNot(HaveOccurred())
				t, ok := pvc.Annotations[controller.AnnExtendedCloneToken]
				if !ok {
					return false
				}
				_, err = v.Validate(t)
				g.Expect(err).ToNot(HaveOccurred())
				return true
			}, 10*time.Second, assertionPollInterval).Should(BeTrue())
		}
	}
}

func cloneOfAnnoExistenceTest(f *framework.Framework, targetNamespaceName string) {
	beforeClone := time.Now()

	// Create targetPvc
	By(fmt.Sprintf("Creating target pvc: %s/target-pvc", targetNamespaceName))
	pvcName := "target-pvc"
	targetPvc, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNamespaceName, utils.NewPVCDefinition(
		pvcName,
		"1Gi",
		map[string]string{
			controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName,
			controller.AnnCloneOf:      "true",
			controller.AnnPodPhase:     "Succeeded",
		},
		nil))
	Expect(err).ToNot(HaveOccurred())
	f.ForceBindIfWaitForFirstConsumer(targetPvc)

	By("Checking no cloning pods were created")
	Eventually(func() (string, error) {
		out, err := f.K8sClient.CoreV1().
			Pods(f.CdiInstallNs).
			GetLogs(f.ControllerPod.Name, &corev1.PodLogOptions{SinceTime: &metav1.Time{Time: beforeClone}}).
			DoRaw(context.Background())
		return string(out), err
	}, time.Minute, time.Second).Should(And(
		ContainSubstring(fmt.Sprintf(`{"PVC": {"name":%q,"namespace":%q}, "isUpload": false, "isCloneTarget": true, "isBound": true, "podSucceededFromPVC": true, "deletionTimeStamp set?": false}`, pvcName, f.Namespace.Name)),
		ContainSubstring(fmt.Sprintf(`{"PVC": {"name":%q,"namespace":%q}, "checkPVC(AnnCloneRequest)": true, "NOT has annotation(AnnCloneOf)": false, "isBound": true, "has finalizer?": false}`, pvcName, targetNamespaceName)),
	))
}

func cleanDv(f *framework.Framework, dv *cdiv1.DataVolume) {
	if dv != nil {
		err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dv.Name)
		Expect(err).ToNot(HaveOccurred())
	}
}

func validateCloneType(f *framework.Framework, dv *cdiv1.DataVolume) {
	if dv.Spec.Source == nil || dv.Spec.Source.PVC == nil {
		return
	}

	cloneType := "copy"
	if f.IsSnapshotStorageClassAvailable() {
		sourceNamespace := dv.Namespace
		if dv.Spec.Source.PVC.Namespace != "" {
			sourceNamespace = dv.Spec.Source.PVC.Namespace
		}

		sourcePVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(sourceNamespace).Get(context.TODO(), dv.Spec.Source.PVC.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		targetPVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		isCrossNamespaceClone := sourcePVC.Namespace != targetPVC.Namespace
		usesPopulator := targetPVC.Spec.DataSourceRef != nil && targetPVC.Spec.DataSourceRef.Kind == "VolumeCloneSource"

		if sourcePVC.Spec.StorageClassName != nil {
			storageProfile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), *sourcePVC.Spec.StorageClassName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			defaultCloneStrategy := cdiv1.CloneStrategySnapshot
			cloneStrategy := &defaultCloneStrategy
			if strategy := storageProfile.Status.CloneStrategy; strategy != nil {
				cloneStrategy = strategy
			}

			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			allowsExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
			bindingMode := storagev1.VolumeBindingImmediate
			if sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
				bindingMode = storagev1.VolumeBindingWaitForFirstConsumer
			}

			if *cloneStrategy == cdiv1.CloneStrategySnapshot &&
				sourcePVC.Spec.StorageClassName != nil &&
				targetPVC.Spec.StorageClassName != nil &&
				*sourcePVC.Spec.StorageClassName == *targetPVC.Spec.StorageClassName &&
				*sourcePVC.Spec.StorageClassName == f.SnapshotSCName &&
				(!isCrossNamespaceClone || bindingMode == storagev1.VolumeBindingImmediate || usesPopulator) &&
				(allowsExpansion || sourcePVC.Status.Capacity.Storage().Cmp(*targetPVC.Status.Capacity.Storage()) == 0) {
				cloneType = "snapshot"
			}
			if *cloneStrategy == cdiv1.CloneStrategyCsiClone &&
				sourcePVC.Spec.StorageClassName != nil &&
				targetPVC.Spec.StorageClassName != nil &&
				*sourcePVC.Spec.StorageClassName == *targetPVC.Spec.StorageClassName &&
				*sourcePVC.Spec.StorageClassName == f.CsiCloneSCName &&
				(!isCrossNamespaceClone || bindingMode == storagev1.VolumeBindingImmediate || usesPopulator) &&
				(allowsExpansion || sourcePVC.Status.Capacity.Storage().Cmp(*targetPVC.Status.Capacity.Storage()) == 0) {
				cloneType = "csi-clone"
			}
		}
	}

	Expect(utils.GetCloneType(f.CdiClient, dv)).To(Equal(cloneType))
}

// VerifyGC verifies DV is garbage collected
func VerifyGC(f *framework.Framework, dvName, dvNamespace string, checkOwnerRefs bool, config *cdiv1.CDIConfig) {
	By("Wait for DV to be in phase succeeded")
	err := utils.WaitForDataVolumePhase(f, dvNamespace, cdiv1.Succeeded, dvName)
	Expect(err).ToNot(HaveOccurred(), "DV is not in phase succeeded in time")

	By("Wait for DV to be garbage collected")
	Eventually(func() bool {
		_, err := f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
		return k8serrors.IsNotFound(err)
	}, timeout, pollingInterval).Should(BeTrue())

	By("Verify PVC still exists")
	pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	if checkOwnerRefs && config != nil {
		By("Verify PVC gets DV original OwnerReferences, and the DV reference is removed")
		Expect(pvc.OwnerReferences).Should(HaveLen(1))
		Expect(pvc.OwnerReferences[0].UID).Should(Equal(config.UID))
	}
}

// VerifyNoGC verifies DV is not garbage collected
func VerifyNoGC(f *framework.Framework, dvName, dvNamespace string) {
	By("Verify DV is not garbage collected")
	matchString := fmt.Sprintf("DataVolume is not annotated to be garbage collected\t{\"DataVolume\": {\"name\":\"%s\",\"namespace\":\"%s\"}}", dvName, dvNamespace)
	Eventually(func() (string, error) {
		out, err := f.K8sClient.CoreV1().
			Pods(f.CdiInstallNs).
			GetLogs(f.ControllerPod.Name, &corev1.PodLogOptions{SinceTime: &metav1.Time{Time: CurrentSpecReport().StartTime}}).
			DoRaw(context.Background())
		return string(out), err
	}, timeout, pollingInterval).Should(ContainSubstring(matchString))

	dv, err := f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(dv.Annotations[controller.AnnDeleteAfterCompletion]).ToNot(Equal("true"))
}

// VerifyDisabledGC verifies DV is not deleted when garbage collection is disabled
func VerifyDisabledGC(f *framework.Framework, dvName, dvNamespace string) {
	By("Verify DV is not deleted when garbage collection is disabled")
	matchString := fmt.Sprintf("Garbage Collection is disabled\t{\"DataVolume\": {\"name\":%q,\"namespace\":%q}}", dvName, dvNamespace)
	Eventually(func() (string, error) {
		out, err := f.K8sClient.CoreV1().
			Pods(f.CdiInstallNs).
			GetLogs(f.ControllerPod.Name, &corev1.PodLogOptions{SinceTime: &metav1.Time{Time: CurrentSpecReport().StartTime}}).
			DoRaw(context.Background())
		return string(out), err
	}, timeout, pollingInterval).Should(ContainSubstring(matchString))

	_, err := f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
}

// EnableGcAndAnnotateLegacyDv enables garbage collection, annotates the DV and verifies it is garbage collected
func EnableGcAndAnnotateLegacyDv(f *framework.Framework, dvName, dvNamespace string) {
	By("Enable Garbage Collection")
	SetConfigTTL(f, 0)

	dv, err := f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	_, ok := dv.Annotations[controller.AnnDeleteAfterCompletion]
	Expect(ok).To(BeFalse())

	By("Add empty DeleteAfterCompletion annotation to DV for reconcile")
	Eventually(func() error {
		dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		controller.AddAnnotation(dv, controller.AnnDeleteAfterCompletion, "")
		// We shouldn't make the test fail if there's a conflict with the update request.
		// These errors are usually transient and should be fixed in subsequent retries.
		dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Update(context.TODO(), dv, metav1.UpdateOptions{})
		return err
	}, timeout, pollingInterval).Should(Succeed())

	VerifyNoGC(f, dvName, dvNamespace)

	By("Add true DeleteAfterCompletion annotation to DV")
	Eventually(func() error {
		dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		controller.AddAnnotation(dv, controller.AnnDeleteAfterCompletion, "true")
		dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Update(context.TODO(), dv, metav1.UpdateOptions{})
		return err
	}, timeout, pollingInterval).Should(Succeed())

	VerifyGC(f, dvName, dvNamespace, false, nil)
}

// SetConfigTTL set CDIConfig DataVolumeTTLSeconds
func SetConfigTTL(f *framework.Framework, ttl int) {
	By(fmt.Sprintf("Set DataVolumeTTLSeconds to %d", ttl))
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.DataVolumeTTLSeconds = ptr.To(int32(ttl))
	})
	Expect(err).ToNot(HaveOccurred())
}
