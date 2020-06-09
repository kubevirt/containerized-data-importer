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
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

func createDataVolumeCRD() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "datavolumes.cdi.kubevirt.io",
			Labels: utils.WithCommonLabels(nil),
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:   "DataVolume",
				Plural: "datavolumes",
				ShortNames: []string{
					"dv",
					"dvs",
				},
				Singular: "datavolume",
				Categories: []string{
					"all",
				},
			},
			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
				},
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
				},
			},
			Conversion: &extv1beta1.CustomResourceConversion{
				Strategy: extv1beta1.NoneConverter,
			},
			Scope: "Namespaced",
			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Description: "DataVolumes are an abstraction on top of PersistentVolumeClaims to allow easy population of those PersistentVolumeClaims with relation to VirtualMachines",
					Type:        "object",
					Properties: map[string]extv1beta1.JSONSchemaProps{
						"spec": {
							Description: "Specification of DataVolumes",
							Type:        "object",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"source": {
									Description: "The source type used to populate the DataVolume",
									Type:        "object",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"http": {
											Description: "Source type of http, can be either an http or https endpoint, with an optional basic auth user name and password, and an optional configmap containing additional CAs",
											Type:        "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
												"url": {
													Description: "The URL of the http(s) endpoint",
													Type:        "string",
												},
												"secretRef": {
													Description: "A configmap reference, the configmap should contain accessKeyId (user name) base64 encoded, and secretKey (password) also base64 encoded",
													Type:        "string",
												},
												"certConfigMap": {
													Description: "A configmap reference, containing a ca.pem key, and a base64 encoded pem certificate",
													Type:        "string",
												},
											},
											Required: []string{
												"url",
											},
										},
										"s3": {
											Description: "Source type of S3 bucket, with an optional basic auth user name and password, and an optional configmap containing additional CAs",
											Type:        "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
												"url": {
													Description: "The URL of the s3 endpoint",
													Type:        "string",
												},
												"secretRef": {
													Description: "A configmap reference, the configmap should contain accessKeyId (user name) base64 encoded, and secretKey (password) also base64 encoded",
													Type:        "string",
												},
												"certConfigMap": {
													Description: "A configmap reference, containing a ca.pem key, and a base64 encoded pem certificate",
													Type:        "string",
												},
											},
											Required: []string{
												"url",
											},
										},
										"registry": {
											Description: "Source type of registry, with an optional basic auth user name and password, and an optional configmap containing additional CAs",
											Type:        "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
												"url": {
													Description: "The URL of the registry endpoint",
													Type:        "string",
												},
												"secretRef": {
													Description: "A configmap reference, the configmap should contain accessKeyId (user name) base64 encoded, and secretKey (password) also base64 encoded",
													Type:        "string",
												},
												"certConfigMap": {
													Description: "A configmap reference, containing a ca.pem key, and a base64 encoded pem certificate",
													Type:        "string",
												},
											},
											Required: []string{
												"url",
											},
										},
										"pvc": {
											Description: "Source type of PVC, will initiate a clone from the source PVC and source Namespace into this DataVolume",
											Type:        "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
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
												"namespace",
												"name",
											},
										},
										"upload": {
											Description: "Source type of upload, starts an upload server pod in the current namespace, that saves the uploaded data in the DataVolume",
											Type:        "object",
										},
										"blank": {
											Description: "Source type of blank, creates a blank raw disk image in the DataVolume",
											Type:        "object",
										},
									},
								},
								"pvc": {
									Description: "Spec defines the desired characteristics of a volume requested by a pod author. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims\nPersistentVolumeClaimSpec describes the common attributes of storage devices and allows a Source for provider-specific attributes",
									Type:        "object",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"resources": {
											Description: "Resources represents the minimum resources the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources",
											Type:        "object",
											Properties: map[string]extv1beta1.JSONSchemaProps{
												"requests": {
													Description: "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
													Type:        "object",
													Properties: map[string]extv1beta1.JSONSchemaProps{
														"storage": {
															Description: "Storage Quantity is a fixed-point representation of a number. It provides convenient marshaling/unmarshaling in JSON and YAML, in addition to String() and AsInt64() accessors. The serialization format is: <quantity> ::= <signedNumber><suffix> (Note that <suffix> may be empty, from the \"\" case in <decimalSI>.) <digit> ::= 0 | 1 | ... | 9 <digits> ::= <digit> | <digit><digits> <number> ::= <digits> | <digits>.<digits> | <digits>. | .<digits> <sign> ::= \"+\" | \"-\" <signedNumber> ::= <number> | <sign><number> <suffix> ::= <binarySI> | <decimalExponent> | <decimalSI> <binarySI> ::= Ki | Mi | Gi | Ti | Pi | Ei (International System of units; See: http://physics.nist.gov/cuu/Units/binary.html) <decimalSI> ::= m | \"\" | k | M | G | T | P | E (Note that 1024 = 1Ki but 1000 = 1k; I didn't choose the capitalization.) <decimalExponent> ::= \"e\" <signedNumber> | \"E\" <signedNumber> No matter which of the three exponent forms is used, no quantity may represent a number greater than 2^63-1 in magnitude, nor may it have more than 3 decimal places. Numbers larger or more precise will be capped or rounded up. (E.g.: 0.1m will rounded up to 1m.) This may be extended in the future if we require larger or smaller quantities. When a Quantity is parsed from a string, it will remember the type of suffix it had, and will use the same type again when it is serialized. Before serializing, Quantity will be put in \"canonical form\". This means that Exponent/suffix will be adjusted up or down (with a corresponding increase or decrease in Mantissa) such that: a. No precision is lost b. No fractional digits will be emitted c. The exponent (or suffix) is as large as possible. The sign will be omitted unless the number is negative. Examples: 1.5 will be serialized as \"1500m\" 1.5Gi will be serialized as \"1536Mi\" Note that the quantity will NEVER be internally represented by a floating point number. That is the whole point of this exercise. Non-canonical values will still parse as long as they are well formed, but will be re-emitted in their canonical form. (So always use canonical form, or don't diff.) This format is intended to make it difficult to use these numbers without writing some sort of special handling code in the hopes that that will cause implementors to also use a fixed point implementation",
															Type:        "string",
														},
													},
												},
											},
											Required: []string{
												"requests",
											},
										},
										"storageClassName": {
											Description: "Name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1",
											Type:        "string",
										},
										"accessModes": {
											Description: "AccessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1",
											Type:        "array",
											Items: &extv1beta1.JSONSchemaPropsOrArray{
												Schema: &extv1beta1.JSONSchemaProps{
													Type: "string",
												},
											},
										},
									},
									Required: []string{
										"resources",
										"accessModes",
									},
								},
							},
						},
						"status": {
							Type:        "object",
							Description: "The most recently observed status of the DataVolume",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"phase": {
									Description: "The current phase of the DataVolume",
									Type:        "string",
								},
								"progress": {
									Description: "The progress of the transfer phase in percentage from 0 to 100, or N/A if unable to determine progress",
									Type:        "string",
								},
								"restartCount": {
									Description: "The number of times the pod populating the DataVolume has restarted",
									Type:        "integer",
								},
							},
						},
					},
				},
			},
			AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
				{
					Name:        "Phase",
					Type:        "string",
					Description: "The phase the data volume is in",
					JSONPath:    ".status.phase",
				},
				{
					Name:        "Progress",
					Type:        "string",
					Description: "Transfer progress in percentage if known, N/A otherwise",
					JSONPath:    ".status.progress",
				},
				{
					Name:        "Restarts",
					Type:        "integer",
					Description: "The number of times the transfer has been restarted.",
					JSONPath:    ".status.restartCount",
				},
				{
					Name:     "Age",
					Type:     "date",
					JSONPath: ".metadata.creationTimestamp",
				},
			},
		},
	}
}
