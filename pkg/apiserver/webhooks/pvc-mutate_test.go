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
 * Copyright 2024 Red Hat, Inc.
 *
 */

package webhooks

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/appscode/jsonpatch"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("Mutating PVC Webhook", func() {
	Context("with PVC admission review", func() {
		It("should reject review without request", func() {
			ar := &admissionv1.AdmissionReview{}
			resp := mutatePvc(ar)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).Should(Equal("AdmissionReview.Request is nil"))
		})

		const testStorageClassName = "sc_test"
		const virtDefaultStorageClassName = "virt_default"

		var (
			storageClass = storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: testStorageClassName,
				},
			}
			defaultStorageClass = storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: testStorageClassName,
					Annotations: map[string]string{
						"storageclass.kubernetes.io/is-default-class": "true",
					},
				},
			}

			virtDefaultStorageClass = storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: virtDefaultStorageClassName,
					Annotations: map[string]string{
						"storageclass.kubevirt.io/is-default-virt-class": "true",
					},
				},
			}

			storageProfile = cdiv1.StorageProfile{
				ObjectMeta: metav1.ObjectMeta{Name: testStorageClassName},
				Status: cdiv1.StorageProfileStatus{
					ClaimPropertySets: []cdiv1.ClaimPropertySet{{
						VolumeMode:  ptr.To[corev1.PersistentVolumeMode](corev1.PersistentVolumeBlock),
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
					}},
				},
			}

			virtDefaultStorageProfile = cdiv1.StorageProfile{
				ObjectMeta: metav1.ObjectMeta{Name: virtDefaultStorageClassName},
				Status: cdiv1.StorageProfileStatus{
					ClaimPropertySets: []cdiv1.ClaimPropertySet{{
						VolumeMode:  ptr.To[corev1.PersistentVolumeMode](corev1.PersistentVolumeBlock),
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
					}},
				},
			}

			partialStorageProfile = cdiv1.StorageProfile{
				ObjectMeta: metav1.ObjectMeta{Name: testStorageClassName},
				Status: cdiv1.StorageProfileStatus{
					ClaimPropertySets: []cdiv1.ClaimPropertySet{{
						VolumeMode: ptr.To[corev1.PersistentVolumeMode](corev1.PersistentVolumeBlock),
					}},
				},
			}
		)

		DescribeTable("should", func(allowed bool, message, scName string, objs ...client.Object) {
			pvc := newPvc()
			dvBytes, _ := json.Marshal(&pvc)

			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Version:  corev1.SchemeGroupVersion.Version,
						Resource: "persistentvolumeclaims",
					},
					Object: runtime.RawExtension{
						Raw: dvBytes,
					},
				},
			}

			resp := mutatePvc(ar, objs...)

			if !allowed {
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result).ToNot(BeNil())
				Expect(resp.Result.Message).To(Equal(message))
				return
			}

			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Result).To(BeNil())
			Expect(resp.Patch).ToNot(BeNil())

			var patchObjs []jsonpatch.Operation
			err := json.Unmarshal(resp.Patch, &patchObjs)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchObjs).Should(HaveLen(3))

			sort.Slice(patchObjs, func(i, j int) bool {
				return patchObjs[i].Path < patchObjs[j].Path
			})

			Expect(patchObjs[0].Operation).Should(Equal("add"))
			Expect(patchObjs[0].Path).Should(Equal("/spec/accessModes"))
			accessModes, ok := patchObjs[0].Value.([]interface{})
			Expect(ok).Should(BeTrue())
			Expect(accessModes).Should(HaveLen(1))
			Expect(accessModes[0]).Should(Equal("ReadWriteMany"))

			Expect(patchObjs[1].Operation).Should(Equal("add"))
			Expect(patchObjs[1].Path).Should(Equal("/spec/storageClassName"))
			Expect(patchObjs[1].Value).Should(Equal(scName))

			Expect(patchObjs[2].Operation).Should(Equal("add"))
			Expect(patchObjs[2].Path).Should(Equal("/spec/volumeMode"))
			Expect(patchObjs[2].Value).Should(Equal("Block"))
		},
			Entry("fail with no storage classes", false,
				"PVC spec is missing accessMode and no storageClass to choose profile", ""),
			Entry("fail with no default storage classes", false,
				"PVC spec is missing accessMode and no storageClass to choose profile", "", &storageClass, &storageProfile),
			Entry("fail with default storage classes but with partial storage profile", false,
				fmt.Sprintf("no accessMode specified in StorageProfile %s", testStorageClassName), testStorageClassName, &defaultStorageClass, &partialStorageProfile),
			Entry("succeed with default storage classes and complete storage profile", true, "", testStorageClassName, &defaultStorageClass, &storageProfile),
			Entry("choose virt default storage class over default", true, "", virtDefaultStorageClassName, &defaultStorageClass, &virtDefaultStorageClass, &storageProfile, &virtDefaultStorageProfile),
		)

		DescribeTable("should handle minimum size labels for", func(kind, source string, optIn bool) {
			pvc := newPvc()
			originalSize := resource.MustParse("1Gi")
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = originalSize
			pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
				APIGroup: ptr.To(cdiv1.SchemeGroupVersion.Group),
				Kind:     kind,
				Name:     "source",
			}
			if source == "datavolume" {
				cc.AddAnnotation(pvc, cc.AnnCreatedForDataVolume, "test-dv-uid")
			}
			if !optIn {
				delete(pvc.Labels, common.PvcApplyStorageProfileLabel)
			}

			raw, err := json.Marshal(pvc)
			Expect(err).ToNot(HaveOccurred())
			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    corev1.SchemeGroupVersion.Group,
						Version:  corev1.SchemeGroupVersion.Version,
						Resource: "persistentvolumeclaims",
					},
					Object: runtime.RawExtension{Raw: raw},
				},
			}

			profile := storageProfile.DeepCopy()
			profile.Annotations = map[string]string{cc.AnnMinimumSupportedPVCSize: "4Gi"}
			resp := mutatePvc(ar, defaultStorageClass.DeepCopy(), profile)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Result).To(BeNil())
			if !optIn {
				Expect(resp.Patch).To(BeEmpty())
				return
			}

			var patches []jsonpatch.Operation
			Expect(json.Unmarshal(resp.Patch, &patches)).To(Succeed())
			sizePath := "/spec/resources/requests/storage"
			originalPath := "/metadata/labels/" + strings.ReplaceAll(cc.LabelOriginalRequestedSizeBytes, "/", "~1")
			sourcePath := "/metadata/labels/" + strings.ReplaceAll(cc.LabelMinSupportedSizeSource, "/", "~1")

			if source == "" {
				// Clone PVCs with a requested size skip minimum-size inflation.
				for _, path := range []string{sizePath, originalPath, sourcePath} {
					Expect(patches).ToNot(ContainElement(HaveField("Path", path)))
				}
				return
			}

			Expect(patches).To(ContainElements(
				jsonpatch.Operation{
					Operation: "replace",
					Path:      sizePath,
					Value:     "4Gi",
				},
				jsonpatch.Operation{
					Operation: "add",
					Path:      originalPath,
					Value:     fmt.Sprint(originalSize.Value()),
				},
				jsonpatch.Operation{
					Operation: "add",
					Path:      sourcePath,
					Value:     source,
				},
			))
		},
			Entry("direct import PVC without a DV", cdiv1.VolumeImportSourceRef, "external", true),
			Entry("direct upload PVC without a DV", cdiv1.VolumeUploadSourceRef, "external", true),
			Entry("DV-created import PVC", cdiv1.VolumeImportSourceRef, "datavolume", true),
			Entry("DV-created upload PVC", cdiv1.VolumeUploadSourceRef, "datavolume", true),
			Entry("clone PVC with a requested size", cdiv1.VolumeCloneSourceRef, "", true),
			Entry("import PVC without opt-in", cdiv1.VolumeImportSourceRef, "", false),
			Entry("upload PVC without opt-in", cdiv1.VolumeUploadSourceRef, "", false),
		)
	})
})

func newPvc() *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "testPvc",
			Labels: map[string]string{common.PvcApplyStorageProfileLabel: "true"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1G"),
				},
			},
		},
	}

	return pvc
}

func mutatePvc(ar *admissionv1.AdmissionReview, objs ...client.Object) *admissionv1.AdmissionResponse {
	_ = storagev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	wh := NewPvcMutatingWebhook(fakeClient)

	return serve(ar, wh)
}
