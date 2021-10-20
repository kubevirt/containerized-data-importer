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
 * Copyright 2021 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclientfake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"
)

var (
	testRegistryURL  = "docker://registry:5000/test"
	testImageStream  = "test-is"
	registryPullNode = cdiv1.RegistryPullNode
)

var _ = Describe("Validating Webhook", func() {
	Context("with DataImportCron admission review", func() {
		It("should accept DataImportCron with Registry source URL on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should accept DataImportCron with Registry source ImageStream and node PullMethod on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{ImageStream: &testImageStream, PullMethod: &registryPullNode})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should reject DataImportCron with name length longer than 253 characters", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			cron.Name = "the-name-length-of-this-dataimportcron-is-longer-then-253-characters" +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789"
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with Registry source ImageStream and pod PullMethod on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{ImageStream: &testImageStream})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with no Registry source URL or ImageStream on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with no Registry source on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{})
			cron.Spec.Template.Spec.Source.Registry = nil
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with no source on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{})
			cron.Spec.Template.Spec.Source = nil
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with unsettable template field on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			ref := cdiv1.DataVolumeSourceRef{Kind: cdiv1.DataVolumeDataSource, Name: "noname"}
			cron.Spec.Template.Spec.SourceRef = &ref
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with both Registry source URL and ImageStream on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL, ImageStream: &testImageStream})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with illegal Registry source URL on create", func() {
			url := "invalidurl"
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &url})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with Registry source URL illegal transport on create", func() {
			url := "joker://registry:5000/test"
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &url})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with Registry source URL illegal importMethod on create", func() {
			pullMethod := cdiv1.RegistryPullMethod("nosuch")
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL, PullMethod: &pullMethod})
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with illegal cron schedule", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			cron.Spec.Schedule = "61 * * * *"
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with illegal ManagedDataSource on create", func() {
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			cron.Spec.ManagedDataSource = ""
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataImportCron with illegal GarbageCollect on create", func() {
			garbageCollect := cdiv1.DataImportCronGarbageCollect("nosuch")
			cron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			cron.Spec.GarbageCollect = &garbageCollect
			resp := validateDataImportCronCreate(cron)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject invalid DataImportCron spec update", func() {
			newCron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			newBytes, _ := json.Marshal(&newCron)

			otherURL := "docker://registry:5000/other"
			oldCron := newCron.DeepCopy()
			oldCron.Spec.Template.Spec.Source.Registry.URL = &otherURL
			oldBytes, _ := json.Marshal(oldCron)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "dataimportcrons",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateDataImportCron(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should accept object meta update", func() {
			newCron := newDataImportCron(cdiv1.DataVolumeSourceRegistry{URL: &testRegistryURL})
			newBytes, _ := json.Marshal(&newCron)

			oldCron := newCron.DeepCopy()
			oldCron.Annotations = map[string]string{"foo": "bar"}
			oldBytes, _ := json.Marshal(oldCron)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "dataimportcrons",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateDataImportCron(ar)
			Expect(resp.Allowed).To(Equal(true))
		})
	})
})

func newDataImportCron(source cdiv1.DataVolumeSourceRegistry) *cdiv1.DataImportCron {
	namespace := k8sv1.NamespaceDefault
	name := "testCron"
	cron := &cdiv1.DataImportCron{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			SelfLink:  fmt.Sprintf("/apis/%s/namespaces/%s/dataimportcrons/%s", cdiv1.SchemeGroupVersion.String(), namespace, name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdiv1.SchemeGroupVersion.String(),
			Kind:       "DataImportCron",
		},
		Status: cdiv1.DataImportCronStatus{},
		Spec: cdiv1.DataImportCronSpec{
			Template: cdiv1.DataVolume{
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{
						Registry: &source,
					},
					PVC: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Mi"),
							},
						},
					},
				},
			},
			Schedule:          "30 1 * * 1",
			ManagedDataSource: "someDataSource",
		},
	}
	return cron
}

func validateDataImportCronCreate(cron *cdiv1.DataImportCron, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	return validateDataImportCronCreateEx(cron, objects, nil)
}

func validateDataImportCronCreateEx(cron *cdiv1.DataImportCron, k8sObjects, cdiObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	wh := NewDataImportCronValidatingWebhook(client, cdiClient)

	cronBytes, _ := json.Marshal(cron)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    cdiv1.SchemeGroupVersion.Group,
				Version:  cdiv1.SchemeGroupVersion.Version,
				Resource: "dataimportcrons",
			},
			Object: runtime.RawExtension{
				Raw: cronBytes,
			},
		},
	}

	return serve(ar, wh)
}

func validateDataImportCron(ar *admissionv1.AdmissionReview, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(objects...)
	cdiClient := cdiclientfake.NewSimpleClientset()
	wh := NewDataImportCronValidatingWebhook(client, cdiClient)
	return serve(ar, wh)
}
