package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	metricPollingInterval          = 5 * time.Second
	metricPollingTimeout           = 5 * time.Minute
	metricConsistentPollingTimeout = 2 * time.Minute
)

var _ = Describe("[Destructive] Monitoring Tests", func() {
	f := framework.NewFramework("monitoring-test")

	var (
		cr                      *cdiv1.CDI
		crModified              bool
		cdiPods                 *corev1.PodList
		defaultStorageClass     *storagev1.StorageClass
		defaultVirtStorageClass *storagev1.StorageClass
		defaultCloneStrategy    *cdiv1.CDICloneStrategy
		numAddedStorageClasses  int
	)

	waitForStorageProfileMetricInit := func() {
		Eventually(func() bool {
			scs, err := f.K8sClient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			return len(scs.Items) == countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "complete", "true")
		}, 2*time.Minute, 1*time.Second).Should(BeTrue())
	}

	updateDefaultStorageClasses := func(boolStr string) {
		By(fmt.Sprintf("Update default storage class annotation to %q", boolStr))
		if defaultStorageClass != nil {
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), defaultStorageClass.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			sc.Annotations[cc.AnnDefaultStorageClass] = boolStr
			_, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), sc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}
		if defaultVirtStorageClass != nil {
			sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), defaultVirtStorageClass.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			sc.Annotations[cc.AnnDefaultVirtStorageClass] = boolStr
			_, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), sc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}
	}

	updateDefaultStorageClassProfileClaimPropertySets := func(accessMode *corev1.PersistentVolumeAccessMode) {
		profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), defaultStorageClass.Name, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) && accessMode == nil {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		profile.Spec.ClaimPropertySets = nil
		if accessMode != nil {
			profile.Spec.ClaimPropertySets = []cdiv1.ClaimPropertySet{{
				AccessModes: []corev1.PersistentVolumeAccessMode{*accessMode},
				VolumeMode:  &cc.BlockMode,
			}}
		}
		_, err = f.CdiClient.CdiV1beta1().StorageProfiles().Update(context.TODO(), profile, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	deleteUnknownStorageClasses := func() {
		By("Delete unknown storage classes")
		for i := 0; i < numAddedStorageClasses; i++ {
			name := fmt.Sprintf("unknown-sc-%d", i)
			_, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				continue
			}
			err = f.K8sClient.StorageV1().StorageClasses().Delete(context.TODO(), name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
		}
	}

	createStubSnapshotClass := func(driver string) {
		err := f.CrClient.Create(context.TODO(), newStubSnapshotClass(driver))
		Expect(err).ToNot(HaveOccurred())
		f.SnapshotSCName = defaultStorageClass.Name
	}

	deleteStubSnapshotClass := func() {
		err := f.CrClient.Delete(context.TODO(), newStubSnapshotClass(""))
		if err != nil && !errors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}
		f.SnapshotSCName = ""
	}

	waitForCloneStrategyInit := func() {
		Eventually(func() *cdiv1.CDICloneStrategy {
			profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), defaultStorageClass.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			defaultCloneStrategy = profile.Status.CloneStrategy
			return defaultCloneStrategy
		}, 2*time.Minute, 5*time.Second).ShouldNot(BeNil())
	}

	waitForCloneStrategy := func(strategy cdiv1.CDICloneStrategy) {
		Eventually(func() bool {
			profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), defaultStorageClass.Name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				return false
			}
			Expect(err).ToNot(HaveOccurred())
			cs := profile.Status.CloneStrategy
			return cs != nil && *cs == strategy
		}, 2*time.Minute, 5*time.Second).Should(BeTrue())
	}

	BeforeEach(func() {
		if !f.IsPrometheusAvailable() {
			Skip("This test depends on prometheus infra being available")
		}

		cr = getCDI(f)
		cdiPods = getCDIPods(f)
		defaultStorageClass = utils.GetDefaultStorageClass(f.K8sClient)
		defaultVirtStorageClass = utils.GetDefaultVirtStorageClass(f.K8sClient)

		waitForStorageProfileMetricInit()
		waitForCloneStrategyInit()
	})

	AfterEach(func() {
		deleteUnknownStorageClasses()
		updateDefaultStorageClasses("true")
		updateDefaultStorageClassProfileClaimPropertySets(nil)
		deleteStubSnapshotClass()
		waitForCloneStrategy(*defaultCloneStrategy)

		if crModified {
			removeCDI(f, cr)
			ensureCDI(f, cr, cdiPods)
			crModified = false
		}

		Eventually(func() int {
			return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "complete", "false")
		}, 5*time.Minute, 5*time.Second).Should(BeZero())
	})

	Context("[rfe_id:7101][crit:medium][vendor:cnv-qe@redhat.com][level:component] Metrics and Alert tests", func() {

		It("[test_id:9656] Metric kubevirt_cdi_cr_ready is 0 when CDI is not ready", func() {
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_cr_ready")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", 1))

			crModified = true
			removeCDI(f, cr)

			By("Creating new CDI with wrong NodeSelector")
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: cr.Name,
				},
				Spec: cr.Spec,
			}
			cdi.Spec.Infra.NodeSelector = map[string]string{"wrong": "wrong"}
			_, err := f.CdiClient.CdiV1beta1().CDIs().Create(context.TODO(), cdi, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Wait for kubevirt_cdi_cr_ready == 0")
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_cr_ready")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", 0))

			waitForPrometheusAlert(f, "CDINotReady")

			By("Revert CDI CR changes")
			cdi, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			cdi.Spec = cr.Spec
			_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cdi, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			waitCDI(f, cr, cdiPods)
			crModified = false

			By("Wait for kubevirt_cdi_cr_ready == 1")
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_cr_ready")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", 1))
		})

		It("[test_id:7963] CDI ready metric value as expected when ready to use", func() {
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_cr_ready")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", 1))
		})

		It("[test_id:7965] StorageProfile incomplete metric expected value when creating an incomplete profile", func() {
			defaultStorageClass := utils.GetDefaultStorageClass(f.K8sClient)
			defaultStorageClassProfile := &cdiv1.StorageProfile{}
			err := f.CrClient.Get(context.TODO(), types.NamespacedName{Name: defaultStorageClass.Name}, defaultStorageClassProfile)
			Expect(err).ToNot(HaveOccurred())

			numAddedStorageClasses = 2
			for i := 0; i < numAddedStorageClasses; i++ {
				_, err = f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), createUnknownStorageClass(fmt.Sprintf("unknown-sc-%d", i), "kubernetes.io/non-existent-provisioner"), metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			expectedIncomplete := numAddedStorageClasses
			Eventually(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "complete", "false")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", expectedIncomplete))

			waitForPrometheusAlert(f, "CDIStorageProfilesIncomplete")

			By("Fix profiles to be complete and test metric value equals original")
			for i := 0; i < numAddedStorageClasses; i++ {
				profile := &cdiv1.StorageProfile{}
				err = f.CrClient.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf("unknown-sc-%d", i)}, profile)
				Expect(err).ToNot(HaveOccurred())
				// These might be wrong values, but at least the profile is no longer incomplete
				profile.Spec.ClaimPropertySets = defaultStorageClassProfile.Status.ClaimPropertySets
				err = f.CrClient.Update(context.TODO(), profile)
				Expect(err).ToNot(HaveOccurred())
				expectedIncomplete--
				Eventually(func() int {
					return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "complete", "false")
				}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", expectedIncomplete))
			}
		})

		It("[test_id:10512]CDIMultipleDefaultVirtStorageClasses fired when more than one default virt storage class exists", func() {
			waitForNoPrometheusAlert(f, "CDIMultipleDefaultVirtStorageClasses")

			numAddedStorageClasses = 2
			for i := 0; i < numAddedStorageClasses; i++ {
				sc := createUnknownStorageClass(fmt.Sprintf("unknown-sc-%d", i), "kubernetes.io/non-existent-provisioner")
				cc.AddAnnotation(sc, cc.AnnDefaultVirtStorageClass, "true")
				_, err := f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), sc, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}

			waitForPrometheusAlert(f, "CDIMultipleDefaultVirtStorageClasses")

			deleteUnknownStorageClasses()

			waitForNoPrometheusAlert(f, "CDIMultipleDefaultVirtStorageClasses")
		})

		It("[test_id:XXXX]CDINoDefaultStorageClass fired when no default storage class exists, and a DataVolume is waiting for one", func() {
			By("Ensure initial metric values")
			defaultSCs := countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "default", "true")
			Expect(defaultSCs).To(Equal(1))
			defaultVSCs := countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "virtdefault", "true")
			Expect(defaultVSCs).To(BeNumerically("<=", 1))
			waitingDVs := getMetricValue(f, "kubevirt_cdi_datavolume_pending")
			Expect(waitingDVs).To(BeZero())

			updateDefaultStorageClasses("false")

			dv := utils.NewDataVolumeWithHTTPImportAndStorageSpec("test-dv", "1Gi", fmt.Sprintf(utils.TinyCoreQcow2URL, f.CdiInstallNs))
			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())

			By("Ensure metric values after removing default storage class annotation")
			Eventually(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "default", "true")
			}, metricPollingTimeout, metricPollingInterval).Should(BeZero())
			Eventually(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "virtdefault", "true")
			}, metricPollingTimeout, metricPollingInterval).Should(BeZero())
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_datavolume_pending")
			}, metricPollingTimeout, metricPollingInterval).Should(Equal(1))

			waitForPrometheusAlert(f, "CDINoDefaultStorageClass")

			updateDefaultStorageClasses("true")

			By("Ensure metric values after restoring default storage class annotation")
			Eventually(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "default", "true")
			}, metricPollingTimeout, metricPollingInterval).Should(Equal(defaultSCs))
			Eventually(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "virtdefault", "true")
			}, metricPollingTimeout, metricPollingInterval).Should(Equal(defaultVSCs))
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_datavolume_pending")
			}, metricPollingTimeout, metricPollingInterval).Should(BeZero())

			waitForNoPrometheusAlert(f, "CDINoDefaultStorageClass")
		})

		It("[test_id:XXXX]CDIDefaultStorageClassDegraded fired when default storage class has no smart clone or ReadWriteMany", func() {
			rwx := corev1.ReadWriteMany
			rwo := corev1.ReadWriteOnce

			profile, err := f.CdiClient.CdiV1beta1().StorageProfiles().Get(context.TODO(), defaultStorageClass.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			if f.SnapshotSCName != defaultStorageClass.Name && f.CsiCloneSCName != defaultStorageClass.Name {
				waitForPrometheusAlert(f, "CDIDefaultStorageClassDegraded")

				By("Default storage class does not support snapshot or CSI clone - adding stub VolumeSnapshot crds and VolumeSnapshotClass")
				createStubSnapshotClass(*profile.Status.Provisioner)
				waitForCloneStrategy(cdiv1.CloneStrategySnapshot)
			}

			if !f.IsSnapshotStorageClassAvailable() && !f.IsCSIVolumeCloneStorageClassAvailable() {
				Skip("Smart Clone is not applicable")
			}

			hasRWX := hasRWX(profile)
			if !hasRWX {
				waitForPrometheusAlert(f, "CDIDefaultStorageClassDegraded")
				By("Default storage class profile has no ReadWriteMany access mode - adding stub ReadWriteMany")
				updateDefaultStorageClassProfileClaimPropertySets(&rwx)
			}

			waitForNoPrometheusAlert(f, "CDIDefaultStorageClassDegraded")

			By("Remove storage profile ReadWriteMany access mode")
			updateDefaultStorageClassProfileClaimPropertySets(&rwo)

			waitForPrometheusAlert(f, "CDIDefaultStorageClassDegraded")

			By("Restore storage profile")
			if hasRWX {
				updateDefaultStorageClassProfileClaimPropertySets(nil)
			} else {
				updateDefaultStorageClassProfileClaimPropertySets(&rwx)
			}

			waitForNoPrometheusAlert(f, "CDIDefaultStorageClassDegraded")
		})

		It("[test_id:9659] StorageProfile incomplete metric expected value remains unchanged for provisioner known to not work", func() {
			sc, err := f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), createUnknownStorageClass("unsupported-provisioner", storagecapabilities.ProvisionerNoobaa), metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Profile created and is incomplete")
			profile := &cdiv1.StorageProfile{}
			Eventually(func() error {
				return f.CrClient.Get(context.TODO(), types.NamespacedName{Name: sc.Name}, profile)
			}, 20*time.Second, 1*time.Second).Should(Succeed())
			Expect(profile.Status.ClaimPropertySets).To(BeNil())

			By("Metric stays the same because we don't support this provisioner")
			Consistently(func() int {
				return countMetricLabelValue(f, "kubevirt_cdi_storageprofile_info", "complete", "false")
			}, metricConsistentPollingTimeout, metricPollingInterval).Should(BeZero())
		})

		It("[test_id:7964] DataImportCron failing metric expected value when patching DesiredDigest annotation with junk sha256 value", func() {
			numCrons := 2
			originalCronMetricVal := getMetricValueWithDefault(f, "kubevirt_cdi_dataimportcron_outdated", true)
			Expect(originalCronMetricVal).To(BeZero())

			waitForNoPrometheusAlert(f, "CDIDataImportCronOutdated")

			reg, err := getDataVolumeSourceRegistry(f)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				if err := utils.RemoveInsecureRegistry(f.CrClient, *reg.URL); err != nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "failed to remove registry; %v", err)
				}
			}()

			for i := 1; i < numCrons+1; i++ {
				cron := utils.NewDataImportCron(fmt.Sprintf("cron-test-%d", i), "5Gi", scheduleOnceAYear, fmt.Sprintf("datasource-test-%d", i), 1, *reg)
				By(fmt.Sprintf("Create new DataImportCron %s", *reg.URL))
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Create(context.TODO(), cron, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Wait for condition UpToDate=true on DataImportCron")
				Eventually(func() bool {
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					upToDateCondition := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
					return upToDateCondition != nil && upToDateCondition.ConditionState.Status == corev1.ConditionTrue
				}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

				Eventually(func() error {
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if cron.Annotations == nil {
						cron.Annotations = make(map[string]string)
					}
					// notarealsha
					cron.Annotations[controller.AnnSourceDesiredDigest] = "sha256:c616e1c85568b1f1d528da2b3dbc257fd2035ada441e286a8c42606491442a5d"
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Update(context.TODO(), cron, metav1.UpdateOptions{})
					return err
				}, dataImportCronTimeout, pollingInterval).Should(BeNil())
				By(fmt.Sprintf("Ensuring metric value incremented to %d", i))
				Eventually(func() int {
					return getMetricValue(f, "kubevirt_cdi_dataimportcron_outdated")
				}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", i))
			}

			waitForPrometheusAlert(f, "CDIDataImportCronOutdated")

			By("Ensure metric value decrements when crons are cleaned up")
			for i := 1; i < numCrons+1; i++ {
				err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Delete(context.TODO(), fmt.Sprintf("cron-test-%d", i), metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() int {
					return getMetricValueWithDefault(f, "kubevirt_cdi_dataimportcron_outdated", true)
				}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", numCrons-i))
			}

			waitForNoPrometheusAlert(f, "CDIDataImportCronOutdated")
		})

		It("[test_id:7962] CDIOperatorDown alert firing when operator scaled down", func() {
			deploymentName := "cdi-operator"
			By("Scale down operator so alert will trigger")
			originalReplicas := scaleDeployment(f, deploymentName, 0)
			Eventually(func() bool {
				dep, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return dep.Status.Replicas == 0
			}, 20*time.Second, 1*time.Second).Should(BeTrue())

			By("Waiting for kubevirt_cdi_operator_up metric to be 0")
			Eventually(func() int {
				return getMetricValue(f, "kubevirt_cdi_operator_up")
			}, metricPollingTimeout, metricPollingInterval).Should(BeNumerically("==", 0))

			waitForPrometheusAlert(f, "CDIOperatorDown")

			By("Ensuring original value of replicas restored")
			scaleDeployment(f, deploymentName, originalReplicas)
			err := utils.WaitForDeploymentReplicasReady(f.K8sClient, f.CdiInstallNs, deploymentName)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Prometheus Rule configuration", func() {
		It("[test_id:8259] Alerts should have all the required annotations", func() {
			promRule := getPrometheusRule(f)
			for _, group := range promRule.Spec.Groups {
				if group.Name == "cdi.rules" {
					for _, rule := range group.Rules {
						if rule.Alert != "" {
							Expect(rule.Annotations).ToNot(BeNil())
							checkRequiredAnnotations(rule)
						}
					}
				}
			}
		})

		It("[test_id:8812] Alerts should have all the required labels", func() {
			promRule := getPrometheusRule(f)
			for _, group := range promRule.Spec.Groups {
				if group.Name == "cdi.rules" {
					for _, rule := range group.Rules {
						if rule.Alert != "" {
							Expect(rule.Labels).ToNot(BeNil())
							checkRequiredLabels(rule)
						}
					}
				}
			}
		})
	})
})

