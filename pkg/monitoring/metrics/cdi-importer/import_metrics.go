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

// AddImportProgress adds value to the importProgress metric
func AddImportProgress(labelValue string, value float64) {
	importProgress.WithLabelValues(labelValue).Add(value)
}

// GetImportProgress returns the importProgress value
func GetImportProgress(labelValue string) (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := importProgress.WithLabelValues(labelValue).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}

// DeleteImportProgress removes the importProgress metric with the passed label
func DeleteImportProgress(labelValue string) {
	importProgress.DeleteLabelValues(labelValue)
}
