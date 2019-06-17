package tests_test

import (
	"fmt"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	cdiDeploymentName = "cdi-deployment"
	cdiOperatorName   = "cdi-operator"
	newDeploymentName = "cdi-new-deployment"

	pollingInterval = 2 * time.Second
	timeout         = 90 * time.Second
)

var (
	logCheckLeaderRegEx  = regexp.MustCompile("Attempting to acquire leader lease")
	logIsLeaderRegex     = regexp.MustCompile("Successfully acquired leadership lease")
	logImporterStarting  = regexp.MustCompile("Validating image")
	logImporterCompleted = regexp.MustCompile("\\] \\d\\d\\.\\d{1,2}")

	// These are constants we want to take the pointer of
	disableOperator = int32(0)
	enableOperator  = int32(1)
	scale2          = int32(2)
)

func checkLogForRegEx(regEx *regexp.Regexp, log string) bool {
	matches := regEx.FindAllStringIndex(log, -1)
	return len(matches) >= 1
}

var _ = Describe("[rfe_id:1250][crit:high][vendor:cnv-qe@redhat.com][level:component]Leader election tests", func() {
	f := framework.NewFrameworkOrDie("leaderelection-test")

	AfterEach(func() {
		deployments := getDeployments(f)
		if len(deployments.Items) > 1 {
			d := &appsv1.Deployment{}
			d.Name = newDeploymentName

			err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(newDeploymentName, &metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []appsv1.Deployment {
				return getDeployments(f).Items
			}, timeout, pollingInterval).Should(HaveLen(1))
		}
	})

	It("[test_id:1365]Should only ever be one controller as leader", func() {
		By("Check only one CDI controller deployment/pod")
		deployments := getDeployments(f)
		Expect(deployments.Items).Should(HaveLen(1))

		pods := getPods(f)
		Expect(pods.Items).Should(HaveLen(1))

		leaderPodName := pods.Items[0].Name

		log := getLog(f, leaderPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeTrue())

		newDeployment := deployments.Items[0].DeepCopy()
		newDeployment.ObjectMeta = metav1.ObjectMeta{Name: newDeploymentName}
		newDeployment.Status = appsv1.DeploymentStatus{}

		By("Creating new controller deployment")
		newDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Create(newDeployment)
		Expect(err).ToNot(HaveOccurred())

		var newPodName string

		Eventually(func() bool {
			pods := getPods(f)
			if len(pods.Items) != 2 {
				return false
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase != "Running" {
					return false
				}

				if pod.Name != leaderPodName {
					newPodName = pod.Name
				}
			}
			return true

		}, timeout, pollingInterval).Should(BeTrue())
		Expect(newPodName).ShouldNot(BeEmpty())

		By("Check that new pod is attempting to become leader")
		Eventually(func() bool {
			log := getLog(f, newPodName)
			return checkLogForRegEx(logCheckLeaderRegEx, log)
		}, timeout, pollingInterval).Should(BeTrue())

		// have to prove new pod won't become leader in period longer than lease duration
		time.Sleep(20 * time.Second)

		By("Confirm pod did not become leader")
		log = getLog(f, newPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeFalse())

		By("Delete new deployment")
		err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(newDeploymentName, &metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Confirm deployment deleted")
		Eventually(func() bool {
			deployments := getDeployments(f)
			return len(deployments.Items) == 1
		}, timeout, pollingInterval).Should(BeTrue())
	})
})

var _ = Describe("[rfe_id:1250][crit:high][test_id:1889][vendor:cnv-qe@redhat.com][level:component]Leader election tests during import", func() {
	f := framework.NewFrameworkOrDie("leaderelection-test")

	BeforeEach(func() {
		By("Unsubscribing from OLM, so we can stop the operator")
		// TODO once we have olm integration completed.

		By("Stopping operator we can increase the controller deployment to two to test leader election steps")
		stopCDIOperator(f)

		By("Increasing the scale of the CDI deployment to 2, we now have 2 controller pods")
		scaleUp(f, scale2)
	})

	AfterEach(func() {
		By("Restoring the operator back to 1 it will fix the things we broke during the test")
		operatorDp := getOperatorDeployment(f)
		if operatorDp.Spec.Replicas != &enableOperator {
			// Set operator replica count to 0
			operatorDp.Spec.Replicas = &enableOperator
			updatedDp, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(operatorDp)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedDp.Spec.Replicas).To(Equal(&enableOperator))
		}
		// Find the existing operator pod.
		_, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiOperatorName, "")
		Expect(err).ToNot(HaveOccurred())

		// Wait for single cdi controller pod to exist.
		By("Waiting for single cdi controller pod to exists, we have restored normal operations")
		Eventually(func() bool {
			pods := getPods(f)
			return len(pods.Items) == 1
		}, timeout, pollingInterval).Should(BeTrue())
	})

	It("Should not not interrupt an import while switching leaders", func() {
		var importer *v1.Pod
		var leaderPodName, secondPodName string

		By("Determining which pod is leader")
		controllerPods := getPods(f)
		Expect(controllerPods.Items).Should(HaveLen(2))
		for _, controllerPod := range controllerPods.Items {
			log := getLog(f, controllerPod.Name)
			if checkLogForRegEx(logIsLeaderRegex, log) {
				leaderPodName = controllerPod.Name
			} else {
				secondPodName = controllerPod.Name
			}
		}
		Expect(leaderPodName).ToNot(Equal(""))
		Expect(secondPodName).ToNot(Equal(""))

		By("Starting slow import, we can monitor if it gets interrupted during leader changes")
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPRateLimitPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
			controller.AnnSecret:   "",
		}

		_, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, utils.NewPVCDefinition("import-e2e", "20M", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			importer, err = utils.FindPodByPrefix(f.K8sClient, f.Namespace.Name, common.ImporterPodName, common.CDILabelSelector)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", f.Namespace.Name+"/"+common.ImporterPodName))
			return importer.Status.Phase == v1.PodRunning || importer.Status.Phase == v1.PodSucceeded
		}, timeout, pollingInterval).Should(BeTrue())

		Eventually(func() bool {
			log, err := tests.RunKubectlCommand(f, "logs", importer.Name, "-n", f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			return checkLogForRegEx(logImporterStarting, log)
		}, timeout, pollingInterval).Should(BeTrue())

		// The import is starting, and the transfer is about to happen. Now kill the leader
		By("Killing leader, we should have a new leader elected")
		err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).Delete(leaderPodName, &metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			log := getLog(f, secondPodName)
			return checkLogForRegEx(logIsLeaderRegex, log)
		}, timeout, pollingInterval).Should(BeTrue())

		By("Verifying imported pod has progressed without issue")
		Eventually(func() bool {
			log, err := tests.RunKubectlCommand(f, "logs", importer.Name, "-n", f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			return checkLogForRegEx(logImporterCompleted, log)
		}, timeout, pollingInterval).Should(BeTrue())

	})
})

