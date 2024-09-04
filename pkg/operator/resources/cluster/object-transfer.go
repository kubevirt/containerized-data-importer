package cluster

import (
	"fmt"
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"

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
	for i := range crd.Spec.Versions {
		deprecationWarning := fmt.Sprintf("cdi.kubevirt.io/%s createObjectTransferCRD is now deprecated and will be removed in v1.", crd.Spec.Versions[i].Name)
		crd.Spec.Versions[i].Deprecated = true
		crd.Spec.Versions[i].DeprecationWarning = ptr.To(deprecationWarning)
	}
	return &crd
}
