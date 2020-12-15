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

package operator

import (
	"encoding/json"

	"github.com/coreos/go-semver/semver"
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	sdkopenapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources/openapi"
)

const (
	serviceAccountName = "cdi-operator"
	roleName           = "cdi-operator"
	clusterRoleName    = roleName + "-cluster"
	prometheusLabel    = common.PrometheusLabel
)

func getClusterPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"rbac.authorization.k8s.io",
			},
			Resources: []string{
				"clusterrolebindings",
				"clusterroles",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"security.openshift.io",
			},
			Resources: []string{
				"securitycontextconstraints",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
				"create",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
				"services",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"apiextensions.k8s.io",
			},
			Resources: []string{
				"customresourcedefinitions",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
				"upload.cdi.kubevirt.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"admissionregistration.k8s.io",
			},
			Resources: []string{
				"validatingwebhookconfigurations",
				"mutatingwebhookconfigurations",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apiregistration.k8s.io",
			},
			Resources: []string{
				"apiservices",
			},
			Verbs: []string{
				"*",
			},
		},
	}
	rules = append(rules, cluster.GetClusterRolePolicyRules()...)
	return rules
}

func createClusterRole() *rbacv1.ClusterRole {
	return utils.ResourcesBuiler.CreateOperatorClusterRole(clusterRoleName, getClusterPolicyRules())
}

func createClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return utils.ResourcesBuiler.CreateOperatorClusterRoleBinding(serviceAccountName, clusterRoleName, serviceAccountName, namespace)
}

func createClusterRBAC(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createClusterRole(),
		createClusterRoleBinding(args.NamespacedArgs.Namespace),
	}
}

func getNamespacedPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"rbac.authorization.k8s.io",
			},
			Resources: []string{
				"rolebindings",
				"roles",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"serviceaccounts",
				"configmaps",
				"events",
				"secrets",
				"services",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"deployments",
				"deployments/finalizers",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"route.openshift.io",
			},
			Resources: []string{
				"routes",
				"routes/custom-host",
			},
			Verbs: []string{
				"*",
			},
		},
	}
	return rules
}

func createServiceAccount(namespace string) *corev1.ServiceAccount {
	return utils.ResourcesBuiler.CreateOperatorServiceAccount(serviceAccountName, namespace)
}

func createNamespacedRole(namespace string) *rbacv1.Role {
	role := utils.ResourcesBuiler.CreateRole(roleName, getNamespacedPolicyRules())
	role.Namespace = namespace
	return role
}

func createNamespacedRoleBinding(namespace string) *rbacv1.RoleBinding {
	roleBinding := utils.ResourcesBuiler.CreateRoleBinding(serviceAccountName, roleName, serviceAccountName, namespace)
	roleBinding.Namespace = namespace
	return roleBinding
}

func createNamespacedRBAC(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createServiceAccount(args.NamespacedArgs.Namespace),
		createNamespacedRole(args.NamespacedArgs.Namespace),
		createNamespacedRoleBinding(args.NamespacedArgs.Namespace),
	}
}

func createDeployment(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createOperatorDeployment(args.NamespacedArgs.OperatorVersion,
			args.NamespacedArgs.Namespace,
			args.NamespacedArgs.DeployClusterResources,
			args.Image,
			args.NamespacedArgs.ControllerImage,
			args.NamespacedArgs.ImporterImage,
			args.NamespacedArgs.ClonerImage,
			args.NamespacedArgs.APIServerImage,
			args.NamespacedArgs.UploadProxyImage,
			args.NamespacedArgs.UploadServerImage,
			args.NamespacedArgs.Verbosity,
			args.NamespacedArgs.PullPolicy),
		createOperatorLeaderElectionConfigMap(args.NamespacedArgs.Namespace),
	}
}

func createOperatorLeaderElectionConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdi-operator-leader-election-helper",
			Namespace: namespace,
			Labels: map[string]string{
				"operator.cdi.kubevirt.io": "",
			},
		},
	}

}

