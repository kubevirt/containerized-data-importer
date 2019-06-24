package catalogsourceconfig

import (
	"context"

	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	"github.com/operator-framework/operator-marketplace/pkg/phase"
	"github.com/sirupsen/logrus"
)

// NewInitialReconciler returns a Reconciler that reconciles a
// CatalogSourceConfig object in the "Initial" phase.
func NewInitialReconciler(log *logrus.Entry) Reconciler {
	return &initialReconciler{
		log: log,
	}
}

// initialReconciler is an implementation of Reconciler interface that
// reconciles a CatalogSourceConfig object in the "Initial" phase.
type initialReconciler struct {
	log *logrus.Entry
}

// Reconcile reconciles a CatalogSourceConfig object that is in the "Initial"
// phase. This is the first phase in the reconciliation process.
//
// Upon success, it returns "Configuring" as the next desired phase.
func (r *initialReconciler) Reconcile(ctx context.Context, in *v2.CatalogSourceConfig) (out *v2.CatalogSourceConfig, nextPhase *shared.Phase, err error) {
	if in.Status.CurrentPhase.Name != phase.Initial {
		err = phase.ErrWrongReconcilerInvoked
		return
	}

	out = in.DeepCopy()

	// When a csc is created, make sure the csc finalizer is included
	// in the object meta.
	out.EnsureFinalizer()

	// Ensure that displayname and publisher are set to default values
	// if not defined in the spec.
	out.EnsureDisplayName()
	out.EnsurePublisher()

	r.log.Info("Scheduling for configuring")

	nextPhase = phase.GetNext(phase.Configuring)
	return
}
