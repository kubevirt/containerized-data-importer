package cdicloner

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var (
	clonerMetrics = []operatormetrics.Metric{
		cloneProgress,
	}

	cloneProgress = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_clone_progress_total",
			Help: "The clone progress in percentage",
		},
		[]string{"ownerUID"},
	)
)

// AddCloneProgress adds value to the cloneProgress metric
func AddCloneProgress(labelValue string, value float64) {
	cloneProgress.WithLabelValues(labelValue).Add(value)
}

// GetCloneProgress returns the cloneProgress value
func GetCloneProgress(labelValue string) (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := cloneProgress.WithLabelValues(labelValue).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}

// DeleteCloneProgress removes the cloneProgress metric with the passed label
func DeleteCloneProgress(labelValue string) {
	cloneProgress.DeleteLabelValues(labelValue)
}

type CloneProgress struct{}

func GetCloneProgressMetric() *CloneProgress {
	return &CloneProgress{}
}

func (cp *CloneProgress) Add(labelValue string, value float64) {
	AddCloneProgress(labelValue, value)
}

func (cp *CloneProgress) Get(labelValue string) (float64, error) {
	return GetCloneProgress(labelValue)
}
