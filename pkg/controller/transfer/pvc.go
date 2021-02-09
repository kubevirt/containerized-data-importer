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
		return 0, h.reconciler.setConditionError(ot, err)
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

	pods, err := cdicontroller.GetPodsUsingPVCs(h.reconciler.client, pvc.Namespace, sets.NewString(pvc.Name), false)
	if err != nil {
		return 0, h.reconciler.setConditionError(ot, err)
	}

	if len(pods) > 0 {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Pods using PVC", ""); err != nil {
			return 0, err
		}

		return defaultRequeue, nil
	}

	v, ok := pvc.Annotations[AnnObjectTransferName]
	if ok && v != ot.Name {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Source in use by another transfer", v); err != nil {
			return 0, err
		}

		return 0, nil
	}

	pvc2 := pvc.DeepCopy()
	pvc2.Status = corev1.PersistentVolumeClaimStatus{}
	data := map[string]string{
		"pvName": pvc.Spec.VolumeName,
	}

	return 0, h.reconciler.pendingHelper(ot, pvc2, data)
}

func (h *pvcTransferHandler) CancelPending(ot *cdiv1.ObjectTransfer) error {
	return h.reconciler.cancelHelper(ot, &corev1.PersistentVolumeClaim{})
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
	if err := h.reconciler.client.Get(context.TODO(), types.NamespacedName{Name: pvName}, pv); err != nil {
		return 0, h.reconciler.setConditionError(ot, err)
	}

	reclaim, ok := ot.Status.Data["pvReclaim"]
	if !ok {
		ot.Status.Data["pvReclaim"] = string(pv.Spec.PersistentVolumeReclaimPolicy)
		if err := h.reconciler.updateResourceStatus(ot, ot); err != nil {
			return 0, err
		}

		return 0, nil
	}

	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		if err := h.reconciler.updateResource(ot, pv); err != nil {
			return 0, h.reconciler.setConditionError(ot, err)
		}

		return 0, nil
	}

	source := &corev1.PersistentVolumeClaim{}
	sourceExists, err := h.reconciler.getSourceResource(ot, source)
	if err != nil {
		return 0, h.reconciler.setConditionError(ot, err)
	}

	if sourceExists {
		if source.DeletionTimestamp == nil {
			if err := h.reconciler.client.Delete(context.TODO(), source); err != nil {
				return 0, h.reconciler.setConditionError(ot, err)
			}
		}

		return 0, nil
	}

	target := &corev1.PersistentVolumeClaim{}
	targetExists, err := h.reconciler.getTargetResource(ot, target)
	if err != nil {
		return 0, h.reconciler.setConditionError(ot, err)
	}

	if !targetExists {
		if pv.Spec.ClaimRef != nil {
			pv.Spec.ClaimRef = nil
			if err := h.reconciler.updateResource(ot, pv); err != nil {
				return 0, h.reconciler.setConditionError(ot, err)
			}

			return 0, nil
		}

		target = &corev1.PersistentVolumeClaim{}
		if err := h.reconciler.createObjectTransferTarget(ot, target, nil); err != nil {
			return 0, h.reconciler.setConditionError(ot, err)
		}

		return 0, nil
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
			return 0, h.reconciler.setConditionError(ot, err)
		}
	}

	ot.Status.Phase = cdiv1.ObjectTransferComplete
	ot.Status.Data = nil
	if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionTrue, "Transfer complete", ""); err != nil {
		return 0, err
	}

	return 0, nil
}
