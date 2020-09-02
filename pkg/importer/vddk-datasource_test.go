package importer

import (
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

const (
	socketPath = "nbd://nbdtest.sock"
)

var _ = Describe("VDDK data source", func() {
	BeforeEach(func() {
		newVddkDataSource = createMockVddkDataSource
		qemuOperations = NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{imgInfo: &fakeSmallImageInfo, e: nil}, nil, nil, nil)
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

	It("VDDK data source Info should fail if qemu-img info fails", func() {
		qemuOperations = NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{imgInfo: &fakeSmallImageInfo, e: errors.New("test qemu-img info failure")}, nil, nil, nil)
		dp, err := NewVDDKDataSource("", "", "", "", "", "")
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("VDDK data source should move to complete phase after TransferFile", func() {
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
		qemuOperations = NewFakeQEMUOperations(errors.New("test qemu-img convert failure"), nil, fakeInfoOpRetVal{imgInfo: &fakeSmallImageInfo, e: nil}, nil, nil, nil)
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

func createMockVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	socketURL, err := url.Parse(socketPath)
	if err != nil {
		return nil, err
	}
	return &VDDKDataSource{
		Command:   nil,
		NbdSocket: socketURL,
	}, nil
}
