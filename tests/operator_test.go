package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	secclient "github.com/openshift/client-go/security/clientset/versioned"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulev1 "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	resourcesutils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
)

var _ = Describe("ALL Operator tests", func() {
	Context("[Destructive]", func() {
		var _ = Describe("Operator tests", func() {
			f := framework.NewFramework("operator-test")

			Context("Adding versions to datavolume CRD", func() {
				deploymentName := "cdi-operator"
				var originalReplicaVal int32

				AfterEach(func() {
					By(fmt.Sprintf("Setting %s replica number back to the original value %d", deploymentName, originalReplicaVal))
					scaleDeployment(f, deploymentName, originalReplicaVal)
					Eventually(func() int32 {
						depl, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())
						return depl.Status.ReadyReplicas
					}, 5*time.Minute, 1*time.Second).Should(Equal(originalReplicaVal))
				})

				It("Alpha versions of CRD are removed, even if objects previously existed", func() {
					By("Scaling down CDI operator")
					originalReplicaVal = scaleDeployment(f, deploymentName, 0)
					Eventually(func() bool {
						_, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, deploymentName, common.CDILabelSelector)
						if !errors.IsNotFound(err) {
							return false
						}
						return true
					}, 20*time.Second, 1*time.Second).Should(BeTrue())

					By("Appending v1alpha1 version as stored version")
					dvCrd, err := f.ExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), "datavolumes.cdi.kubevirt.io", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					oldVer := dvCrd.Spec.Versions[0].DeepCopy()
					oldVer.Name = "v1alpha1"
					dvCrd.Spec.Versions[0].Storage = false
					oldVer.Storage = true
					dvCrd.Spec.Versions = append(dvCrd.Spec.Versions, *oldVer)

					dvCrd, err = f.ExtClient.ApiextensionsV1().CustomResourceDefinitions().Update(context.TODO(), dvCrd, metav1.UpdateOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(dvCrd.Status.StoredVersions).Should(ContainElement("v1alpha1"))

					By("Creating datavolume")
					dv := utils.NewDataVolumeForUpload("delete-me", "1Gi")
					dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
					Expect(err).ToNot(HaveOccurred())

					By("Making sure we can get datavolume in v1alpha1 version")
					Eventually(func() error {
						u := &unstructured.Unstructured{}
						gvk := schema.GroupVersionKind{
							Group:   "cdi.kubevirt.io",
							Version: "v1alpha1",
							Kind:    "DataVolume",
						}
						u.SetGroupVersionKind(gvk)
						nn := crclient.ObjectKey{Namespace: dv.Namespace, Name: dv.Name}
						err = f.CrClient.Get(context.TODO(), nn, u)
						return err
					}, 1*time.Minute, 2*time.Second).Should(BeNil())

					By("Scaling up CDI operator")
					scaleDeployment(f, deploymentName, originalReplicaVal)
					By("Eventually, CDI will restore v1beta1 to be the only stored version")
					Eventually(func() bool {
						dvCrd, err = f.ExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), "datavolumes.cdi.kubevirt.io", metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())
						for _, ver := range dvCrd.Spec.Versions {
							if !(ver.Name == "v1beta1" && ver.Storage == true) {
								return false
							}
						}
						return true
					}, 1*time.Minute, 2*time.Second).Should(BeTrue())

					By("Datavolume is still there")
					_, err = f.CdiClient.CdiV1beta1().DataVolumes(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			It("[test_id:3951]should create a route in OpenShift", func() {
				if !utils.IsOpenshift(f.K8sClient) {
					Skip("This test is OpenShift specific")
				}

				routeClient, err := routeclient.NewForConfig(f.RestConfig)
				Expect(err).ToNot(HaveOccurred())

				r, err := routeClient.RouteV1().Routes(f.CdiInstallNs).Get(context.TODO(), "cdi-uploadproxy", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				Expect(r.Spec.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))
			})

			It("[test_id:4351]should create a prometheus service in cdi namespace", func() {
				promService, err := f.K8sClient.CoreV1().Services(f.CdiInstallNs).Get(context.TODO(), common.PrometheusServiceName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(promService.Spec.Ports[0].Name).To(Equal("metrics"))
				Expect(promService.Spec.Selector[common.PrometheusLabelKey]).To(Equal(common.PrometheusLabelValue))
				originalTimeStamp := promService.ObjectMeta.CreationTimestamp

				By("Deleting the service")
				err = f.K8sClient.CoreV1().Services(f.CdiInstallNs).Delete(context.TODO(), common.PrometheusServiceName, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
				By("Verifying the operator has re-created the service")
				Eventually(func() bool {
					promService, err = f.K8sClient.CoreV1().Services(f.CdiInstallNs).Get(context.TODO(), common.PrometheusServiceName, metav1.GetOptions{})
					if err == nil {
						return originalTimeStamp.Before(&promService.ObjectMeta.CreationTimestamp)
					}
					return false
				}, 1*time.Minute, 2*time.Second).Should(BeTrue())
				Expect(promService.Spec.Ports[0].Name).To(Equal("metrics"))
				Expect(promService.Spec.Selector[common.PrometheusLabelKey]).To(Equal(common.PrometheusLabelValue))
			})

			It("[test_id:3952]add cdi-sa to containerized-data-importer scc", func() {
				if !utils.IsOpenshift(f.K8sClient) {
					Skip("This test is OpenShift specific")
				}

				secClient, err := secclient.NewForConfig(f.RestConfig)
				Expect(err).ToNot(HaveOccurred())

				scc, err := secClient.SecurityV1().SecurityContextConstraints().Get(context.TODO(), "containerized-data-importer", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				cdiSA := fmt.Sprintf("system:serviceaccount:%s:cdi-sa", f.CdiInstallNs)
				Expect(scc.Users).Should(ContainElement(cdiSA))
			})

			// Condition flags can be found here with their meaning https://github.com/kubevirt/hyperconverged-cluster-operator/blob/main/docs/conditions.md
			It("[test_id:3953]Condition flags on CR should be healthy and operating", func() {
				cdiObjects, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(cdiObjects.Items)).To(Equal(1))
				cdiObject := cdiObjects.Items[0]
				conditionMap := sdk.GetConditionValues(cdiObject.Status.Conditions)
				// Application should be fully operational and healthy.
				Expect(conditionMap[conditions.ConditionAvailable]).To(Equal(corev1.ConditionTrue))
				Expect(conditionMap[conditions.ConditionProgressing]).To(Equal(corev1.ConditionFalse))
				Expect(conditionMap[conditions.ConditionDegraded]).To(Equal(corev1.ConditionFalse))
			})

			It("should make CDI config authority", func() {
				Eventually(func() bool {
					cdiObjects, err := f.CdiClient.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(len(cdiObjects.Items)).To(Equal(1))
					cdiObject := cdiObjects.Items[0]
					_, ok := cdiObject.Annotations["cdi.kubevirt.io/configAuthority"]
					return ok
				}, 1*time.Minute, 2*time.Second).Should(BeTrue())
			})
		})

		var _ = Describe("Tests needing the restore of nodes", func() {
			var nodes *corev1.NodeList
			var cdiPods *corev1.PodList
			var err error

			f := framework.NewFramework("operator-delete-cdi-test")

			BeforeEach(func() {
				nodes, err = f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
				Expect(nodes.Items).ToNot(BeEmpty(), "There should be some compute node")
				Expect(err).ToNot(HaveOccurred())

				cdiPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})

				Expect(err).ToNot(HaveOccurred(), "failed listing cdi pods")
				Expect(len(cdiPods.Items)).To(BeNumerically(">", 0), "no cdi pods found")
			})

			AfterEach(func() {
				var errors []error
				var newCdiPods *corev1.PodList
				By("Restoring nodes")
				for _, node := range nodes.Items {
					newNode, err := f.K8sClient.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					newNode.Spec = node.Spec
					_, err = f.K8sClient.CoreV1().Nodes().Update(context.TODO(), newNode, metav1.UpdateOptions{})
					if err != nil {
						errors = append(errors, err)
					}
				}
				Expect(errors).Should(BeEmpty(), "failed restoring one or more nodes")

				By("Waiting for there to be as many CDI pods as before")
				Eventually(func() bool {
					newCdiPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred(), "failed getting CDI pods")

					By(fmt.Sprintf("number of cdi pods: %d\n new number of cdi pods: %d\n", len(cdiPods.Items), len(newCdiPods.Items)))
					return len(cdiPods.Items) == len(newCdiPods.Items)
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())

				for _, newCdiPod := range newCdiPods.Items {
					By(fmt.Sprintf("Waiting for CDI pod %s to be ready", newCdiPod.Name))
					err := utils.WaitTimeoutForPodReady(f.K8sClient, newCdiPod.Name, newCdiPod.Namespace, 20*time.Minute)
					Expect(err).ToNot(HaveOccurred())
				}

				Eventually(func() bool {
					services, err := f.K8sClient.CoreV1().Services(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred(), "failed getting CDI services")
					for _, service := range services.Items {
						if service.Name != "cdi-prometheus-metrics" {
							endpoint, err := f.K8sClient.CoreV1().Endpoints(f.CdiInstallNs).Get(context.TODO(), service.Name, metav1.GetOptions{})
							Expect(err).ToNot(HaveOccurred(), "failed getting service endpoint")
							for _, subset := range endpoint.Subsets {
								if len(subset.NotReadyAddresses) > 0 {
									By(fmt.Sprintf("Not all endpoints of service %s are ready", service.Name))
									return false
								}
							}
						}
					}
					return true
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())
			})

			It("should deploy components that tolerate CriticalAddonsOnly taint", func() {
				var err error
				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())

				criticalAddonsToleration := corev1.Toleration{
					Key:      "CriticalAddonsOnly",
					Operator: corev1.TolerationOpExists,
				}

				if !tolerationExists(cr.Spec.Infra.Tolerations, criticalAddonsToleration) {
					Skip("Unexpected CDI CR (not from cdi-cr.yaml), doesn't tolerate CriticalAddonsOnly")
				}

				labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"cdi.kubevirt.io/testing": ""}}
				cdiTestPods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
				})
				Expect(err).ToNot(HaveOccurred(), "failed listing cdi testing pods")
				Expect(len(cdiTestPods.Items)).To(BeNumerically(">", 0), "no cdi testing pods found")

				By("adding taints to all nodes")
				criticalPodTaint := corev1.Taint{
					Key:    "CriticalAddonsOnly",
					Value:  "",
					Effect: corev1.TaintEffectNoExecute,
				}

				for _, node := range nodes.Items {
					Eventually(func() bool {
						nodeCopy, err := f.K8sClient.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
						Expect(err).ToNot(HaveOccurred())

						if nodeHasTaint(*nodeCopy, criticalPodTaint) {
							return true
						}

						nodeCopy.Spec.Taints = append(nodeCopy.Spec.Taints, criticalPodTaint)
						_, _ = f.K8sClient.CoreV1().Nodes().Update(context.TODO(), nodeCopy, metav1.UpdateOptions{})
						return false
					}, 5*time.Minute, 2*time.Second).Should(BeTrue())
				}

				By("Waiting for all CDI testing pods to terminate")
				Eventually(func() bool {
					for _, cdiTestPod := range cdiTestPods.Items {
						By(fmt.Sprintf("CDI test pod: %s", cdiTestPod.Name))
						_, err := f.K8sClient.CoreV1().Pods(cdiTestPod.Namespace).Get(context.TODO(), cdiTestPod.Name, metav1.GetOptions{})
						if !errors.IsNotFound(err) {
							return false
						}
					}
					return true
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())

				By("Checking that all the non-testing pods are running")
				for _, cdiPod := range cdiPods.Items {
					if _, isTestingComponent := cdiPod.Labels["cdi.kubevirt.io/testing"]; isTestingComponent {
						continue
					}
					By(fmt.Sprintf("Non-test CDI pod: %s", cdiPod.Name))
					podUpdated, err := f.K8sClient.CoreV1().Pods(cdiPod.Namespace).Get(context.TODO(), cdiPod.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred(), "failed setting taint on node")
					Expect(podUpdated.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})

		})

		var _ = Describe("Operator delete CDI CR tests", func() {
			var cr *cdiv1.CDI
			f := framework.NewFramework("operator-delete-cdi-test")
			var cdiPods *corev1.PodList

			BeforeEach(func() {
				var err error
				cdiPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})

				Expect(err).ToNot(HaveOccurred(), "failed listing cdi pods")
				Expect(len(cdiPods.Items)).To(BeNumerically(">", 0), "no cdi pods found")

				cr, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())
			})

			removeCDI := func() {
				By("Deleting CDI CR if exists")
				_ = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{})

				By("Waiting for CDI CR and infra deployments to be gone now that we are sure there's no CDI CR")
				Eventually(func() bool { return infraDeploymentGone(f) && crGone(f, cr) }, 15*time.Minute, 2*time.Second).Should(BeTrue())
			}

			ensureCDI := func() {
				var newCdiPods *corev1.PodList

				if cr == nil {
					return
				}

				By("Check if CDI CR exists")
				cdi, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
				if err == nil {
					if cdi.DeletionTimestamp == nil {
						By("CDI CR exists")
						cdi.Spec = cr.Spec
						_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cdi, metav1.UpdateOptions{})
						Expect(err).ToNot(HaveOccurred())
						return
					}

					By("Waiting for CDI CR deletion")
					Eventually(func() bool {
						_, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
						if errors.IsNotFound(err) {
							return true
						}
						Expect(err).ToNot(HaveOccurred())
						return false
					}, 5*time.Minute, 2*time.Second).Should(BeTrue())
				} else {
					Expect(errors.IsNotFound(err)).To(BeTrue())
				}

				cdi = &cdiv1.CDI{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cdi",
					},
					Spec: cr.Spec,
				}

				By("Create CDI CR")
				cdi, err = f.CdiClient.CdiV1beta1().CDIs().Create(context.TODO(), cdi, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for CDI CR")
				Eventually(func() bool {
					cdi, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(cdi.Status.Phase).ShouldNot(Equal(sdkapi.PhaseError))
					return conditions.IsStatusConditionTrue(cdi.Status.Conditions, conditions.ConditionAvailable)
				}, 10*time.Minute, 2*time.Second).Should(BeTrue())

				By("Verifying CDI apiserver, deployment, uploadproxy exist, before continuing")
				Eventually(func() bool { return infraDeploymentAvailable(f, cr) }, CompletionTimeout, assertionPollInterval).Should(BeTrue(), "Timeout reading CDI deployments")

				By("Verifying CDI config object exists, before continuing")
				Eventually(func() bool {
					_, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return false
					}
					Expect(err).ToNot(HaveOccurred(), "Unable to read CDI Config, %v, expect more failures", err)
					return true
				}, CompletionTimeout, assertionPollInterval).Should(BeTrue(), "Timeout reading CDI Config, expect more failures")

				By("Waiting for there to be as many CDI pods as before")
				Eventually(func() bool {
					newCdiPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred(), "failed getting CDI pods")

					By(fmt.Sprintf("number of cdi pods: %d\n new number of cdi pods: %d\n", len(cdiPods.Items), len(newCdiPods.Items)))
					return len(cdiPods.Items) == len(newCdiPods.Items)
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())

				for _, newCdiPod := range newCdiPods.Items {
					By(fmt.Sprintf("Waiting for CDI pod %s to be ready", newCdiPod.Name))
					err := utils.WaitTimeoutForPodReady(f.K8sClient, newCdiPod.Name, newCdiPod.Namespace, 20*time.Minute)
					Expect(err).ToNot(HaveOccurred())
				}
			}

			AfterEach(func() {
				removeCDI()
				ensureCDI()
			})

			It("[test_id:4986]should remove/install CDI a number of times successfully", func() {
				for i := 0; i < 10; i++ {
					err := f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
					ensureCDI()
				}
			})

			It("[test_id:3954]should delete an upload pod", func() {
				dv := utils.NewDataVolumeForUpload("delete-me", "1Gi")

				By("Creating datavolume")
				dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)

				By("Waiting for pod to be running")
				Eventually(func() bool {
					pod, err := f.K8sClient.CoreV1().Pods(dv.Namespace).Get(context.TODO(), "cdi-upload-"+dv.Name, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return false
					}
					Expect(err).ToNot(HaveOccurred())
					return pod.Status.Phase == corev1.PodRunning
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())

				By("Deleting CDI")
				err = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for pod to be deleted")
				Eventually(func() bool {
					_, err = f.K8sClient.CoreV1().Pods(dv.Namespace).Get(context.TODO(), "cdi-upload-"+dv.Name, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return true
					}
					Expect(err).ToNot(HaveOccurred())
					return false
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:3955]should block CDI delete", func() {
				uninstallStrategy := cdiv1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist
				updateUninstallStrategy(f.CdiClient, &uninstallStrategy)

				By("Creating datavolume")
				dv := utils.NewDataVolumeForUpload("delete-me", "1Gi")
				dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)

				By("Creating datavolume with DataImportCron label")
				dv = utils.NewDataVolumeForUpload("retain-me", "1Gi")
				dv.Labels = map[string]string{common.DataImportCronLabel: "dic"}
				dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).ToNot(HaveOccurred())
				f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)

				By("Cannot delete CDI")
				err = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("there are still DataVolumes present"))

				By("Delete the unlabeled datavolume")
				err = f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Delete(context.TODO(), "delete-me", metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Can delete CDI")
				err = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:8087]CDI CR deletion should delete DataImportCron CRD and all DataImportCrons", func() {
				reg, err := getDataVolumeSourceRegistry(f)
				Expect(err).To(BeNil())

				By("Create new DataImportCron")
				cron := utils.NewDataImportCron("cron-test", "5Gi", scheduleEveryMinute, "ds", 1, *reg)
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Create(context.TODO(), cron, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Verify cron first import completed")
				Eventually(func() bool {
					cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Get(context.TODO(), cron.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					upToDateCond := controller.FindDataImportCronConditionByType(cron, cdiv1.DataImportCronUpToDate)
					return upToDateCond != nil && upToDateCond.Status == corev1.ConditionTrue
				}, dataImportCronTimeout, pollingInterval).Should(BeTrue())

				pvc := cron.Status.LastImportedPVC
				Expect(pvc).ToNot(BeNil())

				By("Verify dv succeeded")
				err = utils.WaitForDataVolumePhase(f, pvc.Namespace, cdiv1.Succeeded, pvc.Name)
				Expect(err).ToNot(HaveOccurred())

				By("Start goroutine creating DataImportCrons")
				go func() {
					defer GinkgoRecover()
					var err error
					for i := 0; i < 100 && err == nil; i++ {
						cronName := fmt.Sprintf("cron-test-%d", i)
						cron := utils.NewDataImportCron(cronName, "5Gi", scheduleEveryMinute, "ds", 1, *reg)
						_, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Create(context.TODO(), cron, metav1.CreateOptions{})
					}
				}()

				removeCDI()

				By("Verify no DataImportCrons are found")
				Eventually(func() bool {
					_, err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
					return err != nil && errors.IsNotFound(err)
				}, 1*time.Minute, 2*time.Second).Should(BeTrue())

				By("Verify no cronjobs left")
				Eventually(func() bool {
					cronjobs, err := f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred())
					return len(cronjobs.Items) == 0
				}, 1*time.Minute, 2*time.Second).Should(BeTrue())

				By("Verify no jobs left")
				Eventually(func() bool {
					jobs, err := f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred())
					return len(jobs.Items) == 0
				}, 1*time.Minute, 2*time.Second).Should(BeTrue())
			})
		})

		var _ = Describe("[rfe_id:4784][crit:high] CDI Operator deployment + CDI CR delete tests", func() {
			var restoreCdiCr *cdiv1.CDI
			var restoreCdiOperatorDeployment *appsv1.Deployment
			f := framework.NewFramework("operator-delete-cdi-test")

			removeCDI := func() {
				By("Deleting CDI CR")
				err := f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), restoreCdiCr.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for CDI CR and infra deployments to be deleted after CDI CR was removed")
				Eventually(func() bool { return infraDeploymentGone(f) && crGone(f, restoreCdiCr) }, 15*time.Minute, 2*time.Second).Should(BeTrue())

				By("Deleting CDI operator")
				err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(context.TODO(), "cdi-operator", metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Waiting for CDI operator deployment to be deleted")
				Eventually(func() bool { return cdiOperatorDeploymentGone(f) }, 5*time.Minute, 2*time.Second).Should(BeTrue())
			}

			ensureCDI := func(cr *cdiv1.CDI) {
				By("Re-creating CDI (CR and deployment)")
				_, err := f.CdiClient.CdiV1beta1().CDIs().Create(context.TODO(), cr, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Recreating CDI operator")
				_, err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Create(context.TODO(), restoreCdiOperatorDeployment, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Verifying CDI apiserver, deployment, uploadproxy exist, before continuing")
				Eventually(func() bool { return infraDeploymentAvailable(f, restoreCdiCr) }, CompletionTimeout, assertionPollInterval).Should(BeTrue(), "Timeout reading CDI deployments")

				By("Verifying CDI config object exists, before continuing")
				Eventually(func() bool {
					_, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
					if errors.IsNotFound(err) {
						return false
					}
					Expect(err).ToNot(HaveOccurred(), "Unable to read CDI Config, %v, expect more failures", err)
					return true
				}, CompletionTimeout, assertionPollInterval).Should(BeTrue(), "Timeout reading CDI Config, expect more failures")
			}

			BeforeEach(func() {
				var err error
				currentCR, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())

				restoreCdiCr = &cdiv1.CDI{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cdi",
					},
					Spec: currentCR.Spec,
				}

				currentCdiOperatorDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), "cdi-operator", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				restoreCdiOperatorDeployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cdi-operator",
						Namespace: f.CdiInstallNs,
					},
					Spec: currentCdiOperatorDeployment.Spec,
				}

				removeCDI()
			})

			AfterEach(func() {
				removeCDI()
				ensureCDI(restoreCdiCr)
			})

			It("[test_id:4782] Should install CDI infrastructure pods with node placement", func() {
				By("Creating modified CDI CR, with infra nodePlacement")
				localSpec := restoreCdiCr.Spec.DeepCopy()
				localSpec.Infra = f.TestNodePlacementValues()

				tempCdiCr := &cdiv1.CDI{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cdi",
					},
					Spec: *localSpec,
				}

				ensureCDI(tempCdiCr)

				By("Testing all infra deployments have the chosen node placement")
				for _, deploymentName := range []string{"cdi-apiserver", "cdi-deployment", "cdi-uploadproxy"} {
					deployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					match := f.PodSpecHasTestNodePlacementValues(deployment.Spec.Template.Spec)
					Expect(match).To(BeTrue(), fmt.Sprintf("node placement in pod spec\n%v\n differs from node placement values in CDI CR\n%v\n", deployment.Spec.Template.Spec, localSpec.Infra))
				}
			})
		})

		var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]Strict Reconciliation tests", func() {
			f := framework.NewFramework("strict-reconciliation-test")

			It("[test_id:5573]cdi-deployment replicas back to original value on attempt to scale", func() {
				By("Overwrite number of replicas with 10")
				deploymentName := "cdi-deployment"
				originalReplicaVal := scaleDeployment(f, deploymentName, 10)

				By("Ensuring original value of replicas restored & extra deployment pod was cleaned up")
				Eventually(func() bool {
					depl, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					_, err = utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, deploymentName, common.CDILabelSelector)
					return *depl.Spec.Replicas == originalReplicaVal && err == nil
				}, 5*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:5574]Service spec.selector restored on overwrite attempt", func() {
				service, err := f.K8sClient.CoreV1().Services(f.CdiInstallNs).Get(context.TODO(), "cdi-api", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				originalSelectorVal := service.Spec.Selector[common.CDIComponentLabel]

				By("Overwrite spec.selector with empty string")
				service.Spec.Selector[common.CDIComponentLabel] = ""
				_, err = f.K8sClient.CoreV1().Services(f.CdiInstallNs).Update(context.TODO(), service, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					svc, err := f.K8sClient.CoreV1().Services(f.CdiInstallNs).Get(context.TODO(), "cdi-api", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By(fmt.Sprintf("Waiting until original spec.selector value: %s\n Matches current: %s\n", originalSelectorVal, svc.Spec.Selector[common.CDIComponentLabel]))
					return svc.Spec.Selector[common.CDIComponentLabel] == originalSelectorVal
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:5575]ClusterRole verb restored on deletion attempt", func() {
				clusterRole, err := f.K8sClient.RbacV1().ClusterRoles().Get(context.TODO(), "cdi.kubevirt.io:config-reader", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Remove list verb")
				clusterRole.Rules = []rbacv1.PolicyRule{
					{
						APIGroups: []string{
							"cdi.kubevirt.io",
						},
						Resources: []string{
							"cdiconfigs",
						},
						Verbs: []string{
							"get",
							// "list",
							"watch",
						},
					},
				}

				_, err = f.K8sClient.RbacV1().ClusterRoles().Update(context.TODO(), clusterRole, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					role, err := f.K8sClient.RbacV1().ClusterRoles().Get(context.TODO(), "cdi.kubevirt.io:config-reader", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By("Waiting until list verb exists")
					for _, verb := range role.Rules[0].Verbs {
						if verb == "list" {
							return true
						}
					}
					return false
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:5576]ServiceAccount values restored on update attempt", func() {
				serviceAccount, err := f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Get(context.TODO(), common.ControllerServiceAccountName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Change one of ServiceAccount labels")
				serviceAccount.Labels[common.CDIComponentLabel] = "somebadvalue"

				_, err = f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					sa, err := f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Get(context.TODO(), common.ControllerServiceAccountName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By("Waiting until label value restored")
					return sa.Labels[common.CDIComponentLabel] == ""
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:5577]Certificate restored to ConfigMap on deletion attempt", func() {
				configMap, err := f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Get(context.TODO(), "cdi-apiserver-signer-bundle", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Empty ConfigMap's data")
				configMap.Data = map[string]string{}

				_, err = f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Update(context.TODO(), configMap, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					cm, err := f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Get(context.TODO(), "cdi-apiserver-signer-bundle", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By("Waiting until ConfigMap's data is not empty")
					return len(cm.Data) != 0
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("[test_id:5578]Cant enable featureGate by editing CDIConfig resource", func() {
				feature := "nonExistantFeature"
				cdiConfig, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Enable non existant featureGate")
				cdiConfig.Spec = cdiv1.CDIConfigSpec{
					FeatureGates: []string{feature},
				}

				_, err = f.CdiClient.CdiV1beta1().CDIConfigs().Update(context.TODO(), cdiConfig, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By(fmt.Sprintf("Waiting until %s featureGate doesn't exist", feature))
					for _, fgate := range config.Spec.FeatureGates {
						if fgate == feature {
							return false
						}
					}
					return true
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})

			It("SCC priority always reset to default", func() {
				if !utils.IsOpenshift(f.K8sClient) {
					Skip("This test is OpenShift specific")
				}

				secClient, err := secclient.NewForConfig(f.RestConfig)
				Expect(err).ToNot(HaveOccurred())

				scc, err := secClient.SecurityV1().SecurityContextConstraints().Get(context.TODO(), "containerized-data-importer", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Overwrite priority of SCC")
				scc.Priority = pointer.Int32(10)
				_, err = secClient.SecurityV1().SecurityContextConstraints().Update(context.TODO(), scc, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() *int32 {
					scc, err := secClient.SecurityV1().SecurityContextConstraints().Get(context.TODO(), "containerized-data-importer", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return scc.Priority
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			})
		})

		var _ = Describe("[rfe_id:7101][crit:medium][vendor:cnv-qe@redhat.com][level:component]Alert tests", func() {
			var numAddedStorageClasses int
			f := framework.NewFramework("alert-tests")

			AfterEach(func() {
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
			})

			getMetricValue := func(endpoint string) int {
				var returnVal string

				Eventually(func() bool {
					var result map[string]interface{}
					resp := f.MakePrometheusHTTPRequest("query?query=" + endpoint)
					defer resp.Body.Close()
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return false
					}
					err = json.Unmarshal(bodyBytes, &result)
					if err != nil {
						return false
					}
					if len(result["data"].(map[string]interface{})["result"].([]interface{})) == 0 {
						return false
					}
					values := result["data"].(map[string]interface{})["result"].([]interface{})[0].(map[string]interface{})["value"].([]interface{})
					for _, v := range values {
						if s, ok := v.(string); ok {
							returnVal = s
							return true
						}
					}
					return false
				}, 1*time.Minute, 1*time.Second).Should(BeTrue())

				i, err := strconv.Atoi(returnVal)
				Expect(err).ToNot(HaveOccurred())
				return i
			}

			createUnknownStorageClass := func(name string) *storagev1.StorageClass {
				immediateBinding := storagev1.VolumeBindingImmediate

				return &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
						Labels: map[string]string{
							"cdi.kubevirt.io/testing": "",
						},
					},
					Provisioner:       "kubernetes.io/non-existent-provisioner",
					VolumeBindingMode: &immediateBinding,
				}
			}

			It("[test_id:7962] CDIOperatorDown alert firing when operator scaled down", func() {
				if !f.IsPrometheusAvailable() {
					Skip("This test depends on prometheus infra being available")
				}

				deploymentName := "cdi-operator"
				By("Scale down operator so alert will trigger")
				originalReplicas := scaleDeployment(f, deploymentName, 0)
				Eventually(func() bool {
					dep, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return dep.Status.Replicas == 0
				}, 20*time.Second, 1*time.Second).Should(BeTrue())
				By("Patch our rule so alert fires a little faster")
				promRule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: f.CdiInstallNs,
					},
				}
				err := f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(promRule), promRule)
				Expect(err).ToNot(HaveOccurred())
				for i, group := range promRule.Spec.Groups {
					if group.Name == "cdi.rules" {
						for j, rule := range group.Rules {
							if rule.Alert == "CDIOperatorDown" {
								rule.For = "1m"
								promRule.Spec.Groups[i].Rules[j] = rule
								break
							}
						}
						break
					}
				}
				err = f.CrClient.Update(context.TODO(), promRule)
				Expect(err).ToNot(HaveOccurred())

				By("Let's see that alert fires")
				Eventually(func() bool {
					var result map[string]interface{}
					resp := f.MakePrometheusHTTPRequest("alerts")
					defer resp.Body.Close()
					// Make sure alert appears and is firing
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return false
					}
					err = json.Unmarshal(bodyBytes, &result)
					if err != nil {
						return false
					}
					alerts := result["data"].(map[string]interface{})["alerts"].([]interface{})
					for _, alert := range alerts {
						name := alert.(map[string]interface{})["labels"].(map[string]interface{})["alertname"].(string)
						if name == "CDIOperatorDown" {
							if state := alert.(map[string]interface{})["state"].(string); state == "firing" {
								return true
							}
						}
					}
					return false
				}, 10*time.Minute, 1*time.Second).Should(BeTrue())

				By("Ensuring original value of replicas restored")
				scaleDeployment(f, deploymentName, originalReplicas)
				err = utils.WaitForDeploymentReplicasReady(f.K8sClient, f.CdiInstallNs, deploymentName)
				Expect(err).ToNot(HaveOccurred())
			})

			It("[test_id:7963] CDI ready metric value as expected when ready to use", func() {
				if !f.IsPrometheusAvailable() {
					Skip("This test depends on prometheus infra being available")
				}

				Eventually(func() int {
					return getMetricValue("kubevirt_cdi_cr_ready")
				}, 1*time.Minute, 1*time.Second).Should(BeNumerically("==", 1))
			})

			It("[test_id:7965] StorageProfile incomplete metric expected value when creating an incomplete profile", func() {
				if !f.IsPrometheusAvailable() {
					Skip("This test depends on prometheus infra being available")
				}

				numAddedStorageClasses = 2
				defaultStorageClass := utils.GetDefaultStorageClass(f.K8sClient)
				defaultStorageClassProfile := &cdiv1.StorageProfile{}
				err := f.CrClient.Get(context.TODO(), types.NamespacedName{Name: defaultStorageClass.Name}, defaultStorageClassProfile)
				Expect(err).ToNot(HaveOccurred())
				for i := 0; i < numAddedStorageClasses; i++ {
					_, err = f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), createUnknownStorageClass(fmt.Sprintf("unknown-sc-%d", i)), metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
				}
				originalMetricVal := getMetricValue("kubevirt_cdi_incomplete_storageprofiles_total")
				expectedIncomplete := originalMetricVal + numAddedStorageClasses

				Eventually(func() int {
					return getMetricValue("kubevirt_cdi_incomplete_storageprofiles_total")
				}, 2*time.Minute, 1*time.Second).Should(BeNumerically("==", expectedIncomplete))

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
						return getMetricValue("kubevirt_cdi_incomplete_storageprofiles_total")
					}, 2*time.Minute, 1*time.Second).Should(BeNumerically("==", expectedIncomplete))
				}
			})

			It("[test_id:7964] DataImportCron failing metric expected value when patching DesiredDigest annotation with junk sha256 value", func() {
				if !f.IsPrometheusAvailable() {
					Skip("This test depends on prometheus infra being available")
				}
				numCrons := 2
				originalMetricVal := getMetricValue("kubevirt_cdi_dataimportcron_outdated_total")
				expectedFailingCrons := originalMetricVal + numCrons

				reg, err := getDataVolumeSourceRegistry(f)
				Expect(err).To(BeNil())
				defer utils.RemoveInsecureRegistry(f.CrClient, *reg.URL)

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
					By(fmt.Sprintf("Ensuring metric value incremented to %d", originalMetricVal+i))
					Eventually(func() int {
						return getMetricValue("kubevirt_cdi_dataimportcron_outdated_total")
					}, 3*time.Minute, 1*time.Second).Should(BeNumerically("==", originalMetricVal+i))
				}
				By("Ensure metric value decrements when crons are cleaned up")
				for i := 1; i < numCrons+1; i++ {
					err = f.CdiClient.CdiV1beta1().DataImportCrons(f.Namespace.Name).Delete(context.TODO(), fmt.Sprintf("cron-test-%d", i), metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() int {
						return getMetricValue("kubevirt_cdi_dataimportcron_outdated_total")
					}, 3*time.Minute, 1*time.Second).Should(BeNumerically("==", expectedFailingCrons-i))
				}
			})

			It("[test_id:8259] Alerts should have all the requried annotations", func() {
				promRule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: f.CdiInstallNs,
					},
				}
				err := f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(promRule), promRule)
				Expect(err).ToNot(HaveOccurred())
				for _, group := range promRule.Spec.Groups {
					if group.Name == "cdi.rules" {
						for _, rule := range group.Rules {
							if rule.Alert != "" {
								Expect(rule.Annotations).ToNot(BeNil())
								checkForRunbookURL(rule)
								checkForSummary(rule)
							}
						}
					}
				}
			})

			It("[test_id:8812] Alerts should have all the requried labels", func() {
				promRule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: f.CdiInstallNs,
					},
				}
				err := f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(promRule), promRule)
				Expect(err).ToNot(HaveOccurred())
				for _, group := range promRule.Spec.Groups {
					if group.Name == "cdi.rules" {
						for _, rule := range group.Rules {
							if rule.Alert != "" {
								Expect(rule.Labels).ToNot(BeNil())
								checkForSeverityLabel(rule)
								checkForPartOfLabel(rule)
								checkForComponentLabel(rule)
							}
						}
					}
				}
			})
		})

		var _ = Describe("Operator cert config tests", func() {
			var cdi *cdiv1.CDI
			f := framework.NewFramework("operator-cert-config-test")

			BeforeEach(func() {
				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())
				cdi = cr
			})

			AfterEach(func() {
				if cdi == nil {
					return
				}

				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				cr.Spec.CertConfig = cdi.Spec.CertConfig

				_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			getSecrets := func(secrets []string) []corev1.Secret {
				var result []corev1.Secret
				for _, s := range secrets {
					s, err := f.K8sClient.CoreV1().Secrets(f.CdiInstallNs).Get(context.TODO(), s, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					result = append(result, *s)
				}
				return result
			}

			validateCertConfig := func(obj metav1.Object, lifetime, refresh string) {
				cca, ok := obj.GetAnnotations()["operator.cdi.kubevirt.io/certConfig"]
				Expect(ok).To(BeTrue())
				certConfig := make(map[string]interface{})
				err := json.Unmarshal([]byte(cca), &certConfig)
				Expect(err).ToNot(HaveOccurred())
				l, ok := certConfig["lifetime"]
				Expect(ok).To(BeTrue())
				Expect(l.(string)).To(Equal(lifetime))
				r, ok := certConfig["refresh"]
				Expect(ok).To(BeTrue())
				Expect(r.(string)).To(Equal(refresh))
			}

			It("should allow update", func() {
				caSecretNames := []string{"cdi-apiserver-signer", "cdi-uploadproxy-signer"}
				serverSecretNames := []string{"cdi-apiserver-server-cert", "cdi-uploadproxy-server-cert"}

				ts := time.Now()

				Eventually(func() bool {
					cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					cr.Spec.CertConfig = &cdiv1.CDICertConfig{
						CA: &cdiv1.CertConfig{
							Duration:    &metav1.Duration{Duration: time.Minute * 20},
							RenewBefore: &metav1.Duration{Duration: time.Minute * 5},
						},
						Server: &cdiv1.CertConfig{
							Duration:    &metav1.Duration{Duration: time.Minute * 5},
							RenewBefore: &metav1.Duration{Duration: time.Minute * 2},
						},
					}
					newCR, err := f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
					if errors.IsConflict(err) {
						return false
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(newCR.Spec.CertConfig).To(Equal(cr.Spec.CertConfig))
					By("Cert config update complete")
					return true
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())

				Eventually(func() bool {
					caSecrets := getSecrets(caSecretNames)
					serverSecrets := getSecrets(serverSecretNames)

					for _, s := range append(caSecrets, serverSecrets...) {
						nba := s.Annotations["auth.openshift.io/certificate-not-before"]
						t, err := time.Parse(time.RFC3339, nba)
						Expect(err).ToNot(HaveOccurred())
						if ts.After(t) {
							return false
						}
					}

					for _, s := range caSecrets {
						nba := s.Annotations["auth.openshift.io/certificate-not-before"]
						t, err := time.Parse(time.RFC3339, nba)
						Expect(err).ToNot(HaveOccurred())
						naa := s.Annotations["auth.openshift.io/certificate-not-after"]
						t2, err := time.Parse(time.RFC3339, naa)
						Expect(err).ToNot(HaveOccurred())
						if t2.Sub(t) < time.Minute*20 {
							fmt.Fprintf(GinkgoWriter, "Not-Before (%s) should be 20 minutes before Not-After (%s)\n", nba, naa)
							return false
						}
						if t2.Sub(t)-(time.Minute*20) > time.Second {
							fmt.Fprintf(GinkgoWriter, "Not-Before (%s) should be 20 minutes before Not-After (%s) with 1 second toleration\n", nba, naa)
							return false
						}
						// 20m - 5m = 15m
						validateCertConfig(&s, "20m0s", "15m0s")
					}

					for _, s := range serverSecrets {
						nba := s.Annotations["auth.openshift.io/certificate-not-before"]
						t, err := time.Parse(time.RFC3339, nba)
						Expect(err).ToNot(HaveOccurred())
						naa := s.Annotations["auth.openshift.io/certificate-not-after"]
						t2, err := time.Parse(time.RFC3339, naa)
						Expect(err).ToNot(HaveOccurred())
						if t2.Sub(t) < time.Minute*5 {
							fmt.Fprintf(GinkgoWriter, "Not-Before (%s) should be 5 minutes before Not-After (%s)\n", nba, naa)
							return false
						}
						if t2.Sub(t)-(time.Minute*5) > time.Second {
							fmt.Fprintf(GinkgoWriter, "Not-Before (%s) should be 5 minutes before Not-After (%s) with 1 second toleration\n", nba, naa)
							return false
						}
						// 5m - 2m = 3m
						validateCertConfig(&s, "5m0s", "3m0s")
					}

					return true
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
			})
		})

		var _ = Describe("Priority class tests", func() {
			var (
				cdi                   *cdiv1.CDI
				systemClusterCritical = cdiv1.CDIPriorityClass("system-cluster-critical")
				osUserCrit            = &schedulev1.PriorityClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourcesutils.CDIPriorityClass,
					},
					Value: 10000,
				}
			)
			f := framework.NewFramework("operator-priority-class-test")
			verifyPodPriorityClass := func(prefix, priorityClassName, labelSelector string) {
				Eventually(func() string {
					controllerPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, prefix, labelSelector)
					if err != nil {
						return ""
					}
					return controllerPod.Spec.PriorityClassName
				}, 2*time.Minute, 1*time.Second).Should(BeEquivalentTo(priorityClassName))
			}

			BeforeEach(func() {
				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())
				cdi = cr
				if cr.Spec.PriorityClass != nil {
					By(fmt.Sprintf("Current priority class is: [%s]", *cr.Spec.PriorityClass))
				}
			})

			AfterEach(func() {
				if cdi == nil {
					return
				}

				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				cr.Spec.PriorityClass = cdi.Spec.PriorityClass

				_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				if !utils.IsOpenshift(f.K8sClient) {
					Eventually(func() bool {
						return errors.IsNotFound(f.K8sClient.SchedulingV1().PriorityClasses().Delete(context.TODO(), osUserCrit.Name, metav1.DeleteOptions{}))
					}, 2*time.Minute, 1*time.Second).Should(BeTrue())
				}
				By("Ensuring the CDI priority class is restored")
				prioClass := ""
				if cr.Spec.PriorityClass != nil {
					prioClass = string(*cr.Spec.PriorityClass)
				}
				// Deployment
				verifyPodPriorityClass(cdiDeploymentPodPrefix, string(prioClass), common.CDILabelSelector)
				// API server
				verifyPodPriorityClass(cdiApiServerPodPrefix, string(prioClass), "")
				// Upload server
				verifyPodPriorityClass(cdiUploadProxyPodPrefix, string(prioClass), "")
				By("Verifying there is just a single cdi controller pod")
				Eventually(func() error {
					_, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiDeploymentPodPrefix, common.CDILabelSelector)
					return err
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
				By("Ensuring this pod is the leader")
				Eventually(func() bool {
					controllerPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiDeploymentPodPrefix, common.CDILabelSelector)
					Expect(err).ToNot(HaveOccurred())
					log := getLog(f, controllerPod.Name)
					return checkLogForRegEx(logIsLeaderRegex, log)
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())

			})

			It("should use kubernetes priority class if set", func() {
				cr, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				Expect(err).ToNot(HaveOccurred())
				By("Setting the priority class to system cluster critical, which is known to exist")
				cr.Spec.PriorityClass = &systemClusterCritical
				_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cr, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				By("Verifying the CDI deployment is updated")
				verifyPodPriorityClass(cdiDeploymentPodPrefix, string(systemClusterCritical), common.CDILabelSelector)
				By("Verifying the CDI api server is updated")
				verifyPodPriorityClass(cdiApiServerPodPrefix, string(systemClusterCritical), "")
				By("Verifying the CDI upload proxy server is updated")
				verifyPodPriorityClass(cdiUploadProxyPodPrefix, string(systemClusterCritical), "")
			})

			It("should use openshift priority class if not set and available", func() {
				_, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
				if errors.IsNotFound(err) {
					Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
				}
				_, err = f.K8sClient.SchedulingV1().PriorityClasses().Create(context.TODO(), osUserCrit, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				By("Verifying the CDI control plane is updated")
				// Deployment
				verifyPodPriorityClass(cdiDeploymentPodPrefix, string(osUserCrit.Name), common.CDILabelSelector)
				// API server
				verifyPodPriorityClass(cdiApiServerPodPrefix, string(osUserCrit.Name), "")
				// Upload server
				verifyPodPriorityClass(cdiUploadProxyPodPrefix, string(osUserCrit.Name), "")
			})
		})
	})
})

