package cluster

import (
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

func createCDIConfigCRD() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cdiconfigs.cdi.kubevirt.io",
			Labels: utils.WithCommonLabels(nil),
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:     "CDIConfig",
				Plural:   "cdiconfigs",
				Singular: "cdiconfig",
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
			Scope: "Cluster",
			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Description: "CDIConfig is the configuration object for Containerized Data Importer",
					Type:        "object",
					Properties: map[string]extv1beta1.JSONSchemaProps{
						"spec": {
							Description: "Specification of CDIConfig",
							Type:        "object",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"uploadProxyURLOverride": {
									Description: "Override the URL used when uploading to a DataVolume",
									Type:        "string",
								},
								"scratchSpaceStorageClass": {
									Description: "Override the storage class to used for scratch space during transfer operations. The scratch space storage class is determined in the following order: 1. value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for scratch space",
									Type:        "string",
								},
								"podResourceRequirements": {
									Description: "The default resource requirements put on all CDI pods",
									Type:        "object",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"limits": {
											Description: "Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
											Type:        "object",
										},
										"requests": {
											Description: "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
											Type:        "object",
										},
									},
								},
							},
						},
						"status": {
							Type:        "object",
							Description: "The most recently observed status of the CDI Config resource",
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"uploadProxyURL": {
									Description: "The calculated upload proxy URL",
									Type:        "string",
								},
								"scratchSpaceStorageClass": {
									Description: "The calculated storage class to be used for scratch space",
									Type:        "string",
								},
								"defaultPodResourceRequirements": {
									Description: "The default resource requirements put on all CDI pods",
									Type:        "object",
									Properties: map[string]extv1beta1.JSONSchemaProps{
										"limits": {
											Description: "Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
											Type:        "object",
										},
										"requests": {
											Description: "Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/",
											Type:        "object",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
