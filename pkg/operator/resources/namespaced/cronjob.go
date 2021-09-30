/*
Copyright 2021 The CDI Authors.

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cronJobResourceName = "cdi-cronjob"
)

func createCronJobResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createCronJobServiceAccount(),
		createCronJobRoleBinding(),
		createCronJobRole(),
	}
}

func createCronJobServiceAccount() *corev1.ServiceAccount {
	return utils.ResourceBuilder.CreateServiceAccount(cronJobResourceName)
}

func createCronJobRoleBinding() *rbacv1.RoleBinding {
	return utils.ResourceBuilder.CreateRoleBinding(cronJobResourceName, cronJobResourceName, cronJobResourceName, "")
}

func createCronJobRole() *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"dataimportcrons",
			},
			Verbs: []string{
				"get",
				"list",
				"update",
			},
		},
	}
	return utils.ResourceBuilder.CreateRole(cronJobResourceName, rules)
}
