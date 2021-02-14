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
 * Copyright 2019 Red Hat, Inc.
 *
 */

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
)

type dataVolumeValidatingWebhook struct {
	client kubernetes.Interface
}

func validateSourceURL(sourceURL string) string {
	if sourceURL == "" {
		return "source URL is empty"
	}
	url, err := url.ParseRequestURI(sourceURL)
	if err != nil {
		return fmt.Sprintf("Invalid source URL: %s", sourceURL)
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Sprintf("Invalid source URL scheme: %s", sourceURL)
	}
	return ""
}

func validateDataVolumeName(name string) []metav1.StatusCause {
	var causes []metav1.StatusCause
	if len(name) > kvalidation.DNS1123SubdomainMaxLength {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Name of data volume cannot be more than %d characters", kvalidation.DNS1123SubdomainMaxLength),
			Field:   "",
		})
	}
	return causes
}

func validateContentTypes(sourcePVC *v1.PersistentVolumeClaim, spec *cdiv1.DataVolumeSpec) (bool, cdiv1.DataVolumeContentType, cdiv1.DataVolumeContentType) {
	sourceContentType := cdiv1.DataVolumeContentType(controller.GetContentType(sourcePVC))
	targetContentType := spec.ContentType
	if targetContentType == "" {
		targetContentType = cdiv1.DataVolumeKubeVirt
	}
	return sourceContentType == targetContentType, sourceContentType, targetContentType
}

func (wh *dataVolumeValidatingWebhook) validateDataVolumeSpec(request *v1beta1.AdmissionRequest, field *k8sfield.Path, spec *cdiv1.DataVolumeSpec) []metav1.StatusCause {
	var causes []metav1.StatusCause
	var url string
	var sourceType string

	numberOfSources := 0
	s := reflect.ValueOf(&spec.Source).Elem()
	for i := 0; i < s.NumField(); i++ {
		if !reflect.ValueOf(s.Field(i).Interface()).IsNil() {
			numberOfSources++
		}
	}
	if numberOfSources == 0 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing Data volume source"),
			Field:   field.Child("source").String(),
		})
		return causes
	}
	if numberOfSources > 1 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Multiple Data volume sources"),
			Field:   field.Child("source").String(),
		})
		return causes
	}
	// if source types are HTTP, Imageio, S3 or VDDK, check if URL is valid
	if spec.Source.HTTP != nil || spec.Source.S3 != nil || spec.Source.Imageio != nil || spec.Source.VDDK != nil {
		if spec.Source.HTTP != nil {
			url = spec.Source.HTTP.URL
			sourceType = field.Child("source", "HTTP", "url").String()
		} else if spec.Source.S3 != nil {
			url = spec.Source.S3.URL
			sourceType = field.Child("source", "S3", "url").String()
		} else if spec.Source.Imageio != nil {
			url = spec.Source.Imageio.URL
			sourceType = field.Child("source", "Imageio", "url").String()
		} else if spec.Source.VDDK != nil {
			url = spec.Source.VDDK.URL
			sourceType = field.Child("source", "VDDK", "url").String()
		}
		err := validateSourceURL(url)
		if err != "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s %s", field.Child("source").String(), err),
				Field:   sourceType,
			})
			return causes
		}
	}

	// Make sure contentType is either empty (kubevirt), or kubevirt or archive
	if spec.ContentType != "" && string(spec.ContentType) != string(cdiv1.DataVolumeKubeVirt) && string(spec.ContentType) != string(cdiv1.DataVolumeArchive) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType not one of: %s, %s", cdiv1.DataVolumeKubeVirt, cdiv1.DataVolumeArchive),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.Blank != nil && string(spec.ContentType) == string(cdiv1.DataVolumeArchive) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("SourceType cannot be blank and the contentType be archive"),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.Registry != nil && spec.ContentType != "" && string(spec.ContentType) != string(cdiv1.DataVolumeKubeVirt) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType must be " + string(cdiv1.DataVolumeKubeVirt) + " when Source is Registry"),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.Imageio != nil {
		if spec.Source.Imageio.SecretRef == "" || spec.Source.Imageio.CertConfigMap == "" || spec.Source.Imageio.DiskID == "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s source Imageio is not valid", field.Child("source", "Imageio").String()),
				Field:   field.Child("source", "Imageio").String(),
			})
			return causes
		}
	}

	if spec.Source.VDDK != nil {
		if spec.Source.VDDK.SecretRef == "" || spec.Source.VDDK.UUID == "" || spec.Source.VDDK.BackingFile == "" || spec.Source.VDDK.Thumbprint == "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s source VDDK is not valid", field.Child("source", "VDDK").String()),
				Field:   field.Child("source", "VDDK").String(),
			})
			return causes
		}
	}

	if spec.Source.PVC != nil {
		if spec.Source.PVC.Namespace == "" || spec.Source.PVC.Name == "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s source PVC is not valid", field.Child("source", "PVC").String()),
				Field:   field.Child("source", "PVC").String(),
			})
			return causes
		}

		if request.Operation == v1beta1.Create {
			sourcePVC, err := wh.client.CoreV1().PersistentVolumeClaims(spec.Source.PVC.Namespace).Get(context.TODO(), spec.Source.PVC.Name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					causes = append(causes, metav1.StatusCause{
						Type:    metav1.CauseTypeFieldValueNotFound,
						Message: fmt.Sprintf("Source PVC %s/%s doesn't exist", spec.Source.PVC.Namespace, spec.Source.PVC.Name),
						Field:   field.Child("source", "PVC").String(),
					})
					return causes
				}
			}
			valid, sourceContentType, targetContentType := validateContentTypes(sourcePVC, spec)
			if !valid {
				causes = append(causes, metav1.StatusCause{
					Type:    metav1.CauseTypeFieldValueInvalid,
					Message: fmt.Sprintf("Source contentType (%s) and target contentType (%s) do not match", sourceContentType, targetContentType),
					Field:   field.Child("PVC").String(),
				})
				return causes
			}
			err = controller.ValidateCanCloneSourceAndTargetSpec(&sourcePVC.Spec, spec.PVC, targetContentType)
			if err != nil {
				causes = append(causes, metav1.StatusCause{
					Type:    metav1.CauseTypeFieldValueInvalid,
					Message: err.Error(),
					Field:   field.Child("PVC").String(),
				})
				return causes
			}
		}
	}

	if spec.PVC == nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing Data volume PVC"),
			Field:   field.Child("PVC").String(),
		})
		return causes
	}
	if pvcSize, ok := spec.PVC.Resources.Requests["storage"]; ok {
		if pvcSize.IsZero() || pvcSize.Value() < 0 {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("PVC size can't be equal or less than zero"),
				Field:   field.Child("PVC", "resources", "requests", "size").String(),
			})
			return causes
		}
	} else {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("PVC size is missing"),
			Field:   field.Child("PVC", "resources", "requests", "size").String(),
		})
		return causes
	}

	accessModes := spec.PVC.AccessModes
	if len(accessModes) == 0 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Required value: at least 1 access mode is required"),
			Field:   field.Child("PVC", "accessModes").String(),
		})
		return causes
	}
	if len(accessModes) > 1 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("PVC multiple accessModes"),
			Field:   field.Child("PVC", "accessModes").String(),
		})
		return causes
	}
	// We know we have one access mode
	if accessModes[0] != v1.ReadWriteOnce && accessModes[0] != v1.ReadOnlyMany && accessModes[0] != v1.ReadWriteMany {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Unsupported value: \"%s\": supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\"", string(accessModes[0])),
			Field:   field.Child("PVC", "accessModes").String(),
		})
		return causes
	}
	return causes
}

