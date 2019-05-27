package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-marketplace/test/testsuites"
)

// ClusterOperatorTestGroup runs test suites that check the status of the Cluster Operator
func ClusterOperatorTestGroup(t *testing.T) {
	// Run the test suites.
	t.Run("cluster-operator-status-on-startup-test-suite", testsuites.ClusterOperatorStatusOnStartup)
}
