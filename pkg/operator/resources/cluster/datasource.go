/*
Copyright 2021 The CDI Authors.

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

// NewDataSourceCrd - provides DataSource CRD
func NewDataSourceCrd() *extv1.CustomResourceDefinition {
	return createDataSourceCRD()
}

// createDataSourceCRD creates the DataSource schema
func createDataSourceCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["datasource"])).Decode(&crd)
	return &crd
}

// NewDataImportCronCrd - provides DataImportCron CRD
func NewDataImportCronCrd() *extv1.CustomResourceDefinition {
	return createDataImportCronCRD()
}

// createDataImportCronCRD creates the DataImportCron schema
func createDataImportCronCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["dataimportcron"])).Decode(&crd)
	return &crd
}
