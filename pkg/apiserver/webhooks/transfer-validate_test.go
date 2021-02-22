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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/api/admission/v1beta1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "k8s.io/client-go/kubernetes/fake"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
)

var _ = Describe("ObjectTransfer webhook", func() {
	Describe("CREATE/UPDATE tests", func() {
		It("should reject invalid resource name", func() {
			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfersxxx",
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("unexpected resource: objecttransfersxxx"))
		})

		It("should allow unsupported operation", func() {
			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Delete,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Describe("CREATE tests", func() {
		It("Should reject no name/namespace", func() {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Name:      "dv",
						Namespace: "ns",
					},
				},
			}

			bytes, _ := json.Marshal(ot)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("Target namespace and/or target name must be supplied"))
		})

		It("Should reject invalid kind", func() {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolumexxx",
						Name:      "dv",
						Namespace: "ns",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"foo"}[0],
					},
				},
			}

			bytes, _ := json.Marshal(ot)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("Unsupported kind \"DataVolumexxx\""))
		})

		DescribeTable("Should reject target exists", func(kind string, k8sObjects, cdiObjects []runtime.Object) {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      kind,
						Name:      "source",
						Namespace: "ns",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"foo"}[0],
					},
				},
			}

			bytes, _ := json.Marshal(ot)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateObjectTransfers(ar, k8sObjects, cdiObjects)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("ObjectTransfer target \"foo/source\" already exists"))
		},
			Entry("DV", "DataVolume", nil, []runtime.Object{&cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source",
					Namespace: "foo",
				},
			}}),
			Entry("PVC", "PersistentVolumeClaim", []runtime.Object{&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source",
					Namespace: "foo",
				},
			}}, nil),
		)

		It("Should accept good stuff", func() {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Name:      "dv",
						Namespace: "ns",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"foo"}[0],
					},
				},
			}

			bytes, _ := json.Marshal(ot)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Describe("UPDATE tests", func() {
		It("Should reject spec update", func() {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Name:      "dv",
						Namespace: "ns",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"foo"}[0],
					},
				},
			}

			ot2 := ot.DeepCopy()
			ot2.Spec.Source.Namespace = "foo"

			bytes, _ := json.Marshal(ot)
			bytes2, _ := json.Marshal(ot2)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
					OldObject: runtime.RawExtension{
						Raw: bytes2,
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("ObjectTransfer spec is immutable"))
		})

		It("Should accept status update", func() {
			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Name:      "dv",
						Namespace: "ns",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"foo"}[0],
					},
				},
			}

			ot2 := ot.DeepCopy()
			ot2.Status.Phase = cdiv1.ObjectTransferPending

			bytes, _ := json.Marshal(ot)
			bytes2, _ := json.Marshal(ot2)

			ar := &admissionv1beta1.AdmissionReview{
				Request: &admissionv1beta1.AdmissionRequest{
					Operation: admissionv1beta1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "objecttransfers",
					},
					Object: runtime.RawExtension{
						Raw: bytes,
					},
					OldObject: runtime.RawExtension{
						Raw: bytes2,
					},
				},
			}

			resp := validateObjectTransfers(ar, nil, nil)
			Expect(resp.Allowed).To(BeTrue())
		})
	})
})

func validateObjectTransfers(ar *admissionv1beta1.AdmissionReview, k8sObjects, cdiObjects []runtime.Object) *v1beta1.AdmissionResponse {
	k8sClient := k8sclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclient.NewSimpleClientset(cdiObjects...)
	wh := NewObjectTransferValidatingWebhook(k8sClient, cdiClient)
	return serve(ar, wh)
}
