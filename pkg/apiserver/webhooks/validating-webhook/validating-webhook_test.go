package validatingwebhook

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	datavolumev1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/datavolumecontroller/v1alpha1"
)

func TestValidatingWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ValidatingWebhook Suite")
}

var _ = Describe("Validating Webhook", func() {
	Context("with DataVolume admission review", func() {
		It("should accept DataVolume with HTTP source on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(true))
		})
		It("should accept DataVolume with PVC source on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(true))
		})

		It("should reject invalid DataVolume source PVC namespace on create", func() {
			dataVolume := newPVCDataVolume("testDV", "", "test")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject invalid DataVolume source PVC name on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume source with invalid URL on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "invalidurl")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with multiple sources on create", func() {
			dataVolume := newDataVolumeWithMultipleSources("testDV")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with empty PVC create", func() {
			dataVolume := newDataVolumeWithEmptyPVCSpec("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
		It("should reject DataVolume with PVC size 0", func() {
			dataVolume := newDataVolumeWithPVCSizeZero("testDV", "http://www.example.com")
			dvBytes, _ := json.Marshal(&dataVolume)

			ar := &v1beta1.AdmissionReview{
				Request: &v1beta1.AdmissionRequest{
					Resource: metav1.GroupVersionResource{
						Group:    datavolumev1alpha1.SchemeGroupVersion.Group,
						Version:  datavolumev1alpha1.SchemeGroupVersion.Version,
						Resource: "datavolumes",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := admitDVs(ar)
			Expect(resp.Allowed).To(Equal(false))
		})
	})
})

func newHTTPDataVolume(name, url string) *datavolumev1alpha1.DataVolume {
	httpSource := datavolumev1alpha1.DataVolumeSource{
		HTTP: &datavolumev1alpha1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, httpSource, pvc)
}

func newPVCDataVolume(name, pvcNamespace, pvcName string) *datavolumev1alpha1.DataVolume {
	pvcSource := datavolumev1alpha1.DataVolumeSource{
		PVC: &datavolumev1alpha1.DataVolumeSourcePVC{
			Namespace: pvcNamespace,
			Name:      pvcName,
		},
	}
	pvc := newPVCSpec(5, "M")
	return newDataVolume(name, pvcSource, pvc)
}

func newDataVolumeWithEmptyPVCSpec(name, url string) *datavolumev1alpha1.DataVolume {

	httpSource := datavolumev1alpha1.DataVolumeSource{
		HTTP: &datavolumev1alpha1.DataVolumeSourceHTTP{URL: url},
	}

	return newDataVolume(name, httpSource, nil)
}

func newDataVolumeWithMultipleSources(name string) *datavolumev1alpha1.DataVolume {
	source := datavolumev1alpha1.DataVolumeSource{
		HTTP: &datavolumev1alpha1.DataVolumeSourceHTTP{URL: "http://www.example.com"},
		S3:   &datavolumev1alpha1.DataVolumeSourceS3{URL: "http://s3.examples3.com"},
	}
	pvc := newPVCSpec(5, "M")

	return newDataVolume(name, source, pvc)
}

func newDataVolumeWithPVCSizeZero(name, url string) *datavolumev1alpha1.DataVolume {

	httpSource := datavolumev1alpha1.DataVolumeSource{
		HTTP: &datavolumev1alpha1.DataVolumeSourceHTTP{URL: url},
	}
	pvc := newPVCSpec(0, "M")

	return newDataVolume(name, httpSource, pvc)
}

func newDataVolume(name string, source datavolumev1alpha1.DataVolumeSource, pvc *corev1.PersistentVolumeClaimSpec) *datavolumev1alpha1.DataVolume {
	namespace := k8sv1.NamespaceDefault
	dv := &datavolumev1alpha1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			SelfLink:  fmt.Sprintf("/apis/%s/namespaces/%s/datavolumes/%s", datavolumev1alpha1.SchemeGroupVersion.String(), namespace, name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: datavolumev1alpha1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: datavolumev1alpha1.DataVolumeStatus{},
		Spec: datavolumev1alpha1.DataVolumeSpec{
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
		Resources: corev1.ResourceRequirements{
			Requests: requests,
		},
	}
	return pvc
}
