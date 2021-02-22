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
	"context"
	"fmt"

	"github.com/go-logr/logr"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdicorev1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdicorev1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	apiServerResourceName = "cdi-apiserver"
	apiServerServiceName  = "cdi-api"
)

func createStaticAPIServerResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createAPIServerClusterRole(),
		createAPIServerClusterRoleBinding(args.Namespace),
	}
}

func createDynamicAPIServerResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createAPIService("v1beta1", args.Namespace, args.Client, args.Logger),
		createAPIService("v1alpha1", args.Namespace, args.Client, args.Logger),
		createDataVolumeValidatingWebhook(args.Namespace, args.Client, args.Logger),
		createDataVolumeMutatingWebhook(args.Namespace, args.Client, args.Logger),
		createCDIValidatingWebhook(args.Namespace, args.Client, args.Logger),
		createObjectTransferValidatingWebhook(args.Namespace, args.Client, args.Logger),
	}
}

func getAPIServerClusterPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
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
				"datavolumes",
			},
			Verbs: []string{
				"list",
				"get",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdis",
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
				"cdis/finalizers",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func createAPIService(version, namespace string, c client.Client, l logr.Logger) *apiregistrationv1beta1.APIService {
	apiService := &apiregistrationv1beta1.APIService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiregistration.k8s.io/v1beta1",
			Kind:       "APIService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", version, cdiuploadv1.SchemeGroupVersion.Group),
			Labels: map[string]string{
				utils.CDILabel: apiServerServiceName,
			},
		},
		Spec: apiregistrationv1beta1.APIServiceSpec{
			Service: &apiregistrationv1beta1.ServiceReference{
				Namespace: namespace,
				Name:      apiServerServiceName,
			},
			Group:                cdiuploadv1.SchemeGroupVersion.Group,
			Version:              version,
			GroupPriorityMinimum: 1000,
			VersionPriority:      15,
		},
	}

	if c == nil {
		return apiService
	}

	bundle := getAPIServerCABundle(namespace, c, l)
	if bundle != nil {
		apiService.Spec.CABundle = bundle
	}

	return apiService
}

func createDataVolumeValidatingWebhook(namespace string, c client.Client, l logr.Logger) *admissionregistrationv1beta1.ValidatingWebhookConfiguration {
	path := "/datavolume-validate"
	defaultServicePort := int32(443)
	allScopes := admissionregistrationv1beta1.AllScopes
	exactPolicy := admissionregistrationv1beta1.Exact
	failurePolicy := admissionregistrationv1beta1.Fail
	defaultTimeoutSeconds := int32(30)
	sideEffect := admissionregistrationv1beta1.SideEffectClassNone
	whc := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdi-api-datavolume-validate",
			Labels: map[string]string{
				utils.CDILabel: apiServerServiceName,
			},
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "datavolume-validate.cdi.kubevirt.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{
						admissionregistrationv1beta1.Create,
						admissionregistrationv1beta1.Update,
					},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups: []string{cdicorev1.SchemeGroupVersion.Group},
						APIVersions: []string{
							cdicorev1.SchemeGroupVersion.Version,
							cdicorev1alpha1.SchemeGroupVersion.Version,
						},
						Resources: []string{"datavolumes"},
						Scope:     &allScopes,
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      apiServerServiceName,
						Path:      &path,
						Port:      &defaultServicePort,
					},
				},
				FailurePolicy:     &failurePolicy,
				SideEffects:       &sideEffect,
				MatchPolicy:       &exactPolicy,
				NamespaceSelector: &metav1.LabelSelector{},
				TimeoutSeconds:    &defaultTimeoutSeconds,
				AdmissionReviewVersions: []string{
					"v1beta1",
				},
				ObjectSelector: &metav1.LabelSelector{},
			},
		},
	}

	if c == nil {
		return whc
	}

	bundle := getAPIServerCABundle(namespace, c, l)
	if bundle != nil {
		whc.Webhooks[0].ClientConfig.CABundle = bundle
	}

	return whc
}

func createCDIValidatingWebhook(namespace string, c client.Client, l logr.Logger) *admissionregistrationv1beta1.ValidatingWebhookConfiguration {
	path := "/cdi-validate"
	sideEffect := admissionregistrationv1beta1.SideEffectClassNone
	defaultServicePort := int32(443)
	allScopes := admissionregistrationv1beta1.AllScopes
	exactPolicy := admissionregistrationv1beta1.Exact
	failurePolicy := admissionregistrationv1beta1.Fail
	defaultTimeoutSeconds := int32(30)
	whc := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdi-api-validate",
			Labels: map[string]string{
				utils.CDILabel: apiServerServiceName,
			},
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "cdi-validate.cdi.kubevirt.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{
						admissionregistrationv1beta1.Delete,
					},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups: []string{cdicorev1.SchemeGroupVersion.Group},
						APIVersions: []string{
							cdicorev1.SchemeGroupVersion.Version,
							cdicorev1alpha1.SchemeGroupVersion.Version,
						},
						Resources: []string{"cdis"},
						Scope:     &allScopes,
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      apiServerServiceName,
						Path:      &path,
						Port:      &defaultServicePort,
					},
				},
				SideEffects:       &sideEffect,
				FailurePolicy:     &failurePolicy,
				MatchPolicy:       &exactPolicy,
				NamespaceSelector: &metav1.LabelSelector{},
				TimeoutSeconds:    &defaultTimeoutSeconds,
				AdmissionReviewVersions: []string{
					"v1beta1",
				},
				ObjectSelector: &metav1.LabelSelector{},
			},
		},
	}

	if c == nil {
		return whc
	}

	bundle := getAPIServerCABundle(namespace, c, l)
	if bundle != nil {
		for i := range whc.Webhooks {
			whc.Webhooks[i].ClientConfig.CABundle = bundle
			whc.Webhooks[i].FailurePolicy = &failurePolicy
		}
	}

	return whc
}

