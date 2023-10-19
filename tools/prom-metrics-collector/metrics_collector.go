/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023 Red Hat, Inc.
 *
 */

package main

import (
	parser "github.com/kubevirt/monitoring/pkg/metrics/parser"
	"kubevirt.io/containerized-data-importer/pkg/monitoring"

	dto "github.com/prometheus/client_model/go"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{}

func recordRulesDescToMetricList(mdl []monitoring.RecordRulesDesc) []monitoring.MetricOpts {
	res := make([]monitoring.MetricOpts, len(mdl))
	for i, md := range mdl {
		res[i] = metricDescriptionToMetric(md)
	}

	return res
}

func metricDescriptionToMetric(rrd monitoring.RecordRulesDesc) monitoring.MetricOpts {
	return monitoring.MetricOpts{
		Name: rrd.Opts.Name,
		Help: rrd.Opts.Help,
		Type: rrd.Opts.Type,
	}
}

// ReadMetrics read and parse the metrics to a MetricFamily
func ReadMetrics() []*dto.MetricFamily {
	cdiMetrics := recordRulesDescToMetricList(monitoring.GetRecordRulesDesc(""))
	for _, opts := range monitoring.MetricOptsList {
		cdiMetrics = append(cdiMetrics, opts)
	}
	metricsList := make([]parser.Metric, len(cdiMetrics))
	var metricFamily []*dto.MetricFamily
	for i, cdiMetric := range cdiMetrics {
		metricsList[i] = parser.Metric{
			Name: cdiMetric.Name,
			Help: cdiMetric.Help,
			Type: cdiMetric.Type,
		}
	}
	for _, cdiMetric := range metricsList {
		// Remove ignored metrics from all rules
		if _, isExcludedMetric := excludedMetrics[cdiMetric.Name]; !isExcludedMetric {
			mf := parser.CreateMetricFamily(cdiMetric)
			metricFamily = append(metricFamily, mf)
		}
	}
	return metricFamily
}
