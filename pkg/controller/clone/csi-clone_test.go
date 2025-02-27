/*
Copyright 2023 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clone

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("CSIClonePhase test", func() {
	log := logf.Log.WithName("csi-clone-phase-test")

	const (
		namespace   = "ns"
		sourceName  = "source"
		desiredName = "desired"
	)

	var (
		storageClassName = "storageclass"
	)

	createCSIClonePhase := func(objects ...runtime.Object) *CSIClonePhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)

		objects = append(objects, cc.MakeEmptyCDICR())

		// Create a fake client to mock API calls.
		builder := fake.NewClientBuilder().
			WithScheme(s).
			WithRuntimeObjects(objects...)

		cl := builder.Build()

		rec := record.NewFakeRecorder(10)

		owner := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "owner",
				UID:       "uid",
			},
		}

		desired := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "desired",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
		}

		return &CSIClonePhase{
			Owner:          owner,
			OwnershipLabel: "label",
			Namespace:      "ns",
			SourceName:     "source",
			DesiredClaim:   desired,
			Client:         cl,
			Recorder:       rec,
			Log:            log,
		}
	}

	getDesiredClaim := func(p *CSIClonePhase) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{}
		err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(p.DesiredClaim), pvc)
		Expect(err).ToNot(HaveOccurred())
		return pvc
	}

	Context("with source PVC and storagelass", func() {
		sourceClaim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      sourceName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimBound,
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		}

		getStorageClass := func() *storagev1.StorageClass {
			return &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: storageClassName,
				},
			}
		}

		It("should create pvc", func() {
			p := createCSIClonePhase(sourceClaim, getStorageClass())

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			pvc := getDesiredClaim(p)
			Expect(pvc.Spec.DataSourceRef).ToNot(BeNil())
			Expect(pvc.Spec.DataSourceRef.Kind).To(Equal("PersistentVolumeClaim"))
			Expect(pvc.Spec.DataSourceRef.Namespace).To(BeNil())
			Expect(pvc.Spec.DataSourceRef.Name).To(Equal(sourceClaim.Name))
			Expect(pvc.Annotations[cc.AnnPopulatorKind]).To(Equal(cdiv1.VolumeCloneSourceRef))
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).
				To(Equal(sourceClaim.Status.Capacity[corev1.ResourceStorage]))
			Expect(pvc.Labels[p.OwnershipLabel]).To(Equal("uid"))
			Expect(pvc.Labels[cc.LabelExcludeFromVeleroBackup]).To(Equal("true"))
		})

		Context("with desired claim created", func() {
			getDesiredClaim := func() *corev1.PersistentVolumeClaim {
				return &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns",
						Name:      "desired",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &storageClassName,
					},
				}
			}

			It("should succeed (immediate bind)", func() {
				desired := getDesiredClaim()
				desired.Spec.VolumeName = "vol"
				desired.Status.Phase = corev1.ClaimBound
				p := createCSIClonePhase(sourceClaim, getStorageClass(), desired)

				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should succeed (WFFC)", func() {
				storageClass := getStorageClass()
				bm := storagev1.VolumeBindingWaitForFirstConsumer
				storageClass.VolumeBindingMode = &bm

				p := createCSIClonePhase(sourceClaim, storageClass, getDesiredClaim())

				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})
})
