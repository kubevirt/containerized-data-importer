package cdiimporter

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var (
	importerMetrics = []operatormetrics.Metric{
		importProgress,
	}

	importProgress = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_import_progress_total",
			Help: "The import progress in percentage",
		},
		[]string{"ownerUID"},
	)
)

type ImportProgress struct {
	ownerUID string
}

func Progress(ownerUID string) *ImportProgress {
	return &ImportProgress{ownerUID}
}

// Add adds value to the importProgress metric
func (ip *ImportProgress) Add(value float64) {
	importProgress.WithLabelValues(ip.ownerUID).Add(value)
}

// Get returns the importProgress value
func (ip *ImportProgress) Get() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := importProgress.WithLabelValues(ip.ownerUID).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}

// Delete removes the importProgress metric with the passed label
func (ip *ImportProgress) Delete() {
	importProgress.DeleteLabelValues(ip.ownerUID)
}
