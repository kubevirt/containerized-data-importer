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
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const (
	boundFalse  = "BoundChangeFalse"
	runningTrue = "RunningTrue"
	pvcBound    = "PVCBound"
	pvcPending  = "PVCPending"
	claimLost   = "ClaimLost"
	notFound    = "NotFound"
)

func findConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []*cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition
		}
	}
	return nil
}

func updateCondition(conditions []*cdiv1.DataVolumeCondition, conditionType cdiv1.DataVolumeConditionType, status corev1.ConditionStatus, message, reason string) []*cdiv1.DataVolumeCondition {
	condition := findConditionByType(conditionType, conditions)
	if condition == nil {
		condition = &cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeReady,
		}
		conditions = append(conditions, condition)
	}
	condition.Status = status
	condition.Message = message
	condition.Reason = reason
	return conditions
}

func updateRunningCondition(conditions []*cdiv1.DataVolumeCondition, anno map[string]string) []*cdiv1.DataVolumeCondition {
	condition := findConditionByType(cdiv1.DataVolumeRunning, conditions)
	if condition == nil {
		condition = &cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeRunning,
		}
		conditions = append(conditions, condition)
	}
	if val, ok := anno[AnnRunningConditionMessage]; ok {
		condition.Message = val
	} else {
		condition.Message = ""
	}
	if val, ok := anno[AnnRunningCondition]; ok {
		if strings.ToLower(val) == "true" {
			condition.Status = corev1.ConditionTrue
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", runningTrue)
		} else if strings.ToLower(val) == "false" {
			condition.Status = corev1.ConditionFalse
		} else {
			condition.Status = corev1.ConditionUnknown
		}
	} else {
		condition.Status = corev1.ConditionUnknown
	}
	if val, ok := anno[AnnRunningConditionReason]; ok {
		condition.Reason = val
	} else {
		condition.Reason = ""
	}
	return conditions
}

func updateReadyCondition(conditions []*cdiv1.DataVolumeCondition, status corev1.ConditionStatus, message, reason string) []*cdiv1.DataVolumeCondition {
	return updateCondition(conditions, cdiv1.DataVolumeReady, status, message, reason)
}

func updateBoundCondition(conditions []*cdiv1.DataVolumeCondition, pvc *corev1.PersistentVolumeClaim) []*cdiv1.DataVolumeCondition {
	condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
	if condition == nil {
		condition = &cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeBound,
		}
		conditions = append(conditions, condition)
	}
	if pvc != nil {
		if pvc.Status.Phase == corev1.ClaimBound {
			condition.Status = corev1.ConditionTrue
			condition.Message = "PVC Bound"
			condition.Reason = pvcBound
		} else if pvc.Status.Phase == corev1.ClaimPending {
			condition.Status = corev1.ConditionFalse
			condition.Message = "PVC Pending"
			condition.Reason = pvcPending
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
		} else if pvc.Status.Phase == corev1.ClaimLost {
			condition.Status = corev1.ConditionFalse
			condition.Message = "Claim Lost"
			condition.Reason = claimLost
			conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
		}
	} else {
		condition.Status = corev1.ConditionUnknown
		condition.Message = "No PVC found"
		condition.Reason = notFound
		conditions = updateReadyCondition(conditions, corev1.ConditionFalse, "", boundFalse)
	}
	return conditions
}
