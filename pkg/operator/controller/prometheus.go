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
	"strconv"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ruleName                 = "prometheus-cdi-rules"
	rbacName                 = "cdi-monitoring"
	monitorName              = "service-monitor-cdi"
	defaultMonitoringNs      = "monitoring"
	runbookURLBasePath       = "https://kubevirt.io/monitoring/runbooks/"
	severityAlertLabelKey    = "severity"
	partOfAlertLabelKey      = "kubernetes_operator_part_of"
	partOfAlertLabelValue    = "kubevirt"
	componentAlertLabelKey   = "kubernetes_operator_component"
	componentAlertLabelValue = common.CDILabelValue
)

func ensurePrometheusResourcesExist(c client.Client, scheme *runtime.Scheme, owner metav1.Object) error {
	namespace := owner.GetNamespace()

	cr, err := controller.GetActiveCDI(c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	prometheusResources := []client.Object{
		newPrometheusRule(namespace),
		newPrometheusServiceMonitor(namespace),
		newPrometheusRole(namespace),
		newPrometheusRoleBinding(namespace),
	}

	for _, desired := range prometheusResources {
		if err := sdk.SetLastAppliedConfiguration(desired, LastAppliedConfigAnnotation); err != nil {
			return err
		}
		util.SetRecommendedLabels(desired, installerLabels, "cdi-operator")
		if err := controllerutil.SetControllerReference(owner, desired, scheme); err != nil {
			return err
		}

		if err := c.Create(context.TODO(), desired); err != nil {
			if errors.IsAlreadyExists(err) {
				current := sdk.NewDefaultInstance(desired)
				nn := client.ObjectKeyFromObject(desired)
				if err := c.Get(context.TODO(), nn, current); err != nil {
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
					if err := c.Update(context.TODO(), merged); err != nil {
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

func isPrometheusDeployed(logger logr.Logger, c client.Client, namespace string) (bool, error) {
	rule := &promv1.PrometheusRule{}
	key := client.ObjectKey{Namespace: namespace, Name: ruleName}
	if err := c.Get(context.TODO(), key, rule); err != nil {
		if meta.IsNoMatchError(err) {
			logger.V(3).Info("No match error for PrometheusRule, must not have prometheus deployed")
			return false, nil
		} else if !errors.IsNotFound(err) {
			return false, err
		}
	}

	return true, nil
}

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	return &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel:  "",
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name: "cdi.rules",
					Rules: []promv1.Rule{
						generateRecordRule(
							"kubevirt_cdi_operator_up_total",
							fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
						),
						generateAlertRule(
							"CDIOperatorDown",
							"kubevirt_cdi_operator_up_total == 0",
							"5m",
							map[string]string{
								"summary":     "CDI operator is down",
								"runbook_url": runbookURLBasePath + "CDIOperatorDown",
							},
							map[string]string{
								severityAlertLabelKey:  "warning",
								partOfAlertLabelKey:    partOfAlertLabelValue,
								componentAlertLabelKey: componentAlertLabelValue,
							},
						),
						generateAlertRule(
							"CDINotReady",
							"kubevirt_cdi_cr_ready == 0",
							"5m",
							map[string]string{
								"summary":     "CDI is not available to use",
								"runbook_url": runbookURLBasePath + "CDINotReady",
							},
							map[string]string{
								severityAlertLabelKey:  "warning",
								partOfAlertLabelKey:    partOfAlertLabelValue,
								componentAlertLabelKey: componentAlertLabelValue,
							},
						),
						generateRecordRule(
							"kubevirt_cdi_import_dv_unusual_restartcount_total",
							fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
						),
						generateRecordRule(
							"kubevirt_cdi_upload_dv_unusual_restartcount_total",
							fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
						),
						generateRecordRule(
							"kubevirt_cdi_clone_dv_unusual_restartcount_total",
							fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
						),
						generateAlertRule(
							"CDIDataVolumeUnusualRestartCount",
							"kubevirt_cdi_import_dv_unusual_restartcount_total > 0 or kubevirt_cdi_upload_dv_unusual_restartcount_total > 0 or kubevirt_cdi_clone_dv_unusual_restartcount_total > 0",
							"5m",
							map[string]string{
								"summary":     "Cluster has DVs with an unusual restart count, meaning they are probably failing and need to be investigated",
								"runbook_url": runbookURLBasePath + "CDIDataVolumeUnusualRestartCount",
							},
							map[string]string{
								severityAlertLabelKey:  "warning",
								partOfAlertLabelKey:    partOfAlertLabelValue,
								componentAlertLabelKey: componentAlertLabelValue,
							},
						),
						generateAlertRule(
							"CDIStorageProfilesIncomplete",
							"kubevirt_cdi_incomplete_storageprofiles_total > 0",
							"5m",
							map[string]string{
								"summary":     "StorageProfiles are incomplete, accessMode/volumeMode cannot be inferred by CDI",
								"runbook_url": runbookURLBasePath + "CDIStorageProfilesIncomplete",
							},
							map[string]string{
								severityAlertLabelKey:  "info",
								partOfAlertLabelKey:    partOfAlertLabelValue,
								componentAlertLabelKey: componentAlertLabelValue,
							},
						),
						generateRecordRule(
							"kubevirt_cdi_dataimportcron_outdated_total",
							"sum(kubevirt_cdi_dataimportcron_outdated or vector(0))",
						),
						generateAlertRule(
							"CDIDataImportCronOutdated",
							"kubevirt_cdi_dataimportcron_outdated_total > 0",
							"15m",
							map[string]string{
								"summary":     "DataImportCron latest imports are outdated",
								"runbook_url": runbookURLBasePath + "CDIDataImportCronOutdated",
							},
							map[string]string{
								severityAlertLabelKey:  "info",
								partOfAlertLabelKey:    partOfAlertLabelValue,
								componentAlertLabelKey: componentAlertLabelValue,
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
				common.CDIComponentLabel:  "",
				common.PrometheusLabelKey: common.PrometheusLabelValue,
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
