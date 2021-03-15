package cluster

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

// NewCdiConfigCrd - provides CDIConfig CRD
func NewCdiConfigCrd() *extv1.CustomResourceDefinition {
	return createCDIConfigCRD()
}

// createCDIConfigCRD creates the CDIConfig schema
func createCDIConfigCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cdiconfigs.cdi.kubevirt.io",
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "CDIConfig",
				Plural:   "cdiconfigs",
				Singular: "cdiconfig",
				ListKind: "CDIConfigList",
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Schema:  createAlphaV1ConfigCRDSchema(),
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Schema:  createBetaV1ConfigCRDSchema(),
				},
			},
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
			Scope: "Cluster",
		},
	}
}

func createBetaV1ConfigCRDSchema() *extv1.CustomResourceValidation {
	// BetaV1 might diverge here from AlphaV1
	return createAlphaV1ConfigCRDSchema()
}

func createAlphaV1ConfigCRDSchema() *extv1.CustomResourceValidation {
	return &extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Description: "CDIConfig provides a user configuration for CDI",
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
					Description: "CDIConfigSpec defines specification for user configuration",
					Type:        "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"featureGates": {
							Description: "FeatureGates are a list of specific enabled feature gates",
							Items: &extv1.JSONSchemaPropsOrArray{
								Schema: &extv1.JSONSchemaProps{
									Type: "string",
								},
							},
							Type: "array",
						},
						"uploadProxyURLOverride": {
							Description: "Override the URL used when uploading to a DataVolume",
							Type:        "string",
						},
						"importProxy": {
							Description: "ImportProxy contains importer pod proxy configuration.",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"HTTPProxy": {
									Description: "HTTPProxy is the URL http://<username>:<pswd>@<ip>:<port> of the import proxy for HTTP requests.  Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"HTTPSProxy": {
									Description: "HTTPSProxy is the URL https://<username>:<pswd>@<ip>:<port> of the import proxy for HTTPS requests.  Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"noProxy": {
									Description: "NoProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used. Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"trustedCAProxy": {
									Description: "TrustedCAProxy is the name of a ConfigMap in the cdi namespace that contains a user-provided trusted certificate authority (CA) bundle. The TrustedCAProxy field is consumed by the import controller that is resposible for coping it to a config map named trusted-ca-proxy-bundle-cm in the cdi namespace. Here is an example of the ConfigMap (in yaml): \n apiVersion: v1 kind: ConfigMap metadata:   name: trusted-ca-proxy-bundle-cm   namespace: cdi data:   ca.pem: |     -----BEGIN CERTIFICATE----- \t   ... <base64 encoded cert> ... \t   -----END CERTIFICATE-----",
									Type:        "string",
								},
							},
						},
						"scratchSpaceStorageClass": {
							Description: "Override the storage class to used for scratch space during transfer operations. The scratch space storage class is determined in the following order: 1. value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for scratch space",
							Type:        "string",
						},
						"podResourceRequirements": {
							Description: "ResourceRequirements describes the compute resource requirements.",
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
						"filesystemOverhead": {
							Description: "FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A value is between 0 and 1, if not defined it is 0.055 (5.5% overhead)",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"global": {
									Description: "Global is how much space of a Filesystem volume should be reserved for overhead. This value is used unless overridden by a more specific value (per storageClass)",
									Type:        "string",
									Pattern:     `^(0(?:\.\d{1,3})?|1)$`,
								},
								"storageClass": {
									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
										Schema: &extv1.JSONSchemaProps{
											Type:        "string",
											Pattern:     `^(0(?:\.\d{1,3})?|1)$`,
											Description: "Percent is a string that can only be a value between [0,1) (Note: we actually rely on reconcile to reject invalid values)",
										},
									},
									Description: "StorageClass specifies how much space of a Filesystem volume should be reserved for safety. The keys are the storageClass and the values are the overhead. This value overrides the global value",
									Type:        "object",
								},
							},
						},
						"preallocation": {
							Description: "Preallocation controls whether storage for DataVolumes should be allocated in advance.",
							Type:        "boolean",
						},
					},
				},
				"status": {
					Type:        "object",
					Description: "CDIConfigStatus provides the most recently observed status of the CDI Config resource",
					Properties: map[string]extv1.JSONSchemaProps{
						"uploadProxyURL": {
							Description: "The calculated upload proxy URL",
							Type:        "string",
						},
						"importProxy": {
							Description: "ImportProxy contains importer pod proxy configuration.",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"HTTPProxy": {
									Description: "HTTPProxy is the URL http://<username>:<pswd>@<ip>:<port> of the import proxy for HTTP requests.  Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"HTTPSProxy": {
									Description: "HTTPSProxy is the URL https://<username>:<pswd>@<ip>:<port> of the import proxy for HTTPS requests.  Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"noProxy": {
									Description: "NoProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used. Empty means unset and will not result in the import pod env var.",
									Type:        "string",
								},
								"trustedCAProxy": {
									Description: "TrustedCAProxy is the name of a ConfigMap in the cdi namespace that contains a user-provided trusted certificate authority (CA) bundle. The TrustedCAProxy field is consumed by the import controller that is resposible for coping it to a config map named trusted-ca-proxy-bundle-cm in the cdi namespace. Here is an example of the ConfigMap (in yaml): \n apiVersion: v1 kind: ConfigMap metadata:   name: trusted-ca-proxy-bundle-cm   namespace: cdi data:   ca.pem: |     -----BEGIN CERTIFICATE----- \t   ... <base64 encoded cert> ... \t   -----END CERTIFICATE-----",
									Type:        "string",
								},
							},
						},
						"scratchSpaceStorageClass": {
							Description: "The calculated storage class to be used for scratch space",
							Type:        "string",
						},
						"defaultPodResourceRequirements": {
							Description: "ResourceRequirements describes the compute resource requirements.",
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
						"filesystemOverhead": {
							Description: "FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A percentage value is between 0 and 1",
							Type:        "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"global": {
									Description: "Global is how much space of a Filesystem volume should be reserved for overhead. This value is used unless overridden by a more specific value (per storageClass)",
									Type:        "string",
									Pattern:     `^(0(?:\.\d{1,3})?|1)$`,
								},
								"storageClass": {
									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
										Schema: &extv1.JSONSchemaProps{
											Type:        "string",
											Pattern:     `^(0(?:\.\d{1,3})?|1)$`,
											Description: "Percent is a string that can only be a value between [0,1) (Note: we actually rely on reconcile to reject invalid values)",
										},
									},
									Description: "StorageClass specifies how much space of a Filesystem volume should be reserved for safety. The keys are the storageClass and the values are the overhead. This value overrides the global value",
									Type:        "object",
								},
							},
						},
						"preallocation": {
							Description: "Preallocation controls whether storage for DataVolumes should be allocated in advance.",
							Type:        "boolean",
						},
					},
				},
			},
			Required: []string{
				"spec",
			},
		},
	}
}
