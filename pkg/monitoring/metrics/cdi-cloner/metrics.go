package cdicloner

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

// SetupMetrics register prometheus metrics
func SetupMetrics() error {
	return operatormetrics.RegisterMetrics(
		clonerMetrics,
	)
}
