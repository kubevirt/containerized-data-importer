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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"

	snapclientfake "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned/fake"
	cdiclientfake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var (
	testNamespace  = "testNamespace"
	emptyNamespace = ""
)

var _ = Describe("Validating Webhook", func() {
	longName := "name-is-longer-than-253-" + strings.Repeat("0123456789", 23)

	Context("with DataVolume admission review", func() {
		It("should accept DataVolume with HTTP source on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept DataVolume with GS source on create", func() {
			dataVolume := newGCSDataVolume("testDV", "gs://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject DataVolume with GCS source on create", func() {
			dataVolume := newGCSDataVolume("testDV", "gcs://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
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
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept DataVolume with Registry source URL on create", func() {
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept DataVolume with Registry source ImageStream and node PullMethod on create", func() {
			imageStream := "istream"
			pullNode := cdiv1.RegistryPullNode
			registrySource := cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{ImageStream: &imageStream, PullMethod: &pullNode},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolume("testDV", registrySource, pvc)
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject DataVolume with Registry source ImageStream and pod PullMethod on create", func() {
			imageStream := "istream"
			registrySource := cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{ImageStream: &imageStream},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolume("testDV", registrySource, pvc)
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with no url or ImageStream", func() {
			registrySource := cdiv1.DataVolumeSource{}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolume("testDV", registrySource, pvc)
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with both url and ImageStream", func() {
			url := "docker://registry:5000/test"
			imageStream := "istream"
			registrySource := cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{URL: &url, ImageStream: &imageStream},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolume("testDV", registrySource, pvc)
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with non-kubevirt contentType", func() {
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with illegal source URL", func() {
			dataVolume := newRegistryDataVolume("testDV", "docker/::registry:5000/test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with illegal transport in source URL", func() {
			dataVolume := newRegistryDataVolume("testDV", "joker://registry:5000/test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with Registry source on create with illegal importMethod", func() {
			pullMethod := cdiv1.RegistryPullMethod("nosuch")
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept DataVolume with Registry source on create with supported importMethod", func() {
			pullMethod := cdiv1.RegistryPullNode
			dataVolume := newRegistryDataVolume("testDV", "docker://registry:5000/test")
			dataVolume.Spec.Source.Registry.PullMethod = &pullMethod
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
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
			Expect(resp.Allowed).To(BeTrue())
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
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept DataVolume with PVC source on create if PVC does not exist", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject invalid DataVolume source PVC namespace on create", func() {
			dataVolume := newPVCDataVolume("testDV", "", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject invalid DataVolume source PVC name on create", func() {
			dataVolume := newPVCDataVolume("testDV", "testNamespace", "")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with name length greater than 253 characters", func() {
			dataVolume := newHTTPDataVolume(longName, "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		DescribeTable("should", func(scName *string, expected bool) {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "http://www.example.com"},
			}
			storage := &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("500Mi"),
					},
				},
				StorageClassName: scName,
			}
			dv := newDataVolumeWithStorageSpec("testDV", httpSource, nil, storage)
			resp := validateDataVolumeCreate(dv)
			Expect(resp.Allowed).To(Equal(expected))
		},
			Entry("should accept DataVolume with Storage spec blank StorageClassName", pointer.String(""), true),
			Entry("should reject DataVolume with Storage spec too long StorageClassName", pointer.String(longName), false),
			Entry("should accept DataVolume with Storage spec empty StorageClassName", nil, true),
		)

		DescribeTable("should", func(scName *string, expected bool) {
			dv := newHTTPDataVolume("testDV", "http://www.example.com")
			dv.Spec.PVC.StorageClassName = scName
			resp := validateDataVolumeCreate(dv)
			Expect(resp.Allowed).To(Equal(expected))
		},
			Entry("should accept DataVolume with PVC spec blank StorageClassName", pointer.String(""), true),
			Entry("should reject DataVolume with PVC spec too long StorageClassName", pointer.String(longName), false),
			Entry("should accept DataVolume with PVC spec empty StorageClassName", nil, true),
		)

		It("should reject DataVolume source with invalid URL on create", func() {
			dataVolume := newHTTPDataVolume("testDV", "invalidurl")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with multiple sources on create", func() {
			dataVolume := newDataVolumeWithMultipleSources("testDV")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with empty PVC create", func() {
			dataVolume := newDataVolumeWithEmptyPVCSpec("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with PVC size 0", func() {
			dataVolume := newDataVolumeWithPVCSizeZero("testDV", "http://www.example.com")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept DataVolume with Blank source and no content type", func() {
			dataVolume := newBlankDataVolume("blank")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())

		})

		It("should accept DataVolume with Blank source and kubevirt contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeKubeVirt
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())

		})

		It("should reject DataVolume with Blank source and archive contentType", func() {
			dataVolume := newBlankDataVolume("blank")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())

		})

		It("should reject DataVolume with invalid contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = "invalid"
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept DataVolume with archive contentType", func() {
			dataVolume := newHTTPDataVolume("testDV", "http://www.example.com")
			dataVolume.Spec.ContentType = cdiv1.DataVolumeArchive
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
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
			Expect(resp.Allowed).To(BeFalse())
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
			Expect(resp.Allowed).To(BeTrue())
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
			Expect(resp.Allowed).To(BeFalse())
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
			Expect(resp.Allowed).To(BeTrue())
		})

		DescribeTable("should validate clones", func(storageSpec *cdiv1.StorageSpec, pvcSpec *corev1.PersistentVolumeClaimSpec, expected bool) {
			dv, pvc := newDataVolumeClone(storageSpec, pvcSpec)
			resp := validateDataVolumeCreate(dv, pvc)
			Expect(resp.Allowed).To(Equal(expected))
		},
			Entry("should reject clones with both nil Storage and PVC spec", nil, nil, false),
			Entry("should reject clones with both Storage and PVC spec", &cdiv1.StorageSpec{}, &corev1.PersistentVolumeClaimSpec{}, false),
			Entry("should accept empty Storage spec when cloning PVC", &cdiv1.StorageSpec{}, nil, true),
			Entry("should accept blank Resources when cloning using Storage API", &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{},
			}, nil, true),
			Entry("should accept empty Requests when cloning using Storage API", &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{
					Requests: make(map[corev1.ResourceName]resource.Quantity),
				},
			}, nil, true),
			Entry("should reject empty Requests when cloning using PVC API", nil, &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: make(map[corev1.ResourceName]resource.Quantity),
				},
			}, false),
		)

		It("should reject empty Requests when using Storage API with DataVolumeSource but without DataVolumeSourcePVC", func() {
			httpSource := &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "http://www.example.com"},
			}
			requests := make(map[corev1.ResourceName]resource.Quantity)
			storage := &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{
					Requests: requests,
				},
			}
			dv := newDataVolumeWithStorageSpec("testDV", httpSource, nil, storage)
			resp := validateDataVolumeCreate(dv)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should allow empty Requests when using Storage API with DataVolumeSourceRef", func() {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testPVC",
					Namespace: testNamespace,
				},
				Spec: *newPVCSpec(pvcSizeDefault),
			}
			dataSource := &cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testDs",
					Namespace: testNamespace,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						PVC: &cdiv1.DataVolumeSourcePVC{
							Name:      pvc.Name,
							Namespace: testNamespace,
						},
					},
				},
			}
			storage := &cdiv1.StorageSpec{}
			sourceRef := &cdiv1.DataVolumeSourceRef{
				Kind:      cdiv1.DataVolumeDataSource,
				Namespace: &testNamespace,
				Name:      dataSource.Name,
			}
			dv := newDataVolumeWithStorageSpec("testDV", nil, sourceRef, storage)
			resp := validateDataVolumeCreateEx(dv, []runtime.Object{pvc}, []runtime.Object{dataSource}, nil)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject snapshot clone when input size is lower than recommended restore size", func() {
			size := resource.MustParse("1G")
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testsnap",
					Namespace: testNamespace,
				},
				Status: &snapshotv1.VolumeSnapshotStatus{
					RestoreSize: &size,
				},
			}
			storage := &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("500Mi"),
					},
				},
			}
			snapSource := &cdiv1.DataVolumeSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Namespace: snapshot.Namespace,
					Name:      snapshot.Name,
				},
			}
			dv := newDataVolumeWithStorageSpec("testDV", snapSource, nil, storage)
			resp := validateDataVolumeCreateEx(dv, nil, nil, []runtime.Object{snapshot})
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject snapshot clone when no input size/recommended restore size", func() {
			snapshot := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testsnap",
					Namespace: testNamespace,
				},
				Status: &snapshotv1.VolumeSnapshotStatus{},
			}
			storage := &cdiv1.StorageSpec{}
			snapSource := &cdiv1.DataVolumeSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Namespace: snapshot.Namespace,
					Name:      snapshot.Name,
				},
			}
			dv := newDataVolumeWithStorageSpec("testDV", snapSource, nil, storage)
			resp := validateDataVolumeCreateEx(dv, nil, nil, []runtime.Object{snapshot})
			Expect(resp.Allowed).To(BeFalse())
		})

		DescribeTable("should", func(oldFinalCheckpoint bool, oldCheckpoints []string, newFinalCheckpoint bool, newCheckpoints []string, modifyDV func(*cdiv1.DataVolume), expectedSuccess bool, sourceFunc func() *cdiv1.DataVolumeSource) {
			oldDV := newMultistageDataVolume("multi-stage", oldFinalCheckpoint, oldCheckpoints, sourceFunc)
			oldBytes, _ := json.Marshal(&oldDV)

			newDV := newMultistageDataVolume("multi-stage", newFinalCheckpoint, newCheckpoints, sourceFunc)
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
			Entry("accept a spec change on multi-stage VDDK import fields", false, []string{"stage-1"}, true, []string{"stage-1", "stage-2"}, nil, true, vddkSource),

			Entry("reject a spec change on un-approved fields of a multi-stage VDDK import", false, []string{"stage-1"}, true, []string{"stage-1", "stage-2"}, func(newDV *cdiv1.DataVolume) { newDV.Spec.Source.VDDK.URL = "testing123" }, false, vddkSource),

			Entry("accept identical multi-stage VDDK import field changes", false, []string{"stage-1"}, false, []string{"stage-1"}, nil, true, vddkSource),

			Entry("reject a spec change on un-approved fields, even with identical non-empty multi-stage fields", false, []string{"stage-1"}, false, []string{"stage-1"}, func(newDV *cdiv1.DataVolume) { newDV.Spec.Source.VDDK.URL = "tesing123" }, false, vddkSource),

			Entry("accept a spec change on multi-stage ImageIO import fields", false, []string{"snapshot-123"}, true, []string{"snapshot-123", "snapshot-234"}, nil, true, imageIOSource),

			Entry("reject a spec change on source type that does not support multi-stage import", false, []string{}, true, []string{}, nil, false, blankSource),
		)

		It("should accept DataVolume without source if dataSource is correctly populated", func() {
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", nil, nil, pvc)
			dataVolume.Spec.PVC.DataSource = &corev1.TypedLocalObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept DataVolume without source if dataSourceRef is correctly populated", func() {
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", nil, nil, pvc)
			dataVolume.Spec.PVC.DataSourceRef = &corev1.TypedObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject DataVolume with populated source and dataSource", func() {
			blankSource := cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", &blankSource, nil, pvc)
			dataVolume.Spec.PVC.DataSource = &corev1.TypedLocalObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with populated source and dataSourceRef", func() {
			blankSource := cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			}
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", &blankSource, nil, pvc)
			dataVolume.Spec.PVC.DataSourceRef = &corev1.TypedObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with badly populated dataSource", func() {
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", nil, nil, pvc)
			// APIGroup can only be empty when kind is PersistentVolumeClaim
			dataVolume.Spec.PVC.DataSource = &corev1.TypedLocalObjectReference{
				Kind: "VolumeSnapshot",
				Name: dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with if dataSource and dataSourceRef are different", func() {
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", nil, nil, pvc)
			dataVolume.Spec.PVC.DataSource = &corev1.TypedLocalObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name,
			}
			dataVolume.Spec.PVC.DataSourceRef = &corev1.TypedObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: dataVolume.Name + "-test",
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with cross namespace dataSourceRef", func() {
			namespace := "foo"
			pvc := newPVCSpec(pvcSizeDefault)
			dataVolume := newDataVolumeWithSourceRef("test-dv", nil, nil, pvc)
			dataVolume.Spec.PVC.DataSourceRef = &corev1.TypedObjectReference{
				Namespace: &namespace,
				Kind:      "PersistentVolumeClaim",
				Name:      dataVolume.Name,
			}

			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})
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
			resp := validateDataVolumeCreateEx(dataVolume, []runtime.Object{pvc}, []runtime.Object{dataSource}, nil)
			Expect(resp.Allowed).To(BeTrue())
		},
			Entry("accept DataVolume with PVC and sourceRef on create", &testNamespace),
			Entry("accept DataVolume with PVC and sourceRef nil namespace on create", nil),
			Entry("accept DataVolume with PVC and sourceRef missing namespace on create", &emptyNamespace),
		)

		It("should reject DataVolume with SourceRef on create if DataSource does not exist", func() {
			ns := "testNamespace"
			dataVolume := newDataSourceDataVolume("testDV", &ns, "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with SourceRef on create if DataSource exists but its PVC field is not populated", func() {
			dataVolume := newDataSourceDataVolume("testDV", &testNamespace, "test")
			dataSource := &cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.SourceRef.Name,
					Namespace: testNamespace,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						PVC: nil,
					},
				},
			}
			resp := validateDataVolumeCreateEx(dataVolume, nil, []runtime.Object{dataSource}, nil)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should accept DataVolume with SourceRef on create if DataSource exists but PVC does not exist", func() {
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
			resp := validateDataVolumeCreateEx(dataVolume, nil, []runtime.Object{dataSource}, nil)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should accept DataVolume with SourceRef on create if DataSource exists but snapshot does not exist", func() {
			dataVolume := newDataSourceDataVolume("testDV", &testNamespace, "test")
			dataSource := &cdiv1.DataSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataVolume.Spec.SourceRef.Name,
					Namespace: testNamespace,
				},
				Spec: cdiv1.DataSourceSpec{
					Source: cdiv1.DataSourceSource{
						Snapshot: &cdiv1.DataVolumeSourceSnapshot{
							Name:      "testNonExistentSnap",
							Namespace: testNamespace,
						},
					},
				},
			}
			resp := validateDataVolumeCreateEx(dataVolume, nil, []runtime.Object{dataSource}, nil)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should reject DataVolume with empty SourceRef name on create", func() {
			dataVolume := newDataSourceDataVolume("testDV", &testNamespace, "")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with both source and sourceRef on create", func() {
			dataVolume := newDataVolumeWithBothSourceAndSourceRef("testDV", "testNamespace", "test")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should reject DataVolume with no source or sourceRef on create", func() {
			dataVolume := newDataVolumeWithNoSourceOrSourceRef("testDV")
			resp := validateDataVolumeCreate(dataVolume)
			Expect(resp.Allowed).To(BeFalse())
		})
	})
})

