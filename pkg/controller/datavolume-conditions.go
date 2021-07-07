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

package controller

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	transferRunning = "TransferRunning"
	pvcBound        = "Bound"
	pvcPending      = "Pending"
	claimLost       = "ClaimLost"
	notFound        = "NotFound"
)

func findConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func updateCondition(conditions []cdiv1.DataVolumeCondition, conditionType cdiv1.DataVolumeConditionType, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	condition := findConditionByType(conditionType, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: conditionType,
		})
		condition = findConditionByType(conditionType, conditions)
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
	if val, ok := anno[AnnRunningCondition]; ok {
		switch strings.ToLower(val) {
		case "true":
			conditions = updateWithTargetRunning(conditions, anno)
		case "false":
			conditions = updateWithTargetNotRunning(conditions, anno)
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
	}
	return conditions
}

func updateWithTargetRunning(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	if sourceRunningVal, ok := anno[AnnSourceRunningCondition]; ok {
		switch strings.ToLower(sourceRunningVal) {
		case "true":
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning)
		case "false":
			// target running, source not running, overall not running.
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[AnnSourceRunningConditionMessage], anno[AnnSourceRunningConditionReason])
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[AnnSourceRunningConditionMessage], anno[AnnSourceRunningConditionReason])
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
		conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning)
	}
	return conditions
}

func updateWithTargetNotRunning(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	if sourceRunningVal, ok := anno[AnnSourceRunningCondition]; ok {
		switch strings.ToLower(sourceRunningVal) {
		case "true":
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
		case "false":
			// target not running and source not running, overall not running.
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, fmt.Sprintf("%s and %s", anno[AnnRunningConditionMessage], anno[AnnSourceRunningConditionMessage]), fmt.Sprintf("%s and %s", anno[AnnRunningConditionReason], anno[AnnSourceRunningConditionReason]))
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, fmt.Sprintf("%s and %s", anno[AnnRunningConditionMessage], anno[AnnSourceRunningConditionMessage]), fmt.Sprintf("%s and %s", anno[AnnRunningConditionReason], anno[AnnSourceRunningConditionReason]))
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[AnnRunningConditionMessage], anno[AnnRunningConditionReason])
	}
	return conditions
}

func updateReadyCondition(conditions []cdiv1.DataVolumeCondition, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	return updateCondition(conditions, cdiv1.DataVolumeReady, status, message, reason)
}

func updateBoundCondition(conditions []cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim) []cdiv1.DataVolumeCondition {
	if pvc != nil {
		pvcCondition := getPVCCondition(pvc.GetAnnotations())
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if pvcCondition == nil || pvcCondition.Status == corev1.ConditionTrue {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionTrue, fmt.Sprintf("PVC %s Bound", pvc.Name), pvcBound)
			} else {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, pvcCondition.Message, pvcCondition.Reason)
				conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			}
		case corev1.ClaimPending:
			if pvcCondition == nil || pvcCondition.Status == corev1.ConditionTrue {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, fmt.Sprintf("PVC %s Pending", pvc.Name), pvcPending)
				conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			} else {
				conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, fmt.Sprintf("target PVC %s Pending and %s", pvc.Name, pvcCondition.Message), pvcCondition.Reason)
				conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
			}
		case corev1.ClaimLost:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, "Claim Lost", claimLost)
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionUnknown, fmt.Sprintf("PVC %s phase unknown", pvc.Name), string(corev1.ConditionUnknown))
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionUnknown, "No PVC found", notFound)
		conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
	}
	return conditions
}

func getPVCCondition(anno map[string]string) *cdiv1.DataVolumeCondition {
	if val, ok := anno[AnnBoundCondition]; ok {
		status := corev1.ConditionUnknown
		if strings.ToLower(val) == "true" {
			status = corev1.ConditionTrue
		} else if strings.ToLower(val) == "false" {
			status = corev1.ConditionFalse
		}
		return &cdiv1.DataVolumeCondition{
			Message: anno[AnnBoundConditionMessage],
			Reason:  anno[AnnBoundConditionReason],
			Status:  status,
		}
	}
	return nil
}