func dataVolumeUnusualRestartTest(f *framework.Framework) {
	By("Test metric for unusual restart count")
	Eventually(func() bool {
		return getMetricValue(f, "kubevirt_cdi_import_pods_high_restart") == 1
	}, 2*time.Minute, 1*time.Second).Should(BeTrue())

	waitForPrometheusAlert(f, "CDIDataVolumeUnusualRestartCount")
}

// Helper functions

// getMetricValue returns the metric value, or the sum in case of multiple values (GaugeVec)
func getMetricValue(f *framework.Framework, endpoint string) int {
	return getMetricValueWithDefault(f, endpoint, false)
}

// getMetricValueWithDefault returns the metric value, or the sum in case of multiple values (GaugeVec)
// if the metric does not exist and useDefault is true, it immediately returns 0
func getMetricValueWithDefault(f *framework.Framework, endpoint string, useDefault bool) int {
	returnVal := 0

	Eventually(func() bool {
		dataResult, err := getMetricDataResult(f, endpoint)
		if err != nil {
			return false
		}
		if dataResult == nil {
			return useDefault
		}

		found := false
		returnVal = 0
		for _, res := range dataResult {
			values := res.(map[string]interface{})["value"].([]interface{})
			for _, v := range values {
				if s, ok := v.(string); ok {
					i, err := strconv.Atoi(s)
					Expect(err).ToNot(HaveOccurred())
					returnVal += i
					found = true
				}
			}

		}
		return found
	}, 1*time.Minute, 1*time.Second).Should(BeTrue())

	return returnVal

}

