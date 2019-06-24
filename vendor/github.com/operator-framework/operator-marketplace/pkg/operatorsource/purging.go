package operatorsource

import (
	"context"

	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	wrapper "github.com/operator-framework/operator-marketplace/pkg/client"
	"github.com/operator-framework/operator-marketplace/pkg/datastore"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewPurgingReconciler returns a Reconciler that reconciles
// an OperatorSource object that is in "Purging" phase.
func NewPurgingReconciler(logger *log.Entry, datastore datastore.Writer, client client.Client) Reconciler {
	return NewPurgingReconcilerWithClientInterface(logger, datastore, wrapper.NewClient(client))
}

// NewPurgingReconcilerWithClientInterface returns a purging
// Reconciler that reconciles an OperatorSource object in "Purging"
// phase. It uses the Client interface which is a wrapper to the raw
// client provided by the operator-sdk, instead of the raw client itself.
// Using this interface facilitates mocking of kube client interaction
// with the cluster, while using fakeclient during unit testing.
func NewPurgingReconcilerWithClientInterface(logger *log.Entry, datastore datastore.Writer, client wrapper.Client) Reconciler {
	return &purgingReconciler{
		logger:    logger,
		datastore: datastore,
		client:    client,
	}
}

// purgingReconciler implements Reconciler interface.
type purgingReconciler struct {
	logger    *log.Entry
	datastore datastore.Writer
	client    wrapper.Client
}

// Reconcile reconciles an OperatorSource object that is in "Purging" phase.
//
// in represents the original OperatorSource object received from the sdk
// and before reconciliation has started.
//
// out represents the OperatorSource object after reconciliation has completed
// and could be different from the original. The OperatorSource object received
// (in) should be deep copied into (out) before changes are made.
//
// nextPhase represents the next desired phase for the given OperatorSource
// object. If nil is returned, it implies that no phase transition is expected.
//
// In this phase, we purge the current OperatorSource object, drop the Status
// field and trigger reconciliation anew from "Validating" phase.
//
// If the purge fails the OperatorSource object is moved to "Failed" phase.
func (r *purgingReconciler) Reconcile(ctx context.Context, in *v1.OperatorSource) (out *v1.OperatorSource, nextPhase *shared.Phase, err error) {
	if in.GetCurrentPhaseName() != phase.OperatorSourcePurging {
		err = phase.ErrWrongReconcilerInvoked
		return
	}

	out = in.DeepCopy()

	// We will purge the datastore and leave the CatalogSourceConfig object
	// alone. It will be updated accordingly by the reconciliation loop.

	r.datastore.RemoveOperatorSource(in.GetUID())

	r.logger.Info("Purged datastore. No change(s) were made to corresponding CatalogSourceConfig")

	// Since all observable states stored in the Status resource might already
	// be stale, we should Reset everything in Status except for 'Current Phase'
	// to their default values.
	// The reason we are not mutating current phase is because it is the
	// responsibility of the caller to set the new phase appropriately.
	tmp := out.Status.CurrentPhase
	out.Status = v1.OperatorSourceStatus{}
	out.Status.CurrentPhase = tmp

	nextPhase = phase.GetNext(phase.Initial)
	r.logger.Info("Scheduling for reconciliation from 'Initial' phase")

	return
}
