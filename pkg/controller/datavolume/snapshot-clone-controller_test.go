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
	"fmt"
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
)

var (
	dvSnapshotCloneLog = logf.Log.WithName("datavolume-snapshot-clone-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *SnapshotCloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	var _ = Describe("Clone from volumesnapshot source", func() {
		createSnapshotInVolumeSnapshotClass := func(name, ns string, snapClassName *string, annotations, labels map[string]string, readyToUse bool) *snapshotv1.VolumeSnapshot {
			pvcName := "some-pvc-that-was-snapshotted"
			volumeSnapshotContentName := "test-snapshot-content-name"
			size := resource.MustParse("1G")

			return &snapshotv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
				Spec: snapshotv1.VolumeSnapshotSpec{
					Source: snapshotv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvcName,
					},
					VolumeSnapshotClassName: snapClassName,
				},
				Status: &snapshotv1.VolumeSnapshotStatus{
					ReadyToUse:                     &readyToUse,
					RestoreSize:                    &size,
					BoundVolumeSnapshotContentName: &volumeSnapshotContentName,
				},
			}
		}

		createDefaultVolumeSnapshotContent := func() *snapshotv1.VolumeSnapshotContent {
			return &snapshotv1.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-snapshot-content-name",
				},
				Spec: snapshotv1.VolumeSnapshotContentSpec{
					Driver: "csi-plugin",
				},
			}
		}

		It("Should fall back to host assisted when target DV storage class has different provisioner", func() {
			dv := newCloneFromSnapshotDataVolume("test-dv")
			scName := "testsc"
			expectedSnapshotClass := "snap-class"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			targetScName := "targetsc"
			tsc := CreateStorageClassWithProvisioner(targetScName, map[string]string{}, map[string]string{}, "another-csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			sp2 := createStorageProfile(targetScName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &targetScName
			snapshot := createSnapshotInVolumeSnapshotClass("test-snap", metav1.NamespaceDefault, &expectedSnapshotClass, nil, nil, true)
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createSnapshotCloneReconciler(sc, tsc, sp, sp2, dv, snapshot, snapClass, createDefaultVolumeSnapshotContent(), createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying that temp host assisted source PVC is being created")
			pvc := &corev1.PersistentVolumeClaim{}
			tempPvcName := getTempHostAssistedSourcePvcName(dv)
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: snapshot.Namespace, Name: tempPvcName}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Labels[common.CDIComponentLabel]).To(Equal("cdi-clone-from-snapshot-source-host-assisted-fallback-pvc"))
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			By("Verifying that target host assisted PVC is being created")
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, pvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
			Expect(pvc.Annotations[AnnCloneRequest]).To(Equal(fmt.Sprintf("%s/%s", snapshot.Namespace, tempPvcName)))
			By("Mark target PVC bound like it would be in a live cluster, so DV status is updated")
			pvc.Status.Phase = corev1.ClaimBound
			err = reconciler.client.Status().Update(context.TODO(), pvc)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.CloneScheduled))
		})

		It("Should pick first storage class when host assisted fallback is needed but multiple matching storage classes exist", func() {
			dv := newCloneFromSnapshotDataVolume("test-dv")
			scName := "testsc"
			expectedSnapshotClass := "snap-class"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			targetScName := "targetsc"
			scSameProvisioner := sc.DeepCopy()
			scSameProvisioner.Name = "same-provisioner-as-source-sc"
			tsc := CreateStorageClassWithProvisioner(targetScName, map[string]string{}, map[string]string{}, "another-csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			sp2 := createStorageProfile(targetScName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			sp3 := createStorageProfile(scSameProvisioner.Name, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &targetScName
			snapshot := createSnapshotInVolumeSnapshotClass("test-snap", metav1.NamespaceDefault, &expectedSnapshotClass, nil, nil, true)
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createSnapshotCloneReconciler(sc, scSameProvisioner, tsc, sp, sp2, sp3, dv, snapshot, snapClass, createDefaultVolumeSnapshotContent(), createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should clean up host assisted source temp PVC when done", func() {
			dv := newCloneFromSnapshotDataVolume("test-dv")
			dv.Status.Phase = cdiv1.Succeeded
			scName := "testsc"
			expectedSnapshotClass := "snap-class"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			targetScName := "targetsc"
			tsc := CreateStorageClassWithProvisioner(targetScName, map[string]string{}, map[string]string{}, "another-csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)
			sp2 := createStorageProfile(targetScName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &targetScName
			snapshot := createSnapshotInVolumeSnapshotClass("test-snap", metav1.NamespaceDefault, &expectedSnapshotClass, nil, nil, true)
			labels := map[string]string{
				common.CDIComponentLabel: "cdi-clone-from-snapshot-source-host-assisted-fallback-pvc",
			}
			tempHostAssistedPvc := CreatePvcInStorageClass(getTempHostAssistedSourcePvcName(dv), snapshot.Namespace, &scName, nil, labels, corev1.ClaimBound)
			err := setAnnOwnedByDataVolume(tempHostAssistedPvc, dv)
			Expect(err).ToNot(HaveOccurred())
			// mimic target PVC being aroud
			annotations := map[string]string{
				AnnCloneToken: "foobar",
			}
			targetPvc := CreatePvcInStorageClass(dv.Name, dv.Namespace, &targetScName, annotations, nil, corev1.ClaimBound)
			controller := true
			targetPvc.OwnerReferences = append(targetPvc.OwnerReferences, metav1.OwnerReference{
				Kind:       "DataVolume",
				Controller: &controller,
				Name:       "test-dv",
				UID:        dv.UID,
			})
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createSnapshotCloneReconciler(sc, tsc, sp, sp2, dv, snapshot, tempHostAssistedPvc, targetPvc, snapClass, createDefaultVolumeSnapshotContent(), createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying that temp host assisted source PVC is being deleted")
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: tempHostAssistedPvc.Namespace, Name: tempHostAssistedPvc.Name}, tempHostAssistedPvc)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		var _ = Describe("Snapshot clone controller populator integration", func() {
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
					expectedSnapshotClass = "snap-class"
				)

				BeforeEach(func() {
					storageClass = CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, pluginName)
				})

				It("should add extended token", func() {
					dv := newCloneFromSnapshotDataVolumeWithPVCNS("test-dv", "source-ns")
					snapshot := createSnapshotInVolumeSnapshotClass("test-snap", "source-ns", &expectedSnapshotClass, nil, nil, true)
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, snapshot)
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
					dv := newCloneFromSnapshotDataVolumeWithPVCNS("test-dv", "source-ns")
					dv.Annotations[AnnExtendedCloneToken] = "test-token"
					snapshot := createSnapshotInVolumeSnapshotClass("test-snap", "source-ns", &expectedSnapshotClass, nil, nil, true)
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, snapshot)
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
					dv := newCloneFromSnapshotDataVolumeWithPVCNS("test-dv", sourceNamespace)
					dv.Annotations[AnnExtendedCloneToken] = "foobar"
					if sourceNamespace != dv.Namespace {
						dv.Finalizers = append(dv.Finalizers, crossNamespaceFinalizer)
					}
					snapshot := createSnapshotInVolumeSnapshotClass("test-snap", sourceNamespace, &expectedSnapshotClass, nil, nil, true)
					reconciler = createSnapshotCloneReconcilerWFFCDisabled(storageClass, csiDriver, dv, snapshot)
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
					Expect(vcs.Spec.Source.APIGroup).ToNot(BeNil())
					Expect(*vcs.Spec.Source.APIGroup).To(Equal("snapshot.storage.k8s.io"))
					Expect(vcs.Spec.Source.Kind).To(Equal("VolumeSnapshot"))
					Expect(vcs.Spec.Source.Name).To(Equal(snapshot.Name))
				},
					Entry("with same namespace", metav1.NamespaceDefault),
					Entry("with different namespace", "source-ns"),
				)

				It("should handle size omitted", func() {
					dv := newCloneFromSnapshotDataVolume("test-dv")
					vm := corev1.PersistentVolumeFilesystem
					dv.Spec.Storage = &cdiv1.StorageSpec{
						AccessModes: dv.Spec.PVC.AccessModes,
						VolumeMode:  &vm,
					}
					dv.Spec.PVC = nil
					snapshot := createSnapshotInVolumeSnapshotClass("test-snap", dv.Namespace, &expectedSnapshotClass, nil, nil, true)
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, snapshot)
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
					Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(*snapshot.Status.RestoreSize))
				})

				It("should add cloneType annotation", func() {
					dv := newCloneFromSnapshotDataVolume("test-dv")
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
								APIGroup: ptr.To[string]("snapshot.storage.k8s.io"),
								Kind:     "VolumeSnapshot",
								Name:     dv.Spec.Source.Snapshot.Name,
							},
						},
					}
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
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
					dv := newCloneFromSnapshotDataVolume("test-dv")
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

					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, pvc)
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
					dv := newCloneFromSnapshotDataVolume("test-dv")
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
								APIGroup: ptr.To[string]("snapshot.storage.k8s.io"),
								Kind:     "VolumeSnapshot",
								Name:     dv.Spec.Source.Snapshot.Name,
							},
						},
					}
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
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
					Entry("host clone phase", clone.HostClonePhaseName, cdiv1.CloneInProgress, CloneInProgress),
					Entry("prep claim phase", clone.PrepClaimPhaseName, cdiv1.PrepClaimInProgress, PrepClaimInProgress),
					Entry("rebind phase", clone.RebindPhaseName, cdiv1.RebindInProgress, RebindInProgress),
					Entry("pvc from snapshot phase", clone.SnapshotClonePhaseName, cdiv1.CloneFromSnapshotSourceInProgress, CloneFromSnapshotSourceInProgress),
				)

				It("should delete VolumeCloneSource on success", func() {
					dv := newCloneFromSnapshotDataVolume("test-dv")
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
								APIGroup: ptr.To[string]("snapshot.storage.k8s.io"),
								Kind:     "VolumeSnapshot",
								Name:     dv.Spec.Source.Snapshot.Name,
							},
						},
					}
					reconciler = createSnapshotCloneReconciler(storageClass, csiDriver, dv, pvc, vcs)
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
	})
})

