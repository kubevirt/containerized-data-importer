package rules

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/alerts"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/recordingrules"
)

const (
	ruleName = "prometheus-cdi-rules"
)

// SetupRules initializes recording and alert rules in a namespace.
func SetupRules(namespace string) error {
	registry := operatorrules.NewRegistry()

	if err := recordingrules.Register(namespace, registry); err != nil {
		return err
	}

	if err := alerts.Register(namespace, registry); err != nil {
		return err
	}

	return nil
}

// BuildPrometheusRule creates a PrometheusRule in a namespace.
func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	registry := operatorrules.NewRegistry()
	_ = recordingrules.Register(namespace, registry)
	_ = alerts.Register(namespace, registry)
	return registry.BuildPrometheusRule(
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
	registry := operatorrules.NewRegistry()
	_ = recordingrules.Register("", registry)
	return registry.ListRecordingRules()
}

// ListAlerts returns all configured alert rules.
func ListAlerts() []promv1.Rule {
	registry := operatorrules.NewRegistry()
	_ = alerts.Register("", registry)
	return registry.ListAlerts()
}