func getDeployments(f *framework.Framework) *appsv1.DeploymentList {
	deployments, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).List(metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
	Expect(err).ToNot(HaveOccurred())
	return deployments
}

func getOperatorDeployment(f *framework.Framework) *appsv1.Deployment {
	deployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(cdiOperatorName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return deployment
}

func getPods(f *framework.Framework) *v1.PodList {
	pods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
	Expect(err).ToNot(HaveOccurred())
	return pods
}

func getLog(f *framework.Framework, name string) string {
	log, err := tests.RunKubectlCommand(f, "logs", name, "-n", f.CdiInstallNs)
	Expect(err).ToNot(HaveOccurred())
	return log
}

func stopCDIOperator(f *framework.Framework) {
	// Find the existing operator pod.
	operatorPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiOperatorName, "")
	Expect(err).ToNot(HaveOccurred())
	operatorPodName := operatorPod.Name

	operatorDp := getOperatorDeployment(f)
	Expect(operatorDp.Spec.Replicas).To(Equal(&enableOperator))
	// Set operator replica count to 0
	operatorDp.Spec.Replicas = &disableOperator
	updatedDp, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(operatorDp)
	Expect(err).ToNot(HaveOccurred())
	Expect(updatedDp.Spec.Replicas).To(Equal(&disableOperator))

	operatorDp = getOperatorDeployment(f)
	//Make sure the operator pod is gone.
	Expect(operatorDp.Spec.Replicas).To(Equal(&disableOperator))
	Eventually(func() bool {
		pod, _ := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).Get(operatorPodName, metav1.GetOptions{})
		return pod == nil || pod.Name == ""
	}, timeout, pollingInterval).Should(BeTrue())
}

func scaleUp(f *framework.Framework, scale int32) {
	deployments := getDeployments(f)
	Expect(deployments.Items).Should(HaveLen(1))

	pods := getPods(f)
	Expect(pods.Items).Should(HaveLen(1))

	deployments.Items[0].Spec.Replicas = &scale
	updatedDp, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(&deployments.Items[0])
	Expect(err).ToNot(HaveOccurred())
	Expect(updatedDp.Spec.Replicas).To(Equal(&scale))
	Eventually(func() bool {
		deployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(updatedDp.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return deployment.Status.AvailableReplicas == scale
	}, timeout, pollingInterval).Should(BeTrue())
}
