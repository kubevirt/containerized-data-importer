package cluster

import (
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources"
)

// NewObjectTransferCrd - provides ObjectTransfer CRD
func NewObjectTransferCrd() *extv1.CustomResourceDefinition {
	return createObjectTransferCRD()
}

// createObjectTransferCRD creates the ObjectTransfer schema
func createObjectTransferCRD() *extv1.CustomResourceDefinition {
	crd := extv1.CustomResourceDefinition{}
	_ = k8syaml.NewYAMLToJSONDecoder(strings.NewReader(resources.CDICRDs["objecttransfer"])).Decode(&crd)
	return &crd
}
