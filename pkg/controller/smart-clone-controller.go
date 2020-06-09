package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	//AnnSmartCloneRequest sets our expected annotation for a CloneRequest
	AnnSmartCloneRequest = "k8s.io/SmartCloneRequest"
)

// SmartCloneReconciler members
type SmartCloneReconciler struct {
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
}

// NewSmartCloneController creates a new instance of the Smart clone controller.
func NewSmartCloneController(mgr manager.Manager, log logr.Logger) (controller.Controller, error) {
	reconciler := &SmartCloneReconciler{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		log:      log.WithName("smartclone-controller"),
		recorder: mgr.GetEventRecorderFor("smartclone-controller"),
	}
	smartCloneController, err := controller.New("smartclone-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addSmartCloneControllerWatches(mgr, smartCloneController); err != nil {
		return nil, err
	}
	return smartCloneController, nil
}

func addSmartCloneControllerWatches(mgr manager.Manager, smartCloneController controller.Controller) error {
	// Add schemes.
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := snapshotv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	// Setup watches
	if err := smartCloneController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return shouldReconcilePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldReconcilePvc(e.ObjectNew.(*corev1.PersistentVolumeClaim))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return shouldReconcilePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return shouldReconcilePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
	}); err != nil {
		return err
	}

	// check if volume snapshots exist
	err := mgr.GetClient().List(context.TODO(), &snapshotv1.VolumeSnapshotList{})
	if meta.IsNoMatchError(err) {
		return nil
	}

	if err != nil && !isErrCacheNotStarted(err) {
		return err
	}

	if err := smartCloneController.Watch(&source.Kind{Type: &snapshotv1.VolumeSnapshot{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataVolume{},
		IsController: true,
	}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return shouldReconcileSnapshot(e.Object.(*snapshotv1.VolumeSnapshot))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldReconcileSnapshot(e.ObjectNew.(*snapshotv1.VolumeSnapshot))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return shouldReconcileSnapshot(e.Object.(*snapshotv1.VolumeSnapshot))
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return shouldReconcileSnapshot(e.Object.(*snapshotv1.VolumeSnapshot))
		},
	}); err != nil {
		return err
	}

	return nil
}

func shouldReconcileSnapshot(snapshot *snapshotv1.VolumeSnapshot) bool {
	_, ok := snapshot.GetAnnotations()[AnnSmartCloneRequest]
	if !ok {
		return false
	}
	return snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse
}

func shouldReconcilePvc(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Status.Phase == corev1.ClaimLost {
		return false
	}

	val, ok := pvc.GetAnnotations()[AnnSmartCloneRequest]
	return ok && val == "true"
}

// Reconcile the reconcile loop for smart cloning.
func (r *SmartCloneReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("Datavolume", req.NamespacedName)
	log.Info("reconciling smart clone")
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			// PVC not found, look up smart clone.
			snapshot := &snapshotv1.VolumeSnapshot{}
			if err := r.client.Get(context.TODO(), req.NamespacedName, snapshot); err != nil {
				if k8serrors.IsNotFound(err) {
					return reconcile.Result{}, nil
				}
				return reconcile.Result{}, err
			}
			return r.reconcileSnapshot(log, snapshot)
		}
		return reconcile.Result{}, err
	}
	return r.reconcilePvc(log, pvc)
}

func (r *SmartCloneReconciler) reconcilePvc(log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	log.WithValues("pvc.Name", pvc.Name).WithValues("pvc.Namespace", pvc.Namespace).Info("PVC created from snapshot, updating datavolume status")
	snapshotName := pvc.Spec.DataSource.Name

	datavolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: snapshotName, Namespace: pvc.Namespace}, datavolume); err != nil {
		return reconcile.Result{}, err
	}

	// Update DV phase and emit PVC in progress event
	if err := r.updateSmartCloneStatusPhase(cdiv1.Succeeded, datavolume, pvc); err != nil {
		// Have not properly updated the data volume status, don't delete the snapshot so we retry.
		log.Error(err, "error updating datavolume with success")
		return reconcile.Result{}, err
	}

	// Don't delete snapshot unless the PVC is bound.
	if pvc.Status.Phase == corev1.ClaimBound {
		snapshotToDelete := &snapshotv1.VolumeSnapshot{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: snapshotName, Namespace: pvc.Namespace}, snapshotToDelete); err != nil {
			if k8serrors.IsNotFound(err) {
				// Already gone, so no need to try a delete.
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		if err := r.client.Delete(context.TODO(), snapshotToDelete); err != nil {
			if !k8serrors.IsNotFound(err) {
				log.Error(err, "error deleting snapshot for smart-clone")
				return reconcile.Result{}, err
			}
		}
		log.V(3).Info("Snapshot deleted")
	}

	return reconcile.Result{}, nil
}