func countMetricLabelValue(f *framework.Framework, endpoint, label, value string) int {
	var count int

	Eventually(func() bool {
		dataResult, err := getMetricDataResult(f, endpoint)
		if err != nil || dataResult == nil {
			return false
		}

		found := false
		count = 0
		for _, res := range dataResult {
			metric := res.(map[string]interface{})["metric"].(map[string]interface{})
			val, ok := metric[label]
			if !ok {
				continue
			}

			values := res.(map[string]interface{})["value"].([]interface{})
			for _, v := range values {
				s, ok := v.(string)
				if !ok {
					continue
				}
				i, err := strconv.Atoi(s)
				Expect(err).ToNot(HaveOccurred())
				if i == 1 && val == value {
					count++
				}
				found = true
				break
			}
		}
		return found
	}, 1*time.Minute, 1*time.Second).Should(BeTrue())

	return count
}

func getMetricDataResult(f *framework.Framework, endpoint string) ([]interface{}, error) {
	var result map[string]interface{}

	By(fmt.Sprintf("Querying metric %s", endpoint))
	resp := f.MakePrometheusHTTPRequest("query?query=" + endpoint)
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}
	dataResult := result["data"].(map[string]interface{})["result"].([]interface{})
	if len(dataResult) == 0 {
		return nil, nil
	}
	return dataResult, nil
}

