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
 * Copyright 2025 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type dataImportCronMutatingWebhook struct{}

func (wh *dataImportCronMutatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if ar.Request.Operation != admissionv1.Create {
		return allowedAdmissionResponse()
	}

	cron := &cdiv1.DataImportCron{}
	if err := json.Unmarshal(ar.Request.Object.Raw, cron); err != nil {
		return toAdmissionResponseError(err)
	}

	modifiedCron := cron.DeepCopy()
	userInfoStr, err := getUserInfoString(&ar.Request.UserInfo)
	if err != nil {
		return toAdmissionResponseError(err)
	}
	modifiedCron.Spec.CreatedBy = userInfoStr

	return toPatchResponse(cron, modifiedCron)
}

func getUserInfoString(ui *authenticationv1.UserInfo) (*string, error) {
	bs, err := json.Marshal(ui)
	if err != nil {
		return nil, err
	}
	userInfoStr := string(bs)
	return &userInfoStr, nil
}
