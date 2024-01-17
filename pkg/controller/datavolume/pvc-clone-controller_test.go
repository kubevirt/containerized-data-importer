/*
Copyright 2022 The CDI Authors.

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

package datavolume

import (
	"context"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/clone"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

var (
	dvCloneLog = logf.Log.WithName("datavolume-clone-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *PvcCloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	var _ = Describe("PVC clone controller populator integration", func() {
		Context("with CSI provisioner", func() {
			const (
				pluginName = "csi-plugin"
			)

			var (
				scName       = "testSC"
				storageClass *storagev1.StorageClass
				csiDriver    = &storagev1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{
						Name: pluginName,
					},
				}
			)

			BeforeEach(func() {
				storageClass = CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, pluginName)
			})

			It("should add extended token", func() {
				dv := newCloneDataVolumeWithPVCNS("test-dv", "source-ns")
				srcPvc := CreatePvcInStorageClass("test", "source-ns", &scName, nil, nil, corev1.ClaimBound)
				reconciler = createCloneReconciler(storageClass, csiDriver, dv, srcPvc)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Annotations).To(HaveKey(AnnExtendedCloneToken))
			})

			It("should add finalizer for cross namespace clone", func() {
				dv := newCloneDataVolumeWithPVCNS("test-dv", "source-ns")
				dv.Annotations[AnnExtendedCloneToken] = "test-token"
				srcPvc := CreatePvcInStorageClass("test", "source-ns", &scName, nil, nil, corev1.ClaimBound)
				reconciler = createCloneReconciler(storageClass, csiDriver, dv, srcPvc)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Finalizers).To(ContainElement(crossNamespaceFinalizer))
				Expect(dv.Status.Phase).To(Equal(cdiv1.CloneScheduled))
			})

			DescribeTable("should create PVC and VolumeCloneSource CR", func(sourceNamespace string) {
				dv := newCloneDataVolume("test-dv")
				dv.Annotations[AnnExtendedCloneToken] = "foobar"
				dv.Spec.Source.PVC.Namespace = sourceNamespace
				if sourceNamespace != dv.Namespace {
					dv.Finalizers = append(dv.Finalizers, crossNamespaceFinalizer)
				}
				srcPvc := CreatePvcInStorageClass("test", sourceNamespace, &scName, nil, nil, corev1.ClaimBound)
				reconciler = createCloneReconcilerWFFCDisabled(storageClass, csiDriver, dv, srcPvc)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				pvc := &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
				Expect(err).ToNot(HaveOccurred())
				Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				Expect(pvc.Labels[common.KubePersistentVolumeFillingUpSuppressLabelKey]).To(Equal(common.KubePersistentVolumeFillingUpSuppressLabelValue))
				Expect(pvc.Spec.DataSourceRef).ToNot(BeNil())
				if sourceNamespace != dv.Namespace {
					Expect(pvc.Annotations[populators.AnnDataSourceNamespace]).To(Equal(sourceNamespace))
				} else {
					Expect(pvc.Annotations).ToNot(HaveKey(populators.AnnDataSourceNamespace))
				}
				cloneSourceName := volumeCloneSourceName(dv)
				Expect(pvc.Spec.DataSourceRef.Name).To(Equal(cloneSourceName))
				Expect(pvc.Spec.DataSourceRef.Kind).To(Equal(cdiv1.VolumeCloneSourceRef))
				Expect(pvc.GetAnnotations()[AnnUsePopulator]).To(Equal("true"))
				_, annExists := pvc.Annotations[AnnImmediateBinding]
				Expect(annExists).To(BeTrue())
				vcs := &cdiv1.VolumeCloneSource{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: cloneSourceName, Namespace: sourceNamespace}, vcs)
				Expect(err).ToNot(HaveOccurred())
				Expect(vcs.Spec.Source.Kind).To(Equal("PersistentVolumeClaim"))
				Expect(vcs.Spec.Source.Name).To(Equal(srcPvc.Name))
			},
				Entry("with same namespace", metav1.NamespaceDefault),
				Entry("with different namespace", "source-ns"),
			)

			It("should add cloneType annotation", func() {
				dv := newCloneDataVolume("test-dv")
				anno := map[string]string{
					AnnExtendedCloneToken: "test-token",
					AnnCloneType:          string(cdiv1.CloneStrategySnapshot),
					AnnUsePopulator:       "true",
				}
				pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, anno, nil, corev1.ClaimPending)
				pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
					Kind: cdiv1.VolumeCloneSourceRef,
					Name: volumeCloneSourceName(dv),
				}
				pvc.OwnerReferences = append(pvc.OwnerReferences, metav1.OwnerReference{
					Kind:       "DataVolume",
					Controller: ptr.To[bool](true),
					Name:       "test-dv",
					UID:        dv.UID,
				})
				vcs := &cdiv1.VolumeCloneSource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceDefault,
						Name:      volumeCloneSourceName(dv),
					},
					Spec: cdiv1.VolumeCloneSourceSpec{
						Source: corev1.TypedLocalObjectReference{
							Kind: "PersistentVolumeClaim",
							Name: dv.Spec.Source.PVC.Name,
						},
					},
				}
				reconciler = createCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Annotations[AnnCloneType]).To(Equal(string(cdiv1.CloneStrategySnapshot)))
			})

			It("Fallback to host-assisted cloning when populator is not used", func() {
				dv := newCloneDataVolume("test-dv")
				dv.Annotations[AnnUsePopulator] = "false"

				anno := map[string]string{
					AnnExtendedCloneToken: "test-token",
					AnnCloneType:          string(cdiv1.CloneStrategySnapshot),
				}
				pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, anno, nil, corev1.ClaimPending)
				pvc.OwnerReferences = append(pvc.OwnerReferences, metav1.OwnerReference{
					Kind:       "DataVolume",
					Controller: ptr.To[bool](true),
					Name:       "test-dv",
					UID:        dv.UID,
				})

				reconciler = createCloneReconciler(storageClass, csiDriver, dv, pvc /*, vcs*/)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(Equal(2 * time.Second))

				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Annotations[AnnCloneType]).To(Equal(string(cdiv1.CloneStrategyHostAssisted)))

				pvc = &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
				Expect(err).ToNot(HaveOccurred())
				Expect(pvc.Annotations[AnnCloneType]).To(Equal(string(cdiv1.CloneStrategyHostAssisted)))
				Expect(pvc.Annotations[populators.AnnCloneFallbackReason]).To(Equal(NoPopulatorMessage))

				event := <-reconciler.recorder.(*record.FakeRecorder).Events
				Expect(event).To(ContainSubstring(NoPopulator))
				Expect(event).To(ContainSubstring(NoPopulatorMessage))
			})

			DescribeTable("should map phase correctly", func(phaseName string, dvPhase cdiv1.DataVolumePhase, eventReason string) {
				dv := newCloneDataVolume("test-dv")
				anno := map[string]string{
					AnnExtendedCloneToken:    "test-token",
					AnnCloneType:             string(cdiv1.CloneStrategySnapshot),
					populators.AnnClonePhase: phaseName,
					AnnUsePopulator:          "true",
				}
				pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, anno, nil, corev1.ClaimPending)
				pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
					Kind: cdiv1.VolumeCloneSourceRef,
					Name: volumeCloneSourceName(dv),
				}
				pvc.OwnerReferences = append(pvc.OwnerReferences, metav1.OwnerReference{
					Kind:       "DataVolume",
					Controller: ptr.To[bool](true),
					Name:       "test-dv",
					UID:        dv.UID,
				})
				vcs := &cdiv1.VolumeCloneSource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceDefault,
						Name:      volumeCloneSourceName(dv),
					},
					Spec: cdiv1.VolumeCloneSourceSpec{
						Source: corev1.TypedLocalObjectReference{
							Kind: "PersistentVolumeClaim",
							Name: dv.Spec.Source.PVC.Name,
						},
					},
				}
				reconciler = createCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				dv = &cdiv1.DataVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
				Expect(err).ToNot(HaveOccurred())
				Expect(dv.Status.Phase).To(Equal(dvPhase))
				found := false
				for event := range reconciler.recorder.(*record.FakeRecorder).Events {
					if strings.Contains(event, eventReason) {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			},
				Entry("empty phase", "", cdiv1.CloneScheduled, CloneScheduled),
				Entry("pending phase", clone.PendingPhaseName, cdiv1.CloneScheduled, CloneScheduled),
				Entry("succeeded phase", clone.SucceededPhaseName, cdiv1.Succeeded, CloneSucceeded),
				Entry("csi clone phase", clone.CSIClonePhaseName, cdiv1.CSICloneInProgress, CSICloneInProgress),
				Entry("host clone phase", clone.HostClonePhaseName, cdiv1.CloneInProgress, CloneInProgress),
				Entry("prep claim phase", clone.PrepClaimPhaseName, cdiv1.PrepClaimInProgress, PrepClaimInProgress),
				Entry("rebind phase", clone.RebindPhaseName, cdiv1.RebindInProgress, RebindInProgress),
				Entry("pvc from snapshot phase", clone.SnapshotClonePhaseName, cdiv1.CloneFromSnapshotSourceInProgress, CloneFromSnapshotSourceInProgress),
				Entry("create snapshot phase", clone.SnapshotPhaseName, cdiv1.SnapshotForSmartCloneInProgress, SnapshotForSmartCloneInProgress),
			)

			It("should delete VolumeCloneSource on success", func() {
				dv := newCloneDataVolume("test-dv")
				dv.Status.Phase = cdiv1.Succeeded
				anno := map[string]string{
					AnnExtendedCloneToken:    "test-token",
					AnnCloneType:             string(cdiv1.CloneStrategySnapshot),
					populators.AnnClonePhase: clone.SucceededPhaseName,
					AnnUsePopulator:          "true",
				}
				pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, anno, nil, corev1.ClaimPending)
				pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
					Kind: cdiv1.VolumeCloneSourceRef,
					Name: volumeCloneSourceName(dv),
				}
				pvc.OwnerReferences = append(pvc.OwnerReferences, metav1.OwnerReference{
					Kind:       "DataVolume",
					Controller: ptr.To[bool](true),
					Name:       "test-dv",
					UID:        dv.UID,
				})
				vcs := &cdiv1.VolumeCloneSource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: metav1.NamespaceDefault,
						Name:      volumeCloneSourceName(dv),
					},
					Spec: cdiv1.VolumeCloneSourceSpec{
						Source: corev1.TypedLocalObjectReference{
							Kind: "PersistentVolumeClaim",
							Name: dv.Spec.Source.PVC.Name,
						},
					},
				}
				reconciler = createCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
				Expect(result.RequeueAfter).To(BeZero())
				err = reconciler.client.Get(context.TODO(), client.ObjectKeyFromObject(vcs), vcs)
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	var _ = Describe("Reconcile Datavolume status", func() {
		DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			scName := "testpvc"
			srcPvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)

			r := createCloneReconciler(testDv, srcPvc, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent)
		},
			Entry("should switch to scheduled for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneScheduled, corev1.ClaimBound, corev1.PodPending, AnnCloneRequest, "Cloning from default/test into default/test-dv scheduled", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to clone in progress for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneInProgress, corev1.ClaimBound, corev1.PodRunning, AnnCloneRequest, "Cloning from default/test into default/test-dv in progress", AnnPriorityClassName, "p0-clone"),
			Entry("should stay the same for clone after pod fails", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.CloneScheduled, corev1.ClaimBound, corev1.PodFailed, AnnCloneRequest, "Cloning from default/test into default/test-dv failed", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to failed on claim lost for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnCloneRequest, "PVC test-dv lost", AnnPriorityClassName, "p0-clone"),
			Entry("should switch to succeeded for clone", newCloneDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnCloneRequest, "Successfully cloned from default/test into default/test-dv", AnnPriorityClassName, "p0-clone"),
		)
	})

	var _ = Describe("sourcePVCPopulated", func() {
		It("Should return true if source has no ownerRef", func() {
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			reconciler = createCloneReconciler(sourcePvc)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeTrue())
		})

		It("Should return false and error if source has an ownerRef, but it doesn't exist", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
			})
			reconciler = createCloneReconciler(sourcePvc)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeFalse())
		})

		It("Should return false if source has an ownerRef, but it is not succeeded", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourceDv := NewImportDataVolume("source-dv")
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "source-dv",
			})
			reconciler = createCloneReconciler(sourcePvc, sourceDv)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeFalse())
		})

		It("Should return true if source has an ownerRef, but it is succeeded", func() {
			controller := true
			sourcePvc := CreatePvc("test", "default", nil, nil)
			targetDv := newCloneDataVolume("test-dv")
			sourceDv := NewImportDataVolume("source-dv")
			sourceDv.Status.Phase = cdiv1.Succeeded
			sourcePvc.OwnerReferences = append(sourcePvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "source-dv",
			})
			reconciler = createCloneReconciler(sourcePvc, sourceDv)
			res, err := reconciler.isSourcePVCPopulated(targetDv)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(BeTrue())
		})
	})

	var _ = Describe("Clone without source", func() {
		scName := "testsc"
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, map[string]string{}, "csi-plugin")

		syncState := func(dv *cdiv1.DataVolume) *dvSyncState {
			return &dvSyncState{dv: dv, dvMutated: dv.DeepCopy()}
		}

		It("Validate clone without source as feasible, but not done", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			reconciler = createCloneReconciler(dv, storageProfile, sc)

			done, err := reconciler.validateCloneAndSourcePVC(syncState(dv), reconciler.log)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
		})

		It("Validate that clone without source completes after PVC is created", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			reconciler = createCloneReconciler(dv, storageProfile, sc)

			done, err := reconciler.validateCloneAndSourcePVC(syncState(dv), reconciler.log)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())

			// We create the source PVC after creating the clone
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			err = reconciler.client.Create(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())

			done, err = reconciler.validateCloneAndSourcePVC(syncState(dv), reconciler.log)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
		})

		It("Validate clone already populated without source completes", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnPopulatedFor] = "test-dv"
			reconciler = createCloneReconciler(dv, pvc, storageProfile, sc)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.ClaimName).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(dv.Annotations[AnnPrePopulated]).To(Equal("test-dv"))
			Expect(dv.Annotations[AnnCloneType]).To(BeEmpty())

			pvc = &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			Expect(pvc.OwnerReferences[0].Name).To(Equal("test-dv"))
			Expect(pvc.OwnerReferences[0].Kind).To(Equal("DataVolume"))
		})

		It("Validate clone will adopt PVC (with annotation)", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnAllowClaimAdoption] = "true"
			reconciler = createCloneReconciler(dv, pvc, storageProfile, sc)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.ClaimName).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(dv.Annotations[AnnCloneType]).To(BeEmpty())

			pvc = &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			Expect(pvc.OwnerReferences[0].Name).To(Equal("test-dv"))
			Expect(pvc.OwnerReferences[0].Kind).To(Equal("DataVolume"))
		})

		It("Validate clone will adopt unbound PVC (with annotation)", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimPending)
			pvc.Spec.VolumeName = ""
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnAllowClaimAdoption] = "true"
			reconciler = createCloneReconciler(dv, pvc, storageProfile, sc)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.ClaimName).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(dv.Annotations[AnnCloneType]).To(BeEmpty())

			pvc = &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			Expect(pvc.OwnerReferences[0].Name).To(Equal("test-dv"))
			Expect(pvc.OwnerReferences[0].Kind).To(Equal("DataVolume"))
		})

		It("Validate clone will adopt PVC (with featuregate)", func() {
			dv := newCloneDataVolume("test-dv")
			storageProfile := createStorageProfile(scName, nil, FilesystemMode)
			pvc := CreatePvcInStorageClass("test-dv", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			featureGates := []string{featuregates.DataVolumeClaimAdoption, featuregates.HonorWaitForFirstConsumer}
			reconciler = createCloneReconcilerWithFeatureGates(featureGates, dv, pvc, storageProfile, sc)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(BeZero())

			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.ClaimName).To(Equal("test-dv"))
			Expect(dv.Status.Phase).To(Equal(cdiv1.Succeeded))
			Expect(dv.Annotations[AnnCloneType]).To(BeEmpty())

			pvc = &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.OwnerReferences).To(HaveLen(1))
			Expect(pvc.OwnerReferences[0].Name).To(Equal("test-dv"))
			Expect(pvc.OwnerReferences[0].Kind).To(Equal("DataVolume"))
		})

		DescribeTable("Validation mechanism rejects or accepts the clone depending on the contentType combination",
			func(sourceContentType, targetContentType string, expectedResult bool) {
				dv := newCloneDataVolume("test-dv")
				dv.Spec.ContentType = cdiv1.DataVolumeContentType(targetContentType)
				storageProfile := createStorageProfile(scName, nil, FilesystemMode)
				reconciler = createCloneReconciler(dv, storageProfile, sc)

				done, err := reconciler.validateCloneAndSourcePVC(syncState(dv), reconciler.log)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeFalse())

				// We create the source PVC after creating the clone
				pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, map[string]string{
					AnnContentType: sourceContentType}, nil, corev1.ClaimBound)
				err = reconciler.client.Create(context.TODO(), pvc)
				Expect(err).ToNot(HaveOccurred())

				done, err = reconciler.validateCloneAndSourcePVC(syncState(dv), reconciler.log)
				Expect(done).To(Equal(expectedResult))
				if expectedResult == false {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry("Archive in source and target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeArchive), true),
			Entry("Archive in source and KubeVirt in target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeKubeVirt), false),
			Entry("KubeVirt in source and archive in target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeArchive), false),
			Entry("KubeVirt in source and target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeKubeVirt), true),
			Entry("Empty (KubeVirt by default) in source and target", "", "", true),
			Entry("Empty (KubeVirt by default) in source and KubeVirt (explicit) in target", "", string(cdiv1.DataVolumeKubeVirt), true),
			Entry("KubeVirt (explicit) in source and empty (KubeVirt by default) in target", string(cdiv1.DataVolumeKubeVirt), "", true),
			Entry("Empty (kubeVirt by default) in source and archive in target", "", string(cdiv1.DataVolumeArchive), false),
			Entry("Archive in source and empty (KubeVirt by default) in target", string(cdiv1.DataVolumeArchive), "", false),
		)
	})

	var _ = Describe("Clone with empty storage size", func() {
		scName := "testsc"
		accessMode := []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
		sc := CreateStorageClassWithProvisioner(scName, map[string]string{
			AnnDefaultStorageClass: "true",
		}, map[string]string{}, "csi-plugin")

		syncState := func(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim, pvcSpec *corev1.PersistentVolumeClaimSpec) *dvSyncState {
			return &dvSyncState{dv: dv, dvMutated: dv.DeepCopy(), pvc: pvc, pvcSpec: pvcSpec}
		}

		// detectCloneSize tests
		It("Size-detection fails when source PVC is not attainable", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			targetPvc := &corev1.PersistentVolumeClaim{}
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			reconciler := createCloneReconciler(dv, storageProfile, sc)
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, nil)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(syncState(dv, targetPvc, pvcSpec))
			Expect(err).To(HaveOccurred())
			Expect(done).To(BeFalse())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("Size-detection fails when source PVC is not fully imported", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			sourceDV := &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-dv",
					Namespace: metav1.NamespaceDefault,
				},
				Status: cdiv1.DataVolumeStatus{
					Phase: cdiv1.ImportInProgress,
				},
			}
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       "DataVolume",
					Name:       sourceDV.Name,
					Controller: ptr.To[bool](true),
				},
			}
			AddAnnotation(pvc, AnnContentType, "kubevirt")
			reconciler := createCloneReconciler(dv, sourceDV, pvc, storageProfile, sc)

			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(syncState(dv, pvc, pvcSpec))
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, ImportPVCNotReady) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Size-detection fails when Pod is not ready", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.Annotations[AnnContentType] = "kubevirt"
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(syncState(dv, pvc, pvcSpec))
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeFalse())
			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, SizeDetectionPodNotReady) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())

		})

		It("Size-detection fails when pod's termination message is invalid", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.Annotations[AnnContentType] = "kubevirt"
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Prepare the size-detection Pod with the required information
			pod := reconciler.makeSizeDetectionPodSpec(pvc, dv)
			pod.Status.Phase = corev1.PodSucceeded
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			// Checks
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
			Expect(err).ToNot(HaveOccurred())
			done, err := reconciler.detectCloneSize(syncState(dv, pvc, pvcSpec))
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrInvalidTermMsg))
			Expect(done).To(BeFalse())
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, pod)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			By("Checking error event recorded")
			event := <-reconciler.recorder.(*record.FakeRecorder).Events
			Expect(event).To(ContainSubstring("Size-detection pod failed due to"))
		})

		It("Should get the size from the size-detection pod", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.Annotations[AnnContentType] = "kubevirt"
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Prepare the size-detection Pod with the required information
			pod := reconciler.makeSizeDetectionPodSpec(pvc, dv)
			pod.Status.Phase = corev1.PodSucceeded
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  "100", // Mock value
						},
					},
				},
			}
			err := reconciler.client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			// Get the expected value
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
			Expect(err).ToNot(HaveOccurred())
			expectedSize, err := InflateSizeWithOverhead(context.TODO(), reconciler.client, int64(100), pvcSpec)
			Expect(err).ToNot(HaveOccurred())
			expectedSizeInt64, _ := expectedSize.AsInt64()

			// Checks
			syncState := syncState(dv, pvc, pvcSpec)
			done, err := reconciler.detectCloneSize(syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(syncState.dvMutated.Annotations[AnnPermissiveClone]).To(Equal("true"))
			targetSize := pvcSpec.Resources.Requests[corev1.ResourceStorage]
			targetSizeInt64, _ := targetSize.AsInt64()
			Expect(targetSizeInt64).To(Equal(expectedSizeInt64))
		})

		It("Should get the size from the source PVC's annotations", func() {
			dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
			cloneStrategy := cdiv1.CloneStrategyHostAssisted
			storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
				{AccessModes: accessMode, VolumeMode: &BlockMode}}, &cloneStrategy)

			// Prepare the source PVC with the required annotations
			pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
			pvc.SetAnnotations(make(map[string]string))
			pvc.GetAnnotations()[AnnVirtualImageSize] = "100" // Mock value
			pvc.GetAnnotations()[AnnSourceCapacity] = string(pvc.Status.Capacity.Storage().String())
			pvc.GetAnnotations()[AnnContentType] = "kubevirt"
			reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

			// Get the expected value
			pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
			Expect(err).ToNot(HaveOccurred())
			expectedSize, err := InflateSizeWithOverhead(context.TODO(), reconciler.client, int64(100), pvcSpec)
			Expect(err).ToNot(HaveOccurred())
			expectedSizeInt64, _ := expectedSize.AsInt64()

			// Checks
			syncState := syncState(dv, pvc, pvcSpec)
			done, err := reconciler.detectCloneSize(syncState)
			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(syncState.dvMutated.Annotations[AnnPermissiveClone]).To(Equal("true"))
			targetSize := pvcSpec.Resources.Requests[corev1.ResourceStorage]
			targetSizeInt64, _ := targetSize.AsInt64()
			Expect(targetSizeInt64).To(Equal(expectedSizeInt64))
		})

		DescribeTable("Should automatically collect the clone size from the source PVC's spec",
			func(cloneStrategy cdiv1.CDICloneStrategy, volumeMode corev1.PersistentVolumeMode) {
				dv := newCloneDataVolumeWithEmptyStorage("test-dv", "default")
				storageProfile := createStorageProfileWithCloneStrategy(scName, []cdiv1.ClaimPropertySet{
					{AccessModes: accessMode, VolumeMode: &volumeMode}}, &cloneStrategy)

				pvc := CreatePvcInStorageClass("test", metav1.NamespaceDefault, &scName, nil, nil, corev1.ClaimBound)
				pvc.Spec.VolumeMode = &volumeMode
				reconciler := createCloneReconciler(dv, pvc, storageProfile, sc)

				pvcSpec, err := renderPvcSpec(reconciler.client, reconciler.recorder, reconciler.log, dv, pvc)
				Expect(err).ToNot(HaveOccurred())
				expectedSize := *pvc.Status.Capacity.Storage()
				done, err := reconciler.detectCloneSize(syncState(dv, pvc, pvcSpec))
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())
				Expect(pvc.Spec.Resources.Requests.Storage().Cmp(expectedSize)).To(Equal(0))
			},
			Entry("hostAssited with empty size and 'Block' volume mode", cdiv1.CloneStrategyHostAssisted, BlockMode),
		)
	})
})

