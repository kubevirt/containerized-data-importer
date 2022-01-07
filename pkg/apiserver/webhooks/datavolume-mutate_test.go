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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/appscode/jsonpatch"
	admissionv1 "k8s.io/api/admission/v1"
	authorization "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	cdiclientfake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"

	cdicorev1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
)

var _ = Describe("Mutating DataVolume Webhook", func() {
	Context("with DataVolume admission review", func() {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)

		It("should reject review without request", func() {
			ar := &admissionv1.AdmissionReview{}

			resp := mutateDVs(key, ar, true)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).Should(Equal("AdmissionReview.Request is nil"))
		})

		It("should allow a DataVolume with HTTP source", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := mutateDVs(key, ar, true)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).To(BeNil())
		})

		It("should reject a DataVolume with sourceRef to non-existing DataSource", func() {
			dataVolume := newDataSourceDataVolume("testDV", nil, "test")
			Expect(dataVolume.Annotations).To(BeNil())
			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}
			resp := mutateDVs(key, ar, true)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Patch).To(BeNil())
		})

		It("should allow a DataVolume with sourceRef to existing DataSource", func() {
			dataVolume := newDataSourceDataVolume("testDV", nil, "test")
			Expect(dataVolume.Annotations).To(BeNil())
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			dataSource := &cdicorev1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.SourceRef.Name,
					Namespace: "default",
				},
				Spec: cdicorev1.DataSourceSpec{
					Source: cdicorev1.DataSourceSource{
						PVC: &cdicorev1.DataVolumeSourcePVC{
							Name: "testPVC",
						},
					},
				},
			}

			resp := mutateDVsEx(key, ar, true, []runtime.Object{dataSource})
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).ToNot(BeNil())

			var patchObjs []jsonpatch.Operation
			err := json.Unmarshal(resp.Patch, &patchObjs)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchObjs).Should(HaveLen(1))
			Expect(patchObjs[0].Operation).Should(Equal("add"))
			Expect(patchObjs[0].Path).Should(Equal("/metadata/annotations"))
		})

		It("should allow a DataVolume update with token unchanged", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			Expect(dataVolume.Annotations).To(BeNil())
			dataVolume.Annotations = make(map[string]string)
			dataVolume.Annotations[controller.AnnCloneToken] = "baz"
			dvBytes, _ := json.Marshal(&dataVolume)

			dataVolume.Annotations["foo"] = "bar"
			dvBytesUpdated, _ := json.Marshal(&dataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytesUpdated,
					},
					OldObject: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := mutateDVs(key, ar, true)
			Expect(resp.Allowed).To(Equal(true))
			Expect(resp.Patch).To(BeNil())
		})

		It("should reject a clone DataVolume", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := mutateDVs(key, ar, false)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Patch).To(BeNil())
		})

		DescribeTable("should", func(srcNamespace string) {
			dataVolume := newPVCDataVolume("testDV", srcNamespace, "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1.SchemeGroupVersion.Group,
						Version:  cdicorev1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := mutateDVs(key, ar, true)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Patch).ToNot(BeNil())

			var patchObjs []jsonpatch.Operation
			err := json.Unmarshal(resp.Patch, &patchObjs)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchObjs).Should(HaveLen(1))
			Expect(patchObjs[0].Operation).Should(Equal("add"))
			Expect(patchObjs[0].Path).Should(Equal("/metadata/annotations"))

		},
			Entry("succeed with explicit namespace", "testNamespace"),
			Entry("succeed with same (default) namespace", "default"),
			Entry("succeed with empty namespace", ""),
		)
	})
})

func mutateDVs(key *rsa.PrivateKey, ar *admissionv1.AdmissionReview, isAuthorized bool) *admissionv1.AdmissionResponse {
	return mutateDVsEx(key, ar, isAuthorized, nil)
}

func mutateDVsEx(key *rsa.PrivateKey, ar *admissionv1.AdmissionReview, isAuthorized bool, cdiObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset()
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetResource().Resource != "subjectaccessreviews" {
			return false, nil, nil
		}

		sar := &authorization.SubjectAccessReview{
			Status: authorization.SubjectAccessReviewStatus{
				Allowed: isAuthorized,
				Reason:  fmt.Sprintf("isAuthorized=%t", isAuthorized),
			},
		}
		return true, sar, nil
	})
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	wh := NewDataVolumeMutatingWebhook(client, cdiClient, key)
	return serve(ar, wh)
}
