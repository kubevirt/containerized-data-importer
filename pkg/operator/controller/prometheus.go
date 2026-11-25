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
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
)

const (
	prometheusRuleCRDName     = "prometheusrules.monitoring.coreos.com"
	rbacName                  = "cdi-monitoring"
	monitorName               = "service-monitor-cdi"
	defaultMonitoringNs       = "monitoring"
	defaultRunbookURLTemplate = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv     = "RUNBOOK_URL_TEMPLATE"
)

func ensurePrometheusResourcesExist(ctx context.Context, c client.Client, scheme *runtime.Scheme, owner metav1.Object) error {
	namespace := owner.GetNamespace()

	cr, err := cc.GetActiveCDI(ctx, c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	for _, desired := range newPrometheusResources(namespace) {
		if err := sdk.SetLastAppliedConfiguration(desired, LastAppliedConfigAnnotation); err != nil {
			return err
		}
		util.SetRecommendedLabels(desired, installerLabels, "cdi-operator")
		if err := controllerutil.SetControllerReference(owner, desired, scheme); err != nil {
			return err
		}

		if err := c.Create(ctx, desired); err != nil {
			if k8serrors.IsAlreadyExists(err) {
				current := sdk.NewDefaultInstance(desired)
				nn := client.ObjectKeyFromObject(desired)
				if err := c.Get(ctx, nn, current); err != nil {
					return err
				}
				current, err = sdk.StripStatusFromObject(current)
				if err != nil {
					return err
				}
				currentObjCopy := current.DeepCopyObject()
				sdk.MergeLabelsAndAnnotations(desired, current)
				merged, err := sdk.MergeObject(desired, current, LastAppliedConfigAnnotation)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(currentObjCopy, merged) {
					if err := c.Update(ctx, merged); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
	}

	return nil
}

// deletePrometheusResources deletes the monitoring resources that owner controls,
// one stage at a time. The token Secret and RBAC stay until the ServiceMonitor is
// gone, so a ServiceMonitor that is still scraped never loses its token or RBAC.
func deletePrometheusResources(ctx context.Context, c client.Client, owner metav1.Object) error {
	namespace := owner.GetNamespace()
	stages := [][]client.Object{
		{newPrometheusServiceMonitor(namespace)},
		newServiceMonitorDependencies(namespace),
	}
	for _, stage := range stages {
		if gone, err := deleteControlledObjects(ctx, c, owner, stage); err != nil || !gone {
			return err
		}
	}

	return nil
}

// deleteControlledObjects deletes each object that owner controls and reports
// whether all of them are gone. A finalizer can keep a deleted object around.
func deleteControlledObjects(ctx context.Context, c client.Client, owner metav1.Object, objs []client.Object) (bool, error) {
	gone := true
	for _, desired := range objs {
		key := client.ObjectKeyFromObject(desired)
		current := sdk.NewDefaultInstance(desired)
		if err := c.Get(ctx, key, current); err != nil {
			if k8serrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return false, err
		}
		if !metav1.IsControlledBy(current, owner) {
			continue
		}
		uid, resourceVersion := current.GetUID(), current.GetResourceVersion()
		if err := c.Delete(ctx, current, client.Preconditions{UID: &uid, ResourceVersion: &resourceVersion}); err != nil && !k8serrors.IsNotFound(err) {
			return false, err
		}
		if err := c.Get(ctx, key, current); err == nil {
			gone = false
		} else if !k8serrors.IsNotFound(err) {
			return false, err
		}
	}

	return gone, nil
}

// newPrometheusResources returns the monitoring resources, each after the resources it relies on.
func newPrometheusResources(namespace string) []client.Object {
	return append(newServiceMonitorDependencies(namespace), newPrometheusRule(namespace), newPrometheusServiceMonitor(namespace))
}

func newServiceMonitorDependencies(namespace string) []client.Object {
	return []client.Object{
		newMonitorTokenSecret(namespace),
		newPrometheusRole(namespace),
		newPrometheusRoleBinding(namespace),
	}
}

func isPrometheusDeployed(logger logr.Logger, c client.Client) (bool, error) {
	crd := &extv1.CustomResourceDefinition{}
	if err := c.Get(context.TODO(), client.ObjectKey{Name: prometheusRuleCRDName}, crd); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.V(3).Info("PrometheusRule CRD not found, must not have prometheus deployed")
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	promRule, err := rules.BuildPrometheusRule(namespace)
	if err != nil {
		panic(err)
	}

	return &promv1.PrometheusRule{
		ObjectMeta: promRule.ObjectMeta,
		Spec:       promRule.Spec,
	}
}

func newPrometheusRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel:  "",
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
		Rules: cdinamespaced.GetPrometheusNamespacedRules(),
	}
}

func newPrometheusRoleBinding(namespace string) *rbacv1.RoleBinding {
	monitoringNamespace := getMonitoringNamespace()

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel:  "",
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     rbacName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: monitoringNamespace,
				Name:      "prometheus-k8s",
			},
		},
	}
}

func newMonitorTokenSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.MetricsReaderTokenName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel:  "",
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: common.MetricsReaderServiceAccountName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
}

func getMonitoringNamespace() string {
	if ns := os.Getenv("MONITORING_NAMESPACE"); ns != "" {
		return ns
	}

	return defaultMonitoringNs
}

func newPrometheusServiceMonitor(namespace string) *promv1.ServiceMonitor {
	return &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      monitorName,
			Labels: map[string]string{
				common.CDIComponentLabel:          "",
				"openshift.io/cluster-monitoring": "",
				common.PrometheusLabelKey:         common.PrometheusLabelValue,
			},
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.PrometheusLabelKey: common.PrometheusLabelValue,
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:   "metrics",
					Scheme: "https",
					Authorization: &promv1.SafeAuthorization{
						Credentials: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: common.MetricsReaderTokenName,
							},
							Key: "token",
						},
					},
					TLSConfig: &promv1.TLSConfig{
						SafeTLSConfig: promv1.SafeTLSConfig{
							InsecureSkipVerify: ptr.To(true),
						},
					},
				},
			},
		},
	}
}

func (r *ReconcileCDI) watchPrometheusResources() error {
	listObjs := []client.ObjectList{
		&promv1.PrometheusRuleList{},
		&promv1.ServiceMonitorList{},
	}

	objs := []client.Object{
		&promv1.PrometheusRule{},
		&promv1.ServiceMonitor{},
	}

	for i, listObj := range listObjs {
		obj := objs[i]
		err := r.uncachedClient.List(context.TODO(), listObj, &client.ListOptions{
			Namespace: util.GetNamespace(),
			Limit:     1,
		})
		if err == nil {
			if err := r.controller.Watch(source.Kind(r.getCache(), obj, enqueueCDI(r.client))); err != nil {
				return err
			}
		} else if meta.IsNoMatchError(err) {
			log.Info("Not watching", "type", fmt.Sprintf("%T", obj))
		} else {
			return err
		}
	}

	objs = []client.Object{
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
	}

	for _, obj := range objs {
		if err := r.controller.Watch(source.Kind(r.getCache(), obj, enqueueCDI(r.client))); err != nil {
			return err
		}
	}

	return nil
}
