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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("HostClonePhase test", func() {
	log := logf.Log.WithName("host-clone-phase-test")

	type ResourceModifier struct {
		modifySourcePvc  func(pvcSpec *corev1.PersistentVolumeClaim)
		modifyDesiredPvc func(pvc *corev1.PersistentVolumeClaim)
	}

	creatHostClonePhase := func(modifier *ResourceModifier, objects ...runtime.Object) *HostClonePhase {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)

		objects = append(objects, cc.MakeEmptyCDICR())

		source := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "source",
			},
		}

		if modifier != nil && modifier.modifySourcePvc != nil {
			modifier.modifySourcePvc(source)
		}

		objects = append(objects, source)

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
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}

		if modifier != nil && modifier.modifyDesiredPvc != nil {
			modifier.modifyDesiredPvc(desired)
		}

		return &HostClonePhase{
			Owner:          owner,
			OwnershipLabel: "label",
			Namespace:      "ns",
			SourceName:     source.Name,
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
		p := creatHostClonePhase(nil)

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
		Expect(pvc.Labels[cc.LabelExcludeFromVeleroBackup]).To(Equal("true"))
		Expect(pvc.Annotations[cc.AnnImmediateBinding]).To(Equal(""))
		_, ok := pvc.Annotations[cc.AnnPriorityClassName]
		Expect(ok).To(BeFalse())
	})

	It("should create pvc with priorityclass", func() {
		p := creatHostClonePhase(nil)
		p.PriorityClassName = "priority"

		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).ToNot(BeZero())

		pvc := getDesiredClaim(p)
		Expect(pvc.Annotations[cc.AnnPriorityClassName]).To(Equal("priority"))
	})

	It("should get error if target pvc does not have size", func() {
		makePvcSizeMissing := func(pvc *corev1.PersistentVolumeClaim, _ corev1.PersistentVolumeMode) {
			pvc.Spec = corev1.PersistentVolumeClaimSpec{}
		}
		cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: common.DefaultGlobalOverhead,
		}

		p := creatHostClonePhase(&ResourceModifier{
			modifyDesiredPvc: func(pvc *corev1.PersistentVolumeClaim) {
				makePvcSizeMissing(pvc, corev1.PersistentVolumeFilesystem)
			},
		}, cdiConfig)

		_, err := p.Reconcile(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no target resource request specified"))
	})

	It("should adjust requested size for block to filesystem volume mode clone", func() {
		setPvcAttributes := func(pvcSpec *corev1.PersistentVolumeClaimSpec, volumeMode corev1.PersistentVolumeMode, storage string) {
			pvcSpec.VolumeMode = &volumeMode
			if pvcSpec.Resources.Requests == nil {
				pvcSpec.Resources.Requests = corev1.ResourceList{}
			}
			pvcSpec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(storage)
		}
		cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: common.DefaultGlobalOverhead,
		}

		p := creatHostClonePhase(&ResourceModifier{
			modifySourcePvc: func(pvc *corev1.PersistentVolumeClaim) {
				setPvcAttributes(&pvc.Spec, corev1.PersistentVolumeBlock, "8Gi")
			},
			modifyDesiredPvc: func(pvc *corev1.PersistentVolumeClaim) {
				setPvcAttributes(&pvc.Spec, corev1.PersistentVolumeFilesystem, "8Gi")
			},
		}, cdiConfig)

		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).ToNot(BeZero())

		pvc := getDesiredClaim(p)

		Expect(*pvc.Spec.VolumeMode).To(Equal(corev1.PersistentVolumeFilesystem))
		actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		originalRequested := resource.MustParse("8Gi")

		Expect(actualSize.Cmp(originalRequested)).To(BeNumerically(">", 0), "The actual should be greater than the requested", "actual", actualSize, "requested", originalRequested)
	})

	It("should inflate the target size for filesystem to filesystem volume mode clone", func() {
		setPvcAttributes := func(pvcSpec *corev1.PersistentVolumeClaimSpec, volumeMode corev1.PersistentVolumeMode, storage string) {
			pvcSpec.VolumeMode = &volumeMode
			if pvcSpec.Resources.Requests == nil {
				pvcSpec.Resources.Requests = corev1.ResourceList{}
			}
			pvcSpec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(storage)
		}
		cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
		cdiConfig.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global: common.DefaultGlobalOverhead,
		}

		p := creatHostClonePhase(&ResourceModifier{
			modifySourcePvc: func(pvc *corev1.PersistentVolumeClaim) {
				setPvcAttributes(&pvc.Spec, corev1.PersistentVolumeFilesystem, "8Gi")
			},
			modifyDesiredPvc: func(pvc *corev1.PersistentVolumeClaim) {
				setPvcAttributes(&pvc.Spec, corev1.PersistentVolumeFilesystem, "8Gi")
			},
		}, cdiConfig)

		result, err := p.Reconcile(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Requeue).To(BeFalse())
		Expect(result.RequeueAfter).ToNot(BeZero())

		pvc := getDesiredClaim(p)

		Expect(*pvc.Spec.VolumeMode).To(Equal(corev1.PersistentVolumeFilesystem))
		actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		originalRequested := resource.MustParse("8Gi")

		Expect(actualSize.Cmp(originalRequested)).To(BeNumerically(">", 0), "The actual should be greater than the requested", "actual", actualSize, "requested", originalRequested)
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
			p := creatHostClonePhase(nil, desired)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).ToNot(BeZero())
		})

		It("should wait for clone to succeed with preallocation", func() {
			desired := getCliam()
			desired.Annotations[cc.AnnPodPhase] = "Succeeded"
			p := creatHostClonePhase(nil, desired)
			p.Preallocation = true

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).ToNot(BeZero())
		})

		It("should succeed", func() {
			desired := getCliam()
			desired.Annotations[cc.AnnPodPhase] = "Succeeded"
			p := creatHostClonePhase(nil, desired)

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should succeed with preallocation", func() {
			desired := getCliam()
			desired.Annotations[cc.AnnPodPhase] = "Succeeded"
			desired.Annotations[cc.AnnPreallocationApplied] = "true"
			p := creatHostClonePhase(nil, desired)
			p.Preallocation = true

			result, err := p.Reconcile(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})
})
