package tests_test

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/reporters"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	pollInterval     = 2 * time.Second
	nsDeletedTimeout = 270 * time.Second
)

var (
	kubectlPath    = flag.String("kubectl-path", "kubectl", "The path to the kubectl binary")
	ocPath         = flag.String("oc-path", "oc", "The path to the oc binary")
	cdiInstallNs   = flag.String("cdi-namespace", "cdi", "The namespace of the CDI controller")
	kubeConfig     = flag.String("kubeconfig", "/var/run/kubernetes/admin.kubeconfig", "The absolute path to the kubeconfig file")
	master         = flag.String("master", "", "master url:port")
	goCLIPath      = flag.String("gocli-path", "cli.sh", "The path to cli script")
	snapshotSCName = flag.String("snapshot-sc", "", "The Storage Class supporting snapshots")
	blockSCName    = flag.String("block-sc", "", "The Storage Class supporting block mode volumes")
)

func TestTests(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(tests.CDIFailHandler)
	BuildTestSuite()
	RunSpecsWithDefaultAndCustomReporters(t, "Tests Suite", reporters.NewReporters())
}

// To understand the order in which things are run, read http://onsi.github.io/ginkgo/#understanding-ginkgos-lifecycle
// flag parsing happens AFTER ginkgo has constructed the entire testing tree. So anything that uses information from flags
// cannot work when called during test tree construction.
func BuildTestSuite() {
	BeforeSuite(func() {
		fmt.Fprintf(ginkgo.GinkgoWriter, "Reading parameters\n")
		// Read flags, and configure client instances
		framework.ClientsInstance.KubectlPath = *kubectlPath
		framework.ClientsInstance.OcPath = *ocPath
		framework.ClientsInstance.CdiInstallNs = *cdiInstallNs
		framework.ClientsInstance.KubeConfig = *kubeConfig
		framework.ClientsInstance.Master = *master
		framework.ClientsInstance.GoCLIPath = *goCLIPath
		framework.ClientsInstance.SnapshotSCName = *snapshotSCName
		framework.ClientsInstance.BlockSCName = *blockSCName

		fmt.Fprintf(ginkgo.GinkgoWriter, "Kubectl path: %s\n", framework.ClientsInstance.KubectlPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "OC path: %s\n", framework.ClientsInstance.OcPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "CDI install NS: %s\n", framework.ClientsInstance.CdiInstallNs)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Kubeconfig: %s\n", framework.ClientsInstance.KubeConfig)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Master: %s\n", framework.ClientsInstance.Master)
		fmt.Fprintf(ginkgo.GinkgoWriter, "GO CLI path: %s\n", framework.ClientsInstance.GoCLIPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Snapshot SC: %s\n", framework.ClientsInstance.SnapshotSCName)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Block SC: %s\n", framework.ClientsInstance.BlockSCName)

		restConfig, err := framework.ClientsInstance.LoadConfig()
		if err != nil {
			// Can't use Expect here due this being called outside of an It block, and Expect
			// requires any calls to it to be inside an It block.
			ginkgo.Fail("ERROR, unable to load RestConfig")
		}
		framework.ClientsInstance.RestConfig = restConfig
		// clients
		kcs, err := framework.ClientsInstance.GetKubeClient()
		if err != nil {
			ginkgo.Fail("ERROR, unable to create K8SClient")
		}
		framework.ClientsInstance.K8sClient = kcs

		cs, err := framework.ClientsInstance.GetCdiClient()
		if err != nil {
			ginkgo.Fail("ERROR, unable to create CdiClient")
		}
		framework.ClientsInstance.CdiClient = cs

		extcs, err := framework.ClientsInstance.GetExtClient()
		if err != nil {
			ginkgo.Fail("ERROR, unable to create CsiClient")
		}
		framework.ClientsInstance.ExtClient = extcs

		crClient, err := framework.ClientsInstance.GetCrClient()
		if err != nil {
			ginkgo.Fail("ERROR, unable to create CrClient")
		}
		framework.ClientsInstance.CrClient = crClient

		utils.CacheTestsData(framework.ClientsInstance.K8sClient, framework.ClientsInstance.CdiInstallNs)
	})

	AfterSuite(func() {
		Eventually(func() []corev1.Namespace {
			nsList, _ := utils.GetTestNamespaceList(framework.ClientsInstance.K8sClient, framework.NsPrefixLabel)
			return nsList.Items
		}, nsDeletedTimeout, pollInterval).Should(BeEmpty())
	})
}
