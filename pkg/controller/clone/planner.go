package clone

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

const (
	// CloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (reason)
	CloneValidationFailed = "CloneValidationFailed"

	// MessageCloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (message)
	MessageCloneValidationFailed = "The clone doesn't meet the validation requirements"

	// CloneWithoutSource reports that the source of a clone doesn't exists (reason)
	CloneWithoutSource = "CloneWithoutSource"

	// MessageCloneWithoutSource reports that the source of a clone doesn't exists (message)
	MessageCloneWithoutSource = "The source %s %s doesn't exist"

	// NoVolumeSnapshotClass reports that no compatible volumesnapshotclass was found (reason)
	NoVolumeSnapshotClass = "NoVolumeSnapshotClass"

	// MessageNoVolumeSnapshotClass reports that no compatible volumesnapshotclass was found (message)
	MessageNoVolumeSnapshotClass = "No compatible volumesnapshotclass found"

	// IncompatibleVolumeModes reports that the volume modes of source and target are incompatible (reason)
	IncompatibleVolumeModes = "IncompatibleVolumeModes"

	// MessageIncompatibleVolumeModes reports that the volume modes of source and target are incompatible (message)
	MessageIncompatibleVolumeModes = "The volume modes of source and target are incompatible"

	// NoVolumeExpansion reports that no volume expansion is possible (reason)
	NoVolumeExpansion = "NoVolumeExpansion"

	// MessageNoVolumeExpansion reports that no volume expansion is possible (message)
	MessageNoVolumeExpansion = "No volume expansion is possible"
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

// Phase is the interface implemented by all clone phases
type Phase interface {
	Name() string
	Reconcile(context.Context) (*reconcile.Result, error)
}

// PhaseStatus contains phase status data
type PhaseStatus struct {
	Progress    string
	Annotations map[string]string
}

// StatusReporter allows a phase to report status
type StatusReporter interface {
	Status(context.Context) (*PhaseStatus, error)
}

// list of all possible (core) types created
var coreTypesCreated = []client.Object{
	&corev1.PersistentVolumeClaim{},
	&corev1.Pod{},
}

// all types that may have been created
var listTypesToDelete = []client.ObjectList{
	&corev1.PersistentVolumeClaimList{},
	&corev1.PodList{},
	&snapshotv1.VolumeSnapshotList{},
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
		return p.computeStrategyForSourcePVC(ctx, args)
	}
	if IsDataSourceSnapshot(args.DataSource.Spec.Source.Kind) {
		args.Log.V(3).Info("Getting strategy for Snapshot source")
		return p.computeStrategyForSourceSnapshot(ctx, args)
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

	if IsDataSourceSnapshot(args.DataSource.Spec.Source.Kind) {
		if args.Strategy == cdiv1.CloneStrategyHostAssisted {
			args.Log.V(3).Info("Planning host assisted clone from Snapshot")

			return p.planHostAssistedFromSnapshot(ctx, args)
		} else if args.Strategy == cdiv1.CloneStrategySnapshot {
			args.Log.V(3).Info("Planning Smart clone from Snapshot")

			return p.planSmartCloneFromSnapshot(ctx, args)
		}
	}

	return nil, fmt.Errorf("unknown strategy/source %s", string(args.Strategy))
}

// Cleanup cleans up after a clone op
func (p *Planner) Cleanup(ctx context.Context, log logr.Logger, owner client.Object) error {
	log.V(3).Info("Cleaning up for obj", "obj", owner)

	for _, lt := range listTypesToDelete {
		ls, err := labels.Parse(fmt.Sprintf("%s=%s", p.OwnershipLabel, string(owner.GetUID())))
		if err != nil {
			return err
		}

		lo := &client.ListOptions{
			LabelSelector: ls,
		}
		if err := cc.BulkDeleteResources(ctx, p.Client, lt, lo); err != nil {
			return err
		}
	}

	return nil
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

func (p *Planner) computeStrategyForSourcePVC(ctx context.Context, args *ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error) {
	if ok, err := p.validateTargetStorageClassAssignment(ctx, args); !ok || err != nil {
		return nil, err
	}

	sourceClaim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceClaim)
	if err != nil {
		return nil, err
	}

	if !exists {
		message := fmt.Sprintf(MessageCloneWithoutSource, "pvc", args.DataSource.Spec.Source.Name)
		p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, CloneWithoutSource, message)
		args.Log.V(3).Info("Source PVC does not exist, cannot compute strategy")
		return nil, nil
	}

	if err = p.validateSourcePVC(args, sourceClaim); err != nil {
		p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, CloneValidationFailed, MessageCloneValidationFailed)
		args.Log.V(3).Info("Validation failed", "target", args.TargetClaim, "source", sourceClaim)
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
			p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, NoVolumeSnapshotClass, MessageNoVolumeSnapshotClass)
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

