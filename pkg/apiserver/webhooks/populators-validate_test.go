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
 * Copyright 2023 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"

	cdiclientfake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var (
	testUploadPopulatorName = "test-upload-populator"
	testImportPopulatorName = "test-import-populator"
)

var _ = Describe("Validating Webhook", func() {
	longName := "name-is-longer-than-253-" + strings.Repeat("0123456789", 23)

	Context("with VolumeImportSource admission review", func() {
		It("should reject VolumeImportSource with invalid name", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, &cdiv1.ImportSourceType{HTTP: &cdiv1.DataVolumeSourceHTTP{}})
			importCR.Name = longName
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource without source", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, &cdiv1.ImportSourceType{})
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource with invalid ContentType", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeContentType("invalid"), &cdiv1.ImportSourceType{})
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource with more than one source", func() {
			url := "docker://registry:5000/test"
			source := &cdiv1.ImportSourceType{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://www.example.com",
				},
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: &url,
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource spec update", func() {
			source := &cdiv1.ImportSourceType{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://www.example.com",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			newBytes, _ := json.Marshal(&importCR)

			oldSource := importCR.DeepCopy()
			oldSource.Spec.Source.HTTP.URL = "http://www.example.es"
			oldBytes, _ := json.Marshal(oldSource)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "volumeimportsources",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}
			resp := validatePopulatorsAdmissionReview(ar)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept VolumeImportSource with HTTP source on create", func() {
			source := &cdiv1.ImportSourceType{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://www.example.com",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept VolumeImportSource with GS source on create", func() {
			source := &cdiv1.ImportSourceType{
				GCS: &cdiv1.DataVolumeSourceGCS{
					URL: "gs://www.example.com",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject VolumeImportSource with GCS source on create", func() {
			source := &cdiv1.ImportSourceType{
				GCS: &cdiv1.DataVolumeSourceGCS{
					URL: "gcs://www.example.com",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource with incomplete VDDK source", func() {
			source := &cdiv1.ImportSourceType{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: "",
					URL:         "",
					UUID:        "",
					Thumbprint:  "",
					SecretRef:   "",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject multi-stage VolumeImportSource without TargetClaim", func() {
			source := &cdiv1.ImportSourceType{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: "[iSCSI_Datastore] vm/vm_1.vmdk",
					URL:         "https://vcenter.corp.com",
					UUID:        "52260566-b032-36cb-55b1-79bf29e30490",
					Thumbprint:  "20:6C:8A:5D:44:40:B3:79:4B:28:EA:76:13:60:90:6E:49:D9:D9:A3",
					SecretRef:   "vddk-credentials",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			importCR.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{Current: "test", Previous: ""},
			}
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept multi-stage VolumeImportSource with TargetClaim", func() {
			source := &cdiv1.ImportSourceType{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: "[iSCSI_Datastore] vm/vm_1.vmdk",
					URL:         "https://vcenter.corp.com",
					UUID:        "52260566-b032-36cb-55b1-79bf29e30490",
					Thumbprint:  "20:6C:8A:5D:44:40:B3:79:4B:28:EA:76:13:60:90:6E:49:D9:D9:A3",
					SecretRef:   "vddk-credentials",
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			importCR.Spec.Checkpoints = []cdiv1.DataVolumeCheckpoint{
				{Current: "test", Previous: ""},
			}
			targetClaim := "test-pvc"
			importCR.Spec.TargetClaim = &targetClaim
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept VolumeImportSource with Registry source URL on create", func() {
			url := "docker://registry:5000/test"
			source := &cdiv1.ImportSourceType{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: &url,
				},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, source)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept VolumeImportSource with Registry source ImageStream and node PullMethod on create", func() {
			imageStream := "istream"
			pullNode := cdiv1.RegistryPullNode
			registrySource := &cdiv1.ImportSourceType{
				Registry: &cdiv1.DataVolumeSourceRegistry{ImageStream: &imageStream, PullMethod: &pullNode},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, registrySource)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject VolumeImportSource with Registry source ImageStream and pod PullMethod on create", func() {
			imageStream := "istream"
			registrySource := &cdiv1.ImportSourceType{
				Registry: &cdiv1.DataVolumeSourceRegistry{ImageStream: &imageStream},
			}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, registrySource)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource with Registry source on create with no url or ImageStream", func() {
			registrySource := &cdiv1.ImportSourceType{Registry: &cdiv1.DataVolumeSourceRegistry{}}
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, registrySource)
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeImportSource with blank source and archive ContentType", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeArchive, &cdiv1.ImportSourceType{Blank: &cdiv1.DataVolumeBlankImage{}})
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept VolumeImportSource with blank source and kubevirt ContentType", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeKubeVirt, &cdiv1.ImportSourceType{Blank: &cdiv1.DataVolumeBlankImage{}})
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept VolumeImportSource with empty ContentType", func() {
			importCR := newVolumeImportSource(cdiv1.DataVolumeContentType(""), &cdiv1.ImportSourceType{Blank: &cdiv1.DataVolumeBlankImage{}})
			resp := validateVolumeImportSourceCreate(importCR)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("with VolumeUploadSource admission review", func() {
		It("should reject VolumeUploadSource with invalid name", func() {
			uploadCR := newVolumeUploadSource(cdiv1.DataVolumeKubeVirt)
			uploadCR.Name = longName
			resp := validateVolumeUploadSourceCreate(uploadCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeUploadSource with invalid ContentType", func() {
			uploadCR := newVolumeUploadSource(cdiv1.DataVolumeContentType("invalid"))
			resp := validateVolumeUploadSourceCreate(uploadCR)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject VolumeUploadSource spec update", func() {
			uploadCR := newVolumeUploadSource(cdiv1.DataVolumeKubeVirt)
			newBytes, _ := json.Marshal(&uploadCR)

			oldSource := uploadCR.DeepCopy()
			oldSource.Spec.ContentType = cdiv1.DataVolumeArchive
			oldBytes, _ := json.Marshal(oldSource)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "volumeuploadsources",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}
			resp := validatePopulatorsAdmissionReview(ar)
			Expect(resp.Allowed).To(BeFalse())
		})
	})

	It("should accept VolumeUploadSource with KubeVirt ContentType and valid name", func() {
		uploadCR := newVolumeUploadSource(cdiv1.DataVolumeKubeVirt)
		resp := validateVolumeUploadSourceCreate(uploadCR)
		Expect(resp.Allowed).To(BeTrue())
	})

	It("should accept VolumeUploadSource with Archive ContentType and valid name", func() {
		uploadCR := newVolumeUploadSource(cdiv1.DataVolumeArchive)
		resp := validateVolumeUploadSourceCreate(uploadCR)
		Expect(resp.Allowed).To(BeTrue())
	})

	It("should accept VolumeUploadSource with empty ContentType and valid name", func() {
		uploadCR := newVolumeUploadSource(cdiv1.DataVolumeContentType(""))
		resp := validateVolumeUploadSourceCreate(uploadCR)
		Expect(resp.Allowed).To(BeTrue())
	})
})

func newVolumeUploadSource(contentType cdiv1.DataVolumeContentType) *cdiv1.VolumeUploadSource {
	return &cdiv1.VolumeUploadSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testUploadPopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.VolumeUploadSourceSpec{
			ContentType: cdiv1.DataVolumeContentType(contentType),
		},
	}
}

func newVolumeImportSource(contentType cdiv1.DataVolumeContentType, source *cdiv1.ImportSourceType) *cdiv1.VolumeImportSource {
	return &cdiv1.VolumeImportSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testImportPopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.VolumeImportSourceSpec{
			ContentType: cdiv1.DataVolumeContentType(contentType),
			Source:      source,
		},
	}
}

func validateVolumeImportSourceCreate(source *cdiv1.VolumeImportSource, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	return validateVolumeImportSourceCreateEx(source, objects, nil, nil)
}

func validateVolumeUploadSourceCreate(source *cdiv1.VolumeUploadSource, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	return validateVolumeUploadSourceCreateEx(source, objects, nil, nil)
}

func validateVolumeImportSourceCreateEx(source *cdiv1.VolumeImportSource, k8sObjects, cdiObjects, snapObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	wh := NewPopulatorValidatingWebhook(client, cdiClient)

	dvBytes, _ := json.Marshal(source)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    cdiv1.SchemeGroupVersion.Group,
				Version:  cdiv1.SchemeGroupVersion.Version,
				Resource: "volumeimportsources",
			},
			Object: runtime.RawExtension{
				Raw: dvBytes,
			},
		},
	}

	return serve(ar, wh)
}

func validateVolumeUploadSourceCreateEx(source *cdiv1.VolumeUploadSource, k8sObjects, cdiObjects, snapObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	wh := NewPopulatorValidatingWebhook(client, cdiClient)

	dvBytes, _ := json.Marshal(source)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    cdiv1.SchemeGroupVersion.Group,
				Version:  cdiv1.SchemeGroupVersion.Version,
				Resource: "volumeuploadsources",
			},
			Object: runtime.RawExtension{
				Raw: dvBytes,
			},
		},
	}

	return serve(ar, wh)
}

func validatePopulatorsAdmissionReview(ar *admissionv1.AdmissionReview, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(objects...)
	cdiClient := cdiclientfake.NewSimpleClientset()
	wh := NewPopulatorValidatingWebhook(client, cdiClient)
	return serve(ar, wh)
}
