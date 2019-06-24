package testsuites

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	"github.com/operator-framework/operator-marketplace/test/helpers"
	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackageTests is a test suite that ensures that package behave as intended
func PackageTests(t *testing.T) {
	t.Run("csc-with-non-existing-package", testCscWithNonExistingPackage)
	t.Run("opsrc-with-conflicting-packages", testOpSrcWithConflictingPackages)
}

// testCscWithNonExistingPackage tests that a csc with a non-existing package
// is handled correctly by the marketplace operator and its child resources are not
// created.
func testCscWithNonExistingPackage(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// Get global framework variables.
	client := test.Global.Client

	// Get test namespace
	namespace, err := ctx.GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// Create a new catalogsourceconfig with a non-existing Package
	nonExistingPackageCSC := &v2.CatalogSourceConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: v2.CatalogSourceConfigKind,
		}, ObjectMeta: metav1.ObjectMeta{
			Name:      cscName,
			Namespace: namespace,
		},
		Spec: v2.CatalogSourceConfigSpec{
			TargetNamespace: namespace,
			Packages:        nonExistingPackageName,
		}}

	err = helpers.CreateRuntimeObject(client, ctx, nonExistingPackageCSC)
	require.NoError(t, err, "Unable to create test CatalogSourceConfig")

	// Check status is updated correctly then check child resources are not created
	t.Run("configuring-state-when-package-name-does-not-exist", configuringStateWhenPackageNameDoesNotExist)
	t.Run("child-resources-not-created", childResourcesNotCreated)
}

// configuringStateWhenTargetNamespaceDoesNotExist is a test case that creates a CatalogSourceConfig
// with a targetNamespace that doesn't exist and ensures that the status is updated to reflect the
// nonexistent namespace.
func configuringStateWhenPackageNameDoesNotExist(t *testing.T) {
	namespace, err := test.NewTestCtx(t).GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// Check that the catalogsourceconfig with an non-existing package eventually reaches the
	// configuring phase with the expected message
	expectedPhase := "Configuring"
	expectedMessage := fmt.Sprintf("Still resolving package(s) - %v. Please make sure these are valid packages.", nonExistingPackageName)
	err = helpers.WaitForExpectedPhaseAndMessage(test.Global.Client, cscName, namespace, expectedPhase, expectedMessage)
	assert.NoError(t, err, fmt.Sprintf("CatalogSourceConfig never reached expected phase/message, expected %v/%v", expectedPhase, expectedMessage))
}

// childResourcesNotCreated checks that once a CatalogSourceConfig with a wrong package name
// is created that all expected runtime objects are not created.
func childResourcesNotCreated(t *testing.T) {
	// Get test namespace.
	namespace, err := test.NewTestCtx(t).GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// Check that the CatalogSourceConfig's child resources were not created.
	err = helpers.CheckCscChildResourcesDeleted(test.Global.Client, cscName, namespace, namespace)
	assert.NoError(t, err, "Child resources of CatalogSourceConfig were unexpectedly created")
}

// testOpSrcWithConflictingPackages ensures that an OperatorSource and its child resources
// are successfully rolled out regardless of whether or not there is a
// package conflict with an exiting OperatorSource.
func testOpSrcWithConflictingPackages(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// Get global framework variables.
	client := test.Global.Client

	// Get test namespace.
	namespace, err := ctx.GetNamespace()
	require.NoError(t, err, "Could not get namespace")

	// The OperatorSource created below will point to the same Application Registry
	// as the OperatorSource created in operatorsourcetests.go and will contain
	// conflicting package names as a result.
	opSrcName := "conflicting-operators"
	err = helpers.CreateRuntimeObject(test.Global.Client, ctx, helpers.CreateOperatorSourceDefinition(opSrcName, namespace))
	assert.NoError(t, err, "Could not create operator source")

	// Confirm child resources were created without errors.
	err = helpers.CheckCscChildResourcesCreated(client, opSrcName, namespace, namespace)
	assert.NoError(t, err, "Child resources not created")
}
