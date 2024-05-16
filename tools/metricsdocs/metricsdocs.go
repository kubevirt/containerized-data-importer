package main

import (
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/docs"
	om "github.com/machadovilaca/operator-observability/pkg/operatormetrics"

	cdiClonerMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	cdiMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	openstackPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/openstack-populator"
	operatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	ovirtPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/ovirt-populator"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
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

	docsString := docs.BuildMetricsDocsWithCustomTemplate(om.ListMetrics(), rules.ListRecordingRules(), tpl)

	fmt.Print(docsString)
}
