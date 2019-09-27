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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

//CreateClusterRoleBinding create cluster role binding
func CreateClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string) *rbacv1.ClusterRoleBinding {
	return createClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace, utils.WithCommonLabels(nil))
}

//CreateOperatorClusterRoleBinding create cluster role binding for operator
func CreateOperatorClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string) *rbacv1.ClusterRoleBinding {
	return createClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace, utils.WithOperatorLabels(nil))
}

func createClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string, labels map[string]string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleRef,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: serviceAccountNamespace,
			},
		},
	}
}

//CreateClusterRole create cluster role
func CreateClusterRole(name string) *rbacv1.ClusterRole {
	return createClusterRole(name, utils.WithCommonLabels(nil))
}

//CreateOperatorClusterRole create cluster role
func CreateOperatorClusterRole(name string) *rbacv1.ClusterRole {
	return createClusterRole(name, utils.WithOperatorLabels(nil))
}

func createClusterRole(name string, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func createAggregateClusterRoles(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createAggregateClusterRole("cdi.kubevirt.io:admin", "admin", getAdminPolicyRules()),
		createAggregateClusterRole("cdi.kubevirt.io:edit", "edit", getEditPolicyRules()),
		createAggregateClusterRole("cdi.kubevirt.io:view", "view", getViewPolicyRules()),
		createConfigReaderClusterRole("cdi.kubevirt.io:config-reader"),
		createConfigReaderClusterRoleBinding("cdi.kubevirt.io:config-reader"),
	}
}

func createAggregateClusterRole(name, aggregateTo string, policyRules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	labels := map[string]string{
		"rbac.authorization.k8s.io/aggregate-to-" + aggregateTo: "true",
	}
	role := createClusterRole(name, utils.WithCommonLabels(labels))
	role.Rules = policyRules
	return role
}

func getAdminPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"update",
			},
		},
		{
			APIGroups: []string{
				"upload.cdi.kubevirt.io",
			},
			Resources: []string{
				"uploadtokenrequests",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func getEditPolicyRules() []rbacv1.PolicyRule {
	// diff between admin and edit ClusterRoles is minimal and limited to RBAC
	// both can CRUD pods/PVCs/etc
	return getAdminPolicyRules()
}

func getViewPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}

func createConfigReaderClusterRole(name string) *rbacv1.ClusterRole {
	role := CreateClusterRole(name)

	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	return role
}

func createConfigReaderClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: utils.WithCommonLabels(nil),
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:authenticated",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
}
