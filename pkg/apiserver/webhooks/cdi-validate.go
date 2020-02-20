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
 * Copyright 2020 Red Hat, Inc.
 *
 */

package webhooks

import (
	"fmt"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const uninstallErrorMsg = "Rejecting the uninstall request, since there are still DataVolumes present. Either delete all DataVolumes or change the uninstall strategy before uninstalling CDI."

type cdiValidatingWebhook struct {
	client cdiclient.Interface
}

func (wh *cdiValidatingWebhook) Admit(ar admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	var cdi cdiv1alpha1.CDI
	deserializer := codecs.UniversalDeserializer()

	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if ar.Request.Resource.Resource != "cdis" {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	if ar.Request.Operation != admissionv1beta1.Delete {
		klog.V(3).Infof("Got unexpected operation type %s", ar.Request.Operation)
		return allowedAdmissionResponse()
	}

	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, &cdi); err != nil {
		return toAdmissionResponseError(err)
	}

	switch cdi.Status.Phase {
	case cdiv1alpha1.CDIPhaseEmpty, cdiv1alpha1.CDIPhaseError:
		return allowedAdmissionResponse()
	}

	if cdi.Spec.UninstallStrategy != nil && *cdi.Spec.UninstallStrategy == cdiv1alpha1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist {
		dvs, err := wh.client.CdiV1alpha1().DataVolumes(metav1.NamespaceAll).List(metav1.ListOptions{Limit: 2})
		if err != nil {
			return toAdmissionResponseError(err)
		}

		if len(dvs.Items) > 0 {
			return toAdmissionResponseError(fmt.Errorf(uninstallErrorMsg))
		}
	}

	return allowedAdmissionResponse()
}
