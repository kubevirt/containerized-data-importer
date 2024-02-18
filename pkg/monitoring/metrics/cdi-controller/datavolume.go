package cdicontroller

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
)

var (
	dataVolumeMetrics = []operatormetrics.Metric{
		dataVolumePending,
	}

	dataVolumePending = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_datavolume_pending",
			Help: "Number of DataVolumes pending for default storage class to be configured",
		},
	)
)

// SetDataVolumePending sets dataVolumePending value
func SetDataVolumePending(value int) {
	dataVolumePending.Set(float64(value))
}
