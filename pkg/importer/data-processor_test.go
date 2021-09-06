package importer

import (
	"io/ioutil"
	"net/url"
	"os"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

type fakeInfoOpRetVal struct {
	imgInfo *image.ImgInfo
	e       error
}

const TestImagesDir = "../../tests/images"

const (
	SmallActualSize  = 1024 * 1024
	SmallVirtualSize = 1024 * 1024
)

var (
	fakeSmallImageInfo = image.ImgInfo{Format: "", BackingFile: "", VirtualSize: SmallVirtualSize, ActualSize: SmallActualSize}
	fakeZeroImageInfo  = image.ImgInfo{Format: "", BackingFile: "", VirtualSize: 0, ActualSize: 0}
	fakeInfoRet        = fakeInfoOpRetVal{imgInfo: &fakeSmallImageInfo, e: nil}
)

type fakeQEMUOperations struct {
	e2             error
	e3             error
	ret4           fakeInfoOpRetVal
	e5             error
	e6             error
	resizeQuantity *resource.Quantity
}

type MockDataProvider struct {
	infoResponse     ProcessingPhase
	transferResponse ProcessingPhase
	url              *url.URL
	transferPath     string
	transferFile     string
	calledPhases     []ProcessingPhase
	needsScratch     bool
}

// Info is called to get initial information about the data
func (m *MockDataProvider) Info() (ProcessingPhase, error) {
	m.calledPhases = append(m.calledPhases, ProcessingPhaseInfo)
	if m.infoResponse == ProcessingPhaseError {
		return ProcessingPhaseError, errors.New("Info errored")
	}
	return m.infoResponse, nil
}

// Transfer is called to transfer the data from the source to the passed in path.
func (m *MockDataProvider) Transfer(path string) (ProcessingPhase, error) {
	m.calledPhases = append(m.calledPhases, m.infoResponse)
	m.transferPath = path
	if m.transferResponse == ProcessingPhaseError {
		if m.needsScratch {
			return ProcessingPhaseError, ErrInvalidPath
		}
		return ProcessingPhaseError, errors.New("Transfer errored")
	}
	return m.transferResponse, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (m *MockDataProvider) TransferFile(fileName string) (ProcessingPhase, error) {
	m.calledPhases = append(m.calledPhases, ProcessingPhaseTransferDataFile)
	m.transferFile = fileName
	if m.transferResponse == ProcessingPhaseError {
		return ProcessingPhaseError, errors.New("TransferFile errored")
	}
	return m.transferResponse, nil
}

// Geturl returns the url that the data processor can use when converting the data.
func (m *MockDataProvider) GetURL() *url.URL {
	return m.url
}

// Close closes any readers or other open resources.
func (m *MockDataProvider) Close() error {
	return nil
}

type MockAsyncDataProvider struct {
	MockDataProvider
	ResumePhase ProcessingPhase
}

// Info is called to get initial information about the data.
func (madp *MockAsyncDataProvider) Info() (ProcessingPhase, error) {
	return madp.MockDataProvider.Info()
}

// Transfer is called to transfer the data from the source to the passed in path.
func (madp *MockAsyncDataProvider) Transfer(path string) (ProcessingPhase, error) {
	return madp.MockDataProvider.Transfer(path)
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (madp *MockAsyncDataProvider) TransferFile(fileName string) (ProcessingPhase, error) {
	return madp.MockDataProvider.TransferFile(fileName)
}

// Close closes any readers or other open resources.
func (madp *MockAsyncDataProvider) Close() error {
	return madp.MockDataProvider.Close()
}

// GetURL returns the url that the data processor can use when converting the data.
func (madp *MockAsyncDataProvider) GetURL() *url.URL {
	return madp.MockDataProvider.GetURL()
}

// GetResumePhase returns the next phase to process when resuming
func (madp *MockAsyncDataProvider) GetResumePhase() ProcessingPhase {
	return madp.ResumePhase
}

var _ = Describe("Data Processor", func() {
	It("should call the right phases based on the responses from the provider, Transfer should pass the scratch data dir as a path", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferScratch,
			transferResponse: ProcessingPhaseComplete,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		err := dp.ProcessData()
		Expect(err).ToNot(HaveOccurred())
		Expect(2).To(Equal(len(mdp.calledPhases)))
		Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
		Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))
		Expect("scratchDataDir").To(Equal(mdp.transferPath))
	})

	It("should call the right phases based on the responses from the provider, TransferTarget should pass the data dir as a path", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferDataDir,
			transferResponse: ProcessingPhaseComplete,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		err := dp.ProcessData()
		Expect(err).ToNot(HaveOccurred())
		Expect(2).To(Equal(len(mdp.calledPhases)))
		Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
		Expect(ProcessingPhaseTransferDataDir).To(Equal(mdp.calledPhases[1]))
		Expect("dataDir").To(Equal(mdp.transferPath))
	})

	It("should error on Transfer phase", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferScratch,
			transferResponse: ProcessingPhaseError,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		err := dp.ProcessData()
		Expect(err).To(HaveOccurred())
		Expect(2).To(Equal(len(mdp.calledPhases)))
		Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
		Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))
	})

	It("should error on Transfer phase if scratch space is required", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferScratch,
			transferResponse: ProcessingPhaseError,
			needsScratch:     true,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		err := dp.ProcessData()
		Expect(err).To(HaveOccurred())
		Expect(ErrRequiresScratchSpace).To(Equal(err))
		Expect(2).To(Equal(len(mdp.calledPhases)))
		Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
		Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))
	})

	It("should call the right phases based on the responses from the provider, TransferDataFile should pass the data file", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferDataFile,
			transferResponse: ProcessingPhaseComplete,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			err := dp.ProcessData()
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(mdp.calledPhases)))
			Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
			Expect(ProcessingPhaseTransferDataFile).To(Equal(mdp.calledPhases[1]))
		})
	})

	It("should fail when TransferDataFile fails", func() {
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferDataFile,
			transferResponse: ProcessingPhaseError,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewQEMUAllErrors()
		replaceQEMUOperations(qemuOperations, func() {
			err := dp.ProcessData()
			Expect(err).To(HaveOccurred())
			Expect(2).To(Equal(len(mdp.calledPhases)))
			Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
			Expect(ProcessingPhaseTransferDataFile).To(Equal(mdp.calledPhases[1]))
		})
	})

	It("should error on Unknown phase", func() {
		mdp := &MockDataProvider{
			infoResponse: ProcessingPhase("invalidphase"),
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		err := dp.ProcessData()
		Expect(err).To(HaveOccurred())
		Expect(1).To(Equal(len(mdp.calledPhases)))
		Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
	})

	It("should call Convert after Process phase", func() {
		tmpDir, err := ioutil.TempDir("", "scratch")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferScratch,
			transferResponse: ProcessingPhaseConvert,
			url:              url,
		}
		dp := NewDataProcessor(mdp, "", "dataDir", tmpDir, "1G", 0.055, false)
		dp.availableSpace = int64(1536000)
		usableSpace := dp.getUsableSpace()

		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(usableSpace, 1024*1024))
		replaceQEMUOperations(qemuOperations, func() {
			err = dp.ProcessData()
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(mdp.calledPhases)))
			Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
			Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))
			Expect(tmpDir).To(Equal(mdp.transferPath))
		})
	})

	table.DescribeTable("should avoid cleanup before delta copies", func(dataSource DataSourceInterface, expectedCleanup bool) {
		tmpDir, err := ioutil.TempDir("", "scratch")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		dp := NewDataProcessor(dataSource, "dest", "dataDir", tmpDir, "1G", 0.055, false)
		Expect(dp.needsDataCleanup).To(Equal(expectedCleanup))
	},
		table.Entry("ImageIO delta copy", &ImageioDataSource{currentSnapshot: "123", previousSnapshot: "123"}, false),
		table.Entry("ImageIO base copy", &ImageioDataSource{currentSnapshot: "123", previousSnapshot: ""}, true),
		table.Entry("VDDK delta copy", &VDDKDataSource{CurrentSnapshot: "123", PreviousSnapshot: "123"}, false),
		table.Entry("VDDK base copy", &VDDKDataSource{CurrentSnapshot: "123", PreviousSnapshot: ""}, true),
	)

	const (
		Mi              = int64(1024 * 1024)
		Gi              = 1024 * Mi
		noOverhead      = float64(0)
		defaultOverhead = float64(0.055)
		largeOverhead   = float64(0.75)
	)
	table.DescribeTable("getusablespace should return properly aligned sizes,", func(virtualSize int64, overhead float64) {
		for i := int64(virtualSize - 1024); i < virtualSize+1024; i++ {
			// Requested space is virtualSize rounded up to 1Mi alignment / (1 - overhead) rounded up
			requestedSpace := int64(float64(util.RoundUp(i, util.DefaultAlignBlockSize)+1) / float64(1-overhead))
			if i <= virtualSize {
				Expect(GetUsableSpace(overhead, requestedSpace)).To(Equal(virtualSize))
			} else {
				Expect(GetUsableSpace(overhead, requestedSpace)).To(Equal(virtualSize + Mi))
			}
		}
	},
		table.Entry("1Mi virtual size, 0 overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, noOverhead),
		table.Entry("1Mi virtual size, default overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, defaultOverhead),
		table.Entry("1Mi virtual size, large overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, largeOverhead),
		table.Entry("40Mi virtual size, 0 overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, noOverhead),
		table.Entry("40Mi virtual size, default overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, defaultOverhead),
		table.Entry("40Mi virtual size, large overhead to be 40Mi if <= 40Mi and 41Mi if > 40Mi", 40*Mi, largeOverhead),
		table.Entry("1Gi virtual size, 0 overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, noOverhead),
		table.Entry("1Gi virtual size, default overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, defaultOverhead),
		table.Entry("1Gi virtual size, large overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, largeOverhead),
		table.Entry("40Gi virtual size, 0 overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, noOverhead),
		table.Entry("40Gi virtual size, default overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, defaultOverhead),
		table.Entry("40Gi virtual size, large overhead to be 40Gi if <= 40Gi and 41Gi if > 40Gi", 40*Gi, largeOverhead),
	)
})

var _ = Describe("Convert", func() {
	It("Should successfully convert and return resize", func() {
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.convert(mdp.GetURL())
			Expect(err).ToNot(HaveOccurred())
			Expect(ProcessingPhaseResize).To(Equal(nextPhase))
		})
	})

	It("Should fail when validation fails and return Error", func() {
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, errors.New("Validation failure"), nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.convert(mdp.GetURL())
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(nextPhase))
		})
	})

	It("Should fail when conversion fails and return Error", func() {
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(errors.New("Conversion failure"), nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.convert(mdp.GetURL())
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(nextPhase))
		})
	})
})

var _ = Describe("Resize", func() {
	It("Should not resize and return complete, when requestedSize is blank", func() {
		tempDir, err := ioutil.TempDir(os.TempDir(), "dest")
		Expect(err).ToNot(HaveOccurred())
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, tempDir, "dataDir", "scratchDataDir", "", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.resize()
			Expect(err).ToNot(HaveOccurred())
			Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
		})
	})

	It("Should not resize and return complete, when requestedSize is valid, but datadir doesn't exist (block device)", func() {
		tempDir, err := ioutil.TempDir(os.TempDir(), "dest")
		Expect(err).ToNot(HaveOccurred())

		replaceAvailableSpaceBlockFunc(func(dataDir string) (int64, error) {
			Expect(tempDir).To(Equal(dataDir))
			return int64(100000), nil
		}, func() {
			url, err := url.Parse("http://fakeurl-notreal.fake")
			Expect(err).ToNot(HaveOccurred())
			mdp := &MockDataProvider{
				url: url,
			}
			dp := NewDataProcessor(mdp, tempDir, "dataDir", "scratchDataDir", "1G", 0.055, false)
			qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
			replaceQEMUOperations(qemuOperations, func() {
				nextPhase, err := dp.resize()
				Expect(err).ToNot(HaveOccurred())
				Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
			})
		})
	})

	It("Should resize and return complete, when requestedSize is valid, and datadir exists", func() {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "data")
		Expect(err).ToNot(HaveOccurred())
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, tmpDir, tmpDir, "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.resize()
			Expect(err).ToNot(HaveOccurred())
			Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
		})
	})

	It("Should not resize and return error, when ResizeImage fails", func() {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "data")
		Expect(err).ToNot(HaveOccurred())
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", tmpDir, "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewQEMUAllErrors()
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.resize()
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(nextPhase))
		})
	})

	It("Should return same value as replaced function", func() {
		replaceAvailableSpaceBlockFunc(func(dataDir string) (int64, error) {
			return int64(100000), nil
		}, func() {
			mdp := &MockDataProvider{}
			dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "", 0.055, false)
			Expect(int64(100000)).To(Equal(dp.calculateTargetSize()))
		})
	})

	It("Should fail if calculate size returns failure", func() {
		replaceAvailableSpaceBlockFunc(func(dataDir string) (int64, error) {
			return int64(-1), errors.New("error")
		}, func() {
			mdp := &MockDataProvider{}
			dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "", 0.055, false)
			// We just log the error if one happens.
			Expect(int64(-1)).To(Equal(dp.calculateTargetSize()))

		})
	})
})

