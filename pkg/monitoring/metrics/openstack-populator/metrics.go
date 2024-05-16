package openstackpopulator

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"

	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// SetupMetrics register prometheus metrics
func SetupMetrics() error {
	operatormetrics.Register = runtimemetrics.Registry.Register
	operatormetrics.Unregister = runtimemetrics.Registry.Unregister
	return operatormetrics.RegisterMetrics(
		populatorMetrics,
	)
}
