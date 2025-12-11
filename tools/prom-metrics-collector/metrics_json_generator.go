package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubevirt/monitoring/pkg/metrics/parser"
	dto "github.com/prometheus/client_model/go"
	om "github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

	cdiClonerMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	cdiMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	cdiImporterMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
	openstackPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/openstack-populator"
	operatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	ovirtPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/ovirt-populator"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{}

type RecordingRule struct {
	Record string `json:"record,omitempty"`
	Expr   string `json:"expr,omitempty"`
	Type   string `json:"type,omitempty"`
}

type Output struct {
	MetricFamilies []*dto.MetricFamily `json:"metricFamilies,omitempty"`
	RecordingRules []RecordingRule     `json:"recordingRules,omitempty"`
}

func main() {
	err := operatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiImporterMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiClonerMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = openstackPopulatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = ovirtPopulatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	if err := rules.SetupRules("test"); err != nil {
		panic(err)
	}

	var metricFamilies []*dto.MetricFamily

	metricsList := om.ListMetrics()
	for _, m := range metricsList {
		if _, isExcludedMetric := excludedMetrics[m.GetOpts().Name]; !isExcludedMetric {
			pm := parser.Metric{
				Name: m.GetOpts().Name,
				Help: m.GetOpts().Help,
				Type: strings.ToUpper(string(m.GetBaseType())),
			}
			metricFamilies = append(metricFamilies, parser.CreateMetricFamily(pm))
		}
	}

	recNames := make(map[string]struct{})
	var recRules []RecordingRule
	rulesList := rules.ListRecordingRules()
	for _, r := range rulesList {
		name := r.GetOpts().Name
		if _, isExcludedMetric := excludedMetrics[name]; isExcludedMetric {
			continue
		}
		recNames[name] = struct{}{}
		recRules = append(recRules, RecordingRule{
			Record: name,
			Expr:   r.Expr.String(),
			Type:   strings.ToUpper(string(r.GetType())),
		})
	}

	// Filter out metric families that are also recording rules
	var filteredFamilies []*dto.MetricFamily
	for _, mf := range metricFamilies {
		if mf == nil || mf.Name == nil {
			continue
		}
		if _, isRec := recNames[*mf.Name]; isRec {
			continue
		}
		filteredFamilies = append(filteredFamilies, mf)
	}

	out := Output{MetricFamilies: filteredFamilies, RecordingRules: recRules}
	jsonBytes, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(jsonBytes)) // Write the JSON string to standard output
}
