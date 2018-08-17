package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/tests"
)

const (
	testSuiteName   = "Importer Test Suite"
	namespacePrefix = "importer"
)

var _ = Describe(testSuiteName, func() {
	f := tests.NewFramework(namespacePrefix)

	Describe("Creating standard PVC should not trigger importer", func() {
		It("PVC should remain empty after being bound", func() {
			pvc := f.CreatePVCFromDefinition(f.GetNamespace().GetName(), tests.NewPVCDefinition("no-import", "1G", nil))
			// PVC will be pending until bound to a pod, this test will fail if bound by a pod.
			// NOTE: not 100% sure if this will work with non local volume storage, those might get bound without having
			// a pod attached.
			err := f.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, pvc.GetName())
			Expect(err).ToNot(BeNil())
			// Can't verify until after the PVC Pending check has completed or our pod will bind the PVC.
			Expect(f.VerifyPVCIsEmpty(pvc)).To(BeTrue())
		})
	})
})
