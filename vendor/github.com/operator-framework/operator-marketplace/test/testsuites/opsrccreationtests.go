package testsuites

import (
	"fmt"
	"testing"

	olm "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-marketplace/test/helpers"
	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpSrcCreation is a test suite that ensures that the expected kubernetets resources are
// created by marketplace after the creation of an OperatorSource.
func OpSrcCreation(t *testing.T) {
	t.Run("operator-source-generates-expected-objects", testOperatorSourceGeneratesExpectedObjects)
}

// testOperatorSourceGeneratesExpectedObjects ensures that after creating an OperatorSource that the
// following objects are generated as a result:
// a CatalogSourceConfig
// a CatalogSource with expected labels
// a Service
// a Deployment that has reached a ready state
func testOperatorSourceGeneratesExpectedObjects(t *testing.T) {
	// Get test namespace
	namespace, err := test.NewTestCtx(t).GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// Check for the CatalogSourceConfig and it's child resources.
	err = helpers.CheckCscSuccessfulCreation(test.Global.Client, helpers.TestOperatorSourceName, namespace, namespace)
	require.NoError(t, err)

	// Then check for the CatalogSource.
	resultCatalogSource := &olm.CatalogSource{}
	err = helpers.WaitForResult(test.Global.Client, resultCatalogSource, namespace, helpers.TestOperatorSourceName)
	require.NoError(t, err)

	// Check that the CatalogSource has the expected labels.
	labels := resultCatalogSource.GetLabels()
	groupGot, ok := labels[helpers.TestOperatorSourceLabelKey]

	assert.True(t, ok)
	assert.Equal(t, helpers.TestOperatorSourceLabelValue, groupGot,
		fmt.Sprintf("The created CatalogSource %s does not have the right label[%s] - want=%s got=%s",
			resultCatalogSource.Name,
			helpers.TestOperatorSourceLabelKey,
			helpers.TestOperatorSourceLabelValue,
			groupGot))
}
