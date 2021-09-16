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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	controllerResourceName = "cdi-deployment"
	prometheusLabel        = common.PrometheusLabel
)

func createControllerResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createControllerServiceAccount(),
		createControllerRoleBinding(),
		createControllerRole(),
		createControllerDeployment(args.ControllerImage,
			args.ImporterImage,
			args.ClonerImage,
			args.UploadServerImage,
			args.Verbosity,
			args.PullPolicy,
			args.PriorityClassName,
			args.InfraNodePlacement),
		createInsecureRegConfigMap(),
	}
}

func createControllerRoleBinding() *rbacv1.RoleBinding {
	return utils.ResourceBuilder.CreateRoleBinding(controllerResourceName, controllerResourceName, common.ControllerServiceAccountName, "")
}

func createControllerRole() *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"configmaps",
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
				"secrets",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
	return utils.ResourceBuilder.CreateRole(controllerResourceName, rules)
}

func createControllerServiceAccount() *corev1.ServiceAccount {
	return utils.ResourceBuilder.CreateServiceAccount(common.ControllerServiceAccountName)
}

func createControllerDeployment(controllerImage, importerImage, clonerImage, uploadServerImage, verbosity, pullPolicy, priorityClassName string, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	deployment := utils.CreateDeployment(controllerResourceName, "app", "containerized-data-importer", common.ControllerServiceAccountName, int32(1), infraNodePlacement)
	if priorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = priorityClassName
	}
	container := utils.CreateContainer("cdi-controller", controllerImage, verbosity, pullPolicy)
	container.Env = []corev1.EnvVar{
		{
			Name:  "IMPORTER_IMAGE",
			Value: importerImage,
		},
		{
			Name:  "CLONER_IMAGE",
			Value: clonerImage,
		},
		{
			Name:  "UPLOADSERVER_IMAGE",
			Value: uploadServerImage,
		},
		{
			Name:  "UPLOADPROXY_SERVICE",
			Value: uploadProxyResourceName,
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
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
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"cat", "/tmp/ready"},
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
			Name:      "cdi-api-signing-key",
			MountPath: controller.TokenKeyDir,
		},
		{
			Name:      "uploadserver-ca-cert",
			MountPath: "/var/run/certs/cdi-uploadserver-signer",
		},
		{
			Name:      "uploadserver-client-ca-cert",
			MountPath: "/var/run/certs/cdi-uploadserver-client-signer",
		},
		{
			Name:      "uploadserver-ca-bundle",
			MountPath: "/var/run/ca-bundle/cdi-uploadserver-signer-bundle",
		},
		{
			Name:      "uploadserver-client-ca-bundle",
			MountPath: "/var/run/ca-bundle/cdi-uploadserver-client-signer-bundle",
		},
	}
	container.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("150Mi"),
		},
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "cdi-api-signing-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-api-signing-key",
					Items: []corev1.KeyToPath{
						{
							Key:  "id_rsa.pub",
							Path: "id_rsa.pub",
						},
						{
							Key:  "id_rsa",
							Path: "id_rsa",
						},
					},
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: "uploadserver-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-uploadserver-signer",
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
			Name: "uploadserver-client-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-uploadserver-client-signer",
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
			Name: "uploadserver-ca-bundle",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-uploadserver-signer-bundle",
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
			Name: "uploadserver-client-ca-bundle",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "cdi-uploadserver-client-signer-bundle",
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
	}
	return deployment
}

func createInsecureRegConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   common.InsecureRegistryConfigMap,
			Labels: utils.ResourceBuilder.WithCommonLabels(nil),
		},
	}
}
