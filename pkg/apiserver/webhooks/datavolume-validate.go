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
	neturl "net/url"
	"reflect"

	snapclient "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

type dataVolumeValidatingWebhook struct {
	k8sClient  kubernetes.Interface
	cdiClient  cdiclient.Interface
	snapClient snapclient.Interface
}

func validateSourceURL(sourceURL string) string {
	if sourceURL == "" {
		return "source URL is empty"
	}
	url, err := neturl.ParseRequestURI(sourceURL)
	if err != nil {
		return fmt.Sprintf("Invalid source URL: %s", sourceURL)
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Sprintf("Invalid source URL scheme: %s", sourceURL)
	}
	return ""
}

func validateNameLength(name string, maxLen int) []metav1.StatusCause {
	var causes []metav1.StatusCause
	if len(name) > maxLen {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Name cannot be longer than %d characters", maxLen),
			Field:   "",
		})
	}
	return causes
}

func (wh *dataVolumeValidatingWebhook) validateDataVolumeSpec(request *admissionv1.AdmissionRequest, field *k8sfield.Path, spec *cdiv1.DataVolumeSpec, namespace *string) []metav1.StatusCause {
	var causes []metav1.StatusCause
	var sourceType string
	var url string
	var dataSourceRef *v1.TypedLocalObjectReference
	var dataSource *v1.TypedLocalObjectReference

	if spec.PVC == nil && spec.Storage == nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing Data volume PVC"),
			Field:   field.Child("PVC").String(),
		})
		return causes
	}
	if spec.PVC != nil && spec.Storage != nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Duplicate storage definition, both target storage and target pvc defined"),
			Field:   field.Child("PVC", "Storage").String(),
		})
		return causes
	}

	cause, valid := validateStorageSize(spec, field)
	if !valid {
		causes = append(causes, *cause)
		return causes
	}

	if spec.PVC != nil {
		dataSourceRef = spec.PVC.DataSourceRef
		dataSource = spec.PVC.DataSource
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
	} else if spec.Storage != nil {
		dataSourceRef = spec.Storage.DataSourceRef
		dataSource = spec.Storage.DataSource
		// here in storage spec we allow empty access mode and AccessModes with more than one entry
		accessModes := spec.Storage.AccessModes
		for _, mode := range accessModes {
			if mode != v1.ReadWriteOnce && mode != v1.ReadOnlyMany && mode != v1.ReadWriteMany {
				causes = append(causes, metav1.StatusCause{
					Type:    metav1.CauseTypeFieldValueInvalid,
					Message: fmt.Sprintf("Unsupported value: \"%s\": supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\"", string(accessModes[0])),
					Field:   field.Child("PVC", "accessModes").String(),
				})
				return causes
			}
		}
	}

	// The PVC is externally populated when using dataSource and/or dataSourceRef
	if externalPopulation := dataSourceRef != nil || dataSource != nil; externalPopulation {
		causes = append(causes, validateExternalPopulation(spec, field, dataSource, dataSourceRef)...)
		return causes
	}

	if (spec.Source == nil && spec.SourceRef == nil) || (spec.Source != nil && spec.SourceRef != nil) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Data volume should have either Source or SourceRef, or be externally populated"),
			Field:   field.Child("source").String(),
		})
		return causes
	}
	if spec.SourceRef != nil {
		cause := wh.validateSourceRef(request, spec, field, namespace)
		if cause != nil {
			causes = append(causes, *cause)
		}
		return causes
	}

	numberOfSources := 0
	s := reflect.ValueOf(spec.Source).Elem()
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

	if spec.Source.Registry != nil {
		if spec.ContentType != "" && string(spec.ContentType) != string(cdiv1.DataVolumeKubeVirt) {
			sourceType = field.Child("contentType").String()
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("ContentType must be %s when Source is Registry", cdiv1.DataVolumeKubeVirt),
				Field:   sourceType,
			})
			return causes
		}

		causes := validateDataVolumeSourceRegistry(spec.Source.Registry, field)
		if len(causes) > 0 {
			return causes
		}

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
		if request.Operation == admissionv1.Create {
			cause := wh.validateDataVolumeSourcePVC(spec.Source.PVC, field.Child("source", "PVC"), spec)
			if cause != nil {
				causes = append(causes, *cause)
			}
		}
	}

	if spec.Source.Snapshot != nil {
		if spec.Source.Snapshot.Namespace == "" || spec.Source.Snapshot.Name == "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s source snapshot is not valid", field.Child("source", "Snapshot").String()),
				Field:   field.Child("source", "Snapshot").String(),
			})
			return causes
		}
		if request.Operation == admissionv1.Create {
			cause := wh.validateDataVolumeSourceSnapshot(spec.Source.Snapshot, field.Child("source", "Snapshot"), spec)
			if cause != nil {
				causes = append(causes, *cause)
			}
		}
	}

	return causes
}