func createUnknownStorageClass(name, provisioner string) *storagev1.StorageClass {
	immediateBinding := storagev1.VolumeBindingImmediate

	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"cdi.kubevirt.io/testing": "",
			},
		},
		Provisioner:       provisioner,
		VolumeBindingMode: &immediateBinding,
	}
}

func getPrometheusRule(f *framework.Framework) *promv1.PrometheusRule {
	By("Wait for prometheus-cdi-rules")
	promRule := &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-cdi-rules",
			Namespace: f.CdiInstallNs,
		},
	}
	Eventually(func() error {
		return f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(promRule), promRule)
	}, 5*time.Minute, 1*time.Second).Should(BeNil())
	return promRule
}

func waitForPrometheusAlert(f *framework.Framework, alertName string) {
	By(fmt.Sprintf("Wait for alert %s to be triggered", alertName))
	Eventually(func() bool {
		result, err := getPrometheusAlerts(f)
		if err != nil {
			return false
		}

		alerts := result["data"].(map[string]interface{})["alerts"].([]interface{})
		for _, alert := range alerts {
			name := alert.(map[string]interface{})["labels"].(map[string]interface{})["alertname"].(string)
			if name == alertName {
				state := alert.(map[string]interface{})["state"].(string)
				By(fmt.Sprintf("Alert %s state %s", name, state))
				return state == "pending" || state == "firing"
			}
		}

		return false
	}, 10*time.Minute, 1*time.Second).Should(BeTrue())
}

