package clone

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// HostClonePhaseName is the name of the host clone phase
const HostClonePhaseName = "HostClone"

// HostClonePhase creates and monitors a dumb clone operation
type HostClonePhase struct {
	Owner             client.Object
	Namespace         string
	SourceName        string
	DesiredClaim      *corev1.PersistentVolumeClaim
	ImmediateBind     bool
	OwnershipLabel    string
	Preallocation     bool
	PriorityClassName string
	Client            client.Client
	Log               logr.Logger
	Recorder          record.EventRecorder
}

var _ Phase = &HostClonePhase{}

var _ StatusReporter = &HostClonePhase{}

var httpClient *http.Client

func init() {
	httpClient = cc.BuildHTTPClient(httpClient)
}

// Name returns the name of the phase
func (p *HostClonePhase) Name() string {
	return HostClonePhaseName
}

// Status returns the phase status
func (p *HostClonePhase) Status(ctx context.Context) (*PhaseStatus, error) {
	result := &PhaseStatus{}
	pvc := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.DesiredClaim.Name, pvc)
	if err != nil {
		return nil, err
	}

	if !exists {
		return result, nil
	}

	result.Annotations = pvc.Annotations

	podName := pvc.Annotations[cc.AnnCloneSourcePod]
	if podName == "" {
		return result, nil
	}

	args := &cc.ProgressFromClaimArgs{
		Client:       p.Client,
		HTTPClient:   httpClient,
		Claim:        pvc,
		PodNamespace: p.Namespace,
		PodName:      podName,
		OwnerUID:     string(p.Owner.GetUID()),
	}

	progress, err := cc.ProgressFromClaim(ctx, args)
	if err != nil {
		return nil, err
	}

	result.Progress = progress

	return result, nil
}

// Reconcile creates the desired pvc and waits for the operation to complete
func (p *HostClonePhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	actualClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.DesiredClaim.Name, actualClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		actualClaim, err = p.createClaim(ctx)
		if err != nil {
			return nil, err
		}
	}

	if !p.hostCloneComplete(actualClaim) {
		// requeue to update status
		return &reconcile.Result{RequeueAfter: 3 * time.Second}, nil
	}

	return nil, nil
}

func (p *HostClonePhase) createClaim(ctx context.Context) (*corev1.PersistentVolumeClaim, error) {
	claim := p.DesiredClaim.DeepCopy()

	claim.Namespace = p.Namespace
	cc.AddAnnotation(claim, cc.AnnPreallocationRequested, fmt.Sprintf("%t", p.Preallocation))
	cc.AddAnnotation(claim, cc.AnnOwnerUID, string(p.Owner.GetUID()))
	cc.AddAnnotation(claim, cc.AnnPodRestarts, "0")
	cc.AddAnnotation(claim, cc.AnnCloneRequest, fmt.Sprintf("%s/%s", p.Namespace, p.SourceName))
	cc.AddAnnotation(claim, cc.AnnPopulatorKind, cdiv1.VolumeCloneSourceRef)
	cc.AddAnnotation(claim, cc.AnnExcludeFromVeleroBackup, "true")
	cc.AddAnnotation(claim, cc.AnnEventSourceKind, p.Owner.GetObjectKind().GroupVersionKind().Kind)
	cc.AddAnnotation(claim, cc.AnnEventSource, fmt.Sprintf("%s/%s", p.Owner.GetNamespace(), p.Owner.GetName()))
	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, claim, p.Owner)
	}
	if p.ImmediateBind {
		cc.AddAnnotation(claim, cc.AnnImmediateBinding, "")
	}
	if p.PriorityClassName != "" {
		cc.AddAnnotation(claim, cc.AnnPriorityClassName, p.PriorityClassName)
	}

	if err := p.Client.Create(ctx, claim); err != nil {
		checkQuotaExceeded(p.Recorder, p.Owner, err)
		return nil, err
	}

	return claim, nil
}

func (p *HostClonePhase) hostCloneComplete(pvc *corev1.PersistentVolumeClaim) bool {
	// this is awfully lame
	// both the upload controller and clone controller update the PVC status to succeeded
	// but only the clone controller will set the preallocation annotation
	// so we have to wait for that
	if p.Preallocation && pvc.Annotations[cc.AnnPreallocationApplied] != "true" {
		return false
	}
	return pvc.Annotations[cc.AnnPodPhase] == string(cdiv1.Succeeded)
}
