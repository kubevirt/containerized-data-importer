package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
)

const (
	createResourceFailed  = "CreateResourceFailed"
	createResourceSuccess = "CreateResourceSuccess"

	deleteResourceFailed  = "DeleteResourceFailed"
	deleteResourceSuccess = "DeleteResourceSuccess"

	updateResourceFailed  = "UpdateResourceFailed"
	updateResourceSuccess = "UpdateResourceSuccess"
)

// PerishablesSynchronizer is expected to execute perishable resources (i.e. certificates) synchronization if required
type PerishablesSynchronizer func(cr client.Object, logger logr.Logger) error

// ControllerConfigUpdater is expected to update controller configuration if required
type ControllerConfigUpdater func(cr client.Object) error

// SanityChecker is expected to check if it makes sense to execute the reconciliation if required
type SanityChecker func(cr client.Object, logger logr.Logger) (*reconcile.Result, error)

// WatchRegistrator is expected to register additional resource watchers if required
type WatchRegistrator func() error

// PreCreateHook is expected to perform custom actions before the creation of the managed resources is initiated
type PreCreateHook func(cr client.Object) error

// CrManager defines interface that needs to be provided for the reconciler to operate
type CrManager interface {
	// IsCreating checks whether creation of the managed resources will be executed
	IsCreating(cr client.Object) (bool, error)
	// Creates empty CR
	Create() client.Object
	// Status extracts status from the cr
	Status(cr client.Object) *sdkapi.Status
	// GetAllResources provides all resources managed by the cr
	GetAllResources(cr client.Object) ([]client.Object, error)
	// GetDependantResourcesListObjects returns resource list objects of dependant resources
	GetDependantResourcesListObjects() []client.ObjectList
}

// CallbackDispatcher manages and executes resource callbacks
type CallbackDispatcher interface {
	// AddCallback registers a callback for given object type
	AddCallback(client.Object, callbacks.ReconcileCallback)

	// InvokeCallbacks executes callbacks for desired/current object type
	InvokeCallbacks(l logr.Logger, cr interface{}, s callbacks.ReconcileState, desiredObj, currentObj client.Object, recorder record.EventRecorder) error
}

// Reconciler is responsible for performing deployment reconciliation
type Reconciler struct {
	crManager CrManager

	watchMutex sync.Mutex
	watching   bool

	controller controller.Controller
	log        logr.Logger

	client client.Client

	callbackDispatcher          CallbackDispatcher
	createVersionLabel          string
	lastAppliedConfigAnnotation string
	updateVersionLabel          string
	scheme                      *runtime.Scheme
	getCache                    func() cache.Cache
	recorder                    record.EventRecorder
	perishablesSyncInterval     time.Duration
	finalizerName               string
	namespacedCR                bool
	subresourceEnabled          bool

	// Hooks
	syncPerishables               PerishablesSynchronizer
	updateControllerConfiguration ControllerConfigUpdater
	checkSanity                   SanityChecker
	watch                         WatchRegistrator
	preCreate                     PreCreateHook
}

