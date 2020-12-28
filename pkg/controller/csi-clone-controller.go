package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ref "k8s.io/client-go/tools/reference"

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
)

const (
	AnnCSICloneRequest     = "cdi.kubevirt.io/CSICloneRequest"       // Annotation associating object with CSI Clone
	AnnCSICloneDVNamespace = "cdi.kubevirt.io/CSICloneDVNamespace"   // Annotation denoting the namespace of the associated datavolume for use by the source PVC
	AnnCSICloneSource      = "cdi.kubevirt.io/CSICloneSource"        // Annotation to represent the source cloning PVC ("true"/"false")
	AnnCSICloneTarget      = "cdi.kubevirt.io/CSICloneTarget"        // Annotation to represent the target cloning PVC ("true"/"false")
	AnnCSICloneCapable     = "cdi.kubevirt.io/CSICloneVolumeCapable" // Annotation for the target storageClass denoting whether the underlying CSI Driver supports CSI Volume Cloning ("true"/"false")
	// This must be set by a cluster admin on a per storageClass basis
	// See https://kubernetes-csi.github.io/docs/volume-cloning.html for details
)

type CSIClonePVCType string

// Enum for PVC types used in CSI Cloning
const (
	CSICloneSourcePVC CSIClonePVCType = AnnCSICloneSource
	CSICloneTargetPVC CSIClonePVCType = AnnCSICloneTarget
)

type CSICloneReconciler struct {
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
}

func NewCSICloneController(mgr manager.Manager, log logr.Logger) (controller.Controller, error) {
	reconciler := &CSICloneReconciler{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		log:      log.WithName("csiclone-controller"),
		recorder: mgr.GetEventRecorderFor("csiclone-controller"),
	}
	csiCloneController, err := controller.New("csiclone-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addCSICloneControllerWatches(mgr, csiCloneController); err != nil {
		return nil, err
	}
	return csiCloneController, nil
}

func addCSICloneControllerWatches(mgr manager.Manager, csiCloneController controller.Controller) error {
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	if err := csiCloneController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return shouldReconcileCSIClonePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldReconcileCSIClonePvc(e.ObjectNew.(*corev1.PersistentVolumeClaim))
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return shouldReconcileCSIClonePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return shouldReconcileCSIClonePvc(e.Object.(*corev1.PersistentVolumeClaim))
		},
	}); err != nil {
		return err
	}

	return nil
}

func shouldReconcileCSIClonePvc(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Status.Phase == corev1.ClaimLost {
		return false
	}

	controllingDv := metav1.GetControllerOf(pvc)

	val, ok := pvc.Annotations[AnnCSICloneRequest]
	isCSICloneRequest, _ := strconv.ParseBool(val)
	return ok && isCSICloneRequest && (controllingDv != nil && controllingDv.Kind == "DataVolume")
}

// Reconcile the reconcile loop for csi cloning
func (r *CSICloneReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.Info("reconciling csi clone")
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	isCloneSource := pvc.Annotations[AnnCSICloneSource]
	isCloneTarget := pvc.Annotations[AnnCSICloneTarget]

	if s, err := strconv.ParseBool(isCloneSource); s && err == nil {
		// Reconciling the Source PVC
		return r.reconcileSourcePvc(log, pvc)
	}

	if s, err := strconv.ParseBool(isCloneTarget); s && err == nil {
		// Reconciling the Target PVC
		return r.reconcileTargetPVC(log, pvc)
	}

	return reconcile.Result{}, nil
}

func verifyTargetPVC(targetPvc *corev1.PersistentVolumeClaim) error {
	controllingDv := metav1.GetControllerOf(targetPvc)
	if controllingDv == nil {
		return fmt.Errorf("No controller for target clone pvc")
	} else if controllingDv.Kind != "DataVolume" {
		return fmt.Errorf("Invalid controller for target clone pvc")
	} else if targetPvc.Status.Phase == corev1.ClaimLost {
		return fmt.Errorf("Target clone pvc claim lost")
	}
	return nil
}

