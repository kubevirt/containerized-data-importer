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

// excludedMetrics defines the metrics to ignore,
// open issue:https://github.com/kubevirt/containerized-data-importer/issues/2773
// Do not add metrics to this list!
var excludedMetrics = map[string]struct{}{
	"clone_progress":                                {},
	"kubevirt_cdi_operator_up_total":                {},
	"kubevirt_cdi_incomplete_storageprofiles_total": {},
}

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
