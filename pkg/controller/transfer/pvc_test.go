/*
Copyright 2020 The CDI Authors.

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

package transfer_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("PVC Transfer Tests", func() {

	var _ = Describe("PVC Transfer Tests", func() {
		It("Should initialize", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferEmpty)

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Finalizers).To(HaveLen(1))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "Initializing", "")
		})

		It("Should handle no source", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "No source", "")
		})

		It("Should handle unbound source", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer, createUnboundPVC())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "PVC not bound", "")
		})

		It("Should handle PVC with finalizer", func() {
			f := "snapshot.storage.kubernetes.io/pvc-as-source-protection"
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)
			pvc := createBoundPVC()
			pvc.Finalizers = append(pvc.Finalizers, f)

			r := createReconciler(xfer, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "PVC has finalizer: "+f, "")
		})

		It("Should handle PV not bound", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)
			pvc := createBoundPVC()
			pv := sourcePV()
			pv.Spec.ClaimRef = nil

			r := createReconciler(xfer, pvc, pv)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "PV not bound", "")
		})

		It("Should handle pod using PVC", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)
			pvc := createBoundPVC()
			pv := sourcePV()

			r := createReconciler(xfer, pvc, pv, createPod(pvc.Name))
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "Pods using PVC", "")
		})

		It("Should handle another transfer", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)
			pvc := createBoundPVC()
			pvc.Annotations = map[string]string{
				"cdi.kubevirt.io/objectTransferName": "baz",
			}

			r := createReconciler(xfer, sourcePV(), pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "Source in use by another transfer", "baz")
		})

		It("Should cancel", func() {
			t := metav1.Now()
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)
			xfer.Finalizers = []string{
				"cdi.kubevirt.io/objectTransfer",
			}
			xfer.DeletionTimestamp = &t

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Finalizers).To(HaveLen(0))
		})

		It("Should become running", func() {
			xfer := pvcTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer, sourcePV(), createBoundPVC())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			Expect(xfer.Status.Data).To(Equal(pvcTransferRunning().Status.Data))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should handle missing pv name", func() {
			xfer := pvcTransferRunning()
			delete(xfer.Status.Data, "pvName")

			r := createReconciler(xfer, createBoundPVC())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferError))
			checkCompleteFalse(xfer, "PV name missing", "")
		})

		It("Should error missing pv", func() {
			xfer := pvcTransferRunning()

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).To(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferError))
			checkCompleteFalse(xfer, "Error", "persistentvolumes \"source-pv\" not found")
		})

		It("Should store reclaim", func() {
			xfer := pvcTransferRunning()

			r := createReconciler(xfer, sourcePV())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			Expect(xfer.Status.Data["pvReclaim"]).To(Equal("Delete"))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should update PV reclaim", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pvc := createBoundPVC()

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", pv.Name, pv)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", pvc.Name, pvc)
			Expect(err).To(HaveOccurred())

			Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(corev1.PersistentVolumeReclaimRetain))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should delete source pvc", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			pvc := createBoundPVC()

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", pv.Name, pv)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, pvc.Namespace, pvc.Name, pvc)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should update claimref", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

			r := createReconciler(xfer, pv)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", pv.Name, pv)
			Expect(err).ToNot(HaveOccurred())

			Expect(pv.Spec.ClaimRef).ToNot(BeNil())
			Expect(&pv.Spec.ClaimRef.Namespace).To(Equal(xfer.Spec.Target.Namespace))
			Expect(&pv.Spec.ClaimRef.Name).To(Equal(xfer.Spec.Target.Name))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should create target", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			pv.Spec.ClaimRef = nil
			pvc := &corev1.PersistentVolumeClaim{}

			r := createReconciler(xfer, pv)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "target-ns", "target-pvc", pvc)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should wait for target to be bound", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			pv.Spec.ClaimRef = nil
			pvc := createUnboundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-pvc"

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Waiting for target to be bound", "")
		})

		It("Should error if PV gets bound to something else", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			pv.Spec.ClaimRef.Namespace = "target-ns"
			pv.Spec.ClaimRef.Name = "baz"
			pvc := createBoundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-pvc"
			pvc.Spec.VolumeName = pv.Name

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).To(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferError))
			checkCompleteFalse(xfer, "PV bound to wrong PVC", "")
		})

		It("Should update PV retain", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			pv.Spec.ClaimRef.Namespace = "target-ns"
			pv.Spec.ClaimRef.Name = "target-pvc"
			pvc := createBoundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-pvc"
			pvc.Spec.VolumeName = pv.Name

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", pv.Name, pv)
			Expect(err).ToNot(HaveOccurred())

			Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(corev1.PersistentVolumeReclaimDelete))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should complete transfer", func() {
			xfer := pvcTransferRunning()
			xfer.Status.Data["pvReclaim"] = "Delete"
			pv := sourcePV()
			pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
			pv.Spec.ClaimRef.Namespace = "target-ns"
			pv.Spec.ClaimRef.Name = "target-pvc"
			pvc := createBoundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-pvc"
			pvc.Spec.VolumeName = pv.Name

			r := createReconciler(xfer, pv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferComplete))
			checkCompleteTrue(xfer)
		})
	})
})

func createPod(pvcName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-ns",
			Name:      "source-pod",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "vol",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
}

func createUnboundPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-ns",
			Name:      "source-pvc",
		},
	}
}

func createBoundPVC() *corev1.PersistentVolumeClaim {
	pvc := createUnboundPVC()
	pvc.Spec.VolumeName = "source-pv"
	pvc.Status.Phase = corev1.ClaimBound
	return pvc
}

func pvcTransfer(phase cdiv1.ObjectTransferPhase) *cdiv1.ObjectTransfer {
	targetNamespace := "target-ns"
	targetName := "target-pvc"
	return &cdiv1.ObjectTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvcTransfer",
			UID:  types.UID("uid-pvcTransfer"),
		},
		Spec: cdiv1.ObjectTransferSpec{
			Source: cdiv1.TransferSource{
				Kind:      "PersistentVolumeClaim",
				Name:      "source-pvc",
				Namespace: "source-ns",
			},
			Target: cdiv1.TransferTarget{
				Namespace: &targetNamespace,
				Name:      &targetName,
			},
		},
		Status: cdiv1.ObjectTransferStatus{
			Phase: phase,
		},
	}
}

func pvcTransferRunning() *cdiv1.ObjectTransfer {
	t := pvcTransfer(cdiv1.ObjectTransferRunning)
	pvc := createBoundPVC()
	pvc.Kind = "PersistentVolumeClaim"
	pvc.APIVersion = "v1"
	pvc.ResourceVersion = "1000"
	pvc.Annotations = map[string]string{
		"cdi.kubevirt.io/objectTransferName": "pvcTransfer",
	}
	pvc.Status = corev1.PersistentVolumeClaimStatus{}
	bs, _ := json.Marshal(pvc)
	t.Status.Data = map[string]string{
		"source": string(bs),
		"pvName": "source-pv",
	}
	return t
}

func sourcePV() *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "source-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Namespace: "source-ns",
				Name:      "source-pvc",
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
		},
	}
}
