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

// NewDataVolumeCrd - provides DataVolume CRD
func NewDataVolumeCrd() *extv1.CustomResourceDefinition {
	return createDataVolumeCRD()
}

// createDataVolumeCRD creates the datavolume schema
func createDataVolumeCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "datavolumes.cdi.kubevirt.io",
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:   "DataVolume",
				Plural: "datavolumes",
				ShortNames: []string{
					"dv",
					"dvs",
				},
				ListKind: "DataVolumeList",
				Singular: "datavolume",
				Categories: []string{
					"all",
				},
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:         "v1alpha1",
					Served:       true,
					Storage:      false,
					Subresources: &extv1.CustomResourceSubresources{},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "DataVolume is an abstraction on top of PersistentVolumeClaims to allow easy population of those PersistentVolumeClaims with relation to VirtualMachines",
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
									Description: "DataVolumeSpec defines the DataVolume type specification",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"contentType": {
											Description: "DataVolumeContentType options: \"kubevirt\", \"archive\"",
											Type:        "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"kubevirt"`),
												},
												{
													Raw: []byte(`"archive"`),
												},
											},
										},
										"source": {
											Description: "Source is the src of the data for the requested DataVolume",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"http": {
													Description: "DataVolumeSourceHTTP can be either an http or https endpoint, with an optional basic auth user name and password, and an optional configmap containing additional CAs",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the URL of the http(s) endpoint",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef A Secret reference, the secret should contain accessKeyId (user name) base64 encoded, and secretKey (password) also base64 encoded",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap is a configmap reference, containing a Certificate Authority(CA) public key, and a base64 encoded pem certificate",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"imageio": {
													Description: "DataVolumeSourceImageIO provides the parameters to create a Data Volume from an imageio source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the URL of the ovirt-engine",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the ovirt-engine",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap provides a reference to the CA cert",
															Type:        "string",
														},
														"diskId": {
															Description: "DiskID provides id of a disk to be imported",
															Type:        "string",
														},
													},
													Required: []string{
														"diskId",
														"url",
													},
												},
												"s3": {
													Description: "DataVolumeSourceS3 provides the parameters to create a Data Volume from an S3 source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the url of the S3 source",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the S3 source",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"registry": {
													Description: "DataVolumeSourceRegistry provides the parameters to create a Data Volume from an registry source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the url of the Docker registry source",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the Registry source",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap provides a reference to the Registry certs",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"pvc": {
													Description: "DataVolumeSourcePVC provides the parameters to create a Data Volume from an existing PVC",
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
												"upload": {
													Description: "DataVolumeSourceUpload provides the parameters to create a Data Volume by uploading the source",
													Type:        "object",
												},
												"vddk": {
													Description: "DataVolumeSourceVDDK provides the parameters to create a Data Volume from a Vmware source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"backingFile": {
															Description: "BackingFile is the path to the virtual hard disk to migrate from vCenter/ESXi",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides a reference to a secret containing the username and password needed to access the vCenter or ESXi host",
															Type:        "string",
														},
														"thumbprint": {
															Description: "Thumbprint is the certificate thumbprint of the vCenter or ESXi host",
															Type:        "string",
														},
														"url": {
															Description: "URL is the URL of the vCenter or ESXi host with the VM to migrate",
															Type:        "string",
														},
														"uuid": {
															Description: "UUID is the UUID of the virtual machine that the backing file is attached to in vCenter/ESXi",
															Type:        "string",
														},
													},
												},
												"blank": {
													Description: "DataVolumeBlankImage provides the parameters to create a new raw blank image for the PVC",
													Type:        "object",
												},
											},
										},
										"pvc": {
											Description: "PVC is the PVC specification",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"resources": {
													Description: "Resources represents the minimum resources the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"limits": {
															Description: "Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
															Type:        "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	AnyOf: []extv1.JSONSchemaProps{
																		{
																			Type: "integer",
																		},
																		{
																			Type: "string",
																		},
																	},
																	Pattern:      "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$",
																	XIntOrString: true,
																},
															},
														},
														"requests": {
															Description: "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
															Type:        "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	AnyOf: []extv1.JSONSchemaProps{
																		{
																			Type: "integer",
																		},
																		{
																			Type: "string",
																		},
																	},
																	Pattern:      "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$",
																	XIntOrString: true,
																},
															},
														},
													},
												},
												"storageClassName": {
													Description: "Name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1",
													Type:        "string",
												},
												"accessModes": {
													Description: "AccessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"dataSource": {
													Description: "This field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot - Beta) * An existing PVC (PersistentVolumeClaim) * An existing custom resource/object that implements data population (Alpha) In order to use VolumeSnapshot object types, the appropriate feature gate must be enabled (VolumeSnapshotDataSource or AnyVolumeDataSource) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. If the specified data source is not supported, the volume will not be created and the failure will be reported as an event. In the future, we plan to support more data source types and the behavior of the provisioner may change.",
													Properties: map[string]extv1.JSONSchemaProps{
														"apiGroup": {
															Description: "APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required.",
															Type:        "string",
														},
														"kind": {
															Description: "Kind is the type of resource being referenced",
															Type:        "string",
														},
														"name": {
															Description: "Name is the name of resource being referenced",
															Type:        "string",
														},
													},
													Type: "object",
													Required: []string{
														"kind",
														"name",
													},
												},
												"selector": {
													Description: "A label query over volumes to consider for binding.",
													Properties: map[string]extv1.JSONSchemaProps{
														"matchExpressions": {
															Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
															Items: &extv1.JSONSchemaPropsOrArray{
																Schema: &extv1.JSONSchemaProps{
																	Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"key": {
																			Description: "key is the label key that the selector applies to.",
																			Type:        "string",
																		},
																		"operator": {
																			Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																			Type:        "string",
																		},
																		"values": {
																			Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Type: "string",
																				},
																			},
																		},
																	},
																	Required: []string{
																		"key",
																		"operator",
																	},
																	Type: "object",
																},
															},
															Type: "array",
														},
														"matchLabels": {
															Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	Type: "string",
																},
															},
															Type: "object",
														},
													},
													Type: "object",
												},
												"volumeMode": {
													Description: "volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.",
													Type:        "string",
												},
												"volumeName": {
													Description: "VolumeName is the binding reference to the PersistentVolume backing this claim.",
													Type:        "string",
												},
											},
										},
									},
									Required: []string{
										"pvc",
										"source",
									},
								},
								"status": {
									Type:        "object",
									Description: "DataVolumeStatus contains the current status of the DataVolume",
									Properties: map[string]extv1.JSONSchemaProps{
										"phase": {
											Description: "Phase is the current phase of the data volume",
											Type:        "string",
										},
										"progress": {
											Description: "DataVolumeProgress is the current progress of the DataVolume transfer operation. Value between 0 and 100 inclusive, N/A if not available",
											Type:        "string",
										},
										"restartCount": {
											Description: "RestartCount is the number of times the pod populating the DataVolume has restarted",
											Type:        "integer",
											Format:      "int32",
										},
										"conditions": {
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "DataVolumeCondition represents the state of a data volume condition.",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Type:   "string",
															Format: "date-time",
														},
														"lastTransitionTime": {
															Type:   "string",
															Format: "date-time",
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
					AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
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
				{
					Name:         "v1beta1",
					Served:       true,
					Storage:      true,
					Subresources: &extv1.CustomResourceSubresources{},
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Description: "DataVolume is an abstraction on top of PersistentVolumeClaims to allow easy population of those PersistentVolumeClaims with relation to VirtualMachines",
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
									Description: "DataVolumeSpec defines the DataVolume type specification",
									Type:        "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"contentType": {
											Description: "DataVolumeContentType options: \"kubevirt\", \"archive\"",
											Type:        "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"kubevirt"`),
												},
												{
													Raw: []byte(`"archive"`),
												},
											},
										},
										"source": {
											Description: "Source is the src of the data for the requested DataVolume",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"http": {
													Description: "DataVolumeSourceHTTP can be either an http or https endpoint, with an optional basic auth user name and password, and an optional configmap containing additional CAs",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the URL of the http(s) endpoint",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef A Secret reference, the secret should contain accessKeyId (user name) base64 encoded, and secretKey (password) also base64 encoded",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap is a configmap reference, containing a Certificate Authority(CA) public key, and a base64 encoded pem certificate",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"imageio": {
													Description: "DataVolumeSourceImageIO provides the parameters to create a Data Volume from an imageio source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the URL of the ovirt-engine",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the ovirt-engine",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap provides a reference to the CA cert",
															Type:        "string",
														},
														"diskId": {
															Description: "DiskID provides id of a disk to be imported",
															Type:        "string",
														},
													},
													Required: []string{
														"diskId",
														"url",
													},
												},
												"s3": {
													Description: "DataVolumeSourceS3 provides the parameters to create a Data Volume from an S3 source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the url of the S3 source",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the S3 source",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"registry": {
													Description: "DataVolumeSourceRegistry provides the parameters to create a Data Volume from an registry source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"url": {
															Description: "URL is the url of the Docker registry source",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides the secret reference needed to access the Registry source",
															Type:        "string",
														},
														"certConfigMap": {
															Description: "CertConfigMap provides a reference to the Registry certs",
															Type:        "string",
														},
													},
													Required: []string{
														"url",
													},
												},
												"pvc": {
													Description: "DataVolumeSourcePVC provides the parameters to create a Data Volume from an existing PVC",
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
												"upload": {
													Description: "DataVolumeSourceUpload provides the parameters to create a Data Volume by uploading the source",
													Type:        "object",
												},
												"vddk": {
													Description: "DataVolumeSourceVDDK provides the parameters to create a Data Volume from a Vmware source",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"backingFile": {
															Description: "BackingFile is the path to the virtual hard disk to migrate from vCenter/ESXi",
															Type:        "string",
														},
														"secretRef": {
															Description: "SecretRef provides a reference to a secret containing the username and password needed to access the vCenter or ESXi host",
															Type:        "string",
														},
														"thumbprint": {
															Description: "Thumbprint is the certificate thumbprint of the vCenter or ESXi host",
															Type:        "string",
														},
														"url": {
															Description: "URL is the URL of the vCenter or ESXi host with the VM to migrate",
															Type:        "string",
														},
														"uuid": {
															Description: "UUID is the UUID of the virtual machine that the backing file is attached to in vCenter/ESXi",
															Type:        "string",
														},
													},
												},
												"blank": {
													Description: "DataVolumeBlankImage provides the parameters to create a new raw blank image for the PVC",
													Type:        "object",
												},
											},
										},
										"pvc": {
											Description: "PVC is the PVC specification",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"resources": {
													Description: "Resources represents the minimum resources the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"limits": {
															Description: "Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
															Type:        "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	AnyOf: []extv1.JSONSchemaProps{
																		{
																			Type: "integer",
																		},
																		{
																			Type: "string",
																		},
																	},
																	Pattern:      "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$",
																	XIntOrString: true,
																},
															},
														},
														"requests": {
															Description: "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
															Type:        "object",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	AnyOf: []extv1.JSONSchemaProps{
																		{
																			Type: "integer",
																		},
																		{
																			Type: "string",
																		},
																	},
																	Pattern:      "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$",
																	XIntOrString: true,
																},
															},
														},
													},
												},
												"storageClassName": {
													Description: "Name of the StorageClass required by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1",
													Type:        "string",
												},
												"accessModes": {
													Description: "AccessModes contains the desired access modes the volume should have. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"dataSource": {
													Description: "This field can be used to specify either: * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot - Beta) * An existing PVC (PersistentVolumeClaim) * An existing custom resource/object that implements data population (Alpha) In order to use VolumeSnapshot object types, the appropriate feature gate must be enabled (VolumeSnapshotDataSource or AnyVolumeDataSource) If the provisioner or an external controller can support the specified data source, it will create a new volume based on the contents of the specified data source. If the specified data source is not supported, the volume will not be created and the failure will be reported as an event. In the future, we plan to support more data source types and the behavior of the provisioner may change.",
													Properties: map[string]extv1.JSONSchemaProps{
														"apiGroup": {
															Description: "APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required.",
															Type:        "string",
														},
														"kind": {
															Description: "Kind is the type of resource being referenced",
															Type:        "string",
														},
														"name": {
															Description: "Name is the name of resource being referenced",
															Type:        "string",
														},
													},
													Type: "object",
													Required: []string{
														"kind",
														"name",
													},
												},
												"selector": {
													Description: "A label query over volumes to consider for binding.",
													Properties: map[string]extv1.JSONSchemaProps{
														"matchExpressions": {
															Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
															Items: &extv1.JSONSchemaPropsOrArray{
																Schema: &extv1.JSONSchemaProps{
																	Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"key": {
																			Description: "key is the label key that the selector applies to.",
																			Type:        "string",
																		},
																		"operator": {
																			Description: "operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.",
																			Type:        "string",
																		},
																		"values": {
																			Description: "values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Type: "string",
																				},
																			},
																		},
																	},
																	Required: []string{
																		"key",
																		"operator",
																	},
																	Type: "object",
																},
															},
															Type: "array",
														},
														"matchLabels": {
															Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
															AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																Schema: &extv1.JSONSchemaProps{
																	Type: "string",
																},
															},
															Type: "object",
														},
													},
													Type: "object",
												},
												"volumeMode": {
													Description: "volumeMode defines what type of volume is required by the claim. Value of Filesystem is implied when not included in claim spec.",
													Type:        "string",
												},
												"volumeName": {
													Description: "VolumeName is the binding reference to the PersistentVolume backing this claim.",
													Type:        "string",
												},
											},
										},
									},
									Required: []string{
										"pvc",
										"source",
									},
								},
								"status": {
									Type:        "object",
									Description: "DataVolumeStatus contains the current status of the DataVolume",
									Properties: map[string]extv1.JSONSchemaProps{
										"phase": {
											Description: "Phase is the current phase of the data volume",
											Type:        "string",
										},
										"progress": {
											Description: "DataVolumeProgress is the current progress of the DataVolume transfer operation. Value between 0 and 100 inclusive, N/A if not available",
											Type:        "string",
										},
										"restartCount": {
											Description: "RestartCount is the number of times the pod populating the DataVolume has restarted",
											Type:        "integer",
											Format:      "int32",
										},
										"conditions": {
											Items: &extv1.JSONSchemaPropsOrArray{
												Schema: &extv1.JSONSchemaProps{
													Description: "DataVolumeCondition represents the state of a data volume condition.",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"lastHeartbeatTime": {
															Type:   "string",
															Format: "date-time",
														},
														"lastTransitionTime": {
															Type:   "string",
															Format: "date-time",
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
					AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
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
			},
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
			Scope: "Namespaced",
		},
	}
}