func vddkSource() *cdiv1.DataVolumeSource {
	return &cdiv1.DataVolumeSource{
		VDDK: &cdiv1.DataVolumeSourceVDDK{
			BackingFile: "disk.img",
			URL:         "http://example.com/data",
			UUID:        "12345",
			Thumbprint:  "aa:bb:cc",
			SecretRef:   "secret",
		},
	}
}

func imageIOSource() *cdiv1.DataVolumeSource {
	return &cdiv1.DataVolumeSource{
		Imageio: &cdiv1.DataVolumeSourceImageIO{
			URL:           "http://example.com/data",
			DiskID:        "disk-123",
			SecretRef:     "secret",
			CertConfigMap: "certs",
		},
	}
}

func blankSource() *cdiv1.DataVolumeSource {
	return &cdiv1.DataVolumeSource{
		Blank: &cdiv1.DataVolumeBlankImage{},
	}
}

func newMultistageDataVolume(name string, final bool, checkpoints []string, sourceFunc func() *cdiv1.DataVolumeSource) *cdiv1.DataVolume {
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
			Source:          sourceFunc(),
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

func newGCSDataVolume(name, url string) *cdiv1.DataVolume {
	gcsSource := cdiv1.DataVolumeSource{
		GCS: &cdiv1.DataVolumeSourceGCS{URL: url},
	}
	pvc := newPVCSpec(pvcSizeDefault)
	return newDataVolume(name, gcsSource, pvc)
}

func newRegistryDataVolume(name, url string) *cdiv1.DataVolume {
	registrySource := cdiv1.DataVolumeSource{
		Registry: &cdiv1.DataVolumeSourceRegistry{URL: &url},
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

func newDataVolumeWithStorageSpec(name string, source *cdiv1.DataVolumeSource, sourceRef *cdiv1.DataVolumeSourceRef, storage *cdiv1.StorageSpec) *cdiv1.DataVolume {
	namespace := k8sv1.NamespaceDefault
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdiv1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: cdiv1.DataVolumeStatus{},
		Spec: cdiv1.DataVolumeSpec{
			Source:    source,
			SourceRef: sourceRef,
			Storage:   storage,
		},
	}
	return dv
}

func newDataVolumeWithoutStorageAndPVC(name string, source *cdiv1.DataVolumeSource, sourceRef *cdiv1.DataVolumeSourceRef) *cdiv1.DataVolume {
	namespace := k8sv1.NamespaceDefault
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: cdiv1.SchemeGroupVersion.String(),
			Kind:       "DataVolume",
		},
		Status: cdiv1.DataVolumeStatus{},
		Spec: cdiv1.DataVolumeSpec{
			Source:    source,
			SourceRef: sourceRef,
		},
	}
	return dv
}

