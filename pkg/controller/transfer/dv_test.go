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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("DataVolume Transfer Tests", func() {

	var _ = Describe("DataVolume Transfer Tests", func() {
		It("Should handle no source", func() {

			xfer := dvTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "No source", "")
		})

		It("Should handle source not populated", func() {
			xfer := dvTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer, createUnpopulatedDV())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "Source not populated", "")
		})

		It("Should handle pod using PVC", func() {
			xfer := dvTransfer(cdiv1.ObjectTransferPending)
			dv := createPopulatedDV()
			pvc := createBoundPVC()
			pvc.Name = dv.Name

			r := createReconciler(xfer, dv, pvc, createPod(pvc.Name))
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferPending))
			checkCompleteFalse(xfer, "Pods using DataVolume PVC", "")
		})

		It("Should become running", func() {
			xfer := dvTransfer(cdiv1.ObjectTransferPending)

			r := createReconciler(xfer, createPopulatedDV())
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			Expect(xfer.Status.Data).To(Equal(dvTransferRunning().Status.Data))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should handle DV has no PVC", func() {
			xfer := dvTransferRunning()
			dv := createPopulatedDV()

			r := createReconciler(xfer, dv)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Source DV has no PVC", "")
		})

		It("Should update PVC and delete DV", func() {
			xfer := dvTransferRunning()
			dv := createPopulatedDV()
			pvc := createBoundPVC()
			pvc.Name = dv.Name
			pvc.OwnerReferences = []metav1.OwnerReference{
				{
					Kind: "DataVolume",
					Name: dv.Name,
					UID:  dv.UID,
				},
			}

			r := createReconciler(xfer, dv, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", dv.Name, dv)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			err = getResource(r.Client, "", pvc.Name, pvc)
			Expect(err).To(HaveOccurred())

			Expect(pvc.OwnerReferences).To(HaveLen(0))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Running", "")
		})

		It("Should create PVC transfer", func() {
			xfer := dvTransferRunning()
			pvcTransfer := &cdiv1.ObjectTransfer{}

			r := createReconciler(xfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", "pvc-transfer-uid-dvTransfer", pvcTransfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(pvcTransfer.Spec.Source.Kind).To(Equal("PersistentVolumeClaim"))
			Expect(pvcTransfer.Spec.Source.Name).To(Equal("source-dv"))
			Expect(pvcTransfer.Spec.Source.Namespace).To(Equal("source-ns"))
			Expect(*pvcTransfer.Spec.Target.Name).To(Equal("target-dv"))
			Expect(*pvcTransfer.Spec.Target.Namespace).To(Equal("target-ns"))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "PVC transfer in progress", "")
		})

		It("Should handle target PVC does not exist", func() {
			xfer := dvTransferRunning()
			pvcTransfer := internalPVCTransfer(xfer)

			r := createReconciler(xfer, pvcTransfer)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferError))
			checkCompleteFalse(xfer, "Transferred PVC does not exist", "")
		})

		It("Should add annotation to PVC and create target DV", func() {
			xfer := dvTransferRunning()
			pvcTransfer := internalPVCTransfer(xfer)
			pvc := createBoundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-dv"
			dv := &cdiv1.DataVolume{}

			r := createReconciler(xfer, pvcTransfer, pvc)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, pvc.Namespace, pvc.Name, pvc)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, pvc.Namespace, pvc.Name, dv)
			Expect(err).ToNot(HaveOccurred())

			Expect(pvc.Annotations["cdi.kubevirt.io/storage.populatedFor"]).To(Equal("target-dv"))
			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferRunning))
			checkCompleteFalse(xfer, "Waiting for target DataVolume", "")
		})

		It("Should delete PVC transfer and complete", func() {
			xfer := dvTransferRunning()
			pvcTransfer := internalPVCTransfer(xfer)
			pvc := createBoundPVC()
			pvc.Namespace = "target-ns"
			pvc.Name = "target-dv"
			pvc.Annotations = map[string]string{
				"cdi.kubevirt.io/storage.populatedFor": "target-dv",
			}
			dv := createPopulatedDV()
			dv.Name = "target-dv"
			dv.Namespace = "target-ns"

			r := createReconciler(xfer, pvcTransfer, pvc, dv)
			_, err := r.Reconcile(context.TODO(), rr(xfer.Name))
			Expect(err).ToNot(HaveOccurred())

			err = getResource(r.Client, "", xfer.Name, xfer)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, dv.Namespace, dv.Name, dv)
			Expect(err).ToNot(HaveOccurred())
			err = getResource(r.Client, "", pvcTransfer.Name, pvcTransfer)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())

			Expect(xfer.Status.Phase).To(Equal(cdiv1.ObjectTransferComplete))
			checkCompleteTrue(xfer)
		})
	})
})

func internalPVCTransfer(dvTransfer *cdiv1.ObjectTransfer) *cdiv1.ObjectTransfer {
	return &cdiv1.ObjectTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-transfer-" + string(dvTransfer.UID),
		},
		Spec: cdiv1.ObjectTransferSpec{
			Source: cdiv1.TransferSource{
				Kind:      "PersistentVolumeClaim",
				Name:      dvTransfer.Spec.Source.Name,
				Namespace: dvTransfer.Spec.Source.Namespace,
			},
			Target: cdiv1.TransferTarget{
				Namespace: dvTransfer.Spec.Target.Namespace,
				Name:      dvTransfer.Spec.Target.Name,
			},
		},
		Status: cdiv1.ObjectTransferStatus{
			Phase: cdiv1.ObjectTransferComplete,
		},
	}
}

func createUnpopulatedDV() *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-ns",
			Name:      "source-dv",
		},
	}
}

func createPopulatedDV() *cdiv1.DataVolume {
	dv := createUnpopulatedDV()
	dv.Status.Phase = cdiv1.Succeeded
	return dv
}

func dvTransferRunning() *cdiv1.ObjectTransfer {
	t := dvTransfer(cdiv1.ObjectTransferRunning)
	dv := createPopulatedDV()
	dv.Kind = "DataVolume"
	dv.APIVersion = "cdi.kubevirt.io/v1beta1"
	dv.ResourceVersion = "1000"
	dv.Annotations = map[string]string{
		"cdi.kubevirt.io/objectTransferName": "dvTransfer",
	}
	dv.Status = cdiv1.DataVolumeStatus{}
	bs, _ := json.Marshal(dv)
	t.Status.Data = map[string]string{
		"source":  string(bs),
		"pvcName": dv.Name,
	}
	return t
}

func dvTransfer(phase cdiv1.ObjectTransferPhase) *cdiv1.ObjectTransfer {
	targetNamespace := "target-ns"
	targetName := "target-dv"
	return &cdiv1.ObjectTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dvTransfer",
			UID:  types.UID("uid-dvTransfer"),
		},
		Spec: cdiv1.ObjectTransferSpec{
			Source: cdiv1.TransferSource{
				Kind:      "DataVolume",
				Name:      "source-dv",
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
