/*
Copyright 2026 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"errors"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChecksumValidator", func() {
	DescribeTable("should", func(checksumStr string, data string, wantCreationErr bool, wantValidationErr bool, wantAlgo string) {
		// Test creation
		validator, err := NewChecksumValidator(checksumStr)
		if wantCreationErr {
			Expect(err).To(HaveOccurred())
			return
		}
		Expect(err).NotTo(HaveOccurred())

		// Test algorithm if validator was created
		if checksumStr != "" {
			Expect(validator).NotTo(BeNil())
			Expect(validator.Algorithm()).To(Equal(wantAlgo))
		}

		// Test validation if data is provided
		if data != "" && validator != nil {
			reader := validator.GetReader(strings.NewReader(data))
			_, err = io.Copy(io.Discard, reader)
			Expect(err).NotTo(HaveOccurred())

			err = validator.Validate()
			if wantValidationErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	},
		// Creation tests
		Entry("return valid validator for SHA256", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "", false, false, "sha256"),
		Entry("return valid validator for MD5", "md5:d41d8cd98f00b204e9800998ecf8427e", "", false, false, "md5"),
		Entry("return valid validator for SHA512", "sha512:cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", "", false, false, "sha512"),
		Entry("return nil for empty string", "", "", false, false, ""),
		Entry("return error for invalid format - no colon", "sha256abc123", "", true, false, ""),
		Entry("return error for invalid format - multiple colons", "sha256:abc:123", "", true, false, ""),
		Entry("return error for unsupported algorithm", "crc32:abc123", "", true, false, ""),
		Entry("return error for empty hash value", "sha256:", "", true, false, ""),
		Entry("handle case insensitive algorithm", "SHA256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", "", false, false, "sha256"),

		// Validation tests
		Entry("succeed with valid SHA256 for empty string", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "", false, false, "sha256"),
		Entry("succeed with valid MD5 for empty string", "md5:d41d8cd98f00b204e9800998ecf8427e", "", false, false, "md5"),
		Entry("succeed with valid SHA256 for hello world", "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", "hello world", false, false, "sha256"),
		Entry("fail with invalid SHA256 - wrong hash", "sha256:0000000000000000000000000000000000000000000000000000000000000000", "hello world", false, true, "sha256"),
		Entry("succeed with valid MD5 for hello world", "md5:5eb63bbbe01eeed093cb22bb8f5acdc3", "hello world", false, false, "md5"),
		Entry("succeed when no checksum specified", "", "hello world", false, false, ""),
	)

	It("GetReader should return TeeReader when validator is non-nil", func() {
		validator, err := NewChecksumValidator("sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9")
		Expect(err).NotTo(HaveOccurred())
		reader := strings.NewReader("hello world")
		result := validator.GetReader(reader)
		Expect(result).NotTo(Equal(reader))
	})

	It("errors.Is should detect ErrChecksumMismatch", func() {
		// Create validator with wrong checksum
		validator, err := NewChecksumValidator("sha256:0000000000000000000000000000000000000000000000000000000000000000")
		Expect(err).NotTo(HaveOccurred())

		// Read some data
		reader := validator.GetReader(strings.NewReader("hello world"))
		_, err = io.Copy(io.Discard, reader)
		Expect(err).NotTo(HaveOccurred())

		// Validate should return ErrChecksumMismatch
		err = validator.Validate()
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrChecksumMismatch)).To(BeTrue())

		// The error message should contain details
		Expect(err.Error()).To(ContainSubstring("expected sha256:"))
	})
})