func validateDataVolumeSourceRegistry(sourceRegistry *cdiv1.DataVolumeSourceRegistry, field *k8sfield.Path) []metav1.StatusCause {
	var causes []metav1.StatusCause
	sourceURL := sourceRegistry.URL
	sourceIS := sourceRegistry.ImageStream
	if (sourceURL == nil && sourceIS == nil) || (sourceURL != nil && sourceIS != nil) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Source registry should have either URL or ImageStream"),
			Field:   field.Child("source", "Registry").String(),
		})
		return causes
	}
	if sourceURL != nil {
		url, err := neturl.Parse(*sourceURL)
		if err != nil {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("Illegal registry source URL %s", *sourceURL),
				Field:   field.Child("source", "Registry", "URL").String(),
			})
			return causes
		}
		scheme := url.Scheme
		if scheme != cdiv1.RegistrySchemeDocker && scheme != cdiv1.RegistrySchemeOci {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("Illegal registry source URL scheme %s", url),
				Field:   field.Child("source", "Registry", "URL").String(),
			})
			return causes
		}
	}
	importMethod := sourceRegistry.PullMethod
	if importMethod != nil && *importMethod != cdiv1.RegistryPullPod && *importMethod != cdiv1.RegistryPullNode {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ImportMethod %s is neither %s, %s or \"\"", *importMethod, cdiv1.RegistryPullPod, cdiv1.RegistryPullNode),
			Field:   field.Child("source", "Registry", "importMethod").String(),
		})
		return causes
	}

	if sourceIS != nil && *sourceIS == "" {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Source registry ImageStream is not valid"),
			Field:   field.Child("source", "Registry", "importMethod").String(),
		})
		return causes
	}

	if sourceIS != nil && (importMethod == nil || *importMethod != cdiv1.RegistryPullNode) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Source registry ImageStream is supported only with node pull import method"),
			Field:   field.Child("source", "Registry", "importMethod").String(),
		})
		return causes
	}

	return causes
}

