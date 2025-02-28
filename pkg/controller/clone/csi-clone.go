package clone

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// CSIClonePhaseName is the name of the csi clone phase
const CSIClonePhaseName = "CSIClone"

// CSIClonePhase is responsible for csi cloning a pvc
type CSIClonePhase struct {
	Owner          client.Object
	Namespace      string
	SourceName     string
	DesiredClaim   *corev1.PersistentVolumeClaim
	OwnershipLabel string
	Client         client.Client
	Log            logr.Logger
	Recorder       record.EventRecorder
}

var _ Phase = &CSIClonePhase{}

// Name returns the name of the phase
func (p *CSIClonePhase) Name() string {
	return CSIClonePhaseName
}

// Reconcile ensures a csi cloned pvc is created correctly
func (p *CSIClonePhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.DesiredClaim.Name, pvc)
	if err != nil {
		return nil, err
	}

	if !exists {
		args := &IsSourceClaimReadyArgs{
			Target:          p.Owner,
			SourceNamespace: p.Namespace,
			SourceName:      p.SourceName,
			Client:          p.Client,
			Log:             p.Log,
			Recorder:        p.Recorder,
		}

		ready, err := IsSourceClaimReady(ctx, args)
		if err != nil {
			return nil, err
		}

		if !ready {
			return &reconcile.Result{}, nil
		}

		pvc, err = p.createClaim(ctx)
		if err != nil {
			return nil, err
		}
	}

	done, err := isClaimBoundOrWFFC(ctx, p.Client, pvc)
	if err != nil {
		return nil, err
	}

	if !done {
		return &reconcile.Result{}, nil
	}

	return nil, nil
}

func (p *CSIClonePhase) createClaim(ctx context.Context) (*corev1.PersistentVolumeClaim, error) {
	sourceClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.SourceName, sourceClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("source claim does not exist")
	}

	desiredClaim := p.DesiredClaim.DeepCopy()
	desiredClaim.Namespace = sourceClaim.Namespace
	desiredClaim.Spec.DataSourceRef = &corev1.TypedObjectReference{
		Kind: "PersistentVolumeClaim",
		Name: sourceClaim.Name,
	}

	sourceSize := sourceClaim.Status.Capacity[corev1.ResourceStorage]
	p.Log.V(3).Info("setting desired pvc request size to", "restoreSize", sourceSize)
	desiredClaim.Spec.Resources.Requests[corev1.ResourceStorage] = sourceSize

	cc.AddAnnotation(desiredClaim, cc.AnnPopulatorKind, cdiv1.VolumeCloneSourceRef)
	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, desiredClaim, p.Owner)
	}
	cc.AddLabel(desiredClaim, cc.LabelExcludeFromVeleroBackup, "true")

	if err := p.Client.Create(ctx, desiredClaim); err != nil {
		checkQuotaExceeded(p.Recorder, p.Owner, err)
		return nil, err
	}

	return desiredClaim, nil
}
