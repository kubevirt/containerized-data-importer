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

package cluster

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	apiServerResourceName = "cdi-apiserver"
)

func createAPIServerResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createAPIServerClusterRole(),
		createAPIServerClusterRoleBinding(args.Namespace),
	}
}

func getAPIServerClusterPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
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
				"create",
				"update",
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
				"",
			},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"persistentvolumeclaims",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func createAPIServerClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return CreateClusterRoleBinding(apiServerResourceName, apiServerResourceName, apiServerResourceName, namespace)
}

func createAPIServerClusterRole() *rbacv1.ClusterRole {
	clusterRole := CreateClusterRole(apiServerResourceName)
	clusterRole.Rules = getAPIServerClusterPolicyRules()
	return clusterRole
}
