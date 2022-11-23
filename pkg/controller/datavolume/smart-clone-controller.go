/*
Copyright 2022 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
limitations under the License.
See the License for the specific language governing permissions and
*/

package datavolume

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	//AnnSmartCloneRequest sets our expected annotation for a CloneRequest
	AnnSmartCloneRequest = "k8s.io/SmartCloneRequest"

	annSmartCloneSnapshot = "cdi.kubevirt.io/smartCloneSnapshot"
)

// SmartCloneReconciler members
type SmartCloneReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	installerLabels map[string]string
}

// NewSmartCloneController creates a new instance of the Smart clone controller.
func NewSmartCloneController(mgr manager.Manager, log logr.Logger, installerLabels map[string]string) (controller.Controller, error) {
	reconciler := &SmartCloneReconciler{
		client:          mgr.GetClient(),
		scheme:          mgr.GetScheme(),
		log:             log.WithName("smartclone-controller"),
		recorder:        mgr.GetEventRecorderFor("smartclone-controller"),
		installerLabels: installerLabels,
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
	if err := smartCloneController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			if hasAnnOwnedByDataVolume(pvc) && shouldReconcilePvc(pvc) {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						},
					},
				}
			}
			return nil
		},
	)); err != nil {
		return err
	}

	// check if volume snapshots exist
	err := mgr.GetClient().List(context.TODO(), &snapshotv1.VolumeSnapshotList{})
	if meta.IsNoMatchError(err) {
		return nil
	}

	if err != nil && !cc.IsErrCacheNotStarted(err) {
		return err
	}

	if err := smartCloneController.Watch(&source.Kind{Type: &snapshotv1.VolumeSnapshot{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			snapshot := obj.(*snapshotv1.VolumeSnapshot)
			if hasAnnOwnedByDataVolume(snapshot) && shouldReconcileSnapshot(snapshot) {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					},
				}
			}
			return nil
		},
	)); err != nil {
		return err
	}

	return nil
}

func shouldReconcileSnapshot(snapshot *snapshotv1.VolumeSnapshot) bool {
	_, ok := snapshot.GetAnnotations()[AnnSmartCloneRequest]
	return ok
}

func shouldReconcilePvc(pvc *corev1.PersistentVolumeClaim) bool {
	val, ok := pvc.GetAnnotations()[AnnSmartCloneRequest]
	return ok && val == "true"
}

