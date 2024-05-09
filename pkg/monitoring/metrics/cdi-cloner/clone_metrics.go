package cdicloner

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var (
	clonerMetrics = []operatormetrics.Metric{
		cloneProgress,
	}

	cloneProgress = newCloneProgressCounterVec()
)

// InitCloneProgressCounterVec initizlizes the cloneProgress metric
func InitCloneProgressCounterVec() {
	cloneProgress = newCloneProgressCounterVec()
}

func newCloneProgressCounterVec() *operatormetrics.CounterVec {
	return operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_clone_progress",
			Help: "The clone progress in percentage",
		},
		[]string{"ownerUID"},
	)
}

// AddCloneProgress adds value to the cloneProgress metric
func AddCloneProgress(labelValue string, value float64) {
	cloneProgress.WithLabelValues(labelValue).Add(value)
}

// WriteCloneProgress writes the cloneProgress metric into a metric protocol buffer
func WriteCloneProgress(labelValue string, metric *ioprometheusclient.Metric) error {
	return cloneProgress.WithLabelValues(labelValue).Write(metric)
}
