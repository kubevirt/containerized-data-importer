package tests_test

import (
	"flag"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/containerized-data-importer/tests"
)

const (
	TestNS = "default"
)

var _ = Describe("Cloning", func() {
	var (
		err error
	)

	flag.Parse()

	BeforeEach(func() {
		tests.CreatePVC(TestNS, "src", "1Mi")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		tests.DeletePVC(TestNS, "src")
	})

	Specify("New source PVC should be empty", func() {
		check := "[ $(ls -1 /pvc | wc -l) == 0 ]"
		client, err := tests.GetKubeClient()
		tests.PanicOnError(err)

		defer client.CoreV1().Pods(TestNS).Delete("src", nil)
		tests.RunPodWithPVC(TestNS, "src", "src", check)

		By("Checking the pod state")
		Eventually(func() k8sv1.PodPhase {
			pod, err := client.CoreV1().Pods(TestNS).Get("src", k8smetav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return pod.Status.Phase
		}, 30, 1).Should(Equal(k8sv1.PodSucceeded))
	})

})

func runWithPVC(pvcName string, cmd string) (string, int) {
	return "", 1
}
