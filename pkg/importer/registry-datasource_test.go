package importer

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
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

	table.DescribeTable("Transfer should ", func(ep, accKey, secKey, certDir, scratchPath string, insecureRegistry bool, skopeoOperations image.SkopeoOperations, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		ds = NewRegistryDataSource(ep, accKey, secKey, certDir, insecureRegistry)
		By("Replacing Skopeo Operations")
		replaceSkopeoOperations(skopeoOperations, func() {
			// Need to pass in a real path if we don't want scratch space needed error.
			result, err := ds.Transfer(scratchPath)
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
				Expect(ProcessingPhaseProcess).To(Equal(result))
				Expect(filepath.Join(scratchPath, containerDiskImageDir)).To(Equal(ds.imageDir))
			} else {
				Expect(err).To(HaveOccurred())
				Expect(ProcessingPhaseError).To(Equal(result))
			}
		})
	},
		table.Entry("successfully return Process on valid scratch space and empty user parameters", "endpoint", "", "", "", "", true, NewFakeSkopeoOperations("endpoint", "", "", "", true, nil), false),
		table.Entry("successfully return Process on valid scratch space and parameters", "endpoint", "username", "password", "/path/to/cert", "", true, NewFakeSkopeoOperations("endpoint", "username", "password", "/path/to/cert", true, nil), false),
		table.Entry("return Error on invalid scratch space", "endpoint", "", "", "", "/invalid", true, NewFakeSkopeoOperations("endpoint", "", "", "", true, nil), true),
		table.Entry("return Error on valid scratch space, but CopyImage failed", "endpoint", "", "", "", "", true, NewSkopeoAllErrors(), true),
	)

	table.DescribeTable("Process should ", func(scratchPath string, wantErr bool) {
		ds = NewRegistryDataSource("", "", "", "", true)
		if scratchPath == "" {
			scratchPath = tmpDir
			err := os.Mkdir(filepath.Join(scratchPath, containerDiskImageDir), os.ModeDir)
			Expect(err).NotTo(HaveOccurred())
			err = util.CopyFile(cirrosFilePath, filepath.Join(scratchPath, containerDiskImageDir, cirrosFileName))
			Expect(err).NotTo(HaveOccurred())
		}
		ds.imageDir = filepath.Join(scratchPath, containerDiskImageDir)
		result, err := ds.Process()
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseConvert).To(Equal(result))
			imageFileName, err := url.Parse(filepath.Join(scratchPath, containerDiskImageDir, cirrosFileName))
			Expect(err).NotTo(HaveOccurred())
			Expect(imageFileName).To(Equal(ds.GetURL()))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		table.Entry("successfully return Convert on valid image file", "", false),
		table.Entry("return Error on invalid image file", "/invalid", true),
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

type fakeSkopeoOperations struct {
	ep               string
	accKey           string
	secKey           string
	certDir          string
	insecureRegistry bool
	e1               error
}

func replaceSkopeoOperations(replacement image.SkopeoOperations, f func()) {
	orig := image.SkopeoInterface
	if replacement != nil {
		image.SkopeoInterface = replacement
		defer func() { image.SkopeoInterface = orig }()
	}
	f()
}

func NewSkopeoAllErrors() image.SkopeoOperations {
	err := errors.New("skopeo should not be called from this test override with replaceSkopeoOperations")
	return &fakeSkopeoOperations{
		e1: err,
	}
}

func NewFakeSkopeoOperations(ep, accKey, secKey, certDir string, insecureRegistry bool, e1 error) image.SkopeoOperations {
	return &fakeSkopeoOperations{
		ep:               ep,
		accKey:           accKey,
		secKey:           secKey,
		certDir:          certDir,
		insecureRegistry: insecureRegistry,
		e1:               e1,
	}
}

func (o *fakeSkopeoOperations) CopyImage(url, dest, accessKey, secKey, certDir string, insecureRegistry bool) error {
	if o.e1 != nil {
		return o.e1
	}
	dest = filepath.Dir(dest)
	dest = strings.Replace(dest, "dir:", "", 1)
	if err := util.UnArchiveLocalTar(imageFile, dest); err != nil {
		return errors.Wrap(err, "could not extract layer tar")
	}
	Expect(o.ep).To(Equal(url))
	Expect(o.accKey).To(Equal(accessKey))
	Expect(o.secKey).To(Equal(secKey))
	Expect(o.certDir).To(Equal(certDir))
	Expect(o.insecureRegistry).To(Equal(insecureRegistry))
	return nil
}
