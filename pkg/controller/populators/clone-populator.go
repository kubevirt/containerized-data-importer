/*
Copyright 2023 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package populators

import (
	"context"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller/clone"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

const (
	// LabelOwnedByUID label sets the relationship between a resource and "owner"
	LabelOwnedByUID = "cdi.kubevirt.io/OwnedByUID"

	// AnnClonePhase tracks the status of the clone op
	AnnClonePhase = "cdi.kubevirt.io/clonePhase"

	// AnnCloneError has the error string for error phase
	AnnCloneError = "cdi.kubevirt.io/cloneError"

	// AnnDataSourceNamespace has the namespace of the DataSource
	// this will be deprecated when cross namespace datasource goes beta
	AnnDataSourceNamespace = "cdi.kubevirt.io/dataSourceNamespace"

	clonePopulatorName = "clone-populator"

	cloneFinalizer = "cdi.kubevirt.io/clonePopulator"
)

var desiredCloneAnnotations = map[string]struct{}{
	cc.AnnPreallocationApplied: {},
	cc.AnnCloneOf:              {},
}

// Planner is an interface to mock out planner implementation for testing
type Planner interface {
	ChooseStrategy(context.Context, *clone.ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error)
	Plan(context.Context, *clone.PlanArgs) ([]clone.Phase, error)
	Cleanup(context.Context, logr.Logger, client.Object) error
}

// ClonePopulatorReconciler reconciles PVCs with VolumeCloneSources
type ClonePopulatorReconciler struct {
	ReconcilerBase
	planner             Planner
	multiTokenValidator *cc.MultiTokenValidator
}

// NewClonePopulator creates a new instance of the clone-populator controller
func NewClonePopulator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	clonerImage string,
	pullPolicy string,
	installerLabels map[string]string,
	publicKey *rsa.PublicKey,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &ClonePopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(clonePopulatorName),
			recorder:        mgr.GetEventRecorderFor(clonePopulatorName),
			featureGates:    featuregates.NewFeatureGates(client),
			sourceKind:      cdiv1.VolumeCloneSourceRef,
			installerLabels: installerLabels,
		},
		multiTokenValidator: cc.NewMultiTokenValidator(publicKey),
	}

	clonePopulator, err := controller.New(clonePopulatorName, mgr, controller.Options{
		MaxConcurrentReconciles: 5,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}

	planner := &clone.Planner{
		RootObjectType:  &corev1.PersistentVolumeClaimList{},
		OwnershipLabel:  LabelOwnedByUID,
		UIDField:        uidField,
		Image:           clonerImage,
		PullPolicy:      corev1.PullPolicy(pullPolicy),
		InstallerLabels: installerLabels,
		Client:          reconciler.client,
		Recorder:        reconciler.recorder,
		Controller:      clonePopulator,
	}
	reconciler.planner = planner

	if err := addCommonPopulatorsWatches(mgr, clonePopulator, log, cdiv1.VolumeCloneSourceRef, &cdiv1.VolumeCloneSource{}); err != nil {
		return nil, err
	}

	if err := planner.AddCoreWatches(reconciler.log); err != nil {
		return nil, err
	}

	return clonePopulator, nil
}

// Reconcile the reconcile loop for the PVC with DataSourceRef of VolumeCloneSource kind
func (r *ClonePopulatorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Clone Source PVC")

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(ctx, req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// don't think this should happen but better safe than sorry
	if !IsPVCDataSourceRefKind(pvc, cdiv1.VolumeCloneSourceRef) {
		return reconcile.Result{}, nil
	}

	hasFinalizer := cc.HasFinalizer(pvc, cloneFinalizer)
	isBound := cc.IsBound(pvc)
	isDeleted := !pvc.DeletionTimestamp.IsZero()
	isSucceeded := isClonePhaseSucceeded(pvc)

	log.V(3).Info("pvc state", "hasFinalizer", hasFinalizer,
		"isBound", isBound, "isDeleted", isDeleted, "isSucceeded", isSucceeded)

	if !isDeleted && !isSucceeded {
		return r.reconcilePending(ctx, log, pvc, isBound)
	}

	if hasFinalizer {
		return r.reconcileDone(ctx, log, pvc)
	}

	return reconcile.Result{}, nil
}

func (r *ClonePopulatorReconciler) reconcilePending(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, statusOnly bool) (reconcile.Result, error) {
	ready, _, err := claimReadyForPopulation(ctx, r.client, pvc)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	if !ready {
		log.V(3).Info("claim not ready for population, exiting")
		return reconcile.Result{}, r.updateClonePhasePending(ctx, log, pvc)
	}

	vcs, err := r.getVolumeCloneSource(ctx, log, pvc)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	if vcs == nil {
		log.V(3).Info("dataSourceRef does not exist, exiting")
		return reconcile.Result{}, r.updateClonePhasePending(ctx, log, pvc)
	}

	if err = r.validateCrossNamespace(pvc, vcs); err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	cs, err := r.getCloneStrategy(ctx, log, pvc, vcs)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	if cs == nil {
		log.V(3).Info("unable to choose clone strategy now")
		// TODO maybe create index/watch to deal with this
		return reconcile.Result{RequeueAfter: 5 * time.Second}, r.updateClonePhasePending(ctx, log, pvc)
	}

	updated, err := r.initTargetClaim(ctx, log, pvc, vcs, *cs)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	if updated {
		log.V(3).Info("initialized target, returning")
		// phase will be set to pending by initTargetClaim if unset
		return reconcile.Result{}, nil
	}

	args := &clone.PlanArgs{
		Log:         log,
		TargetClaim: pvc,
		DataSource:  vcs,
		Strategy:    *cs,
	}

	return r.planAndExecute(ctx, log, pvc, statusOnly, args)
}

func (r *ClonePopulatorReconciler) getCloneStrategy(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, vcs *cdiv1.VolumeCloneSource) (*cdiv1.CDICloneStrategy, error) {
	cs := getSavedCloneStrategy(pvc)
	if cs != nil {
		return cs, nil
	}

	args := &clone.ChooseStrategyArgs{
		Log:         log,
		TargetClaim: pvc,
		DataSource:  vcs,
	}

	cs, err := r.planner.ChooseStrategy(ctx, args)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (r *ClonePopulatorReconciler) planAndExecute(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, statusOnly bool, args *clone.PlanArgs) (reconcile.Result, error) {
	phases, err := r.planner.Plan(ctx, args)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
	}

	log.V(3).Info("created phases", "num", len(phases))

	var statusResults []*clone.PhaseStatus
	for _, p := range phases {
		var result *reconcile.Result
		var err error
		var progress string
		if !statusOnly {
			result, err = p.Reconcile(ctx)
			if err != nil {
				return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
			}
		}

		if sr, ok := p.(clone.StatusReporter); ok {
			ps, err := sr.Status(ctx)
			if err != nil {
				return reconcile.Result{}, r.updateClonePhaseError(ctx, log, pvc, err)
			}
			progress = ps.Progress
			statusResults = append(statusResults, ps)
		}

		if result != nil {
			log.V(1).Info("currently in phase, returning", "name", p.Name(), "progress", progress)
			return *result, r.updateClonePhase(ctx, log, pvc, p.Name(), statusResults)
		}
	}

	log.V(3).Info("executed all phases, setting phase to Succeeded")

	return reconcile.Result{}, r.updateClonePhaseSucceeded(ctx, log, pvc, statusResults)
}

func (r *ClonePopulatorReconciler) validateCrossNamespace(pvc *corev1.PersistentVolumeClaim, vcs *cdiv1.VolumeCloneSource) error {
	if pvc.Namespace == vcs.Namespace {
		return nil
	}

	anno, ok := pvc.Annotations[AnnDataSourceNamespace]
	if ok && anno == vcs.Namespace {
		if err := r.multiTokenValidator.ValidatePopulator(vcs, pvc); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("cross-namespace with resource grants is not supported yet")
}

func (r *ClonePopulatorReconciler) reconcileDone(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	log.V(3).Info("executing cleanup")
	if err := r.planner.Cleanup(ctx, log, pvc); err != nil {
		return reconcile.Result{}, err
	}

	log.V(1).Info("removing finalizer")
	claimCpy := pvc.DeepCopy()
	cc.RemoveFinalizer(claimCpy, cloneFinalizer)
	return reconcile.Result{}, r.client.Update(ctx, claimCpy)
}

func (r *ClonePopulatorReconciler) initTargetClaim(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, vcs *cdiv1.VolumeCloneSource, cs cdiv1.CDICloneStrategy) (bool, error) {
	claimCpy := pvc.DeepCopy()
	clone.AddCommonClaimLabels(claimCpy)
	setSavedCloneStrategy(claimCpy, cs)
	if claimCpy.Annotations[AnnClonePhase] == "" {
		cc.AddAnnotation(claimCpy, AnnClonePhase, clone.PendingPhaseName)
	}
	cc.AddFinalizer(claimCpy, cloneFinalizer)

	if !apiequality.Semantic.DeepEqual(pvc, claimCpy) {
		if err := r.client.Update(ctx, claimCpy); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (r *ClonePopulatorReconciler) updateClonePhasePending(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim) error {
	return r.updateClonePhase(ctx, log, pvc, clone.PendingPhaseName, nil)
}

func (r *ClonePopulatorReconciler) updateClonePhaseSucceeded(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, status []*clone.PhaseStatus) error {
	if status == nil {
		status = []*clone.PhaseStatus{{}}
	}
	status[len(status)-1].Progress = cc.ProgressDone
	return r.updateClonePhase(ctx, log, pvc, clone.SucceededPhaseName, status)
}

func (r *ClonePopulatorReconciler) updateClonePhase(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, phase string, status []*clone.PhaseStatus) error {
	claimCpy := pvc.DeepCopy()
	delete(claimCpy.Annotations, AnnCloneError)
	cc.AddAnnotation(claimCpy, AnnClonePhase, phase)

	var mergedAnnotations = make(map[string]string)
	for _, ps := range status {
		if ps.Progress != "" {
			cc.AddAnnotation(claimCpy, cc.AnnPopulatorProgress, ps.Progress)
		}
		for k, v := range ps.Annotations {
			mergedAnnotations[k] = v
			if _, ok := desiredCloneAnnotations[k]; ok {
				cc.AddAnnotation(claimCpy, k, v)
			}
		}
	}

	r.addRunningAnnotations(claimCpy, phase, mergedAnnotations)

	if !apiequality.Semantic.DeepEqual(pvc, claimCpy) {
		return r.client.Update(ctx, claimCpy)
	}

	return nil
}

func (r *ClonePopulatorReconciler) updateClonePhaseError(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, lastError error) error {
	claimCpy := pvc.DeepCopy()
	cc.AddAnnotation(claimCpy, AnnClonePhase, clone.ErrorPhaseName)
	cc.AddAnnotation(claimCpy, AnnCloneError, lastError.Error())

	r.addRunningAnnotations(claimCpy, clone.ErrorPhaseName, nil)

	if !apiequality.Semantic.DeepEqual(pvc, claimCpy) {
		if err := r.client.Update(ctx, claimCpy); err != nil {
			log.V(1).Info("error setting error annotations")
		}
	}

	return lastError
}

func (r *ClonePopulatorReconciler) addRunningAnnotations(pvc *corev1.PersistentVolumeClaim, phase string, annotations map[string]string) {
	if !cc.OwnedByDataVolume(pvc) {
		return
	}

	var running, message, reason string
	if phase == clone.SucceededPhaseName {
		running = "false"
		message = "Clone Complete"
		reason = "Completed"
	} else if phase == clone.PendingPhaseName {
		running = "false"
		message = "Clone Pending"
		reason = "Pending"
	} else if phase == clone.ErrorPhaseName {
		running = "false"
		message = pvc.Annotations[AnnCloneError]
		reason = "Error"
	} else if _, ok := annotations[cc.AnnRunningCondition]; ok {
		running = annotations[cc.AnnRunningCondition]
		message = annotations[cc.AnnRunningConditionMessage]
		reason = annotations[cc.AnnRunningConditionReason]
		if restarts, ok := annotations[cc.AnnPodRestarts]; ok {
			cc.AddAnnotation(pvc, cc.AnnPodRestarts, restarts)
		}
	} else {
		running = "true"
		reason = "Populator is running"
	}

	cc.AddAnnotation(pvc, cc.AnnRunningCondition, running)
	cc.AddAnnotation(pvc, cc.AnnRunningConditionMessage, message)
	cc.AddAnnotation(pvc, cc.AnnRunningConditionReason, reason)
}

func (r *ClonePopulatorReconciler) getVolumeCloneSource(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim) (*cdiv1.VolumeCloneSource, error) {
	if !IsPVCDataSourceRefKind(pvc, cdiv1.VolumeCloneSourceRef) {
		return nil, fmt.Errorf("pvc %s/%s does not refer to a %s", pvc.Namespace, pvc.Name, cdiv1.VolumeCloneSourceRef)
	}

	ns := pvc.Namespace
	anno, ok := pvc.Annotations[AnnDataSourceNamespace]
	if ok {
		log.V(3).Info("found datasource namespace annotation", "namespace", ns)
		ns = anno
	} else if pvc.Spec.DataSourceRef.Namespace != nil {
		ns = *pvc.Spec.DataSourceRef.Namespace
	}

	obj := &cdiv1.VolumeCloneSource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      pvc.Spec.DataSourceRef.Name,
		},
	}

	if err := r.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return obj, nil
}

func isClonePhaseSucceeded(obj client.Object) bool {
	return obj.GetAnnotations()[AnnClonePhase] == clone.SucceededPhaseName
}

func getSavedCloneStrategy(obj client.Object) *cdiv1.CDICloneStrategy {
	if val, ok := obj.GetAnnotations()[cc.AnnCloneType]; ok {
		cs := cdiv1.CDICloneStrategy(val)
		return &cs
	}
	return nil
}

func setSavedCloneStrategy(obj client.Object, strategy cdiv1.CDICloneStrategy) {
	cc.AddAnnotation(obj, cc.AnnCloneType, string(strategy))
}
