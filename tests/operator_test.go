package tests_test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	secclient "github.com/openshift/client-go/security/clientset/versioned"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("ALL Operator tests", func() {
	Context("[Destructive]", func() {
		var _ = Describe("Operator tests", func() {
			f := framework.NewFramework("operator-test")

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
				Expect(promService.Spec.Selector[common.PrometheusLabel]).To(Equal(""))
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
				Expect(promService.Spec.Selector[common.PrometheusLabel]).To(Equal(""))
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

			// Condition flags can be found here with their meaning https://github.com/kubevirt/hyperconverged-cluster-operator/blob/master/docs/conditions.md
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
					if len(cdiPods.Items) != len(newCdiPods.Items) {
						return false
					}
					return true
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
						if !k8serrors.IsNotFound(err) {
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

				cdi, err := f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
				if err == nil {
					if cdi.DeletionTimestamp == nil {
						cdi.Spec = cr.Spec
						_, err = f.CdiClient.CdiV1beta1().CDIs().Update(context.TODO(), cdi, metav1.UpdateOptions{})
						Expect(err).ToNot(HaveOccurred())
						return
					}

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

				cdi, err = f.CdiClient.CdiV1beta1().CDIs().Create(context.TODO(), cdi, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					cdi, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), cr.Name, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(cdi.Status.Phase).ShouldNot(Equal(sdkapi.PhaseError))
					if conditions.IsStatusConditionTrue(cdi.Status.Conditions, conditions.ConditionAvailable) {
						return true
					}
					return false
				}, 10*time.Minute, 2*time.Second).Should(BeTrue())

				By("Verifying CDI apiserver, deployment, uploadproxy exist, before continuing")
				Eventually(func() bool { return infraDeploymentAvailable(f, cr) }, CompletionTimeout, assertionPollInterval).Should(BeTrue(), "Timeout reading CDI deployments")

				By("Verifying CDI config object exists, before continuing")
				Eventually(func() bool {
					_, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
					if k8serrors.IsNotFound(err) {
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
					if len(cdiPods.Items) != len(newCdiPods.Items) {
						return false
					}
					return true
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

				By("Cannot delete CDI")
				err = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("there are still DataVolumes present"))

				err = f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Delete(context.TODO(), dv.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Can delete CDI")
				err = f.CdiClient.CdiV1beta1().CDIs().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{DryRun: []string{"All"}})
				Expect(err).ToNot(HaveOccurred())
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
					if k8serrors.IsNotFound(err) {
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
				localSpec.Infra = tests.TestNodePlacementValues(f)

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

					match := tests.PodSpecHasTestNodePlacementValues(f, deployment.Spec.Template.Spec)
					Expect(match).To(BeTrue(), fmt.Sprintf("node placement in pod spec\n%v\n differs from node placement values in CDI CR\n%v\n", deployment.Spec.Template.Spec, localSpec.Infra))
				}
			})
		})

		var _ = Describe("[vendor:cnv-qe@redhat.com][level:component]Strict Reconciliation tests", func() {
			f := framework.NewFramework("strict-reconciliation-test")

			It("[test_id:5573]cdi-deployment replicas back to original value on attempt to scale", func() {
				deploymentName := "cdi-deployment"
				cdiDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), deploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				originalReplicaVal := *cdiDeployment.Spec.Replicas

				By("Overwrite number of replicas with originalVal + 1")
				cdiDeployment.Spec.Replicas = &[]int32{originalReplicaVal + 1}[0]
				_, err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(context.TODO(), cdiDeployment, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

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

			It("[test_id:5576]ServiceAccount secrets restored on deletion attempt", func() {
				serviceAccount, err := f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Get(context.TODO(), common.ControllerServiceAccountName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Remove secrets from ServiceAccount")
				serviceAccount.Secrets = []corev1.ObjectReference{}

				_, err = f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					sa, err := f.K8sClient.CoreV1().ServiceAccounts(f.CdiInstallNs).Get(context.TODO(), common.ControllerServiceAccountName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					By("Waiting until secrets are repopulated")
					return len(sa.Secrets) != 0
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
							RenewBefore: &metav1.Duration{Duration: time.Minute * 10},
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
					}

					return true
				}, 2*time.Minute, 1*time.Second).Should(BeTrue())
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
	if !errors.IsNotFound(err) {
		return false
	}
	return true
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
