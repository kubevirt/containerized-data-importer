package tests_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	testSuiteName           = "Importer Test Suite"
	namespacePrefix         = "importer"
	waitForCDIImportTimeout = 30 * time.Second
)

var _ = Describe(testSuiteName, func() {
	f, err := framework.NewFramework(namespacePrefix)
	if err != nil {
		Fail("Unable to create framework struct")
	}

	It("Should not perform CDI operations on PVC without annotations", func() {
		pvc, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition("no-import", "1G", nil, nil))
		Expect(err).ToNot(HaveOccurred())
		// Wait a while to see if CDI puts anything in the PVC.
		time.Sleep(waitForCDIImportTimeout)
		Expect(framework.VerifyPVCIsEmpty(f, pvc)).To(BeTrue())
		// Not deleting PVC as it will be removed with the NS removal.
	})
})