// Reconcile performs request reconciliation
func (r *Reconciler) Reconcile(request reconcile.Request, operatorVersion string, reqLogger logr.Logger) (reconcile.Result, error) {
	// Fetch the CR instance
	cr, err := r.GetCr(request.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			reqLogger.Info("CR no longer exists")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// make sure we're watching eveything
	if err := r.WatchDependantResources(cr); err != nil {
		return reconcile.Result{}, err
	}

	// mid delete
	if cr.GetDeletionTimestamp() != nil {
		reqLogger.Info("Doing reconcile delete")
		return r.ReconcileDelete(reqLogger, cr, r.finalizerName)
	}

	status := r.status(cr)
	creating, err := r.crManager.IsCreating(cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	if creating {
		if status.Phase != "" {
			reqLogger.Info("Reconciling to error state, illegal phase", "phase", status.Phase)
			// we are in a weird state
			return r.ReconcileError(cr, "Reconciling to error state, illegal phase")
		}

		haveOrphans, err := r.CheckForOrphans(reqLogger, cr)
		if err != nil {
			return reconcile.Result{}, err
		}

		if haveOrphans {
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}
		reqLogger.Info("Doing reconcile create")
		if err := r.preCreate(cr); err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("Pre-create hook executed successfully")

		status := r.crManager.Status(cr)
		sdk.MarkCrDeploying(cr, status, "DeployStarted", "Started Deployment", r.recorder)

		if err := r.CrInit(cr, operatorVersion); err != nil {
			return reconcile.Result{}, err
		}

		reqLogger.Info("Successfully entered Deploying state")
	}

	// do we even care about this CR?
	result, err := r.checkSanity(cr, reqLogger)
	if result != nil {
		return *result, err
	}

	currentConditionValues := sdk.GetConditionValues(status.Conditions)
	reqLogger.Info("Doing reconcile update")

	res, err := r.ReconcileUpdate(reqLogger, cr, operatorVersion)
	if sdk.ConditionsChanged(currentConditionValues, sdk.GetConditionValues(status.Conditions)) {
		if err := r.CrUpdateStatus(status.Phase, cr); err != nil {
			return reconcile.Result{}, err
		}
	}

	return res, err
}

// ReconcileUpdate executes Update operation
func (r *Reconciler) ReconcileUpdate(logger logr.Logger, cr client.Object, operatorVersion string) (reconcile.Result, error) {
	if err := r.CheckUpgrade(logger, cr, operatorVersion); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateControllerConfiguration(cr); err != nil {
		logger.Error(err, "Error while customizing controller configuration")
		return reconcile.Result{}, err
	}

	resources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	var allErrors []error
	for _, desiredObj := range resources {
		currentObj := sdk.NewDefaultInstance(desiredObj)

		key := client.ObjectKey{
			Namespace: desiredObj.GetNamespace(),
			Name:      desiredObj.GetName(),
		}
		err = r.client.Get(context.TODO(), key, currentObj)

		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			r.setLastAppliedConfiguration(desiredObj)
			sdk.SetLabel(r.createVersionLabel, operatorVersion, desiredObj)
			r.setRecommendedLabels(cr, desiredObj)

			if err = controllerutil.SetControllerReference(cr, desiredObj, r.scheme); err != nil {
				r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to create resource %s, %v", desiredObj.GetName(), err))
				return reconcile.Result{}, err
			}

			// PRE_CREATE callback
			if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePreCreate, desiredObj, nil, r.recorder); err != nil {
				r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to create resource %s, %v", desiredObj.GetName(), err))
				return reconcile.Result{}, err
			}

			currentObj = desiredObj.DeepCopyObject().(client.Object)
			if err = r.client.Create(context.TODO(), currentObj); err != nil {
				logger.Error(err, "")
				allErrors = append(allErrors, err)
				r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to create resource %s, %v", desiredObj.GetName(), err))
				continue
			}

			// POST_CREATE callback
			if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePostCreate, desiredObj, nil, r.recorder); err != nil {
				r.recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to create resource %s, %v", desiredObj.GetName(), err))
				return reconcile.Result{}, err
			}

			logger.Info("Resource created",
				"namespace", desiredObj.GetNamespace(),
				"name", desiredObj.GetName(),
				"type", fmt.Sprintf("%T", desiredObj))
			r.recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, fmt.Sprintf("Successfully created resource %T %s", desiredObj, desiredObj.GetName()))
		} else {
			// POST_READ callback
			if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePostRead, desiredObj, currentObj, r.recorder); err != nil {
				return reconcile.Result{}, err
			}

			currentObj, err = sdk.StripStatusFromObject(currentObj)
			if err != nil {
				return reconcile.Result{}, err
			}
			currentObjCopy := currentObj.DeepCopyObject().(client.Object)

			// allow users to add new annotations (but not change ours)
			sdk.MergeLabelsAndAnnotations(desiredObj, currentObj)

			// recommended label values can change by installer, set on update as well
			r.setRecommendedLabels(cr, currentObj)

			if !sdk.IsMutable(currentObj) {
				r.setLastAppliedConfiguration(desiredObj)

				// overwrite currentRuntimeObj
				currentObj, err = sdk.MergeObject(desiredObj, currentObj, r.lastAppliedConfigAnnotation)
				if err != nil {
					return reconcile.Result{}, err
				}
			}

			if !reflect.DeepEqual(currentObjCopy, currentObj) {
				sdk.LogJSONDiff(logger, currentObjCopy, currentObj)
				sdk.SetLabel(r.updateVersionLabel, operatorVersion, currentObj)

				// PRE_UPDATE callback
				if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePreUpdate, desiredObj, currentObj, r.recorder); err != nil {
					r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf("Failed to update resource %s, %v", desiredObj.GetName(), err))
					return reconcile.Result{}, err
				}

				if err = r.client.Update(context.TODO(), currentObj); err != nil {
					logger.Error(err, "")
					allErrors = append(allErrors, err)
					r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf("Failed to update resource %s, %v", desiredObj.GetName(), err))
					continue
				}

				// POST_UPDATE callback
				if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePostUpdate, desiredObj, nil, r.recorder); err != nil {
					r.recorder.Event(cr, corev1.EventTypeWarning, updateResourceFailed, fmt.Sprintf("Failed to update resource %s, %v", desiredObj.GetName(), err))
					return reconcile.Result{}, err
				}

				logger.Info("Resource updated",
					"namespace", desiredObj.GetNamespace(),
					"name", desiredObj.GetName(),
					"type", fmt.Sprintf("%T", desiredObj))
				r.recorder.Event(cr, corev1.EventTypeNormal, updateResourceSuccess, fmt.Sprintf("Successfully updated resource %T %s", desiredObj, desiredObj.GetName()))
			} else {
				logger.V(3).Info("Resource unchanged",
					"namespace", desiredObj.GetNamespace(),
					"name", desiredObj.GetName(),
					"type", fmt.Sprintf("%T", desiredObj))
			}
		}
	}

	if err = r.syncPerishables(cr, logger); err != nil {
		return reconcile.Result{}, err
	}

	if len(allErrors) > 0 {
		return reconcile.Result{}, fmt.Errorf("reconcile encountered %d errors", len(allErrors))
	}

	degraded, err := r.CheckDegraded(logger, cr)
	if err != nil {
		return reconcile.Result{}, err
	}

	status := r.status(cr)
	if status.Phase != sdkapi.PhaseDeployed && !sdk.IsUpgrading(status) && !degraded {
		//We are not moving to Deployed phase until new operator deployment is ready in case of Upgrade
		status.ObservedVersion = operatorVersion
		sdk.MarkCrHealthyMessage(cr, status, "DeployCompleted", "Deployment Completed", r.recorder)
		if err = r.CrUpdateStatus(sdkapi.PhaseDeployed, cr); err != nil {
			return reconcile.Result{}, err
		}

		logger.Info("Successfully entered Deployed state")
	}

	if !degraded && sdk.IsUpgrading(status) {
		logger.Info("Completing upgrade process...")

		if err = r.completeUpgrade(logger, cr, operatorVersion); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.perishablesSyncInterval}, nil
}

