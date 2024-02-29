package rules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/alerts"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/recordingrules"
)

const (
	ruleName = "prometheus-cdi-rules"
)

// SetupRules initializes recording and alert rules in a namespace.
func SetupRules(namespace string) error {
	if err := recordingrules.Register(namespace); err != nil {
		return err
	}

	if err := alerts.Register(namespace); err != nil {
		return err
	}

	return nil
}

// BuildPrometheusRule creates a PrometheusRule in a namespace.
func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	return operatorrules.BuildPrometheusRule(
		ruleName,
		namespace,
		map[string]string{
			common.CDIComponentLabel:  "",
			common.PrometheusLabelKey: common.PrometheusLabelValue,
		},
	)
}

// ListRecordingRules returns all configured recording rules.
func ListRecordingRules() []operatorrules.RecordingRule {
	return operatorrules.ListRecordingRules()
}

// ListAlerts returns all configured alert rules.
func ListAlerts() []promv1.Rule {
	return operatorrules.ListAlerts()
}
