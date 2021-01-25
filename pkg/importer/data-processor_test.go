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
)

type fakeInfoOpRetVal struct {
	imgInfo *image.ImgInfo
	e       error
}

const TestImagesDir = "../../tests/images"

const (
	SmallActualSize  = 1024
	SmallVirtualSize = 1024
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
		dp := NewDataProcessor(mdp, "dest", "dataDir", tmpDir, "1G", 0.055, false)
		dp.availableSpace = int64(1500)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(dp.getUsableSpace(), 0))
		replaceQEMUOperations(qemuOperations, func() {
			err = dp.ProcessData()
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(mdp.calledPhases)))
			Expect(ProcessingPhaseInfo).To(Equal(mdp.calledPhases[0]))
			Expect(ProcessingPhaseTransferScratch).To(Equal(mdp.calledPhases[1]))
			Expect(tmpDir).To(Equal(mdp.transferPath))
		})
	})
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
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.resize()
			Expect(err).ToNot(HaveOccurred())
			Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
		})
	})

	It("Should not resize and return complete, when requestedSize is valid, but datadir doesn't exist (block device)", func() {
		replaceAvailableSpaceBlockFunc(func(dataDir string) (int64, error) {
			Expect("dest").To(Equal(dataDir))
			return int64(100000), nil
		}, func() {
			url, err := url.Parse("http://fakeurl-notreal.fake")
			Expect(err).ToNot(HaveOccurred())
			mdp := &MockDataProvider{
				url: url,
			}
			dp := NewDataProcessor(mdp, "dest", "dataDir", "scratchDataDir", "1G", 0.055, false)
			qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
			replaceQEMUOperations(qemuOperations, func() {
				nextPhase, err := dp.resize()
				Expect(err).ToNot(HaveOccurred())
				Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
			})
		})
	})

	It("Should resize and return complete, when requestedSize is valid, and datadir exists", func() {
		tmpDir, err := ioutil.TempDir("", "data")
		Expect(err).ToNot(HaveOccurred())
		url, err := url.Parse("http://fakeurl-notreal.fake")
		Expect(err).ToNot(HaveOccurred())
		mdp := &MockDataProvider{
			url: url,
		}
		dp := NewDataProcessor(mdp, "dest", tmpDir, "scratchDataDir", "1G", 0.055, false)
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			nextPhase, err := dp.resize()
			Expect(err).ToNot(HaveOccurred())
			Expect(ProcessingPhaseComplete).To(Equal(nextPhase))
		})
	})

	It("Should not resize and return error, when ResizeImage fails", func() {
		tmpDir, err := ioutil.TempDir("", "data")
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
	table.DescribeTable("calling ResizeImage", func(qemuOperations image.QEMUOperations, imageSize string, totalSpace int64, wantErr bool, wantResized bool) {
		replaceQEMUOperations(qemuOperations, func() {
			resized, err := ResizeImage("dest", imageSize, totalSpace)
			if !wantErr {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
			Expect(resized).To(Equal(wantResized))
		})
	},
		table.Entry("successfully resize to imageSize when imageSize > info.VirtualSize and < totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(1500), 0)), "1500", int64(2048), false, true),
		table.Entry("successfully resize to totalSize when imageSize > info.VirtualSize and > totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(2048), 0)), "2500", int64(2048), false, false),
		table.Entry("successfully do nothing when imageSize = info.VirtualSize and > totalSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(1024), 0)), "1024", int64(1024), false, false),
		table.Entry("fail to resize to with blank imageSize", NewFakeQEMUOperations(nil, nil, fakeInfoRet, nil, nil, resource.NewScaledQuantity(int64(2048), 0)), "", int64(2048), true, false),
		table.Entry("fail to resize to with blank imageSize", NewQEMUAllErrors(), "", int64(2048), true, false),
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

func (o *fakeQEMUOperations) Resize(dest string, size resource.Quantity) error {
	if o.resizeQuantity != nil {
		Expect(o.resizeQuantity.Cmp(size)).To(Equal(0))
	}
	return o.e3
}

func (o *fakeQEMUOperations) Info(url *url.URL) (*image.ImgInfo, error) {
	return o.ret4.imgInfo, o.ret4.e
}

func (o *fakeQEMUOperations) CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	return o.e6
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
