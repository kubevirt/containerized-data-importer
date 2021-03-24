package install

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ghodss/yaml"
	secv1 "github.com/openshift/api/security/v1"
	"k8s.io/api/admissionregistration/v1beta1"
	v1beta12 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

type Strategy struct {
	serviceAccounts                 []*corev1.ServiceAccount
	clusterRoles                    []*rbacv1.ClusterRole
	clusterRoleBindings             []*rbacv1.ClusterRoleBinding
	roles                           []*rbacv1.Role
	roleBindings                    []*rbacv1.RoleBinding
	crds                            []*v1.CustomResourceDefinition
	services                        []*corev1.Service
	deployments                     []*appsv1.Deployment
	daemonSets                      []*appsv1.DaemonSet
	validatingWebhookConfigurations []*v1beta1.ValidatingWebhookConfiguration
	mutatingWebhookConfigurations   []*v1beta1.MutatingWebhookConfiguration
	apiServices                     []*v1beta12.APIService
	certificateSecrets              []*corev1.Secret
	sccs                            []*secv1.SecurityContextConstraints
	configMaps                      []*corev1.ConfigMap
	cdi                             []*cdiv1.CDI
}

func (ins *Strategy) ServiceAccounts() []*corev1.ServiceAccount {
	return ins.serviceAccounts
}

func (ins *Strategy) ClusterRoles() []*rbacv1.ClusterRole {
	return ins.clusterRoles
}

func (ins *Strategy) ClusterRoleBindings() []*rbacv1.ClusterRoleBinding {
	return ins.clusterRoleBindings
}

func (ins *Strategy) Roles() []*rbacv1.Role {
	return ins.roles
}

func (ins *Strategy) RoleBindings() []*rbacv1.RoleBinding {
	return ins.roleBindings
}

func (ins *Strategy) Services() []*corev1.Service {
	return ins.services
}

func (ins *Strategy) Deployments() []*appsv1.Deployment {
	return ins.deployments
}

func (ins *Strategy) ApiDeployments() []*appsv1.Deployment {
	var deployments []*appsv1.Deployment

	for _, deployment := range ins.deployments {
		if !strings.Contains(deployment.Name, "virt-api") {
			continue
		}
		deployments = append(deployments, deployment)
	}

	return deployments
}

func (ins *Strategy) ControllerDeployments() []*appsv1.Deployment {
	var deployments []*appsv1.Deployment

	for _, deployment := range ins.deployments {
		if strings.Contains(deployment.Name, "virt-api") {
			continue
		}
		deployments = append(deployments, deployment)

	}

	return deployments
}

func (ins *Strategy) DaemonSets() []*appsv1.DaemonSet {
	return ins.daemonSets
}

func (ins *Strategy) ValidatingWebhookConfigurations() []*admissionregistrationv1beta1.ValidatingWebhookConfiguration {
	return ins.validatingWebhookConfigurations
}

func (ins *Strategy) MutatingWebhookConfigurations() []*admissionregistrationv1beta1.MutatingWebhookConfiguration {
	return ins.mutatingWebhookConfigurations
}

func (ins *Strategy) APIServices() []*v1beta12.APIService {
	return ins.apiServices
}

func (ins *Strategy) CertificateSecrets() []*corev1.Secret {
	return ins.certificateSecrets
}

func (ins *Strategy) SCCs() []*secv1.SecurityContextConstraints {
	return ins.sccs
}

func (ins *Strategy) ConfigMaps() []*corev1.ConfigMap {
	return ins.configMaps
}

func (ins *Strategy) CRDs() []*v1.CustomResourceDefinition {
	return ins.crds
}

func NewInstallStrategyConfigMap(objects []runtime.Object, reqLogger logr.Logger, namespace string) (*corev1.ConfigMap, error) {

	strategy, err := GenerateCurrentInstallStrategy(objects, reqLogger)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdi-install-strategy",
			Namespace: namespace,
		},
		Data: map[string]string{
			"manifests": string(dumpInstallStrategyToBytes(strategy)),
		},
	}
	return configMap, nil
}

