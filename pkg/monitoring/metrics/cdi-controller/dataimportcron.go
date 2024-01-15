package cdicontroller

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
)

const (
	prometheusNsLabel       = "ns"
	prometheusCronNameLabel = "cron_name"
)

var (
	dataImportCronMetrics = []operatormetrics.Metric{
		dataImportCronOutdated,
	}

	dataImportCronOutdated = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_dataimportcron_outdated",
			Help: "DataImportCron has an outdated import",
		},
		[]string{prometheusNsLabel, prometheusCronNameLabel},
	)
)

// SetDataImportCronOutdated sets dataImportCronOutdated value
func SetDataImportCronOutdated(labels prometheus.Labels, isOutdated bool) {
	var outdatedValue float64
	if isOutdated {
		outdatedValue = 1.0 // true
	} else {
		outdatedValue = 0.0 // false
	}
	dataImportCronOutdated.With(labels).Set(outdatedValue)
}

// DeleteDataImportCronOutdated deletes metrics by their labels, and return the number of deleted metrics
func DeleteDataImportCronOutdated(labels prometheus.Labels) int {
	return dataImportCronOutdated.DeletePartialMatch(labels)
}
