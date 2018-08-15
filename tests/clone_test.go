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
	TestNS  = "default"
	TestPOD = "test-pod"
)

var _ = Describe("Cloning", func() {
	var (
		pvc *k8sv1.PersistentVolumeClaim
	)

	flag.Parse()
	client, err := tests.GetKubeClient()
	tests.PanicOnError(err)

	BeforeEach(func() {
		pvc = tests.CreatePVC(TestNS, "clone-pvc", "1Mi")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		tests.DeletePVC(TestNS, pvc)
		client.CoreV1().Pods(TestNS).Delete(TestPOD, nil)
	})

	Specify("New PVC should be empty", func() {
		check := "[ $(ls -1 /pvc | wc -l) == 0 ]"

		tests.RunPodWithPVC(TestNS, TestPOD, pvc, check)

		By("Checking the pod state")
		Eventually(func() k8sv1.PodPhase {
			pod, err := client.CoreV1().Pods(TestNS).Get(TestPOD, k8smetav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return pod.Status.Phase
		}, 30, 1).Should(Equal(k8sv1.PodSucceeded))
	})

})

func runWithPVC(pvcName string, cmd string) (string, int) {
	return "", 1
}
