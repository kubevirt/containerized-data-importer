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

var _ = Describe("Leader election tests", func() {
	var err error
	var deployment *appsv1.Deployment
	var leaderPodName string

	f := framework.NewFrameworkOrDie("leaderelection-test")

	getDeployment := func() *appsv1.Deployment {
		deployment, err := f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Get(cdiDeploymentName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return deployment
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
		deployment = getDeployment()
		if *deployment.Spec.Replicas > 1 {
			n := int32(1)
			deployment.Spec.Replicas = &n
			_, err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(deployment)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1.Pod {
				return getPods().Items
			}, timeout, pollingInterval).Should(HaveLen(1))
		}
	})

	It("Should be one active CDI controller", func() {
		deployment = getDeployment()
		Expect(*deployment.Spec.Replicas).Should(Equal(int32(1)))

		pods := getPods()
		Expect(pods.Items).Should(HaveLen(1))

		leaderPodName = pods.Items[0].Name

		log := getLog(leaderPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeTrue())
	})

	It("Scale controller deployment", func() {
		numReplicas := int32(2)
		deployment.Spec.Replicas = &numReplicas

		deployment, err = f.K8sClient.AppsV1().Deployments(f.CdiInstallNs).Update(deployment)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			pods := getPods()
			if len(pods.Items) != 2 {
				return false
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase != "Running" {
					return false
				}
			}
			return true

		}, timeout, pollingInterval).Should(BeTrue())

		newPodName := ""
		pods := getPods().Items
		Expect(pods).To(HaveLen(2))
		for _, pod := range getPods().Items {
			if pod.Name != leaderPodName {
				newPodName = pod.Name
				break
			}
		}
		Expect(newPodName).ShouldNot(Equal(""))

		Eventually(func() bool {
			log := getLog(newPodName)
			return checkLogForRegEx(logCheckLeaderRegEx, log)
		}, timeout, pollingInterval).Should(BeTrue())

		// have to prove new pod won't become leader in period longer than lease duration
		time.Sleep(20 * time.Second)

		log := getLog(newPodName)
		Expect(checkLogForRegEx(logIsLeaderRegex, log)).To(BeFalse())
	})

	It("Check scale back down", func() {
		deployment = getDeployment()
		Expect(*deployment.Spec.Replicas).Should(Equal(int32(1)))

		Eventually(func() bool {
			pods := getPods()
			Expect(pods.Items).Should(HaveLen(1))
			log := getLog(pods.Items[0].Name)
			return checkLogForRegEx(logIsLeaderRegex, log)
		}, timeout, pollingInterval).Should(BeTrue())
	})
})
