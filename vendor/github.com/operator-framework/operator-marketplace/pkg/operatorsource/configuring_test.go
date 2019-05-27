package operatorsource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gomock "github.com/golang/mock/gomock"
	marketplace "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	mocks "github.com/operator-framework/operator-marketplace/pkg/mocks/operatorsource_mocks"
	"github.com/operator-framework/operator-marketplace/pkg/operatorsource"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// Use Case: Not configured, CatalogSourceConfig object has not been created yet.
// Expected Result: A properly populated CatalogSourceConfig should get created
// and the next phase should be set to "Succeeded".
func TestReconcile_NotConfigured_NewCatalogConfigSourceObjectCreated(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Succeeded,
		Message: phase.GetMessage(phase.Succeeded),
	}

	datastore := mocks.NewDatastoreWriter(controller)

	fakeclient := NewFakeClient()
	reconciler := operatorsource.NewConfiguringReconciler(helperGetContextLogger(), datastore, fakeclient)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.Configuring)

	labelsWant := map[string]string{
		"opsrc-group":                           "Community",
		operatorsource.OpsrcOwnerNameLabel:      "foo",
		operatorsource.OpsrcOwnerNamespaceLabel: "marketplace",
	}
	opsrcIn.SetLabels(labelsWant)

	packages := "a,b,c"
	datastore.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return(packages)

	cscWant := helperNewCatalogSourceConfigWithLabels(opsrcIn.Namespace, opsrcIn.Name, labelsWant)
	cscWant.Spec = marketplace.CatalogSourceConfigSpec{
		TargetNamespace: opsrcIn.Namespace,
		Packages:        packages,
	}

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Not configured, CatalogSourceConfig object already exists due to
// past errors.
// Expected Result: The existing CatalogSourceConfig object should be updated
// accordingly and the next phase should be set to "Succeeded".
func TestReconcile_CatalogSourceConfigAlreadyExists_Updated(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	namespace, name := "marketplace", "foo"
	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Succeeded,
		Message: phase.GetMessage(phase.Succeeded),
	}

	datastore := mocks.NewDatastoreWriter(controller)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase(namespace, name, phase.Configuring)

	labelsWant := map[string]string{
		"opsrc-group":                           "Community",
		operatorsource.OpsrcOwnerNameLabel:      "foo",
		operatorsource.OpsrcOwnerNamespaceLabel: "marketplace",
	}
	opsrcIn.SetLabels(labelsWant)

	packages := "a,b,c"
	datastore.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return(packages)

	csc := helperNewCatalogSourceConfigWithLabels(namespace, name, labelsWant)
	fakeclient := NewFakeClientWithCSC(csc)

	reconciler := operatorsource.NewConfiguringReconciler(helperGetContextLogger(), datastore, fakeclient)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcIn, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}

// Use Case: Update of existing CatalogSourceConfig object fails.
// Expected Result: The object is moved to "Failed" phase.
func TestReconcile_UpdateError_MovedToFailedPhase(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	namespace, name := "marketplace", "foo"

	updateError := k8s_errors.NewServerTimeout(schema.GroupResource{}, "operation", 1)
	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Configuring,
		Message: updateError.Error(),
	}

	datastore := mocks.NewDatastoreWriter(controller)
	kubeclient := mocks.NewClient(controller)

	reconciler := operatorsource.NewReconcilerWithInterfaceClient(helperGetContextLogger(), datastore, kubeclient)

	ctx := context.TODO()
	opsrcIn := helperNewOperatorSourceWithPhase(namespace, name, phase.Configuring)

	datastore.EXPECT().GetPackageIDsByOperatorSource(opsrcIn.GetUID()).Return("a,b,c")

	createErr := k8s_errors.NewAlreadyExists(schema.GroupResource{}, "CatalogSourceConfig already exists")
	kubeclient.EXPECT().Create(context.TODO(), gomock.Any()).Return(createErr)

	kubeclient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).Return(nil)
	kubeclient.EXPECT().Update(context.TODO(), gomock.Any()).Return(updateError)

	_, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.Error(t, errGot)
	assert.Equal(t, updateError, errGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}
