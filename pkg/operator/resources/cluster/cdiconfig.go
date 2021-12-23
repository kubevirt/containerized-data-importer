package cluster

import (
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources"
)

// NewCdiConfigCrd - provides CDIConfig CRD
func NewCdiConfigCrd() *extv1.CustomResourceDefinition {
	return createCDIConfigCRD()
}

// createCDIConfigCRD creates the CDIConfig schema
func createCDIConfigCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["cdiconfig"])).Decode(&crd)
	return &crd
}

// CreateConfigPropertiesSchema creates the CDIConfigSpec properties schema
func CreateConfigPropertiesSchema() map[string]extv1.JSONSchemaProps {
	crd := createCDIConfigCRD()
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			return version.Schema.OpenAPIV3Schema.Properties["spec"].Properties
		}
	}
	return make(map[string]extv1.JSONSchemaProps)
}