func (p *Planner) computeStrategyForSourceSnapshot(ctx context.Context, args *ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error) {
	// Defaulting to host-assisted clone since it has fewer requirements
	strategy := cdiv1.CloneStrategyHostAssisted

	if ok, err := p.validateTargetStorageClassAssignment(ctx, args); !ok || err != nil {
		return nil, err
	}

	// Check that snapshot exists
	sourceSnapshot := &snapshotv1.VolumeSnapshot{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceSnapshot)
	if err != nil {
		return nil, err
	}
	if !exists {
		message := fmt.Sprintf(MessageCloneWithoutSource, "snapshot", args.DataSource.Spec.Source.Name)
		p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, CloneWithoutSource, message)
		args.Log.V(3).Info("Source Snapshot does not exist, cannot compute strategy")
		return nil, nil
	}

	// Do snapshot and storage class validation
	targetStorageClass, err := GetStorageClassForClaim(ctx, p.Client, args.TargetClaim)
	if err != nil {
		return nil, err
	}
	if targetStorageClass == nil {
		return nil, fmt.Errorf("target claim's storageclass doesn't exist, clone will not work")
	}
	valid, err := cc.ValidateSnapshotCloneProvisioners(ctx, p.Client, sourceSnapshot, targetStorageClass)
	if err != nil {
		return nil, err
	}
	if !valid {
		args.Log.V(3).Info("Provisioner differs, need to fall back to host assisted")
		return &strategy, nil
	}

	// Lastly, do size validation to determine whether to use dumb or smart cloning
	valid, err = cc.ValidateSnapshotCloneSize(sourceSnapshot, &args.TargetClaim.Spec, targetStorageClass, args.Log)
	if err != nil {
		return nil, err
	}
	if valid {
		strategy = cdiv1.CloneStrategySnapshot
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

	sc, err := GetStorageClassForClaim(ctx, p.Client, args.TargetClaim)
	if err != nil {
		return false, err
	}

	if sc == nil {
		args.Log.V(3).Info("Target PVC has no storage class, cannot compute strategy")
		return false, fmt.Errorf("target storage class not found")
	}

	return true, nil
}

func (p *Planner) validateSourcePVC(args *ChooseStrategyArgs, sourceClaim *corev1.PersistentVolumeClaim) error {
	_, permissive := args.TargetClaim.Annotations[cc.AnnPermissiveClone]
	if permissive {
		args.Log.V(3).Info("permissive clone annotation found, skipping size validation")
		return nil
	}

	if err := cc.ValidateRequestedCloneSize(sourceClaim.Spec.Resources, args.TargetClaim.Spec.Resources); err != nil {
		p.Recorder.Eventf(args.TargetClaim, corev1.EventTypeWarning, cc.ErrIncompatiblePVC, err.Error())
		return err
	}

	return nil
}

func (p *Planner) validateAdvancedClonePVC(ctx context.Context, args *ChooseStrategyArgs, sourceClaim *corev1.PersistentVolumeClaim) (bool, error) {
	if !SameVolumeMode(sourceClaim, args.TargetClaim) {
		p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, IncompatibleVolumeModes, MessageIncompatibleVolumeModes)
		args.Log.V(3).Info("volume modes not compatible for advanced clone")
		return false, nil
	}

	sc, err := GetStorageClassForClaim(ctx, p.Client, args.TargetClaim)
	if err != nil {
		return false, err
	}

	if sc == nil {
		args.Log.V(3).Info("target storage class not found")
		return false, fmt.Errorf("target storage class not found")
	}

	srcCapacity, hasSrcCapacity := sourceClaim.Status.Capacity[corev1.ResourceStorage]
	targetRequest, hasTargetRequest := args.TargetClaim.Spec.Resources.Requests[corev1.ResourceStorage]
	allowExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
	if !hasSrcCapacity || !hasTargetRequest {
		return false, fmt.Errorf("source/target size info missing")
	}

	if srcCapacity.Cmp(targetRequest) < 0 && !allowExpansion {
		p.Recorder.Event(args.TargetClaim, corev1.EventTypeWarning, NoVolumeExpansion, MessageNoVolumeExpansion)
		args.Log.V(3).Info("advanced clone not possible, no volume expansion")
		return false, nil
	}

	return true, nil
}