func (wh *dataVolumeValidatingWebhook) validateSourceRef(request *admissionv1.AdmissionRequest, spec *cdiv1.DataVolumeSpec, field *k8sfield.Path, namespace *string) *metav1.StatusCause {
	if spec.SourceRef.Kind == "" {
		return &metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing sourceRef kind"),
			Field:   field.Child("sourceRef", "Kind").String(),
		}
	}
	if spec.SourceRef.Kind != cdiv1.DataVolumeDataSource {
		return &metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Unsupported sourceRef kind %s, currently only %s is supported", spec.SourceRef.Kind, cdiv1.DataVolumeDataSource),
			Field:   field.Child("sourceRef", "Kind").String(),
		}
	}
	if spec.SourceRef.Name == "" {
		return &metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing sourceRef name"),
			Field:   field.Child("sourceRef", "Name").String(),
		}
	}
	if request.Operation != admissionv1.Create {
		return nil
	}
	ns := namespace
	if spec.SourceRef.Namespace != nil && *spec.SourceRef.Namespace != "" {
		ns = spec.SourceRef.Namespace
	}
	dataSource, err := wh.cdiClient.CdiV1beta1().DataSources(*ns).Get(context.TODO(), spec.SourceRef.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return &metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueNotFound,
				Message: fmt.Sprintf("SourceRef %s/%s/%s not found", spec.SourceRef.Kind, *ns, spec.SourceRef.Name),
				Field:   field.Child("sourceRef").String(),
			}
		}
		return &metav1.StatusCause{
			Message: err.Error(),
			Field:   field.Child("sourceRef").String(),
		}
	}
	dataSourceSnapshot := dataSource.Spec.Source.Snapshot
	if dataSourceSnapshot != nil {
		return &metav1.StatusCause{
			Message: "Snapshot sourceRef not allowed at this point",
			Field:   field.Child("sourceRef").String(),
		}
	}
	dataSourcePVC := dataSource.Spec.Source.PVC
	if dataSourcePVC == nil {
		return &metav1.StatusCause{
			Message: fmt.Sprintf("Empty PVC field in '%s'. DataSource may not be ready yet", dataSource.Name),
			Field:   field.Child("sourceRef").String(),
		}
	}
	return wh.validateDataVolumeSourcePVC(dataSourcePVC, field.Child("sourceRef"), spec)
}

func (wh *dataVolumeValidatingWebhook) validateDataVolumeSourcePVC(PVC *cdiv1.DataVolumeSourcePVC, field *k8sfield.Path, spec *cdiv1.DataVolumeSpec) *metav1.StatusCause {
	sourcePVC, err := wh.k8sClient.CoreV1().PersistentVolumeClaims(PVC.Namespace).Get(context.TODO(), PVC.Name, metav1.GetOptions{})
	if err != nil {
		// We allow the creation of a clone DV even if the source PVC doesn't exist.
		// The validation will be completed once the source PVC is created.
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return &metav1.StatusCause{
			Message: err.Error(),
			Field:   field.String(),
		}
	}

	if err := cc.ValidateClone(sourcePVC, spec); err != nil {
		return &metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: err.Error(),
			Field:   field.String(),
		}
	}

	return nil
}

// validateDataSource validates a DataSource/DataSourceRef in a DataVolume spec
func validateDataSource(dataSource *v1.TypedLocalObjectReference, field *k8sfield.Path) []metav1.StatusCause {
	var causes []metav1.StatusCause

	if len(dataSource.Name) == 0 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Required value: DataSource/DataSourceRef name"),
			Field:   field.Child("name", "").String(),
		})
	}
	if len(dataSource.Kind) == 0 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Required value: DataSource/DataSourceRef kind"),
			Field:   field.Child("kind").String(),
		})
	}
	apiGroup := ""
	if dataSource.APIGroup != nil {
		apiGroup = *dataSource.APIGroup
	}
	if len(apiGroup) == 0 && dataSource.Kind != "PersistentVolumeClaim" {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Required value: DataSource/DataSourceRef apiGroup when kind is not 'PersistentVolumeClaim'"),
			Field:   field.Child("apiGroup", "").String(),
		})
	}

	return causes
}

func (wh *dataVolumeValidatingWebhook) validateDataVolumeSourceSnapshot(snapshot *cdiv1.DataVolumeSourceSnapshot, field *k8sfield.Path, spec *cdiv1.DataVolumeSpec) *metav1.StatusCause {
	sourceSnapshot, err := wh.snapClient.SnapshotV1().VolumeSnapshots(snapshot.Namespace).Get(context.TODO(), snapshot.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return &metav1.StatusCause{
			Message: err.Error(),
			Field:   field.String(),
		}
	}

	if err := cc.ValidateSnapshotClone(sourceSnapshot, spec); err != nil {
		return &metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: err.Error(),
			Field:   field.String(),
		}
	}

	return nil
}

