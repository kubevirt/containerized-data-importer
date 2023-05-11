package clone

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// Planner plans clone operations
type Planner struct {
	RootObjectType  client.ObjectList
	OwnershipLabel  string
	UIDField        string
	Image           string
	PullPolicy      corev1.PullPolicy
	InstallerLabels map[string]string
	Client          client.Client
	Recorder        record.EventRecorder
	Controller      controller.Controller

	watchingCore      bool
	watchingSnapshots bool
	watchMutex        sync.Mutex
}

// Phase is the interface implemeented by all clone phases
type Phase interface {
	Name() string
	Reconcile(context.Context) (*reconcile.Result, error)
}

// ProgressReporter allows a phase to report progress
type ProgressReporter interface {
	Progress(context.Context) (string, error)
}

// list of all possible (core) types created
var coreTypesCreated = []client.Object{
	&corev1.PersistentVolumeClaim{},
	&corev1.Pod{},
}

// AddCoreWatches watches "core" types
func (p *Planner) AddCoreWatches(log logr.Logger) error {
	p.watchMutex.Lock()
	defer p.watchMutex.Unlock()
	if p.watchingCore {
		return nil
	}

	for _, obj := range coreTypesCreated {
		if err := p.watchOwned(log, obj); err != nil {
			return err
		}
	}

	p.watchingCore = true

	return nil
}

// ChooseStrategyArgs are args for ChooseStrategy function
type ChooseStrategyArgs struct {
	Log         logr.Logger
	TargetClaim *corev1.PersistentVolumeClaim
	DataSource  *cdiv1.VolumeCloneSource
}

// ChooseStrategy picks the strategy for a clone op
func (p *Planner) ChooseStrategy(ctx context.Context, args *ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error) {
	if IsDataSourcePVC(args.DataSource.Spec.Source.Kind) {
		args.Log.V(3).Info("Getting strategy for PVC source")
		return p.strategyForSourcePVC(ctx, args)
	}

	return nil, fmt.Errorf("unsupported datasource")
}

// PlanArgs are args to plan clone op for populator
type PlanArgs struct {
	Log         logr.Logger
	TargetClaim *corev1.PersistentVolumeClaim
	DataSource  *cdiv1.VolumeCloneSource
	Strategy    cdiv1.CDICloneStrategy
}

// Plan creates phases for populator clone
func (p *Planner) Plan(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	if args.Strategy == cdiv1.CloneStrategySnapshot {
		if err := p.watchSnapshots(ctx, args.Log); err != nil {
			return nil, err
		}
	}

	if IsDataSourcePVC(args.DataSource.Spec.Source.Kind) {
		if args.Strategy == cdiv1.CloneStrategyHostAssisted {
			args.Log.V(3).Info("Planning host assisted clone from PVC")

			return p.planHostAssistedFromPVC(ctx, args)
		} else if args.Strategy == cdiv1.CloneStrategySnapshot {
			args.Log.V(3).Info("Planning snapshot clone from PVC")

			return p.planSnapshotFromPVC(ctx, args)
		} else if args.Strategy == cdiv1.CloneStrategyCsiClone {
			args.Log.V(3).Info("Planning csi clone from PVC")

			return p.planCSIClone(ctx, args)
		}
	}

	return nil, fmt.Errorf("unknown strategy/source %s", string(args.Strategy))
}

func (p *Planner) watchSnapshots(ctx context.Context, log logr.Logger) error {
	p.watchMutex.Lock()
	defer p.watchMutex.Unlock()
	if p.watchingSnapshots {
		return nil
	}

	vsl := &snapshotv1.VolumeSnapshotList{}
	lo := &client.ListOptions{Limit: 1}
	if err := p.Client.List(ctx, vsl, lo); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
	}

	if err := p.watchOwned(log, &snapshotv1.VolumeSnapshot{}); err != nil {
		return err
	}

	log.V(3).Info("watching volumesnapshots now")
	p.watchingSnapshots = true

	return nil
}

func (p *Planner) watchOwned(log logr.Logger, obj client.Object) error {
	objList := p.RootObjectType.DeepCopyObject().(client.ObjectList)
	if err := p.Controller.Watch(&source.Kind{Type: obj}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) (reqs []reconcile.Request) {
			uid, ok := obj.GetLabels()[p.OwnershipLabel]
			if !ok {
				return
			}
			matchingFields := client.MatchingFields{
				p.UIDField: uid,
			}
			if err := p.Client.List(context.Background(), objList, matchingFields); err != nil {
				log.Error(err, "Unable to list resource", "matchingFields", matchingFields)
				return
			}
			sv := reflect.ValueOf(objList).Elem()
			iv := sv.FieldByName("Items")
			for i := 0; i < iv.Len(); i++ {
				o := iv.Index(i).Addr().Interface().(client.Object)
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: o.GetNamespace(),
						Name:      o.GetName(),
					},
				})
			}
			return
		}),
	); err != nil {
		return err
	}
	return nil
}

