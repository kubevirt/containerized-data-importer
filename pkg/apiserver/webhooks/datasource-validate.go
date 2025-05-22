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
 * Copyright the CDI Authros.
 *
 */

package webhooks

import (
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	dataSourceResource = "datasources"
)

type dataSourceValidatingWebhook struct{}

func isPointerDataSource(ar admissionv1.AdmissionReview) bool {
	return ar.Request.Resource.Resource == dataSourceResource
}

func (wh *dataSourceValidatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var causes []metav1.StatusCause
	var err error

	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || !isPointerDataSource(ar) {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	raw := ar.Request.Object.Raw
	causes, err = wh.validateDataSourcePointer(ar, raw)

	if err != nil {
		return toAdmissionResponseError(err)
	}
	if causes != nil {
		return toRejectedAdmissionResponse(causes)
	}

	return allowedAdmissionResponse()
}

func (wh *dataSourceValidatingWebhook) validateDataSourcePointer(ar admissionv1.AdmissionReview, raw []byte) ([]metav1.StatusCause, error) {
	dataSource := cdiv1.DataSource{}
	err := json.Unmarshal(raw, &dataSource)
	if err != nil {
		return nil, err
	}

	// Reject self-pointer
	if dataSource.Spec.Source.DataSource != nil && ar.Request.Operation != admissionv1.Delete {
		if dataSource.Spec.Source.DataSource.Name == dataSource.Name && dataSource.Spec.Source.DataSource.Namespace == dataSource.Namespace {
			klog.Errorf("DataSource cannot point to itself")
			return []metav1.StatusCause{{
				Type:    metav1.CauseTypeFieldValueNotSupported,
				Message: "DataSource cannot point to itself",
				Field:   k8sfield.NewPath("DataSource").Child("Spec").Child("Source").Child("DataSource").String(),
			}}, nil
		}
	}

	return nil, nil
}