func DumpInstallStrategyToConfigMap(clientset client.Client, objects []runtime.Object, reqLogger logr.Logger, namespace string) error {
	configMap, err := NewInstallStrategyConfigMap(objects, reqLogger, namespace)
	if err != nil {
		return err
	}

	err = clientset.Create(context.TODO(), configMap)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// force update if already exists
			err = clientset.Update(context.TODO(), configMap)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func dumpInstallStrategyToBytes(strategy *Strategy) []byte {
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)

	for _, entry := range strategy.serviceAccounts {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.clusterRoles {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.clusterRoleBindings {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.roles {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.roleBindings {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.crds {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.services {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.certificateSecrets {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.validatingWebhookConfigurations {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.mutatingWebhookConfigurations {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.apiServices {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.deployments {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.daemonSets {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.sccs {
		MarshallObject(entry, writer)
	}
	for _, entry := range strategy.configMaps {
		MarshallObject(entry, writer)
	}
	writer.Flush()
	return b.Bytes()
}

func GenerateCurrentInstallStrategy(resources []runtime.Object, reqLogger logr.Logger) (*Strategy, error) {

	strategy := &Strategy{}

	for _, desiredRuntimeObj := range resources {
		kind := desiredRuntimeObj.GetObjectKind().GroupVersionKind().Kind
		switch kind {
		case "ClusterRole":
			strategy.clusterRoles = append(strategy.clusterRoles, desiredRuntimeObj.(*rbacv1.ClusterRole))
		case "ClusterRoleBinding":
			strategy.clusterRoleBindings = append(strategy.clusterRoleBindings, desiredRuntimeObj.(*rbacv1.ClusterRoleBinding))
		case "CustomResourceDefinition":
			strategy.crds = append(strategy.crds, desiredRuntimeObj.(*v1.CustomResourceDefinition))
		case "RoleBinding":
			strategy.roleBindings = append(strategy.roleBindings, desiredRuntimeObj.(*rbacv1.RoleBinding))
		case "Role":
			strategy.roles = append(strategy.roles, desiredRuntimeObj.(*rbacv1.Role))
		case "Service":
			strategy.services = append(strategy.services, desiredRuntimeObj.(*corev1.Service))
		case "Deployment":
			strategy.deployments = append(strategy.deployments, desiredRuntimeObj.(*appsv1.Deployment))
		case "ServiceAccount":
			strategy.serviceAccounts = append(strategy.serviceAccounts, desiredRuntimeObj.(*corev1.ServiceAccount))
		case "ConfigMap":
			strategy.configMaps = append(strategy.configMaps, desiredRuntimeObj.(*corev1.ConfigMap))
		case "APIService":
			strategy.apiServices = append(strategy.apiServices, desiredRuntimeObj.(*v1beta12.APIService))
		case "ValidatingWebhookConfiguration":
			strategy.validatingWebhookConfigurations = append(strategy.validatingWebhookConfigurations, desiredRuntimeObj.(*v1beta1.ValidatingWebhookConfiguration))
		case "MutatingWebhookConfiguration":
			strategy.mutatingWebhookConfigurations = append(strategy.mutatingWebhookConfigurations, desiredRuntimeObj.(*v1beta1.MutatingWebhookConfiguration))
		default:
			reqLogger.Info("Object not added to install strategy ", "kind", kind)
		}
	}

	return strategy, nil
}

func MarshallObject(obj interface{}, writer io.Writer) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}
	// remove status and metadata.creationTimestamp
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "spec", "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "status")
	// remove dataSource from PVCs if empty
	templates, exists, err := unstructured.NestedSlice(r.Object, "spec", "dataVolumeTemplates")
	if exists {
		for _, tmpl := range templates {
			template := tmpl.(map[string]interface{})
			_, exists, err = unstructured.NestedString(template, "spec", "pvc", "dataSource")
			if !exists {
				unstructured.RemoveNestedField(template, "spec", "pvc", "dataSource")
			}
		}
		unstructured.SetNestedSlice(r.Object, templates, "spec", "dataVolumeTemplates")
	}
	objects, exists, err := unstructured.NestedSlice(r.Object, "objects")
	if exists {
		for _, obj := range objects {
			object := obj.(map[string]interface{})
			kind, exists, _ := unstructured.NestedString(object, "kind")
			if exists && kind == "PersistentVolumeClaim" {
				_, exists, err = unstructured.NestedString(object, "spec", "dataSource")
				if !exists {
					unstructured.RemoveNestedField(object, "spec", "dataSource")
				}
			}
		}
		unstructured.SetNestedSlice(r.Object, objects, "objects")
	}

	deployments, exists, err := unstructured.NestedSlice(r.Object, "spec", "install", "spec", "deployments")
	if exists {
		for _, obj := range deployments {
			deployment := obj.(map[string]interface{})
			unstructured.RemoveNestedField(deployment, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "spec", "template", "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "status")
		}
		unstructured.SetNestedSlice(r.Object, deployments, "spec", "install", "spec", "deployments")
	}

	// remove "managed by operator" label...
	labels, exists, err := unstructured.NestedMap(r.Object, "metadata", "labels")
	if exists {
		delete(labels, "app.kubernetes.io/managed-by")
		unstructured.SetNestedMap(r.Object, labels, "metadata", "labels")
	}

	jsonBytes, err = json.Marshal(r.Object)
	if err != nil {
		return err
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return err
	}

	// fix templates by removing unneeded single quotes...
	s := string(yamlBytes)
	s = strings.Replace(s, "'{{", "{{", -1)
	s = strings.Replace(s, "}}'", "}}", -1)

	// fix double quoted strings by removing unneeded single quotes...
	s = strings.Replace(s, " '\"", " \"", -1)
	s = strings.Replace(s, "\"'\n", "\"\n", -1)

	yamlBytes = []byte(s)

	_, err = writer.Write([]byte("---\n"))
	if err != nil {
		return err
	}

	_, err = writer.Write(yamlBytes)
	if err != nil {
		return err
	}

	return nil
}
