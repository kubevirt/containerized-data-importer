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
package image

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

const testImagesDir = "../../tests/images"

var _ = Describe("Registry Importer", func() {
	source := "docker://docker.io/fedora"
	dest := "/data"

	table.DescribeTable("with import source should", func(execfunc execFunctionType, errString string, errFunc func() error) {
		replaceSkopeoFunctions(execfunc, func() {
			err := errFunc()

			if errString == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				rootErr := errors.Cause(err)
				if rootErr.Error() != errString {
					Fail(fmt.Sprintf("Got wrong failure: %s, expected %s", rootErr, errString))
				}
			}
		})
	},
		table.Entry("copy success", mockExecFunction("", ""), "", func() error { return CopyRegistryImage(source, dest, "", "") }),
		table.Entry("copy failure", mockExecFunction("", "exit status 1"), "exit status 1", func() error { return CopyRegistryImage(source, dest, "", "") }),
	)

})

var _ = Describe("Extract image layers", func() {
	var tmpDir string
	var dataTmpPath string
	var err error

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "image-layers-test")
		Expect(err).NotTo(HaveOccurred())
		dataTmpPath = filepath.Join(tmpDir, dataTmpDir)
		err = os.MkdirAll(dataTmpPath, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("Should not fail on, no layers v2 manifest", func() {
		err = copyFile(filepath.Join(testImagesDir, "valid_manifest/manifest.json"), filepath.Join(dataTmpPath, "manifest.json"))
		Expect(err).NotTo(HaveOccurred())
		err := extractImageLayers(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should not fail on, layered image", func() {
		err = util.UnArchiveLocalTar(filepath.Join(testImagesDir, "docker-image.tar"), tmpDir)
		Expect(err).NotTo(HaveOccurred())
		err := extractImageLayers(filepath.Join(tmpDir, "data"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Image manifest", func() {
	It("Should not parse a non-existing file", func() {
		_, err := getImageManifest("invalid_dir")
		Expect(err).To(HaveOccurred())
	})

	It("Should parse a valid file", func() {
		manifest, err := getImageManifest(filepath.Join(testImagesDir, "valid_manifest"))
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest.SchemaVersion).To(Equal(2))
	})

	It("Should NOT parse an invalid file", func() {
		_, err := getImageManifest(filepath.Join(testImagesDir, "invalid_manifest"))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Clean whiteout files", func() {
	var tmpDir string
	var err error

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "whiteout-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("Should remove whiteout files in valid directory", func() {
		// Create some whiteout files.
		file, err := os.OpenFile(filepath.Join(tmpDir, whFilePrefix+"file1.txt"), os.O_CREATE, 0666)
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		file, err = os.OpenFile(filepath.Join(tmpDir, whFilePrefix+"file2.txt"), os.O_CREATE, 0666)
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		file, err = os.OpenFile(filepath.Join(tmpDir, whFilePrefix+"file3.txt"), os.O_CREATE, 0666)
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		file, err = os.OpenFile(filepath.Join(tmpDir, "file4.txt"), os.O_CREATE, 0666)
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())

		files, err := ioutil.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(files)).To(Equal(4))
		err = cleanWhiteoutFiles(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		files, err = ioutil.ReadDir(tmpDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(files)).To(Equal(1))
	})

	It("Should error on invalid directory", func() {
		err = cleanWhiteoutFiles("invalid_dir")
		Expect(err).To(HaveOccurred())
	})
})

func replaceSkopeoFunctions(mockSkopeoExecFunction execFunctionType, f func()) {
	origSkopeoExecFunction := skopeoExecFunction
	origExtractImageLayers := extractImageLayers
	if mockSkopeoExecFunction != nil {
		skopeoExecFunction = mockSkopeoExecFunction
		defer func() { skopeoExecFunction = origSkopeoExecFunction }()
	}
	extractImageLayers = mockExtractImageLayers
	defer func() { extractImageLayers = origExtractImageLayers }()
	f()
}

func mockExtractImageLayers(dest string) error {
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
