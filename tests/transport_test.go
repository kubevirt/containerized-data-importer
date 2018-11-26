package tests

import (
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Transport Tests", func() {

	const (
		secretPrefix   = "transport-e2e-sec"
		targetFile     = "tinyCore.iso"
		targetQCOWFile = "tinyCore.qcow2"
		sizeCheckPod   = "size-checker"
	)

	var (
		ns  string
		f   = framework.NewFrameworkOrDie("transport", framework.Config{SkipNamespaceCreation: false})
		c   = f.K8sClient
		sec *v1.Secret
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
		By(fmt.Sprintf("Waiting for all \"%s/%s\" deployment replicas to be Ready", utils.FileHostNs, utils.FileHostName))
		utils.WaitForDeploymentReplicasReadyOrDie(c, utils.FileHostNs, utils.FileHostName)
	})

	// it() is the body of the test and is executed once per Entry() by DescribeTable()
	// closes over c and ns
	it := func(ep, file, accessKey, secretKey string, shouldSucceed bool) {

		var (
			err error // prevent shadowing
		)

		pvcAnn := map[string]string{
			controller.AnnEndpoint: ep + "/" + file,
			controller.AnnSecret:   "",
		}

		if accessKey != "" || secretKey != "" {
			By(fmt.Sprintf("Creating secret for endpoint %s", ep))
			if accessKey == "" {
				accessKey = utils.AccessKeyValue
			}
			if secretKey == "" {
				secretKey = utils.SecretKeyValue
			}
			stringData := make(map[string]string)
			stringData[common.KeyAccess] = accessKey
			stringData[common.KeySecret] = secretKey

			sec, err = utils.CreateSecretFromDefinition(c, utils.NewSecretDefinition(nil, stringData, nil, ns, secretPrefix))
			Expect(err).NotTo(HaveOccurred(), "Error creating test secret")
			pvcAnn[controller.AnnSecret] = sec.Name
		}

		By(fmt.Sprintf("Creating PVC with endpoint annotation %q", pvcAnn[controller.AnnEndpoint]))
		pvc, err := utils.CreatePVCFromDefinition(c, ns, utils.NewPVCDefinition("transport-e2e", "20M", pvcAnn, nil))
		Expect(err).NotTo(HaveOccurred(), "Error creating PVC")

		importer, err := utils.FindPodByPrefix(c, ns, common.ImporterPodName, common.CDILabelSelector)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Unable to get importer pod %q", ns+"/"+common.ImporterPodName))

		err = utils.WaitTimeoutForPodStatus(c, importer.Name, importer.Namespace, v1.PodSucceeded, utils.PodWaitForTime)

		if shouldSucceed {
			By("Verifying PVC is not empty")
			Expect(framework.VerifyPVCIsEmpty(f, pvc)).To(BeFalse(), fmt.Sprintf("Found 0 imported files on PVC %q", pvc.Namespace+"/"+pvc.Name))

			pod, err := utils.CreateExecutorPodWithPVC(c, sizeCheckPod, ns, pvc)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitTimeoutForPodReady(c, sizeCheckPod, ns, 20*time.Second)).To(Succeed())

			command := `expSize=20971520; haveSize=$(wc -c < /pvc/disk.img); (( $expSize == $haveSize )); echo $?`
			exitCode := f.ExecShellInPod(pod.Name, ns, command)
			// A 0 exitCode should indicate that $expSize == $haveSize
			Expect(strconv.Atoi(exitCode)).To(BeZero())
		} else {
			By("Verifying PVC is empty")
			Expect(framework.VerifyPVCIsEmpty(f, pvc)).To(BeTrue(), fmt.Sprintf("Found 0 imported files on PVC %q", pvc.Namespace+"/"+pvc.Name))
		}
	}

	httpNoAuthEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+utils.FileHostNs, utils.HTTPNoAuthPort)
	httpAuthEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+utils.FileHostNs, utils.HTTPAuthPort)
	DescribeTable("Transport Test Table", it,
		Entry("should connect to http endpoint without credentials", httpNoAuthEp, targetFile, "", "", true),
		Entry("should connect to http endpoint with credentials", httpAuthEp, targetFile, utils.AccessKeyValue, utils.SecretKeyValue, true),
		Entry("should not connect to http endpoint with invalid credentials", httpAuthEp, targetFile, "gopats", "bradyisthegoat", false),
		Entry("should connect to QCOW http endpoint without credentials", httpNoAuthEp, targetQCOWFile, "", "", true),
		Entry("should connect to QCOW http endpoint with credentials", httpAuthEp, targetQCOWFile, utils.AccessKeyValue, utils.SecretKeyValue, true))
})
