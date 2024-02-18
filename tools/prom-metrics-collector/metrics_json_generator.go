package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kubevirt/monitoring/pkg/metrics/parser"
	cdiMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	operatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{}

func main() {
	err := operatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	if err := rules.SetupRules("test"); err != nil {
		panic(err)
	}

	var metricFamilies []parser.Metric

	metricsList := operatorMetrics.ListMetrics()
	for _, m := range metricsList {
		if _, isExcludedMetric := excludedMetrics[m.GetOpts().Name]; !isExcludedMetric {
			metricFamilies = append(metricFamilies, parser.Metric{
				Name: m.GetOpts().Name,
				Help: m.GetOpts().Help,
				Type: strings.ToUpper(string(m.GetBaseType())),
			})
		}
	}

	rulesList := rules.ListRecordingRules()
	for _, r := range rulesList {
		if _, isExcludedMetric := excludedMetrics[r.GetOpts().Name]; !isExcludedMetric {
			metricFamilies = append(metricFamilies, parser.Metric{
				Name: r.GetOpts().Name,
				Help: r.GetOpts().Help,
				Type: strings.ToUpper(string(r.GetType())),
			})
		}
	}

	jsonBytes, err := json.Marshal(metricFamilies)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(jsonBytes)) // Write the JSON string to standard output
}