func tolerationExists(tolerations []corev1.Toleration, testValue corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if reflect.DeepEqual(toleration, testValue) {
			return true
		}
	}
	return false
}

func nodeHasTaint(node corev1.Node, testedTaint corev1.Taint) bool {
	for _, taint := range node.Spec.Taints {
		if reflect.DeepEqual(taint, testedTaint) {
			return true
		}
	}
	return false
}

func infraDeploymentAvailable(f *framework.Framework, cr *cdiv1.CDI) bool {
	cdi, _ := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
	if !conditions.IsStatusConditionTrue(cdi.Status.Conditions, conditions.ConditionAvailable) {
		return false
	}

	for _, deploymentName := range []string{"cdi-apiserver", "cdi-deployment", "cdi-uploadproxy"} {
		_, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false
		}
	}

	return true
}

func infraDeploymentGone(f *framework.Framework) bool {
	for _, deploymentName := range []string{"cdi-apiserver", "cdi-deployment", "cdi-uploadproxy"} {
		_, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			return false
		}
	}
	return true
}

func crGone(f *framework.Framework, cr *cdiv1.CDI) bool {
	_, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
	return errors.IsNotFound(err)
}

func cdiOperatorDeploymentGone(f *framework.Framework) bool {
	_, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), "cdi-operator", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return true
	}
	Expect(err).ToNot(HaveOccurred())
	return false
}

