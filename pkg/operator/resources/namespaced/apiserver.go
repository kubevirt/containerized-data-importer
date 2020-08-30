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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	apiServerRessouceName = "cdi-apiserver"
)

const (
	cdiLabel = common.CDIComponentLabel
)

func createAPIServerResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createAPIServerServiceAccount(),
		createAPIServerRoleBinding(),
		createAPIServerRole(),
		createAPIServerService(),
		createAPIServerDeployment(args.APIServerImage, args.Verbosity, args.PullPolicy, args.InfraNodePlacement),
	}
}

func createAPIServerServiceAccount() *corev1.ServiceAccount {
	return utils.CreateServiceAccount(apiServerRessouceName)
}

func createAPIServerRoleBinding() *rbacv1.RoleBinding {
	return utils.CreateRoleBinding(apiServerRessouceName, apiServerRessouceName, apiServerRessouceName, "")
}

func createAPIServerRole() *rbacv1.Role {
	role := utils.CreateRole(apiServerRessouceName)
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"*",
			},
		},
	}
	return role
}

func createAPIServerService() *corev1.Service {
	service := utils.CreateService("cdi-api", cdiLabel, apiServerRessouceName)
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

func createAPIServerDeployment(image, verbosity, pullPolicy string, infraNodePlacement *cdiv1.NodePlacement) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	deployment := utils.CreateDeployment(apiServerRessouceName, cdiLabel, apiServerRessouceName, apiServerRessouceName, 1, infraNodePlacement)
	container := utils.CreateContainer(apiServerRessouceName, image, verbosity, corev1.PullPolicy(pullPolicy))
	container.ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
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
