/*
Copyright 2023 The CDI Authors.

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

package cluster

import (
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources"
)

// NewVolumeImportSourceCrd - provides VolumeImportSource CRD
func NewVolumeImportSourceCrd() *extv1.CustomResourceDefinition {
	return createVolumeImportSourceCRD()
}

// createVolumeImportSourceCRD creates the VolumeImportSource schema
func createVolumeImportSourceCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["volumeimportsource"])).Decode(&crd)
	return &crd
}

// NewVolumeUploadSourceCrd - provides VolumeUploadSource CRD
func NewVolumeUploadSourceCrd() *extv1.CustomResourceDefinition {
	return createVolumeUploadSourceCRD()
}

// createVolumeUploadSourceCRD creates the VolumeUploadSource schema
func createVolumeUploadSourceCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["volumeuploadsource"])).Decode(&crd)
	return &crd
}

// NewVolumeCloneSourceCrd - provides VolumeCloneSource CRD
func NewVolumeCloneSourceCrd() *extv1.CustomResourceDefinition {
	return createVolumeCloneSourceCRD()
}

// createVolumeCloneSourceCRD creates the VolumeCloneSource schema
func createVolumeCloneSourceCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["volumeclonesource"])).Decode(&crd)
	return &crd
}
