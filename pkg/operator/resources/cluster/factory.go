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
	"fmt"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	utils "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
)

// FactoryArgs contains the required parameters to generate all cluster-scoped resources
type FactoryArgs struct {
	Namespace string
	Client    client.Client
	Logger    logr.Logger
}

type factoryFunc func(*FactoryArgs) []client.Object

type factoryFuncMap map[string]factoryFunc

var staticFactoryFunctions = factoryFuncMap{
	"apiserver-rbac":   createStaticAPIServerResources,
	"controller-rbac":  createControllerResources,
	"crd-resources":    createCRDResources,
	"uploadproxy-rbac": createUploadProxyResources,
	"aggregate-roles":  createAggregateClusterRoles,
}

var dynamicFactoryFunctions = factoryFuncMap{
	"apiserver-registrations": createDynamicAPIServerResources,
}

func createCRDResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createDataVolumeCRD(),
		createCDIConfigCRD(),
		createStorageProfileCRD(),
		createDataSourceCRD(),
		createDataImportCronCRD(),
		createObjectTransferCRD(),
	}
}

// CreateAllStaticResources creates all static cluster-wide resources
func CreateAllStaticResources(args *FactoryArgs) ([]client.Object, error) {
	return createAllResources(staticFactoryFunctions, args)
}

// CreateStaticResourceGroup creates all static cluster resources for a specific group/component
func CreateStaticResourceGroup(group string, args *FactoryArgs) ([]client.Object, error) {
	return createResourceGroup(staticFactoryFunctions, group, args)
}

// CreateAllDynamicResources creates all dynamic cluster-wide resources
func CreateAllDynamicResources(args *FactoryArgs) ([]client.Object, error) {
	return createAllResources(dynamicFactoryFunctions, args)
}

// CreateDynamicResourceGroup creates all dynamic cluster resources for a specific group/component
func CreateDynamicResourceGroup(group string, args *FactoryArgs) ([]client.Object, error) {
	return createResourceGroup(dynamicFactoryFunctions, group, args)
}

func createAllResources(funcMap factoryFuncMap, args *FactoryArgs) ([]client.Object, error) {
	var resources []client.Object
	for group := range funcMap {
		rs, err := createResourceGroup(funcMap, group, args)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rs...)
	}
	return resources, nil
}

func createResourceGroup(funcMap factoryFuncMap, group string, args *FactoryArgs) ([]client.Object, error) {
	f, ok := funcMap[group]
	if !ok {
		return nil, fmt.Errorf("group %s does not exist", group)
	}
	resources := f(args)
	for _, r := range resources {
		utils.ValidateGVKs([]runtime.Object{r})
	}
	return resources, nil
}

// GetClusterRolePolicyRules returns all cluster PolicyRules
func GetClusterRolePolicyRules() []rbacv1.PolicyRule {
	result := getAPIServerClusterPolicyRules()
	result = append(result, getControllerClusterPolicyRules()...)
	result = append(result, getUploadProxyClusterPolicyRules()...)
	return result
}
