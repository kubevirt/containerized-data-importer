package cdicontroller

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// SetupMetrics register prometheus metrics
func SetupMetrics() error {
	operatormetrics.Register = runtimemetrics.Registry.Register
	return operatormetrics.RegisterMetrics(
		dataImportCronMetrics,
		storageMetrics,
		dataVolumeMetrics,
	)
}

// ListMetrics registered prometheus metrics
func ListMetrics() []operatormetrics.Metric {
	return operatormetrics.ListMetrics()
}