func updateUninstallStrategy(client cdiClientset.Interface, strategy *cdiv1.CDIUninstallStrategy) *cdiv1.CDIUninstallStrategy {
	By("Getting CDI resource")
	cdis, err := client.CdiV1beta1().CDIs().List(context.TODO(), metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(cdis.Items).To(HaveLen(1))

	cdi := &cdis.Items[0]
	result := cdi.Spec.UninstallStrategy

	cdi.Spec.UninstallStrategy = strategy
	_, err = client.CdiV1beta1().CDIs().Update(context.TODO(), cdi, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())

	By("Waiting for update")
	Eventually(func() bool {
		cdi, err = client.CdiV1beta1().CDIs().Get(context.TODO(), cdi.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		if strategy == nil {
			return cdi.Spec.UninstallStrategy == nil
		}
		return cdi.Spec.UninstallStrategy != nil && *cdi.Spec.UninstallStrategy == *strategy
	}, 2*time.Minute, 1*time.Second).Should(BeTrue())

	return result
}

func scaleDeployment(f *framework.Framework, deploymentName string, replicas int32) int32 {
	operatorDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	originalReplicas := *operatorDeployment.Spec.Replicas
	operatorDeployment.Spec.Replicas = &[]int32{replicas}[0]
	_, err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(context.TODO(), operatorDeployment, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return originalReplicas
}

func checkForRunbookURL(rule promv1.Rule) {
	url, ok := rule.Annotations["runbook_url"]
	ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("%s does not have runbook_url annotation", rule.Alert))
	resp, err := http.Head(url)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), fmt.Sprintf("%s runbook is not available", rule.Alert))
	ExpectWithOffset(1, resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("%s runbook is not available", rule.Alert))
}

