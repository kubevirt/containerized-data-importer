package importer

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

var _ = Describe("Parse endpoints", func() {
	var envURL, _ = url.Parse("http://www.google.com")
	var validURL, _ = url.Parse("http://www.bing.com")

	BeforeEach(func() {
		os.Setenv(common.ImporterEndpoint, envURL.String())
	})

	AfterEach(func() {
		os.Unsetenv(common.ImporterEndpoint)
	})

	DescribeTable("with env set", func(ep string, expect *url.URL, wantErr bool) {
		result, err := ParseEndpoint(ep)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(expect).To(Equal(result))
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		Entry("successfully get url, with valid ep", "http://www.bing.com", validURL, false),
		Entry("successfully get env url, with blank ep", "", envURL, false),
		Entry("fail to get url, with invalid ep", "htdsd://@#$%&%$^@#%%$&", nil, true),
	)

	It("with env set to specific value", func() {
		os.Setenv(common.ImporterEndpoint, "")
		_, err := ParseEndpoint("")
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "is missing or blank")).To(BeTrue())
	})

})

var _ = Describe("Clean dir", func() {
	var (
		err    error
		tmpDir string
	)

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "stream")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("Should be okay to delete a non existing file", func() {
		err = CleanAll("/invalid")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should clean a file", func() {
		f := filepath.Join(tmpDir, "newfile1")
		_, err = os.Create(f)
		Expect(err).NotTo(HaveOccurred())
		dir, err := os.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(1).To(Equal(len(dir)))
		err = CleanAll(f)
		Expect(err).NotTo(HaveOccurred())
		dir, err = os.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(0).To(Equal(len(dir)))
	})

	It("Should clean a directory", func() {
		td := filepath.Join(tmpDir, "xx")
		err = os.Mkdir(td, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Create(filepath.Join(td, "newfile1"))
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Create(filepath.Join(td, "newfile2"))
		Expect(err).NotTo(HaveOccurred())
		dir, err := os.ReadDir(td)
		Expect(err).NotTo(HaveOccurred())
		Expect(2).To(Equal(len(dir)))
		err = CleanAll(td)
		Expect(err).NotTo(HaveOccurred())
		dir, err = os.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(0).To(Equal(len(dir)))
	})
})

var _ = Describe("Env vars to labels", func() {
	It("Should convert KUBEVIRT_IO_ env vars to labels with values", func() {
		envs := []string{
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE=u1.small",
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE=fedora",
		}
		labels := envsToLabels(envs)
		Expect(labels).To(HaveLen(2))
		Expect(labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-instancetype", "u1.small"))
		Expect(labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-preference", "fedora"))
	})

	It("Should ignore invalid env vars and env vars without KUBEVIRT_IO_", func() {
		envs := []string{
			"SOMETHING_ELSE_IO=testval",
			"NOTAVALIDENVVAR",
		}
		Expect(envsToLabels(envs)).To(BeEmpty())
	})

	DescribeTable("Should correctly convert an env var to a label key", func(env, label string) {
		Expect(envToLabel(env)).To(Equal(label))
	},
		Entry("Simple suffix", "KUBEVIRT_IO_TEST", "kubevirt.io/test"),
		Entry("Double suffix", "KUBEVIRT_IO_TEST_TEST", "kubevirt.io/test-test"),
		Entry("Simple prefix", "TEST_KUBEVIRT_IO_TEST", "test.kubevirt.io/test"),
		Entry("Double prefix", "TEST_TEST_KUBEVIRT_IO_TEST", "test.test.kubevirt.io/test"),
		Entry("Double prefix and suffix", "TEST_TEST_KUBEVIRT_IO_TEST_TEST", "test.test.kubevirt.io/test-test"),
	)
})

// For use in transfer cancellation unit tests, currently VDDK/ImageIO
var mockTerminationChannel chan os.Signal

func createMockTerminationChannel() <-chan os.Signal {
	mockTerminationChannel = make(chan os.Signal, 1)
	return mockTerminationChannel
}
