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

	secv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

func createControllerResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createControllerServiceAccount(),
		createControllerRoleBinding(),
		createControllerRole(),
		createControllerDeployment(args.ControllerImage,
			args.ImporterImage,
			args.ClonerImage,
			args.OvirtPopulatorImage,
			args.UploadServerImage,
			args.Verbosity,
			args.PullPolicy,
			args.ImagePullSecrets,
			args.PriorityClassName,
			args.InfraNodePlacement,
			args.ControllerReplicas),
		createPrometheusService(),
	}
}

func createControllerRoleBinding() *rbacv1.RoleBinding {
	return utils.ResourceBuilder.CreateRoleBinding(common.CDIControllerResourceName, common.CDIControllerResourceName, common.ControllerServiceAccountName, "")
}

func getControllerNamespacedRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"create",
				"update",
				"delete",
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
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"cronjobs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"create",
				"update",
				"deletecollection",
			},
		},

		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
				"deletecollection",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"coordination.k8s.io",
			},
			Resources: []string{
				"leases",
			},
			Verbs: []string{
				"get",
				"create",
				"update",
			},
		},
		{
			APIGroups: []string{
				"networking.k8s.io",
			},
			Resources: []string{
				"ingresses",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"route.openshift.io",
			},
			Resources: []string{
				"routes",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}

func createControllerRole() *rbacv1.Role {
	return utils.ResourceBuilder.CreateRole(common.CDIControllerResourceName, getControllerNamespacedRules())
}

func createControllerServiceAccount() *corev1.ServiceAccount {
	return utils.ResourceBuilder.CreateServiceAccount(common.ControllerServiceAccountName)
}

func createControllerDeployment(controllerImage, importerImage, clonerImage, ovirtPopulatorImage, uploadServerImage, verbosity, pullPolicy string, imagePullSecrets []corev1.LocalObjectReference, priorityClassName string, infraNodePlacement *sdkapi.NodePlacement, replicas int32) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	// The match selector is immutable. that's why we should always use the same labels.
	deployment := utils.CreateDeployment(common.CDIControllerResourceName, common.CDIComponentLabel, common.CDIControllerResourceName, common.ControllerServiceAccountName, imagePullSecrets, int32(1), infraNodePlacement)
	deployment.ObjectMeta.Labels[common.CDIComponentLabel] = common.CDIControllerResourceName
	if priorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = priorityClassName
	}
	if replicas > 1 {
		deployment.Spec.Replicas = &replicas
	}
	container := utils.CreateContainer(common.CDIControllerResourceName, controllerImage, verbosity, pullPolicy)
	container.Ports = []corev1.ContainerPort{
		{
			Name:          "metrics",
			ContainerPort: 8443,
			Protocol:      "TCP",
		},
	}
	labels := util.MergeLabels(deployment.Spec.Template.GetLabels(), map[string]string{common.PrometheusLabelKey: common.PrometheusLabelValue})
	//Add label for pod affinity
	deployment.SetLabels(labels)
	deployment.Spec.Template.SetLabels(labels)
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[secv1.RequiredSCCAnnotation] = common.RestrictedSCCName
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
			Value: common.CDIUploadProxyResourceName,
		},
		{
			Name:  "OVIRT_POPULATOR_IMAGE",
			Value: ovirtPopulatorImage,
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
		ProbeHandler: corev1.ProbeHandler{
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
			corev1.ResourceCPU:    resource.MustParse("100m"),
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

func createPrometheusService() *corev1.Service {
	service := utils.ResourceBuilder.CreateService(common.PrometheusServiceName, common.PrometheusLabelKey, common.PrometheusLabelValue, nil)
	service.Spec.Ports = []corev1.ServicePort{
		{
			Name: "metrics",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "metrics",
			},
			Protocol: corev1.ProtocolTCP,
		},
	}
	return service
}
