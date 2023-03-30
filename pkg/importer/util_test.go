package importer

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
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

	table.DescribeTable("with env set", func(ep string, expect *url.URL, wantErr bool) {
		result, err := ParseEndpoint(ep)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(expect).To(Equal(result))
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("successfully get url, with valid ep", "http://www.bing.com", validURL, false),
		table.Entry("successfully get env url, with blank ep", "", envURL, false),
		table.Entry("fail to get url, with invalid ep", "htdsd://@#$%&%$^@#%%$&", nil, true),
	)

	It("with env set to specific value", func() {
		os.Setenv(common.ImporterEndpoint, "")
		_, err := ParseEndpoint("")
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "is missing or blank")).To(BeTrue())
	})

})

var _ = Describe("Stream Data To File", func() {
	var (
		err    error
		tmpDir string
	)

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "stream")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	table.DescribeTable("should", func(fileName string, useTmpDir bool, r io.Reader, errMsg string, wantErr bool) {
		if useTmpDir {
			fileName = filepath.Join(tmpDir, fileName)
		}
		err = util.StreamDataToFile(r, fileName)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), errMsg)).To(BeTrue())
		}
	},
		table.Entry("succeed with valid reader and filename", "valid", true, strings.NewReader("test reader"), "", false),
		table.Entry("fail with valid reader and invalid filename", "/invalidpath/invalidfile", false, strings.NewReader("test reader"), "no such file or directory", true),
	)
})

var _ = Describe("Clean dir", func() {
	var (
		err    error
		tmpDir string
	)

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "stream")
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
		dir, err := ioutil.ReadDir(tmpDir)
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
		dir, err = ioutil.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(0).To(Equal(len(dir)))
	})
})

// For use in transfer cancellation unit tests, currently VDDK/ImageIO
var mockTerminationChannel chan os.Signal

func createMockTerminationChannel() <-chan os.Signal {
	mockTerminationChannel = make(chan os.Signal, 1)
	return mockTerminationChannel
}
