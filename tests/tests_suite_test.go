package tests_test

import (
	"flag"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/reporters"
	"kubevirt.io/containerized-data-importer/tests/utils"
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
		// Read flags, and configure client instances
		framework.ClientsInstance.KubectlPath = *kubectlPath
		framework.ClientsInstance.OcPath = *ocPath
		framework.ClientsInstance.CdiInstallNs = *cdiInstallNs
		framework.ClientsInstance.KubeConfig = *kubeConfig
		framework.ClientsInstance.Master = *master
		framework.ClientsInstance.GoCLIPath = *goCLIPath
		framework.ClientsInstance.SnapshotSCName = *snapshotSCName
		framework.ClientsInstance.BlockSCName = *blockSCName

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
}
