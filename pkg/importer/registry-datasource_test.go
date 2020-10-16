package importer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var (
	imageFile = filepath.Join(imageDir, "registry-image.tar")
)

var _ = Describe("Registry data source", func() {
	var tmpDir string
	var err error
	var ds *RegistryDataSource

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
		if ds != nil {
			err = ds.Close()
			Expect(err).NotTo(HaveOccurred())
			ds = nil
		}
	})

	It("should return transfer after info is called", func() {
		ds = NewRegistryDataSource("", "", "", "", true)
		result, err := ds.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	table.DescribeTable("Transfer should ", func(ep, accKey, secKey, certDir, scratchPath string, insecureRegistry bool, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		ds = NewRegistryDataSource(ep, accKey, secKey, certDir, insecureRegistry)

		// Need to pass in a real path if we don't want scratch space needed error.
		result, err := ds.Transfer(scratchPath)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseConvert).To(Equal(result))
			Expect(filepath.Join(scratchPath, containerDiskImageDir)).To(Equal(ds.imageDir))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		table.Entry("successfully return Convert on valid scratch space and empty user parameters", "oci-archive:"+imageFile, "", "", "", "", true, false),
		table.Entry("successfully return Convert on valid scratch space and parameters", "oci-archive:"+imageFile, "username", "password", "/path/to/cert", "", true, false),
		table.Entry("return Error on invalid scratch space", "oci-archive:"+imageFile, "", "", "", "/invalid", true, true),
		table.Entry("return Error on valid scratch space, but CopyImage failed", "invalid", "", "", "", "", true, true),
	)

	It("TransferFile should not be called", func() {
		ds = NewRegistryDataSource("", "", "", "", true)
		result, err := ds.TransferFile("file")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("getImageFileName should return an error with non-existing image directory", func() {
		_, err := getImageFileName("/invalid")
		Expect(err).To(HaveOccurred())
		Expect("image directory does not exist").To(Equal(err.Error()))
	})

	It("getImageFileName should return an error with invalid image directory", func() {
		file, err := os.Create(filepath.Join(tmpDir, "test"))
		Expect(err).NotTo(HaveOccurred())
		_, err = getImageFileName(file.Name())
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "image file does not exist in image directory")).To(BeTrue())
	})

	It("getImageFileName should return an error with empty image directory", func() {
		err := os.Mkdir(filepath.Join(tmpDir, containerDiskImageDir), os.ModeDir)
		Expect(err).NotTo(HaveOccurred())
		_, err = getImageFileName(filepath.Join(tmpDir, containerDiskImageDir))
		Expect(err).To(HaveOccurred())
		Expect("image file does not exist in image directory - directory is empty").To(Equal(err.Error()))
	})

	It("getImageFileName should return an error with image directory containing another directory", func() {
		err := os.Mkdir(filepath.Join(tmpDir, containerDiskImageDir), os.ModeDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.Mkdir(filepath.Join(tmpDir, containerDiskImageDir, "anotherdir"), os.ModeDir)
		Expect(err).NotTo(HaveOccurred())
		_, err = getImageFileName(filepath.Join(tmpDir, containerDiskImageDir))
		Expect(err).To(HaveOccurred())
		Expect("image directory contains another directory").To(Equal(err.Error()))
	})

	It("getImageFileName should return an error with zero length filename", func() {
		err := os.Mkdir(filepath.Join(tmpDir, containerDiskImageDir), os.ModeDir)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Create(filepath.Join(tmpDir, containerDiskImageDir, " "))
		Expect(err).NotTo(HaveOccurred())
		_, err = getImageFileName(filepath.Join(tmpDir, containerDiskImageDir))
		Expect(err).To(HaveOccurred())
		Expect("image file does has no name").To(Equal(err.Error()))
	})

	It("getImageFileName should return an error with multiple files in the image directory", func() {
		err := os.Mkdir(filepath.Join(tmpDir, containerDiskImageDir), os.ModeDir)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Create(filepath.Join(tmpDir, containerDiskImageDir, "extra-file"))
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Create(filepath.Join(tmpDir, containerDiskImageDir, "disk.img"))
		Expect(err).NotTo(HaveOccurred())
		_, err = getImageFileName(filepath.Join(tmpDir, containerDiskImageDir))
		Expect(err).To(HaveOccurred())
		Expect("image directory contains more than one file").To(Equal(err.Error()))
	})
})
