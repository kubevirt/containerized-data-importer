package controller

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var (
	csiCloneLog          = log.Log.WithName("csi-clone-controller-test")
	mockStorageClassName = "mockStorageClass"
	mockStorageSize, _   = resource.ParseQuantity("1Gi")
	mockSourcePvc        = corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockSourcePVC",
			Namespace: "mockSourcePVCNamespace",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mockStorageSize,
				},
			},
			StorageClassName: &mockStorageClassName,
		},
	}
	mockDv = cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockDv",
			Namespace: "mockDvNamespace",
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      mockSourcePvc.Name,
					Namespace: mockSourcePvc.Namespace,
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: mockStorageSize,
					},
				},
				StorageClassName: &mockStorageClassName,
			},
		},
	}
	mockPv = corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mockPv",
		},
	}
	mockSourceClonePvc = NewVolumeClonePVC(&mockDv, *mockDv.Spec.PVC.StorageClassName, mockDv.Spec.PVC.AccessModes, CSICloneSourcePVC)
	mockTargetClonePvc = NewVolumeClonePVC(&mockDv, *mockDv.Spec.PVC.StorageClassName, mockDv.Spec.PVC.AccessModes, CSICloneTargetPVC)
)

func createCSICloneReconciler(objects ...runtime.Object) *CSICloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, MakeEmptyCDICR())

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	rec := record.NewFakeRecorder(1)
	cl := fake.NewFakeClientWithScheme(s, objs...)

	return &CSICloneReconciler{
		client:   cl,
		scheme:   s,
		log:      csiCloneLog,
		recorder: rec,
	}
}

var _ = Describe("CSI-clone reconciliation", func() {
	DescribeTable("pvc reconciliation",
		func(annotations map[string]string, phase corev1.PersistentVolumeClaimPhase, controllerKind string, expected bool) {
			isController := true
			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       controllerKind,
							Controller: &isController,
						},
					},
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: phase,
				},
			}
			Expect(shouldReconcileCSIClonePvc(&pvc)).To(Equal(expected))
		},
		Entry("should reconcile with AnnCSICloneRequest = true, claim bound, and controller kind = DataVolume", map[string]string{AnnCSICloneRequest: "true"}, corev1.ClaimBound, "DataVolume", true),
		Entry("should not reconcile with AnnCSICloneRequest unset", map[string]string{}, corev1.ClaimBound, "DataVolume", false),
		Entry("should not reconcile with claim lost", map[string]string{AnnCSICloneRequest: "true"}, corev1.ClaimLost, "DataVolume", false),
		Entry("should not reconcile with incorrect controller reference", map[string]string{AnnCSICloneRequest: "true"}, corev1.ClaimLost, "NotADataVolume", false),
	)
})

