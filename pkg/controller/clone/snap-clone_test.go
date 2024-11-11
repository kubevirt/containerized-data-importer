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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("SnapshotClonePhase test", func() {
	log := logf.Log.WithName("snapshot-clone-phase-test")

	const (
		namespace   = "ns"
		sourceName  = "source"
		desiredName = "desired"
	)

	var (
		storageClassName = "storageclass"
	)

	createSnapshotClonePhase := func(objects ...runtime.Object) *SnapshotClonePhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)
		_ = snapshotv1.AddToScheme(s)

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
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
		}

		return &SnapshotClonePhase{
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

	assertNotFound := func(err error) {
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())
	}

	getDesiredClaim := func(p *SnapshotClonePhase) (*corev1.PersistentVolumeClaim, error) {
		pvc := &corev1.PersistentVolumeClaim{}
		err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(p.DesiredClaim), pvc)
		return pvc, err
	}

	It("should error if snapshot does not exist", func() {
		p := createSnapshotClonePhase()
		result, err := p.Reconcile(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("source snapshot does not exist"))
		Expect(result).To(BeNil())
	})

	Context("with source snapshot and storageclass", func() {
		restoreSize := resource.MustParse("10Gi")
		getSnapshot := func() *snapshotv1.VolumeSnapshot {
			return &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      sourceName,
				},
				Status: &snapshotv1.VolumeSnapshotStatus{
					RestoreSize: &restoreSize,
				},
			}
		}

		getStorageClass := func() *storagev1.StorageClass {
			return &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: storageClassName,
				},
			}
		}

		It("should do nothing if snapshot not ready", func() {
			snapshot := getSnapshot()
			snapshot.Status.ReadyToUse = pointer.Bool(false)

			p := createSnapshotClonePhase(snapshot)
			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(BeZero())

			_, err = getDesiredClaim(p)
			assertNotFound(err)
		})

		It("should should create the claim when ready", func() {
			snapshot := getSnapshot()
			snapshot.Status.ReadyToUse = pointer.Bool(true)

			p := createSnapshotClonePhase(snapshot, getStorageClass())
			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(BeZero())

			pvc, err := getDesiredClaim(p)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Spec.DataSourceRef).ToNot(BeNil())
			Expect(pvc.Spec.DataSourceRef.Kind).To(Equal("VolumeSnapshot"))
			Expect(pvc.Spec.DataSourceRef.Namespace).To(BeNil())
			Expect(pvc.Spec.DataSourceRef.Name).To(Equal(sourceName))
			Expect(pvc.Annotations[cc.AnnPopulatorKind]).To(Equal(cdiv1.VolumeCloneSourceRef))
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(restoreSize))
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
				p := createSnapshotClonePhase(getStorageClass(), desired)

				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should succeed (WFFC)", func() {
				storageClass := getStorageClass()
				bm := storagev1.VolumeBindingWaitForFirstConsumer
				storageClass.VolumeBindingMode = &bm

				p := createSnapshotClonePhase(storageClass, getDesiredClaim())

				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})
})