// CheckForOrphans checks whether there are any orphaned resources (ones that exist in the cluster but shouldn't)
func (r *Reconciler) CheckForOrphans(logger logr.Logger, cr client.Object) (bool, error) {
	resources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return false, err
	}

	for _, resource := range resources {
		cpy := resource.DeepCopyObject().(client.Object)
		key := client.ObjectKeyFromObject(cpy)

		if err = r.client.Get(context.TODO(), key, cpy); err != nil {
			if errors.IsNotFound(err) {
				continue
			}

			return false, err
		}

		logger.Info("Orphan object exists", "obj", cpy)
		return true, nil
	}

	return false, nil
}

// CrUpdate sets given phase on the CR and updates it in the cluster
func (r *Reconciler) CrUpdate(cr client.Object) error {
	return r.client.Update(context.TODO(), cr)
}

// CrUpdateStatus sets given phase on the CR and updates it in the cluster
func (r *Reconciler) CrUpdateStatus(phase sdkapi.Phase, cr client.Object) error {
	status := r.crManager.Status(cr)
	status.Phase = phase
	if r.subresourceEnabled {
		return r.client.Status().Update(context.TODO(), cr)
	}
	return r.CrUpdate(cr)
}

// CrSetVersion sets version and phase on the CR object
func (r *Reconciler) CrSetVersion(cr client.Object, version string) error {
	phase := sdkapi.PhaseDeployed
	if version == "" {
		phase = sdkapi.PhaseEmpty
	}
	status := r.status(cr)
	status.ObservedVersion = version
	status.OperatorVersion = version
	status.TargetVersion = version
	return r.CrUpdateStatus(phase, cr)
}

