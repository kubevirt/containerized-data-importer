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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	cdiclientfake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var (
	testNamespace  = "testNamespace"
	emptyNamespace = ""
)

var _ = Describe("Validating Webhook", func() {
	Context("with DataVolume admission review", func() {
		It("should accept DataVolume with HTTP source on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should reject DataVolume when target pvc exists", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Name,
					Namespace: dataVolume.Namespace,
				},
				Spec: *dataVolume.Spec.PVC,
			}
			resp := validateDataVolumeCreate(dataVolume, pvc)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should accept DataVolume with Registry source on create", func() {
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should accept DataVolume with PVC source on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.Source.PVC.Name,
					Namespace: dataVolume.Spec.Source.PVC.Namespace,
				},
				Spec: *dataVolume.Spec.PVC,
			}
			resp := validateDataVolumeCreate(dataVolume, pvc)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should accept DataVolume with PVC initialized create", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Name,
					Namespace: dataVolume.Namespace,
					Annotations: map[string]string{
						"cdi.kubevirt.io/storage.populatedFor": dataVolume.Name,
					},
				},
				Spec: *dataVolume.Spec.PVC,
			}
			resp := validateDataVolumeCreate(dataVolume, pvc)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should reject DataVolume with PVC source on create if PVC does not exist", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject invalid DataVolume source PVC namespace on create", func() {
			dataVolume := newPVCDataVolume("testDV", "", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject invalid DataVolume source PVC name on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with name length greater than 253 characters", func() {
			longName := "the-name-length-of-this-datavolume-is-greater-then-253-characters" +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-" +
				"123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789-123456789"
			dataVolume := newHTTPDataVolume(
				longName,
				"http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume source with invalid URL on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "invalidurl")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with multiple sources on create", func() {
			dataVolume := newDataVolumeWithMultipleSources("testDV")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with empty PVC create", func() {
			dataVolume := newDataVolumeWithEmptyPVCSpec("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with PVC size 0", func() {
			dataVolume := newDataVolumeWithPVCSizeZero("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should accept DataVolume with Blank source and no content type", func() {
			dataVolume := newBlankDataVolume("blank")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(true))

		})

		It("should accept DataVolume with Blank source and kubevirt contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeKubeVirt
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(true))

		})

		It("should reject DataVolume with Blank source and archive contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))

		})

		It("should reject DataVolume with invalid contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = "invalid"
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))

		})

		It("should accept DataVolume with archive contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(true))

		})

		It("should reject invalid DataVolume spec update", func() {
			newDataVolume := newPVCDataVolume("testDV", "newNamespace", "testName")
			newBytes, _ := json.Marshal(&newDataVolume)

			oldDataVolume := newDataVolume.DeepCopy()
			oldDataVolume.Spec.Source.PVC.Namespace = "oldNamespace"
			oldBytes, _ := json.Marshal(oldDataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateAdmissionReview(ar)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should accept object meta update", func() {
			newDataVolume := newPVCDataVolume("testDV", "newNamespace", "testName")
			newBytes, _ := json.Marshal(&newDataVolume)

			oldDataVolume := newDataVolume.DeepCopy()
			oldDataVolume.Annotations = map[string]string{"foo": "bar"}
			oldBytes, _ := json.Marshal(oldDataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateAdmissionReview(ar)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should reject DataVolume spec PVC size update", func() {
			blankSource := cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			newDataVolume := newDataVolume("testDv", blankSource, pvc)
			newBytes, _ := json.Marshal(&newDataVolume)

			oldDataVolume := newDataVolume.DeepCopy()
			oldDataVolume.Spec.PVC.Resources.Requests["storage"] =
				*resource.NewQuantity(pvcSizeDefault+1, resource.BinarySI)
			oldBytes, _ := json.Marshal(oldDataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateAdmissionReview(ar)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should accept DataVolume spec PVC size format update", func() {
			blankSource := cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			newDataVolume := newDataVolume("testDv", blankSource, pvc)
			newBytes, _ := json.Marshal(&newDataVolume)

			oldDataVolume := newDataVolume.DeepCopy()
			oldDataVolume.Spec.PVC.Resources.Requests["storage"] =
				*resource.NewQuantity(pvcSizeDefault, resource.DecimalSI)
			oldBytes, _ := json.Marshal(oldDataVolume)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateAdmissionReview(ar)
			Expect(resp.Allowed).To(Equal(true))
		})

		DescribeTable("should", func(oldFinalCheckpoint bool, oldCheckpoints []string, newFinalCheckpoint bool, newCheckpoints []string, modifyDV func(*cdiv1.DataVolume), expectedSuccess bool, sourceType string) {
			oldDV := newMultistageDataVolume("multi-stage", oldFinalCheckpoint, oldCheckpoints, sourceType)
			oldBytes, _ := json.Marshal(&oldDV)

			newDV := newMultistageDataVolume("multi-stage", newFinalCheckpoint, newCheckpoints, sourceType)
			if modifyDV != nil {
				modifyDV(newDV)
			}
			newBytes, _ := json.Marshal(&newDV)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.SchemeGroupVersion.Group,
						Version:  cdiv1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: newBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldBytes,
					},
				},
			}

			resp := validateAdmissionReview(ar)
			Expect(resp.Allowed).To(Equal(expectedSuccess))
		},
			Entry("accept a spec change on multi-stage VDDK import fields", false, []string{"stage-1"}, true, []string{"stage-1", "stage-2"}, nil, true, "vddk"),

			Entry("reject a spec change on un-approved fields of a multi-stage VDDK import", false, []string{"stage-1"}, true, []string{"stage-1", "stage-2"}, func(newDV *cdiv1.DataVolume) { newDV.Spec.Source.VDDK.URL = "testing123" }, false, "vddk"),

			Entry("accept identical multi-stage VDDK import field changes", false, []string{"stage-1"}, false, []string{"stage-1"}, nil, true, "vddk"),

			Entry("reject a spec change on un-approved fields, even with identical non-empty multi-stage fields", false, []string{"stage-1"}, false, []string{"stage-1"}, func(newDV *cdiv1.DataVolume) { newDV.Spec.Source.VDDK.URL = "tesing123" }, false, "vddk"),

			Entry("accept a spec change on multi-stage ImageIO import fields", false, []string{"snapshot-123"}, true, []string{"snapshot-123", "snapshot-234"}, nil, true, "imageio"),

			Entry("reject a spec change on source type that does not support multi-stage import", false, []string{}, true, []string{}, nil, false, "blank"),
		)
	})

	Context("with DataVolume (using sourceRef) admission review", func() {
		DescribeTable("should", func(dataSourceNamespace *string) {
			pvcName := "testPVC"
			dataVolume := newDataSourceDataVolume("testDV", dataSourceNamespace, "test")
			dataVolume.Namespace = testNamespace
			dataSource := &cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.SourceRef.Name,
					Namespace: testNamespace,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						PVC: &cdiv1.DataVolumeSourcePVC{
							Name:      pvcName,
							Namespace: testNamespace,
						},
					},
				},
			}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: testNamespace,
				},
				Spec: *newPVCSpec(pvcSizeDefault),
			}
			resp := validateDataVolumeCreateEx(dataVolume, []runtime.Object{pvc}, []runtime.Object{dataSource})
			Expect(resp.Allowed).To(Equal(true))
		},
			Entry("accept DataVolume with PVC and sourceRef on create", &testNamespace),
			Entry("accept DataVolume with PVC and sourceRef nil namespace on create", nil),
			Entry("accept DataVolume with PVC and sourceRef missing namespace on create", &emptyNamespace),
		)

		It("should reject DataVolume with SourceRef on create if DataSource does not exist", func() {
			ns := "testNamespace"
			dataVolume := newDataSourceDataVolume("testDV", &ns, "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with SourceRef on create if DataSource exists but PVC does not exist", func() {
			dataVolume := newDataSourceDataVolume("testDV", &testNamespace, "test")
			dataSource := &cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.SourceRef.Name,
					Namespace: testNamespace,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						PVC: &cdiv1.DataVolumeSourcePVC{
							Name:      "testPVC",
							Namespace: testNamespace,
						},
					},
				},
			}
			resp := validateDataVolumeCreateEx(dataVolume, nil, []runtime.Object{dataSource})
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with empty SourceRef name on create", func() {
			dataVolume := newDataSourceDataVolume("testDV", &testNamespace, "")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with both source and sourceRef on create", func() {
			dataVolume := newDataVolumeWithBothSourceAndSourceRef("testDV", "testNamespace", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})

		It("should reject DataVolume with no source or sourceRef on create", func() {
			dataVolume := newDataVolumeWithNoSourceOrSourceRef("testDV")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(Equal(false))
		})
	})
})

