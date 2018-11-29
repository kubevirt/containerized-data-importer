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

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Importer", func() {
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
