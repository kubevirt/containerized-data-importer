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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cdicorev1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

var _ = Describe("Validating Webhook", func() {
	Context("with DataVolume admission review", func() {
		It("should accept DataVolume with HTTP source on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should accept DataVolume with Registry source on create", func() {
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should accept DataVolume with PVC source on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should reject invalid DataVolume source PVC namespace on create", func() {
			dataVolume := newPVCDataVolume("testDV", "", "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject invalid DataVolume source PVC name on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume source with invalid URL on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "invalidurl")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with multiple sources on create", func() {
			dataVolume := newDataVolumeWithMultipleSources("testDV")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with empty PVC create", func() {
			dataVolume := newDataVolumeWithEmptyPVCSpec("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with PVC size 0", func() {
			dataVolume := newDataVolumeWithPVCSizeZero("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should accept DataVolume with Blank source and no content type", func() {
			dataVolume := newBlankDataVolume("blank")
			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))

		})
		It("should accept DataVolume with Blank source and kubevirt contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdicorev1alpha1.DataVolumeKubeVirt

			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))

		})
		It("should reject DataVolume with Blank source and archive contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdicorev1alpha1.DataVolumeArchive

			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))

		})
		It("should reject DataVolume with invalid contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = "invalid"

			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(false))

		})
		It("should accept DataVolume with archive contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = cdicorev1alpha1.DataVolumeArchive

			dvBytes, _ := json.Marshal(&dataVolume)
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
						Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := validateDVs(ar)
			Expect(resp.Allowed).To(Equal(true))

		})
	})
})

func newHTTPDataVolume(name, url string) *cdicorev1alpha1.DataVolume {
	httpSource := cdicorev1alpha1.DataVolumeSource{
		HTTP: &cdicorev1alpha1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, httpSource, pvc)
}

func newRegistryDataVolume(name, url string) *cdicorev1alpha1.DataVolume {
	registrySource := cdicorev1alpha1.DataVolumeSource{
		Registry: &cdicorev1alpha1.DataVolumeSourceRegistry{URL: url},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, registrySource, pvc)
}

func newBlankDataVolume(name string) *cdicorev1alpha1.DataVolume {
	blankSource := cdicorev1alpha1.DataVolumeSource{
		Blank: &cdicorev1alpha1.DataVolumeBlankImage{},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, blankSource, pvc)
}

func newPVCDataVolume(name, pvcNamespace, pvcName string) *cdicorev1alpha1.DataVolume {
	pvcSource := cdicorev1alpha1.DataVolumeSource{
		PVC: &cdicorev1alpha1.DataVolumeSourcePVC{
			Namespace: pvcNamespace,
			Name:      pvcName,
		},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, pvcSource, pvc)
}

func newDataVolumeWithEmptyPVCSpec(name, url string) *cdicorev1alpha1.DataVolume {

	httpSource := cdicorev1alpha1.DataVolumeSource{
		HTTP: &cdicorev1alpha1.DataVolumeSourceHTTP{URL: url},
	}

	return newDataVolume(name, httpSource, nil)
}

func newDataVolumeWithMultipleSources(name string) *cdicorev1alpha1.DataVolume {
	source := cdicorev1alpha1.DataVolumeSource{
		HTTP: &cdicorev1alpha1.DataVolumeSourceHTTP{URL: "http://www.example.com"},
		S3:   &cdicorev1alpha1.DataVolumeSourceS3{URL: "http://s3.examples3.com"},
	}
	pvc := newPVCSpec(5, "M")

	return newDataVolume(name, source, pvc)
}

func newDataVolumeWithPVCSizeZero(name, url string) *cdicorev1alpha1.DataVolume {

	httpSource := cdicorev1alpha1.DataVolumeSource{
		HTTP: &cdicorev1alpha1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(0, "M")

	return newDataVolume(name, httpSource, pvc)
}

func newDataVolume(name string, source cdicorev1alpha1.DataVolumeSource, pvc *corev1.PersistentVolumeClaimSpec) *cdicorev1alpha1.DataVolume {
	namespace := k8sv1.NamespaceDefault
	dv := &cdicorev1alpha1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			SelfLink:  fmt.Sprintf("/apis/%s/namespaces/%s/datavolumes/%s", cdicorev1alpha1.SchemeGroupVersion.String(), namespace, name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdicorev1alpha1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: cdicorev1alpha1.DataVolumeStatus{},
		Spec: cdicorev1alpha1.DataVolumeSpec{
			Source: source,
			PVC:    pvc,
		},
	}

	return dv

}

func newPVCSpec(sizeValue int64, sizeFormat resource.Format) *corev1.PersistentVolumeClaimSpec {
	requests := make(map[corev1.ResourceName]resource.Quantity)
	requests["storage"] = *resource.NewQuantity(sizeValue, sizeFormat)

	pvc := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
		Resources: corev1.ResourceRequirements{
			Requests: requests,
		},
	}
	return pvc
}

func validateDVs(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	wh := NewDataVolumeValidatingWebhook(nil)
	return serve(ar, wh)
}

func serve(ar *v1beta1.AdmissionReview, handler http.Handler) *v1beta1.AdmissionResponse {
	reqBytes, _ := json.Marshal(ar)
	req, err := http.NewRequest("POST", "/foobar", bytes.NewReader(reqBytes))
	Expect(err).ToNot(HaveOccurred())

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var response v1beta1.AdmissionReview
	err = json.NewDecoder(rr.Body).Decode(&response)
	Expect(err).ToNot(HaveOccurred())

	return response.Response
}