var _ = Describe("ResizeImage", func() {
	//fakeInfoRet has info.VirtualSize=1024
	table.DescribeTable("calling ResizeImage", func(qemuOperations image.QEMUOperations, imageSize string, totalSpace int64, wantErr bool) {
		replaceQEMUOperations(qemuOperations, func() {
			err := ResizeImage("dest", imageSize, totalSpace, false)
			if !wantErr {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully resize to imageSize when imageSize > info.VirtualSize and < totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(1500*1024), 0)), "1536000", int64(2048*1024), false),
		table.Entry("successfully resize to totalSize when imageSize > info.VirtualSize and > totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(2048*1024), 0)), "2560000", int64(2048*1024), false),
		table.Entry("successfully do nothing when imageSize = info.VirtualSize and > totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(1024*1024), 0)), "1048576", int64(1024*1024), false),
		table.Entry("fail to resize to with blank imageSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(2048), 0)), "", int64(2048), true),
		table.Entry("fail to resize to with blank imageSize", NewQEMUAllErrors(), "", int64(2048), true),
	)
})

var _ = Describe("DataProcessorResume", func() {
	It("Should fail with an error if the data provider cannot resume", func() {
		mdp := &MockDataProvider{}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "", 0.055, false)
		err := dp.ProcessDataResume()
		Expect(err).To(HaveOccurred())
	})

	It("Should resume properly based on resume phase", func() {
		amdp := &MockAsyncDataProvider{
			ResumePhase: ProcessingPhaseComplete,
		}
		dp := NewDataProcessor(amdp, "dest", "dataDir", "scratchDataDir", "", 0.055, false)
		err := dp.ProcessDataResume()
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("MergeDelta", func() {
	It("Should correctly move to merge phase, then rebase and commit", func() {
		url := &url.URL{}
		originalBackingFile := "original-backing-file"
		expectedBackingFile := "rebased-backing-file"
		originalActualSize := int64(5)
		expectedActualSize := int64(6)

		mdp := &MockDataProvider{
			infoResponse:     ProcessingPhaseTransferScratch,
			transferResponse: ProcessingPhaseMergeDelta,
			needsScratch:     true,
			url:              url,
		}

		dp := NewDataProcessor(mdp, expectedBackingFile, "dataDir", "scratchDataDir", "", 0.055, false)
		err := errors.New("this operation should not be called")
		info := &image.ImgInfo{
			Format:      "",
			BackingFile: originalBackingFile,
			VirtualSize: 10,
			ActualSize:  originalActualSize,
		}
		qemuOperations := NewFakeQEMUOperations(err, err, fakeInfoOpRetVal{info, nil}, err, err, nil)
		replaceQEMUOperations(qemuOperations, func() {
			// Check original backing file and size before processing
			info, err := qemuOperations.Info(url)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.BackingFile).To(Equal(originalBackingFile))
			Expect(info.ActualSize).To(Equal(originalActualSize))

			// This should run rebase and commit
			err = dp.ProcessData()
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(mdp.calledPhases)))
			Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
			Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))

			// Verify backing file was rebased and committed to main data file
			info, err = qemuOperations.Info(url)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.BackingFile).To(Equal(expectedBackingFile))
			Expect(info.ActualSize).To(Equal(expectedActualSize))
		})
	})
})

func replaceQEMUOperations(replacement image.QEMUOperations, f func()) {
	orig := qemuOperations
	if replacement != nil {
		qemuOperations = replacement
		defer func() { qemuOperations = orig }()
	}
	f()
}

func NewFakeQEMUOperations(e2, e3 error, ret4 fakeInfoOpRetVal, e5 error, e6 error, targetResize *resource.Quantity) image.QEMUOperations {
	return &fakeQEMUOperations{e2, e3, ret4, e5, e6, targetResize}
}

func (o *fakeQEMUOperations) ConvertToRawStream(*url.URL, string, bool) error {
	return o.e2
}

func (o *fakeQEMUOperations) Validate(*url.URL, int64, float64) error {
	return o.e5
}

func (o *fakeQEMUOperations) Resize(dest string, size resource.Quantity, preallocate bool) error {
	if o.resizeQuantity != nil {
		Expect(o.resizeQuantity.Cmp(size)).To(Equal(0), "sizes don't match %v, %v", o.resizeQuantity.String(), size.String())
	}
	return o.e3
}

func (o *fakeQEMUOperations) Info(url *url.URL) (*image.ImgInfo, error) {
	return o.ret4.imgInfo, o.ret4.e
}

func (o *fakeQEMUOperations) CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	return o.e6
}

