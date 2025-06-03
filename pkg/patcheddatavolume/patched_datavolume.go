package patcheddatavolume

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	QoutaNotExceededConditionType cdiv1.DataVolumeConditionType = "QuotaNotExceeded"

	QuotaNotExceededReason string = "QuotaNotExceeded"
	QuotaExceededReason    string = "QuotaExceeded"

	RunningConditionErrorReason string = "Error"
)

func FindConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func UpdateDVQuotaNotExceededCondition(conditions []cdiv1.DataVolumeCondition) []cdiv1.DataVolumeCondition {
	CreateDVQuotaIsNotExceededConditionIfNotExists(&conditions)
	readyCondition := FindConditionByType(cdiv1.DataVolumeReady, conditions)
	boundCondition := FindConditionByType(cdiv1.DataVolumeBound, conditions)
	runningCondition := FindConditionByType(cdiv1.DataVolumeRunning, conditions)

	switch {
	case readyCondition != nil && readyCondition.Reason == common.ErrExceededQuota:
		conditions = updateCondition(conditions, QoutaNotExceededConditionType, corev1.ConditionFalse, fmt.Sprintf("Exceeded quota: %q", readyCondition.Message), QuotaExceededReason)
	case boundCondition != nil && boundCondition.Reason == common.ErrExceededQuota:
		conditions = updateCondition(conditions, QoutaNotExceededConditionType, corev1.ConditionFalse, fmt.Sprintf("Exceeded quota: %q", boundCondition.Message), QuotaExceededReason)
	case runningCondition != nil:
		if runningCondition.Reason == common.ErrExceededQuota ||
			runningCondition.Reason == RunningConditionErrorReason && strings.Contains(runningCondition.Message, "exceeded quota") {
			conditions = updateCondition(conditions, QoutaNotExceededConditionType, corev1.ConditionFalse, fmt.Sprintf("Exceeded quota: %q", runningCondition.Message), QuotaExceededReason)
		} else if runningCondition.Status == corev1.ConditionTrue {
			conditions = updateCondition(conditions, QoutaNotExceededConditionType, corev1.ConditionTrue, "", QuotaNotExceededReason)
		}
	}

	return conditions
}

func UpdateDVQuotaNotExceededConditionByPVC(clientObject client.Client, pvc *corev1.PersistentVolumeClaim, status corev1.ConditionStatus, message, reason string) error {
	dv := getDVByPVC(clientObject, pvc, common.AnnCreatedForDataVolume)
	if dv == nil {
		return nil
	}

	dv.Status.Conditions = updateCondition(dv.Status.Conditions, QoutaNotExceededConditionType, status, message, reason)
	return clientObject.Status().Update(context.TODO(), dv)
}

func CreateDVQuotaIsNotExceededConditionIfNotExists(conditions *[]cdiv1.DataVolumeCondition) {
	if conditions == nil {
		return
	}

	condition := FindConditionByType(QoutaNotExceededConditionType, *conditions)
	if condition == nil {
		*conditions = append(*conditions, cdiv1.DataVolumeCondition{
			Type:    QoutaNotExceededConditionType,
			Status:  corev1.ConditionTrue,
			Reason:  QuotaNotExceededReason,
			Message: "",
		})
	}
}

func updateCondition(conditions []cdiv1.DataVolumeCondition, conditionType cdiv1.DataVolumeConditionType, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	condition := FindConditionByType(conditionType, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: conditionType,
		})
		condition = &conditions[len(conditions)-1]
	}
	if condition.Status != status {
		condition.LastTransitionTime = metav1.Now()
		condition.Message = message
		condition.Reason = reason
		condition.LastHeartbeatTime = condition.LastTransitionTime
	} else if condition.Message != message || condition.Reason != reason {
		condition.Message = message
		condition.Reason = reason
		condition.LastHeartbeatTime = metav1.Now()
	}
	condition.Status = status
	return conditions
}

func getDVByPVC(clientObject client.Client, pvc *corev1.PersistentVolumeClaim, ann string) *cdiv1.DataVolume {
	uid, ok := pvc.Annotations[ann]
	if !ok {
		return nil
	}

	var dvList cdiv1.DataVolumeList

	err := clientObject.List(context.TODO(), &dvList, client.InNamespace(pvc.Namespace))
	if err != nil {
		return nil
	}

	for _, dv := range dvList.Items {
		if string(dv.UID) == uid {
			return &dv
		}
	}

	return nil
}
