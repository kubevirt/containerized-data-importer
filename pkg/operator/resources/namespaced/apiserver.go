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

package namespaced

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/common"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

func createAPIServerResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createAPIServerServiceAccount(),
		createAPIServerRoleBinding(),
		createAPIServerRole(),
		createAPIServerService(),
		createAPIServerDeployment(args.APIServerImage, args.Verbosity, args.PullPolicy, args.ImagePullSecrets, args.PriorityClassName, args.InfraNodePlacement, args.APIServerReplicas),
	}
}

func createAPIServerServiceAccount() *corev1.ServiceAccount {
	return utils.ResourceBuilder.CreateServiceAccount(common.CDIApiServerResourceName)
}

func createAPIServerRoleBinding() *rbacv1.RoleBinding {
	return utils.ResourceBuilder.CreateRoleBinding(common.CDIApiServerResourceName, common.CDIApiServerResourceName, common.CDIApiServerResourceName, "")
}

func getAPIServerNamespacedRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"create",
			},
		},
	}
}

func createAPIServerRole() *rbacv1.Role {
	return utils.ResourceBuilder.CreateRole(common.CDIApiServerResourceName, getAPIServerNamespacedRules())
}

func createAPIServerService() *corev1.Service {
	service := utils.ResourceBuilder.CreateService("cdi-api", common.CDIComponentLabel, common.CDIApiServerResourceName, nil)
	service.Spec.Ports = []corev1.ServicePort{
		{
			Port: 443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
			Protocol: corev1.ProtocolTCP,
		},
	}
	return service
}

func createAPIServerDeployment(image, verbosity, pullPolicy string, imagePullSecrets []corev1.LocalObjectReference, priorityClassName string, infraNodePlacement *sdkapi.NodePlacement, replicas int32) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	deployment := utils.CreateDeployment(common.CDIApiServerResourceName, common.CDIComponentLabel, common.CDIApiServerResourceName, common.CDIApiServerResourceName, imagePullSecrets, 1, infraNodePlacement)
	if priorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = priorityClassName
	}
	if replicas > 1 {
		deployment.Spec.Replicas = &replicas
	}
	container := utils.CreatePortsContainer(common.CDIApiServerResourceName, image, pullPolicy, createAPIServerPorts(common.CDIApiServerResourceName))
	container.Args = []string{"-v=" + verbosity}
	container.Env = []corev1.EnvVar{
		{
			Name: common.InstallerPartOfLabel,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  fmt.Sprintf("metadata.labels['%s']", common.AppKubernetesPartOfLabel),
				},
			},
		},
		{
			Name: common.InstallerVersionLabel,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  fmt.Sprintf("metadata.labels['%s']", common.AppKubernetesVersionLabel),
				},
			},
		},
	}
	container.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 8443,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 2,
		PeriodSeconds:       5,
		FailureThreshold:    3,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "ca-bundle",
			MountPath: "/var/run/certs/cdi-apiserver-signer-bundle",
			ReadOnly:  true,
		},
		{
			Name:      "server-cert",
			MountPath: "/var/run/certs/cdi-apiserver-server-cert",
			ReadOnly:  true,
		},
	}
	container.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("150Mi"),
		},
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "ca-bundle",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-apiserver-signer-bundle",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "ca-bundle.crt",
							Path: "ca-bundle.crt",
						},
					},
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: "server-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-apiserver-server-cert",
					Items: []corev1.KeyToPath{
						{
							Key:  "tls.crt",
							Path: "tls.crt",
						},
						{
							Key:  "tls.key",
							Path: "tls.key",
						},
					},
					DefaultMode: &defaultMode,
				},
			},
		},
	}
	return deployment
}

func createAPIServerPorts(name string) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          name,
			ContainerPort: 8443,
			Protocol:      "TCP",
		},
	}
}
