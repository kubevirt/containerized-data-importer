package controller

import (
	"context"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	// AnnCSICloneRequest annotation associates object with CSI Clone Request
	AnnCSICloneRequest = "cdi.kubevirt.io/CSICloneRequest"
)

// CSICloneReconciler is the CSI clone reconciler
type CSICloneReconciler struct {
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
}

// NewCSICloneController creates an instance of the CSICloneReconciler
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
	isOwnedByDatavolume := controllingDv != nil && controllingDv.Kind == "DataVolume"

	val, ok := pvc.Annotations[AnnCSICloneRequest]
	cloneRequestAnnVal, _ := strconv.ParseBool(val)
	isCSICloneRequest := ok && cloneRequestAnnVal

	return isCSICloneRequest && isOwnedByDatavolume
}

// Reconcile the reconcile loop for csi cloning
func (r *CSICloneReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PersistentVolumeClaim", req.NamespacedName)
	log.Info("reconciling csi clone")

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("pvc not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	return r.reconcilePvc(log, pvc)
}

func (r *CSICloneReconciler) reconcilePvc(log logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	log.WithValues("pvc.Name", pvc.Name).
		WithValues("pvc.Namespace", pvc.Namespace).
		Info("Reconciling PVC")

	if pvc.Status.Phase == corev1.ClaimBound {
		if v, ok := pvc.Annotations[AnnCloneOf]; !ok || v != "true" {
			if pvc.Annotations == nil {
				pvc.Annotations = make(map[string]string)
			}
			pvc.Annotations[AnnCloneOf] = "true"

			if err := r.client.Update(context.TODO(), pvc); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// TODO: is there any work for ClaimLost, and ClaimPending
	// update annotations / events
	log.WithValues("pvc.Name", pvc.Name).
		WithValues("pvc.Namespace", pvc.Namespace).
		Info("--> NOP PVC Pending")
	return reconcile.Result{}, nil
}