// Returns both the DV clone and the original PVC
func newDataVolumeClone(storageSpec *cdiv1.StorageSpec, pvcSpec *corev1.PersistentVolumeClaimSpec) (*cdiv1.DataVolume, *corev1.PersistentVolumeClaim) {
	var dv *cdiv1.DataVolume
	pvcSource := &cdiv1.DataVolumeSource{
		PVC: &cdiv1.DataVolumeSourcePVC{
			Namespace: "testNamespace",
			Name:      "test",
		},
	}

	if storageSpec != nil && pvcSpec != nil {
		dv = newDataVolumeWithoutStorageAndPVC("testDV", pvcSource, nil)
		dv.Spec.Storage = storageSpec
		dv.Spec.PVC = pvcSpec
	} else if storageSpec != nil {
		dv = newDataVolumeWithStorageSpec("testDV", pvcSource, nil, storageSpec)
	} else if pvcSpec != nil {
		dv = newDataVolumeWithSourceRef("testDV", pvcSource, nil, pvcSpec)
	} else {
		dv = newDataVolumeWithoutStorageAndPVC("testDV", pvcSource, nil)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dv.Spec.Source.PVC.Name,
			Namespace: dv.Spec.Source.PVC.Namespace,
		},
		Spec: *newPVCSpec(pvcSizeDefault),
	}

	return dv, pvc
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
	return validateDataVolumeCreateEx(dv, objects, nil, nil)
}

func validateDataVolumeCreateEx(dv *cdiv1.DataVolume, k8sObjects, cdiObjects, snapObjects []runtime.Object) *admissionv1.AdmissionResponse {
	client := fakeclient.NewSimpleClientset(k8sObjects...)
	cdiClient := cdiclientfake.NewSimpleClientset(cdiObjects...)
	snapClient := snapclientfake.NewSimpleClientset(snapObjects...)
	wh := NewDataVolumeValidatingWebhook(client, cdiClient, snapClient)

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
	snapClient := snapclientfake.NewSimpleClientset()
	wh := NewDataVolumeValidatingWebhook(client, cdiClient, snapClient)
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
