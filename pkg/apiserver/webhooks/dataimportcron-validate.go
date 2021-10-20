/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2021 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"
	"fmt"

	"github.com/gorhill/cronexpr"

	admissionv1 "k8s.io/api/admission/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type dataImportCronValidatingWebhook struct {
	dataVolumeValidatingWebhook
}

func (wh *dataImportCronValidatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || ar.Request.Resource.Resource != "dataimportcrons" {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	raw := ar.Request.Object.Raw
	cron := cdiv1.DataImportCron{}
	err := json.Unmarshal(raw, &cron)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	causes := validateNameLength(cron.Name)
	if len(causes) > 0 {
		return toRejectedAdmissionResponse(causes)
	}

	if ar.Request.Operation == admissionv1.Update {
		oldCron := cdiv1.DataImportCron{}
		err = json.Unmarshal(ar.Request.OldObject.Raw, &oldCron)
		if err != nil {
			return toAdmissionResponseError(err)
		}
		if !apiequality.Semantic.DeepEqual(cron.Spec, oldCron.Spec) {
			klog.Errorf("Cannot update spec for DataImportCron %s/%s", cron.GetNamespace(), cron.GetName())
			var causes []metav1.StatusCause
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueDuplicate,
				Message: fmt.Sprintf("Cannot update DataImportCron Spec"),
				Field:   k8sfield.NewPath("DataImportCron").Child("Spec").String(),
			})
			return toRejectedAdmissionResponse(causes)
		}
		return allowedAdmissionResponse()
	}

	causes = wh.validateDataImportCronSpec(ar.Request, k8sfield.NewPath("spec"), &cron.Spec, &cron.Namespace)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission %s", causes)
		return toRejectedAdmissionResponse(causes)
	}

	return allowedAdmissionResponse()
}

func (wh *dataImportCronValidatingWebhook) validateDataImportCronSpec(request *admissionv1.AdmissionRequest, field *k8sfield.Path, spec *cdiv1.DataImportCronSpec, namespace *string) []metav1.StatusCause {
	var causes []metav1.StatusCause

	if spec.Template.Spec.Source == nil || spec.Template.Spec.Source.Registry == nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing registry source"),
			Field:   field.Child("Template").String(),
		})
		return causes
	}

	if spec.Template.Spec.SourceRef != nil ||
		spec.Template.Spec.ContentType != "" ||
		len(spec.Template.Spec.Checkpoints) > 0 ||
		spec.Template.Spec.FinalCheckpoint == true {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Unsettable fields: SourceRef, ContentType, Checkpoints, FinalCheckpoint"),
			Field:   field.Child("Template").String(),
		})
		return causes
	}

	causes = wh.validateDataVolumeSpec(request, k8sfield.NewPath("Template"), &spec.Template.Spec, nil)
	if len(causes) > 0 {
		return causes
	}

	if _, err := cronexpr.Parse(spec.Schedule); err != nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Illegal cron schedule"),
			Field:   field.Child("Schedule").String(),
		})
		return causes
	}

	if spec.GarbageCollect != nil &&
		*spec.GarbageCollect != cdiv1.DataImportCronGarbageCollectNever &&
		*spec.GarbageCollect != cdiv1.DataImportCronGarbageCollectOutdated {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Illegal GarbageCollect value"),
			Field:   field.Child("Schedule").String(),
		})
		return causes
	}

	if spec.ManagedDataSource == "" {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Illegal ManagedDataSource value"),
			Field:   field.Child("ManagedDataSource").String(),
		})
		return causes
	}

	causes = validateNameLength(spec.ManagedDataSource)
	if len(causes) > 0 {
		return causes
	}

	return causes
}
