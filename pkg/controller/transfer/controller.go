/*
Copyright 2018 The CDI Authors.

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

package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	// AnnObjectTransferName holds the name of related ObjectTransfer resource
	AnnObjectTransferName = "cdi.kubevirt.io/objectTransferName"

	objectTransferFinalizer = "cdi.kubevirt.io/objectTransfer"

	defaultRequeue = 2 * time.Second
)

type transferHandler interface {
	ReconcilePending(*cdiv1.ObjectTransfer) (time.Duration, error)
	ReconcileRunning(*cdiv1.ObjectTransfer) (time.Duration, error)
}

type objectTransferHandler struct {
	reconciler *ObjectTransferReconciler
}

// ObjectTransferReconciler members
type ObjectTransferReconciler struct {
	Client   client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Log      logr.Logger
}

func getTransferTargetName(ot *cdiv1.ObjectTransfer) string {
	if ot.Spec.Target.Name != nil {
		return *ot.Spec.Target.Name
	}

	return ot.Spec.Source.Name
}

func getTransferTargetNamespace(ot *cdiv1.ObjectTransfer) string {
	if ot.Spec.Target.Namespace != nil {
		return *ot.Spec.Target.Namespace
	}

	return ot.Spec.Source.Namespace
}

// NewObjectTransferController creates a new instance of the ObjectTransfer controller.
func NewObjectTransferController(mgr manager.Manager, log logr.Logger) (controller.Controller, error) {
	name := "transfer-controller"
	client := mgr.GetClient()
	reconciler := &ObjectTransferReconciler{
		Client:   client,
		Scheme:   mgr.GetScheme(),
		Log:      log.WithName(name),
		Recorder: mgr.GetEventRecorderFor(name),
	}

	ctrl, err := controller.New(name, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := addObjectTransferControllerWatches(mgr, ctrl); err != nil {
		return nil, err
	}

	return ctrl, nil
}

func (r *ObjectTransferReconciler) logger(ot *cdiv1.ObjectTransfer) logr.Logger {
	return r.Log.WithValues(
		"namespace", ot.Namespace,
		"name", ot.Name,
		"kind", ot.Spec.Source.Kind,
		"sName", ot.Spec.Source.Name,
		"tNamespace", getTransferTargetNamespace(ot),
		"tName", getTransferTargetName(ot),
	)
}

// Reconcile the reconcile loop for the data volumes.
func (r *ObjectTransferReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	ot := &cdiv1.ObjectTransfer{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, ot); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	r.logger(ot).V(1).Info("Handling request")

	switch ot.Status.Phase {
	case cdiv1.ObjectTransferEmpty:

		return r.reconcileEmpty(ot)
	case cdiv1.ObjectTransferPending:
		handler, err := r.getHandler(ot)
		if err != nil {
			return reconcile.Result{}, err
		}

		if ot.DeletionTimestamp != nil {
			return r.reconcileCleanup(ot)
		}

		if requeue, err := handler.ReconcilePending(ot); requeue > 0 || err != nil {
			return reconcile.Result{RequeueAfter: requeue}, err
		}
	case cdiv1.ObjectTransferRunning, cdiv1.ObjectTransferError:
		if ot.DeletionTimestamp != nil && ot.Status.Phase == cdiv1.ObjectTransferError {
			return r.reconcileCleanup(ot)
		}

		handler, err := r.getHandler(ot)
		if err != nil {
			return reconcile.Result{}, err
		}

		if requeue, err := handler.ReconcileRunning(ot); requeue > 0 || err != nil {
			return reconcile.Result{RequeueAfter: requeue}, err
		}
	case cdiv1.ObjectTransferComplete:

		return r.reconcileCleanup(ot)
	}

	return reconcile.Result{}, nil
}

func (r *ObjectTransferReconciler) reconcileEmpty(ot *cdiv1.ObjectTransfer) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(ot, objectTransferFinalizer) {
		controllerutil.AddFinalizer(ot, objectTransferFinalizer)
		if err := r.updateResource(ot, ot); err != nil {
			return reconcile.Result{}, err
		}
	}

	ot.Status.Phase = cdiv1.ObjectTransferPending
	if err := r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Initializing", ""); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ObjectTransferReconciler) reconcileCleanup(ot *cdiv1.ObjectTransfer) (reconcile.Result, error) {
	if controllerutil.ContainsFinalizer(ot, objectTransferFinalizer) {
		controllerutil.RemoveFinalizer(ot, objectTransferFinalizer)
		return reconcile.Result{}, r.updateResource(ot, ot)
	}

	return reconcile.Result{}, nil
}

func (r *ObjectTransferReconciler) getHandler(ot *cdiv1.ObjectTransfer) (transferHandler, error) {
	switch strings.ToLower(ot.Spec.Source.Kind) {
	case "datavolume":
		return &dataVolumeTransferHandler{
			objectTransferHandler: objectTransferHandler{
				reconciler: r,
			},
		}, nil
	case "persistentvolumeclaim":
		return &pvcTransferHandler{
			objectTransferHandler: objectTransferHandler{
				reconciler: r,
			},
		}, nil
	}

	return nil, fmt.Errorf("invalid kind %q", ot.Spec.Source.Kind)
}

func (r *ObjectTransferReconciler) getResource(ns, name string, obj client.Object) (bool, error) {
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (r *ObjectTransferReconciler) updateResource(ot *cdiv1.ObjectTransfer, obj client.Object) error {
	log := r.logger(ot)

	log.V(1).Info("Updating resource", "obj", obj)
	if err := r.Client.Update(context.TODO(), obj); err != nil {
		log.Error(err, "Update error")
		return err
	}

	return nil
}

func (r *ObjectTransferReconciler) updateResourceStatus(ot *cdiv1.ObjectTransfer, obj client.Object) error {
	log := r.logger(ot)

	log.V(1).Info("Updating resource status", "obj", obj)
	if err := r.Client.Status().Update(context.TODO(), obj); err != nil {
		log.Error(err, "Update status error")
		return err
	}

	return nil
}

func (r *ObjectTransferReconciler) getSourceResource(ot *cdiv1.ObjectTransfer, obj client.Object) (bool, error) {
	return r.getResource(ot.Spec.Source.Namespace, ot.Spec.Source.Name, obj)
}

func (r *ObjectTransferReconciler) getTargetResource(ot *cdiv1.ObjectTransfer, obj client.Object) (bool, error) {
	return r.getResource(getTransferTargetNamespace(ot), getTransferTargetName(ot), obj)
}

func (r *ObjectTransferReconciler) getCondition(ot *cdiv1.ObjectTransfer, t cdiv1.ObjectTransferConditionType) *cdiv1.ObjectTransferCondition {
	for i := range ot.Status.Conditions {
		c := &ot.Status.Conditions[i]
		if c.Type == t {
			return c
		}
	}

	return nil
}

func (r *ObjectTransferReconciler) setAndUpdateCompleteCondition(ot *cdiv1.ObjectTransfer, status corev1.ConditionStatus, message, reason string) error {
	return r.setAndUpdateCondition(ot, cdiv1.ObjectTransferConditionComplete, status, message, reason)
}

func (r *ObjectTransferReconciler) setCompleteConditionError(ot *cdiv1.ObjectTransfer, lastError error) error {
	if ot.Status.Phase == cdiv1.ObjectTransferRunning {
		ot.Status.Phase = cdiv1.ObjectTransferError
	}

	if err := r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Error", lastError.Error()); err != nil {
		return err
	}

	return lastError
}

func (r *ObjectTransferReconciler) setCompleteConditionRunning(ot *cdiv1.ObjectTransfer) error {
	ot.Status.Phase = cdiv1.ObjectTransferRunning
	return r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Running", "")
}

func (r *ObjectTransferReconciler) setAndUpdateCondition(ot *cdiv1.ObjectTransfer, t cdiv1.ObjectTransferConditionType, status corev1.ConditionStatus, message, reason string) error {
	r.logger(ot).V(1).Info("Updating condition", "type", t, "status", status, "message", message, "reason", reason)

	ot2 := &cdiv1.ObjectTransfer{}
	if _, err := r.getResource("", ot.Name, ot2); err != nil {
		return err
	}

	if r.setCondition(ot, t, status, message, reason) || !apiequality.Semantic.DeepEqual(ot.Status, ot2.Status) {
		return r.updateResourceStatus(ot, ot)
	}

	return nil
}

func (r *ObjectTransferReconciler) setCondition(ot *cdiv1.ObjectTransfer, t cdiv1.ObjectTransferConditionType, status corev1.ConditionStatus, message, reason string) bool {
	updated := false
	cond := r.getCondition(ot, t)
	if cond == nil {
		ot.Status.Conditions = append(ot.Status.Conditions, cdiv1.ObjectTransferCondition{Type: t})
		cond = &ot.Status.Conditions[len(ot.Status.Conditions)-1]
		updated = true
	}

	if cond.Status != status {
		cond.Status = status
		updated = true
	}

	if cond.Message != message {
		cond.Message = message
		updated = true
	}

	if cond.Reason != reason {
		cond.Reason = reason
		updated = true
	}

	if updated {
		cond.LastTransitionTime = metav1.Now()
		cond.LastHeartbeatTime = cond.LastTransitionTime
	}

	return updated
}

func (r *ObjectTransferReconciler) createObjectTransferTarget(ot *cdiv1.ObjectTransfer, obj client.Object, mutateFn func(client.Object)) error {
	s, ok := ot.Status.Data["source"]
	if !ok {
		return fmt.Errorf("source spec missing")
	}

	if err := json.Unmarshal([]byte(s), obj); err != nil {
		return err
	}

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	metaObj.SetNamespace(getTransferTargetNamespace(ot))
	metaObj.SetName(getTransferTargetName(ot))
	metaObj.SetGenerateName("")
	metaObj.SetUID("")
	metaObj.SetResourceVersion("")
	metaObj.SetGeneration(0)
	metaObj.SetSelfLink("")
	metaObj.SetCreationTimestamp(metav1.Time{})
	metaObj.SetDeletionTimestamp(nil)
	metaObj.SetManagedFields(nil)

	if ot.Spec.Target.Namespace != nil && *ot.Spec.Target.Namespace != ot.Spec.Source.Namespace {
		metaObj.SetOwnerReferences(nil)
	}

	delete(metaObj.GetAnnotations(), AnnObjectTransferName)

	if mutateFn != nil {
		mutateFn(obj)
	}

	return r.Client.Create(context.TODO(), obj)
}

func (r *ObjectTransferReconciler) pendingHelper(ot *cdiv1.ObjectTransfer, obj client.Object, data map[string]string) error {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return r.setCompleteConditionError(ot, err)
	}

	if !r.hasRequiredAnnotations(ot, metaObj) {
		if err := r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Required annotation missing", ""); err != nil {
			return err
		}

		return nil
	}

	v, ok := metaObj.GetAnnotations()[AnnObjectTransferName]
	if ok && v != ot.Name {
		if err := r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Source in use by another transfer", v); err != nil {
			return err
		}

		return nil
	}

	if !ok {
		if metaObj.GetAnnotations() == nil {
			metaObj.SetAnnotations(make(map[string]string))
		}
		metaObj.GetAnnotations()[AnnObjectTransferName] = ot.Name
		if err := r.updateResource(ot, obj); err != nil {
			return r.setCompleteConditionError(ot, err)
		}

		if err := r.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Pending", ""); err != nil {
			return err
		}
	}

	bs, err := json.Marshal(obj)
	if err != nil {
		return r.setCompleteConditionError(ot, err)
	}

	source := string(bs)
	ot.Status.Data = map[string]string{
		"source": source,
	}

	for k, v := range data {
		ot.Status.Data[k] = v
	}

	ot.Status.Phase = cdiv1.ObjectTransferRunning
	if err := r.setCompleteConditionRunning(ot); err != nil {
		return err
	}

	return nil
}

func (r *ObjectTransferReconciler) hasRequiredAnnotations(ot *cdiv1.ObjectTransfer, obj metav1.Object) bool {
	for rk, rv := range ot.Spec.Source.RequiredAnnotations {
		if v, ok := obj.GetAnnotations()[rk]; !ok || v != rv {
			return false
		}
	}

	return true
}
