package importer

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

type FakeDataStream struct {
	DataRdr     io.ReadCloser
	url         *url.URL
	accessKeyID string
	secretKey   string
	err         error
}

func (d *FakeDataStream) Error() error {
	return d.err
}

func (d *FakeDataStream) fakeDataStreamSelector() io.ReadCloser {
	switch d.url.Scheme {
	case "s3":
		return d.s3()
	case "http", "https":
		return d.http()
	default:
		Fail(fmt.Sprintf("fakeDataStreamSelector: invalid url scheme: %s", d.url.Scheme))
	}
	return nil
}

func (d *FakeDataStream) s3() io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte("s3 dataStream")))
}

func (d *FakeDataStream) http() io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte("http dataStream")))
}

// NewFakeDataStream: construct a new FakeDataStream object from params.
func NewFakeDataStream(ep *url.URL, accKey, secKey string) *FakeDataStream {

	ds := &FakeDataStream{
		url:         ep,
		accessKeyID: accKey,
		secretKey:   secKey,
	}
	rdr := ds.fakeDataStreamSelector()
	ds.DataRdr = rdr
	return ds
}

var _ = Describe("Importer", func() {
	const importerTestFolder = "/tmp/importer-test/"

	Context("Test StreamDataToFile when", func() {
		type testT struct {
			descr       string
			endpoint    string
			filename    string
			createFile  bool
			expected    string
			expectError bool
		}

		BeforeEach(func() {
			// create importerTestFolder
			err := os.RemoveAll(importerTestFolder)
			if err == os.ErrNotExist {
				err = nil
			}
			Expect(err).To(BeNil(), fmt.Sprintf("os.RemoveAll: %v", err))
			err = os.MkdirAll(importerTestFolder, os.ModePerm)
			Expect(err).To(BeNil(), fmt.Sprintf("os.MkdirAll: %v", err))
		})

		AfterEach(func() {
			os.RemoveAll(importerTestFolder)
		})

		tests := []testT{
			{
				descr:       "use http",
				filename:    "test-http",
				createFile:  false,
				expected:    "http dataStream",
				endpoint:    "http://www.google.com",
				expectError: false,
			},
			{
				descr:       "use s3",
				filename:    "test-s3",
				createFile:  false,
				expected:    "s3 dataStream",
				endpoint:    "s3://test123",
				expectError: false,
			},
			{
				descr:       "file already exist",
				filename:    "test_file_already_exist",
				createFile:  true,
				expected:    "",
				endpoint:    "http://www.google.com",
				expectError: true,
			},
		}

		for _, test := range tests {
			ep, _ := ParseEndpoint(test.endpoint)
			fn := filepath.Join(importerTestFolder, test.filename)
			mkFile := test.createFile
			expt := test.expected
			expErr := test.expectError

			It(test.descr, func() {
				By("creating dataStream object")
				dataStream := NewFakeDataStream(ep, "", "")
				Expect(dataStream).ToNot(BeNil(), "dataStream is nil")
				Expect(dataStream.DataRdr).ToNot(BeNil(), "dataStream.DataRdr is nil")
				defer dataStream.DataRdr.Close()
				if mkFile {
					By(fmt.Sprintf("creating file %q", fn))
					_, err := os.Create(fn)
					Expect(err).To(BeNil(), fmt.Sprintf("os.Create: %v", err))
				}
				By("copying test data")
				err := StreamDataToFile(dataStream.DataRdr, fn)
				if expErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
					By("reading file content")
					content, err := ioutil.ReadFile(fn)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(Equal(expt))
				}
			})
		}
	})

	Context("Test ParseEnvVar when", func() {
		type testT struct {
			descr    string
			testEnv  string
			value    string
			decode   bool
			expected string
		}
		tests := []testT{
			{
				descr:    "use base64",
				testEnv:  "TEST",
				value:    "cmVkaGF0",
				decode:   true,
				expected: "redhat",
			},
			{
				descr:    "use base64",
				testEnv:  "TEST",
				value:    "MTIz",
				decode:   true,
				expected: "123",
			},
			{
				descr:    "not use base64",
				testEnv:  "TEST",
				value:    "test",
				decode:   false,
				expected: "test",
			},
		}

		for _, test := range tests {
			e := test.testEnv
			v := test.value
			b := test.decode
			d := test.expected
			It("use base64", func() {
				os.Setenv(e, v)
				Expect(util.ParseEnvVar(e, b)).To(Equal(d))
			})
		}
	})

})
