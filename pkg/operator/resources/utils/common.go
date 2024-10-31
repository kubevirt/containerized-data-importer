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

package utils

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	utils "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
)

const (
	// CDILabel is the labe applied to all non operator resources
	CDILabel = "cdi.kubevirt.io"
	// CDIPriorityClass is the priority class for all CDI pods.
	CDIPriorityClass = "kubevirt-cluster-critical"
)

var commonLabels = map[string]string{
	CDILabel:                           "",
	common.AppKubernetesManagedByLabel: "cdi-operator",
	common.AppKubernetesComponentLabel: "storage",
	common.CDILabelKey:                 common.CDILabelValue,
}

var operatorLabels = map[string]string{
	"operator.cdi.kubevirt.io": "",
}

// ResourceBuilder helps in creating k8s resources
var ResourceBuilder = utils.NewResourceBuilder(commonLabels, operatorLabels)

// CreateContainer creates container
func CreateContainer(name, image, verbosity, pullPolicy string) corev1.Container {
	container := ResourceBuilder.CreateContainer(name, image, pullPolicy)
	container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	container.TerminationMessagePath = corev1.TerminationMessagePathDefault
	container.Args = []string{"-v=" + verbosity}
	container.SecurityContext = &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		AllowPrivilegeEscalation: ptr.To[bool](false),
		RunAsNonRoot:             ptr.To[bool](true),
	}
	return *container
}

// CreatePortsContainer creates container with ports
func CreatePortsContainer(name, image, pullPolicy string, ports []corev1.ContainerPort) corev1.Container {
	container := ResourceBuilder.CreatePortsContainer(name, image, pullPolicy, ports)
	container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	container.TerminationMessagePath = corev1.TerminationMessagePathDefault
	container.SecurityContext = &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		AllowPrivilegeEscalation: ptr.To[bool](false),
		RunAsNonRoot:             ptr.To[bool](true),
	}
	return *container
}

// CreateDeployment creates deployment
func CreateDeployment(name, matchKey, matchValue, serviceAccountName string, imagePullSecrets []corev1.LocalObjectReference, replicas int32, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	podSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: &[]bool{true}[0],
		},
		ImagePullSecrets: imagePullSecrets,
	}
	inpCopy := infraNodePlacement.DeepCopy()
	if inpCopy == nil {
		inpCopy = &sdkapi.NodePlacement{}
	}
	inpCopy.Affinity = AddPodPreferredDuringSchedulingIgnoredDuringExecution(name, inpCopy.Affinity)
	deployment := ResourceBuilder.CreateDeployment(name, "", matchKey, matchValue, serviceAccountName, replicas, podSpec, inpCopy)
	return deployment
}

// CreateOperatorDeployment creates operator deployment
func CreateOperatorDeployment(name, namespace, matchKey, matchValue, serviceAccount string, imagePullSecrets []corev1.LocalObjectReference, numReplicas int32) *appsv1.Deployment {
	podSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: &[]bool{true}[0],
		},
		ImagePullSecrets: imagePullSecrets,
		NodeSelector:     map[string]string{"kubernetes.io/os": "linux"},
		Tolerations: []corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: corev1.TolerationOpExists,
			},
		},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: int32(1),
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "cdi.kubevirt.io",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{name}},
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}
	deployment := ResourceBuilder.CreateOperatorDeployment(name, namespace, matchKey, matchValue, serviceAccount, numReplicas, podSpec)
	labels := util.MergeLabels(deployment.Spec.Template.GetLabels(), map[string]string{common.PrometheusLabelKey: common.PrometheusLabelValue, common.CDIComponentLabel: common.CDIOperatorName})
	deployment.SetLabels(labels)
	deployment.Spec.Template.SetLabels(labels)
	return deployment
}

// AddPodPreferredDuringSchedulingIgnoredDuringExecution to affinity
func AddPodPreferredDuringSchedulingIgnoredDuringExecution(name string, affinity *corev1.Affinity) *corev1.Affinity {
	var affinityCopy *corev1.Affinity
	preferredDuringSchedulingIgnoredDuringExecution := corev1.WeightedPodAffinityTerm{
		Weight: int32(1),
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "cdi.kubevirt.io",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{name}},
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	}

	if affinity != nil && affinity.PodAntiAffinity != nil {
		affinityCopy = affinity.DeepCopy()
		affinityCopy.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinityCopy.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, preferredDuringSchedulingIgnoredDuringExecution)
	} else if affinity != nil {
		affinityCopy = affinity.DeepCopy()
		affinityCopy.PodAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{preferredDuringSchedulingIgnoredDuringExecution},
		}
	} else {
		affinityCopy = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{preferredDuringSchedulingIgnoredDuringExecution},
			},
		}
	}
	return affinityCopy
}
