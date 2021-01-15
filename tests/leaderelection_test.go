package tests_test

import (
	"context"
	"fmt"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	cdiDeploymentName      = "cdi-deployment"
	cdiDeploymentPodPrefix = "cdi-deployment-"
	cdiOperatorName        = "cdi-operator"
	cdiOperatorPodPrefix   = "cdi-operator-"
	newDeploymentName      = "cdi-new-deployment"

	pollingInterval = 2 * time.Second
	timeout         = 360 * time.Second
)

var (
	logCheckLeaderRegEx  = regexp.MustCompile("Attempting to acquire leader lease")
	logIsLeaderRegex     = regexp.MustCompile("Successfully acquired leadership lease")
	logImporterStarting  = regexp.MustCompile("Converting to Raw")
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
	f := framework.NewFramework("leaderelection-test")
	var (
		leaderPodName string
		newDeployment *appsv1.Deployment
	)

	BeforeEach(func() {
		leaderPodName, newDeployment = getLeaderAndNewDeployment(f)
	})

	AfterEach(func() {
		cleanupTest(f, newDeployment)
	})

	It("[test_id:1365]Should only ever be one controller as leader", func() {
		Expect(leaderPodName).ShouldNot(BeEmpty())

		newPodName := locateNewPod(f, leaderPodName)
		Expect(newPodName).ShouldNot(BeEmpty())

		By("Check that new pod is attempting to become leader")
		Eventually(func() bool {
			log := getLog(f, newPodName)
			return checkLogForRegEx(logCheckLeaderRegEx, log)
		}, timeout, pollingInterval).Should(BeTrue())

		// have to prove new pod won't become leader in period longer than lease duration
		time.Sleep(20 * time.Second)

		By("Confirm pod did not become leader")
		log := getLog(f, newPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeFalse())
	})
})

var _ = Describe("[rfe_id:1250][crit:high][test_id:1889][vendor:cnv-qe@redhat.com][level:component]Leader election tests during import", func() {
	f := framework.NewFramework("leaderelection-test")
	var (
		leaderPodName string
		newDeployment *appsv1.Deployment
	)

	BeforeEach(func() {
		leaderPodName, newDeployment = getLeaderAndNewDeployment(f)
	})

	AfterEach(func() {
		cleanupTest(f, newDeployment)
	})

	It("Should not not interrupt an import while switching leaders", func() {
		Expect(leaderPodName).ShouldNot(BeEmpty())
		var importer *v1.Pod

		newPodName := locateNewPod(f, leaderPodName)
		Expect(newPodName).ShouldNot(BeEmpty())

		By("Starting slow import, we can monitor if it gets interrupted during leader changes")
		httpEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+f.CdiInstallNs, utils.HTTPRateLimitPort)
		pvcAnn := map[string]string{
			controller.AnnEndpoint: httpEp + "/tinyCore.iso",
			controller.AnnSecret:   "",
		}

		pvc, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, utils.NewPVCDefinition("import-e2e", "40Mi", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred())
		f.ForceBindIfWaitForFirstConsumer(pvc)

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
		err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).Delete(context.TODO(), leaderPodName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to kill leader: %v", err))

		By("Verifying that the original leader pod is gone.")
		Eventually(func() bool {
			_, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).Get(context.TODO(), leaderPodName, metav1.GetOptions{})
			return err != nil && k8serrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())

		Eventually(func() bool {
			newDeploymentPodName := locateNewPod(f, newPodName)
			newDeploymentLog := ""
			if newDeploymentPodName != "" {
				newDeploymentLog = getLog(f, newDeploymentPodName)
			}
			log := getLog(f, newPodName)
			fmt.Fprintf(GinkgoWriter, "INFO: Lookin for: %s\n", logIsLeaderRegex)
			fmt.Fprintf(GinkgoWriter, "INFO: In new deployment pod log: %s\n", log)
			if newDeploymentLog != "" {
				fmt.Fprintf(GinkgoWriter, "INFO: In original leader pod log: %s\n", newDeploymentLog)
			}
			return checkLogForRegEx(logIsLeaderRegex, log) || checkLogForRegEx(logIsLeaderRegex, newDeploymentLog)
		}, timeout, pollingInterval).Should(BeTrue())

		By("Verifying imported pod has progressed without issue")
		Eventually(func() bool {
			log, err := tests.RunKubectlCommand(f, "logs", importer.Name, "-n", f.Namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			return checkLogForRegEx(logImporterCompleted, log)
		}, timeout, pollingInterval).Should(BeTrue())

	})
})

func getLeaderAndNewDeployment(f *framework.Framework) (string, *appsv1.Deployment) {
	By("Check only one CDI controller deployment/pod")
	deployments := getDeployments(f)
	Expect(deployments.Items).Should(HaveLen(1))
	var pods *v1.PodList
	Eventually(func() bool {
		pods = getPods(f)
		return len(pods.Items) == 1
	}, timeout, pollingInterval).Should(BeTrue())

	leaderPodName := pods.Items[0].Name

	log := getLog(f, leaderPodName)
	Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeTrue())

	newDeployment := deployments.Items[0].DeepCopy()
	newDeployment.ObjectMeta = metav1.ObjectMeta{Name: newDeploymentName}
	newDeployment.Status = appsv1.DeploymentStatus{}

	By("Creating new controller deployment")
	newDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Create(context.TODO(), newDeployment, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return leaderPodName, newDeployment
}

func locateNewPod(f *framework.Framework, leaderPodName string) string {
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
	return newPodName
}

func cleanupTest(f *framework.Framework, newDeployment *appsv1.Deployment) {
	if newDeployment != nil {
		By("Cleaning up new deployments")
		err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(context.TODO(), newDeployment.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() []appsv1.Deployment {
			return getDeployments(f).Items
		}, timeout, pollingInterval).Should(HaveLen(1))
	}
	By("Making sure we have only 1 pod")
	Eventually(func() bool {
		return len(getPods(f).Items) == 1
	}, timeout, pollingInterval).Should(BeTrue())

	leaderPod := getPods(f).Items[0]

	Eventually(func() bool {
		return checkLogForRegEx(logIsLeaderRegex, getLog(f, leaderPod.Name))
	}, timeout, pollingInterval).Should(BeTrue())
}

func getDeployments(f *framework.Framework) *appsv1.DeploymentList {
	deployments, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
	Expect(err).ToNot(HaveOccurred())
	return deployments
}

func getOperatorDeployment(f *framework.Framework) *appsv1.Deployment {
	deployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(context.TODO(), cdiOperatorName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	return deployment
}

func getPods(f *framework.Framework) *v1.PodList {
	pods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
	Expect(err).ToNot(HaveOccurred())
	return pods
}

func getLog(f *framework.Framework, name string) string {
	log, err := tests.RunKubectlCommand(f, "logs", name, "-n", f.CdiInstallNs)
	Expect(err).ToNot(HaveOccurred())
	return log
}