func validateStorageSize(spec *cdiv1.DataVolumeSpec, field *k8sfield.Path) (*metav1.StatusCause, bool) {
	var name string
	var resources v1.ResourceRequirements

	if spec.PVC != nil {
		resources = spec.PVC.Resources
		name = "PVC"
	} else if spec.Storage != nil {
		resources = spec.Storage.Resources
		name = "Storage"
	}

	// The storage size of a DataVolume can only be empty when two conditios are met:
	//	1. The 'Storage' spec API is used, which allows for additional logic in CDI.
	//	2. The 'PVC'/'Snapshot' source or SourceRef is used, so the original size can be extracted from the source.
	isClone := spec.SourceRef != nil || (spec.Source != nil && spec.Source.PVC != nil) || (spec.Source != nil && spec.Source.Snapshot != nil)
	if pvcSize, ok := resources.Requests["storage"]; ok {
		if pvcSize.IsZero() || pvcSize.Value() < 0 {
			cause := metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s size can't be equal or less than zero", name),
				Field:   field.Child(name, "resources", "requests", "size").String(),
			}
			return &cause, false
		}
	} else if spec.Storage == nil || !isClone {
		cause := metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("%s size is missing", name),
			Field:   field.Child(name, "resources", "requests", "size").String(),
		}
		return &cause, false
	}

	return nil, true
}

// validateExternalPopulation validates a DataVolume meant to be externally populated
func validateExternalPopulation(spec *cdiv1.DataVolumeSpec, field *k8sfield.Path, dataSource, dataSourceRef *v1.TypedLocalObjectReference) []metav1.StatusCause {
	var causes []metav1.StatusCause

	if spec.Source != nil || spec.SourceRef != nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("External population is incompatible with Source and SourceRef"),
			Field:   field.Child("source").String(),
		})
	}

	if dataSource != nil && dataSourceRef != nil {
		if !apiequality.Semantic.DeepEqual(dataSource, dataSourceRef) {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("DataSourceRef and DataSource must match"),
				Field:   "",
			})
		}
	}
	if dataSource != nil {
		causes = append(causes, validateDataSource(dataSource, field.Child("dataSource"))...)
	}
	if dataSourceRef != nil {
		causes = append(causes, validateDataSource(dataSourceRef, field.Child("dataSourceRef"))...)
	}

	return causes
}

func (wh *dataVolumeValidatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if err := validateDataVolumeResource(ar); err != nil {
		return toAdmissionResponseError(err)
	}

	raw := ar.Request.Object.Raw
	dv := cdiv1.DataVolume{}

	err := json.Unmarshal(raw, &dv)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if ar.Request.Operation == admissionv1.Update {
		oldDV := cdiv1.DataVolume{}
		err = json.Unmarshal(ar.Request.OldObject.Raw, &oldDV)
		if err != nil {
			return toAdmissionResponseError(err)
		}

		// Always admit checkpoint updates for multi-stage migrations.
		multiStageAdmitted := false
		isMultiStage := dv.Spec.Source != nil && len(dv.Spec.Checkpoints) > 0 &&
			(dv.Spec.Source.VDDK != nil || dv.Spec.Source.Imageio != nil)
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

	causes := validateNameLength(dv.Name, kvalidation.DNS1123SubdomainMaxLength)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission")
		return toRejectedAdmissionResponse(causes)
	}

	if ar.Request.Operation == admissionv1.Create {
		pvc, err := wh.k8sClient.CoreV1().PersistentVolumeClaims(dv.GetNamespace()).Get(context.TODO(), dv.GetName(), metav1.GetOptions{})
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				return toAdmissionResponseError(err)
			}
		} else {
			dvName, ok := pvc.Annotations[cc.AnnPopulatedFor]
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

	causes = wh.validateDataVolumeSpec(ar.Request, k8sfield.NewPath("spec"), &dv.Spec, &dv.Namespace)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission %s", causes)
		return toRejectedAdmissionResponse(causes)
	}

	reviewResponse := admissionv1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}
