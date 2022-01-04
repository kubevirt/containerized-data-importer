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
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	noDigest   = "NoDigest"
	noImport   = "NoImport"
	outdated   = "Outdated"
	scheduled  = "ImportScheduled"
	inProgress = "ImportProgressing"
	failed     = "ImportFailed"
	upToDate   = "UpToDate"
	ready      = "Ready"
)

func updateDataImportCronCondition(cron *cdiv1.DataImportCron, conditionType cdiv1.DataImportCronConditionType, status corev1.ConditionStatus, message, reason string) {
	if conditionType == cdiv1.DataImportCronUpToDate {
		if status != corev1.ConditionTrue {
			DataImportCronOutdatedGauge.With(getPrometheusCronLabels(cron)).Set(1)
		} else {
			DataImportCronOutdatedGauge.With(getPrometheusCronLabels(cron)).Set(0)
		}
	}
	if condition := FindDataImportCronConditionByType(cron, conditionType); condition != nil {
		updateConditionState(&condition.ConditionState, status, message, reason)
	} else {
		condition = &cdiv1.DataImportCronCondition{Type: conditionType}
		updateConditionState(&condition.ConditionState, status, message, reason)
		cron.Status.Conditions = append(cron.Status.Conditions, *condition)
	}
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

func updateDataSourceCondition(ds *cdiv1.DataSource, conditionType cdiv1.DataSourceConditionType, status corev1.ConditionStatus, message, reason string) {
	if condition := FindDataSourceConditionByType(ds, conditionType); condition != nil {
		updateConditionState(&condition.ConditionState, status, message, reason)
	} else {
		condition = &cdiv1.DataSourceCondition{Type: conditionType}
		updateConditionState(&condition.ConditionState, status, message, reason)
		ds.Status.Conditions = append(ds.Status.Conditions, *condition)
	}
}

// FindDataSourceConditionByType finds DataSourceCondition by condition type
func FindDataSourceConditionByType(ds *cdiv1.DataSource, conditionType cdiv1.DataSourceConditionType) *cdiv1.DataSourceCondition {
	for i, condition := range ds.Status.Conditions {
		if condition.Type == conditionType {
			return &ds.Status.Conditions[i]
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

func getPrometheusCronLabels(cron *cdiv1.DataImportCron) prometheus.Labels {
	return prometheus.Labels{
		prometheusNsLabel:       cron.GetNamespace(),
		prometheusCronNameLabel: cron.GetName(),
	}
}
