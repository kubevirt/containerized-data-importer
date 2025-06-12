/*
Copyright 2020 The CDI Authors.

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

package datavolume

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	transferRunning = "TransferRunning"
	pvcBound        = "Bound"
	pvcPending      = "Pending"
)

// FindConditionByType finds condition by type
func FindConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func updateCondition(conditions []cdiv1.DataVolumeCondition, conditionType cdiv1.DataVolumeConditionType, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	condition := FindConditionByType(conditionType, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: conditionType,
		})
		condition = FindConditionByType(conditionType, conditions)
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

func updateRunningCondition(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	if val, ok := anno[cc.AnnRunningCondition]; ok {
		switch strings.ToLower(val) {
		case "true":
			conditions = updateWithTargetRunning(conditions, anno)
		case "false":
			conditions = updateWithTargetNotRunning(conditions, anno)
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
	}
	return conditions
}

func updateWithTargetRunning(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	if sourceRunningVal, ok := anno[cc.AnnSourceRunningCondition]; ok {
		switch strings.ToLower(sourceRunningVal) {
		case "true":
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
			conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning)
		case "false":
			// target running, source not running, overall not running.
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[cc.AnnSourceRunningConditionMessage], anno[cc.AnnSourceRunningConditionReason])
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[cc.AnnSourceRunningConditionMessage], anno[cc.AnnSourceRunningConditionReason])
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
		conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning)
	}
	return conditions
}

func updateWithTargetNotRunning(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	if sourceRunningVal, ok := anno[cc.AnnSourceRunningCondition]; ok {
		switch strings.ToLower(sourceRunningVal) {
		case "true":
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
		case "false":
			// target not running and source not running, overall not running.
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, fmt.Sprintf("%s and %s", anno[cc.AnnRunningConditionMessage], anno[cc.AnnSourceRunningConditionMessage]), fmt.Sprintf("%s and %s", anno[cc.AnnRunningConditionReason], anno[cc.AnnSourceRunningConditionReason]))
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, fmt.Sprintf("%s and %s", anno[cc.AnnRunningConditionMessage], anno[cc.AnnSourceRunningConditionMessage]), fmt.Sprintf("%s and %s", anno[cc.AnnRunningConditionReason], anno[cc.AnnSourceRunningConditionReason]))
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[cc.AnnRunningConditionMessage], anno[cc.AnnRunningConditionReason])
	}
	return conditions
}

// UpdateReadyCondition updates the ready condition
func UpdateReadyCondition(conditions []cdiv1.DataVolumeCondition, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	return updateCondition(conditions, cdiv1.DataVolumeReady, status, message, reason)
}

func updateBoundCondition(conditions []cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim, message, reason string) []cdiv1.DataVolumeCondition {
	if pvc != nil {
		pvcCondition := getPVCCondition(pvc.GetAnnotations())
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if pvcCondition == nil || pvcCondition.Status == corev1.ConditionTrue {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionTrue, fmt.Sprintf("PVC %s Bound", pvc.Name), pvcBound)
			} else {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, pvcCondition.Message, pvcCondition.Reason)
				conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			}
		case corev1.ClaimPending:
			pvcPrimeMessage := getPrimeMessage(pvc)
			if pvcCondition == nil || pvcCondition.Status == corev1.ConditionTrue {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, fmt.Sprintf("PVC %s Pending%s", pvc.Name, pvcPrimeMessage), pvcPending)
				conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			} else {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, fmt.Sprintf("target PVC %s Pending%s and %s", pvc.Name, pvcPrimeMessage, pvcCondition.Message), pvcCondition.Reason)
				conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			}
		case corev1.ClaimLost:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, "Claim Lost", cc.ClaimLost)
			conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, "", "")
			conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
		}
	} else {
		if message == "" {
			message = "No PVC found"
		}
		if reason == "" {
			reason = cc.NotFound
		}
		conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, message, reason)
		conditions = UpdateReadyCondition(conditions, corev1.ConditionFalse, "", "")
	}
	return conditions
}

func getPVCCondition(anno map[string]string) *cdiv1.DataVolumeCondition {
	if val, ok := anno[cc.AnnBoundCondition]; ok {
		status := corev1.ConditionUnknown
		if strings.ToLower(val) == "true" {
			status = corev1.ConditionTrue
		} else if strings.ToLower(val) == "false" {
			status = corev1.ConditionFalse
		}
		return &cdiv1.DataVolumeCondition{
			Message: anno[cc.AnnBoundConditionMessage],
			Reason:  anno[cc.AnnBoundConditionReason],
			Status:  status,
		}
	}
	return nil
}

func getPrimeMessage(pvc *corev1.PersistentVolumeClaim) string {
	val, exists := pvc.GetAnnotations()[cc.AnnPVCPrimeName]
	if exists {
		pvcPrimeMessage := fmt.Sprintf(" [prime PVC %s]", val)
		return pvcPrimeMessage
	}
	return ""
}
