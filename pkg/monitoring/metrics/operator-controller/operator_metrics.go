package operatorcontroller

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

var (
	operatorMetrics = []operatormetrics.Metric{
		readyGauge,
	}

	readyGauge = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_cr_ready",
			Help: "CDI install ready",
		},
	)
)

// SetReady sets the readyGauge to true
func SetReady() {
	readyGauge.Set(1.0)
}

// SetNotReady sets the readyGauge to false
func SetNotReady() {
	readyGauge.Set(0.0)
}

// SetInit sets the readyGauge to -1, 0 is our value for alert to start firing, so can't default to that.
func SetInit() {
	readyGauge.Set(-1.0)
}
