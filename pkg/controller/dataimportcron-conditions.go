/*
Copyright 2021 The CDI Authors.

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

package controller

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	noDigest   = "NoDigest"
	noImport   = "NoImport"
	outdated   = "Outdated"
	scheduled  = "ImportScheduled"
	inProgress = "ImportProgressing"
	upToDate   = "UpToDate"
)

func updateDataImportCronCondition(cron *cdiv1.DataImportCron, conditionType cdiv1.DataImportCronConditionType, status corev1.ConditionStatus, message, reason string) {
	if conditionType == cdiv1.DataImportCronUpToDate {
		updateDataImportCronOutdatedMetric(cron, status)
	}

	if condition := FindDataImportCronConditionByType(cron, conditionType); condition != nil {
		updateConditionState(&condition.ConditionState, status, message, reason)
	} else {
		condition = &cdiv1.DataImportCronCondition{Type: conditionType}
		updateConditionState(&condition.ConditionState, status, message, reason)
		cron.Status.Conditions = append(cron.Status.Conditions, *condition)
	}
}

func updateDataImportCronOutdatedMetric(cron *cdiv1.DataImportCron, status corev1.ConditionStatus) {
	gaugeVal := float64(0)
	isUpToDate := status == corev1.ConditionTrue
	isPending := false
	// Check if the DataImportCron import DV is pending for default k8s/virt storage class
	if !isUpToDate {
		gaugeVal = 1
		_, scExists := cron.Annotations[AnnStorageClass]
		isPending = !scExists && common.IsDataVolumeUsingDefaultStorageClass(&cron.Spec.Template)
	}

	labels := getPrometheusCronLabels(cron.Namespace, cron.Name)
	DataImportCronOutdatedGauge.DeletePartialMatch(getPrometheusCronLabels(cron.Namespace, cron.Name))

	labels[prometheusCronPendingLabel] = strconv.FormatBool(isPending)
	DataImportCronOutdatedGauge.With(labels).Set(gaugeVal)
}

// FindDataImportCronConditionByType finds DataImportCronCondition by condition type
func FindDataImportCronConditionByType(cron *cdiv1.DataImportCron, conditionType cdiv1.DataImportCronConditionType) *cdiv1.DataImportCronCondition {
	for i, condition := range cron.Status.Conditions {
		if condition.Type == conditionType {
			return &cron.Status.Conditions[i]
		}
	}
	return nil
}

func updateConditionState(condition *cdiv1.ConditionState, status corev1.ConditionStatus, message, reason string) {
	conditionStatusUpdated := condition.Status != status
	conditionUpdated := conditionStatusUpdated || condition.Message != message || condition.Reason != reason
	if conditionUpdated {
		now := metav1.Now()
		if conditionStatusUpdated {
			condition.Status = status
			condition.LastTransitionTime = now
		}
		condition.LastHeartbeatTime = now
		condition.Reason = reason
		condition.Message = message
	}
}

func getPrometheusCronLabels(cronNamespace, cronName string) prometheus.Labels {
	return prometheus.Labels{
		prometheusCronNsLabel:   cronNamespace,
		prometheusCronNameLabel: cronName,
	}
}