var _ = Describe("CSI-clone reconcile loop", func() {
	var (
		reconciler *CSICloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("should return nil if the pvc is not found", func() {
		reconciler = createCSICloneReconciler()
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "no-pvc", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})
	It("should return nil if the pvc is neither annotated AnnCSICloneSource or AnnCSICloneTarget", func() {
		reconciler = createCSICloneReconciler(mockSourcePvc.DeepCopy())
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourcePvc.Name, Namespace: mockSourcePvc.Namespace}})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when a AnnCSICloneSource annotated PVC is reconciled", func() {
		It("should return an error if the datavolume is not found", func() {
			reconciler = createCSICloneReconciler(mockSourceClonePvc.DeepCopy())
			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourceClonePvc.Name, Namespace: mockSourceClonePvc.Namespace}})
			Expect(err).To(HaveOccurred())
		})
		It("should return nil if claim has yet to be bound", func() {
			unboundSourceClonePvc := mockSourceClonePvc.DeepCopy()
			unboundSourceClonePvc.Status.Phase = corev1.ClaimPending
			unboundSourceClonePvc.Spec.VolumeName = ""
			reconciler = createCSICloneReconciler(unboundSourceClonePvc, mockDv.DeepCopy())
			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourceClonePvc.Name, Namespace: mockSourceClonePvc.Namespace}})
			Expect(err).ToNot(HaveOccurred())
		})
		Context("when the source clone pvc loses its claim", func() {
			var claimLostSourceClonePvc *corev1.PersistentVolumeClaim
			BeforeEach(func() {
				claimLostSourceClonePvc = mockSourceClonePvc.DeepCopy()
				claimLostSourceClonePvc.Status.Phase = corev1.ClaimLost
			})

			It("should update datavolume to CloneSourcePVLost if target clone pvc does not exist", func() {
				reconciler = createCSICloneReconciler(claimLostSourceClonePvc, mockDv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourceClonePvc.Name, Namespace: mockSourceClonePvc.Namespace}})
				Expect(err).ToNot(HaveOccurred())
				By("checking datavolume phase")
				dv := &cdiv1.DataVolume{}
				_ = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: mockDv.Name, Namespace: mockDv.Namespace}, dv)
				Expect(dv.Status.Phase).To(Equal(cdiv1.CloneSourcePVLost))
			})
			It("should update datavolume to CloneSourcePVLost if target clone pvc is invalid", func() {
				claimLostTargetPvc := mockTargetClonePvc.DeepCopy()
				claimLostTargetPvc.Status.Phase = corev1.ClaimLost
				reconciler = createCSICloneReconciler(claimLostSourceClonePvc, claimLostTargetPvc, mockDv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourceClonePvc.Name, Namespace: mockSourceClonePvc.Namespace}})
				Expect(err).ToNot(HaveOccurred())
				By("checking datavolume phase")
				dv := &cdiv1.DataVolume{}
				_ = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: mockDv.Name, Namespace: mockDv.Namespace}, dv)
				Expect(dv.Status.Phase).To(Equal(cdiv1.CloneSourcePVLost))
			})
			It("should delete the source clone pvc if clone pvc is valid", func() {
				reconciler = createCSICloneReconciler(claimLostSourceClonePvc, mockTargetClonePvc.DeepCopy(), mockDv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockSourceClonePvc.Name, Namespace: mockSourceClonePvc.Namespace}})
				Expect(err).ToNot(HaveOccurred())
				By("attempting to get source clone pvc")
				pvc := &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: claimLostSourceClonePvc.Name, Namespace: claimLostSourceClonePvc.Namespace}, pvc)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
		Context("when the source clone pvc is claim bound", func() {
			var boundSourcePvc *corev1.PersistentVolumeClaim
			var boundTargetPvc *corev1.PersistentVolumeClaim
			BeforeEach(func() {
				boundSourcePvc = mockSourceClonePvc.DeepCopy()
				boundSourcePvc.Status.Phase = corev1.ClaimBound
				boundSourcePvc.Spec.VolumeName = mockPv.Name
				boundTargetPvc = mockTargetClonePvc.DeepCopy()
				boundTargetPvc.Status.Phase = corev1.ClaimBound
				boundTargetPvc.Spec.VolumeName = mockPv.Name
			})

			It("should return an error if the pv cannot be found", func() {
				reconciler = createCSICloneReconciler(mockDv.DeepCopy(), boundSourcePvc)
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
			It("should return an error if the target clone PVC already exists and it is not valid", func() {
				invalidTargetPvc := mockTargetClonePvc.DeepCopy()
				invalidTargetPvc.OwnerReferences = []metav1.OwnerReference{}
				reconciler = createCSICloneReconciler(mockDv.DeepCopy(), boundSourcePvc, invalidTargetPvc, mockPv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}})
				Expect(k8serrors.IsAlreadyExists(err)).To(BeTrue())
			})
			It("should return nil and delete the source clone pvc if the target clone PVC already exists and it is valid", func() {
				reconciler = createCSICloneReconciler(mockDv.DeepCopy(), boundSourcePvc, boundTargetPvc, mockPv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}})
				Expect(err).ToNot(HaveOccurred())
				By("attempting to get source clone pvc")
				pvc := &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}, pvc)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
			It("should return nil and create a target clone pvc if it does not exist", func() {
				reconciler = createCSICloneReconciler(mockDv.DeepCopy(), boundSourcePvc, mockPv.DeepCopy())
				_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}})
				Expect(err).ToNot(HaveOccurred())
				By("checking the created target pvc")
				pvc := &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: mockTargetClonePvc.Name, Namespace: mockTargetClonePvc.Namespace}, pvc)
				Expect(err).ToNot(HaveOccurred())
				Expect(pvc.Spec.VolumeName).To(Equal(mockPv.Name))
				By("checking the pv")
				pv := &corev1.PersistentVolume{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: mockPv.Name}, pv)
				Expect(err).ToNot(HaveOccurred())
				ref, _ := ref.GetReference(reconciler.scheme, pvc)
				Expect(pv.Spec.ClaimRef).To(Equal(ref))
				By("attempting to get source clone pvc")
				pvc = &corev1.PersistentVolumeClaim{}
				err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: boundSourcePvc.Name, Namespace: boundSourcePvc.Namespace}, pvc)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
	Context("when a AnnCSICloneTarget annotated PVC is reconciled", func() {
		It("should return nil if the claim is not yet bound", func() {
			reconciler = createCSICloneReconciler(mockTargetClonePvc.DeepCopy())
			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockTargetClonePvc.Name, Namespace: mockTargetClonePvc.Namespace}})
			Expect(err).ToNot(HaveOccurred())
		})
		It("should return nil if the claim is bound but the datavolume cannot be found", func() {
			boundTargetPvc := mockTargetClonePvc.DeepCopy()
			boundTargetPvc.Status.Phase = corev1.ClaimBound
			reconciler = createCSICloneReconciler(boundTargetPvc)
			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockTargetClonePvc.Name, Namespace: mockTargetClonePvc.Namespace}})
			Expect(err).ToNot(HaveOccurred())
		})
		It("should update the datavolume phase to PVCBound if the claim is bound", func() {
			boundTargetPvc := mockTargetClonePvc.DeepCopy()
			boundTargetPvc.Status.Phase = corev1.ClaimBound
			reconciler = createCSICloneReconciler(boundTargetPvc, mockDv.DeepCopy())
			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mockTargetClonePvc.Name, Namespace: mockTargetClonePvc.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			dv := &cdiv1.DataVolume{}
			_ = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: mockDv.Name, Namespace: mockDv.Namespace}, dv)
			Expect(dv.Status.Phase).To(Equal(cdiv1.PVCBound))
		})
	})
})
