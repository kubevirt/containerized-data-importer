package tests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
)

var _ = Describe("Explain tests", func() {
	f := framework.NewFramework("explain-test", framework.Config{
		SkipNamespaceCreation: true,
	})

	It("[test_id:4964]explain should have descriptions for CDI", func() {
		out, err := tests.RunKubectlCommand(f, "explain", "CDI")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("CDIStatus defines the status of the CDI installation"))
		out, err = tests.RunKubectlCommand(f, "explain", "CDI.status")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("The desired version of the CDI resource"))
	})

	It("[test_id:4965]explain should have descriptions for CDIConfig", func() {
		out, err := tests.RunKubectlCommand(f, "explain", "CDIConfig")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("CDIConfigStatus provides the most recently observed status of the CDI"))
		out, err = tests.RunKubectlCommand(f, "explain", "CDIConfig.status")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("The calculated upload proxy URL"))
	})

	It("[test_id:4966]explain should have descriptions for Datavolume", func() {
		out, err := tests.RunKubectlCommand(f, "explain", "dv")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("DataVolumeStatus contains the current status of the DataVolume"))
		out, err = tests.RunKubectlCommand(f, "explain", "dv.status")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("RestartCount is the number of times the pod populating the DataVolume has"))
	})
})
