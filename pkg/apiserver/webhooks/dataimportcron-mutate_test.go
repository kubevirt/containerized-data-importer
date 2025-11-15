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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/appscode/jsonpatch"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("Mutating DataImportCron Webhook", func() {
	Context("with DataImportCron admission review", func() {
		It("should reject review without request", func() {
			ar := &admissionv1.AdmissionReview{}
			resp := mutateDataImportCrons(ar)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).Should(Equal("AdmissionReview.Request is nil"))
		})

		It("should allow non-Create operations without mutation", func() {
			cron := cdiv1.DataImportCron{}
			ar := createDataImportCronAdmissionReview(cron, admissionv1.Update, nil)

			resp := mutateDataImportCrons(ar)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).To(BeNil())
		})

		It("should set CreatedBy field on Create operation", func() {
			cron := cdiv1.DataImportCron{}
			userInfo := &authenticationv1.UserInfo{
				Username: "test-user",
				UID:      "test-uid",
				Groups:   []string{"test-group"},
			}
			ar := createDataImportCronAdmissionReview(cron, admissionv1.Create, userInfo)

			resp := mutateDataImportCrons(ar)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).ToNot(BeNil())
			var patchOps []jsonpatch.Operation
			err := json.Unmarshal(resp.Patch, &patchOps)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchOps).To(HaveLen(1))
			Expect(patchOps[0].Operation).To(Equal("add"))
			Expect(patchOps[0].Path).To(Equal("/spec/createdBy"))
			valueStr, ok := patchOps[0].Value.(string)
			Expect(ok).To(BeTrue())
			Expect(valueStr).To(ContainSubstring(userInfo.Username))
			Expect(valueStr).To(ContainSubstring(userInfo.UID))
		})
	})
})

func mutateDataImportCrons(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	wh := NewDataImportCronMutatingWebhook()
	return serveDataImportCron(ar, wh)
}

func serveDataImportCron(ar *admissionv1.AdmissionReview, handler http.Handler) *admissionv1.AdmissionResponse {
	reqBytes, _ := json.Marshal(ar)
	req, err := http.NewRequest(http.MethodPost, "/foobar", bytes.NewReader(reqBytes))
	Expect(err).ToNot(HaveOccurred())

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var response admissionv1.AdmissionReview
	err = json.NewDecoder(rr.Body).Decode(&response)
	Expect(err).ToNot(HaveOccurred())

	return response.Response
}

func createDataImportCronAdmissionReview(cron cdiv1.DataImportCron, operation admissionv1.Operation, userInfo *authenticationv1.UserInfo) *admissionv1.AdmissionReview {
	cronBytes, _ := json.Marshal(&cron)

	request := &admissionv1.AdmissionRequest{
		Operation: operation,
		Resource: metav1.GroupVersionResource{
			Group:    cdiv1.CDIGroupVersionKind.Group,
			Version:  cdiv1.CDIGroupVersionKind.Version,
			Resource: "dataimportcrons",
		},
		Object: runtime.RawExtension{
			Raw: cronBytes,
		},
	}

	if userInfo != nil {
		request.UserInfo = *userInfo
	}

	return &admissionv1.AdmissionReview{
		Request: request,
	}
}