func waitForNoPrometheusAlert(f *framework.Framework, alertName string) {
	By(fmt.Sprintf("Verify no %s alert", alertName))
	Eventually(func() bool {
		result, err := getPrometheusAlerts(f)
		if err != nil {
			return false
		}

		alerts := result["data"].(map[string]interface{})["alerts"].([]interface{})
		for _, alert := range alerts {
			name := alert.(map[string]interface{})["labels"].(map[string]interface{})["alertname"].(string)
			if name == alertName {
				return false
			}
		}

		return true
	}, 2*time.Minute, 1*time.Second).Should(BeTrue())
}

func getPrometheusAlerts(f *framework.Framework) (alerts map[string]interface{}, err error) {
	resp := f.MakePrometheusHTTPRequest("alerts")
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return alerts, err
	}

	err = json.Unmarshal(bodyBytes, &alerts)
	if err != nil {
		return alerts, err
	}

	return alerts, nil
}

func checkRequiredAnnotations(rule promv1.Rule) {
	ExpectWithOffset(1, rule.Annotations).To(HaveKeyWithValue("summary", Not(BeEmpty())),
		"%s summary is missing or empty", rule.Alert)
	ExpectWithOffset(1, rule.Annotations).To(HaveKey("runbook_url"),
		"%s runbook_url is missing", rule.Alert)
	ExpectWithOffset(1, rule.Annotations).To(HaveKeyWithValue("runbook_url", HaveSuffix(rule.Alert)),
		"%s runbook is not equal to alert name", rule.Alert)

	resp, err := http.Head(rule.Annotations["runbook_url"])
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), fmt.Sprintf("%s runbook is not available", rule.Alert))
	ExpectWithOffset(1, resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("%s runbook is not available", rule.Alert))
}

func checkRequiredLabels(rule promv1.Rule) {
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("severity", BeElementOf("info", "warning", "critical")),
		"%s severity label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("operator_health_impact", BeElementOf("none", "warning", "critical")),
		"%s operator_health_impact label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("kubernetes_operator_part_of", "kubevirt"),
		"%s kubernetes_operator_part_of label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "containerized-data-importer"),
		"%s kubernetes_operator_component label is missing or not valid", rule.Alert)
}

func newStubSnapshotClass(driver string) *snapshotv1.VolumeSnapshotClass {
	return &snapshotv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vsc",
		},
		Driver:         driver,
		DeletionPolicy: snapshotv1.VolumeSnapshotContentDelete,
	}
}

func hasRWX(profile *cdiv1.StorageProfile) bool {
	for _, cps := range profile.Status.ClaimPropertySets {
		for _, am := range cps.AccessModes {
			if am == corev1.ReadWriteMany {
				return true
			}
		}
	}
	return false
}
