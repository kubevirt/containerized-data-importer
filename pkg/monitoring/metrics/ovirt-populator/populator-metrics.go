package ovirtpopulator

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var (
	populatorMetrics = []operatormetrics.Metric{
		populatorProgress,
	}

	populatorProgress = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_ovirt_progress_total",
			Help: "Progress of volume population",
		},
		[]string{"ownerUID"},
	)
)

// AddPopulatorProgress adds value to the populatorProgress metric
func AddPopulatorProgress(labelValue string, value float64) {
	populatorProgress.WithLabelValues(labelValue).Add(value)
}

// GetPopulatorProgress returns the populatorProgress value
func GetPopulatorProgress(labelValue string) (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := populatorProgress.WithLabelValues(labelValue).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}
