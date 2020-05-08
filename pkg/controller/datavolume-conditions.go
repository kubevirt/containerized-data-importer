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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	boundFalse      = "Bound changed to false"
	transferRunning = "Transfer is running"
	pvcBound        = "PVC is bound"
	pvcPending      = "PVC is pending"
	claimLost       = "Claim lost"
	notFound        = "Not found"
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
		if strings.ToLower(val) == "true" {
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionTrue, anno[AnnLastTerminationMessage], anno[AnnLastTerminationReason])
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning)
		} else if strings.ToLower(val) == "false" {
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionFalse, anno[AnnLastTerminationMessage], anno[AnnLastTerminationReason])
		} else {
			conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[AnnLastTerminationMessage], anno[AnnLastTerminationReason])
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeRunning, corev1.ConditionUnknown, anno[AnnLastTerminationMessage], anno[AnnLastTerminationReason])
	}
	return conditions
}

func updateReadyCondition(conditions []cdiv1.DataVolumeCondition, status corev1.ConditionStatus, message, reason string) []cdiv1.DataVolumeCondition {
	return updateCondition(conditions, cdiv1.DataVolumeReady, status, message, reason)
}

func updateBoundCondition(conditions []cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim) []cdiv1.DataVolumeCondition {
	if pvc != nil {
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionTrue, "PVC Bound", pvcBound)
		case corev1.ClaimPending:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, "PVC Pending", pvcPending)
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "")
		case corev1.ClaimLost:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionFalse, "Claim Lost", claimLost)
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
		default:
			conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionUnknown, "PVC phase unknown", string(corev1.ConditionUnknown))
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
		}
	} else {
		conditions = updateCondition(conditions, cdiv1.DataVolumeBound, corev1.ConditionUnknown, "No PVC found", notFound)
		conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
	}
	return conditions
}
