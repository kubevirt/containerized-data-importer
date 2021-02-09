package cluster

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

// NewObjectTransferCrd - provides ObjectTransfer CRD
func NewObjectTransferCrd() *extv1.CustomResourceDefinition {
	return createObjectTransferCRD()
}

// createObjectTransferCRD creates the ObjectTransfer schema
func createObjectTransferCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "objecttransfers.cdi.kubevirt.io",
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "ObjectTransfer",
				Plural:   "objecttransfers",
				Singular: "objecttransfer",
				ListKind: "ObjectTransferList",
				ShortNames: []string{
					"ot",
					"ots",
				},
				Categories: []string{
					"all",
				},
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "ObjectTransfer is the cluster scoped object transfer resource",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"apiVersion": {
									Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
									Type:        "string",
								},
								"kind": {
									Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds",
									Type:        "string",
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Description: "ObjectTransferSpec specifies the source/target of the transfer",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"source": {
											Description: "TransferSource is the source of a ObjectTransfer",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"apiVersion": {
													Type: "string",
												},
												"kind": {
													Type: "string",
												},
												"name": {
													Type: "string",
												},
												"namespace": {
													Type: "string",
												},
											},
											Required: []string{
												"kind",
												"name",
												"namespace",
											},
										},
										"target": {
											Description: "TransferTarget is the target of an ObjectTransfer",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"name": {
													Type: "string",
												},
												"namespace": {
													Type: "string",
												},
											},
										},
										"parentName": {
											Type: "string",
										},
									},
									Required: []string{
										"source",
										"target",
									},
								},
								"status": {
									Type:        "object",
									Description: "ObjectTransferStatus is the status of the ObjectTransfer",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "ObjectTransferCondition contains condition data",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Format: "date-time",
															Type:   "string",
														},
														"lastTransitionTime": {
															Format: "date-time",
															Type:   "string",
														},
														"message": {
															Type: "string",
														},
														"reason": {
															Type: "string",
														},
														"status": {
															Type: "string",
														},
														"type": {
															Description: "ObjectTransferConditionType is the type of ObjectTransferCondition",
															Type:        "string",
														},
													},
													Required: []string{
														"status",
														"type",
													},
													Type: "object",
												},
											},
											Type: "array",
										},
										"data": {
											AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
												Schema: &extv1.JSONSchemaProps{
													Type: "string",
												},
											},
											Description: "Data is a place for intermediary state.  Or anything really.",
											Type:        "object",
										},
										"phase": {
											Description: "Phase is the current phase of the transfer",
											Type:        "string",
										},
									},
								},
							},
							Required: []string{
								"spec",
							},
						},
					},
					AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
						{
							Name:     "Age",
							Type:     "date",
							JSONPath: ".metadata.creationTimestamp",
						},
						{
							Name:        "Phase",
							Type:        "string",
							Description: "The phase of the ObjectTransfer",
							JSONPath:    ".status.phase",
						},
					},
				},
			},
			Scope: "Cluster",
		},
	}
}
