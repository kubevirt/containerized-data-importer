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
	"fmt"

	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	utils "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
)

// FactoryArgs contains the required parameters to generate all cluster-scoped resources
type FactoryArgs struct {
	NamespacedArgs namespaced.FactoryArgs
	Image          string
}

type factoryFunc func(*FactoryArgs) []runtime.Object

func aggregateFactoryFunc(funcs ...factoryFunc) factoryFunc {
	return func(args *FactoryArgs) []runtime.Object {
		var result []runtime.Object
		for _, f := range funcs {
			result = append(result, f(args)...)
		}
		return result
	}
}

var operatorFactoryFunctions = map[string]factoryFunc{
	"operator-cluster-rbac": createClusterRBAC,
	"operator-rbac":         createNamespacedRBAC,
	"operator-deployment":   createDeployment,
	"operator-crd":          createCRD,
	"everything":            aggregateFactoryFunc(createCRD, createClusterRBAC, createNamespacedRBAC, createDeployment),
}

// ClusterServiceVersionData - Data arguments used to create CDI's CSV manifest
type ClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	Namespace          string
	ImagePullPolicy    string
	IconBase64         string
	Verbosity          string

	OperatorVersion string

	ControllerImage   string
	ImporterImage     string
	ClonerImage       string
	APIServerImage    string
	UplodaProxyImage  string
	UplodaServerImage string
	OperatorImage     string
}

// CreateAllOperatorResources creates all cluster-wide resources
func CreateAllOperatorResources(args *FactoryArgs) ([]runtime.Object, error) {
	var resources []runtime.Object
	for group := range operatorFactoryFunctions {
		rs, err := CreateOperatorResourceGroup(group, args)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rs...)
	}
	return resources, nil
}

// CreateOperatorResourceGroup creates all cluster resources fr a specific group/component
func CreateOperatorResourceGroup(group string, args *FactoryArgs) ([]runtime.Object, error) {
	f, ok := operatorFactoryFunctions[group]
	if !ok {
		return nil, fmt.Errorf("group %s does not exist", group)
	}
	resources := f(args)
	utils.ValidateGVKs(resources)
	return resources, nil
}

// NewCdiCrd - provides CDI CRD
func NewCdiCrd() *extv1.CustomResourceDefinition {
	return createCDIListCRD()
}

// NewClusterServiceVersion - generates CSV for CDI
func NewClusterServiceVersion(data *ClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {
	return createClusterServiceVersion(data)
}
