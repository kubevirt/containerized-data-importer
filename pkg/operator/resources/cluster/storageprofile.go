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

package cluster

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

// NewStorageProfileCrd - provides StorageProfile CRD
func NewStorageProfileCrd() *extv1.CustomResourceDefinition {
	return createStorageProfileCRD()
}

// createStorageProfileCRD creates the StorageProfile schema
func createStorageProfileCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "storageprofiles.cdi.kubevirt.io",
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "StorageProfile",
				Plural:   "storageprofiles",
				ListKind: "StorageProfileList",
				Singular: "storageprofile",
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:         "v1beta1",
					Served:       true,
					Storage:      true,
					Subresources: &extv1.CustomResourceSubresources{},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "StorageProfile provides a CDI specific recommendation for storage parameters",
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
									Description: "StorageProfileSpec defines specification for StorageProfile",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"claimPropertySets": {
											Description: "ClaimPropertySets is a provided set of properties applicable to PVC",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "ClaimPropertySet is a set of properties applicable to PVC",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"accessModes": {
															Description: "AccessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1",
															Type:        "array",
															Items: &extv1.JSONSchemaPropsOrArray{
																Schema: &extv1.JSONSchemaProps{
																	Type: "string",
																},
															},
														},
														"volumeMode": {
															Description: "volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.",
															Type:        "string",
														},
													},
												},
											},
											Type: "array",
										},
									},
								},
								"status": {
									Type:        "object",
									Description: "StorageProfileStatus provides the most recently observed status of the StorageProfile",
									Properties: map[string]extv1.JSONSchemaProps{
										"storageClass": {
											Description: "The StorageClass name for which capabilities are defined",
											Type:        "string",
										},
										"provisioner": {
											Description: "The Storage class provisioner plugin name",
											Type:        "string",
										},
										"claimPropertySets": {
											Description: "ClaimPropertySets computed from the spec and detected in the system",
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "ClaimPropertySet is a set of properties applicable to PVC",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"accessModes": {
															Description: "AccessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1",
															Type:        "array",
															Items: &extv1.JSONSchemaPropsOrArray{
																Schema: &extv1.JSONSchemaProps{
																	Type: "string",
																},
															},
														},
														"volumeMode": {
															Description: "volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.",
															Type:        "string",
														},
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
			Scope: "Cluster",
		},
	}
}
