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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	"kubevirt.io/containerized-data-importer/pkg/util"

	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
)

const (
	ruleName                  = "prometheus-cdi-rules"
	rbacName                  = "cdi-monitoring"
	monitorName               = "service-monitor-cdi"
	defaultMonitoringNs       = "monitoring"
	defaultRunbookURLTemplate = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv     = "RUNBOOK_URL_TEMPLATE"
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
	partOfAlertLabelKey       = "kubernetes_operator_part_of"
	partOfAlertLabelValue     = "kubevirt"
	componentAlertLabelKey    = "kubernetes_operator_component"
	componentAlertLabelValue  = common.CDILabelValue
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

func isPrometheusDeployed(logger logr.Logger, c client.Client, namespace string) (bool, error) {
	rule := &promv1.PrometheusRule{}
	key := client.ObjectKey{Namespace: namespace, Name: ruleName}
	if err := c.Get(context.TODO(), key, rule); err != nil {
		if meta.IsNoMatchError(err) {
			logger.V(3).Info("No match error for PrometheusRule, must not have prometheus deployed")
			return false, nil
		} else if !k8serrors.IsNotFound(err) {
			return false, err
		}
	}

	return true, nil
}

func getRecordRules(namespace string) []promv1.Rule {
	var recordRules []promv1.Rule

	for _, rrd := range monitoring.GetRecordRulesDesc(namespace) {
		recordRules = append(recordRules, generateRecordRule(rrd.Opts.Name, rrd.Expr))
	}

	return recordRules
}

func getAlertRules(runbookURLTemplate string) []promv1.Rule {
	return []promv1.Rule{
		generateAlertRule(
			"CDIOperatorDown",
			"kubevirt_cdi_operator_up == 0",
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "CDI operator is down",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIOperatorDown"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDINotReady",
			"kubevirt_cdi_cr_ready == 0",
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "CDI is not available to use",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDINotReady"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDIDataVolumeUnusualRestartCount",
			`kubevirt_cdi_import_pods_high_restart > 0 or
			kubevirt_cdi_upload_pods_high_restart > 0 or
			kubevirt_cdi_clone_pods_high_restart > 0`,
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "Some CDI population workloads have an unusual restart count, meaning they are probably failing and need to be investigated",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIDataVolumeUnusualRestartCount"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDIStorageProfilesIncomplete",
			`sum by(storageclass,provisioner) ((kubevirt_cdi_storageprofile_info{complete="false"}>0))`,
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "Incomplete StorageProfile {{ $labels.storageclass }}, accessMode/volumeMode cannot be inferred by CDI for PVC population request",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIStorageProfilesIncomplete"),
			},
			map[string]string{
				severityAlertLabelKey:     "info",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDIDataImportCronOutdated",
			`sum by(ns,cron_name) (kubevirt_cdi_dataimportcron_outdated{pending="false"}) > 0`,
			promv1.Duration("15m"),
			map[string]string{
				"summary":     "DataImportCron (recurring polling of VM templates disk image sources, also known as golden images) PVCs are not being updated on the defined schedule",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIDataImportCronOutdated"),
			},
			map[string]string{
				severityAlertLabelKey:     "info",
				healthImpactAlertLabelKey: "warning",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDINoDefaultStorageClass",
			`sum(kubevirt_cdi_storageprofile_info{default="true"} or on() vector(0)) +
			sum(kubevirt_cdi_storageprofile_info{virtdefault="true"} or on() vector(0)) +
			(count(kubevirt_cdi_datavolume_pending == 0) or on() vector(0)) == 0`,
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "No default StorageClass or virtualization StorageClass, and a DataVolume is pending for one",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDINoDefaultStorageClass"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDIMultipleDefaultVirtStorageClasses",
			`sum(kubevirt_cdi_storageprofile_info{virtdefault="true"} or on() vector(0)) > 1`,
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "More than one default virtualization StorageClass detected",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIMultipleDefaultVirtStorageClasses"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
		generateAlertRule(
			"CDIDefaultStorageClassDegraded",
			`sum(kubevirt_cdi_storageprofile_info{default="true",rwx="true",smartclone="true"} or on() vector(0)) +
			sum(kubevirt_cdi_storageprofile_info{virtdefault="true",rwx="true",smartclone="true"} or on() vector(0)) +
			on () (0*(sum(kubevirt_cdi_storageprofile_info{default="true"}) or sum(kubevirt_cdi_storageprofile_info{virtdefault="true"}))) == 0`,
			promv1.Duration("5m"),
			map[string]string{
				"summary":     "Default storage class has no smart clone or ReadWriteMany",
				"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIDefaultStorageClassDegraded"),
			},
			map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
				partOfAlertLabelKey:       partOfAlertLabelValue,
				componentAlertLabelKey:    componentAlertLabelValue,
			},
		),
	}
}

func newPrometheusRule(namespace string) *promv1.PrometheusRule {
	runbookURLTemplate := getRunbookURLTemplate()

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
					Name:  "cdi.rules",
					Rules: append(getRecordRules(namespace), getAlertRules(runbookURLTemplate)...),
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
						SafeTLSConfig: promv1.SafeTLSConfig{
							InsecureSkipVerify: true,
						},
					},
				},
			},
		},
	}
}

func generateAlertRule(alert, expr string, duration promv1.Duration, annotations, labels map[string]string) promv1.Rule {
	return promv1.Rule{
		Alert:       alert,
		Expr:        intstr.FromString(expr),
		For:         &duration,
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
			if err := r.controller.Watch(&source.Kind{Type: obj}, enqueueCDI(r.client)); err != nil {
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
		if err := r.controller.Watch(&source.Kind{Type: obj}, enqueueCDI(r.client)); err != nil {
			return err
		}
	}

	return nil
}

func getRunbookURLTemplate() string {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		panic(errors.New("runbook URL template must have exactly 1 %s substring"))
	}

	return runbookURLTemplate
}
