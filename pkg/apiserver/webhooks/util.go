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
	"fmt"
	neturl "net/url"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	field "k8s.io/apimachinery/pkg/util/validation/field"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func validateNumberOfSources(source interface{}, sourceKind string, field *field.Path) []metav1.StatusCause {
	numberOfSources := 0
	s := reflect.ValueOf(source).Elem()
	for i := range s.NumField() {
		if !reflect.ValueOf(s.Field(i).Interface()).IsNil() {
			numberOfSources++
		}
	}
	if numberOfSources == 0 {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing %s source", sourceKind),
			Field:   field.Child("source").String(),
		}}
	}
	if numberOfSources > 1 {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Multiple %s sources", sourceKind),
			Field:   field.Child("source").String(),
		}}
	}
	return nil
}

func validateContentType(contentType cdiv1.DataVolumeContentType, field *field.Path) []metav1.StatusCause {
	// Make sure contentType is either empty (kubevirt), or kubevirt or archive
	if contentType != "" && string(contentType) != string(cdiv1.DataVolumeKubeVirt) && string(contentType) != string(cdiv1.DataVolumeArchive) {
		sourceType := field.Child("contentType").String()
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType not one of: %s, %s", cdiv1.DataVolumeKubeVirt, cdiv1.DataVolumeArchive),
			Field:   sourceType,
		}}
	}
	return nil
}

func validateBlankSource(contentType cdiv1.DataVolumeContentType, field *field.Path) []metav1.StatusCause {
	if string(contentType) == string(cdiv1.DataVolumeArchive) {
		sourceType := field.Child("contentType").String()
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: "SourceType cannot be blank and the contentType be archive",
			Field:   sourceType,
		}}
	}
	return nil
}

func validateRegistrySource(registry *cdiv1.DataVolumeSourceRegistry, contentType cdiv1.DataVolumeContentType, field *field.Path) []metav1.StatusCause {
	if contentType != "" && string(contentType) != string(cdiv1.DataVolumeKubeVirt) {
		sourceType := field.Child("contentType").String()
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType must be %s when Source is Registry", cdiv1.DataVolumeKubeVirt),
			Field:   sourceType,
		}}
	}
	causes := validateDataVolumeSourceRegistry(registry, field)
	if len(causes) > 0 {
		return causes
	}

	return nil
}

func validateDataVolumeSourceRegistry(sourceRegistry *cdiv1.DataVolumeSourceRegistry, field *field.Path) []metav1.StatusCause {
	var causes []metav1.StatusCause
	sourceURL := sourceRegistry.URL
	sourceIS := sourceRegistry.ImageStream
	if (sourceURL == nil && sourceIS == nil) || (sourceURL != nil && sourceIS != nil) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: "Source registry should have either URL or ImageStream",
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
			Message: "Source registry ImageStream is not valid",
			Field:   field.Child("source", "Registry", "importMethod").String(),
		})
		return causes
	}

	if sourceIS != nil && (importMethod == nil || *importMethod != cdiv1.RegistryPullNode) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: "Source registry ImageStream is supported only with node pull import method",
			Field:   field.Child("source", "Registry", "importMethod").String(),
		})
		return causes
	}

	return causes
}

// if source types are HTTP, Imageio, S3, GCS or VDDK, check if URL is valid

func validateHTTPSource(http *cdiv1.DataVolumeSourceHTTP, field *field.Path) []metav1.StatusCause {
	return checkSourceURL(http.URL, "HTTP", field)
}

func validateS3Source(s3 *cdiv1.DataVolumeSourceS3, field *field.Path) []metav1.StatusCause {
	return checkSourceURL(s3.URL, "S3", field)
}

func validateGCSSource(gcs *cdiv1.DataVolumeSourceGCS, field *field.Path) []metav1.StatusCause {
	return checkSourceURL(gcs.URL, "GCS", field)
}

func validateImageIOSource(imageio *cdiv1.DataVolumeSourceImageIO, field *field.Path) []metav1.StatusCause {
	if imageio.SecretRef == "" || imageio.CertConfigMap == "" || imageio.DiskID == "" {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("%s source Imageio is not valid", field.Child("source", "Imageio").String()),
			Field:   field.Child("source", "Imageio").String(),
		}}
	}
	return checkSourceURL(imageio.URL, "ImageIO", field)
}

func validateVDDKSource(vddk *cdiv1.DataVolumeSourceVDDK, field *field.Path) []metav1.StatusCause {
	if vddk.SecretRef == "" || vddk.UUID == "" || vddk.BackingFile == "" || vddk.Thumbprint == "" {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("%s source VDDK is not valid", field.Child("source", "VDDK").String()),
			Field:   field.Child("source", "VDDK").String(),
		}}
	}
	return checkSourceURL(vddk.URL, "VDDK", field)
}

func checkSourceURL(url, sourceType string, field *field.Path) []metav1.StatusCause {
	if errString := validateSourceURL(url); errString != "" {
		return []metav1.StatusCause{{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("%s %s", field.Child("source").String(), errString),
			Field:   field.Child("source", sourceType, "url").String(),
		}}
	}
	return nil
}

func validateSourceURL(sourceURL string) string {
	if sourceURL == "" {
		return "source URL is empty"
	}
	url, err := neturl.ParseRequestURI(sourceURL)
	if err != nil {
		return fmt.Sprintf("Invalid source URL: %s", sourceURL)
	}

	if url.Scheme != "http" && url.Scheme != "https" && url.Scheme != "gs" {
		return fmt.Sprintf("Invalid source URL scheme: %s", sourceURL)
	}
	return ""
}
