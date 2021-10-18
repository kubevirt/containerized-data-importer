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
	"reflect"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ruleName    = "prometheus-cdi-rules"
	rbacName    = "cdi-monitoring"
	monitorName = "service-monitor-cdi"
)

func ensurePrometheusRuleExists(logger logr.Logger, c client.Client, owner metav1.Object) error {
	namespace := owner.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("cluster scoped owner not supported")
	}

	cr, err := controller.GetActiveCDI(c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	desiredRule := newPrometheusRule(namespace)
	util.SetRecommendedLabels(desiredRule, installerLabels, "cdi-operator")

	currentRule := &promv1.PrometheusRule{}
	key := client.ObjectKey{Namespace: namespace, Name: ruleName}
	err = c.Get(context.TODO(), key, currentRule)
	if err == nil {
		if !reflect.DeepEqual(currentRule.Spec, desiredRule.Spec) {
			currentRule.Spec = desiredRule.Spec
			return c.Update(context.TODO(), currentRule)
		}

		return nil
	}
	if meta.IsNoMatchError(err) {
		logger.V(3).Info("No match error for PrometheusRule, must not have prometheus deployed")
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	return c.Create(context.TODO(), desiredRule)
}

func ensurePrometheusRbacExists(logger logr.Logger, c client.Client, owner metav1.Object) error {
	namespace := owner.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("cluster scoped owner not supported")
	}

	cr, err := controller.GetActiveCDI(c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	desiredRole := newPrometheusRole(namespace)
	util.SetRecommendedLabels(desiredRole, installerLabels, "cdi-operator")
	desiredRoleBinding := newPrometheusRoleBinding(namespace)
	util.SetRecommendedLabels(desiredRoleBinding, installerLabels, "cdi-operator")

	promRule := &promv1.PrometheusRule{}
	promRulekey := client.ObjectKey{Namespace: namespace, Name: ruleName}
	if err := c.Get(context.TODO(), promRulekey, promRule); err != nil && meta.IsNoMatchError(err) {
		logger.V(3).Info("No match error for PrometheusRule, must not have prometheus deployed")
		return nil
	}

	key := client.ObjectKey{Namespace: namespace, Name: rbacName}
	if err := c.Create(context.TODO(), desiredRole); err != nil && errors.IsAlreadyExists(err) {
		currentRole := &rbacv1.Role{}
		if err := c.Get(context.TODO(), key, currentRole); err != nil {
			return err
		}
		if !reflect.DeepEqual(currentRole.Rules, desiredRole.Rules) {
			currentRole.Rules = desiredRole.Rules
			if err := c.Update(context.TODO(), currentRole); err != nil {
				return err
			}
		}
	}
	if err := c.Create(context.TODO(), desiredRoleBinding); err != nil && errors.IsAlreadyExists(err) {
		currentRoleBinding := &rbacv1.RoleBinding{}
		if err := c.Get(context.TODO(), key, currentRoleBinding); err != nil {
			return err
		}
		if !reflect.DeepEqual(currentRoleBinding.Subjects, desiredRoleBinding.Subjects) {
			currentRoleBinding.Subjects = desiredRoleBinding.Subjects
			if err := c.Update(context.TODO(), currentRoleBinding); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensurePrometheusServiceMonitorExists(logger logr.Logger, c client.Client, owner metav1.Object) error {
	namespace := owner.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("cluster scoped owner not supported")
	}

	cr, err := controller.GetActiveCDI(c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	desiredMonitor := newPrometheusServiceMonitor(namespace)
	util.SetRecommendedLabels(desiredMonitor, installerLabels, "cdi-operator")

	promRule := &promv1.PrometheusRule{}
	promRulekey := client.ObjectKey{Namespace: namespace, Name: ruleName}
	if err := c.Get(context.TODO(), promRulekey, promRule); err != nil && meta.IsNoMatchError(err) {
		logger.V(3).Info("No match error for PrometheusRule, must not have prometheus deployed")
		return nil
	}

	key := client.ObjectKey{Namespace: namespace, Name: monitorName}
	if err := c.Create(context.TODO(), desiredMonitor); err != nil && errors.IsAlreadyExists(err) {
		currentMonitor := &promv1.ServiceMonitor{}
		if err := c.Get(context.TODO(), key, currentMonitor); err != nil {
			return err
		}
		if !reflect.DeepEqual(currentMonitor.Spec, desiredMonitor.Spec) {
			currentMonitor.Spec = desiredMonitor.Spec
			if err := c.Update(context.TODO(), currentMonitor); err != nil {
				return err
			}
		}
	}

	return nil
}

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	return &promv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
			Labels: map[string]string{
				common.PrometheusLabel: "",
			},
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name: "cdi.rules",
					Rules: []promv1.Rule{
						generateRecordRule(
							"cdi_num_up_operators",
							fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
						),
						generateAlertRule(
							"CdiOperatorDown",
							"cdi_num_up_operators == 0",
							"5m",
							map[string]string{
								"summary": "CDI operator is down",
							},
							map[string]string{
								"severity": "warning",
							},
						),
					},
				},
			},
		},
	}
}

func newPrometheusRole(namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				common.PrometheusLabel: "",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{
					"get", "list", "watch",
				},
			},
		},
	}
}

func newPrometheusRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: namespace,
			Labels: map[string]string{
				common.PrometheusLabel: "",
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
				Namespace: "openshift-monitoring",
				Name:      "prometheus-k8s",
			},
		},
	}
}

func newPrometheusServiceMonitor(namespace string) *promv1.ServiceMonitor {
	return &promv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: promv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      monitorName,
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "",
				common.PrometheusLabel:            "",
			},
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.PrometheusLabel: "",
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []promv1.Endpoint{
				{
					Port:   "metrics",
					Scheme: "http",
					TLSConfig: &promv1.TLSConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
	}
}

func generateAlertRule(alert, expr, duration string, annotations, labels map[string]string) promv1.Rule {
	return promv1.Rule{
		Alert:       alert,
		Expr:        intstr.FromString(expr),
		For:         duration,
		Annotations: annotations,
		Labels:      labels,
	}
}

func generateRecordRule(record, expr string) promv1.Rule {
	return promv1.Rule{
		Record: record,
		Expr:   intstr.FromString(expr),
	}
}

func (r *ReconcileCDI) watchPrometheusResources() error {
	var err error

	err = r.controller.Watch(
		&source.Kind{Type: &promv1.PrometheusRule{}},
		enqueueCDI(r.client),
	)
	if err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Not watching PrometheusRules")
			return nil
		}

		return err
	}

	err = r.controller.Watch(
		&source.Kind{Type: &promv1.ServiceMonitor{}},
		enqueueCDI(r.client),
	)
	if err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Not watching ServiceMonitors")
			return nil
		}

		return err
	}

	err = r.controller.Watch(
		&source.Kind{Type: &rbacv1.Role{}},
		enqueueCDI(r.client),
	)
	if err != nil {
		return err
	}

	err = r.controller.Watch(
		&source.Kind{Type: &rbacv1.RoleBinding{}},
		enqueueCDI(r.client),
	)
	if err != nil {
		return err
	}

	return nil
}
