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
	"time"

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

func updateCondition(conditions []cdiv1.DataVolumeCondition, conditionType cdiv1.DataVolumeConditionType, status corev1.ConditionStatus, message, reason string, lastHeartBeat time.Time) []cdiv1.DataVolumeCondition {
	condition := findConditionByType(conditionType, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: conditionType,
		})
		condition = findConditionByType(conditionType, conditions)
	}
	if condition.Status != status || lastHeartBeat.After(condition.LastHeartBeatTime.Time) {
		condition.LastTransitionTime = metav1.Now()
		condition.Message = message
		condition.Reason = reason
		condition.LastHeartBeatTime = metav1.NewTime(lastHeartBeat)
	}
	condition.Status = status
	return conditions
}

func updateRunningCondition(conditions []cdiv1.DataVolumeCondition, anno map[string]string) []cdiv1.DataVolumeCondition {
	condition := findConditionByType(cdiv1.DataVolumeRunning, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeRunning,
		})
		condition = findConditionByType(cdiv1.DataVolumeRunning, conditions)
	}
	heartBeat, err := time.Parse(time.RFC3339Nano, anno[AnnRunningConditionHeartBeat])
	if err != nil {
		heartBeat = time.Time{}
	}
	if val, ok := anno[AnnRunningCondition]; ok {
		if strings.ToLower(val) == "true" {
			if condition.Status != corev1.ConditionTrue || heartBeat.After(condition.LastHeartBeatTime.Time) {
				condition.LastTransitionTime = metav1.Now()
				condition.Message = anno[AnnLastTerminationMessage]
				condition.Reason = anno[AnnRunningConditionReason]
				condition.LastHeartBeatTime = metav1.NewTime(heartBeat)
			}
			condition.Status = corev1.ConditionTrue
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", transferRunning, heartBeat)
		} else if strings.ToLower(val) == "false" {
			if condition.Status != corev1.ConditionFalse || heartBeat.After(condition.LastHeartBeatTime.Time) {
				condition.LastTransitionTime = metav1.Now()
				condition.Message = anno[AnnLastTerminationMessage]
				condition.Reason = anno[AnnRunningConditionReason]
				condition.LastHeartBeatTime = metav1.NewTime(heartBeat)
			}
			condition.Status = corev1.ConditionFalse
		} else {
			if condition.Status != corev1.ConditionUnknown || heartBeat.After(condition.LastHeartBeatTime.Time) {
				condition.LastTransitionTime = metav1.Now()
				condition.Message = anno[AnnLastTerminationMessage]
				condition.Reason = anno[AnnRunningConditionReason]
				condition.LastHeartBeatTime = metav1.NewTime(heartBeat)
			}
			condition.Status = corev1.ConditionUnknown
		}
	} else {
		condition.Message = anno[AnnLastTerminationMessage]
		condition.Reason = anno[AnnRunningConditionReason]
		condition.LastTransitionTime = metav1.Now()
		condition.LastHeartBeatTime = metav1.Now()
		condition.Status = corev1.ConditionUnknown
	}
	return conditions
}

func updateReadyCondition(conditions []cdiv1.DataVolumeCondition, status corev1.ConditionStatus, message, reason string, lastHeartBeat time.Time) []cdiv1.DataVolumeCondition {
	return updateCondition(conditions, cdiv1.DataVolumeReady, status, message, reason, lastHeartBeat)
}

func updateBoundCondition(conditions []cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim) []cdiv1.DataVolumeCondition {
	condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
	if condition == nil {
		conditions = append(conditions, cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeBound,
		})
		condition = findConditionByType(cdiv1.DataVolumeBound, conditions)
	}
	if pvc != nil {
		switch pvc.Status.Phase {
		case corev1.ClaimBound:
			if condition.Reason != pvcBound {
				condition.LastTransitionTime = metav1.Now()
			}
			condition.Status = corev1.ConditionTrue
			condition.Message = "PVC Bound"
			condition.Reason = pvcBound
			condition.LastHeartBeatTime = metav1.Now()
		case corev1.ClaimPending:
			if condition.Reason != pvcPending {
				condition.LastTransitionTime = metav1.Now()
			}
			condition.Status = corev1.ConditionFalse
			condition.Message = "PVC Pending"
			condition.Reason = pvcPending
			condition.LastHeartBeatTime = metav1.Now()
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", "", time.Now())
		case corev1.ClaimLost:
			if condition.Reason != claimLost {
				condition.LastTransitionTime = metav1.Now()
			}
			condition.Status = corev1.ConditionFalse
			condition.Message = "Claim Lost"
			condition.Reason = claimLost
			condition.LastHeartBeatTime = metav1.Now()
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse, time.Now())
		default:
			condition.Status = corev1.ConditionUnknown
			condition.Message = "PVC phase unknown"
			condition.Reason = string(corev1.ConditionUnknown)
			condition.LastHeartBeatTime = metav1.Now()
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse, time.Now())
		}
	} else {
		if condition.Reason != notFound {
			condition.LastTransitionTime = metav1.Now()
		}
		condition.Status = corev1.ConditionUnknown
		condition.Message = "No PVC found"
		condition.Reason = notFound
		condition.LastHeartBeatTime = metav1.Now()
		conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse, time.Now())
	}
	return conditions
}
