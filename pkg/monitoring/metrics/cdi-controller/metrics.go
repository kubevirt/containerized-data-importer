package cdicontroller

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

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
