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

package checksum

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseAndValidate", func() {
	DescribeTable("should parse and validate checksum strings", func(checksumStr string, wantAlgorithm string, wantHash string, wantErr bool) {
		algorithm, hash, err := ParseAndValidate(checksumStr)
		if wantErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
			Expect(algorithm).To(Equal(wantAlgorithm))
			Expect(hash).To(Equal(wantHash))
		}
	},
		Entry("valid sha256", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false),
		Entry("valid sha256 uppercase converted to lowercase", "SHA256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", "sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false),
		Entry("valid md5", "md5:d41d8cd98f00b204e9800998ecf8427e", "md5", "d41d8cd98f00b204e9800998ecf8427e", false),
		Entry("invalid format", "sha256", "", "", true),
		Entry("unsupported algorithm", "sha384:abc123", "", "", true),
	)
})