func (wh *dataVolumeValidatingWebhook) Admit(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	if err := validateDataVolumeResource(ar); err != nil {
		return toAdmissionResponseError(err)
	}

	raw := ar.Request.Object.Raw
	dv := cdiv1.DataVolume{}

	err := json.Unmarshal(raw, &dv)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if ar.Request.Operation == v1beta1.Update {
		oldDV := cdiv1.DataVolume{}
		err = json.Unmarshal(ar.Request.OldObject.Raw, &oldDV)
		if err != nil {
			return toAdmissionResponseError(err)
		}

		// Always admit checkpoint updates for multi-stage migrations.
		multiStageAdmitted := false
		isMultiStage := dv.Spec.Source.VDDK != nil && len(dv.Spec.Checkpoints) > 0
		if isMultiStage {
			oldSpec := oldDV.Spec.DeepCopy()
			oldSpec.FinalCheckpoint = false
			oldSpec.Checkpoints = nil

			newSpec := dv.Spec.DeepCopy()
			newSpec.FinalCheckpoint = false
			newSpec.Checkpoints = nil

			multiStageAdmitted = apiequality.Semantic.DeepEqual(newSpec, oldSpec)
		}

		if !multiStageAdmitted && !apiequality.Semantic.DeepEqual(dv.Spec, oldDV.Spec) {
			klog.Errorf("Cannot update spec for DataVolume %s/%s", dv.GetNamespace(), dv.GetName())
			var causes []metav1.StatusCause
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueDuplicate,
				Message: fmt.Sprintf("Cannot update DataVolume Spec"),
				Field:   k8sfield.NewPath("DataVolume").Child("Spec").String(),
			})
			return toRejectedAdmissionResponse(causes)
		}
	}

	causes := validateDataVolumeName(dv.Name)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission")
		return toRejectedAdmissionResponse(causes)
	}

	if ar.Request.Operation == v1beta1.Create {
		pvc, err := wh.client.CoreV1().PersistentVolumeClaims(dv.GetNamespace()).Get(context.TODO(), dv.GetName(), metav1.GetOptions{})
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				return toAdmissionResponseError(err)
			}
		} else {
			dvName, ok := pvc.Annotations[controller.AnnPopulatedFor]
			if !ok || dvName != dv.GetName() {
				pvcOwner := metav1.GetControllerOf(pvc)
				// We should reject the DV if a PVC with the same name exists, and that PVC has no ownerRef, or that
				// PVC has an ownerRef that is not a DataVolume. Because that means that PVC is not managed by the
				// datavolume controller, and we can't use it.
				if (pvcOwner == nil) || (pvcOwner.Kind != "DataVolume") {
					klog.Errorf("destination PVC %s/%s already exists", pvc.GetNamespace(), pvc.GetName())
					var causes []metav1.StatusCause
					causes = append(causes, metav1.StatusCause{
						Type:    metav1.CauseTypeFieldValueDuplicate,
						Message: fmt.Sprintf("Destination PVC %s/%s already exists", pvc.GetNamespace(), pvc.GetName()),
						Field:   k8sfield.NewPath("DataVolume").Child("Name").String(),
					})
					return toRejectedAdmissionResponse(causes)
				}
			}

			klog.Infof("Using initialized PVC %s for DataVolume %s", pvc.GetName(), dv.GetName())
		}
	}

	causes = wh.validateDataVolumeSpec(ar.Request, k8sfield.NewPath("spec"), &dv.Spec)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission")
		return toRejectedAdmissionResponse(causes)
	}

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}
