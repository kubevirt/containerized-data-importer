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
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	uploadProxyResourceName = "cdi-uploadproxy"
)

func createUploadProxyResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createUploadProxyServiceAccount(),
		createUploadProxyService(),
		createUploadProxyRoleBinding(),
		createUploadProxyRole(),
		createUploadProxyDeployment(args.UploadProxyImage, args.Verbosity, args.PullPolicy, args.InfraNodePlacement),
	}
}

func createUploadProxyService() *corev1.Service {
	service := utils.ResourcesBuiler.CreateService(uploadProxyResourceName, cdiLabel, uploadProxyResourceName, nil)
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

func createUploadProxyServiceAccount() *corev1.ServiceAccount {
	return utils.ResourcesBuiler.CreateServiceAccount(uploadProxyResourceName)
}

func createUploadProxyRoleBinding() *rbacv1.RoleBinding {
	return utils.ResourcesBuiler.CreateRoleBinding(uploadProxyResourceName, uploadProxyResourceName, uploadProxyResourceName, "")
}

func createUploadProxyRole() *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"get",
			},
		},
	}
	return utils.ResourcesBuiler.CreateRole(uploadProxyResourceName, rules)
}

func createUploadProxyDeployment(image, verbosity, pullPolicy string, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	deployment := utils.CreateDeployment(uploadProxyResourceName, cdiLabel, uploadProxyResourceName, uploadProxyResourceName, int32(1), infraNodePlacement)
	container := utils.CreateContainer(uploadProxyResourceName, image, verbosity, pullPolicy)
	container.Env = []corev1.EnvVar{
		{
			Name: "APISERVER_PUBLIC_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-api-signing-key",
					},
					Key: "id_rsa.pub",
				},
			},
		},
	}
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
			Name:      "server-cert",
			MountPath: "/var/run/certs/cdi-uploadproxy-server-cert",
			ReadOnly:  true,
		},
		{
			Name:      "client-cert",
			MountPath: "/var/run/certs/cdi-uploadserver-client-cert",
			ReadOnly:  true,
		},
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "server-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-uploadproxy-server-cert",
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
		{
			Name: "client-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-uploadserver-client-cert",
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