// Simulate rebase by changing the backing file.
func (o *fakeQEMUOperations) Rebase(backingFile string, delta string) error {
	if o.ret4.imgInfo == nil {
		return errors.New("invalid image info")
	}
	o.ret4.imgInfo.BackingFile = backingFile
	return nil
}

// Simulate commit by increasing the image size.
func (o *fakeQEMUOperations) Commit(image string) error {
	if o.ret4.imgInfo == nil {
		return errors.New("invalid image info")
	}
	o.ret4.imgInfo.ActualSize++
	return nil
}

func NewQEMUAllErrors() image.QEMUOperations {
	err := errors.New("qemu should not be called from this test override with replaceQEMUOperations")
	return NewFakeQEMUOperations(err, err, fakeInfoOpRetVal{nil, err}, err, err, nil)
}

func replaceAvailableSpaceBlockFunc(replacement func(string) (int64, error), f func()) {
	origFunc := getAvailableSpaceBlockFunc
	getAvailableSpaceBlockFunc = replacement
	defer func() {
		getAvailableSpaceBlockFunc = origFunc
	}()
	f()
}

func replaceAvailableSpaceFunc(replacement func(string) (int64, error), f func()) {
	origFunc := getAvailableSpaceFunc
	getAvailableSpaceFunc = replacement
	defer func() {
		getAvailableSpaceFunc = origFunc
	}()
	f()
}
