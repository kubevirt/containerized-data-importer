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
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

//NewClusterServiceVersionData - Data arguments used to create CDI's CSV manifest
type NewClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	Namespace          string
	ImagePullPolicy    string
	IconBase64         string
	Verbosity          string

	OperatorVersion string

	CdiImageNames *CdiImages
}

//CdiImages - images to be provied to cdi operator
type CdiImages struct {
	ControllerImage   string
	ImporterImage     string
	ClonerImage       string
	APIServerImage    string
	UplodaProxyImage  string
	UplodaServerImage string
	OperatorImage     string
}

//NewCdiOperatorDeployment - provides operator deployment spec
func NewCdiOperatorDeployment(operatorVersion string, namespace string, imagePullPolicy string, verbosity string, cdiImages *CdiImages) (*appsv1.Deployment, error) {
	deployment := createOperatorDeployment(
		operatorVersion,
		namespace,
		"true",
		cdiImages.OperatorImage,
		cdiImages.ControllerImage,
		cdiImages.ImporterImage,
		cdiImages.ClonerImage,
		cdiImages.APIServerImage,
		cdiImages.UplodaProxyImage,
		cdiImages.UplodaServerImage,
		verbosity,
		imagePullPolicy)

	return deployment, nil
}

//NewCdiOperatorClusterRole - provides operator clusterRole
func NewCdiOperatorClusterRole() *rbacv1.ClusterRole {
	return createOperatorClusterRole(operatorClusterRoleName)
}

//NewCdiCrd - provides CDI CRD
func NewCdiCrd() *extv1beta1.CustomResourceDefinition {
	return createCDIListCRD()
}

//NewClusterServiceVersion - generates CSV for CDI
func NewClusterServiceVersion(data *NewClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {
	return createClusterServiceVersion(data)
}
