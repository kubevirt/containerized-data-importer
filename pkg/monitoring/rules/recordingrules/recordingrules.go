package recordingrules

import "github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

// Register sets up recording rules in the given namespace.
func Register(namespace string, registry *operatorrules.Registry) error {
	return registry.RegisterRecordingRules(
		operatorRecordingRules(namespace),
		podsRecordingRules,
	)
}
