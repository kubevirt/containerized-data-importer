package testsuites

import (
	"testing"

	"github.com/operator-framework/operator-marketplace/test/helpers"
	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/require"
)

// DeleteOpSrc tests that the correct cleanup occurs when an OpSrc is deleted
func DeleteOpSrc(t *testing.T) {
	t.Run("delete-operator-source", testDeleteOpSrc)
}

// testDeleteOpSrc ensures that after deleting an OperatorSource that the
// objects created as a result are deleted
func testDeleteOpSrc(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// Get global framework variables
	client := test.Global.Client

	// Get test namespace
	namespace, err := test.NewTestCtx(t).GetNamespace()
	require.NoError(t, err, "Could not get namespace.")

	testOperatorSource := helpers.CreateOperatorSourceDefinition(helpers.TestOperatorSourceName, namespace)

	// Create the OperatorSource with no cleanup options.
	err = helpers.CreateRuntimeObjectNoCleanup(client, testOperatorSource)
	require.NoError(t, err, "Could not create OperatorSource.")

	// Check for the child resources.
	err = helpers.CheckOpsrcChildResourcesCreated(test.Global.Client, testOperatorSource.Name, namespace)
	require.NoError(t, err, "Could not ensure that child resources were created")

	// Now let's delete the OperatorSource
	err = helpers.DeleteRuntimeObject(client, testOperatorSource)
	require.NoError(t, err, "OperatorSource could not be deleted successfully. Client returned error.")

	// Now let's wait until the OperatorSource is successfully deleted and the
	// child resources are removed.
	err = helpers.CheckOpsrcChildResourcesDeleted(test.Global.Client, testOperatorSource.Name, namespace)
	require.NoError(t, err, "Could not ensure child resources were deleted.")
}
