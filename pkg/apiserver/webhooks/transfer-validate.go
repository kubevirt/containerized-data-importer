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

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

type objectTransferValidatingWebhook struct {
	k8sClient kubernetes.Interface
	cdiClient cdiclient.Interface
}

func (wh *objectTransferValidatingWebhook) Admit(ar admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if ar.Request.Resource.Group != cdiv1.CDIGroupVersionKind.Group || ar.Request.Resource.Resource != "objecttransfers" {
		klog.V(3).Infof("Got unexpected resource type %s", ar.Request.Resource.Resource)
		return toAdmissionResponseError(fmt.Errorf("unexpected resource: %s", ar.Request.Resource.Resource))
	}

	switch ar.Request.Operation {
	case admissionv1beta1.Create:
	case admissionv1beta1.Update:
	default:
		klog.V(3).Infof("Got unexpected operation type %s", ar.Request.Operation)
		return allowedAdmissionResponse()
	}

	obj := &cdiv1.ObjectTransfer{}
	if err := json.Unmarshal(ar.Request.Object.Raw, obj); err != nil {
		return toAdmissionResponseError(err)
	}

	if ar.Request.Operation == admissionv1beta1.Update {
		oldObj := &cdiv1.ObjectTransfer{}
		if err := json.Unmarshal(ar.Request.OldObject.Raw, oldObj); err != nil {
			return toAdmissionResponseError(err)
		}

		if !apiequality.Semantic.DeepEqual(obj.Spec, oldObj.Spec) {
			return toAdmissionResponseError(fmt.Errorf("ObjectTransfer spec is immutable"))
		}

		return allowedAdmissionResponse()
	}

	if obj.Spec.Target.Namespace == nil && obj.Spec.Target.Name == nil {
		return toAdmissionResponseError(fmt.Errorf("Target namespace and/or target name must be supplied"))
	}

	var err error
	ns := obj.Spec.Source.Namespace
	name := obj.Spec.Source.Name

	if obj.Spec.Target.Namespace != nil {
		ns = *obj.Spec.Target.Namespace
	}

	if obj.Spec.Target.Name != nil {
		name = *obj.Spec.Target.Name
	}

	switch obj.Spec.Source.Kind {
	case "DataVolume":
		_, err = wh.cdiClient.CdiV1beta1().DataVolumes(ns).Get(context.TODO(), name, metav1.GetOptions{})
	case "PersistentVolumeClaim":
		_, err = wh.k8sClient.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), name, metav1.GetOptions{})
	default:
		return toAdmissionResponseError(fmt.Errorf("Unsupported kind %q", obj.Spec.Source.Kind))
	}

	if err == nil {
		return toAdmissionResponseError(fmt.Errorf("ObjectTransfer target \"%s/%s\" already exists", ns, name))
	}

	if !errors.IsNotFound(err) {
		return toAdmissionResponseError(err)
	}

	return allowedAdmissionResponse()
}
