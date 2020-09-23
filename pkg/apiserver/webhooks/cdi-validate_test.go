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
	"encoding/json"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/api/admission/v1beta1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
)

var (
	block   = cdiv1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist
	noBlock = cdiv1.CDIUninstallStrategyRemoveWorkloads
)

var _ = Describe("CDI Delete Webhook", func() {
	Context("with CDI admission review", func() {
		DescribeTable("should accept with no DataVolumes present", func(strategy *cdiv1.CDIUninstallStrategy, op admissionv1beta1.Operation) {
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi",
				},
				Spec: cdiv1.CDISpec{
					UninstallStrategy: strategy,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: sdkapi.PhaseDeployed,
					},
				},
			}

			bytes, _ := json.Marshal(cdi)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: op,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "cdis",
					},
					OldObject: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateCDIs(ar)
			Expect(resp.Allowed).To(BeTrue())
		}, Entry("BLOCK DELETE", &block, admissionv1beta1.Delete),
			Entry("BLOCK UPDATE", &block, admissionv1beta1.Update),
			Entry("NO BLOCK DELETE", &noBlock, admissionv1beta1.Delete),
			Entry("NO BLOCK UPDATE", &noBlock, admissionv1beta1.Update),
			Entry("EMPTY DELETE", nil, admissionv1beta1.Delete),
			Entry("EMPTY UPDATE", nil, admissionv1beta1.Update),
		)

		DescribeTable("should accept with DataVolumes present", func(strategy *cdiv1.CDIUninstallStrategy, op admissionv1beta1.Operation) {
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi",
				},
				Spec: cdiv1.CDISpec{
					UninstallStrategy: strategy,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: sdkapi.PhaseDeployed,
					},
				},
			}

			bytes, _ := json.Marshal(cdi)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: op,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "cdis",
					},
					OldObject: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateCDIs(ar, newDataVolumeWithName("foo"))
			Expect(resp.Allowed).To(BeTrue())
		}, Entry("BLOCK UPDATE", &block, admissionv1beta1.Update),
			Entry("NO BLOCK DELETE", &noBlock, admissionv1beta1.Delete),
			Entry("NO BLOCK UPDATE", &noBlock, admissionv1beta1.Update),
			Entry("EMPTY DELETE", nil, admissionv1beta1.Delete),
			Entry("EMPTY UPDATE", nil, admissionv1beta1.Update),
		)

		It("should reject with DataVolumes present", func() {
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi",
				},
				Spec: cdiv1.CDISpec{
					UninstallStrategy: &block,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: sdkapi.PhaseDeployed,
					},
				},
			}

			bytes, _ := json.Marshal(cdi)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Delete,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "cdis",
					},
					OldObject: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateCDIs(ar, newDataVolumeWithName("foo"))
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("Rejecting the uninstall request, since there are still DataVolumes present."))
		})

		It("should reject with DataVolumes present and oldobject not populated", func() {
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi",
				},
				Spec: cdiv1.CDISpec{
					UninstallStrategy: &block,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: sdkapi.PhaseDeployed,
					},
				},
			}

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Delete,
					Name:      cdi.Name,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "cdis",
					},
				},
			}

			resp := validateCDIs(ar, cdi, newDataVolumeWithName("foo"))
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("Rejecting the uninstall request, since there are still DataVolumes present."))
		})

		It("should allow error CDI to be deleted with DataVolumes present", func() {
			cdi := &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi",
				},
				Spec: cdiv1.CDISpec{
					UninstallStrategy: &block,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: sdkapi.PhaseError,
					},
				},
			}

			bytes, _ := json.Marshal(cdi)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Delete,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "cdis",
					},
					OldObject: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateCDIs(ar, newDataVolumeWithName("foo"))
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject weird resource", func() {
			bytes, _ := json.Marshal(newDataVolumeWithName("foo"))

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Delete,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					OldObject: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateCDIs(ar)
			Expect(resp.Allowed).To(BeFalse())
		})

	})
})

func newDataVolumeWithName(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func validateCDIs(ar *admissionv1beta1.AdmissionReview, cdiObjects ...runtime.Object) *v1beta1.AdmissionResponse {
	client := cdiclient.NewSimpleClientset(cdiObjects...)
	wh := NewCDIValidatingWebhook(client)
	return serve(ar, wh)
}