func newMultistageDataVolume(name string, final bool, checkpoints []string, sourceType string) *cdiv1.DataVolume {
	pvc := newPVCSpec(pvcSizeDefault)

	previous := ""
	dvCheckpoints := make([]cdiv1.DataVolumeCheckpoint, len(checkpoints))
	for index, checkpoint := range checkpoints {
		dvCheckpoints[index] = cdiv1.DataVolumeCheckpoint{
			Current:  checkpoint,
			Previous: previous,
		}
		previous = checkpoint
	}

	var source *cdiv1.DataVolumeSource
	if sourceType == "vddk" {
		source = &cdiv1.DataVolumeSource{
			VDDK: &cdiv1.DataVolumeSourceVDDK{
				BackingFile: "disk.img",
				URL:         "http://example.com/data",
				UUID:        "12345",
				Thumbprint:  "aa:bb:cc",
				SecretRef:   "secret",
			},
		}
	} else if sourceType == "imageio" {
		source = &cdiv1.DataVolumeSource{
			Imageio: &cdiv1.DataVolumeSourceImageIO{
				URL:           "http://example.com/data",
				DiskID:        "disk-123",
				SecretRef:     "secret",
				CertConfigMap: "certs",
			},
		}
	} else if sourceType == "blank" {
		source = &cdiv1.DataVolumeSource{
			Blank: &cdiv1.DataVolumeBlankImage{},
		}
	} else {
		return nil
	}

	namespace := metav1.NamespaceDefault
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			SelfLink:  fmt.Sprintf("/apis/%s/namespaces/%s/datavolumes/%s", cdiv1.SchemeGroupVersion.String(), namespace, name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdiv1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: cdiv1.DataVolumeStatus{},
		Spec: cdiv1.DataVolumeSpec{
			Source:          source,
			FinalCheckpoint: final,
			Checkpoints:     dvCheckpoints,
			PVC:             pvc,
		},
	}
	return dv
}

