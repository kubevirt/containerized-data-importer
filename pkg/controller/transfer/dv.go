package transfer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdicontroller "kubevirt.io/containerized-data-importer/pkg/controller"
)

type dataVolumeTransferHandler struct {
	objectTransferHandler
}

func (h *dataVolumeTransferHandler) ReconcilePending(ot *cdiv1.ObjectTransfer) (time.Duration, error) {
	dv := &cdiv1.DataVolume{}
	dvExists, err := h.reconciler.getSourceResource(ot, dv)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if !dvExists {
		// will reconcile again when dv is created/updated
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "No source", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	if dv.Status.Phase != cdiv1.Succeeded {
		// will reconcile again when dv is updated
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Source not populated", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	pods, err := cdicontroller.GetPodsUsingPVCs(h.reconciler.Client, dv.Namespace, sets.NewString(cdicontroller.GetDataVolumeClaimName(dv)), false)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if len(pods) > 0 {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Pods using DataVolume PVC", ""); err != nil {
			return 0, err
		}

		return defaultRequeue, nil
	}

	dv2 := dv.DeepCopy()
	dv2.Status = cdiv1.DataVolumeStatus{}
	data := map[string]string{
		"pvcName": cdicontroller.GetDataVolumeClaimName(dv),
	}

	return 0, h.reconciler.pendingHelper(ot, dv2, data)
}

func (h *dataVolumeTransferHandler) ReconcileRunning(ot *cdiv1.ObjectTransfer) (time.Duration, error) {
	dv := &cdiv1.DataVolume{}
	dvExists, err := h.reconciler.getSourceResource(ot, dv)
	if err != nil {
		return 0, err
	}

	if dvExists {
		return h.deleteDataVolume(ot, dv)
	}

	pvcTransferName := fmt.Sprintf("pvc-transfer-%s", ot.UID)

	pvcTransfer := &cdiv1.ObjectTransfer{}
	pvcTransferExists, err := h.reconciler.getResource("", pvcTransferName, pvcTransfer)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	target := &cdiv1.DataVolume{}
	targetExists, err := h.reconciler.getTargetResource(ot, target)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	pvcName := ot.Status.Data["pvcName"]

	if !targetExists && !pvcTransferExists {
		targetNamespace := getTransferTargetNamespace(ot)
		targetName := getTransferTargetName(ot)
		pvcTransfer = &cdiv1.ObjectTransfer{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcTransferName,
			},
			Spec: cdiv1.ObjectTransferSpec{
				Source: cdiv1.TransferSource{
					Kind:      "PersistentVolumeClaim",
					Namespace: ot.Spec.Source.Namespace,
					Name:      pvcName,
				},
				Target: cdiv1.TransferTarget{
					Namespace: &targetNamespace,
					Name:      &targetName,
				},
				ParentName: &ot.Name,
			},
		}

		if err := h.reconciler.Client.Create(context.TODO(), pvcTransfer); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}

		pvcTransferExists = true
	}

	if pvcTransferExists {
		if pvcTransfer.Status.Phase != cdiv1.ObjectTransferComplete {
			ot.Status.Phase = cdiv1.ObjectTransferRunning
			if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "PVC transfer in progress", ""); err != nil {
				return 0, err
			}

			return 0, nil
		}

		pvc := &corev1.PersistentVolumeClaim{}
		pvcExists, err := h.reconciler.getTargetResource(pvcTransfer, pvc)
		if err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}

		if !pvcExists {
			ot.Status.Phase = cdiv1.ObjectTransferError
			if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Transferred PVC does not exist", ""); err != nil {
				return 0, err
			}

			return 0, nil
		}

		if err := h.addPopulatedAnnotation(ot, pvc); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}
	}

	if !targetExists {
		target = &cdiv1.DataVolume{}
		if err := h.reconciler.createObjectTransferTarget(ot, target, nil); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}
	}

	if target.Status.Phase != cdiv1.Succeeded {
		ot.Status.Phase = cdiv1.ObjectTransferRunning
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Waiting for target DataVolume", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	if pvcTransferExists && pvcTransfer.DeletionTimestamp == nil {
		if err := h.reconciler.Client.Delete(context.TODO(), pvcTransfer); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}
	}

	ot.Status.Phase = cdiv1.ObjectTransferComplete
	ot.Status.Data = nil
	if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionTrue, "Transfer complete", ""); err != nil {
		return 0, err
	}

	return 0, nil
}

func (h *dataVolumeTransferHandler) deleteDataVolume(ot *cdiv1.ObjectTransfer, dv *cdiv1.DataVolume) (time.Duration, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvcExists, err := h.reconciler.getResource(dv.Namespace, cdicontroller.GetDataVolumeClaimName(dv), pvc)
	if err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	if !pvcExists {
		if err := h.reconciler.setAndUpdateCompleteCondition(ot, corev1.ConditionFalse, "Source DV has no PVC", ""); err != nil {
			return 0, err
		}

		return 0, nil
	}

	idx := -1
	for i, o := range pvc.OwnerReferences {
		if o.Kind == "DataVolume" &&
			o.Name == dv.Name &&
			o.UID == dv.UID {
			idx = i
			break
		}
	}

	if idx >= 0 {
		os := pvc.OwnerReferences
		pvc.OwnerReferences = append(os[0:idx], os[idx+1:]...)
	}

	_, ok := pvc.Annotations[cdicontroller.AnnPopulatedFor]
	if ok {
		delete(pvc.Annotations, cdicontroller.AnnPopulatedFor)
	}

	if idx >= 0 || ok {
		if err := h.reconciler.updateResource(ot, pvc); err != nil {
			return 0, h.reconciler.setCompleteConditionError(ot, err)
		}

		return time.Second, h.reconciler.setCompleteConditionRunning(ot)
	}

	if err := h.reconciler.Client.Delete(context.TODO(), dv); err != nil {
		return 0, h.reconciler.setCompleteConditionError(ot, err)
	}

	return 0, h.reconciler.setCompleteConditionRunning(ot)
}

func (h *dataVolumeTransferHandler) addPopulatedAnnotation(ot *cdiv1.ObjectTransfer, pvc *corev1.PersistentVolumeClaim) error {
	dvName := getTransferTargetName(ot)

	if v, ok := pvc.Annotations[cdicontroller.AnnPopulatedFor]; ok {
		if v == dvName {
			return nil
		}

		return fmt.Errorf("PVC populated for a different DataVolume")
	}

	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}

	pvc.Annotations[cdicontroller.AnnPopulatedFor] = dvName

	return h.reconciler.updateResource(ot, pvc)
}
