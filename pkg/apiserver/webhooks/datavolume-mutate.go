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

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/clone"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

type dataVolumeMutatingWebhook struct {
	client         kubernetes.Interface
	tokenGenerator token.Generator
	proxy          clone.SubjectAccessReviewsProxy
}

type sarProxy struct {
	client kubernetes.Interface
}

var (
	tokenResource = metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}
)

func (p *sarProxy) Create(sar *authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return p.client.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
}

func (wh *dataVolumeMutatingWebhook) Admit(ar admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	var dataVolume, oldDataVolume cdiv1.DataVolume

	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if err := validateDataVolumeResource(ar); err != nil {
		return toAdmissionResponseError(err)
	}

	if err := json.Unmarshal(ar.Request.Object.Raw, &dataVolume); err != nil {
		return toAdmissionResponseError(err)
	}

	pvcSource := dataVolume.Spec.Source.PVC
	targetNamespace, targetName := dataVolume.Namespace, dataVolume.Name
	if targetNamespace == "" {
		targetNamespace = ar.Request.Namespace
	}

	if targetName == "" {
		targetName = ar.Request.Name
	}

	if pvcSource == nil {
		klog.V(3).Infof("DataVolume %s/%s not cloning", targetNamespace, targetName)
		return allowedAdmissionResponse()
	}

	sourceNamespace, sourceName := pvcSource.Namespace, pvcSource.Name
	if sourceNamespace == "" {
		sourceNamespace = targetNamespace
	}

	if ar.Request.Operation == admissionv1beta1.Update {
		if err := json.Unmarshal(ar.Request.OldObject.Raw, &oldDataVolume); err != nil {
			return toAdmissionResponseError(err)
		}

		_, ok := oldDataVolume.Annotations[controller.AnnCloneToken]
		if ok {
			klog.V(3).Infof("DataVolume %s/%s already has clone token", targetNamespace, targetName)
			return allowedAdmissionResponse()
		}
	}

	ok, reason, err := clone.CanUserClonePVC(wh.proxy, sourceNamespace, sourceName, targetNamespace, ar.Request.UserInfo)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if !ok {
		causes := []metav1.StatusCause{
			{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: reason,
				Field:   k8sfield.NewPath("spec", "source", "PVC", "namespace").String(),
			},
		}
		return toRejectedAdmissionResponse(causes)
	}

	tokenData := &token.Payload{
		Operation: token.OperationClone,
		Name:      sourceName,
		Namespace: sourceNamespace,
		Resource:  tokenResource,
		Params: map[string]string{
			"targetNamespace": targetNamespace,
			"targetName":      targetName,
		},
	}

	token, err := wh.tokenGenerator.Generate(tokenData)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	modifiedDataVolume := dataVolume.DeepCopy()
	if modifiedDataVolume.Annotations == nil {
		modifiedDataVolume.Annotations = make(map[string]string)
	}

	modifiedDataVolume.Annotations[controller.AnnCloneToken] = token

	klog.V(3).Infof("Sending patch response...")

	return toPatchResponse(dataVolume, modifiedDataVolume)
}
