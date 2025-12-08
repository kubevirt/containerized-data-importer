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
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"github.com/appscode/jsonpatch"
	snapclient "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

// Admitter is the interface implemented by admission webhooks
type Admitter interface {
	Admit(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

type admissionHandler struct {
	a Admitter
}

// NewDataVolumeValidatingWebhook creates a new DataVolumeValidation webhook
func NewDataVolumeValidatingWebhook(k8sClient kubernetes.Interface, cdiClient cdiclient.Interface,
	snapClient snapclient.Interface, controllerRuntimeClient client.Client) http.Handler {
	return newAdmissionHandler(&dataVolumeValidatingWebhook{
		k8sClient:               k8sClient,
		cdiClient:               cdiClient,
		snapClient:              snapClient,
		controllerRuntimeClient: controllerRuntimeClient,
	})
}

// NewDataVolumeMutatingWebhook creates a new DataVolumeMutation webhook
func NewDataVolumeMutatingWebhook(k8sClient kubernetes.Interface, cdiClient cdiclient.Interface, key *rsa.PrivateKey) http.Handler {
	generator := newCloneTokenGenerator(key)
	return newAdmissionHandler(&dataVolumeMutatingWebhook{k8sClient: k8sClient, cdiClient: cdiClient, tokenGenerator: generator})
}

// NewPvcMutatingWebhook creates a new PvcMutation webhook
func NewPvcMutatingWebhook(cachedClient client.Client) http.Handler {
	return newAdmissionHandler(&pvcMutatingWebhook{cachedClient: cachedClient})
}

// NewCDIValidatingWebhook creates a new CDI validating webhook
func NewCDIValidatingWebhook(client cdiclient.Interface) http.Handler {
	return newAdmissionHandler(&cdiValidatingWebhook{client: client})
}

// NewObjectTransferValidatingWebhook creates a new ObjectTransfer validating webhook
func NewObjectTransferValidatingWebhook(k8sClient kubernetes.Interface, cdiClient cdiclient.Interface) http.Handler {
	return newAdmissionHandler(&objectTransferValidatingWebhook{k8sClient: k8sClient, cdiClient: cdiClient})
}

// NewDataImportCronValidatingWebhook creates a new DataVolumeValidation webhook
func NewDataImportCronValidatingWebhook(k8sClient kubernetes.Interface, cdiClient cdiclient.Interface) http.Handler {
	return newAdmissionHandler(&dataImportCronValidatingWebhook{dataVolumeValidatingWebhook{k8sClient: k8sClient, cdiClient: cdiClient}})
}

// NewPopulatorValidatingWebhook creates a new DataVolumeValidation webhook
func NewPopulatorValidatingWebhook(k8sClient kubernetes.Interface, cdiClient cdiclient.Interface) http.Handler {
	return newAdmissionHandler(&populatorValidatingWebhook{dataVolumeValidatingWebhook{k8sClient: k8sClient, cdiClient: cdiClient}})
}

func newCloneTokenGenerator(key *rsa.PrivateKey) token.Generator {
	return token.NewGenerator(common.CloneTokenIssuer, key, 5*time.Minute)
}

func newAdmissionHandler(a Admitter) http.Handler {
	return &admissionHandler{a: a}
}

func (h *admissionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	// The AdmissionReview that was sent to the webhook
	requestedAdmissionReview := admissionv1.AdmissionReview{}

	// The AdmissionReview that will be returned
	responseAdmissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionv1.SchemeGroupVersion.String(),
			Kind:       "AdmissionReview",
		},
	}

	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		klog.Error(err)
		responseAdmissionReview.Response = toAdmissionResponseError(err)
	} else {
		if requestedAdmissionReview.Request == nil {
			responseAdmissionReview.Response = toAdmissionResponseError(fmt.Errorf("AdmissionReview.Request is nil"))
		} else {
			// pass to Admitter
			responseAdmissionReview.Response = h.a.Admit(requestedAdmissionReview)
		}
	}

	// Return the same UID
	if requestedAdmissionReview.Request != nil {
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
	}

	// Match request's APIVersion for backwards compatibility with v1beta1
	if requestedAdmissionReview.APIVersion != "" {
		responseAdmissionReview.APIVersion = requestedAdmissionReview.APIVersion
	}

	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseAdmissionReview.Response))

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		klog.Error(err)
	}
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func toRejectedAdmissionResponse(causes []metav1.StatusCause) *admissionv1.AdmissionResponse {
	globalMessage := ""
	for _, cause := range causes {
		globalMessage = fmt.Sprintf("%s %s", globalMessage, cause.Message)
	}

	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: globalMessage,
			Code:    http.StatusUnprocessableEntity,
			Details: &metav1.StatusDetails{
				Causes: causes,
			},
		},
	}
}

func toAdmissionResponseError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		},
	}
}

func allowedAdmissionResponse() *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func validateDataVolumeResource(ar admissionv1.AdmissionReview) error {
	dvResource := metav1.GroupVersionResource{
		Group:    cdiv1.SchemeGroupVersion.Group,
		Version:  cdiv1.SchemeGroupVersion.Version,
		Resource: "datavolumes",
	}
	if ar.Request.Resource == dvResource {
		return nil
	}

	klog.Errorf("resource is %s but request is: %s", dvResource, ar.Request.Resource)
	return fmt.Errorf("expect resource to be '%s'", dvResource.Resource)
}

func validatePvcResource(ar admissionv1.AdmissionReview) error {
	pvcResource := metav1.GroupVersionResource{
		Group:    corev1.SchemeGroupVersion.Group,
		Version:  corev1.SchemeGroupVersion.Version,
		Resource: "persistentvolumeclaims",
	}
	if ar.Request.Resource == pvcResource {
		return nil
	}

	klog.Errorf("resource is %s but request is: %s", pvcResource, ar.Request.Resource)
	return fmt.Errorf("expect resource to be '%s'", pvcResource.Resource)
}

func toPatchResponse(original, current interface{}) *admissionv1.AdmissionResponse {
	if reflect.DeepEqual(original, current) {
		return allowedAdmissionResponse()
	}

	patchType := admissionv1.PatchTypeJSONPatch

	ob, err := json.Marshal(original)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	cb, err := json.Marshal(current)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	patches, err := jsonpatch.CreatePatch(ob, cb)
	if err != nil {
		toAdmissionResponseError(err)
	}

	pb, err := json.Marshal(patches)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	klog.V(1).Infof("sending patches\n%s", pb)

	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     pb,
		PatchType: &patchType,
	}
}