func newHTTPDataVolume(name, url string) *cdiv1.DataVolume {
	httpSource := cdiv1.DataVolumeSource{
		HTTP: &cdiv1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolume(name, httpSource, pvc)
}

func newRegistryDataVolume(name, url string) *cdiv1.DataVolume {
	registrySource := cdiv1.DataVolumeSource{
		Registry: &cdiv1.DataVolumeSourceRegistry{URL: url},
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolume(name, registrySource, pvc)
}

func newBlankDataVolume(name string) *cdiv1.DataVolume {
	blankSource := cdiv1.DataVolumeSource{
		Blank: &cdiv1.DataVolumeBlankImage{},
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolume(name, blankSource, pvc)
}

func newPVCDataVolume(name, pvcNamespace, pvcName string) *cdiv1.DataVolume {
	pvcSource := cdiv1.DataVolumeSource{
		PVC: &cdiv1.DataVolumeSourcePVC{
			Namespace: pvcNamespace,
			Name:      pvcName,
		},
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolume(name, pvcSource, pvc)
}

func newDataVolumeWithEmptyPVCSpec(name, url string) *cdiv1.DataVolume {

	httpSource := cdiv1.DataVolumeSource{
		HTTP: &cdiv1.DataVolumeSourceHTTP{URL: url},
	}

	return newDataVolume(name, httpSource, nil)
}

func newDataVolumeWithMultipleSources(name string) *cdiv1.DataVolume {
	source := cdiv1.DataVolumeSource{
		HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "http://www.example.com"},
		S3:   &cdiv1.DataVolumeSourceS3{URL: "http://s3.examples3.com"},
	}
	pvc := newPVCSpec(pvcSizeDefault)

	return newDataVolume(name, source, pvc)
}

func newDataVolumeWithPVCSizeZero(name, url string) *cdiv1.DataVolume {
	httpSource := cdiv1.DataVolumeSource{
		HTTP: &cdiv1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(0)

	return newDataVolume(name, httpSource, pvc)
}

func newDataSourceDataVolume(name string, sourceRefNamespace *string, sourceRefName string) *cdiv1.DataVolume {
	sourceRef := cdiv1.DataVolumeSourceRef{
		Kind:      cdiv1.DataVolumeDataSource,
		Namespace: sourceRefNamespace,
		Name:      sourceRefName,
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolumeWithSourceRef(name, nil, &sourceRef, pvc)
}

func newDataVolumeWithBothSourceAndSourceRef(name string, pvcNamespace string, pvcName string) *cdiv1.DataVolume {
	pvcSource := cdiv1.DataVolumeSource{
		PVC: &cdiv1.DataVolumeSourcePVC{
			Namespace: pvcNamespace,
			Name:      pvcName,
		},
	}
	sourceRef := cdiv1.DataVolumeSourceRef{
		Kind:      cdiv1.DataVolumeDataSource,
		Namespace: &pvcNamespace,
		Name:      pvcName,
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolumeWithSourceRef(name, &pvcSource, &sourceRef, pvc)
}

func newDataVolumeWithNoSourceOrSourceRef(name string) *cdiv1.DataVolume {
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolumeWithSourceRef(name, nil, nil, pvc)
}

func newDataVolume(name string, source cdiv1.DataVolumeSource, pvc *corev1.PersistentVolumeClaimSpec) *cdiv1.DataVolume {
	return newDataVolumeWithSourceRef(name, &source, nil, pvc)
}

func newDataVolumeWithSourceRef(name string, source *cdiv1.DataVolumeSource, sourceRef *cdiv1.DataVolumeSourceRef, pvc *corev1.PersistentVolumeClaimSpec) *cdiv1.DataVolume {
	namespace := k8sv1.NamespaceDefault
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			SelfLink:  fmt.Sprintf("/apis/%s/namespaces/%s/datavolumes/%s", cdiv1.SchemeGroupVersion.String(), namespace, name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdiv1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: cdiv1.DataVolumeStatus{},
		Spec: cdiv1.DataVolumeSpec{
			Source:    source,
			SourceRef: sourceRef,
			PVC:       pvc,
		},
	}
	return dv
}

const pvcSizeDefault = 5 << 20 // 5Mi

func newPVCSpec(sizeValue int64) *corev1.PersistentVolumeClaimSpec {
	requests := make(map[corev1.ResourceName]resource.Quantity)
	requests["storage"] = *resource.NewQuantity(sizeValue, resource.BinarySI)

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

func validateDataVolumeCreate(dv *cdiv1.DataVolume, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	return validateDataVolumeCreateEx(dv, objects, nil)
}

func validateDataVolumeCreateEx(dv *cdiv1.DataVolume, k8sObjects, cdiObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	wh := NewDataVolumeValidatingWebhook(client, cdiClient)

	dvBytes, _ := json.Marshal(dv)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Resource: metav1.GroupVersionResource{
				Group:    cdiv1.SchemeGroupVersion.Group,
				Version:  cdiv1.SchemeGroupVersion.Version,
				Resource: "datavolumes",
			},
			Object: runtime.RawExtension{
				Raw: dvBytes,
			},
		},
	}

	return serve(ar, wh)
}

func validateAdmissionReview(ar *admissionv1.AdmissionReview, objects ...runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(objects...)
	cdiClient := cdiclientfake.NewSimpleClientset()
	wh := NewDataVolumeValidatingWebhook(client, cdiClient)
	return serve(ar, wh)
}

func serve(ar *admissionv1.AdmissionReview, handler http.Handler) *admissionv1.AdmissionResponse {
	reqBytes, _ := json.Marshal(ar)
	req, err := http.NewRequest("POST", "/foobar", bytes.NewReader(reqBytes))
	Expect(err).ToNot(HaveOccurred())

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var response admissionv1.AdmissionReview
	err = json.NewDecoder(rr.Body).Decode(&response)
	Expect(err).ToNot(HaveOccurred())

	return response.Response
}
