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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

var _ = Describe("PrepClaimPhase test", func() {
	log := logf.Log.WithName("prep-claim-phase-test")

	const (
		namespace   = "ns"
		desiredName = "desired"
	)

	var (
		smallerThanDefault = resource.MustParse("10Gi")
		defaultRequestSize = resource.MustParse("20Gi")
	)

	createPrepClaimPhase := func(objects ...runtime.Object) *PrepClaimPhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)

		cdiConfig := &cdiv1.CDIConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "config",
			},
		}

		objects = append(objects, cc.MakeEmptyCDICR(), cdiConfig)

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
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: defaultRequestSize,
					},
				},
			},
		}

		return &PrepClaimPhase{
			Owner:        owner,
			DesiredClaim: desired,
			Image:        "image",
			PullPolicy:   corev1.PullIfNotPresent,
			InstallerLabels: map[string]string{
				"foo": "bar",
			},
			OwnershipLabel: "label",
			Client:         cl,
			Recorder:       rec,
			Log:            log,
		}
	}

	getDesiredClaim := func(p *PrepClaimPhase) *corev1.PersistentVolumeClaim {
		pvc := &corev1.PersistentVolumeClaim{}
		err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(p.DesiredClaim), pvc)
		Expect(err).ToNot(HaveOccurred())
		return pvc
	}

	getCreatedPod := func(p *PrepClaimPhase) *corev1.Pod {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "prep-" + string(p.Owner.GetUID()),
			},
		}
		err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(pod), pod)
		Expect(err).ToNot(HaveOccurred())
		return pod
	}

	It("should error if claim does not exist", func() {
		p := createPrepClaimPhase()
		result, err := p.Reconcile(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("claim ns/desired does not exist"))
		Expect(result).To(BeNil())
	})

	Context("with desired PVC", func() {
		getClaim := func() *corev1.PersistentVolumeClaim {
			return &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      desiredName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{},
					},
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase:    corev1.ClaimBound,
					Capacity: corev1.ResourceList{},
				},
			}
		}

		It("should error if sizes missing", func() {
			p := createPrepClaimPhase(getClaim())

			result, err := p.Reconcile(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("requested PVC sizes missing"))
			Expect(result).To(BeNil())
		})

		It("should do nothing if sizes match", func() {
			claim := getClaim()
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize
			claim.Status.Capacity[corev1.ResourceStorage] = defaultRequestSize

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should not return error if actual PVC is bound but status isnt' updated", func() {
			claim := getClaim()
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize
			delete(claim.Status.Capacity, corev1.ResourceStorage)
			claim.Status.Phase = corev1.ClaimPending
			claim.Spec.VolumeName = "test01"

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

		It("should return error if PVC is fully bound but doesn't have capacity", func() {
			claim := getClaim()
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize
			delete(claim.Status.Capacity, corev1.ResourceStorage)
			claim.Status.Phase = corev1.ClaimBound
			claim.Spec.VolumeName = "test01"

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("actual PVC size missing"))
			Expect(result).To(BeNil())
		})

		It("should update PVC requested size if necessary", func() {
			claim := getClaim()
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = smallerThanDefault
			claim.Status.Capacity[corev1.ResourceStorage] = smallerThanDefault

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			pvc := getDesiredClaim(p)
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(defaultRequestSize))
		})

		It("should create pod if desired is unbound", func() {
			claim := getClaim()
			cc.AddAnnotation(claim, cc.AnnSelectedNode, "node1")
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			pod := getCreatedPod(p)

			Expect(pod.Labels[p.OwnershipLabel]).To(Equal("uid"))
			Expect(pod.Spec.Containers[0].Image).To(Equal(p.Image))
			Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(p.PullPolicy))
			Expect(pod.Spec.NodeName).To(Equal("node1"))
		})

		It("should create pod if desired is bigger", func() {
			claim := getClaim()
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize
			claim.Status.Capacity[corev1.ResourceStorage] = smallerThanDefault

			p := createPrepClaimPhase(claim)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			pod := getCreatedPod(p)

			Expect(pod.Labels[p.OwnershipLabel]).To(Equal("uid"))
			Expect(pod.Spec.Containers[0].Image).To(Equal(p.Image))
			Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(p.PullPolicy))
			Expect(pod.Spec.NodeName).To(Equal(""))
		})

		Context("with prep pod created", func() {
			getPod := func() *corev1.Pod {
				return &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "prep-uid",
					},
				}
			}

			It("should delete pod if succeeded", func() {
				pod := getPod()
				pod.Status.Phase = corev1.PodSucceeded
				claim := getClaim()
				claim.Spec.Resources.Requests[corev1.ResourceStorage] = defaultRequestSize
				claim.Status.Capacity[corev1.ResourceStorage] = defaultRequestSize

				p := createPrepClaimPhase(claim, pod)

				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())

				err = p.Client.Get(context.Background(), client.ObjectKeyFromObject(pod), pod)
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})
