package main

import (
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/docs"
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	cdiMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	operatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/recordingrules"
)

const tpl = `# Containerized Data Importer metrics
{{- range . }}

{{ $deprecatedVersion := "" -}}
{{- with index .ExtraFields "DeprecatedVersion" -}}
    {{- $deprecatedVersion = printf " in %s" . -}}
{{- end -}}

{{- $stabilityLevel := "" -}}
{{- if and (.ExtraFields.StabilityLevel) (ne .ExtraFields.StabilityLevel "STABLE") -}}
	{{- $stabilityLevel = printf "[%s%s] " .ExtraFields.StabilityLevel $deprecatedVersion -}}
{{- end -}}

### {{ .Name }}
{{ print $stabilityLevel }}{{ .Help }}. Type: {{ .Type -}}.

{{- end }}

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
`

func main() {
	err := operatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	metricsList := operatorMetrics.ListMetrics()
	recordingRulesList := convertToRecordingRules(recordingrules.GetRecordRulesDesc(""))

	docsString := docs.BuildMetricsDocsWithCustomTemplate(metricsList, recordingRulesList, tpl)
	fmt.Print(docsString)
}

func convertToRecordingRules(recordRulesDesc []recordingrules.RecordRulesDesc) []operatorrules.RecordingRule {
	var recordingRules []operatorrules.RecordingRule

	for _, ruleDesc := range recordRulesDesc {
		recordingRule := operatorrules.RecordingRule{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: ruleDesc.Opts.Name,
				Help: ruleDesc.Opts.Help,
				// Assuming the rest of the fields are correctly mapped
			},
			MetricType: convertRulesType(ruleDesc.Opts.Type),
		}
		recordingRules = append(recordingRules, recordingRule)
	}
	return recordingRules
}

// when adding new recording rule please note that
func convertRulesType(metricType string) operatormetrics.MetricType {
	if metricType == "Gauge" {
		return operatormetrics.GaugeType
		// ... other cases ...
	}
	return ""
}
