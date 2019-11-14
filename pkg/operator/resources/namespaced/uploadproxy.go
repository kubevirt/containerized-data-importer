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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	uploadProxyResourceName = "cdi-uploadproxy"
)

func createUploadProxyResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createUploadProxyServiceAccount(),
		createUploadProxyService(),
		createUploadProxyDeployment(args.UploadProxyImage, args.Verbosity, args.PullPolicy),
	}
}

func createUploadProxyService() *corev1.Service {
	service := utils.CreateService(uploadProxyResourceName, cdiLabel, uploadProxyResourceName)
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
	return utils.CreateServiceAccount(uploadProxyResourceName)
}

func createUploadProxyDeployment(image, verbosity, pullPolicy string) *appsv1.Deployment {
	deployment := utils.CreateDeployment(uploadProxyResourceName, cdiLabel, uploadProxyResourceName, uploadProxyResourceName, int32(1))
	container := utils.CreateContainer(uploadProxyResourceName, image, verbosity, corev1.PullPolicy(pullPolicy))
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
		{
			Name: "UPLOAD_SERVER_CLIENT_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-upload-server-client-key",
					},
					Key: "tls.key",
				},
			},
		},
		{
			Name: "UPLOAD_SERVER_CLIENT_CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-upload-server-client-key",
					},
					Key: "tls.crt",
				},
			},
		},
		{
			Name: "UPLOAD_SERVER_CA_CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-upload-server-client-key",
					},
					Key: "ca.crt",
				},
			},
		},
		{
			Name: "SERVICE_TLS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-upload-proxy-server-key",
					},
					Key: "tls.key",
				},
			},
		},
		{
			Name: "SERVICE_TLS_CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-upload-proxy-server-key",
					},
					Key: "tls.crt",
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
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}

	return deployment
}
