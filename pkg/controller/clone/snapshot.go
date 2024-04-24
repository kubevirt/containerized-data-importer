package clone

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SnapshotPhaseName is the name of the snapshot phase
const SnapshotPhaseName = "Snapshot"

// SnapshotPhase snapshots a PVC
type SnapshotPhase struct {
	Owner               client.Object
	SourceNamespace     string
	SourceName          string
	TargetName          string
	VolumeSnapshotClass string
	OwnershipLabel      string
	Client              client.Client
	Log                 logr.Logger
	Recorder            record.EventRecorder
}

var _ Phase = &SnapshotPhase{}

// Name returns the name of the phase
func (p *SnapshotPhase) Name() string {
	return SnapshotPhaseName
}

// Reconcile ensures a snapshot is created correctly
func (p *SnapshotPhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	snapshot := &snapshotv1.VolumeSnapshot{}
	exists, err := getResource(ctx, p.Client, p.SourceNamespace, p.TargetName, snapshot)
	if err != nil {
		return nil, err
	}

	if !exists {
		args := &IsSourceClaimReadyArgs{
			Target:          p.Owner,
			SourceNamespace: p.SourceNamespace,
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
			// TODO - maybe make this event based somehow
			return &reconcile.Result{RequeueAfter: 2 * time.Second}, nil
		}

		snapshot, err = p.createSnapshot(ctx)
		if err != nil {
			return nil, err
		}
	}

	if snapshot.Status == nil ||
		snapshot.Status.CreationTime.IsZero() {
		return &reconcile.Result{}, nil
	}

	return nil, nil
}

func (p *SnapshotPhase) createSnapshot(ctx context.Context) (*snapshotv1.VolumeSnapshot, error) {
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.SourceNamespace,
			Name:      p.TargetName,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &p.SourceName,
			},
			VolumeSnapshotClassName: &p.VolumeSnapshotClass,
		},
	}

	AddCommonLabels(snapshot)
	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, snapshot, p.Owner)
	}

	if err := p.Client.Create(ctx, snapshot); err != nil {
		return nil, err
	}

	return snapshot, nil
}
