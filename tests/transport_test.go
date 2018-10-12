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
		ns string
		f  = framework.NewFrameworkOrDie("transport", framework.Config{SkipNamespaceCreation: false})
		c  = f.K8sClient
	)

	BeforeEach(func() {
		ns = f.Namespace.Name
		By(fmt.Sprintf("Waiting for all \"%s/%s\" deployment replicas to be Ready", utils.FileHostNs, utils.FileHostName))
		utils.WaitForDeploymentReplicasReadyOrDie(c, utils.FileHostNs, utils.FileHostName)
	})

	// it() is the body of the test and is executed once per Entry() by DescribeTable()
	it := func(ep, file, accessKey, secretKey string) {
		var (
			err error // prevent shadowing
			sec *v1.Secret
		)

		pvcAnn := map[string]string{
			controller.AnnEndpoint: ep + "/" + file,
			controller.AnnSecret:   "",
		}

		if accessKey != "" || secretKey != "" {
			By(fmt.Sprintf("Creating secret for endpoint %s", ep))
			stringData := map[string]string{
				common.KeyAccess: utils.AccessKeyValue,
				common.KeySecret: utils.SecretKeyValue,
			}
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
		if err != nil {
			PrintPodLog(f, importer.Name, ns)
		}

		By("Verifying PVC is not empty")
		Expect(framework.VerifyPVCIsEmpty(f, pvc)).To(BeFalse(), fmt.Sprintf("Found 0 imported files on PVC %q", pvc.Namespace+"/"+pvc.Name))

		pod, err := utils.CreateExecutorPodWithPVC(c, sizeCheckPod, ns, pvc)
		Expect(err).NotTo(HaveOccurred())
		Expect(utils.WaitTimeoutForPodReady(c, sizeCheckPod, ns, 20*time.Second)).To(Succeed())

		// Get the remote file size, the imported file size and compare them
		command := `expSize=$(curl -s http://cdi-file-host.kube-system | grep -E '"name":"tinyCore.iso"' | grep -Eo '"size":[0-9]+' | tr -d '":[:alpha:]'); haveSize=$(wc -c < /pvc/disk.img); (( $expSize == $haveSize )); echo $?`

		exitCode := f.ExecShellInPod(pod.Name, ns, command)
		// A 0 exitCode should indicate that $expSize == $haveSize
		Expect(strconv.Atoi(exitCode)).To(BeZero())
	}

	httpNoAuthEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+utils.FileHostNs, utils.HTTPNoAuthPort)
	httpAuthEp := fmt.Sprintf("http://%s:%d", utils.FileHostName+"."+utils.FileHostNs, utils.HTTPAuthPort)
	DescribeTable("Transport Test Table", it,
		Entry("should connect to http endpoint without credentials", httpNoAuthEp, targetFile, "", ""),
		Entry("should connect to http endpoint with credentials", httpAuthEp, targetFile, utils.AccessKeyValue, utils.SecretKeyValue),
		Entry("should connect to QCOW http endpoint without credentials", httpNoAuthEp, targetQCOWFile, "", ""),
		Entry("should connect to QCOW http endpoint with credentials", httpAuthEp, targetQCOWFile, utils.AccessKeyValue, utils.SecretKeyValue))
})
