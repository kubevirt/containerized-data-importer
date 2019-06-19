package operatorsource_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	gomock "github.com/golang/mock/gomock"
	marketplace "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	mocks "github.com/operator-framework/operator-marketplace/pkg/mocks/operatorsource_mocks"
	"github.com/operator-framework/operator-marketplace/pkg/operatorsource"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
)

// This test verifies the happy path for purge. We expect purge to be successful
// and the next desired phase set to "Initial" so that reconciliation can start
// anew.
func TestReconcileWithPurging(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	ctx := context.TODO()

	opsrcIn := helperNewOperatorSourceWithPhase("marketplace", "foo", phase.OperatorSourcePurging)
	opsrcWant := opsrcIn.DeepCopy()

	nextPhaseWant := &marketplace.Phase{
		Name:    phase.Initial,
		Message: phase.GetMessage(phase.Initial),
	}

	datastore := mocks.NewDatastoreWriter(controller)
	fakeclient := NewFakeClient()
	reconciler := operatorsource.NewPurgingReconciler(helperGetContextLogger(), datastore, fakeclient)

	// We expect the operator source to be removed from the datastore.
	datastore.EXPECT().RemoveOperatorSource(opsrcIn.GetUID()).Times(1)

	opsrcGot, nextPhaseGot, errGot := reconciler.Reconcile(ctx, opsrcIn)

	assert.NoError(t, errGot)
	assert.Equal(t, opsrcWant, opsrcGot)
	assert.Equal(t, nextPhaseWant, nextPhaseGot)
}
