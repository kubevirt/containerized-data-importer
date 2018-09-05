package tests_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	testSuiteName           = "Importer Test Suite"
	namespacePrefix         = "importer"
	bedEndpoint             = "http://gopats.com/who-is-the goat.iso"
	endpoint                = "http://distro.ibiblio.org/tinycorelinux/9.x/x86/release/Core-current.iso" 
	waitForCDIImportTimeout = 30 * time.Second
)

var _ = Describe(testSuiteName, func() {
	f := framework.NewFrameworkOrDie(namespacePrefix)

	It("Should not perform CDI operations on PVC without annotations", func() {
		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition("no-import", "1G", nil, nil))
		Expect(err).ToNot(HaveOccurred())
		// Wait a while to see if CDI puts anything in the PVC.
		time.Sleep(waitForCDIImportTimeout)
		Expect(framework.VerifyPVCIsEmpty(f, pvc)).To(BeTrue())
		// Not deleting PVC as it will be removed with the NS removal.
	})
	
	It("Import pod status should be Fail on unavailable endpoint", func() {
		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(
		"no-import", 
		"1G", 
		map[string]string{controller.AnnEndpoint: bedEndpoint}, 
		nil))
		
		Expect(err).ToNot(HaveOccurred())
		// Wait a while to see if CDI puts anything in the PVC.
		time.Sleep(waitForCDIImportTimeout)
		By("Verify the pod status is Failed on the target PVC")
		status, phaseAnnotation, err := utils.WaitForPVCAnnotation(f.K8sClient, f.Namespace.Name, pvc, controller.AnnPodPhase)
		Expect(phaseAnnotation).To(BeTrue())
		Expect(status).Should(BeEquivalentTo(v1.PodFailed))
	})
})
