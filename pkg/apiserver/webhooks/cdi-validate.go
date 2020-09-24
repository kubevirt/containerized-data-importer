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
	"context"
	"encoding/json"
	"fmt"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const uninstallErrorMsg = "Rejecting the uninstall request, since there are still DataVolumes present. Either delete all DataVolumes or change the uninstall strategy before uninstalling CDI."

type cdiValidatingWebhook struct {
	client cdiclient.Interface
}

func (wh *cdiValidatingWebhook) Admit(ar admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || ar.Request.Resource.Resource != "cdis" {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	if ar.Request.Operation != admissionv1beta1.Delete {
		klog.V(3).Infof("Got unexpected operation type %s", ar.Request.Operation)
		return allowedAdmissionResponse()
	}

	cdi, err := wh.getResource(ar)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	switch cdi.Status.Phase {
	case sdkapi.PhaseEmpty, sdkapi.PhaseError:
		return allowedAdmissionResponse()
	}

	if cdi.Spec.UninstallStrategy != nil && *cdi.Spec.UninstallStrategy == cdiv1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist {
		dvs, err := wh.client.CdiV1beta1().DataVolumes(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{Limit: 2})
		if err != nil {
			return toAdmissionResponseError(err)
		}

		if len(dvs.Items) > 0 {
			return toAdmissionResponseError(fmt.Errorf(uninstallErrorMsg))
		}
	}

	return allowedAdmissionResponse()
}

func (wh *cdiValidatingWebhook) getResource(ar admissionv1beta1.AdmissionReview) (*cdiv1.CDI, error) {
	var cdi *cdiv1.CDI

	if len(ar.Request.OldObject.Raw) > 0 {
		cdi = &cdiv1.CDI{}
		err := json.Unmarshal(ar.Request.OldObject.Raw, cdi)
		if err != nil {
			return nil, err
		}
	} else if len(ar.Request.Name) > 0 {
		var err error
		cdi, err = wh.client.CdiV1beta1().CDIs().Get(context.TODO(), ar.Request.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("cannot derive deleted resource")
	}

	return cdi, nil
}
