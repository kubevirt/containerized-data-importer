package validatingwebhook

import (
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/api/admission/v1beta1"
	authorization "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	cdicorev1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

func TestValidatingWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ValidatingWebhook Suite")
}

var _ = Describe("DataVolume Validating Webhook", func() {
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should accept DataVolume with PVC source on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
				},
			}
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

			resp := dvAdmitWithAuthorization(ar, true, pvc)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should NOT accept DataVolume with PVC source and auth failure", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
				},
			}
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

			resp := dvAdmitWithAuthorization(ar, false, pvc)
			Expect(resp.Allowed).To(Equal(false))
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
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

			resp := dvAdmit(ar)
			Expect(resp.Allowed).To(Equal(true))

		})
	})
})

var _ = Describe("PVC Validating Webhook", func() {
	Context("with DataVolume admission review", func() {
		DescribeTable("should", func(cloneRequest string, authorized bool) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"k8s.io/CloneRequest": cloneRequest,
					},
				},
			}
			pvcBytes, _ := json.Marshal(pvc)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "persistentvolumeclaims",
					},
					Object: runtime.RawExtension{
						Raw: pvcBytes,
					},
				},
			}

			resp := pvcAdmitWithAuthorization(ar, authorized)
			Expect(resp.Allowed).To(Equal(authorized))
		},
			Entry("succeed on create with authorization", "foo/bar", true),
			Entry("fail on create without authorization", "foo/bar", false),
			Entry("succeed on create with bad cloneRequest", "", true),
			Entry("succeed on create with bad cloneRequest", "foo", true),
		)

		It("should fail unexpected type", func() {
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
				},
			}

			resp := pvcAdmit(ar)
			Expect(resp.Allowed).To(Equal(false))
		})

		DescribeTable("should", func(cloneRequest string, authorized, allowed bool) {
			oldPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"k8s.io/CloneRequest": "foo/bar",
					},
				},
			}
			oldPVCBytes, _ := json.Marshal(oldPVC)

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"k8s.io/CloneRequest": cloneRequest,
					},
				},
			}
			pvcBytes, _ := json.Marshal(pvc)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "persistentvolumeclaims",
					},
					OldObject: runtime.RawExtension{
						Raw: oldPVCBytes,
					},
					Object: runtime.RawExtension{
						Raw: pvcBytes,
					},
				},
			}

			resp := pvcAdmitWithAuthorization(ar, authorized)
			Expect(resp.Allowed).To(Equal(allowed))
		},
			Entry("succeed on update with authorization", "foo/baz", true, true),
			Entry("fail on update without authorization", "foo/baz", false, false),
			Entry("succeed no change", "foo/bar", false, true),
		)

		It("should fail unexpected type", func() {
			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
				},
			}

			resp := pvcAdmit(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
	})
})

type webhookFunc func(kubernetes.Interface) ValidatingWebhook

func dvAdmit(ar *v1beta1.AdmissionReview, objects ...runtime.Object) *v1beta1.AdmissionResponse {
	return dvAdmitWithAuthorization(ar, false, objects...)
}

func dvAdmitWithAuthorization(ar *v1beta1.AdmissionReview, isAuthorized bool, objects ...runtime.Object) *v1beta1.AdmissionResponse {
	return admitWithAuthorization(NewDataVolumeWebhook, ar, isAuthorized, objects...)
}

func pvcAdmit(ar *v1beta1.AdmissionReview, objects ...runtime.Object) *v1beta1.AdmissionResponse {
	return pvcAdmitWithAuthorization(ar, false, objects...)
}

func pvcAdmitWithAuthorization(ar *v1beta1.AdmissionReview, isAuthorized bool, objects ...runtime.Object) *v1beta1.AdmissionResponse {
	return admitWithAuthorization(NewPVCWebhook, ar, isAuthorized, objects...)
}

func admitWithAuthorization(f webhookFunc, ar *v1beta1.AdmissionReview, isAuthorized bool, objects ...runtime.Object) *v1beta1.AdmissionResponse {
	client := k8sfake.NewSimpleClientset(objects...)
	client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
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
	wh := f(client)
	return wh.Admit(ar)
}

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
	namespace := corev1.NamespaceDefault
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
