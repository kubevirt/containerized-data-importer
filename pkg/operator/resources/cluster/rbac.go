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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

func createAggregateClusterRoles(_ *FactoryArgs) []client.Object {
	return []client.Object{
		utils.ResourceBuilder.CreateAggregateClusterRole("cdi.kubevirt.io:admin", "admin", getAdminPolicyRules()),
		utils.ResourceBuilder.CreateAggregateClusterRole("cdi.kubevirt.io:edit", "edit", getEditPolicyRules()),
		utils.ResourceBuilder.CreateAggregateClusterRole("cdi.kubevirt.io:view", "view", getViewPolicyRules()),
		createConfigReaderClusterRole("cdi.kubevirt.io:config-reader"),
		createConfigReaderClusterRoleBinding("cdi.kubevirt.io:config-reader"),
	}
}

func getAdminPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
				"dataimportcrons",
				"datasources",
				"volumeimportsources",
				"volumeuploadsources",
				"volumeclonesources",
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
				"upload.cdi.kubevirt.io",
			},
			Resources: []string{
				"uploadtokenrequests",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"forklift.cdi.kubevirt.io",
			},
			Resources: []string{
				"ovirtvolumepopulators",
				"openstackvolumepopulators",
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
				"cdiconfigs",
				"dataimportcrons",
				"datasources",
				"datavolumes",
				"objecttransfers",
				"storageprofiles",
				"volumeimportsources",
				"volumeuploadsources",
				"volumeclonesources",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"forklift.cdi.kubevirt.io",
			},
			Resources: []string{
				"ovirtvolumepopulators",
				"openstackvolumepopulators",
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
	}
}

func createConfigReaderClusterRole(name string) *rbacv1.ClusterRole {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
				"storageprofiles",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	return utils.ResourceBuilder.CreateClusterRole(name, rules)
}

func createConfigReaderClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: utils.ResourceBuilder.WithCommonLabels(nil),
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
			{
				Kind:     "Group",
				Name:     "system:serviceaccount",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
}
