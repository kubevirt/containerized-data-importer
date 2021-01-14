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

package controller

import (
	cdicluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
)

// Status provides CDI status sub-resource
func (r *ReconcileCDI) Status(cr runtime.Object) *sdkapi.Status {
	return &cr.(*cdiv1.CDI).Status.Status
}

// Create creates new CDI resource
func (r *ReconcileCDI) Create() controllerutil.Object {
	return &cdiv1.CDI{}
}

// GetDependantResourcesListObjects provides slice of List resources corresponding to CDI-dependant resource types
func (r *ReconcileCDI) GetDependantResourcesListObjects() []runtime.Object {
	return []runtime.Object{
		&extv1.CustomResourceDefinitionList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&rbacv1.RoleBindingList{},
		&rbacv1.RoleList{},
		&corev1.ServiceAccountList{},
		&apiregistrationv1beta1.APIServiceList{},
		&admissionregistrationv1beta1.ValidatingWebhookConfigurationList{},
		&admissionregistrationv1beta1.MutatingWebhookConfigurationList{},
	}
}

// IsCreating checks whether operator config is missing (which means it is create-type reconciliation)
func (r *ReconcileCDI) IsCreating(_ controllerutil.Object) (bool, error) {
	configMap, err := r.getConfigMap()
	if err != nil {
		return true, nil
	}
	return configMap == nil, nil
}

func (r *ReconcileCDI) getNamespacedArgs(cr *cdiv1.CDI) *cdinamespaced.FactoryArgs {
	result := *r.namespacedArgs

	if cr != nil {
		if cr.Spec.ImagePullPolicy != "" {
			result.PullPolicy = string(cr.Spec.ImagePullPolicy)
		}
		result.InfraNodePlacement = &cr.Spec.Infra
	}

	return &result
}

// GetAllResources provides slice of resources CDI depends on
func (r *ReconcileCDI) GetAllResources(crObject runtime.Object) ([]runtime.Object, error) {
	cr := crObject.(*cdiv1.CDI)
	var resources []runtime.Object

	if sdk.DeployClusterResources() {
		crs, err := cdicluster.CreateAllStaticResources(r.clusterArgs)
		if err != nil {
			sdk.MarkCrFailedHealing(cr, r.Status(cr), "CreateResources", "Unable to create all resources", r.recorder)
			return nil, err
		}

		resources = append(resources, crs...)
	}

	nsrs, err := cdinamespaced.CreateAllResources(r.getNamespacedArgs(cr))
	if err != nil {
		sdk.MarkCrFailedHealing(cr, r.Status(cr), "CreateNamespaceResources", "Unable to create all namespaced resources", r.recorder)
		return nil, err
	}

	resources = append(resources, nsrs...)

	drs, err := cdicluster.CreateAllDynamicResources(r.clusterArgs)
	if err != nil {
		sdk.MarkCrFailedHealing(cr, r.Status(cr), "CreateDynamicResources", "Unable to create all dynamic resources", r.recorder)
		return nil, err
	}

	resources = append(resources, drs...)

	certs := r.getCertificateDefinitions(cr)
	for _, cert := range certs {
		if cert.SignerSecret != nil {
			resources = append(resources, cert.SignerSecret)
		}

		if cert.CertBundleConfigmap != nil {
			resources = append(resources, cert.CertBundleConfigmap)
		}

		if cert.TargetSecret != nil {
			resources = append(resources, cert.TargetSecret)
		}
	}

	return resources, nil
}
