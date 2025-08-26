package cdicloner

import (
	ioprometheusclient "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

const (
	// CloneProgressMetricName is the name of the clone progress metric
	CloneProgressMetricName = "kubevirt_cdi_clone_progress_total"
)

var (
	clonerMetrics = []operatormetrics.Metric{
		cloneProgress,
	}

	cloneProgress = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: CloneProgressMetricName,
			Help: "The clone progress in percentage",
		},
		[]string{"ownerUID"},
	)
)

type CloneProgress struct {
	ownerUID string
}

func Progress(ownerUID string) *CloneProgress {
	return &CloneProgress{ownerUID}
}

// Add adds value to the cloneProgress metric
func (cp *CloneProgress) Add(value float64) {
	cloneProgress.WithLabelValues(cp.ownerUID).Add(value)
}

// Get returns the cloneProgress value
func (cp *CloneProgress) Get() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := cloneProgress.WithLabelValues(cp.ownerUID).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}

// Delete removes the cloneProgress metric with the passed label
func (cp *CloneProgress) Delete() {
	cloneProgress.DeleteLabelValues(cp.ownerUID)
}
