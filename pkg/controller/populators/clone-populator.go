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

	clonePopulatorName = "clone-populator"

	cloneFinalizer = "cdi.kubevirt.io/clonePopulator"

	pendingPhase = "Pending"

	succeededPhase = "Succeeded"

	errorPhase = "Error"
)

// Planner is an interface to mock out planner implementation for testing
type Planner interface {
	ChooseStrategy(context.Context, *clone.ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error)
	Plan(context.Context, *clone.PlanArgs) ([]clone.Phase, error)
	Cleanup(context.Context, logr.Logger, client.Object) error
}

// ClonePopulatorReconciler reconciles PVCs with VolumeCloneSources
type ClonePopulatorReconciler struct {
	ReconcilerBase
	planner Planner
}

// NewClonePopulator creates a new instance of the clone-populator controller
func NewClonePopulator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	clonerImage string,
	pullPolicy string,
	installerLabels map[string]string,
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

	// replace this eventually
	if pvc.Spec.DataSourceRef != nil &&
		pvc.Spec.DataSourceRef.Namespace != nil &&
		*pvc.Spec.DataSourceRef.Namespace != pvc.Namespace {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, fmt.Errorf("cross namespace datasource not supported yet"))
	}

	hasFinalizer := cc.HasFinalizer(pvc, cloneFinalizer)
	isBound := cc.IsBound(pvc)
	isDeleted := !pvc.DeletionTimestamp.IsZero()

	log.V(3).Info("pvc state", "hasFinalizer", hasFinalizer, "isBound", isBound, "isDeleted", isDeleted)

	if !isDeleted && !isBound {
		return r.reconcilePending(ctx, log, pvc)
	}

	if hasFinalizer {
		if isBound && !isDeleted && !isClonePhaseSucceeded(pvc) {
			log.V(1).Info("setting phase to Succeeded")
			return reconcile.Result{}, r.updateClonePhaseSucceeded(ctx, pvc)
		}

		return r.reconcileDone(ctx, log, pvc)
	}

	return reconcile.Result{}, nil
}

func (r *ClonePopulatorReconciler) reconcilePending(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	ready, _, err := claimReadyForPopulation(ctx, r.client, pvc)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
	}

	if !ready {
		log.V(3).Info("claim not ready for population, exiting")
		return reconcile.Result{}, r.updateClonePhasePending(ctx, pvc)
	}

	vcs, err := getVolumeCloneSource(ctx, r.client, pvc)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
	}

	if vcs == nil {
		log.V(3).Info("dataSourceRef does not exist, exiting")
		return reconcile.Result{}, r.updateClonePhasePending(ctx, pvc)
	}

	cs, err := r.getCloneStrategy(ctx, log, pvc, vcs)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
	}

	if cs == nil {
		log.V(3).Info("unable to choose clone strategy now")
		// TODO maybe create index/watch to deal with this
		return reconcile.Result{RequeueAfter: 5 * time.Second}, r.updateClonePhasePending(ctx, pvc)
	}

	updated, err := r.initTargetClaim(ctx, pvc, vcs, *cs)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
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

	return r.planAndExecute(ctx, log, pvc, args)
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

func (r *ClonePopulatorReconciler) planAndExecute(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim, args *clone.PlanArgs) (reconcile.Result, error) {
	phases, err := r.planner.Plan(ctx, args)
	if err != nil {
		return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
	}

	log.V(3).Info("created phases", "num", len(phases))

	for _, p := range phases {
		var progress string
		result, err := p.Reconcile(ctx)
		if err != nil {
			return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
		}

		if pr, ok := p.(clone.ProgressReporter); ok {
			progress, err = pr.Progress(ctx)
			if err != nil {
				return reconcile.Result{}, r.updateClonePhaseError(ctx, pvc, err)
			}
		}

		if result != nil {
			log.V(1).Info("currently in phase, returning", "name", p.Name(), "progress", progress)
			return *result, r.updateClonePhase(ctx, pvc, p.Name(), progress)
		}
	}

	log.V(3).Info("executed all phases, setting phase to Succeeded")

	return reconcile.Result{}, r.updateClonePhaseSucceeded(ctx, pvc)
}

func (r *ClonePopulatorReconciler) reconcileDone(ctx context.Context, log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	log.V(3).Info("executing cleanup")
	if err := r.planner.Cleanup(ctx, log, pvc); err != nil {
		return reconcile.Result{}, err
	}

	log.V(1).Info("removing finalizer")
	cc.RemoveFinalizer(pvc, cloneFinalizer)
	return reconcile.Result{}, r.client.Update(ctx, pvc)
}

func (r *ClonePopulatorReconciler) initTargetClaim(ctx context.Context, pvc *corev1.PersistentVolumeClaim, vcs *cdiv1.VolumeCloneSource, cs cdiv1.CDICloneStrategy) (bool, error) {
	claimCpy := pvc.DeepCopy()
	clone.AddCommonClaimLabels(claimCpy)
	setSavedCloneStrategy(claimCpy, cs)
	if claimCpy.Annotations[AnnClonePhase] == "" {
		cc.AddAnnotation(claimCpy, AnnClonePhase, pendingPhase)
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

func (r *ClonePopulatorReconciler) updateClonePhasePending(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	return r.updateClonePhase(ctx, pvc, pendingPhase, "")
}

func (r *ClonePopulatorReconciler) updateClonePhaseSucceeded(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	return r.updateClonePhase(ctx, pvc, succeededPhase, cc.ProgressDone)
}

func (r *ClonePopulatorReconciler) updateClonePhase(ctx context.Context, pvc *corev1.PersistentVolumeClaim, phase, progress string) error {
	claimCpy := pvc.DeepCopy()
	delete(claimCpy.Annotations, AnnCloneError)
	cc.AddAnnotation(claimCpy, AnnClonePhase, phase)
	if progress != "" {
		cc.AddAnnotation(claimCpy, AnnPopulatorProgress, progress)
	}

	if !apiequality.Semantic.DeepEqual(pvc, claimCpy) {
		return r.client.Update(ctx, claimCpy)
	}

	return nil
}

func (r *ClonePopulatorReconciler) updateClonePhaseError(ctx context.Context, pvc *corev1.PersistentVolumeClaim, lastError error) error {
	claimCpy := pvc.DeepCopy()
	cc.AddAnnotation(claimCpy, AnnClonePhase, errorPhase)
	cc.AddAnnotation(claimCpy, AnnCloneError, lastError.Error())

	if !apiequality.Semantic.DeepEqual(pvc, claimCpy) {
		if err := r.client.Update(ctx, claimCpy); err != nil {
			r.log.V(1).Info("error setting error annotations")
		}
	}

	return lastError
}

func isClonePhaseSucceeded(obj client.Object) bool {
	return obj.GetAnnotations()[AnnClonePhase] == succeededPhase
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

func getVolumeCloneSource(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*cdiv1.VolumeCloneSource, error) {
	if !IsPVCDataSourceRefKind(pvc, cdiv1.VolumeCloneSourceRef) {
		return nil, nil
	}

	ns := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		ns = *pvc.Spec.DataSourceRef.Namespace
	}

	obj := &cdiv1.VolumeCloneSource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      pvc.Spec.DataSourceRef.Name,
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return obj, nil
}
