package importer

import (
	"bytes"
	"net/url"

	libnbd "github.com/mrnold/go-libnbd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
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
		newVddkDataSource = createMockVddkDataSource
		newVddkDataSink = createMockVddkDataSink
		currentExport = defaultMockNbdExport()
	})

	AfterEach(func() {
		newVddkDataSource = createVddkDataSource
	})

	It("NewVDDKDataSource should fail when called with an invalid endpoint", func() {
		newVddkDataSource = createVddkDataSource
		_, err := NewVDDKDataSource("httpx://-------", "", "", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("VDDK data source GetURL should pass through NBD socket information", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		socket := dp.GetURL()
		path := socket.String()
		Expect(path).To(Equal(socketPath))
	})

	It("VDDK data source should move to transfer data phase after Info", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
	})

	It("VDDK data source Info should fail if GetSize fails", func() {
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return 0, errors.New("forced GetSize failure")
		}
		currentExport = replaceExport
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
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
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
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
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
		phase, err = dp.TransferFile("")
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})
})

type mockNbdOperations struct{}

func (handle *mockNbdOperations) GetSize() (uint64, error) {
	return currentExport.Size()
}

func (handle *mockNbdOperations) Pread(buf []byte, offset uint64, optargs *libnbd.PreadOptargs) error {

	fakebuf, err := currentExport.Read(offset)
	copy(buf, fakebuf)
	return err
}

func (handle *mockNbdOperations) Close() *libnbd.LibnbdError {
	return nil
}

func createMockVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	socketURL, err := url.Parse(socketPath)
	if err != nil {
		return nil, err
	}

	handle := &mockNbdOperations{}

	return &VDDKDataSource{
		Command:   nil,
		NbdHandle: handle,
		NbdSocket: socketURL,
	}, nil
}

type mockVddkDataSink struct{}

func (sink mockVddkDataSink) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func (sink mockVddkDataSink) Close() {}

func createMockVddkDataSink(destinationFile string, size uint64) (VDDKDataSink, error) {
	sink := mockVddkDataSink{}
	return sink, nil
}
