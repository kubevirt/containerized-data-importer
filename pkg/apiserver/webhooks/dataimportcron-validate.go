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

	admissionv1 "k8s.io/api/admission/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

type dataImportCronValidatingWebhook struct {
	k8sClient kubernetes.Interface
	cdiClient cdiclient.Interface
}

//FIXME: complete validation
func (wh *dataImportCronValidatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	klog.V(3).Infof("FIXME Admit resource type %s", ar.Request.Resource.Resource)
	/* FIXME
	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || ar.Request.Resource.Resource != "dataimportcrons" {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}
	*/
	raw := ar.Request.Object.Raw
	dic := cdiv1.DataImportCron{}
	err := json.Unmarshal(raw, &dic)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if ar.Request.Operation == admissionv1.Update {
		oldDic := cdiv1.DataImportCron{}
		err = json.Unmarshal(ar.Request.OldObject.Raw, &oldDic)
		if err != nil {
			return toAdmissionResponseError(err)
		}
		if !apiequality.Semantic.DeepEqual(dic.Spec, oldDic.Spec) {
			klog.Errorf("Cannot update spec for DataImportCron %s/%s", dic.GetNamespace(), dic.GetName())
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

	return allowedAdmissionResponse()
}