func checkForSummary(rule promv1.Rule) {
	summary, ok := rule.Annotations["summary"]
	ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("%s does not have summary annotation", rule.Alert))
	ExpectWithOffset(1, summary).ToNot(BeEmpty(), fmt.Sprintf("%s has an empty summary", rule.Alert))
}

func checkForSeverityLabel(rule promv1.Rule) {
	severity, ok := rule.Labels["severity"]
	ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("%s does not have severity label", rule.Alert))
	ExpectWithOffset(1, severity).To(BeElementOf("info", "warning", "critical"), fmt.Sprintf("%s severity label is not valid", rule.Alert))
}

func checkForPartOfLabel(rule promv1.Rule) {
	kubernetesOperatorPartOf, ok := rule.Labels["kubernetes_operator_part_of"]
	ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("%s does not have kubernetes_operator_part_of label", rule.Alert))
	ExpectWithOffset(1, kubernetesOperatorPartOf).To(Equal("kubevirt"), fmt.Sprintf("%s kubernetes_operator_part_of label is not valid", rule.Alert))
}

func checkForComponentLabel(rule promv1.Rule) {
	kubernetesOperatorComponent, ok := rule.Labels["kubernetes_operator_component"]
	ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("%s does not have kubernetes_operator_component label", rule.Alert))
	ExpectWithOffset(1, kubernetesOperatorComponent).To(Equal("containerized-data-importer"), fmt.Sprintf("%s kubernetes_operator_component label is not valid", rule.Alert))
}
