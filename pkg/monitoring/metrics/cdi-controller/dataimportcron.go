package cdicontroller

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"github.com/prometheus/client_golang/prometheus"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

const (
	// PrometheusCronNsLabel labels the DataImportCron namespace
	PrometheusCronNsLabel = "ns"
	// PrometheusCronNameLabel labels the DataImportCron name
	PrometheusCronNameLabel = "cron_name"
	// PrometheusCronPendingLabel labels whether the DataImportCron import DataVolume is pending for default storage class
	PrometheusCronPendingLabel = "pending"
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
		[]string{PrometheusCronNsLabel, PrometheusCronNameLabel, PrometheusCronPendingLabel},
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

// GetDataImportCronOutdated returns the dataImportCronOutdated value
func GetDataImportCronOutdated(labels prometheus.Labels) float64 {
	dto := &ioprometheusclient.Metric{}
	_ = dataImportCronOutdated.With(labels).Write(dto)
	return dto.Gauge.GetValue()
}

// DeleteDataImportCronOutdated deletes metrics by their labels, and return the number of deleted metrics
func DeleteDataImportCronOutdated(labels prometheus.Labels) int {
	return dataImportCronOutdated.DeletePartialMatch(labels)
}