func createSnapshotCloneReconcilerWFFCDisabled(objects ...client.Object) *SnapshotCloneReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{}

	objs := []client.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createSnapshotCloneReconcilerWithoutConfig(objs...)
}

func createSnapshotCloneReconciler(objects ...client.Object) *SnapshotCloneReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []client.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createSnapshotCloneReconcilerWithoutConfig(objs...)
}

func createSnapshotCloneReconcilerWithoutConfig(objects ...client.Object) *SnapshotCloneReconciler {
	objs := []client.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &SnapshotCloneReconciler{
		CloneReconcilerBase: CloneReconcilerBase{
			ReconcilerBase: ReconcilerBase{
				client:       cl,
				scheme:       s,
				log:          dvSnapshotCloneLog,
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
			cloneSourceAPIGroup: ptr.To[string]("snapshot.storage.k8s.io"),
			cloneSourceKind:     "VolumeSnapshot",
		},
	}
	return r
}

func newCloneFromSnapshotDataVolume(name string) *cdiv1.DataVolume {
	return newCloneFromSnapshotDataVolumeWithPVCNS(name, "default")
}

func newCloneFromSnapshotDataVolumeWithPVCNS(name string, snapNamespace string) *cdiv1.DataVolume {
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
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Name:      "test-snap",
					Namespace: snapNamespace,
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