func (p *Planner) strategyForSourcePVC(ctx context.Context, args *ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error) {
	if ok, err := p.validateTargetStorageClassAssignment(ctx, args); !ok || err != nil {
		return nil, err
	}

	sourceClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		// TODO EVENT
		//Event{
		//	eventType: corev1.EventTypeWarning,
		//	reason:    CloneWithoutSource,
		//	message:   fmt.Sprintf(MessageCloneWithoutSource, "pvc", datavolume.Spec.Source.PVC.Name),
		//})

		args.Log.V(3).Info("Source PVC does not exist, cannot compute strategy")
		return nil, nil
	}

	if sourceClaim.Spec.VolumeName == "" {
		args.Log.V(3).Info("Source PVC is not bound, too early to compute strategy")
		return nil, nil
	}

	if err = p.validateSourcePVC(args, sourceClaim); err != nil {
		// TODO EVENT
		//r.recorder.Event(datavolume, corev1.EventTypeWarning, CloneValidationFailed, MessageCloneValidationFailed)
		//return false, err
		args.Log.V(1).Info("Validation failed", "target", args.TargetClaim, "source", sourceClaim)
		return nil, err
	}

	strategy := cdiv1.CloneStrategySnapshot
	cs, err := GetGlobalCloneStrategyOverride(ctx, p.Client)
	if err != nil {
		return nil, err
	}

	if cs != nil {
		strategy = *cs
	} else if args.TargetClaim.Spec.StorageClassName != nil {
		sp := &cdiv1.StorageProfile{}
		exists, err := getResource(ctx, p.Client, "", *args.TargetClaim.Spec.StorageClassName, sp)
		if err != nil {
			return nil, err
		}

		if !exists {
			args.Log.V(3).Info("missing storageprofile for", "name", *args.TargetClaim.Spec.StorageClassName)
		}

		if exists && sp.Status.CloneStrategy != nil {
			strategy = *sp.Status.CloneStrategy
		}
	}

	if strategy == cdiv1.CloneStrategySnapshot {
		n, err := GetCompatibleVolumeSnapshotClass(ctx, p.Client, sourceClaim, args.TargetClaim)
		if err != nil {
			return nil, err
		}

		if n == nil {
			strategy = cdiv1.CloneStrategyHostAssisted
		}
	}

	if strategy == cdiv1.CloneStrategySnapshot ||
		strategy == cdiv1.CloneStrategyCsiClone {
		ok, err := p.validateAdvancedClonePVC(ctx, args, sourceClaim)
		if err != nil {
			return nil, err
		}

		if !ok {
			strategy = cdiv1.CloneStrategyHostAssisted
		}
	}

	return &strategy, nil
}

func (p *Planner) validateTargetStorageClassAssignment(ctx context.Context, args *ChooseStrategyArgs) (bool, error) {
	if args.TargetClaim.Spec.StorageClassName == nil {
		args.Log.V(3).Info("Target PVC has nil storage class, cannot compute strategy")
		return false, nil
	}

	if *args.TargetClaim.Spec.StorageClassName == "" {
		args.Log.V(3).Info("Target PVC has \"\" storage class, cannot compute strategy")
		return false, fmt.Errorf("claim has emptystring storageclass, will not work")
	}

	_, err := MustGetStorageClassForClaim(ctx, p.Client, args.TargetClaim)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *Planner) validateSourcePVC(args *ChooseStrategyArgs, sourceClaim *corev1.PersistentVolumeClaim) error {
	if err := ValidateContentTypes(sourceClaim, args.DataSource.Spec.ContentType); err != nil {
		return err
	}

	if err := cc.ValidateRequestedCloneSize(sourceClaim.Spec.Resources, args.TargetClaim.Spec.Resources); err != nil {
		return err
	}

	return nil
}

func (p *Planner) validateAdvancedClonePVC(ctx context.Context, args *ChooseStrategyArgs, sourceClaim *corev1.PersistentVolumeClaim) (bool, error) {
	if !SameVolumeMode(sourceClaim, args.TargetClaim) {
		args.Log.V(3).Info("volume modes not compatible for advanced clone")
		return false, nil
	}

	sc, err := MustGetStorageClassForClaim(ctx, p.Client, args.TargetClaim)
	if err != nil {
		return false, err
	}

	srcCapacity, hasSrcCapacity := sourceClaim.Status.Capacity[corev1.ResourceStorage]
	targetRequest, hasTargetRequest := args.TargetClaim.Spec.Resources.Requests[corev1.ResourceStorage]
	allowExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
	if !hasSrcCapacity || !hasTargetRequest {
		return false, fmt.Errorf("source/target size info missing")
	}

	if srcCapacity.Cmp(targetRequest) < 0 && !allowExpansion {
		args.Log.V(3).Info("advanced clone not possible, no volume expansion")
		return false, nil
	}

	return true, nil
}