func createCRD(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createCDIListCRD(),
	}
}

func createCDIListCRD() *extv1.CustomResourceDefinition {
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdis.cdi.kubevirt.io",
			Labels: map[string]string{
				"operator.cdi.kubevirt.io": "",
			},
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "cdi.kubevirt.io",
			Scope: "Cluster",
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type:        "object",
							Description: "CDI is the CDI Operator CRD",
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
									Properties: map[string]extv1.JSONSchemaProps{
										"imagePullPolicy": {
											Description: "PullPolicy describes a policy for if/when to pull a container image",
											Type:        "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"Always"`),
												},
												{
													Raw: []byte(`"IfNotPresent"`),
												},
												{
													Raw: []byte(`"Never"`),
												},
											},
										},
										"infra": {
											Description: "Rules on which nodes CDI infrastructure pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
										"workload": {
											Description: "Restrict on which nodes CDI workload pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
										"uninstallStrategy": {
											Type:        "string",
											Description: "CDIUninstallStrategy defines the state to leave CDI on uninstall",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"RemoveWorkloads"`),
												},
												{
													Raw: []byte(`"BlockUninstallIfWorkloadsExist"`),
												},
											},
										},
										"cloneStrategyOverride": {
											Type:        "string",
											Description: "Clone strategy override: should we use a host-assisted copy even if snapshots are available?",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"copy"`),
												},
												{
													Raw: []byte(`"snapshot"`),
												},
											},
										},
										"config": {
											Description: "CDIConfig at CDI level",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"uploadProxyURLOverride": {
													Description: "Override the URL used when uploading to a DataVolume",
													Type:        "string",
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
											},
										},
										"certConfig": {
											Type:        "object",
											Description: "certificate configuration",
											Properties: map[string]extv1.JSONSchemaProps{
												"ca": {
													Type:        "object",
													Description: "CA configuration CA certs are kept in the CA bundle as long as they are valid",
													Properties: map[string]extv1.JSONSchemaProps{
														"duration": {
															Description: "The requested 'duration' (i.e. lifetime) of the Certificate.",
															Type:        "string",
														},
														"renewBefore": {
															Description: "The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate.",
															Type:        "string",
														},
													},
												},
												"server": {
													Type:        "object",
													Description: "Server configuration Certs are rotated and discarded",
													Properties: map[string]extv1.JSONSchemaProps{
														"duration": {
															Description: "The requested 'duration' (i.e. lifetime) of the Certificate.",
															Type:        "string",
														},
														"renewBefore": {
															Description: "The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate.",
															Type:        "string",
														},
													},
												},
											},
										},
									},
									Type:        "object",
									Description: "CDISpec defines our specification for the CDI installation",
								},
								"status": sdkopenapi.OperatorConfigStatus("CDIStatus"),
							},
							Required: []string{
								"spec",
							},
						},
					},
					AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
						{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
						{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
					},
					Subresources: &extv1.CustomResourceSubresources{},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: &extv1.JSONSchemaProps{
							Type:        "object",
							Description: "CDI is the CDI Operator CRD",
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
									Properties: map[string]extv1.JSONSchemaProps{
										"imagePullPolicy": {
											Description: "PullPolicy describes a policy for if/when to pull a container image",
											Type:        "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"Always"`),
												},
												{
													Raw: []byte(`"IfNotPresent"`),
												},
												{
													Raw: []byte(`"Never"`),
												},
											},
										},
										"infra": {
											Description: "Rules on which nodes CDI infrastructure pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
										"workload": {
											Description: "Restrict on which nodes CDI workload pods will be scheduled",
											Type:        "object",
											Properties: map[string]extv1.JSONSchemaProps{
												"affinity": {
													Description: "affinity enables pod affinity/anti-affinity placement expanding the types of constraints that can be expressed with nodeSelector. affinity is going to be applied to the relevant kind of pods in parallel with nodeSelector See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity",
													Type:        "object",
													Properties: map[string]extv1.JSONSchemaProps{
														"nodeAffinity": {
															Description: "Describes node affinity scheduling rules for the pod.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"preference": {
																					Description: "A node selector term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																				"weight": {
																					Description: "Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.",
																					Format:      "int32",
																					Type:        "integer",
																				},
																			},
																			Required: []string{
																				"preference",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.",
																	Type:        "object",
																	Properties: map[string]extv1.JSONSchemaProps{
																		"nodeSelectorTerms": {
																			Description: "Required. A list of node selector terms. The terms are ORed.",
																			Type:        "array",
																			Items: &extv1.JSONSchemaPropsOrArray{
																				Schema: &extv1.JSONSchemaProps{
																					Description: "A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "A list of node selector requirements by node's labels.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																						"matchFields": {
																							Description: "A list of node selector requirements by node's fields.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
																									Properties: map[string]extv1.JSONSchemaProps{
																										"key": {
																											Description: "The label key that the selector applies to.",
																											Type:        "string",
																										},
																										"operator": {
																											Description: "Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.",
																											Type:        "string",
																										},
																										"values": {
																											Description: "An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.",
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
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																	Required: []string{
																		"nodeSelectorTerms",
																	},
																},
															},
														},
														"podAffinity": {
															Description: "Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
														"podAntiAffinity": {
															Description: "Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"preferredDuringSchedulingIgnoredDuringExecution": {
																	Description: "The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \"weight\" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"podAffinityTerm": {
																					Description: "Required. A pod affinity term, associated with the corresponding weight.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"labelSelector": {
																							Description: "A label query over a set of resources, in this case pods.",
																							Type:        "object",
																							Properties: map[string]extv1.JSONSchemaProps{
																								"matchExpressions": {
																									Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																									Type:        "array",
																									Items: &extv1.JSONSchemaPropsOrArray{
																										Schema: &extv1.JSONSchemaProps{
																											Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																											Type:        "object",
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
																										},
																									},
																								},
																								"matchLabels": {
																									Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																									Type:        "object",
																									AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																										Schema: &extv1.JSONSchemaProps{
																											Type: "string",
																										},
																									},
																								},
																							},
																						},
																						"namespaces": {
																							Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																						"topologyKey": {
																							Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																							Type:        "string",
																						},
																					},
																					Required: []string{
																						"topologyKey",
																					},
																				},
																				"weight": {
																					Description: "weight associated with matching the corresponding podAffinityTerm, in the range 1-100.",
																					Type:        "integer",
																					Format:      "int32",
																				},
																			},
																			Required: []string{
																				"podAffinityTerm",
																				"weight",
																			},
																		},
																	},
																},
																"requiredDuringSchedulingIgnoredDuringExecution": {
																	Description: "If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.",
																	Type:        "array",
																	Items: &extv1.JSONSchemaPropsOrArray{
																		Schema: &extv1.JSONSchemaProps{
																			Description: "Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running",
																			Type:        "object",
																			Properties: map[string]extv1.JSONSchemaProps{
																				"labelSelector": {
																					Description: "A label query over a set of resources, in this case pods.",
																					Type:        "object",
																					Properties: map[string]extv1.JSONSchemaProps{
																						"matchExpressions": {
																							Description: "matchExpressions is a list of label selector requirements. The requirements are ANDed.",
																							Type:        "array",
																							Items: &extv1.JSONSchemaPropsOrArray{
																								Schema: &extv1.JSONSchemaProps{
																									Description: "A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.",
																									Type:        "object",
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
																								},
																							},
																						},
																						"matchLabels": {
																							Description: "matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.",
																							Type:        "object",
																							AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
																								Schema: &extv1.JSONSchemaProps{
																									Type: "string",
																								},
																							},
																						},
																					},
																				},
																				"namespaces": {
																					Description: "namespaces specifies which namespaces the labelSelector applies to (matches against); null or empty list means \"this pod's namespace\"",
																					Type:        "array",
																					Items: &extv1.JSONSchemaPropsOrArray{
																						Schema: &extv1.JSONSchemaProps{
																							Type: "string",
																						},
																					},
																				},
																				"topologyKey": {
																					Description: "This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.",
																					Type:        "string",
																				},
																			},
																			Required: []string{
																				"topologyKey",
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"nodeSelector": {
													Description: "nodeSelector is the node selector applied to the relevant kind of pods It specifies a map of key-value pairs: for the pod to be eligible to run on a node, the node must have each of the indicated key-value pairs as labels (it can have additional labels as well). See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
													Type:        "object",
													AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
														Schema: &extv1.JSONSchemaProps{
															Type: "string",
														},
													},
												},
												"tolerations": {
													Description: "tolerations is a list of tolerations applied to the relevant kind of pods See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ for more info. These are additional tolerations other than default ones.",
													Type:        "array",
													Items: &extv1.JSONSchemaPropsOrArray{
														Schema: &extv1.JSONSchemaProps{
															Description: "The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
															Type:        "object",
															Properties: map[string]extv1.JSONSchemaProps{
																"effect": {
																	Description: "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.",
																	Type:        "string",
																},
																"key": {
																	Description: "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
																	Type:        "string",
																},
																"operator": {
																	Description: "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.",
																	Type:        "string",
																},
																"tolerationSeconds": {
																	Description: "TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.",
																	Type:        "integer",
																	Format:      "int64",
																},
																"value": {
																	Description: "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
																	Type:        "string",
																},
															},
														},
													},
												},
											},
										},
										"uninstallStrategy": {
											Type:        "string",
											Description: "CDIUninstallStrategy defines the state to leave CDI on uninstall",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"RemoveWorkloads"`),
												},
												{
													Raw: []byte(`"BlockUninstallIfWorkloadsExist"`),
												},
											},
										},
										"cloneStrategyOverride": {
											Type:        "string",
											Description: "Clone strategy override: should we use a host-assisted copy even if snapshots are available?",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"copy"`),
												},
												{
													Raw: []byte(`"snapshot"`),
												},
											},
										},
										"config": {
											Description: "CDIConfig at CDI level",
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
										"certConfig": {
											Type:        "object",
											Description: "certificate configuration",
											Properties: map[string]extv1.JSONSchemaProps{
												"ca": {
													Type:        "object",
													Description: "CA configuration CA certs are kept in the CA bundle as long as they are valid",
													Properties: map[string]extv1.JSONSchemaProps{
														"duration": {
															Description: "The requested 'duration' (i.e. lifetime) of the Certificate.",
															Type:        "string",
														},
														"renewBefore": {
															Description: "The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate.",
															Type:        "string",
														},
													},
												},
												"server": {
													Type:        "object",
													Description: "Server configuration Certs are rotated and discarded",
													Properties: map[string]extv1.JSONSchemaProps{
														"duration": {
															Description: "The requested 'duration' (i.e. lifetime) of the Certificate.",
															Type:        "string",
														},
														"renewBefore": {
															Description: "The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate.",
															Type:        "string",
														},
													},
												},
											},
										},
									},
									Type:        "object",
									Description: "CDISpec defines our specification for the CDI installation",
								},
								"status": sdkopenapi.OperatorConfigStatus("CDIStatus"),
							},
							Required: []string{
								"spec",
							},
						},
					},
					AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
						{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
						{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
					},
					Subresources: &extv1.CustomResourceSubresources{},
				},
			},
			Conversion: &extv1.CustomResourceConversion{
				Strategy: extv1.NoneConverter,
			},
			Names: extv1.CustomResourceDefinitionNames{
				Kind:       "CDI",
				ListKind:   "CDIList",
				Plural:     "cdis",
				Singular:   "cdi",
				ShortNames: []string{"cdi", "cdis"},
			},
		},
	}
}

