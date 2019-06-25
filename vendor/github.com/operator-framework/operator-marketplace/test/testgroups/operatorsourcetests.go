package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-marketplace/test/helpers"
	"github.com/operator-framework/operator-marketplace/test/testsuites"
	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/require"
)

// OperatorSourceTestGroup creates an OperatorSource and then runs a series of test suites that rely on this resource.
func OperatorSourceTestGroup(t *testing.T) {
	// Create a ctx that is used to delete the OperatorSource at the completion of this function.
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// Get test namespace.
	namespace, err := ctx.GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// Create the OperatorSource.
	err = helpers.CreateRuntimeObject(test.Global.Client, ctx, helpers.CreateOperatorSourceDefinition(helpers.TestOperatorSourceName, namespace))
	require.NoError(t, err, "Could not create OperatorSource")

	// Run the test suites.
	t.Run("opsrc-creation-test-suite", testsuites.OpSrcCreation)
	t.Run("csc-target-namespace-test-suite", testsuites.CscTargetNamespace)
	t.Run("packages-test-suite", testsuites.PackageTests)
}
