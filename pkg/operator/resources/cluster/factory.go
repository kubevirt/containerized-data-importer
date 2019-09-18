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

	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

// FactoryArgs contains the required parameters to generate all cluster-scoped resources
type FactoryArgs struct {
	Namespace string
}

type factoryFunc func(*FactoryArgs) []runtime.Object

const (
	//CdiRBAC - groupCode to generate only operator rbac manifest
	CdiRBAC = "cdi-rbac"
	//APIServerRBAC - groupCode to generate only apiserver rbac manifest
	APIServerRBAC = "apiserver-rbac"
	//UploadProxyRBAC - groupCode to generate only apiserver rbac manifest
	UploadProxyRBAC = "uploadproxy-rbac"
	//ControllerRBAC - groupCode to generate only controller rbac manifest
	ControllerRBAC = "controller-rbac"
	//CRDResources - groupCode to generate only resources' manifest
	CRDResources = "crd-resources"
	//AggregateRoles = groupCode to generate only aggregate roles
	AggregateRoles = "aggregate-roles"
)

var factoryFunctions = map[string]factoryFunc{
	CdiRBAC:         createCdiRBAC,
	APIServerRBAC:   createAPIServerResources,
	ControllerRBAC:  createControllerResources,
	CRDResources:    createCRDResources,
	UploadProxyRBAC: createUploadProxyResources,
	AggregateRoles:  createAggregateClusterRoles,
}

//IsFactoryResource returns true id codeGroupo belolngs to factory functions
func IsFactoryResource(codeGroup string) bool {
	for k := range factoryFunctions {
		if codeGroup == k {
			return true
		}
	}
	return false
}

func createCRDResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createDataVolumeCRD(),
		createCDIConfigCRD(),
	}
}

func createCdiRBAC(args *FactoryArgs) []runtime.Object {
	return append(
		createAPIServerResources(args),
		createControllerResources(args)...)
}

// CreateAllResources creates all cluster-wide resources
func CreateAllResources(args *FactoryArgs) ([]runtime.Object, error) {
	var resources []runtime.Object
	for group := range factoryFunctions {
		rs, err := CreateResourceGroup(group, args)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rs...)
	}
	return resources, nil
}

// CreateResourceGroup creates all cluster resources fr a specific group/component
func CreateResourceGroup(group string, args *FactoryArgs) ([]runtime.Object, error) {
	f, ok := factoryFunctions[group]
	if !ok {
		return nil, fmt.Errorf("group %s does not exist", group)
	}
	resources := f(args)
	utils.ValidateGVKs(resources)
	return resources, nil
}