func createOperatorEnvVar(operatorVersion, deployClusterResources, operatorImage, controllerImage, importerImage, clonerImage, apiServerImage, uploadProxyImage, uploadServerImage, verbosity, pullPolicy string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "DEPLOY_CLUSTER_RESOURCES",
			Value: deployClusterResources,
		},
		{
			Name:  "OPERATOR_VERSION",
			Value: operatorVersion,
		},
		{
			Name:  "CONTROLLER_IMAGE",
			Value: controllerImage,
		},
		{
			Name:  "IMPORTER_IMAGE",
			Value: importerImage,
		},
		{
			Name:  "CLONER_IMAGE",
			Value: clonerImage,
		},
		{
			Name:  "APISERVER_IMAGE",
			Value: apiServerImage,
		},
		{
			Name:  "UPLOAD_SERVER_IMAGE",
			Value: uploadServerImage,
		},
		{
			Name:  "UPLOAD_PROXY_IMAGE",
			Value: uploadProxyImage,
		},
		{
			Name:  "VERBOSITY",
			Value: verbosity,
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
	}
}

func createOperatorDeployment(operatorVersion, namespace, deployClusterResources, operatorImage, controllerImage, importerImage, clonerImage, apiServerImage, uploadProxyImage, uploadServerImage, verbosity, pullPolicy string) *appsv1.Deployment {
	deployment := utils.CreateOperatorDeployment("cdi-operator", namespace, "name", "cdi-operator", serviceAccountName, int32(1))
	container := utils.CreatePortsContainer("cdi-operator", operatorImage, pullPolicy, createPrometheusPorts())
	container.Env = createOperatorEnvVar(operatorVersion, deployClusterResources, operatorImage, controllerImage, importerImage, clonerImage, apiServerImage, uploadProxyImage, uploadServerImage, verbosity, pullPolicy)
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	return deployment
}

func createPrometheusPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "metrics",
			ContainerPort: 60000,
			Protocol:      "TCP",
		},
	}
}

type csvPermissions struct {
	ServiceAccountName string              `json:"serviceAccountName"`
	Rules              []rbacv1.PolicyRule `json:"rules"`
}
type csvDeployments struct {
	Name string                `json:"name"`
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

type csvStrategySpec struct {
	Permissions        []csvPermissions `json:"permissions"`
	ClusterPermissions []csvPermissions `json:"clusterPermissions"`
	Deployments        []csvDeployments `json:"deployments"`
}

func createClusterServiceVersion(data *ClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {

	description := `
CDI is a kubernetes extension that provides the ability to populate PVCs with VM images upon creation. Multiple image formats and sources are supported

_The CDI Operator does not support updates yet._
`

	deployment := createOperatorDeployment(
		data.OperatorVersion,
		data.Namespace,
		"true",
		data.OperatorImage,
		data.ControllerImage,
		data.ImporterImage,
		data.ClonerImage,
		data.APIServerImage,
		data.UplodaProxyImage,
		data.UplodaServerImage,
		data.Verbosity,
		data.ImagePullPolicy)

	strategySpec := csvStrategySpec{
		Permissions: []csvPermissions{
			{
				ServiceAccountName: serviceAccountName,
				Rules:              getNamespacedPolicyRules(),
			},
		},
		ClusterPermissions: []csvPermissions{
			{
				ServiceAccountName: serviceAccountName,
				Rules:              getClusterPolicyRules(),
			},
		},
		Deployments: []csvDeployments{
			{
				Name: "cdi-operator",
				Spec: deployment.Spec,
			},
		},
	}

	strategySpecJSONBytes, err := json.Marshal(strategySpec)
	if err != nil {
		return nil, err
	}

	return &csvv1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterServiceVersion",
			APIVersion: "operators.coreos.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdioperator." + data.CsvVersion,
			Namespace: data.Namespace,
			Annotations: map[string]string{

				"capabilities": "Full Lifecycle",
				"categories":   "Storage,Virtualization",
				"alm-examples": `
      [
        {
          "apiVersion":"cdi.kubevirt.io/v1beta1",
          "kind":"CDI",
          "metadata": {
            "name":"cdi",
            "namespace":"cdi"
          },
          "spec": {
            "imagePullPolicy":"IfNotPresent"
          }
        }
      ]`,
				"description": "Creates and maintains CDI deployments",
			},
		},

		Spec: csvv1.ClusterServiceVersionSpec{
			DisplayName: "CDI",
			Description: description,
			Keywords:    []string{"CDI", "Virtualization", "Storage"},
			Version:     *semver.New(data.CsvVersion),
			Maturity:    "alpha",
			Replaces:    data.ReplacesCsvVersion,
			Maintainers: []csvv1.Maintainer{{
				Name:  "KubeVirt project",
				Email: "kubevirt-dev@googlegroups.com",
			}},
			Provider: csvv1.AppLink{
				Name: "KubeVirt/CDI project",
			},
			Links: []csvv1.AppLink{
				{
					Name: "CDI",
					URL:  "https://github.com/kubevirt/containerized-data-importer/blob/master/README.md",
				},
				{
					Name: "Source Code",
					URL:  "https://github.com/kubevirt/containerized-data-importer",
				},
			},
			Icon: []csvv1.Icon{{
				Data:      data.IconBase64,
				MediaType: "image/png",
			}},
			Labels: map[string]string{
				"alm-owner-cdi": "cdi-operator",
				"operated-by":   "cdi-operator",
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"alm-owner-cdi": "cdi-operator",
					"operated-by":   "cdi-operator",
				},
			},
			InstallModes: []csvv1.InstallMode{
				{
					Type:      csvv1.InstallModeTypeOwnNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeSingleNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeMultiNamespace,
					Supported: true,
				},
				{
					Type:      csvv1.InstallModeTypeAllNamespaces,
					Supported: true,
				},
			},
			InstallStrategy: csvv1.NamedInstallStrategy{
				StrategyName:    "deployment",
				StrategySpecRaw: json.RawMessage(strategySpecJSONBytes),
			},
			CustomResourceDefinitions: csvv1.CustomResourceDefinitions{

				Owned: []csvv1.CRDDescription{
					{
						Name:        "cdis.cdi.kubevirt.io",
						Version:     "v1beta1",
						Kind:        "CDI",
						DisplayName: "CDI deployment",
						Description: "Represents a CDI deployment",
						Resources: []csvv1.APIResourceReference{
							{
								Kind:    "ConfigMap",
								Name:    "cdi-operator-leader-election-helper",
								Version: "v1",
							},
						},
						SpecDescriptors: []csvv1.SpecDescriptor{

							{
								Description:  "The ImageRegistry to use for the CDI components.",
								DisplayName:  "ImageRegistry",
								Path:         "imageRegistry",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The ImageTag to use for the CDI components.",
								DisplayName:  "ImageTag",
								Path:         "imageTag",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The ImagePullPolicy to use for the CDI components.",
								DisplayName:  "ImagePullPolicy",
								Path:         "imagePullPolicy",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:imagePullPolicy"},
							},
						},
						StatusDescriptors: []csvv1.StatusDescriptor{
							{
								Description:  "The deployment phase.",
								DisplayName:  "Phase",
								Path:         "phase",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes.phase"},
							},
							{
								Description:  "Explanation for the current status of the CDI deployment.",
								DisplayName:  "Conditions",
								Path:         "conditions",
								XDescriptors: []string{"urn:alm:descriptor:io.kubernetes.conditions"},
							},
							{
								Description:  "The observed version of the CDI deployment.",
								DisplayName:  "Observed CDI Version",
								Path:         "observedVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The targeted version of the CDI deployment.",
								DisplayName:  "Target CDI Version",
								Path:         "targetVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
							{
								Description:  "The version of the CDI Operator",
								DisplayName:  "CDI Operator Version",
								Path:         "operatorVersion",
								XDescriptors: []string{"urn:alm:descriptor:text"},
							},
						},
					},
				},
			},
		},
	}, nil
}