func createCloneReconcilerWFFCDisabled(objects ...runtime.Object) *PvcCloneReconciler {
	return createCloneReconcilerWithFeatureGates(nil, objects...)
}

func createCloneReconciler(objects ...runtime.Object) *PvcCloneReconciler {
	return createCloneReconcilerWithFeatureGates([]string{featuregates.HonorWaitForFirstConsumer}, objects...)
}

func createCloneReconcilerWithFeatureGates(featireGates []string, objects ...runtime.Object) *PvcCloneReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = featireGates

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createCloneReconcilerWithoutConfig(objs...)
}

func createCloneReconcilerWithoutConfig(objects ...runtime.Object) *PvcCloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &PvcCloneReconciler{
		CloneReconcilerBase: CloneReconcilerBase{
			ReconcilerBase: ReconcilerBase{
				client:       cl,
				scheme:       s,
				log:          dvCloneLog,
				recorder:     rec,
				featureGates: featuregates.NewFeatureGates(cl),
				installerLabels: map[string]string{
					common.AppKubernetesPartOfLabel:  "testing",
					common.AppKubernetesVersionLabel: "v0.0.0-tests",
				},
				shouldUpdateProgress: true,
			},
			shortTokenValidator: &FakeValidator{Match: "foobar"},
			longTokenValidator:  &FakeValidator{Match: "foobar", Params: map[string]string{"uid": "uid"}},
			tokenGenerator:      &FakeGenerator{token: "foobar"},
			cloneSourceKind:     "PersistentVolumeClaim",
		},
	}
	return r
}

