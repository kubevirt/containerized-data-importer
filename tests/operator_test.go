package tests_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk"
	sdkapi "github.com/kubevirt/controller-lifecycle-operator-sdk/pkg/sdk/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	secclient "github.com/openshift/client-go/security/clientset/versioned"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Operator tests", func() {
	f := framework.NewFramework("operator-test")

	It("[test_id:3951]should create a route in OpenShift", func() {
		if !isOpenshift(f.K8sClient) {
			Skip("This test is OpenShift specific")
		}

		routeClient, err := routeclient.NewForConfig(f.RestConfig)
		Expect(err).ToNot(HaveOccurred())

		r, err := routeClient.RouteV1().Routes(f.CdiInstallNs).Get(context.TODO(), "cdi-uploadproxy", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(r.Spec.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))
	})

	It("should create a prometheus service in cdi namespace", func() {
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
		if !isOpenshift(f.K8sClient) {
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

	It("should deploy components that tolerate CriticalAddonsOnly taint", func() {
		var err error
		cdiPods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred(), "failed listing cdi pods")
		Expect(len(cdiPods.Items)).To(BeNumerically(">", 0), "no cdi pods found")

		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"cdi.kubevirt.io/testing": ""}}
		cdiTestPods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		})
		Expect(err).ToNot(HaveOccurred(), "failed listing cdi testing pods")
		Expect(len(cdiPods.Items)).To(BeNumerically(">", 0), "no cdi testing pods found")

		By("adding taints to all nodes")
		criticalPodTaint := corev1.Taint{
			Key:    "CriticalAddonsOnly",
			Value:  "",
			Effect: corev1.TaintEffectNoExecute,
		}

		nodes, err := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		Expect(nodes.Items).ToNot(BeEmpty(), "There should be some compute node")
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			var errors []error
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

			var newCdiTestPods *corev1.PodList
			By("Waiting for there to be as many CDI testing pods as before")
			Eventually(func() bool {
				newCdiTestPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
				})
				Expect(err).ToNot(HaveOccurred(), "failed getting CDI test pods")

				By(fmt.Sprintf("cdi test pod len: %d\n new cdi test pod len: %d\n", len(cdiTestPods.Items), len(newCdiTestPods.Items)))
				if len(cdiTestPods.Items) != len(newCdiTestPods.Items) {
					return false
				}
				return true
			}, 5*time.Minute, 2*time.Second).Should(BeTrue())

			for _, newCdiTestPod := range newCdiTestPods.Items {
				By(fmt.Sprintf("Waiting for CDI test pod %s to be ready", newCdiTestPod.Name))
				err := utils.WaitTimeoutForPodReady(f.K8sClient, newCdiTestPod.Name, newCdiTestPod.Namespace, 20*time.Minute)
				Expect(err).ToNot(HaveOccurred())
			}
		}()

		for _, node := range nodes.Items {
			nodeCopy := node.DeepCopy()
			if nodeHasTaint(node, criticalPodTaint) {
				continue
			}

			nodeCopy.Spec.Taints = append(node.Spec.Taints, criticalPodTaint)
			_, err = f.K8sClient.CoreV1().Nodes().Update(context.TODO(), nodeCopy, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred(), "failed setting taint on node")
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

var _ = Describe("Operator delete CDI tests", func() {
	var cr *cdiv1.CDI
	f := framework.NewFramework("operator-delete-cdi-test")

	BeforeEach(func() {
		var err error
		cr, err = f.CdiClient.CdiV1beta1().CDIs().Get(context.TODO(), "cdi", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
		}
		Expect(err).ToNot(HaveOccurred())
	})

	ensureCDI := func() {
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

	AfterEach(func() {
		ensureCDI()
	})

	It("should remove/install CDI a number of times successfully", func() {
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

var _ = Describe("[rfe_id:4784][crit:high] Operator deployment + CDI delete tests", func() {
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

//IsOpenshift checks if we are on OpenShift platform
func isOpenshift(client kubernetes.Interface) bool {
	//OpenShift 3.X check
	result := client.Discovery().RESTClient().Get().AbsPath("/oapi/v1").Do(context.TODO())
	var statusCode int
	result.StatusCode(&statusCode)

	if result.Error() == nil {
		// It is OpenShift
		if statusCode == http.StatusOK {
			return true
		}
	} else {
		// Got 404 so this is not Openshift 3.X, let's check OpenShift 4
		result = client.Discovery().RESTClient().Get().AbsPath("/apis/route.openshift.io").Do(context.TODO())
		var statusCode int
		result.StatusCode(&statusCode)

		if result.Error() == nil {
			// It is OpenShift
			if statusCode == http.StatusOK {
				return true
			}
		}
	}

	return false
}
