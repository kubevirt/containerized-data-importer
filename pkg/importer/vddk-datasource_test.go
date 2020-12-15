package importer

import (
	"bytes"
	"crypto/md5"
	"net/url"

	libnbd "github.com/mrnold/go-libnbd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	socketPath = "nbd://nbdtest.sock"
)

type mockNbdExport struct {
	Size func() (uint64, error)
	Read func(uint64) ([]byte, error)
}

func defaultMockNbdExport() mockNbdExport {
	export := &mockNbdExport{}
	export.Size = func() (uint64, error) {
		return 12345, nil
	}
	export.Read = func(uint64) ([]byte, error) {
		return bytes.Repeat([]byte{0}, 512), nil
	}
	return *export
}

var currentExport mockNbdExport

var _ = Describe("VDDK data source", func() {
	BeforeEach(func() {
		mockSinkBuffer = bytes.Repeat([]byte{0x00}, 512)
		newVddkDataSource = createMockVddkDataSource
		newVddkDataSink = createMockVddkDataSink
		currentExport = defaultMockNbdExport()
	})

	AfterEach(func() {
		newVddkDataSource = createVddkDataSource
	})

	It("NewVDDKDataSource should fail when called with an invalid endpoint", func() {
		newVddkDataSource = createVddkDataSource
		_, err := NewVDDKDataSource("httpx://-------", "", "", "", "", "", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("VDDK data source GetURL should pass through NBD socket information", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		socket := dp.GetURL()
		path := socket.String()
		Expect(path).To(Equal(socketPath))
	})

	It("VDDK data source should move to transfer data phase after Info", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
	})

	It("VDDK data source should move to complete phase after TransferFile", func() {
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return 512, nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return bytes.Repeat([]byte{0x55}, 512), nil
		}
		currentExport = replaceExport
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
		phase, err = dp.TransferFile("")
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))
	})

	It("VDDK data source should fail if TransferFile fails", func() {
		newVddkDataSink = createVddkDataSink
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
		phase, err = dp.TransferFile("")
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("VDDK data source should know if it is a delta copy", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "checkpoint-1", "checkpoint-2", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.IsDeltaCopy()).To(Equal(true))
	})

	It("VDDK data source should know if it is not a delta copy", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.IsDeltaCopy()).To(Equal(false))
	})

	It("VDDK delta copy should return immediately if there are no changed blocks", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "checkpoint-1", "checkpoint-2", "")
		dp.ChangedBlocks = &types.DiskChangeInfo{
			StartOffset: 0,
			Length:      0,
			ChangedArea: []types.DiskChangeExtent{},
		}
		phase, err := dp.TransferFile("")
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))
	})

	It("VDDK full copy should successfully copy the same bytes passed in", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "")
		dp.Size = 40 << 20
		sourceBytes := bytes.Repeat([]byte{0x55}, int(dp.Size))
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return dp.Size, nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return sourceBytes, nil
		}
		currentExport = replaceExport

		mockSinkBuffer = bytes.Repeat([]byte{0x00}, int(dp.Size))

		phase, err := dp.TransferFile(".")
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))

		sourceSum := md5.Sum(sourceBytes)
		destSum := md5.Sum(mockSinkBuffer)
		Expect(sourceSum).To(Equal(destSum))
	})

	It("VDDK delta copy should sucessfully apply a delta to a base disk image", func() {

		// Copy base disk ("snapshot 1")
		snap1, err := NewVDDKDataSource("", "", "", "", "", "", "checkpoint-1", "", "")
		snap1.Size = 40 << 20
		sourceBytes := bytes.Repeat([]byte{0x55}, int(snap1.Size))
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return snap1.Size, nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return sourceBytes, nil
		}
		currentExport = replaceExport

		mockSinkBuffer = bytes.Repeat([]byte{0x00}, int(snap1.Size))

		phase, err := snap1.TransferFile(".")
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))

		sourceSum := md5.Sum(sourceBytes)
		destSum := md5.Sum(mockSinkBuffer)
		Expect(sourceSum).To(Equal(destSum))

		// Write some data to the first snapshot, then copy the delta from difference between the two snapshots
		snap2, err := NewVDDKDataSource("", "", "", "", "", "", "checkpoint-1", "checkpoint-2", "")
		snap2.Size = 40 << 20
		copy(sourceBytes[1024:2048], bytes.Repeat([]byte{0xAA}, 1024))
		snap2.ChangedBlocks = &types.DiskChangeInfo{
			StartOffset: 1024,
			Length:      1024,
			ChangedArea: []types.DiskChangeExtent{{
				Start:  1024,
				Length: 1024,
			}},
		}
		changedSourceSum := md5.Sum(sourceBytes)

		phase, err = snap2.TransferFile(".")
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))

		deltaSum := md5.Sum(mockSinkBuffer)
		Expect(changedSourceSum).To(Equal(deltaSum))
	})
})

type mockNbdOperations struct{}

func (handle *mockNbdOperations) GetSize() (uint64, error) {
	return currentExport.Size()
}

func (handle *mockNbdOperations) Pread(buf []byte, offset uint64, optargs *libnbd.PreadOptargs) error {

	fakebuf, err := currentExport.Read(offset)
	copy(buf, fakebuf[offset:offset+uint64(len(buf))])
	return err
}

func (handle *mockNbdOperations) Close() *libnbd.LibnbdError {
	return nil
}

func (handle *mockNbdOperations) BlockStatus(length uint64, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.BlockStatusOptargs) error {
	err := 0
	callback("base:allocation", offset, []uint32{uint32(length), 0}, &err)
	return nil
}

func createMockVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKDataSource, error) {
	socketURL, err := url.Parse(socketPath)
	if err != nil {
		return nil, err
	}

	handle := &mockNbdOperations{}

	nbdkit := &NbdKitWrapper{
		Command: nil,
		Socket:  socketURL,
		Handle:  handle,
	}

	return &VDDKDataSource{
		NbdKit:           nbdkit,
		ChangedBlocks:    nil,
		CurrentSnapshot:  currentCheckpoint,
		PreviousSnapshot: previousCheckpoint,
		Size:             0,
	}, nil
}

var mockSinkBuffer []byte

type mockVddkDataSink struct {
	position int
}

func (sink *mockVddkDataSink) Ftruncate(size int64) error {
	mockSinkBuffer = bytes.Repeat([]byte{0x00}, int(size))
	sink.position = 0
	return nil
}

func (sink *mockVddkDataSink) Pwrite(buf []byte, offset uint64) (int, error) {
	copy(mockSinkBuffer[offset:offset+uint64(len(buf))], buf)
	if len(buf) > sink.position {
		sink.position = int(offset) + len(buf)
	}
	return len(buf), nil
}

func (sink *mockVddkDataSink) Write(buf []byte) (int, error) {
	copy(mockSinkBuffer[sink.position:sink.position+len(buf)], buf)
	sink.position += len(buf)
	return len(buf), nil
}

func (sink *mockVddkDataSink) Close() {}

func createMockVddkDataSink(destinationFile string, size uint64) (VDDKDataSink, error) {
	sink := &mockVddkDataSink{0}
	return sink, nil
}