func newCloneDataVolume(name string) *cdiv1.DataVolume {
	return newCloneDataVolumeWithPVCNS(name, "default")
}

func newCloneDataVolumeWithPVCNS(name string, pvcNamespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Annotations: map[string]string{
				AnnCloneToken: "foobar",
			},
			UID: types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: pvcNamespace,
				},
			},
			PriorityClassName: "p0-clone",
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1G"),
					},
				},
			},
		},
	}
}

func newCloneDataVolumeWithEmptyStorage(name string, pvcNamespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			Annotations: map[string]string{
				AnnCloneToken: "foobar",
			},
			UID: types.UID("uid"),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: pvcNamespace,
				},
			},
			PriorityClassName: "p0-clone",
			Storage:           &cdiv1.StorageSpec{},
		},
	}
}

func createSnapshotClass(name string, annotations map[string]string, snapshotter string) *snapshotv1.VolumeSnapshotClass {
	return &snapshotv1.VolumeSnapshotClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumeSnapshotClass",
			APIVersion: snapshotv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Driver: snapshotter,
	}
}

func createVolumeSnapshotContentCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshotcontents"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.ClusterScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshotContent{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
			},
		},
	}
}

func createVolumeSnapshotClassCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshotclasses"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.ClusterScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshotClass{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
			},
		},
	}
}

func createVolumeSnapshotCrd() *extv1.CustomResourceDefinition {
	pluralName := "volumesnapshots"
	return &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: extv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pluralName + "." + snapshotv1.GroupName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: snapshotv1.GroupName,
			Scope: extv1.NamespaceScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Plural: pluralName,
				Kind:   reflect.TypeOf(snapshotv1.VolumeSnapshot{}).Name(),
			},
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:   snapshotv1.SchemeGroupVersion.Version,
					Served: true,
				},
			},
		},
	}
}

type FakeGenerator struct {
	token string
}

func (g *FakeGenerator) Generate(*token.Payload) (string, error) {
	return g.token, nil
}
