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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("RebindPhase test", func() {
	log := logf.Log.WithName("rebind-phase-test")

	const (
		namespace  = "ns"
		sourceName = "source"
		targetName = "target"
		pvName     = "pv"
	)

	createRebindPhase := func(objects ...runtime.Object) *RebindPhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)

		objects = append(objects, cc.MakeEmptyCDICR())

		// Create a fake client to mock API calls.
		builder := fake.NewClientBuilder().
			WithScheme(s).
			WithRuntimeObjects(objects...)

		cl := builder.Build()

		rec := record.NewFakeRecorder(10)

		return &RebindPhase{
			SourceNamespace: namespace,
			SourceName:      sourceName,
			TargetNamespace: namespace,
			TargetName:      targetName,
			Client:          cl,
			Recorder:        rec,
			Log:             log,
		}
	}

	It("should error if target claim does not exist", func() {
		p := createRebindPhase()
		result, err := p.Reconcile(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("target claim does not exist"))
		Expect(result).To(BeNil())
	})

	Context("with target PVC", func() {
		createClaim := func(namespace, name string) *corev1.PersistentVolumeClaim {
			return &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            name,
					UID:             types.UID(name + "-uid"),
					ResourceVersion: name + "-version",
				},
			}
		}

		createTarget := func() *corev1.PersistentVolumeClaim {
			return createClaim(namespace, targetName)
		}

		getTargetClaim := func(p *RebindPhase) *corev1.PersistentVolumeClaim {
			pvc := &corev1.PersistentVolumeClaim{}
			err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(createTarget()), pvc)
			Expect(err).ToNot(HaveOccurred())
			return pvc
		}

		It("should succeed if target is bound", func() {
			target := createTarget()
			target.Spec.VolumeName = pvName

			p := createRebindPhase(target)
			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should error if source does not exist", func() {
			target := createTarget()

			p := createRebindPhase(target)
			result, err := p.Reconcile(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("source claim does not exist"))
			Expect(result).To(BeNil())
		})

		Context("with source and PV", func() {

			createSource := func() *corev1.PersistentVolumeClaim {
				return createClaim(namespace, sourceName)
			}

			createVolume := func() *corev1.PersistentVolume {
				source := createSource()
				return &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: pvName,
					},
					Spec: corev1.PersistentVolumeSpec{
						ClaimRef: &corev1.ObjectReference{
							Namespace:       source.Namespace,
							Name:            source.Name,
							UID:             source.UID,
							ResourceVersion: source.ResourceVersion,
						},
					},
				}
			}

			getVolume := func(p *RebindPhase) *corev1.PersistentVolume {
				pv := &corev1.PersistentVolume{}
				err := p.Client.Get(context.Background(), client.ObjectKeyFromObject(createVolume()), pv)
				Expect(err).ToNot(HaveOccurred())
				return pv
			}

			It("should rebind volume", func() {
				volume := createVolume()
				source := createSource()
				source.Spec.VolumeName = volume.Name
				p := createRebindPhase(createTarget(), source, volume)
				result, err := p.Reconcile(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())

				pv := getVolume(p)
				target := getTargetClaim(p)
				Expect(pv.Spec.ClaimRef.Namespace).To(Equal(target.Namespace))
				Expect(pv.Spec.ClaimRef.Name).To(Equal(target.Name))
				Expect(pv.Spec.ClaimRef.UID).To(Equal(target.UID))
				Expect(pv.Spec.ClaimRef.ResourceVersion).To(Equal(target.ResourceVersion))
			})

			It("should error if pv bound to something else", func() {
				volume := createVolume()
				volume.Spec.ClaimRef.Name = "foo"
				source := createSource()
				source.Spec.VolumeName = volume.Name
				p := createRebindPhase(createTarget(), source, volume)
				result, err := p.Reconcile(context.Background())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("PV pv bound to unexpected claim foo"))
				Expect(result).To(BeNil())
			})
		})
	})
})