func createObjectTransferValidatingWebhook(namespace string, c client.Client, l logr.Logger) *admissionregistrationv1beta1.ValidatingWebhookConfiguration {
	path := "/objecttransfer-validate"
	sideEffect := admissionregistrationv1beta1.SideEffectClassNone
	defaultServicePort := int32(443)
	allScopes := admissionregistrationv1beta1.AllScopes
	exactPolicy := admissionregistrationv1beta1.Exact
	failurePolicy := admissionregistrationv1beta1.Fail
	defaultTimeoutSeconds := int32(30)
	whc := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "objecttransfer-api-validate",
			Labels: map[string]string{
				utils.CDILabel: apiServerServiceName,
			},
		},
		Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
			{
				Name: "objecttransfer-validate.cdi.kubevirt.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{
						admissionregistrationv1beta1.Create,
						admissionregistrationv1beta1.Update,
					},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups: []string{cdicorev1.SchemeGroupVersion.Group},
						APIVersions: []string{
							cdicorev1.SchemeGroupVersion.Version,
						},
						Resources: []string{"objecttransfers"},
						Scope:     &allScopes,
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      apiServerServiceName,
						Path:      &path,
						Port:      &defaultServicePort,
					},
				},
				SideEffects:       &sideEffect,
				FailurePolicy:     &failurePolicy,
				MatchPolicy:       &exactPolicy,
				NamespaceSelector: &metav1.LabelSelector{},
				TimeoutSeconds:    &defaultTimeoutSeconds,
				AdmissionReviewVersions: []string{
					"v1beta1",
				},
				ObjectSelector: &metav1.LabelSelector{},
			},
		},
	}

	if c == nil {
		return whc
	}

	bundle := getAPIServerCABundle(namespace, c, l)
	if bundle != nil {
		for i := range whc.Webhooks {
			whc.Webhooks[i].ClientConfig.CABundle = bundle
			whc.Webhooks[i].FailurePolicy = &failurePolicy
		}
	}

	return whc
}

func createDataVolumeMutatingWebhook(namespace string, c client.Client, l logr.Logger) *admissionregistrationv1beta1.MutatingWebhookConfiguration {
	path := "/datavolume-mutate"
	defaultServicePort := int32(443)
	allScopes := admissionregistrationv1beta1.AllScopes
	exactPolicy := admissionregistrationv1beta1.Exact
	failurePolicy := admissionregistrationv1beta1.Fail
	defaultTimeoutSeconds := int32(30)
	reinvocationNever := admissionregistrationv1beta1.NeverReinvocationPolicy
	sideEffect := admissionregistrationv1beta1.SideEffectClassNone
	whc := &admissionregistrationv1beta1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdi-api-datavolume-mutate",
			Labels: map[string]string{
				utils.CDILabel: apiServerServiceName,
			},
		},
		Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name: "datavolume-mutate.cdi.kubevirt.io",
				Rules: []admissionregistrationv1beta1.RuleWithOperations{{
					Operations: []admissionregistrationv1beta1.OperationType{
						admissionregistrationv1beta1.Create,
						admissionregistrationv1beta1.Update,
					},
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups: []string{cdicorev1.SchemeGroupVersion.Group},
						APIVersions: []string{
							cdicorev1.SchemeGroupVersion.Version,
							cdicorev1alpha1.SchemeGroupVersion.Version,
						},
						Resources: []string{"datavolumes"},
						Scope:     &allScopes,
					},
				}},
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					Service: &admissionregistrationv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      apiServerServiceName,
						Path:      &path,
						Port:      &defaultServicePort,
					},
				},
				FailurePolicy:     &failurePolicy,
				SideEffects:       &sideEffect,
				MatchPolicy:       &exactPolicy,
				NamespaceSelector: &metav1.LabelSelector{},
				TimeoutSeconds:    &defaultTimeoutSeconds,
				AdmissionReviewVersions: []string{
					"v1beta1",
				},
				ObjectSelector:     &metav1.LabelSelector{},
				ReinvocationPolicy: &reinvocationNever,
			},
		},
	}

	if c == nil {
		return whc
	}

	bundle := getAPIServerCABundle(namespace, c, l)
	if bundle != nil {
		whc.Webhooks[0].ClientConfig.CABundle = bundle
	}

	return whc
}

func getAPIServerCABundle(namespace string, c client.Client, l logr.Logger) []byte {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: namespace, Name: "cdi-apiserver-signer-bundle"}
	if err := c.Get(context.TODO(), key, cm); err != nil {
		if l != nil {
			l.Error(err, "error getting apiserver ca bundle")
		}
		return nil
	}
	if cert, ok := cm.Data["ca-bundle.crt"]; ok {
		return []byte(cert)
	}
	if l != nil {
		l.V(2).Info("apiserver ca bundle missing from configmap")
	}
	return nil
}

func createAPIServerClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return utils.ResourcesBuiler.CreateClusterRoleBinding(apiServerResourceName, apiServerResourceName, apiServerResourceName, namespace)
}

func createAPIServerClusterRole() *rbacv1.ClusterRole {
	return utils.ResourcesBuiler.CreateClusterRole(apiServerResourceName, getAPIServerClusterPolicyRules())
}