func (r *CSICloneReconciler) reconcileSourcePvc(log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	// Get DataVolume of PVC
	dv := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: metav1.GetControllerOf(pvc).Name, Namespace: pvc.Annotations[AnnCSICloneDVNamespace]}, dv); err != nil {
		return reconcile.Result{}, err
	}

	if pvc.Status.Phase == corev1.ClaimBound {
		// Get PV bound to source clone PVC
		pv := &corev1.PersistentVolume{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Spec.VolumeName}, pv); err != nil {
			return reconcile.Result{}, err
		}
		// Deep copy pv object for mutation
		pvCopy := pv.DeepCopy()

		// Build new Target clone pvc
		targetClonerPvc := NewVolumeClonePVC(dv, *pvc.Spec.StorageClassName, pvc.Spec.AccessModes, CSICloneTargetPVC)
		// Set target clone pvc volumeName to PV of source clone PVC
		targetClonerPvc.Spec.VolumeName = pvCopy.Name

		// Create Target clone pvc
		if err := r.client.Create(context.TODO(), targetClonerPvc); err != nil {
			if k8serrors.IsAlreadyExists(err) {
				targetClonerPvcAlreadyExist := &corev1.PersistentVolumeClaim{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: targetClonerPvc.Name, Namespace: targetClonerPvc.Namespace}, targetClonerPvcAlreadyExist); err != nil {
					return reconcile.Result{}, err
				}

				if verifyTargetPVC(targetClonerPvcAlreadyExist) == nil {
					// Target clone pvc already exists, and is valid; delete Source clone PVC
					return reconcile.Result{}, r.client.Delete(context.TODO(), pvc)
				}
			}
			return reconcile.Result{}, err
		}

		// Get ObjectReference of target Clone PVC for PV ClaimRef
		claimRef, err := ref.GetReference(r.scheme, targetClonerPvc)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Set and update ClaimRef of PV to Target clone pvc
		pvCopy.Spec.ClaimRef = claimRef
		if err := r.client.Update(context.TODO(), pvCopy); err != nil {
			return reconcile.Result{}, err
		}

		// Delete Source clone pvc
		if err := r.client.Delete(context.TODO(), pvc); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if pvc.Status.Phase == corev1.ClaimLost {
		// If Source pvc claim is lost
		// Verify if target clone pvc valid
		targetClonerPvc := NewVolumeClonePVC(dv, *pvc.Spec.StorageClassName, pvc.Spec.AccessModes, CSICloneTargetPVC)
		targetPvc := &corev1.PersistentVolumeClaim{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: targetClonerPvc.Name, Namespace: targetClonerPvc.Namespace}, targetPvc); err != nil {
			if k8serrors.IsNotFound(err) {
				// Target clone pvc was either not created, or delete during cloning process
				// Set dv err status CloneSourcePVLost
				return reconcile.Result{}, r.updateDVStatus(cdiv1.CloneSourcePVLost, dv)
			}
			// Unable to get Target clone pvc for other reason; Requeue with error
			return reconcile.Result{}, err
		}
		if verifyTargetPVC(targetPvc) != nil {
			// Target clone pvc is not valid
			// Set dv err status CloneSourcePVLost
			return reconcile.Result{}, r.updateDVStatus(cdiv1.CloneSourcePVLost, dv)
		} else {
			// Target clone pvc successfully created, delete Source clone pvc
			if err := r.client.Delete(context.TODO(), pvc); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	return reconcile.Result{}, nil
}

func (r *CSICloneReconciler) reconcileTargetPVC(log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	if pvc.Status.Phase == corev1.ClaimBound {
		dv := &cdiv1.DataVolume{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: metav1.GetControllerOf(pvc).Name, Namespace: pvc.Namespace}, dv); err != nil {
			if k8serrors.IsNotFound(err) {
				// Datavolume deleted
				return reconcile.Result{}, nil
			}
		}
		return reconcile.Result{}, r.updateDVStatus(cdiv1.PVCBound, dv)
	}
	return reconcile.Result{}, nil
}

func (r *CSICloneReconciler) updateDVStatus(phase cdiv1.DataVolumePhase, dataVolume *cdiv1.DataVolume) error {
	var dataVolumeCopy = dataVolume.DeepCopy()
	var event DataVolumeEvent

	switch phase {
	case cdiv1.CloneSourcePVLost:
		dataVolumeCopy.Status.Phase = cdiv1.CloneSourcePVLost
		event.eventType = corev1.EventTypeWarning
		event.reason = string(cdiv1.CloneSourcePVLost)
		event.message = "Source PVC lost its PV binding during the cloning process."
	case cdiv1.CloneTargetPVCLost:
		dataVolumeCopy.Status.Phase = cdiv1.CloneTargetPVCLost
		event.eventType = corev1.EventTypeWarning
		event.reason = string(cdiv1.CloneTargetPVCLost)
		event.message = fmt.Sprintf("Target PVC %s lost during the cloning process.", dataVolume.Name)
	case cdiv1.PVCBound:
		dataVolumeCopy.Status.Phase = cdiv1.PVCBound
	}

	return r.emitEvent(dataVolume, dataVolumeCopy, &event)
}

func (r *CSICloneReconciler) emitEvent(dataVolume *cdiv1.DataVolume, dataVolumeCopy *cdiv1.DataVolume, event *DataVolumeEvent) error {
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
