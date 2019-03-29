package importer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Upload data source", func() {
	var (
		ud     *UploadDataSource
		tmpDir string
		err    error
	)

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		if ud != nil {
			ud.Close()
		}
		os.RemoveAll(tmpDir)
	})

	It("Info should return Error, when passed in an invalid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		ud = NewUploadDataSource(file)
		result, err := ud.Info()
		Expect(err).To(HaveOccurred())
		Expect(Error).To(Equal(result))
	})

	It("Info should return Transfer, when passed in a valid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		ud = NewUploadDataSource(file)
		result, err := ud.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(Transfer).To(Equal(result))
	})

	table.DescribeTable("calling transfer should", func(fileName, scratchPath string, want []byte, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		sourceFile, err := os.Open(fileName)
		Expect(err).NotTo(HaveOccurred())

		ud = NewUploadDataSource(sourceFile)
		nextPhase, err := ud.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(Transfer).To(Equal(nextPhase))
		result, err := ud.Transfer(scratchPath)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(Process))
			file, err := os.Open(filepath.Join(scratchPath, tempFile))
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			fileStat, err := file.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(int64(len(want))).To(Equal(fileStat.Size()))
			resultBuffer, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
			Expect(file.Name()).To(Equal(ud.GetURL().String()))
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("return Error with missing scratch space", cirrosFilePath, "/imaninvalidpath", nil, true),
		table.Entry("return Process with scratch space and valid qcow file", cirrosFilePath, "", cirrosData, false),
	)

	It("Transfer should fail on reader error", func() {
		sourceFile, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())

		ud = NewUploadDataSource(sourceFile)
		nextPhase, err := ud.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(Transfer).To(Equal(nextPhase))
		err = sourceFile.Close()
		Expect(err).NotTo(HaveOccurred())
		result, err := ud.Transfer(tmpDir)
		Expect(err).To(HaveOccurred())
		Expect(Error).To(Equal(result))
	})

	It("Process should return Convert", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		ud = NewUploadDataSource(file)
		result, err := ud.Process()
		Expect(err).NotTo(HaveOccurred())
		Expect(Convert).To(Equal(result))
	})

	It("Close with nil stream should not fail", func() {
		ud = NewUploadDataSource(nil)
		err := ud.Close()
		Expect(err).NotTo(HaveOccurred())
	})
})
