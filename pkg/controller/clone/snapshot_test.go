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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("SnapshotPhase test", func() {
	log := logf.Log.WithName("snapshot-phase-test")

	const (
		namespace  = "ns"
		targetName = "target"
	)

	var (
		sourceName = "source"
		snapClass  = "snapclass"
	)

	createSnapshotPhase := func(objects ...runtime.Object) *SnapshotPhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)
		_ = snapshotv1.AddToScheme(s)

		objects = append(objects, cc.MakeEmptyCDICR())

		// Create a fake client to mock API calls.
		builder := fake.NewClientBuilder().
			WithScheme(s).
			WithRuntimeObjects(objects...)

		cl := builder.Build()

		owner := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "owner",
				UID:       "uid",
			},
		}

		return &SnapshotPhase{
			Owner:               owner,
			SourceNamespace:     namespace,
			SourceName:          sourceName,
			TargetName:          targetName,
			VolumeSnapshotClass: snapClass,
			OwnershipLabel:      "label",
			Client:              cl,
			Log:                 log,
		}
	}

	It("should requeue if source does not exist", func() {
		p := createSnapshotPhase()
		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.RequeueAfter).ToNot(BeZero())
	})

	Context("with source", func() {
		sourceClaim := func() *corev1.PersistentVolumeClaim {
			return &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      sourceName,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			}
		}

		getSnapshot := func(p *SnapshotPhase) *snapshotv1.VolumeSnapshot {
			vs := &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      targetName,
				},
			}
			err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(vs), vs)
			Expect(err).ToNot(HaveOccurred())
			return vs
		}

		It("should requeue if source not ready", func() {
			sc := sourceClaim()
			sc.Status.Phase = corev1.ClaimPending
			p := createSnapshotPhase()
			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).ToNot(BeZero())
		})

		It("should create snapshot", func() {
			p := createSnapshotPhase(sourceClaim())
			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			snapshot := getSnapshot(p)
			Expect(*snapshot.Spec.Source.PersistentVolumeClaimName).To(Equal(sourceName))
			Expect(*snapshot.Spec.VolumeSnapshotClassName).To(Equal(snapClass))
		})

		Context("with snapshot", func() {
			createSnapshot := func() *snapshotv1.VolumeSnapshot {
				return &snapshotv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      targetName,
					},
					Spec: snapshotv1.VolumeSnapshotSpec{
						Source: snapshotv1.VolumeSnapshotSource{
							PersistentVolumeClaimName: &sourceName,
						},
						VolumeSnapshotClassName: &snapClass,
					},
					Status: &snapshotv1.VolumeSnapshotStatus{},
				}
			}

			It("should wait for snapshot to create", func() {
				p := createSnapshotPhase(createSnapshot())
				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
			})

			It("succeed if snapshot created", func() {
				t := metav1.Now()
				s := createSnapshot()
				s.Status.CreationTime = &t
				p := createSnapshotPhase(s)
				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})
})
