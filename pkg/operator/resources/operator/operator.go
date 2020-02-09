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

	"github.com/blang/semver"
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/lib/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
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
				"rolebindings",
				"roles",
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
				"get",
				"list",
				"watch",
				"create",
				"update",
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
				"",
			},
			Resources: []string{
				"serviceaccounts",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"authorization.k8s.io",
			},
			Resources: []string{
				"subjectaccessreviews",
			},
			Verbs: []string{
				"create",
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
				"get",
				"list",
				"watch",
				"create",
				"update",
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
				"get",
				"list",
				"watch",
				"create",
				"update",
			},
		},
	}
	rules = append(rules, cluster.GetClusterRolePolicyRules()...)
	return rules
}

func createClusterRole() *rbacv1.ClusterRole {
	clusterRole := cluster.CreateOperatorClusterRole(clusterRoleName)
	clusterRole.Rules = getClusterPolicyRules()
	return clusterRole
}

func createClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return cluster.CreateOperatorClusterRoleBinding(serviceAccountName, clusterRoleName, serviceAccountName, namespace)
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
	return utils.CreateOperatorServiceAccount(serviceAccountName, namespace)
}

func createNamespacedRole(namespace string) *rbacv1.Role {
	role := utils.CreateRole(roleName)
	role.Namespace = namespace
	role.Rules = getNamespacedPolicyRules()
	return role
}

func createNamespacedRoleBinding(namespace string) *rbacv1.RoleBinding {
	roleBinding := utils.CreateRoleBinding(serviceAccountName, roleName, serviceAccountName, namespace)
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

func createCDIListCRD() *extv1beta1.CustomResourceDefinition {
	return &extv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdis.cdi.kubevirt.io",
			Labels: map[string]string{
				"operator.cdi.kubevirt.io": "",
			},
		},
		Spec: extv1beta1.CustomResourceDefinitionSpec{
			Group:   "cdi.kubevirt.io",
			Version: "v1alpha1",
			Scope:   "Cluster",

			Versions: []extv1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
				},
			},
			Names: extv1beta1.CustomResourceDefinitionNames{
				Kind:     "CDI",
				ListKind: "CDIList",
				Plural:   "cdis",
				Singular: "cdi",
				Categories: []string{
					"all",
				},
				ShortNames: []string{"cdi", "cdis"},
			},

			Validation: &extv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &extv1beta1.JSONSchemaProps{
					Properties: map[string]extv1beta1.JSONSchemaProps{
						"apiVersion": {
							Type: "string",
						},
						"kind": {
							Type: "string",
						},
						"metadata": {
							Type: "object",
						},

						"spec": {
							Properties: map[string]extv1beta1.JSONSchemaProps{
								"imagePullPolicy": {
									Type: "string",
									Enum: []extv1beta1.JSON{
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
								"uninstallStrategy": {
									Type: "string",
									Enum: []extv1beta1.JSON{
										{
											Raw: []byte(`"BlockUninstallIfWorkloadsExist"`),
										},
										{
											Raw: []byte(`"RemoveWorkloads"`),
										},
									},
								},
							},
							Type: "object",
						},
					},
				},
			},

			AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
				{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
				{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
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
	container := utils.CreatePortsContainer("cdi-operator", operatorImage, verbosity, corev1.PullPolicy(pullPolicy), createPrometheusPorts())
	container.Env = createOperatorEnvVar(operatorVersion, deployClusterResources, operatorImage, controllerImage, importerImage, clonerImage, apiServerImage, uploadProxyImage, uploadServerImage, verbosity, pullPolicy)
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	return deployment
}

func createPrometheusPorts() *[]corev1.ContainerPort {
	return &[]corev1.ContainerPort{
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

	csvVersion, err := semver.New(data.CsvVersion)
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
          "apiVersion":"cdi.kubevirt.io/v1alpha1",
          "kind":"CDI",
          "metadata": {
            "name":"cdi",
            "namespace":"cdi"
          },
          "spec": {
            "imagePullPolicy":"IfNotPresent"
            "uninstallStrategy":"RemoveWorkloads"
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
			Version:     version.OperatorVersion{Version: *csvVersion},
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
						Version:     "v1alpha1",
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