// CrError sets the CR's phase to "Error"
func (r *Reconciler) CrError(cr client.Object) error {
	status := r.status(cr)
	if status.Phase != sdkapi.PhaseError {
		return r.CrUpdateStatus(sdkapi.PhaseError, cr)
	}
	return nil
}

// WatchDependantResources registers watches for dependant resource types
func (r *Reconciler) WatchDependantResources(cr client.Object) error {
	r.watchMutex.Lock()
	defer r.watchMutex.Unlock()

	if r.watching {
		return nil
	}

	resources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return err
	}

	if err = r.WatchResourceTypes(resources...); err != nil {
		return err
	}

	if err = r.watch(); err != nil {
		return err
	}

	r.watching = true

	return nil
}

// ReconcileError Marks CR as failed
func (r *Reconciler) ReconcileError(cr client.Object, message string) (reconcile.Result, error) {
	status := r.status(cr)
	sdk.MarkCrFailed(cr, status, "ConfigError", message, r.recorder)
	if err := r.CrUpdateStatus(status.Phase, cr); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.CrError(cr); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// CheckDegraded checks whether the deployment is degraded and updates CR status conditions accordingly
func (r *Reconciler) CheckDegraded(logger logr.Logger, cr client.Object) (bool, error) {
	degraded := false

	deployments, err := r.GetAllDeployments(cr)
	if err != nil {
		return true, err
	}

	for _, deployment := range deployments {
		key := client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}

		if err = r.client.Get(context.TODO(), key, deployment); err != nil {
			return true, err
		}

		if !sdk.CheckDeploymentReady(deployment) {
			degraded = true
			break
		}
	}

	logger.Info("Degraded check", "Degraded", degraded)

	// If deployed and degraded, mark degraded, otherwise we are still deploying or not degraded.
	status := r.status(cr)
	if degraded && status.Phase == sdkapi.PhaseDeployed {
		conditions.SetStatusCondition(&status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionTrue,
		})
	} else {
		conditions.SetStatusCondition(&status.Conditions, conditions.Condition{
			Type:   conditions.ConditionDegraded,
			Status: corev1.ConditionFalse,
		})
	}

	logger.Info("Finished degraded check", "conditions", status.Conditions)
	return degraded, nil
}

// InvokeDeleteCallbacks executes operator deletion callbacks
func (r *Reconciler) InvokeDeleteCallbacks(logger logr.Logger, cr client.Object) error {
	desiredResources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return err
	}

	for _, desiredObj := range desiredResources {
		if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStateOperatorDelete, desiredObj, nil, r.recorder); err != nil {
			return err
		}
	}

	return nil
}

// GetAllDeployments retrieves all deployments associated to the given CR object
func (r *Reconciler) GetAllDeployments(cr client.Object) ([]*appsv1.Deployment, error) {
	var result []*appsv1.Deployment

	resources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		if deployment, ok := resource.(*appsv1.Deployment); ok {
			result = append(result, deployment)
		}
	}

	return result, nil
}

// WatchCR registers watch for the managed CR
func (r *Reconciler) WatchCR() error {
	// Watch for changes to managed CR
	return r.controller.Watch(source.Kind(r.getCache(), r.crManager.Create(), &handler.EnqueueRequestForObject{}))
}

// InvokeCallbacks executes callbacks registered
func (r *Reconciler) InvokeCallbacks(l logr.Logger, cr client.Object, s callbacks.ReconcileState, desiredObj, currentObj client.Object, recorder record.EventRecorder) error {
	return r.callbackDispatcher.InvokeCallbacks(l, cr, s, desiredObj, currentObj, recorder)
}

