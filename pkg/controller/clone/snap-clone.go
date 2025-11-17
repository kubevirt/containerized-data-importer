package clone

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// SnapshotClonePhaseName is the name of the snapshot clone phase
const SnapshotClonePhaseName = "SnapshotClone"

// SnapshotClonePhase waits for a snapshot to be ready and creates a PVC from it
type SnapshotClonePhase struct {
	Owner          client.Object
	Namespace      string
	SourceName     string
	DesiredClaim   *corev1.PersistentVolumeClaim
	OwnershipLabel string
	Client         client.Client
	Log            logr.Logger
	Recorder       record.EventRecorder
}

var _ Phase = &SnapshotClonePhase{}

// Name returns the name of the phase
func (p *SnapshotClonePhase) Name() string {
	return SnapshotClonePhaseName
}

// Reconcile ensures a snapshot is created correctly
func (p *SnapshotClonePhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.DesiredClaim.Name, pvc)
	if err != nil {
		return nil, err
	}

	if !exists {
		snapshot := &snapshotv1.VolumeSnapshot{}
		exists, err := getResource(ctx, p.Client, p.Namespace, p.SourceName, snapshot)
		if err != nil {
			return nil, err
		}

		if !exists {
			return nil, fmt.Errorf("source snapshot does not exist")
		}

		if !cc.IsSnapshotReady(snapshot) {
			return &reconcile.Result{}, nil
		}

		pvc, err = p.createClaim(ctx, snapshot)
		if err != nil {
			return nil, err
		}
	}

	targetPvc, err := cc.GetAnnotatedEventSource(ctx, p.Client, pvc)
	if err != nil {
		return nil, err
	}
	cc.CopyEvents(pvc, targetPvc, p.Client, p.Recorder)

	done, err := isClaimBoundOrWFFC(ctx, p.Client, pvc)
	if err != nil {
		return nil, err
	}

	if !done {
		return &reconcile.Result{}, nil
	}

	return nil, nil
}

func (p *SnapshotClonePhase) createClaim(ctx context.Context, snapshot *snapshotv1.VolumeSnapshot) (*corev1.PersistentVolumeClaim, error) {
	claim := p.DesiredClaim.DeepCopy()
	claim.Namespace = p.Namespace
	claim.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: ptr.To[string]("snapshot.storage.k8s.io"),
		Kind:     "VolumeSnapshot",
		Name:     p.SourceName,
	}

	if snapshot.Status == nil || snapshot.Status.RestoreSize == nil {
		return nil, fmt.Errorf("snapshot missing restoresize")
	}

	// 0 restore size is a special case, provisioners that do that seem to allow restoring to bigger pvcs
	rs := snapshot.Status.RestoreSize
	if !rs.IsZero() {
		p.Log.V(3).Info("setting desired pvc request size to", "restoreSize", *rs)
		claim.Spec.Resources.Requests[corev1.ResourceStorage] = *rs
	}

	cc.AddAnnotation(claim, cc.AnnEventSourceKind, p.Owner.GetObjectKind().GroupVersionKind().Kind)
	cc.AddAnnotation(claim, cc.AnnEventSource, fmt.Sprintf("%s/%s", p.Owner.GetNamespace(), p.Owner.GetName()))
	cc.AddAnnotation(claim, cc.AnnPopulatorKind, cdiv1.VolumeCloneSourceRef)
	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, claim, p.Owner)
	}
	cc.AddLabel(claim, cc.LabelExcludeFromVeleroBackup, "true")

	if err := p.Client.Create(ctx, claim); err != nil {
		checkQuotaExceeded(p.Recorder, p.Owner, err)
		return nil, err
	}

	return claim, nil
}
