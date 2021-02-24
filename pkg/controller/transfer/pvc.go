package transfer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdicontroller "kubevirt.io/containerized-data-importer/pkg/controller"
)

type pvcTransferHandler struct {
	objectTransferHandler
}

func (h *pvcTransferHandler) ReconcilePending(ot *cdiv1.ObjectTransfer) (time.Duration, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvcExists, err := h.reconciler.getSourceResource(ot, pvc)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if !pvcExists {
		// will reconcile again when pvc is created/updated
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "No source", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	if pvc.Spec.VolumeName == "" || pvc.Status.Phase != corev1.ClaimBound {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "PVC not bound", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	pods, err := cdicontroller.GetPodsUsingPVCs(h.reconciler.Client, pvc.Namespace, sets.NewString(pvc.Name), false)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if len(pods) > 0 {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Pods using PVC", ""); err != nil {
			return 0, err
		}

		return defaultRequeue, nil
	}

	pvc2 := pvc.DeepCopy()
	pvc2.Status = corev1.PersistentVolumeClaimStatus{}
	data := map[string]string{
		"pvName": pvc.Spec.VolumeName,
	}

	return 0, h.reconciler.pendingHelper(ot, pvc2, data)
}

func (h *pvcTransferHandler) ReconcileRunning(ot *cdiv1.ObjectTransfer) (time.Duration, error) {
	pvName, ok := ot.Status.Data["pvName"]
	if !ok {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "PV name missing", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	pv := &corev1.PersistentVolume{}
	if err := h.reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: pvName}, pv); err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	reclaim, ok := ot.Status.Data["pvReclaim"]
	if !ok {
		ot.Status.Data["pvReclaim"] = string(pv.Spec.PersistentVolumeReclaimPolicy)
		if err := h.reconciler.setCompleteConditionRunning(ot); err != nil {
			return 0, err
		}

		return 0, nil
	}

	source := &corev1.PersistentVolumeClaim{}
	sourceExists, err := h.reconciler.getSourceResource(ot, source)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	target := &corev1.PersistentVolumeClaim{}
	targetExists, err := h.reconciler.getTargetResource(ot, target)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if sourceExists {
		if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			if err := h.reconciler.updateResource(ot, pv); err != nil {
				return 0, h.reconciler.setCompleteConditionError(ot, err)
			}

			return 0, h.reconciler.setCompleteConditionRunning(ot)
		}

		if source.DeletionTimestamp == nil {
			if err := h.reconciler.Client.Delete(context.TODO(), source); err != nil {
				return 0, h.reconciler.setCompleteConditionError(ot, err)
			}
		}

		return 0, h.reconciler.setCompleteConditionRunning(ot)
	}

	if !targetExists {
		if pv.Spec.ClaimRef != nil {
			pv.Spec.ClaimRef = nil
			if err := h.reconciler.updateResource(ot, pv); err != nil {
				return 0, h.reconciler.setCompleteConditionError(ot, err)
			}

			return 0, h.reconciler.setCompleteConditionRunning(ot)
		}

		target = &corev1.PersistentVolumeClaim{}
		if err := h.reconciler.createObjectTransferTarget(ot, target, nil); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}
	}

	if target.Status.Phase != corev1.ClaimBound {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Waiting for target to be bound", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	if pv.Spec.ClaimRef.Namespace != target.Namespace || pv.Spec.ClaimRef.Name != target.Name {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "PV bound to wrong PVC", ""); err != nil {
			return 0, err
		}

		// TODO what to do here
		return 0, fmt.Errorf("PV bound to wrong PVC")
	}

	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimPolicy(reclaim) {
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimPolicy(reclaim)
		if err := h.reconciler.updateResource(ot, pv); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}

		return 0, h.reconciler.setCompleteConditionRunning(ot)
	}

	ot.Status.Phase = cdiv1.ObjectTransferComplete
	ot.Status.Data = nil
	if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionTrue, "Transfer complete", ""); err != nil {
		return 0, err
	}

	return 0, nil
}
