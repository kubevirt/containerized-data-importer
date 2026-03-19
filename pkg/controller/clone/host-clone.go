package clone

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
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

	args := &progressFromClaimArgs{
		Client:       p.Client,
		HTTPClient:   httpClient,
		Claim:        pvc,
		PodNamespace: p.Namespace,
		PodName:      podName,
		OwnerUID:     string(p.Owner.GetUID()),
	}

	progress, err := progressFromClaim(ctx, args)
	if err != nil {
		return nil, err
	}

	result.Progress = progress

	return result, nil
}

// progressFromClaimArgs are the args for ProgressFromClaim
type progressFromClaimArgs struct {
	Client       client.Client
	HTTPClient   *http.Client
	Claim        *corev1.PersistentVolumeClaim
	OwnerUID     string
	PodNamespace string
	PodName      string
}

// progressFromClaim returns the progres
func progressFromClaim(ctx context.Context, args *progressFromClaimArgs) (string, error) {
	// Just set 100.0% if pod is succeeded
	if args.Claim.Annotations[cc.AnnPodPhase] == string(corev1.PodSucceeded) {
		return cc.ProgressDone, nil
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.PodNamespace,
			Name:      args.PodName,
		},
	}
	if err := args.Client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		if k8serrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	// This will only work when the clone source pod is running
	if pod.Status.Phase != corev1.PodRunning {
		return "", nil
	}
	url, err := cc.GetMetricsURL(pod)
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", nil
	}

	// We fetch the clone progress from the clone source pod metrics
	progressReport, err := cc.GetProgressReportFromURL(ctx, url, args.HTTPClient, metrics.CloneProgressMetricName, args.OwnerUID)
	if err != nil {
		return "", err
	}
	if progressReport != "" {
		if f, err := strconv.ParseFloat(progressReport, 64); err == nil {
			return fmt.Sprintf("%.2f%%", f), nil
		}
	}

	return "", nil
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

	targetPvc, err := cc.GetAnnotatedEventSource(ctx, p.Client, actualClaim)
	if err != nil {
		return nil, err
	}
	cc.CopyEvents(actualClaim, targetPvc, p.Client, p.Recorder)

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
	cc.AddLabel(claim, cc.LabelExcludeFromVeleroBackup, "true")

	if err := p.MakeSureTargetPVCHasSufficientSpace(ctx, claim); err != nil {
		return nil, err
	}

	if err := p.Client.Create(ctx, claim); err != nil {
		checkQuotaExceeded(p.Recorder, p.Owner, err)
		return nil, err
	}

	return claim, nil
}

// The purpose of this check is to make sure the target PVC to have sufficient space
// before the cloning actually happens.
func (p *HostClonePhase) MakeSureTargetPVCHasSufficientSpace(ctx context.Context, targetPvc *corev1.PersistentVolumeClaim) error {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	sourcePvcKey := client.ObjectKey{Namespace: p.Namespace, Name: p.SourceName}

	if err := p.Client.Get(ctx, sourcePvcKey, sourcePvc); err != nil {
		return err
	}

	if targetPvc.Spec.Resources.Requests == nil {
		return fmt.Errorf("no target resource request specified")
	}
	targetSize, ok := targetPvc.Spec.Resources.Requests[corev1.ResourceStorage]

	if !ok {
		return fmt.Errorf("no target size specified")
	}

	unInflatedSourceSize := sourcePvc.Spec.Resources.Requests[corev1.ResourceStorage]
	sourceVolumeMode := cc.GetVolumeMode(sourcePvc)
	targetVolumeMode := cc.GetVolumeMode(targetPvc)
	var err error

	// For filesystem source PVCs, we need to get the original DV size to account for
	if sourceVolumeMode == corev1.PersistentVolumeFilesystem {
		original, err := cc.GetHostCloneOriginalSourceDVSize(ctx, p.Client, sourcePvc)
		if err != nil {
			return err
		}
		if !original.IsZero() {
			unInflatedSourceSize = original
		}
	}

	targetSizeUpdated := false
	if unInflatedSourceSize.Cmp(targetSize) >= 0 {
		targetSizeUpdated = true
		targetSize = unInflatedSourceSize
	}

	// if target is filesystem, inflate the original size if target
	if targetVolumeMode == corev1.PersistentVolumeFilesystem && targetSizeUpdated {
		// when only sourc pvc size is available (snapshot case) we have no
		// way to trace back to its original DV size (because the DV probably
		// doesn't exist anymore), so we use source pvc size to just inflate
		// the target size with overhead
		targetSize, err = cc.InflateSizeWithOverhead(ctx, p.Client, unInflatedSourceSize.Value(), &targetPvc.Spec)
		if err != nil {
			return err
		}
	}

	targetPvc.Spec.Resources.Requests[corev1.ResourceStorage] = targetSize
	return nil
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
