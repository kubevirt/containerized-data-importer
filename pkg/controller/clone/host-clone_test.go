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

	corev1 "k8s.io/api/core/v1"
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

var _ = Describe("HostClonePhase test", func() {
	log := logf.Log.WithName("host-clone-phase-test")

	creatHostClonePhase := func(objects ...runtime.Object) *HostClonePhase {
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
		}

		return &HostClonePhase{
			Owner:          owner,
			OwnershipLabel: "label",
			Namespace:      "ns",
			SourceName:     "source",
			DesiredClaim:   desired,
			ImmediateBind:  true,
			Preallocation:  false,
			Client:         cl,
			Recorder:       rec,
			Log:            log,
		}
	}

	getDesiredClaim := func(p *HostClonePhase) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{}
		err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(p.DesiredClaim), pvc)
		Expect(err).ToNot(HaveOccurred())
		return pvc
	}

	It("should create pvc", func() {
		p := creatHostClonePhase()

		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).ToNot(BeZero())

		pvc := getDesiredClaim(p)
		Expect(pvc.Spec.DataSourceRef).To(BeNil())
		Expect(pvc.Annotations[cc.AnnPreallocationRequested]).To(Equal("false"))
		Expect(pvc.Annotations[cc.AnnOwnerUID]).To(Equal(string(p.Owner.GetUID())))
		Expect(pvc.Annotations[cc.AnnPodRestarts]).To(Equal("0"))
		Expect(pvc.Annotations[cc.AnnCloneRequest]).To(Equal("ns/source"))
		Expect(pvc.Annotations[cc.AnnPopulatorKind]).To(Equal(cdiv1.VolumeCloneSourceRef))
		Expect(pvc.Labels[p.OwnershipLabel]).To(Equal("uid"))
		Expect(pvc.Annotations[cc.AnnImmediateBinding]).To(Equal(""))
		_, ok := pvc.Annotations[cc.AnnPriorityClassName]
		Expect(ok).To(BeFalse())
	})

	It("should create pvc with priorityclass", func() {
		p := creatHostClonePhase()
		p.PriorityClassName = "priority"

		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).ToNot(BeZero())

		pvc := getDesiredClaim(p)
		Expect(pvc.Annotations[cc.AnnPriorityClassName]).To(Equal("priority"))
	})

	Context("with desired claim created", func() {
		getCliam := func() *corev1.PersistentVolumeClaim {
			return &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "ns",
					Name:        "desired",
					Annotations: map[string]string{},
				},
			}
		}

		It("should wait for clone to succeed", func() {
			desired := getCliam()
			desired.Annotations[cc.AnnPodPhase] = "Running"
			p := creatHostClonePhase(desired)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).ToNot(BeZero())
		})

		It("should succeed", func() {
			desired := getCliam()
			desired.Annotations[cc.AnnPodPhase] = "Succeeded"
			p := creatHostClonePhase(desired)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})
})
