/*
Copyright 2018 The CDI Authors.

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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry Importer", func() {
	source := "oci-archive:" + imageFile
	malformedSource := "oci-archive:" + filepath.Join(imageDir, "malformed-registry-image.tar")

	var tmpDir string
	var err error

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	DescribeTable("Should extract a single file", func(source string) {
		info, err := CopyRegistryImage(source, tmpDir, "disk/cirros-0.3.4-x86_64-disk.img", "", "", "", false, false)
		Expect(err).ToNot(HaveOccurred())
		Expect(info).ToNot(BeNil())

		file := filepath.Join(tmpDir, "disk/cirros-0.3.4-x86_64-disk.img")
		Expect(file).To(BeARegularFile())
	},
		Entry("when all image layers are valid", source),
		Entry("when one of the image layers is malformed", malformedSource),
	)
	It("Should extract files prefixed by path", func() {
		info, err := CopyRegistryImageAll(source, tmpDir, "etc/", "", "", "", false, false)
		Expect(err).ToNot(HaveOccurred())
		Expect(info).ToNot(BeNil())

		file := filepath.Join(tmpDir, "etc/hosts")
		Expect(file).To(BeARegularFile())

		file = filepath.Join(tmpDir, "etc/hostname")
		Expect(file).To(BeARegularFile())
	})
	It("Should return an error if a single file is not found", func() {
		info, err := CopyRegistryImage(source, tmpDir, "disk/invalid.img", "", "", "", false, false)
		Expect(err).To(HaveOccurred())
		Expect(info).To(BeNil())

		file := filepath.Join(tmpDir, "disk/cirros-0.3.4-x86_64-disk.img")
		_, err = os.Stat(file)
		Expect(err).To(HaveOccurred())
	})
	It("Should return an error if no files matches a prefix", func() {
		info, err := CopyRegistryImageAll(source, tmpDir, "invalid/", "", "", "", false, false)
		Expect(err).To(HaveOccurred())
		Expect(info).To(BeNil())
	})
})
