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

	admissionv1 "k8s.io/api/admission/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

type dataVolumeMutatingWebhook struct {
	k8sClient      kubernetes.Interface
	cdiClient      cdiclient.Interface
	tokenGenerator token.Generator
}

type authProxy struct {
	k8sClient kubernetes.Interface
	cdiClient cdiclient.Interface
}

func (p *authProxy) CreateSar(sar *authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return p.k8sClient.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
}

func (p *authProxy) GetNamespace(name string) (*corev1.Namespace, error) {
	return p.k8sClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
}

func (p *authProxy) GetDataSource(namespace, name string) (*cdiv1.DataSource, error) {
	return p.cdiClient.CdiV1beta1().DataSources(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (wh *dataVolumeMutatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	dataVolume := &cdiv1.DataVolume{}

	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if err := validateDataVolumeResource(ar); err != nil {
		return toAdmissionResponseError(err)
	}

	if err := json.Unmarshal(ar.Request.Object.Raw, &dataVolume); err != nil {
		return toAdmissionResponseError(err)
	}

	if dataVolume.GetDeletionTimestamp() != nil {
		// No point continuing if DV is flagged for deletion
		return allowedAdmissionResponse()
	}

	modifiedDataVolume := dataVolume.DeepCopy()

	if ar.Request.Operation == admissionv1.Create {
		if err := wh.mutateCreatedDataVolume(modifiedDataVolume); err != nil {
			return toAdmissionResponseError(err)
		}
	}

	targetNamespace, targetName := dataVolume.Namespace, dataVolume.Name
	if targetNamespace == "" {
		targetNamespace = ar.Request.Namespace
	}

	if targetName == "" {
		targetName = ar.Request.Name
	}

	proxy := &authProxy{k8sClient: wh.k8sClient, cdiClient: wh.cdiClient}
	response, err := modifiedDataVolume.AuthorizeUser(ar.Request.Namespace, ar.Request.Name, proxy, ar.Request.UserInfo)
	if err != nil {
		if err == cdiv1.ErrNoTokenOkay {
			return toPatchResponse(dataVolume, modifiedDataVolume)
		}
		return toAdmissionResponseError(err)
	}

	if !response.Allowed {
		causes := []metav1.StatusCause{
			{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: response.Reason,
				Field:   k8sfield.NewPath("spec", "source", "PVC", "namespace").String(),
			},
		}
		return toRejectedAdmissionResponse(causes)
	}

	// only add token at create time
	if ar.Request.Operation != admissionv1.Create {
		return toPatchResponse(dataVolume, modifiedDataVolume)
	}

	sourceName, sourceNamespace := response.Handler.SourceName, response.Handler.SourceNamespace
	if sourceNamespace == "" {
		sourceNamespace = targetNamespace
	}

	tokenData := &token.Payload{
		Operation: token.OperationClone,
		Name:      sourceName,
		Namespace: sourceNamespace,
		Resource:  response.Handler.TokenResource,
		Params: map[string]string{
			"targetNamespace": targetNamespace,
			"targetName":      targetName,
		},
	}

	token, err := wh.tokenGenerator.Generate(tokenData)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if modifiedDataVolume.Annotations == nil {
		modifiedDataVolume.Annotations = make(map[string]string)
	}
	modifiedDataVolume.Annotations[cc.AnnCloneToken] = token

	klog.V(3).Infof("Sending patch response...")

	return toPatchResponse(dataVolume, modifiedDataVolume)
}

func (wh *dataVolumeMutatingWebhook) mutateCreatedDataVolume(dv *cdiv1.DataVolume) error {
	if dv.Annotations[cc.AnnDeleteAfterCompletion] != "true" {
		// If it's an "apply" and there is a PVC, annotate the DV to not be GCed
		if _, isApply := dv.Annotations[corev1.LastAppliedConfigAnnotation]; isApply {
			_, err := wh.k8sClient.CoreV1().PersistentVolumeClaims(dv.Namespace).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			if err == nil {
				cc.AddAnnotation(dv, cc.AnnDeleteAfterCompletion, "false")
			} else if !k8serrors.IsNotFound(err) {
				return err
			}
		}
	}

	// Consider annotating for GC only if not annotated to disable GC
	if dv.Annotations[cc.AnnDeleteAfterCompletion] != "false" {
		config, err := wh.cdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cc.GetDataVolumeTTLSeconds(config) >= 0 {
			cc.AddAnnotation(dv, cc.AnnDeleteAfterCompletion, "true")
		}
	}

	return nil
}
