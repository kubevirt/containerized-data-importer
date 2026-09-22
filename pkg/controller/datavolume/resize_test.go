
package datavolume

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("DataVolume Resize", func() {
	It("Should resize PVC when DataVolume size is increased", func() {
		dvName := "test-dv"
		namespace := metav1.NamespaceDefault
		initialSize := "1Gi"
		updatedSize := "2Gi"

		dv := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dvName,
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					HTTP: &cdiv1.DataVolumeSourceHTTP{
						URL: "http://example.com/image.img",
					},
				},
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(initialSize),
						},
					},
				},
			},
		}

		reconciler := createImportReconciler(dv)

		// First reconcile to create PVC
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		initialQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qInitial := resource.MustParse(initialSize)
		Expect(initialQuantity.Value()).To(Equal(qInitial.Value()))

		// Update DataVolume size
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, dv)
		Expect(err).ToNot(HaveOccurred())
		dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(updatedSize)
		err = reconciler.client.Update(context.TODO(), dv)
		Expect(err).ToNot(HaveOccurred())

		// Second reconcile to trigger resize
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		// Check if PVC is resized
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		updatedQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qUpdated := resource.MustParse(updatedSize)
		Expect(updatedQuantity.Value()).To(Equal(qUpdated.Value()))
	})

	It("Should resize PVC when DataVolume size is increased (using Spec.Storage)", func() {
		dvName := "test-dv-storage"
		namespace := metav1.NamespaceDefault
		initialSize := "1Gi"
		updatedSize := "2Gi"

		dv := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dvName,
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					HTTP: &cdiv1.DataVolumeSourceHTTP{
						URL: "http://example.com/image.img",
					},
				},
				Storage: &cdiv1.StorageSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(initialSize),
						},
					},
				},
			},
		}

		reconciler := createImportReconciler(dv)

		// First reconcile to create PVC
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		initialQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qInitial := resource.MustParse(initialSize)
		Expect(initialQuantity.Value()).To(Equal(qInitial.Value()))

		// Update DataVolume size
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, dv)
		Expect(err).ToNot(HaveOccurred())
		dv.Spec.Storage.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(updatedSize)
		err = reconciler.client.Update(context.TODO(), dv)
		Expect(err).ToNot(HaveOccurred())

		// Second reconcile to trigger resize
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		// Check if PVC is resized
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		updatedQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qUpdated := resource.MustParse(updatedSize)
		Expect(updatedQuantity.Value()).To(Equal(qUpdated.Value()))
	})

	It("Should resize PVC when DataVolume size is increased (Clone)", func() {
		dvName := "test-dv-clone"
		namespace := metav1.NamespaceDefault
		initialSize := "1Gi"
		updatedSize := "2Gi"

		sourcePvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "source-pvc",
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(initialSize),
					},
				},
			},
		}

		dv := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dvName,
				Namespace: namespace,
			},
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					PVC: &cdiv1.DataVolumeSourcePVC{
						Name:      "source-pvc",
						Namespace: namespace,
					},
				},
				PVC: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(initialSize),
						},
					},
				},
			},
		}

		reconciler := createCloneReconciler(sourcePvc, dv)

		// First reconcile to create PVC
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		pvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		initialQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qInitial := resource.MustParse(initialSize)
		Expect(initialQuantity.Value()).To(Equal(qInitial.Value()))

		// Update DataVolume size
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, dv)
		Expect(err).ToNot(HaveOccurred())
		dv.Spec.PVC.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(updatedSize)
		err = reconciler.client.Update(context.TODO(), dv)
		Expect(err).ToNot(HaveOccurred())

		// Second reconcile to trigger resize
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: dvName, Namespace: namespace}})
		Expect(err).ToNot(HaveOccurred())

		// Check if PVC is resized
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: dvName, Namespace: namespace}, pvc)
		Expect(err).ToNot(HaveOccurred())
		updatedQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		qUpdated := resource.MustParse(updatedSize)
		Expect(updatedQuantity.Value()).To(Equal(qUpdated.Value()))
	})
})