// WatchResourceTypes registers watches for given resources types
func (r *Reconciler) WatchResourceTypes(resources ...client.Object) error {
	typeSet := map[reflect.Type]bool{}

	for _, resource := range resources {
		t := reflect.TypeOf(resource)
		if typeSet[t] {
			continue
		}

		eventHandler := handler.EnqueueRequestForOwner(r.scheme, r.client.RESTMapper(), r.crManager.Create(), handler.OnlyControllerOwner())

		predicates := []predicate.Predicate{sdk.NewIgnoreLeaderElectionPredicate()}

		if err := r.controller.Watch(source.Kind(r.getCache(), resource, eventHandler, predicates...)); err != nil {
			if meta.IsNoMatchError(err) || strings.Contains(err.Error(), "failed to find API group") {
				r.log.Info("No match for type, NOT WATCHING", "type", t)
				continue
			}
			return err
		}

		r.log.Info("Watching", "type", t)

		typeSet[t] = true
	}

	return nil
}

// AddCallback registers a callback for given object type
func (r *Reconciler) AddCallback(obj client.Object, cb callbacks.ReconcileCallback) {
	r.callbackDispatcher.AddCallback(obj, cb)
}

// CheckUpgrade checks whether an upgrade should be performed
func (r *Reconciler) CheckUpgrade(logger logr.Logger, cr client.Object, targetVersion string) error {
	// should maybe put this in separate function
	status := r.status(cr)
	if status.OperatorVersion != targetVersion {
		status.OperatorVersion = targetVersion
		status.TargetVersion = targetVersion
		if err := r.CrUpdateStatus(status.Phase, cr); err != nil {
			return err
		}
	}

	deploying := status.Phase == sdkapi.PhaseDeploying
	isUpgrade, err := ShouldTakeUpdatePath(targetVersion, status.ObservedVersion, deploying)
	if err != nil {
		logger.Error(err, "", "current", status.ObservedVersion, "target", targetVersion)
		return err
	}

	if isUpgrade && status.Phase != sdkapi.PhaseUpgrading {
		logger.Info("Observed version is not target version. Begin upgrade", "Observed version ", status.ObservedVersion, "TargetVersion", targetVersion)
		sdk.MarkCrUpgradeHealingDegraded(cr, status, "UpgradeStarted", fmt.Sprintf("Started upgrade to version %s", targetVersion), r.recorder)
		status.TargetVersion = targetVersion
		if err := r.CrUpdateStatus(sdkapi.PhaseUpgrading, cr); err != nil {
			return err
		}
	}

	return nil
}

// CleanupUnusedResources removes unused resources
func (r *Reconciler) CleanupUnusedResources(logger logr.Logger, cr client.Object) error {
	//Iterate over installed resources of
	//Deployment/CRDs/Services etc and delete all resources that
	//do not exist in current version

	desiredResources, err := r.crManager.GetAllResources(cr)
	if err != nil {
		return err
	}

	listTypes := r.crManager.GetDependantResourcesListObjects()

	ls, err := labels.Parse(r.createVersionLabel)
	if err != nil {
		return err
	}

	for _, lt := range listTypes {
		lo := &client.ListOptions{LabelSelector: ls}

		if err := r.client.List(context.TODO(), lt, lo); err != nil {
			logger.Error(err, "Error listing resources")
			return err
		}

		sv := reflect.ValueOf(lt).Elem()
		iv := sv.FieldByName("Items")

		for i := 0; i < iv.Len(); i++ {
			found := false
			observedObj := iv.Index(i).Addr().Interface().(client.Object)
			observedMetaObj := observedObj.(metav1.Object)

			for _, desiredObj := range desiredResources {
				if sdk.SameResource(observedObj, desiredObj) {
					found = true
					break
				}
			}

			if !found && metav1.IsControlledBy(observedMetaObj, cr) {
				//Invoke pre delete callback
				if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePreDelete, nil, observedObj, r.recorder); err != nil {
					r.recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed deleting resource %s, %v", observedMetaObj.GetName(), err))
					return err
				}

				logger.Info("Deleting  ", "type", reflect.TypeOf(observedObj), "Name", observedMetaObj.GetName())
				err = r.client.Delete(context.TODO(), observedObj, &client.DeleteOptions{
					PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
				})
				if err != nil && !errors.IsNotFound(err) {
					r.recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed deleting resource %s, %v", observedMetaObj.GetName(), err))
					return err
				}

				//invoke post delete callback
				if err = r.InvokeCallbacks(logger, cr, callbacks.ReconcileStatePostDelete, nil, observedObj, r.recorder); err != nil {
					r.recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed deleting resource %s, %v", observedMetaObj.GetName(), err))
					return err
				}
				r.recorder.Event(cr, corev1.EventTypeNormal, deleteResourceSuccess, fmt.Sprintf("Successfully deleted resource %T %s", observedMetaObj, observedMetaObj.GetName()))
			}
		}
	}

	return nil
}

