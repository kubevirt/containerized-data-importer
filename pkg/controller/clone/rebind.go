package clone

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// RebindPhaseName is the name of the rebind phase
const RebindPhaseName = "Rebind"

// RebindPhase binds a PV from one PVC to another
type RebindPhase struct {
	SourceNamespace string
	SourceName      string
	TargetNamespace string
	TargetName      string
	Client          client.Client
	Log             logr.Logger
	Recorder        record.EventRecorder
}

var _ Phase = &RebindPhase{}

// Name returns the name of the phase
func (p *RebindPhase) Name() string {
	return RebindPhaseName
}

// Reconcile rebinds a PV
func (p *RebindPhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	targetClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.TargetNamespace, p.TargetName, targetClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("target claim does not exist")
	}

	if targetClaim.Spec.VolumeName != "" {
		// guess we're all done
		return nil, nil
	}

	sourceClaim := &corev1.PersistentVolumeClaim{}
	exists, err = getResource(ctx, p.Client, p.SourceNamespace, p.SourceName, sourceClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("source claim does not exist")
	}

	if err := cc.Rebind(ctx, p.Client, sourceClaim, targetClaim); err != nil {
		return nil, err
	}

	return &reconcile.Result{}, nil
}