// Reconcile the reconcile loop for smart cloning.
func (r *SmartCloneReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("VolumeSnapshot/PersistentVolumeClaim", req.NamespacedName)
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
	log.WithValues("pvc.Name", pvc.Name).WithValues("pvc.Namespace", pvc.Namespace).Info("Reconciling PVC")

	snapshotName, hasSnapshot := pvc.Annotations[annSmartCloneSnapshot]

	// Don't delete snapshot unless the PVC is bound.
	if hasSnapshot && pvc.Status.Phase == corev1.ClaimBound {
		namespace, name, err := cache.SplitMetaNamespaceKey(snapshotName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if err := r.deleteSnapshot(log, namespace, name); err != nil {
			return reconcile.Result{}, err
		}

		if v, ok := pvc.Annotations[cc.AnnCloneOf]; !ok || v != "true" {
			if pvc.Annotations == nil {
				pvc.Annotations = make(map[string]string)
			}
			pvc.Annotations[cc.AnnCloneOf] = "true"

			if err := r.client.Update(context.TODO(), pvc); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *SmartCloneReconciler) reconcileSnapshot(log logr.Logger, snapshot *snapshotv1.VolumeSnapshot) (reconcile.Result, error) {
	log.WithValues("snapshot.Name", snapshot.Name).
		WithValues("snapshot.Namespace", snapshot.Namespace).
		Info("Reconciling snapshot")

	if snapshot.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	dataVolume, err := r.getDataVolume(snapshot)
	if err != nil {
		return reconcile.Result{}, err
	}

	if dataVolume == nil || dataVolume.DeletionTimestamp != nil {
		if err := r.deleteSnapshot(log, snapshot.Namespace, snapshot.Name); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// pvc may have been transferred
	targetPVC, err := r.getTargetPVC(dataVolume)
	if err != nil {
		return reconcile.Result{}, err
	}

	if targetPVC != nil {
		return reconcile.Result{}, nil
	}

	if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse {
		// wait for ready to use
		return reconcile.Result{}, nil
	}

	targetPvcSpec, err := renderPvcSpec(r.client, r.recorder, r.log, dataVolume)
	if err != nil {
		return reconcile.Result{}, err
	}
	newPvc, err := newPvcFromSnapshot(snapshot, targetPvcSpec)
	if err != nil {
		return reconcile.Result{}, err
	}
	util.SetRecommendedLabels(newPvc, r.installerLabels, "cdi-controller")

	if err := setAnnOwnedByDataVolume(newPvc, dataVolume); err != nil {
		return reconcile.Result{}, err
	}
	//passing annotations from the target DV to the matching target PVC
	if len(dataVolume.GetAnnotations()) > 0 {
		for k, v := range dataVolume.GetAnnotations() {
			if !strings.Contains(k, common.CDIAnnKey) {
				newPvc.Annotations[k] = v
			}
		}
	}
	if snapshot.Spec.Source.PersistentVolumeClaimName != nil {
		event := &Event{
			eventType: corev1.EventTypeNormal,
			reason:    SmartClonePVCInProgress,
			message:   fmt.Sprintf(MessageSmartClonePVCInProgress, snapshot.Namespace, *snapshot.Spec.Source.PersistentVolumeClaimName),
		}

		r.emitEvent(snapshot, event)
	}

	log.V(3).Info("Creating PVC from snapshot", "pvc.Namespace", newPvc.Namespace, "pvc.Name", newPvc.Name)
	if err := r.client.Create(context.TODO(), newPvc); err != nil {
		if cc.ErrQuotaExceeded(err) {
			event := &Event{
				eventType: corev1.EventTypeWarning,
				reason:    cc.ErrExceededQuota,
				message:   err.Error(),
			}

			r.emitEvent(snapshot, event)
		}
		log.Error(err, "error creating pvc from snapshot")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *SmartCloneReconciler) deleteSnapshot(log logr.Logger, namespace, name string) error {
	snapshotToDelete := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, snapshotToDelete); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if snapshotToDelete.DeletionTimestamp != nil {
		return nil
	}

	if err := r.client.Delete(context.TODO(), snapshotToDelete); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Error(err, "error deleting snapshot for smart-clone")
			return err
		}
	}

	log.V(3).Info("Snapshot deleted")
	return nil
}

func (r *SmartCloneReconciler) emitEvent(snapshot *snapshotv1.VolumeSnapshot, event *Event) {
	if event.eventType != "" {
		r.recorder.Event(snapshot, event.eventType, event.reason, event.message)
	}
}

func (r *SmartCloneReconciler) getDataVolume(snapshot *snapshotv1.VolumeSnapshot) (*cdiv1.DataVolume, error) {
	namespace, name, err := getAnnOwnedByDataVolume(snapshot)
	if err != nil {
		return nil, err
	}

	dataVolume := &cdiv1.DataVolume{}
	nn := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.client.Get(context.TODO(), nn, dataVolume); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return dataVolume, nil
}

func (r *SmartCloneReconciler) getTargetPVC(dataVolume *cdiv1.DataVolume) (*corev1.PersistentVolumeClaim, error) {
	// TODO update when PVC name may differ from DataVolume
	pvc := &corev1.PersistentVolumeClaim{}
	nn := types.NamespacedName{Name: dataVolume.Name, Namespace: dataVolume.Namespace}
	if err := r.client.Get(context.TODO(), nn, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return pvc, nil
}

func newPvcFromSnapshot(snapshot *snapshotv1.VolumeSnapshot, targetPvcSpec *corev1.PersistentVolumeClaimSpec) (*corev1.PersistentVolumeClaim, error) {
	restoreSize := snapshot.Status.RestoreSize
	if restoreSize == nil {
		return nil, fmt.Errorf("snapshot has no RestoreSize")
	}

	key, err := cache.MetaNamespaceKeyFunc(snapshot)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"cdi-controller":         snapshot.Name,
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: common.SmartClonerCDILabel,
	}
	if util.ResolveVolumeMode(targetPvcSpec.VolumeMode) == corev1.PersistentVolumeFilesystem {
		labels[common.KubePersistentVolumeFillingUpSuppressLabelKey] = common.KubePersistentVolumeFillingUpSuppressLabelValue
	}

	target := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				AnnSmartCloneRequest:          "true",
				cc.AnnRunningCondition:        string(corev1.ConditionFalse),
				cc.AnnRunningConditionMessage: cc.CloneComplete,
				cc.AnnRunningConditionReason:  "Completed",
				annSmartCloneSnapshot:         key,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSource: &corev1.TypedLocalObjectReference{
				Name:     snapshot.Name,
				Kind:     "VolumeSnapshot",
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
			},
			VolumeMode:       targetPvcSpec.VolumeMode,
			AccessModes:      targetPvcSpec.AccessModes,
			StorageClassName: targetPvcSpec.StorageClassName,
			Resources:        targetPvcSpec.Resources,
		},
	}

	if target.Spec.Resources.Requests == nil {
		target.Spec.Resources.Requests = corev1.ResourceList{}
	}

	target.Spec.Resources.Requests[corev1.ResourceStorage] = *restoreSize

	ownerRef := metav1.GetControllerOf(snapshot)
	if ownerRef != nil {
		target.OwnerReferences = append(target.OwnerReferences, *ownerRef)
	}

	return target, nil
}
