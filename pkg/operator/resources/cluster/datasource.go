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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

// NewDataSourceCrd - provides DataSource CRD
func NewDataSourceCrd() *extv1.CustomResourceDefinition {
	return createDataSourceCRD()
}

// createDataSourceCRD creates the DataSource schema
func createDataSourceCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "datasources.cdi.kubevirt.io",
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "DataSource",
				Plural:   "datasources",
				ListKind: "DataSourceList",
				Singular: "datasource",
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true, //FIXME?
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "DataSource references an import/clone source for a DataVolume",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								// We are aware apiVersion, kind, and metadata are technically not needed, but to make comparision with
								// kubebuilder easier, we add it here.
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
									Description: "DataSourceSpec defines specification for DataSource",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"source": {
											Description: "Source is the src of the data for the requested DataVolume",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"pvc": {
													Description: "DataSource source PVC provides the parameters to create a Data Volume from an existing PVC",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"namespace": {
															Description: "The namespace of the source PVC",
															Type:        "string",
														},
														"name": {
															Description: "The name of the source PVC",
															Type:        "string",
														},
													},
													Required: []string{
														"name",
														"namespace",
													},
												},
											},
										},
									},
								},
								"status": {
									Type:        "object",
									Description: "DataSourceStatus provides the most recently observed status of the DataSource",
									Properties: map[string]extv1.JSONSchemaProps{
										"conditions": {
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "DataVolumeCondition represents the state of a data volume condition.",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"status": {
															Type: "string",
														},
														"type": {
															Description: "DataVolumeConditionType is the string representation of known condition types",
															Type:        "string",
														},
													},
													Required: []string{
														"status",
														"type",
													},
												},
											},
											Type: "array",
										},
									},
								},
							},
							Required: []string{
								"spec",
							},
						},
					},
				},
			},
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
			Scope: "Namespaced", //FIXM: "Cluster"?
		},
	}
}
