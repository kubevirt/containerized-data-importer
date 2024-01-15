package recordingrules

import "github.com/machadovilaca/operator-observability/pkg/operatorrules"

// Register sets up recording rules in the given namespace.
func Register(namespace string) error {
	return operatorrules.RegisterRecordingRules(
		operatorRecordingRules(namespace),
		podsRecordingRules,
	)
}
