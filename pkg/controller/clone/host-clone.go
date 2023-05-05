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

// HostClonePhase creates and monitors a dumb clone operation
type HostClonePhase struct {
	Owner          client.Object
	Namespace      string
	SourceName     string
	DesiredClaim   *corev1.PersistentVolumeClaim
	ImmediateBind  bool
	OwnershipLabel string
	Client         client.Client
	Log            logr.Logger
	Recorder       record.EventRecorder
}

var _ Phase = &HostClonePhase{}

var _ ProgressReporter = &HostClonePhase{}

var httpClient *http.Client

func init() {
	httpClient = cc.BuildHTTPClient(httpClient)
}

// Name returns the name of the phase
func (p *HostClonePhase) Name() string {
	return "HostClone"
}

// Progress returns the progress of the operation as a percentage
func (p *HostClonePhase) Progress(ctx context.Context) (string, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, p.Namespace, p.DesiredClaim.Name, pvc)
	if err != nil {
		return "", err
	}

	podName := pvc.Annotations[cc.AnnCloneSourcePod]
	if !exists || podName == "" {
		return "", nil
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
		return "", err
	}

	return progress, nil
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

	if !hostCloneComplete(actualClaim) {
		// requeue to update status
		return &reconcile.Result{RequeueAfter: 3 * time.Second}, nil
	}

	return nil, nil
}

func (p *HostClonePhase) createClaim(ctx context.Context) (*corev1.PersistentVolumeClaim, error) {
	claim := p.DesiredClaim.DeepCopy()

	claim.Namespace = p.Namespace
	cc.AddAnnotation(claim, cc.AnnOwnerUID, string(p.Owner.GetUID()))
	cc.AddAnnotation(claim, cc.AnnPodRestarts, "0")
	cc.AddAnnotation(claim, cc.AnnCloneRequest, fmt.Sprintf("%s/%s", p.Namespace, p.SourceName))
	cc.AddAnnotation(claim, cc.AnnPopulatorKind, cdiv1.VolumeCloneSourceRef)
	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, claim, p.Owner)
	}
	if p.ImmediateBind {
		cc.AddAnnotation(claim, cc.AnnImmediateBinding, "")
	}

	if err := p.Client.Create(ctx, claim); err != nil {
		checkQuotaExceeded(p.Recorder, p.Owner, err)
		return nil, err
	}

	return claim, nil
}

func hostCloneComplete(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Annotations[cc.AnnPodPhase] == string(cdiv1.Succeeded)
}