func (p *Planner) planHostAssistedFromPVC(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)

	// only inflate for kubevirt content type
	// maybe do this for smart clone/csi clone too too?
	// shouldn't be necessary though
	if args.DataSource.Spec.ContentType == "" || args.DataSource.Spec.ContentType == cdiv1.DataVolumeKubeVirt {
		ds := desiredClaim.Spec.Resources.Requests[corev1.ResourceStorage]
		is, err := cc.InflateSizeWithOverhead(ctx, p.Client, ds.Value(), &args.TargetClaim.Spec)
		if err != nil {
			return nil, err
		}
		desiredClaim.Spec.Resources.Requests[corev1.ResourceStorage] = is
	}

	ct := cdiv1.DataVolumeKubeVirt
	if args.DataSource.Spec.ContentType != "" {
		ct = args.DataSource.Spec.ContentType
	}

	hcp := &HostClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      args.DataSource.Namespace,
		SourceName:     args.DataSource.Spec.Source.Name,
		DesiredClaim:   desiredClaim,
		ImmediateBind:  true,
		OwnershipLabel: p.OwnershipLabel,
		ContentType:    string(ct),
		Preallocation:  cc.GetPreallocation(ctx, p.Client, args.DataSource.Spec.Preallocation),
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	rp := &RebindPhase{
		SourceNamespace: desiredClaim.Namespace,
		SourceName:      desiredClaim.Name,
		TargetNamespace: args.TargetClaim.Namespace,
		TargetName:      args.TargetClaim.Name,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	return []Phase{hcp, rp}, nil
}

func (p *Planner) planSnapshotFromPVC(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	sourceClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("source claim does not exist")
	}

	vsc, err := GetCompatibleVolumeSnapshotClass(ctx, p.Client, sourceClaim, args.TargetClaim)
	if err != nil {
		return nil, err
	}

	if vsc == nil {
		return nil, fmt.Errorf("no compatible volumesnapshotclass")
	}

	sp := &SnapshotPhase{
		Owner:               args.TargetClaim,
		SourceNamespace:     args.DataSource.Namespace,
		SourceName:          args.DataSource.Spec.Source.Name,
		TargetName:          fmt.Sprintf("tmp-snapshot-%s", string(args.TargetClaim.UID)),
		VolumeSnapshotClass: *vsc,
		OwnershipLabel:      p.OwnershipLabel,
		Client:              p.Client,
		Log:                 args.Log,
	}

	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)
	cfsp := &SnapshotClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      args.DataSource.Namespace,
		SourceName:     sp.TargetName,
		DesiredClaim:   desiredClaim.DeepCopy(),
		OwnershipLabel: p.OwnershipLabel,
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	pcp := &PrepClaimPhase{
		Owner:           args.TargetClaim,
		DesiredClaim:    desiredClaim.DeepCopy(),
		Image:           p.Image,
		PullPolicy:      p.PullPolicy,
		InstallerLabels: p.InstallerLabels,
		OwnershipLabel:  p.OwnershipLabel,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	rp := &RebindPhase{
		SourceNamespace: desiredClaim.Namespace,
		SourceName:      desiredClaim.Name,
		TargetNamespace: args.TargetClaim.Namespace,
		TargetName:      args.TargetClaim.Name,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	return []Phase{sp, cfsp, pcp, rp}, nil
}

func (p *Planner) planCSIClone(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)
	cp := &CSIClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      args.DataSource.Namespace,
		SourceName:     args.DataSource.Spec.Source.Name,
		DesiredClaim:   desiredClaim.DeepCopy(),
		OwnershipLabel: p.OwnershipLabel,
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	pcp := &PrepClaimPhase{
		Owner:           args.TargetClaim,
		DesiredClaim:    desiredClaim.DeepCopy(),
		Image:           p.Image,
		PullPolicy:      p.PullPolicy,
		InstallerLabels: p.InstallerLabels,
		OwnershipLabel:  p.OwnershipLabel,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	rp := &RebindPhase{
		SourceNamespace: desiredClaim.Namespace,
		SourceName:      desiredClaim.Name,
		TargetNamespace: args.TargetClaim.Namespace,
		TargetName:      args.TargetClaim.Name,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	return []Phase{cp, pcp, rp}, nil
}

func createDesiredClaim(namespace string, targetClaim *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	targetCpy := targetClaim.DeepCopy()
	desiredClaim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        fmt.Sprintf("tmp-pvc-%s", string(targetClaim.UID)),
			Labels:      targetCpy.Labels,
			Annotations: targetCpy.Annotations,
		},
		Spec: targetCpy.Spec,
	}
	desiredClaim.Spec.DataSource = nil
	desiredClaim.Spec.DataSourceRef = nil

	return desiredClaim
}
