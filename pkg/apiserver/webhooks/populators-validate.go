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
 * Copyright 2023 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	importResource = "volumeimportsources"
	uploadResource = "volumeuploadsources"
)

type populatorValidatingWebhook struct {
	dataVolumeValidatingWebhook
}

func isPopulatorSource(ar admissionv1.AdmissionReview) bool {
	return ar.Request.Resource.Resource == importResource || ar.Request.Resource.Resource == uploadResource
}

func (wh *populatorValidatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var causes []metav1.StatusCause
	var err error

	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || !isPopulatorSource(ar) {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	raw := ar.Request.Object.Raw
	switch ar.Request.Resource.Resource {
	case importResource:
		causes, err = wh.validateVolumeImportSource(ar, raw)
	case uploadResource:
		causes, err = wh.validateVolumeUploadSource(ar, raw)
	}

	if err != nil {
		return toAdmissionResponseError(err)
	}
	if causes != nil {
		return toRejectedAdmissionResponse(causes)
	}

	return allowedAdmissionResponse()
}

// Upload validation

func (wh *populatorValidatingWebhook) validateVolumeUploadSource(ar admissionv1.AdmissionReview, raw []byte) ([]metav1.StatusCause, error) {
	volumeUploadSource := cdiv1.VolumeUploadSource{}
	err := json.Unmarshal(raw, &volumeUploadSource)
	if err != nil {
		return nil, err
	}

	// Reject spec updates
	if ar.Request.Operation == admissionv1.Update {
		oldSource := cdiv1.VolumeUploadSource{}
		err = json.Unmarshal(ar.Request.OldObject.Raw, &oldSource)
		if err != nil {
			return nil, err
		}

		if !apiequality.Semantic.DeepEqual(volumeUploadSource.Spec, oldSource.Spec) {
			klog.Errorf("Cannot update spec for VolumeUploadSource %s/%s", volumeUploadSource.GetNamespace(), volumeUploadSource.GetName())
			return []metav1.StatusCause{{
				Type:    metav1.CauseTypeFieldValueDuplicate,
				Message: "Cannot update VolumeUploadSource Spec",
				Field:   k8sfield.NewPath("VolumeUploadSource").Child("Spec").String(),
			}}, nil
		}
	}

	cause := validateNameLength(volumeUploadSource.Name, validation.DNS1035LabelMaxLength)
	if cause != nil {
		return []metav1.StatusCause{*cause}, nil
	}

	causes := wh.validateVolumeUploadSourceSpec(k8sfield.NewPath("spec"), &volumeUploadSource.Spec)
	if causes != nil {
		klog.Infof("rejected VolumeUploadSource admission %s", causes)
		return causes, nil
	}

	return nil, nil
}

func (wh *populatorValidatingWebhook) validateVolumeUploadSourceSpec(field *k8sfield.Path, spec *cdiv1.VolumeUploadSourceSpec) []metav1.StatusCause {
	// Make sure contentType is either empty (kubevirt), or kubevirt or archive
	if causes := validateContentType(spec.ContentType, field); causes != nil {
		return causes
	}

	return nil
}

// Import validation

func (wh *populatorValidatingWebhook) validateVolumeImportSource(ar admissionv1.AdmissionReview, raw []byte) ([]metav1.StatusCause, error) {
	volumeImportSource := cdiv1.VolumeImportSource{}
	err := json.Unmarshal(raw, &volumeImportSource)
	if err != nil {
		return nil, err
	}

	// Reject spec updates
	if ar.Request.Operation == admissionv1.Update {
		cause, err := wh.validateVolumeImportSourceUpdate(ar, &volumeImportSource)
		if err != nil {
			return nil, err
		}
		if cause != nil {
			return cause, nil
		}
	}

	cause := validateNameLength(volumeImportSource.Name, validation.DNS1035LabelMaxLength)
	if cause != nil {
		return []metav1.StatusCause{*cause}, nil
	}

	causes := wh.validateVolumeImportSourceSpec(k8sfield.NewPath("spec"), &volumeImportSource.Spec)
	if causes != nil {
		klog.Infof("rejected VolumeImportSource admission %s", causes)
		return causes, nil
	}

	return nil, nil
}

func (wh *populatorValidatingWebhook) validateVolumeImportSourceSpec(field *k8sfield.Path, spec *cdiv1.VolumeImportSourceSpec) []metav1.StatusCause {
	// Make sure contentType is either empty (kubevirt), or kubevirt or archive
	if causes := validateContentType(spec.ContentType, field); causes != nil {
		return causes
	}

	if causes := validateNumberOfSources(spec.Source, "VolumeImport", field); causes != nil {
		return causes
	}

	// validate multi-stage import
	if isMultiStageImport(spec) && (spec.TargetClaim == nil || *spec.TargetClaim == "") {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: "Unable to do multi-stage import without specifying a target claim",
			Field:   field.Child("targetClaim").String(),
		}}
	}

	// Validate import sources
	if http := spec.Source.HTTP; http != nil {
		return validateHTTPSource(http, field)
	}
	if s3 := spec.Source.S3; s3 != nil {
		return validateS3Source(s3, field)
	}
	if gcs := spec.Source.GCS; gcs != nil {
		return validateGCSSource(gcs, field)
	}
	if blank := spec.Source.Blank; blank != nil {
		return validateBlankSource(spec.ContentType, field)
	}
	if registry := spec.Source.Registry; registry != nil {
		return validateRegistrySource(registry, spec.ContentType, field)
	}
	if imageio := spec.Source.Imageio; imageio != nil {
		return validateImageIOSource(imageio, field)
	}
	if vddk := spec.Source.VDDK; vddk != nil {
		return validateVDDKSource(vddk, field)
	}
	// Should never reach this return
	return nil
}

func (wh *populatorValidatingWebhook) validateVolumeImportSourceUpdate(ar admissionv1.AdmissionReview, volumeImportSource *cdiv1.VolumeImportSource) ([]metav1.StatusCause, error) {
	oldSource := cdiv1.VolumeImportSource{}
	err := json.Unmarshal(ar.Request.OldObject.Raw, &oldSource)
	if err != nil {
		return nil, err
	}
	newSpec := volumeImportSource.Spec.DeepCopy()
	oldSpec := oldSource.Spec.DeepCopy()

	// Always admit checkpoint updates for multi-stage migrations.
	if isMultiStageImport(newSpec) {
		oldSpec.FinalCheckpoint = pointer.Bool(false)
		oldSpec.Checkpoints = nil
		newSpec.FinalCheckpoint = pointer.Bool(false)
		newSpec.Checkpoints = nil
	}

	// Reject all other updates
	if !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		klog.Errorf("Cannot update spec for VolumeImportSource %s/%s", volumeImportSource.GetNamespace(), volumeImportSource.GetName())
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueDuplicate,
			Message: "Cannot update VolumeImportSource Spec",
			Field:   k8sfield.NewPath("VolumeImportSource").Child("Spec").String(),
		}}, nil
	}

	return nil, nil
}

func isMultiStageImport(spec *cdiv1.VolumeImportSourceSpec) bool {
	return spec.Source != nil && len(spec.Checkpoints) > 0 &&
		(spec.Source.VDDK != nil || spec.Source.Imageio != nil)
}