// ReconcileDelete executes Delete operation
func (r *Reconciler) ReconcileDelete(logger logr.Logger, cr client.Object, finalizerName string) (reconcile.Result, error) {
	i := -1
	finalizers := cr.GetFinalizers()
	for j, f := range finalizers {
		if f == finalizerName {
			i = j
			break
		}
	}

	if i < 0 {
		return reconcile.Result{}, nil
	}

	status := r.status(cr)
	if status.Phase != sdkapi.PhaseDeleting {
		if err := r.CrUpdateStatus(sdkapi.PhaseDeleting, cr); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.InvokeDeleteCallbacks(logger, cr); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.CrUpdateStatus(sdkapi.PhaseDeleted, cr); err != nil {
		return reconcile.Result{}, err
	}

	finalizers = append(finalizers[0:i], finalizers[i+1:]...)
	cr.SetFinalizers(finalizers)
	if err := r.CrUpdate(cr); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Finalizer complete")

	return reconcile.Result{}, nil
}

// CrInit initializes the CR and moves it to CR to  "Deploying" status
func (r *Reconciler) CrInit(cr client.Object, operatorVersion string) error {
	status := r.status(cr)
	status.OperatorVersion = operatorVersion
	status.TargetVersion = operatorVersion
	if err := r.CrUpdateStatus(sdkapi.PhaseDeploying, cr); err != nil {
		return err
	}

	for _, f := range cr.GetFinalizers() {
		if f == r.finalizerName {
			return nil
		}
	}

	finalizers := append(cr.GetFinalizers(), r.finalizerName)
	cr.SetFinalizers(finalizers)
	return r.CrUpdate(cr)
}

// GetCr retrieves the CR
func (r *Reconciler) GetCr(name types.NamespacedName) (client.Object, error) {
	cr := r.crManager.Create()
	var crKey client.ObjectKey
	if r.namespacedCR {
		crKey = name
	} else {
		// check at cluster level
		crKey = client.ObjectKey{Namespace: "", Name: name.Name}
	}
	err := r.client.Get(context.TODO(), crKey, cr)
	return cr, err
}

func (r *Reconciler) status(object client.Object) *sdkapi.Status {
	return r.crManager.Status(object)
}

func (r *Reconciler) setLastAppliedConfiguration(obj metav1.Object) error {
	return sdk.SetLastAppliedConfiguration(obj, r.lastAppliedConfigAnnotation)
}

func (r *Reconciler) completeUpgrade(logger logr.Logger, cr client.Object, operatorVersion string) error {
	if err := r.CleanupUnusedResources(logger, cr); err != nil {
		return err
	}

	status := r.status(cr)
	previousVersion := status.ObservedVersion
	status.ObservedVersion = operatorVersion

	sdk.MarkCrHealthyMessage(cr, status, "DeployCompleted", "Deployment Completed", r.recorder)
	if err := r.CrUpdateStatus(sdkapi.PhaseDeployed, cr); err != nil {
		return err
	}

	logger.Info("Successfully finished Upgrade and entered Deployed state", "from version", previousVersion, "to version", status.ObservedVersion)

	return nil
}

func (r *Reconciler) setRecommendedLabels(cr client.Object, obj metav1.Object) {
	labels := sdk.GetRecommendedLabelsFromCr(cr)

	for k, v := range labels {
		sdk.SetLabel(k, v, obj)
	}

	// Actual workload templates need special care, otherwise we just update the top level labels
	switch typedObj := obj.(type) {
	case *appsv1.Deployment:
		if typedObj.Spec.Template.GetLabels() == nil {
			typedObj.Spec.Template.SetLabels(make(map[string]string))
		}
		for k, v := range labels {
			typedObj.Spec.Template.GetLabels()[k] = v
		}
	}
}
