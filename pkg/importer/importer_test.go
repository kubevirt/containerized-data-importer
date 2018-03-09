package importer_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/url"
	"os"

	. "github.com/kubevirt/containerized-data-importer/pkg/importer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeDataStream struct {
	url         *url.URL
	accessKeyId string
	secretKey   string
	err         error
}

func (d *fakeDataStream) Error() error {
	return d.err
}

func (d *fakeDataStream) DataStreamSelector() (io.ReadCloser, error) {
	if d.err != nil {
		return nil, d.err
	}
	if d.url.Scheme == "s3" {
		return d.s3()
	}
	return d.http()
}

func (d *fakeDataStream) s3() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte("s3 dataStream"))), nil
}

func (d *fakeDataStream) http() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte("http dataStream"))), nil
}

// parseDataPath only for debug, never used in mock
func (d *fakeDataStream) parseDataPath() (string, string, error) {
	return "", "", nil
}

// NewFakeDataStream: construct a new fakeDataStream object from params.
func NewFakeDataStream(ep, accKey, secKey string) *fakeDataStream {
	epUrl, err := url.Parse(ep)
	if err != nil {
		return &fakeDataStream{err: err}
	}
	return &fakeDataStream{
		url:         epUrl,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
}

var _ = Describe("Importer", func() {
	Context("Test StreamDataToFile when", func() {
		var dataStream io.ReadCloser
		importerTestFolder := "/tmp/importer-test/"
		type testT struct {
			descr       string
			endpoint    string
			filename    string
			expected    string
			expectError bool
		}

		BeforeEach(func() {
			// create a files and importerTestFolder
			var err error
			if _, err = os.Stat(importerTestFolder); os.IsNotExist(err) {
				os.Mkdir(importerTestFolder, os.ModePerm)
			}
			Expect(os.Create(importerTestFolder + "test_file_already_exist")).ToNot(BeNil())
		})

		AfterEach(func() {
			if dataStream != nil {
				dataStream.Close()
			}
			os.RemoveAll(importerTestFolder)
		})

		tests := []testT{
			{
				descr:       "use http",
				filename:    "test-http",
				expected:    "http dataStream",
				endpoint:    "http://www.google.com",
				expectError: false,
			},
			{
				descr:       "use s3",
				filename:    "test-s3",
				expected:    "s3 dataStream",
				endpoint:    "s3://test123",
				expectError: false,
			},
			{
				descr:       "file already exist",
				filename:    "test_file_already_exist",
				expected:    "",
				endpoint:    "http://www.google.com",
				expectError: true,
			},
		}

		for _, test := range tests {
			fn := test.filename
			expt := test.expected
			ep := test.endpoint
			expErr := test.expectError
			It(test.descr, func() {
				dataStream, err := NewFakeDataStream(ep, "", "").DataStreamSelector()
				Expect(err).ToNot(HaveOccurred())
				err = StreamDataToFile(dataStream, importerTestFolder+fn)
				if expErr {
					Expect(err).To(HaveOccurred())
				} else {
					content, err := ioutil.ReadFile(importerTestFolder + fn)
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
				testEnv:  "PARSEENVVAR-TEST",
				value:    "dGVzdA==",
				decode:   true,
				expected: "test",
			},
			{
				descr:    "not use base64",
				testEnv:  "PARSEENVVAR-TEST",
				value:    "test",
				decode:   false,
				expected: "test",
			},
		}

		for _, test := range tests {
			It("use base64", func() {
				os.Setenv(test.testEnv, test.value)
				Expect(ParseEnvVar(test.testEnv, test.decode)).To(Equal(test.expected))
			})
		}
	})

})