func (p *Planner) planHostAssistedFromPVC(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)

	hcp := &HostClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      args.DataSource.Namespace,
		SourceName:     args.DataSource.Spec.Source.Name,
		DesiredClaim:   desiredClaim,
		ImmediateBind:  true,
		OwnershipLabel: p.OwnershipLabel,
		Preallocation:  cc.GetPreallocation(ctx, p.Client, args.DataSource.Spec.Preallocation),
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	if args.DataSource.Spec.PriorityClassName != nil {
		hcp.PriorityClassName = *args.DataSource.Spec.PriorityClassName
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

func (p *Planner) planHostAssistedFromSnapshot(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	sourceSnapshot := &snapshotv1.VolumeSnapshot{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceSnapshot)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("source claim does not exist")
	}

	sourceClaimForDumbClone, err := createTempSourceClaim(ctx, args.DataSource.Namespace, args.TargetClaim, sourceSnapshot, p.Client)
	if err != nil {
		return nil, err
	}
	cfsp := &SnapshotClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      args.DataSource.Namespace,
		SourceName:     args.DataSource.Spec.Source.Name,
		DesiredClaim:   sourceClaimForDumbClone,
		OwnershipLabel: p.OwnershipLabel,
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	pcp := &PrepClaimPhase{
		Owner:           args.TargetClaim,
		DesiredClaim:    sourceClaimForDumbClone.DeepCopy(),
		Image:           p.Image,
		PullPolicy:      p.PullPolicy,
		InstallerLabels: p.InstallerLabels,
		OwnershipLabel:  p.OwnershipLabel,
		Client:          p.Client,
		Log:             args.Log,
		Recorder:        p.Recorder,
	}

	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)

	hcp := &HostClonePhase{
		Owner:          args.TargetClaim,
		Namespace:      sourceClaimForDumbClone.Namespace,
		SourceName:     sourceClaimForDumbClone.Name,
		DesiredClaim:   desiredClaim,
		ImmediateBind:  true,
		OwnershipLabel: p.OwnershipLabel,
		Client:         p.Client,
		Log:            args.Log,
		Recorder:       p.Recorder,
	}

	if args.DataSource.Spec.PriorityClassName != nil {
		hcp.PriorityClassName = *args.DataSource.Spec.PriorityClassName
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

	return []Phase{cfsp, pcp, hcp, rp}, nil
}

func (p *Planner) planSmartCloneFromSnapshot(ctx context.Context, args *PlanArgs) ([]Phase, error) {
	sourceSnapshot := &snapshotv1.VolumeSnapshot{}
	exists, err := getResource(ctx, p.Client, args.DataSource.Namespace, args.DataSource.Spec.Source.Name, sourceSnapshot)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("source claim does not exist")
	}

	desiredClaim := createDesiredClaim(args.DataSource.Namespace, args.TargetClaim)
	cfsp := &SnapshotClonePhase{
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

	return []Phase{cfsp, pcp, rp}, nil
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
		Recorder:            p.Recorder,
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

func createTempSourceClaim(ctx context.Context, namespace string, targetClaim *corev1.PersistentVolumeClaim, snapshot *snapshotv1.VolumeSnapshot, client client.Client) (*corev1.PersistentVolumeClaim, error) {
	scName, err := getStorageClassNameForTempSourceClaim(ctx, snapshot, client)
	if err != nil {
		return nil, err
	}
	// Get the appropriate size from the snapshot
	targetCpy := targetClaim.DeepCopy()
	if snapshot.Status == nil || snapshot.Status.RestoreSize == nil || snapshot.Status.RestoreSize.Sign() == -1 {
		return nil, fmt.Errorf("snapshot has no RestoreSize")
	}
	restoreSize := snapshot.Status.RestoreSize
	if restoreSize.IsZero() {
		reqSize := targetCpy.Spec.Resources.Requests[corev1.ResourceStorage]
		restoreSize = &reqSize
	}

	desiredClaim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        fmt.Sprintf("tmp-source-pvc-%s", string(targetClaim.UID)),
			Labels:      targetCpy.Labels,
			Annotations: targetCpy.Annotations,
		},
		Spec: targetCpy.Spec,
	}
	desiredClaim.Spec.DataSource = nil
	desiredClaim.Spec.DataSourceRef = nil
	desiredClaim.Spec.StorageClassName = &scName
	desiredClaim.Spec.Resources.Requests[corev1.ResourceStorage] = *restoreSize

	return desiredClaim, nil
}

func getStorageClassNameForTempSourceClaim(ctx context.Context, snapshot *snapshotv1.VolumeSnapshot, client client.Client) (string, error) {
	var matches []string

	if snapshot.Status == nil || snapshot.Status.BoundVolumeSnapshotContentName == nil {
		return "", fmt.Errorf("volumeSnapshotContent name not found")
	}
	volumeSnapshotContent := &snapshotv1.VolumeSnapshotContent{}
	if err := client.Get(ctx, types.NamespacedName{Name: *snapshot.Status.BoundVolumeSnapshotContentName}, volumeSnapshotContent); err != nil {
		return "", err
	}
	// Attempting to get a storageClass compatible with the source snapshot
	storageClasses := &storagev1.StorageClassList{}
	if err := client.List(ctx, storageClasses); err != nil {
		return "", err
	}
	for _, storageClass := range storageClasses.Items {
		if storageClass.Provisioner == volumeSnapshotContent.Spec.Driver {
			matches = append(matches, storageClass.Name)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("unable to find a valid storage class for the temporal source claim")
	}
	sort.Strings(matches)
	return matches[0], nil
}