func (r *SmartCloneReconciler) reconcileSnapshot(log logr.Logger, snapshot *snapshotv1.VolumeSnapshot) (reconcile.Result, error) {
	log.WithValues("snapshot.Name", snapshot.Name).WithValues("snapshot.Namespace", snapshot.Namespace).Info("Updating datavolume status using snapshot")
	datavolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, datavolume); err != nil {
		return reconcile.Result{}, err
	}

	// Update DV phase and emit PVC in progress event
	if err := r.updateSmartCloneStatusPhase(SmartClonePVCInProgress, datavolume, nil); err != nil {
		// Have not properly updated the data volume status, don't delete the snapshot so we retry.
		log.Error(err, "error updating datavolume with success")
		return reconcile.Result{}, err
	}

	newPvc := newPvcFromSnapshot(snapshot, datavolume)
	if newPvc == nil {
		return reconcile.Result{}, errors.New("error creating new pvc from snapshot object, snapshot has no owner")
	}

	log.V(3).Info("Creating PVC from snapshot", "pvc.Namespace", newPvc.Namespace, "pvc.Name", newPvc.Name)
	if err := r.client.Create(context.TODO(), newPvc); err != nil {
		log.Error(err, "error creating pvc from snapshot")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *SmartCloneReconciler) updateSmartCloneStatusPhase(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume, newPVC *corev1.PersistentVolumeClaim) error {
	var dataVolumeCopy = dataVolume.DeepCopy()
	var event DataVolumeEvent

	switch phase {
	case cdiv1.SmartClonePVCInProgress:
		dataVolumeCopy.Status.Phase = cdiv1.SmartClonePVCInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = SmartClonePVCInProgress
		event.message = fmt.Sprintf(MessageSmartClonePVCInProgress, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name)
		dataVolume.Status.Conditions = updateBoundCondition(dataVolume.Status.Conditions, newPVC)
		dataVolume.Status.Conditions = updateReadyCondition(dataVolume.Status.Conditions, corev1.ConditionFalse, "", "")
		dataVolume.Status.Conditions = updateCondition(dataVolume.Status.Conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, MessageSmartClonePVCInProgress, SmartClonePVCInProgress)
	case cdiv1.Succeeded:
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, dataVolumeCopy.Spec.Source.PVC.Namespace, dataVolumeCopy.Spec.Source.PVC.Name, newPVC.Namespace, newPVC.Name)
		dataVolume.Status.Conditions = updateBoundCondition(dataVolume.Status.Conditions, newPVC)
		dataVolume.Status.Conditions = updateReadyCondition(dataVolume.Status.Conditions, corev1.ConditionTrue, "", "")
		dataVolume.Status.Conditions = updateCondition(dataVolume.Status.Conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, cloneComplete, "Completed")
	}

	return r.emitEvent(dataVolume, dataVolumeCopy, &event, newPVC)
}

func (r *SmartCloneReconciler) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent, newPVC *corev1.PersistentVolumeClaim) error {
	// Only update the object if something actually changed in the status.
	if !reflect.DeepEqual(dataVolume.Status, dataVolumeCopy.Status) {
		if err := r.client.Update(context.TODO(), dataVolumeCopy); err == nil {
			// Emit the event only when the status change happens, not every time
			if event.eventType != "" {
				r.recorder.Event(dataVolume, event.eventType, event.reason, event.message)
			}
		} else {
			return err
		}
	}
	return nil
}

func newPvcFromSnapshot(snapshot *snapshotv1.VolumeSnapshot, dataVolume *cdiv1.DataVolume) *corev1.PersistentVolumeClaim {
	labels := map[string]string{
		"cdi-controller":         snapshot.Name,
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	ownerRef := metav1.GetControllerOf(snapshot)
	if ownerRef == nil {
		return nil
	}
	annotations := make(map[string]string)
	annotations[AnnSmartCloneRequest] = "true"
	annotations[AnnCloneOf] = "true"
	annotations[AnnRunningCondition] = string(corev1.ConditionFalse)
	annotations[AnnRunningConditionMessage] = cloneComplete
	annotations[AnnRunningConditionReason] = "Completed"

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            snapshot.Name,
			Namespace:       snapshot.Namespace,
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSource: &corev1.TypedLocalObjectReference{
				Name:     snapshot.Name,
				Kind:     "VolumeSnapshot",
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
			},
			VolumeMode:       dataVolume.Spec.PVC.VolumeMode,
			AccessModes:      dataVolume.Spec.PVC.AccessModes,
			StorageClassName: dataVolume.Spec.PVC.StorageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: dataVolume.Spec.PVC.Resources.Requests,
			},
		},
	}
}
