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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cont "kubevirt.io/containerized-data-importer/pkg/controller"
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
)

var _ = Describe("all clone tests", func() {
	var _ = Describe("[rfe_id:1277][crit:high][vendor:cnv-qe@redhat.com][level:component]Cloner Test Suite", func() {
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
			smartApplicable := f.IsSnapshotStorageClassAvailable()
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			if err == nil {
				value, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]
				if smartApplicable && ok && strings.Compare(value, "true") == 0 {
					Skip("Cannot test host assisted cloning for within namespace when all pvcs are smart clone capable.")
				}
			}

			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
			By(fmt.Sprintf("Create new datavolume %s", dataVolume.Name))
			dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
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
			utils.WaitForDataVolumePhaseWithTimeout(f, targetDataVolume.Namespace, cdiv1.Succeeded, targetDV.Name, cloneCompleteTimeout)

			By("Find cloner source pod after completion")
			cloner, err := utils.FindPodBySuffixOnce(f.K8sClient, targetDataVolume.Namespace, common.ClonerSourcePodNameSuffix, common.CDILabelSelector)
			Expect(err).ToNot(HaveOccurred())
			Expect(cloner.DeletionTimestamp).To(BeNil())

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
				utils.WaitForDataVolumePhaseWithTimeout(f, targetDataVolume.Namespace, cdiv1.Succeeded, targetDV.Name, cloneCompleteTimeout)
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
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDV.Name, 3*90*time.Second)
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

				targetDS := utils.NewDataSource("test-datasource", pvc.Namespace, pvc.Name, pvc.Namespace)
				By(fmt.Sprintf("Create new datasource %s", targetDS.Name))
				targetDataSource, err := f.CdiClient.CdiV1beta1().DataSources(pvc.Namespace).Create(context.TODO(), targetDS, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeWithSourceRef("target-dv", "1Gi", targetDataSource.Namespace, targetDataSource.Name)
				By(fmt.Sprintf("Create new target datavolume %s", targetDV.Name))
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)

				By("Wait for target datavolume phase Succeeded")
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDV.Name, 3*90*time.Second)
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

			It("[posneg:negative][test_id:6612]Clone with CSI as PVC source with target name that already exists", func() {
				if cloneType == "network" {
					Skip("Cannot simulate target pvc name conflict for host-assisted clone ")
				}
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				targetNamespaceName := f.Namespace.Name

				// 1. use the srcPvc so the clone cannot be started
				pod, err := f.CreateExecutorPodWithPVC("temp-pod", f.Namespace.Name, sourcePvc)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					pod, err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), pod.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return pod.Status.Phase == v1.PodRunning
				}, 90*time.Second, 2*time.Second).Should(BeTrue())

				// 2. Create a clone DataVolume
				targetDV := utils.NewCloningDataVolume("target-pvc", "1Gi", sourcePvc)
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNamespaceName, targetDV)
				Expect(err).ToNot(HaveOccurred())

				actualCloneType := utils.GetCloneType(f.CdiClient, dataVolume)
				if actualCloneType == "snapshot" {
					f.ExpectEvent(targetNamespaceName).Should(ContainSubstring(dvc.SmartCloneSourceInUse))
				} else if actualCloneType == "csivolumeclone" {
					f.ExpectEvent(targetNamespaceName).Should(ContainSubstring(dvc.CSICloneSourceInUse))
				} else {
					Fail(fmt.Sprintf("Unknown clonetype %s", actualCloneType))
				}

				// 3. Knowing that clone cannot yet advance, Create targetPvc with a "conflicting name"
				By(fmt.Sprintf("Creating target pvc: %s/target-pvc", targetNamespaceName))
				annotations := map[string]string{"cdi.kubevirt.io/conflicting-pvc": dataVolumeName}

				targetPvc, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNamespaceName,
					utils.NewPVCDefinition("target-pvc", "1Gi", annotations, nil))
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindIfWaitForFirstConsumer(targetPvc)

				err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				//verify event
				f.ExpectEvent(targetNamespaceName).Should(ContainSubstring(dvc.ErrResourceExists))
			})

			It("[test_id:1356]Should not clone anything when CloneOf annotation exists", func() {
				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)
				cloneOfAnnoExistenceTest(f, f.Namespace.Name)
			})

			It("[posneg:negative][test_id:3617]Should clone across nodes when multiple local filesystem volumes exist,", func() {
				// Get nodes, need at least 2
				nodeList, err := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				if len(nodeList.Items) < 2 {
					Skip("Need at least 2 nodes to copy accross nodes")
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
							sourcePV = &pv
							if sourcePV.GetLabels() == nil {
								sourcePV.SetLabels(make(map[string]string))
							}
							sourcePV.GetLabels()["source-pv"] = "yes"
							sourcePV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), sourcePV, metav1.UpdateOptions{})
							Expect(err).ToNot(HaveOccurred())
						}
					} else if targetPV == nil {
						pvNode := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
						if ok, val := nodeMap[pvNode]; ok && val {
							nodeMap[pvNode] = false
							By("Labeling PV " + pv.Name + " as target")
							targetPV = &pv
							if targetPV.GetLabels() == nil {
								targetPV.SetLabels(make(map[string]string))
							}
							targetPV.GetLabels()["target-pv"] = "yes"
							targetPV, err = f.K8sClient.CoreV1().PersistentVolumes().Update(context.TODO(), targetPV, metav1.UpdateOptions{})
							Expect(err).ToNot(HaveOccurred())
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
				utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
				completeClone(f, f.Namespace, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, "")
			})

			It("[test_id:cnv-5569]Should clone data from filesystem to block", func() {
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csivolumeclone" || cloneType == "snapshot" {
					Skip("csivolumeclone only works for the same volumeMode")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeCloneToBlockPV("target-dv", "1Gi", sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Wait for target PVC Bound phase")
				utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
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
				Expect(sourceMD5 == targetMD5).To(BeTrue())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:cnv-5570]Should clone data from block to filesystem", func() {
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csivolumeclone" {
					Skip("csivolumeclone only works for the same volumeMode")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImportToBlockPV(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs), f.BlockSCName)
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				volumeMode := v1.PersistentVolumeMode(v1.PersistentVolumeFilesystem)
				targetDV := utils.NewDataVolumeForImageCloning("target-dv", "1.1Gi", sourcePvc.Namespace, sourcePvc.Name, nil, &volumeMode)
				targetDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDataVolume)
				targetPvc, err := utils.WaitForPVC(f.K8sClient, targetDataVolume.Namespace, targetDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Wait for target PVC Bound phase")
				utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
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
				Expect(sourceMD5 == targetMD5).To(BeTrue())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("bz:2079781 Should clone data from filesystem to block, when using storage API ", func() {
				SetFilesystemOverhead(f, "0.50", "0.50")
				if !f.IsBlockVolumeStorageClassAvailable() {
					Skip("Storage Class for block volume is not available")
				}
				if cloneType == "csivolumeclone" || cloneType == "snapshot" {
					Skip("csivolumeclone only works for the same volumeMode")
				}
				dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "2Gi", fmt.Sprintf(utils.LargeVirtualDiskQcow, f.CdiInstallNs))
				filesystem := v1.PersistentVolumeFilesystem
				dataVolume.Spec.Storage.VolumeMode = &filesystem

				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
				sourcePvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				targetDV := utils.NewDataVolumeCloneToBlockPVStorageAPI("target-dv", "2Gi", sourcePvc.Namespace, sourcePvc.Name, f.BlockSCName)
				targetDV.Spec.Storage.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadWriteMany}

				tagretDataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				targetPvc, err := utils.WaitForPVC(f.K8sClient, tagretDataVolume.Namespace, tagretDataVolume.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Wait for target PVC Bound phase")
				utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)
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
				Expect(sourceMD5 == targetMD5).To(BeTrue())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should clone data from fs to fs while using calculated storage size", func() {
				// should clone from fs to fs using the same size in spec.storage.size
				// source pvc might be bigger than the size, but the clone should work
				// as the actual data is the same
				volumeMode := v1.PersistentVolumeMode(v1.PersistentVolumeFilesystem)

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
				Expect(sourceMD5 == targetMD5).To(BeTrue())
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
				utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

				By("Cloning from the source DataVolume to under sized target")
				targetDv := utils.NewDataVolumeForImageCloningAndStorageSpec("target-dv", "100Mi",
					f.Namespace.Name,
					sourceDv.Name,
					sourceDv.Spec.PVC.StorageClassName,
					sourceDv.Spec.PVC.VolumeMode)

				targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
				Expect(err).ToNot(HaveOccurred())
				// As of now, csi and smart clone check the values before the pvc is created, and the network clone
				// checks it in the upload phase, so it needs a PVC, this might be improved
				if cloneType == "network" {
					f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)
				}

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
				Expect(err).To(BeNil())
				Expect(md5Match).To(BeTrue())
			})

			DescribeTable("Should clone with empty volume size without using size-detection pod",
				func(sourceVolumeMode, targetVolumeMode v1.PersistentVolumeMode) {
					// When cloning without defining the target's storage size, the source's size can be attainable
					// by different means depending on the clone type and the volume mode used.
					// Either if "block" is used as volume mode or smart/csi cloning is used as clone strategy,
					// the value is simply extracted from the original PVC's spec.

					var sourceSCName string
					var targetSCName string
					targetDiskImagePath := filepath.Join(testBaseDir, testFile)
					sourceDiskImagePath := filepath.Join(testBaseDir, testFile)

					if cloneType == "network" && sourceVolumeMode == v1.PersistentVolumeFilesystem {
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

					// We attempt to create the sizeless DV
					targetDV := utils.NewDataVolumeForCloningWithEmptySize("target-dv", sourcePvc.Namespace, sourcePvc.Name, nil, &targetVolumeMode)
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
				Entry("[test_id:8492]Block to block (empty storage size)", v1.PersistentVolumeBlock, v1.PersistentVolumeBlock),
				Entry("[test_id:8491]Block to filesystem (empty storage size)", v1.PersistentVolumeBlock, v1.PersistentVolumeFilesystem),
				Entry("[test_id:8490]Filesystem to filesystem(empty storage size)", v1.PersistentVolumeFilesystem, v1.PersistentVolumeFilesystem),
			)

			Context("WaitForFirstConsumer status with advanced cloning methods", func() {
				var wffcStorageClass *storagev1.StorageClass

				BeforeEach(func() {
					if cloneType != "csivolumeclone" && cloneType != "snapshot" {
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
						if cloneType == "csivolumeclone" {
							utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, wffcStorageClass.Name, spec, cdiv1.CloneStrategyCsiClone)
						} else if cloneType == "snapshot" {
							utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, wffcStorageClass.Name, spec, cdiv1.CloneStrategySnapshot)
						}
					}
				})

				AfterEach(func() {
					if wffcStorageClass != nil {
						err := f.K8sClient.StorageV1().StorageClasses().Delete(context.TODO(), wffcStorageClass.Name, metav1.DeleteOptions{})
						Expect(err).ToNot(HaveOccurred())
						Eventually(func() bool {
							_, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), wffcStorageClass.Name, metav1.GetOptions{})
							return err != nil && k8serrors.IsNotFound(err)
						}, time.Minute, time.Second).Should(BeTrue())
					}
				})

				It("should report correct status for smart/CSI clones", func() {
					volumeMode := v1.PersistentVolumeMode(v1.PersistentVolumeFilesystem)

					dataVolume := utils.NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName, "1Gi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
					dataVolume.Spec.Storage.VolumeMode = &volumeMode
					if wffcStorageClass != nil {
						dataVolume.Spec.Storage.StorageClassName = &wffcStorageClass.Name
					}
					dataVolume.Annotations[cont.AnnImmediateBinding] = "true"
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
					err = utils.WaitForDataVolumePhase(f, targetDataVolume.Namespace, cdiv1.WaitForFirstConsumer, targetDataVolume.Name)
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
				utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyHostAssisted)

			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)
			})
			ClonerBehavior(cloneStorageClassName, "network")
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
			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)
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
				utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyCsiClone)
			})
			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)
			})
			ClonerBehavior(cloneStorageClassName, "csivolumeclone")
		})

		// The size-detection pod is only used in cloning when three requirements are met:
		// 	1. The clone manifest is created without defining a storage size.
		//	2. 'Filesystem' is used as volume mode.
		//	3. 'HostAssisted' is used as clone strategy.
		Context("Clone with empty size using the size-detection pod", func() {
			diskImagePath := filepath.Join(testBaseDir, testFile)
			volumeMode := v1.PersistentVolumeMode(v1.PersistentVolumeFilesystem)

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
				utils.ConfigureCloneStrategy(f.CrClient, f.CdiClient, cloneStorageClassName, originalProfileSpec, cdiv1.CloneStrategyHostAssisted)

			})

			AfterEach(func() {
				By("[AfterEach] Restore the profile")
				utils.UpdateStorageProfile(f.CrClient, cloneStorageClassName, *originalProfileSpec)
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
				Expect(available).To(Equal(true))
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
				sourcePvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourcePvc.Namespace).Get(context.TODO(), sourcePvc.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				sourcePvc.Annotations[dvc.AnnSourceCapacity] = "400Mi"
				_, err = f.K8sClient.CoreV1().PersistentVolumeClaims(sourcePvc.Namespace).Update(context.TODO(), sourcePvc, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

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
		})

		Context("CloneStrategy on storageclass annotation", func() {
			cloneType := cdiv1.CloneStrategyCsiClone

			BeforeEach(func() {
				if !f.IsCSIVolumeCloneStorageClassAvailable() {
					Skip("CSI Volume Clone is not applicable")
				}
				cloneStorageClassName = f.CsiCloneSCName
				By(fmt.Sprintf("Get original storage profile: %s", cloneStorageClassName))
				storageProfile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), cloneStorageClassName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(storageProfile.Spec.CloneStrategy).To(BeNil())
				Expect(storageProfile.Status.CloneStrategy).To(BeNil())

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
				}, time.Minute, time.Second).Should(BeNil())
			})
			It("Should clone  with correct strategy from storageclass annotation ", func() {

				pvcDef := utils.NewPVCDefinition(sourcePVCName, "1Gi", nil, nil)
				pvcDef.Spec.StorageClassName = &cloneStorageClassName
				pvcDef.Namespace = f.Namespace.Name

				sourcePvc = f.CreateAndPopulateSourcePVC(pvcDef, sourcePodFillerName, fillCommand+testFile+"; chmod 660 "+testBaseDir+testFile)

				targetDV := utils.NewCloningDataVolume("target-pvc", "1Gi", sourcePvc)

				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDV)
				Expect(err).ToNot(HaveOccurred())
				Expect(utils.GetCloneType(f.CdiClient, dataVolume)).To(Equal("csivolumeclone"))
			})
		})

		Context("Clone without a source PVC", func() {
			diskImagePath := filepath.Join(testBaseDir, testFile)
			fsVM := v1.PersistentVolumeMode(v1.PersistentVolumeFilesystem)
			blockVM := v1.PersistentVolumeMode(v1.PersistentVolumeBlock)

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
				cloneDV := utils.NewDataVolumeForImageCloningAndStorageSpec("clone-dv", "1Mi", f.Namespace.Name, dataVolumeName, nil, &blockVM)
				cloneDV, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, cloneDV)
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
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+utils.DefaultImagePath)
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
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
			retry := 0
			for err != nil && retry < 10 {
				retry++
				md5sum, err = f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
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
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

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

	var _ = Describe("Validate Data Volume clone to smaller size", func() {
		f := framework.NewFramework(namespacePrefix)
		tinyCoreIsoURL := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }

		var (
			sourceDv, targetDv *cdiv1.DataVolume
			err                error
		)

		AfterEach(func() {
			if sourceDv != nil {
				By("Cleaning up source DV")
				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, sourceDv.Name)
				Expect(err).ToNot(HaveOccurred())
			}
			if targetDv != nil {
				By("Cleaning up target DV")
				err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, targetDv.Name)
				Expect(err).ToNot(HaveOccurred())
				if targetDv.Status.Phase == cdiv1.Succeeded {
					validateCloneType(f, targetDv)
				}
			}
		})

		It("[rfe_id:1126][test_id:1896][crit:High][vendor:cnv-qe@redhat.com][level:component] Should not allow cloning into a smaller sized data volume", func() {
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "50Mi", tinyCoreIsoURL())
			filesystem := v1.PersistentVolumeFilesystem
			sourceDv.Spec.PVC.VolumeMode = &filesystem
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			fmt.Fprintf(GinkgoWriter, "sourceDv %v", sourceDv)

			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.GetMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(sourceDv), utils.DefaultImagePath, 0)
			Expect(err).ToNot(HaveOccurred())
			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Cloning from the source DataVolume to under sized target")
			targetDv = utils.NewDataVolumeForImageCloning("target-dv-too-small", "15Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "target resources requests storage size is smaller than the source")).To(BeTrue())

			By("Cloning from the source DataVolume to properly sized target")
			targetDv = utils.NewDataVolumeForImageCloning("target-dv", "50Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)

			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv), utils.DefaultImagePath, md5sum[:32])).To(BeTrue())
			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying the image is sparse")
			Expect(f.VerifySparse(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv), utils.DefaultImagePath)).To(BeTrue())
			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[rfe_id:1126][test_id:1896][crit:High][vendor:cnv-qe@redhat.com][level:component] Should not allow cloning into a smaller sized data volume in block volume mode", func() {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL(), f.BlockSCName)
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

			By("Cloning from the source DataVolume to under sized target")
			targetDv = utils.NewDataVolumeForImageCloning("target-dv", "50Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			_, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "target resources requests storage size is smaller than the source")).To(BeTrue())

			By("Cloning from the source DataVolume to properly sized target")
			targetDv = utils.NewDataVolumeForImageCloning("target-dv", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv)

			By("Waiting for clone to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, targetDv.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(targetDv), testBaseDir, md5sum[:32])).To(BeTrue())
			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
			Expect(err).ToNot(HaveOccurred())
		})

	})

	var _ = Describe("Validate Data Volume should clone multiple clones in parallel", func() {
		f := framework.NewFramework(namespacePrefix)
		tinyCoreIsoURL := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }

		var (
			sourceDv, targetDv1, targetDv2, targetDv3 *cdiv1.DataVolume
			err                                       error
		)

		AfterEach(func() {
			dvs := []*cdiv1.DataVolume{sourceDv, targetDv1, targetDv2, targetDv3}
			for _, dv := range dvs {
				cleanDv(f, dv)
				if dv != nil && dv.Status.Phase == cdiv1.Succeeded {
					validateCloneType(f, dv)
				}
			}
		})

		It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel", func() {
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImport("source-dv", "200Mi", tinyCoreIsoURL())
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+utils.DefaultImagePath)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

			err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), "execute-command", metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitPodDeleted(f.K8sClient, "execute-command", f.Namespace.Name, verifyPodDeletedTimeout)
			Expect(err).ToNot(HaveOccurred())

			// By not waiting for completion, we will start 3 transfers in parallell
			By("Cloning from the source DataVolume to target1")
			targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv1, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv1)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv1)

			By("Cloning from the source DataVolume to target2 in parallel")
			targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv2, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv2)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv2)

			By("Cloning from the source DataVolume to target3 in parallel")
			targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv3, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv3)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv3)

			dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}
			for _, dv := range dvs {
				By("Waiting for clone to be completed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, dv := range dvs {
				By("Verifying MD5 sum matches")
				matchFile := filepath.Join(testBaseDir, "disk.img")
				Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), matchFile, md5sum[:32])).To(BeTrue())
				By("Deleting verifier pod")
				err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), utils.VerifierPodName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				_, err = utils.WaitPodDeleted(f.K8sClient, utils.VerifierPodName, f.Namespace.Name, verifyPodDeletedTimeout)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("[rfe_id:1277][test_id:1899][crit:High][vendor:cnv-qe@redhat.com][level:component] Should allow multiple cloning operations in parallel for block devices", func() {
			if !f.IsBlockVolumeStorageClassAvailable() {
				Skip("Storage Class for block volume is not available")
			}
			By("Creating a source from a real image")
			sourceDv = utils.NewDataVolumeWithHTTPImportToBlockPV("source-dv", "200Mi", tinyCoreIsoURL(), f.BlockSCName)
			sourceDv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, sourceDv)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(sourceDv)

			By("Waiting for import to be completed")
			utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, sourceDv.Name, 3*90*time.Second)

			By("Calculating the md5sum of the source data volume")
			md5sum, err := f.RunCommandAndCaptureOutput(utils.PersistentVolumeClaimFromDataVolume(sourceDv), "md5sum "+testBaseDir)
			Expect(err).ToNot(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "INFO: MD5SUM for source is: %s\n", md5sum[:32])

			// By not waiting for completion, we will start 3 transfers in parallell
			By("Cloning from the source DataVolume to target1")
			targetDv1 = utils.NewDataVolumeForImageCloning("target-dv1", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv1, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv1)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv1)

			By("Cloning from the source DataVolume to target2 in parallel")
			targetDv2 = utils.NewDataVolumeForImageCloning("target-dv2", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv2, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv2)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv2)

			By("Cloning from the source DataVolume to target3 in parallel")
			targetDv3 = utils.NewDataVolumeForImageCloning("target-dv3", "200Mi", f.Namespace.Name, sourceDv.Name, sourceDv.Spec.PVC.StorageClassName, sourceDv.Spec.PVC.VolumeMode)
			targetDv3, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, targetDv3)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindPvcIfDvIsWaitForFirstConsumer(targetDv3)

			dvs := []*cdiv1.DataVolume{targetDv1, targetDv2, targetDv3}
			for _, dv := range dvs {
				By("Waiting for clone to be completed")
				err = utils.WaitForDataVolumePhaseWithTimeout(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name, 3*90*time.Second)
				Expect(err).ToNot(HaveOccurred())
			}

			for _, dv := range dvs {
				By("Verifying MD5 sum matches")
				Expect(f.VerifyTargetPVCContentMD5(f.Namespace, utils.PersistentVolumeClaimFromDataVolume(dv), testBaseDir, md5sum[:32])).To(BeTrue())
				By("Deleting verifier pod")
				err = utils.DeleteVerifierPod(f.K8sClient, f.Namespace.Name)
				Expect(err).ToNot(HaveOccurred())
			}

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

			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())

			fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
			utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetPvc.Name)

			err = utils.WaitForDataVolumePhaseWithTimeout(f, targetNs.Name, cdiv1.Succeeded, "target-dv", 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.VerifyTargetPVCContentMD5(targetNs, targetPvc, testBaseDir, sourceMD5, ss.Value())).To(BeTrue())
			By("Deleting verifier pod")
			err = utils.DeleteVerifierPod(f.K8sClient, targetNs.Name)
			Expect(err).ToNot(HaveOccurred())

			validateCloneType(f, dataVolume)

			targetPvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(targetPvc.Namespace).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			es := resource.MustParse(targetSize)
			Expect(es.Cmp(*targetPvc.Status.Capacity.Storage()) <= 0).To(BeTrue())
		},
			Entry("with same target size", "500M"),
			Entry("with bigger target", "1Gi"),
		)
	})

	var _ = Describe("Namespace with quota", func() {
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

			By("Verify Quota was exceeded in logs")
			matchString := "\\\"cdi-upload-target-dv\\\" is forbidden: exceeded quota: test-quota, requested"
			Eventually(func() string {
				log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
				Expect(err).NotTo(HaveOccurred())
				return log
			}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv"),
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, f.Namespace.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			msg := fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv")
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(msg))
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

			By("Verify Quota was exceeded in logs")
			matchString := "\\\"cdi-upload-target-dv\\\" is forbidden: exceeded quota: test-quota, requested"
			Eventually(func() string {
				log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
				Expect(err).NotTo(HaveOccurred())
				return log
			}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv"),
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, f.Namespace.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			msg := fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv")
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(msg))
			f.ExpectEvent(f.Namespace.Name).Should(ContainSubstring(controller.ErrExceededQuota))

			err = f.UpdateQuotaInNs(int64(1), int64(512*1024*1024), int64(4), int64(512*1024*1024))
			Expect(err).NotTo(HaveOccurred())
			utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, f.Namespace.Name, v1.ClaimBound, targetDV.Name)
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

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "network" {
				Skip("only valid for network clone")
			}

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Verify Quota was exceeded in logs")
			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("\\\"%s-source-pod\\\" is forbidden: exceeded quota: test-quota, requested", targetPvc.GetUID())
			Eventually(func() string {
				log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
				Expect(err).NotTo(HaveOccurred())
				return log
			}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

			podName := fmt.Sprintf("%s-source-pod", targetPvc.GetUID())
			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: fmt.Sprintf(controller.MessageErrStartingPod, podName),
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, targetNs.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			msg := fmt.Sprintf(controller.MessageErrStartingPod, podName)
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring(msg))
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

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "network" {
				Skip("only valid for network clone")
			}

			f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

			By("Verify Quota was exceeded in logs")
			matchString := strings.Trim(fmt.Sprintf("\"namespace\": \"%s\", \"error\": \"pods \\\"cdi-upload-target-dv\\\" is forbidden: exceeded quota: test-quota, requested", targetNs.Name), " ")
			Eventually(func() string {
				log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
				Expect(err).NotTo(HaveOccurred())
				return strings.Trim(log, " ")
			}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(ContainSubstring(matchString))

			expectedCondition := &cdiv1.DataVolumeCondition{
				Type:    cdiv1.DataVolumeRunning,
				Status:  v1.ConditionFalse,
				Message: fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv"),
				Reason:  controller.ErrExceededQuota,
			}

			By("Verify target DV has 'false' as running condition")
			utils.WaitForConditions(f, targetDV.Name, targetNs.Name, timeout, pollingInterval, expectedCondition)

			By("Check the expected event")
			msg := fmt.Sprintf(controller.MessageErrStartingPod, "cdi-upload-target-dv")
			f.ExpectEvent(targetNs.Name).Should(ContainSubstring(msg))
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

		It("[test_id:3999] Create a data volume and then clone it and verify retry count", func() {
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
			if cloneType != "network" {
				Skip("only valid for network clone")
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

			cloneType := utils.GetCloneType(f.CdiClient, dataVolume)
			if cloneType != "network" {
				Skip("only valid for network clone")
			}

			targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(targetPvc)

			fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
			utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)

			By("Wait for upload pod")
			err = utils.WaitTimeoutForPodReadyPollPeriod(f.K8sClient, utils.UploadPodName(targetPvc), targetNs.Name, utils.PodWaitIntervalFast, utils.PodWaitForTime)
			Expect(err).ToNot(HaveOccurred())

			By("Kill upload pod to force error")
			// exit code 137 = 128 + 9, it means parent process issued kill -9, in our case it is not a problem
			_, _, err = f.ExecShellInPod(utils.UploadPodName(targetPvc), targetNs.Name, "kill 1")
			Expect(err).To(Or(
				BeNil(),
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

		It("[test_id:4276] Clone datavolume with short name", func() {
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
			if cloneType == "network" {
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

		It("[test_id:4277] Clone datavolume with long name", func() {
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
			if cloneType == "network" {
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

		It("[test_id:4278] Clone datavolume with long name including special character '.'", func() {
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
			if cloneType == "network" {
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
	})

	var _ = Describe("Preallocation", func() {
		f := framework.NewFramework(namespacePrefix)

		var sourcePvc *v1.PersistentVolumeClaim
		var targetPvc *v1.PersistentVolumeClaim
		var cdiCr cdiv1.CDI
		var cdiCrSpec *cdiv1.CDISpec

		BeforeEach(func() {
			By("[BeforeEach] Saving CDI CR spec")
			crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(crList.Items)).To(Equal(1))

			cdiCrSpec = crList.Items[0].Spec.DeepCopy()
			cdiCr = crList.Items[0]

			By("[BeforeEach] Forcing Host Assisted cloning")
			var cloneStrategy cdiv1.CDICloneStrategy = cdiv1.CloneStrategyHostAssisted
			cdiCr.Spec.CloneStrategyOverride = &cloneStrategy
			_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), &cdiCr, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			err = utils.WaitForCDICrCloneStrategy(f.CdiClient, cloneStrategy)
			Expect(err).ToNot(HaveOccurred())
		})

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

			By("[AfterEach] Restoring CDI CR spec to original state")
			crList, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(crList.Items)).To(Equal(1))

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
				f.PrintControllerLog()
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(preallocationAnnotationFound).To(BeTrue())
			Expect(annValue).To(Equal("true"))

			fmt.Fprintf(GinkgoWriter, "INFO: wait for target DV phase Succeeded: %s\n", targetPvc.Name)

			annValue, preallocationAnnotationFound, err = utils.WaitForPVCAnnotation(f.K8sClient, targetDataVolume.Namespace, targetPvc, controller.AnnPreallocationApplied)
			if err != nil {
				f.PrintControllerLog()
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(preallocationAnnotationFound).To(BeTrue())
			Expect(annValue).To(Equal("true"))
		})
	})
})

func doFileBasedCloneTest(f *framework.Framework, srcPVCDef *v1.PersistentVolumeClaim, targetNs *v1.Namespace, targetDv string, targetSize ...string) {
	if len(targetSize) == 0 {
		targetSize = []string{"1Gi"}
	}
	// Create targetPvc in new NS.
	targetDV := utils.NewCloningDataVolume(targetDv, targetSize[0], srcPVCDef)
	dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, targetNs.Name, targetDV)
	Expect(err).ToNot(HaveOccurred())

	targetPvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
	Expect(err).ToNot(HaveOccurred())
	f.ForceBindIfWaitForFirstConsumer(targetPvc)

	fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)
	sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, srcPVCDef, true)
	fmt.Fprintf(GinkgoWriter, "INFO: %s\n", sourcePvcDiskGroup)
	Expect(err).ToNot(HaveOccurred())

	completeClone(f, targetNs, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, sourcePvcDiskGroup)

	targetPvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(targetPvc.Namespace).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	es := resource.MustParse(targetSize[0])
	Expect(es.Cmp(*targetPvc.Status.Capacity.Storage()) <= 0).To(BeTrue())
}

func doInUseCloneTest(f *framework.Framework, srcPVCDef *v1.PersistentVolumeClaim, targetNs *v1.Namespace, targetDv string) {
	pod, err := f.CreateExecutorPodWithPVC("temp-pod", f.Namespace.Name, srcPVCDef)
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
	cloneType := utils.GetCloneType(f.CdiClient, dataVolume)

	if cloneType == "network" {
		targetPvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)

		f.ExpectEvent(targetNs.Name).Should(ContainSubstring(controller.CloneSourceInUse))
		err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())
	} else if cloneType == "snapshot" {
		f.ExpectEvent(targetNs.Name).Should(ContainSubstring(dvc.SmartCloneSourceInUse))
		err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		targetPvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
	} else {
		f.ExpectEvent(targetNs.Name).Should(ContainSubstring(dvc.CSICloneSourceInUse))
		err = f.K8sClient.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		targetPvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
	}

	fmt.Fprintf(GinkgoWriter, "INFO: wait for PVC claim phase: %s\n", targetPvc.Name)
	utils.WaitForPersistentVolumeClaimPhase(f.K8sClient, targetNs.Name, v1.ClaimBound, targetPvc.Name)
	sourcePvcDiskGroup, err := f.GetDiskGroup(f.Namespace, srcPVCDef, true)
	fmt.Fprintf(GinkgoWriter, "INFO: %s\n", sourcePvcDiskGroup)
	Expect(err).ToNot(HaveOccurred())

	completeClone(f, targetNs, targetPvc, filepath.Join(testBaseDir, testFile), fillDataFSMD5sum, sourcePvcDiskGroup)
}

func completeClone(f *framework.Framework, targetNs *v1.Namespace, targetPvc *v1.PersistentVolumeClaim, filePath, expectedMD5, sourcePvcDiskGroup string) {
	By("Verify the clone annotation is on the target PVC")
	_, cloneAnnotationFound, err := utils.WaitForPVCAnnotation(f.K8sClient, targetNs.Name, targetPvc, controller.AnnCloneOf)
	if err != nil {
		f.PrintControllerLog()
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(cloneAnnotationFound).To(BeTrue())

	By("Verify the clone status is success on the target datavolume")
	err = utils.WaitForDataVolumePhase(f, targetNs.Name, cdiv1.Succeeded, targetPvc.Name)
	Expect(err).ToNot(HaveOccurred())

	By("Verify the content")
	md5Match, err := f.VerifyTargetPVCContentMD5(targetNs, targetPvc, filePath, expectedMD5)
	Expect(err).To(BeNil())
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
	case "csivolumeclone":
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
	case "network":
		s, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), "cdi-api-signing-key", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		bytes, ok := s.Data["id_rsa.pub"]
		Expect(ok).To(BeTrue())
		objs, err := cert.ParsePublicKeysPEM(bytes)
		Expect(err).ToNot(HaveOccurred())
		Expect(objs).To(HaveLen(1))
		v := token.NewValidator("cdi-deployment", objs[0].(*rsa.PublicKey), time.Minute)

		By("checking long token added")
		Eventually(func() bool {
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(targetNs.Name).Get(context.TODO(), targetPvc.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			t, ok := pvc.Annotations[controller.AnnExtendedCloneToken]
			if !ok {
				return false
			}
			_, err = v.Validate(t)
			Expect(err).ToNot(HaveOccurred())
			return true
		}, 10*time.Second, assertionPollInterval).Should(BeTrue())
	}
}

func cloneOfAnnoExistenceTest(f *framework.Framework, targetNamespaceName string) {
	// Create targetPvc
	By(fmt.Sprintf("Creating target pvc: %s/target-pvc", targetNamespaceName))
	targetPvc, err := utils.CreatePVCFromDefinition(f.K8sClient, targetNamespaceName, utils.NewPVCDefinition(
		"target-pvc",
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

	matchString := fmt.Sprintf("{\"PVC\": \"%s/target-pvc\", \"isUpload\": false, \"isCloneTarget\": true, \"isBound\": true, \"podSucceededFromPVC\": true, \"deletionTimeStamp set?\": false}", f.Namespace.Name)
	Eventually(func() bool {
		log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return strings.Contains(log, matchString)
	}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	Expect(err).ToNot(HaveOccurred())

	By("Checking logs explicitly skips PVC")
	Eventually(func() bool {
		log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return strings.Contains(log, fmt.Sprintf("{\"PVC\": \"%s/%s\", \"checkPVC(AnnCloneRequest)\": true, \"NOT has annotation(AnnCloneOf)\": false, \"isBound\": true, \"has finalizer?\": false}", targetNamespaceName, "target-pvc"))
	}, controllerSkipPVCCompleteTimeout, assertionPollInterval).Should(BeTrue())
	Expect(err).ToNot(HaveOccurred())
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

	cloneType := "network"
	if f.IsSnapshotStorageClassAvailable() {
		sourceNamespace := dv.Namespace
		if dv.Spec.Source.PVC.Namespace != "" {
			sourceNamespace = dv.Spec.Source.PVC.Namespace
		}

		sourcePVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(sourceNamespace).Get(context.TODO(), dv.Spec.Source.PVC.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		targetPVC, err := f.K8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		if sourcePVC.Spec.StorageClassName != nil {
			spec, err := utils.GetStorageProfileSpec(f.CdiClient, *sourcePVC.Spec.StorageClassName)
			Expect(err).NotTo(HaveOccurred())

			defaultCloneStrategy := cdiv1.CloneStrategySnapshot
			cloneStrategy := &defaultCloneStrategy
			if spec.CloneStrategy != nil {
				cloneStrategy = spec.CloneStrategy
			}

			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			allowsExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion

			if *cloneStrategy == cdiv1.CloneStrategySnapshot &&
				sourcePVC.Spec.StorageClassName != nil &&
				targetPVC.Spec.StorageClassName != nil &&
				*sourcePVC.Spec.StorageClassName == *targetPVC.Spec.StorageClassName &&
				*sourcePVC.Spec.StorageClassName == f.SnapshotSCName &&
				(allowsExpansion || sourcePVC.Status.Capacity.Storage().Cmp(*targetPVC.Status.Capacity.Storage()) == 0) {
				cloneType = "snapshot"
			}
			if *cloneStrategy == cdiv1.CloneStrategyCsiClone &&
				sourcePVC.Spec.StorageClassName != nil &&
				targetPVC.Spec.StorageClassName != nil &&
				*sourcePVC.Spec.StorageClassName == *targetPVC.Spec.StorageClassName &&
				*sourcePVC.Spec.StorageClassName == f.CsiCloneSCName {
				//(allowsExpansion || sourcePVC.Status.Capacity.Storage().Cmp(*targetPVC.Status.Capacity.Storage()) == 0)

				cloneType = "csivolumeclone"
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
	matchString := "DataVolume is not annotated to be garbage collected\t{\"DataVolume\": \"" + dvNamespace + "/" + dvName + "\"}"
	fmt.Fprintf(GinkgoWriter, "INFO: matchString: [%s]\n", matchString)
	Eventually(func() string {
		log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return log
	}, timeout, pollingInterval).Should(ContainSubstring(matchString))
	dv, err := f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(dv.Annotations[controller.AnnDeleteAfterCompletion]).ToNot(Equal("true"))
}

// VerifyDisabledGC verifies DV is not deleted when garbage collection is disabled
func VerifyDisabledGC(f *framework.Framework, dvName, dvNamespace string) {
	By("Verify DV is not deleted when garbage collection is disabled")
	matchString := "Garbage Collection is disabled\t{\"DataVolume\": \"" + dvNamespace + "/" + dvName + "\"}"
	fmt.Fprintf(GinkgoWriter, "INFO: matchString: [%s]\n", matchString)
	Eventually(func() string {
		log, err := f.RunKubectlCommand("logs", f.ControllerPod.Name, "-n", f.CdiInstallNs)
		Expect(err).NotTo(HaveOccurred())
		return log
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
	controller.AddAnnotation(dv, controller.AnnDeleteAfterCompletion, "")
	dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Update(context.TODO(), dv, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	VerifyNoGC(f, dvName, dvNamespace)

	By("Add true DeleteAfterCompletion annotation to DV")
	controller.AddAnnotation(dv, controller.AnnDeleteAfterCompletion, "true")
	dv, err = f.CdiClient.CdiV1beta1().DataVolumes(dvNamespace).Update(context.TODO(), dv, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	VerifyGC(f, dvName, dvNamespace, false, nil)
}

// SetConfigTTL set CDIConfig DataVolumeTTLSeconds
func SetConfigTTL(f *framework.Framework, ttl int) {
	By(fmt.Sprintf("Set DataVolumeTTLSeconds to %d", ttl))
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.DataVolumeTTLSeconds = pointer.Int32(int32(ttl))
	})
	Expect(err).ToNot(HaveOccurred())
}
