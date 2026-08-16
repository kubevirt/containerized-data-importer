package importer

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containers/image/v5/types"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	imageFile = filepath.Join(imageDir, "registry-image.tar")
)

var _ = Describe("Registry data source", func() {
	var tmpDir string
	var err error
	var ds *RegistryDataSource

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "scratch")
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
		ds = NewRegistryDataSource("", "", "", "", "", true)
		result, err := ds.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	DescribeTable("pod pull Transfer should ", func(ep, accKey, secKey, certDir, scratchPath string, insecureRegistry bool, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		ds = NewRegistryDataSource(ep, accKey, secKey, "", certDir, insecureRegistry)

		// Need to pass in a real path if we don't want scratch space needed error.
		result, err := ds.Transfer(scratchPath, false)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseConvert).To(Equal(result))
			Expect(filepath.Join(scratchPath, containerDiskImageDir)).To(Equal(ds.imageDir))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		Entry("successfully return Convert on valid scratch space and empty user parameters", "oci-archive:"+imageFile, "", "", "", "", true, false),
		Entry("successfully return Convert on valid scratch space and parameters", "oci-archive:"+imageFile, "username", "password", "/path/to/cert", "", true, false),
		Entry("return Error on invalid scratch space", "oci-archive:"+imageFile, "", "", "", "/invalid", true, true),
		Entry("return Error on valid scratch space, but CopyImage failed", "invalid", "", "", "", "", true, true),
	)

	It("node pull Transfer should successfully return Convert on valid environment variables", func() {
		diskDir := filepath.Join(tmpDir, containerDiskImageDir)
		Expect(os.Mkdir(diskDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(diskDir, "disk.img"), []byte("image"), 0600)).To(Succeed())

		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		GinkgoT().Setenv(common.ImporterImageRootDir, tmpDir)

		ds = NewRegistryDataSource("", "", "", "", "", true)

		result, err := ds.Transfer(tmpDir, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseConvert).To(Equal(result))
		Expect(filepath.Join(tmpDir, containerDiskImageDir)).To(Equal(ds.imageDir))
	})

	It("TransferFile should not be called", func() {
		ds = NewRegistryDataSource("", "", "", "", "", true)
		result, err := ds.TransferFile("file", false)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	DescribeTable("GetTerminationMessage should contain labels collected from the image", func(pullMethod string) {
		ds = NewRegistryDataSource("", "", "", "", "", true)
		envVariables := []string{
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE=u1.small",
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE=fedora",
		}
		if pullMethod == "node" {
			envFile := path.Join(tmpDir, "env-file")
			GinkgoT().Setenv(common.ImporterPullMethod, pullMethod)
			GinkgoT().Setenv(common.ImporterImageRootDir, tmpDir)
			GinkgoT().Setenv(common.ImporterEnvFile, envFile)
			envString := strings.Join(envVariables, "\n")
			err := os.WriteFile(envFile, []byte(envString), 0600)
			Expect(err).NotTo(HaveOccurred())
		} else {
			ds.info = &types.ImageInspectInfo{
				Env: envVariables,
			}
		}

		termMesg := ds.GetTerminationMessage()
		Expect(termMesg).ToNot(BeNil())
		Expect(termMesg.Labels).To(HaveLen(2))
		Expect(termMesg.Labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-instancetype", "u1.small"))
		Expect(termMesg.Labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-preference", "fedora"))
	},
		Entry("when pull method = pod", "pod"),
		Entry("when pull method = node", "node"),
	)

	It("Transfer should return error for bootc image when pull = pod", func() {
		ds = NewRegistryDataSource("oci-archive:"+filepath.Join(imageDir, "bootc-registry-image.tar"), "", "", "", "", true)
		result, err := ds.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bootc image detected"))
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Transfer should return error for bootc image when pull = node and ostree folder exists in imageRootDir", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		GinkgoT().Setenv(common.ImporterImageRootDir, tmpDir)
		err := os.Mkdir(filepath.Join(tmpDir, "ostree"), 0755)
		Expect(err).NotTo(HaveOccurred())
		ds = NewRegistryDataSource("oci-archive:"+filepath.Join(imageDir, "bootc-registry-image.tar"), "", "", "", "", true)
		result, err := ds.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bootc image detected"))
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Transfer should return error for bootc image when pull = node and sysroot folder exists in imageRootDir", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		GinkgoT().Setenv(common.ImporterImageRootDir, tmpDir)
		err := os.Mkdir(filepath.Join(tmpDir, "sysroot"), 0755)
		Expect(err).NotTo(HaveOccurred())
		ds = NewRegistryDataSource("oci-archive:"+filepath.Join(imageDir, "bootc-registry-image.tar"), "", "", "", "", true)
		result, err := ds.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bootc image detected"))
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Transfer should return error when imageRootDir is empty", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		GinkgoT().Setenv(common.ImporterImageRootDir, "")
		ds = NewRegistryDataSource("", "", "", "", "", true)
		result, err := ds.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("GetTerminationMessage should return nil when ImporterEnvFile is empty", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		GinkgoT().Setenv(common.ImporterEnvFile, "")
		ds = NewRegistryDataSource("", "", "", "", "", true)
		termMesg := ds.GetTerminationMessage()
		Expect(termMesg).To(BeNil())
	})

	It("Transfer should return error when imageRootDir is not set", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		ds = NewRegistryDataSource("", "", "", "", "", true)
		result, err := ds.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("GetTerminationMessage should return nil when ImporterEnvFile is not set", func() {
		GinkgoT().Setenv(common.ImporterPullMethod, "node")
		ds = NewRegistryDataSource("", "", "", "", "", true)
		termMesg := ds.GetTerminationMessage()
		Expect(termMesg).To(BeNil())
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
