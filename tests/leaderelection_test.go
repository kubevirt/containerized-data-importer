package tests_test

import (
	"regexp"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

const (
	cdiDeploymentName = "cdi-deployment"
	newDeploymentName = "cdi-deployment-new"

	pollingInterval = 2 * time.Second
	timeout         = 90 * time.Second
)

var (
	logCheckLeaderRegEx = regexp.MustCompile("Attempting to acquire leader lease")
	logIsLeaderRegex    = regexp.MustCompile("Successfully acquired leadership lease")
)

func checkLogForRegEx(regEx *regexp.Regexp, log string) bool {
	matches := regEx.FindAllStringIndex(log, -1)
	return len(matches) == 1
}

var _ = Describe("[rfe_id:1250][crit:high][vendor:cnv-qe@redhat.com][level:component]Leader election tests", func() {
	f := framework.NewFrameworkOrDie("leaderelection-test")

	getDeployments := func() *appsv1.DeploymentList {
		deployments, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).List(metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
		Expect(err).ToNot(HaveOccurred())
		return deployments
	}

	getPods := func() *v1.PodList {
		pods, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(metav1.ListOptions{LabelSelector: "app=containerized-data-importer"})
		Expect(err).ToNot(HaveOccurred())
		return pods
	}

	getLog := func(name string) string {
		log, err := tests.RunKubectlCommand(f, "logs", name, "-n", f.CdiInstallNs)
		Expect(err).ToNot(HaveOccurred())
		return log
	}

	AfterEach(func() {
		deployments := getDeployments()
		if len(deployments.Items) > 1 {
			d := &appsv1.Deployment{}
			d.Name = newDeploymentName

			err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(newDeploymentName, &metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []appsv1.Deployment {
				return getDeployments().Items
			}, timeout, pollingInterval).Should(HaveLen(1))
		}
	})

	It("[test_id:1365]Should only ever be one controller as leader", func() {
		By("Check only one CDI controller deployment/pod")
		deployments := getDeployments()
		Expect(deployments.Items).Should(HaveLen(1))

		pods := getPods()
		Expect(pods.Items).Should(HaveLen(1))

		leaderPodName := pods.Items[0].Name

		log := getLog(leaderPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeTrue())

		newDeployment := deployments.Items[0].DeepCopy()
		newDeployment.ObjectMeta = metav1.ObjectMeta{Name: newDeploymentName}
		newDeployment.Status = appsv1.DeploymentStatus{}

		By("Creating new controller deployment")
		newDeployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Create(newDeployment)
		Expect(err).ToNot(HaveOccurred())

		var newPodName string

		Eventually(func() bool {
			pods := getPods()
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
			log := getLog(newPodName)
			return checkLogForRegEx(logCheckLeaderRegEx, log)
		}, timeout, pollingInterval).Should(BeTrue())

		// have to prove new pod won't become leader in period longer than lease duration
		time.Sleep(20 * time.Second)

		By("Confirm pod did not become leader")
		log = getLog(newPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeFalse())

		By("Delete new deployment")
		err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Delete(newDeploymentName, &metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Confirm deployment deleted")
		Eventually(func() bool {
			deployments := getDeployments()
			return len(deployments.Items) == 1
		}, timeout, pollingInterval).Should(BeTrue())
	})
})
