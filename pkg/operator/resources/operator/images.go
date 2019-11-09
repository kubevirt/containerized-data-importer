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

package operator

const (
	// ControllerImageDefault - default value
	ControllerImageDefault = "cdi-controller"

	// ImporterImageDefault - default value
	ImporterImageDefault = "cdi-importer"

	// ClonerImageDefault - default value
	ClonerImageDefault = "cdi-cloner"

	// APIServerImageDefault - default value
	APIServerImageDefault = "cdi-apiserver"

	// UploadProxyImageDefault - default value
	UploadProxyImageDefault = "cdi-uploadproxy"

	// UploadServerImageDefault - default value
	UploadServerImageDefault = "cdi-uploadserver"

	// OperatorImageDefault - default value
	OperatorImageDefault = "cdi-operator"
)

// Images - images to be provied to cdi operator
type Images struct {
	ControllerImage   string
	ImporterImage     string
	ClonerImage       string
	APIServerImage    string
	UplodaProxyImage  string
	UplodaServerImage string
	OperatorImage     string
}

// FillDefaults - fill image names with defaults
func (ci *Images) FillDefaults() *Images {
	if ci.ControllerImage == "" {
		ci.ControllerImage = ControllerImageDefault
	}
	if ci.ImporterImage == "" {
		ci.ImporterImage = ImporterImageDefault
	}
	if ci.ClonerImage == "" {
		ci.ClonerImage = ClonerImageDefault
	}
	if ci.APIServerImage == "" {
		ci.APIServerImage = APIServerImageDefault
	}
	if ci.UplodaProxyImage == "" {
		ci.UplodaProxyImage = UploadProxyImageDefault
	}
	if ci.UplodaServerImage == "" {
		ci.UplodaServerImage = UploadServerImageDefault
	}
	if ci.OperatorImage == "" {
		ci.OperatorImage = OperatorImageDefault
	}

	return ci
}
