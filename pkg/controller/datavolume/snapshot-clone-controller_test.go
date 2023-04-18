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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
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
					ReadyToUse:  &readyToUse,
					RestoreSize: &size,
				},
			}
		}

		It("Should create a restore PVC if snapclass exists and no reason to fall back to host assisted", func() {
			dv := newCloneFromSnapshotDataVolume("test-dv")
			scName := "testsc"
			expectedSnapshotClass := "snap-class"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{
				AnnDefaultStorageClass: "true",
			}, map[string]string{}, "csi-plugin")
			sp := createStorageProfile(scName, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, BlockMode)

			dv.Spec.PVC.StorageClassName = &scName
			snapshot := createSnapshotInVolumeSnapshotClass("test-snap", metav1.NamespaceDefault, &expectedSnapshotClass, nil, nil, true)
			snapClass := createSnapshotClass(expectedSnapshotClass, nil, "csi-plugin")
			reconciler = createSnapshotCloneReconciler(sc, sp, dv, snapshot, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying that target PVC now exists")
			pvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}, pvc)
			Expect(err).ToNot(HaveOccurred())
			expectedDataSource := &corev1.TypedLocalObjectReference{
				Name:     snapshot.Name,
				Kind:     "VolumeSnapshot",
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
			}
			Expect(pvc.Spec.DataSource).To(Equal(expectedDataSource))
			Expect(pvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

			dv = &cdiv1.DataVolume{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}, dv)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv.Status.Phase).To(Equal(cdiv1.CloneFromSnapshotSourceInProgress))
		})

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
			reconciler = createSnapshotCloneReconciler(sc, tsc, sp, sp2, dv, snapshot, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
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
			err = reconciler.client.Update(context.TODO(), pvc)
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
			reconciler = createSnapshotCloneReconciler(sc, scSameProvisioner, tsc, sp, sp2, sp3, dv, snapshot, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
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
			reconciler = createSnapshotCloneReconciler(sc, tsc, sp, sp2, dv, snapshot, tempHostAssistedPvc, targetPvc, snapClass, createVolumeSnapshotContentCrd(), createVolumeSnapshotClassCrd(), createVolumeSnapshotCrd())
			_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			By("Verifying that temp host assisted source PVC is being deleted")
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: tempHostAssistedPvc.Namespace, Name: tempHostAssistedPvc.Name}, tempHostAssistedPvc)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})
	})
})

func createSnapshotCloneReconciler(objects ...runtime.Object) *SnapshotCloneReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createSnapshotCloneReconcilerWithoutConfig(objs...)
}

func createSnapshotCloneReconcilerWithoutConfig(objects ...runtime.Object) *SnapshotCloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)
	extv1.AddToScheme(s)

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
			},
			tokenValidator: &FakeValidator{Match: "foobar"},
			tokenGenerator: &FakeGenerator{token: "foobar"},
		},
	}
	r.Reconciler = r
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
