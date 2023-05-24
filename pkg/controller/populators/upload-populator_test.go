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

package populators

import (
	"context"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

const (
	testPopulatorName = "upload-populator-test"
	testStorageClass  = "test-sc"
	pvcPrimeUID       = "pvcPrimeUID"
)

var (
	uploadPopLog = logf.Log.WithName("upload-populator-controller-test")

	scName = "test-sc"
)

var _ = Describe("Datavolume controller reconcile loop", func() {
	DescribeTable("should create PVC prime", func(contentType string, preallocation bool) {
		pvc := newUploadPopulatorPVC("test-pvc")
		volumeUploadSourceCR := newUploadPopulatorCR(contentType, preallocation)
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc)
		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		expectEvent(r, createdPVCPrimeSuccessfully)

		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvcPrime).ToNot(BeNil())
		Expect(pvcPrime.GetAnnotations()).ToNot(BeNil())
		Expect(pvcPrime.GetAnnotations()[cc.AnnImmediateBinding]).To(Equal(""))
		Expect(pvcPrime.GetAnnotations()[cc.AnnUploadRequest]).To(Equal(""))
		Expect(pvcPrime.GetAnnotations()[cc.AnnContentType]).To(Equal(contentType))
		Expect(pvcPrime.GetAnnotations()[cc.AnnPreallocationRequested]).To(Equal(strconv.FormatBool(preallocation)))
		Expect(pvcPrime.GetAnnotations()[cc.AnnPopulatorKind]).To(Equal(cdiv1.VolumeUploadSourceRef))

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}, updatedPVC)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPVC.GetAnnotations()).ToNot(BeNil())
		Expect(updatedPVC.GetAnnotations()[AnnPVCPrimeName]).To(Equal(pvcPrime.Name))
	},
		Entry("kubevirt content type", "kubevirt", false),
		Entry("kubevirt content type with preallocation", "kubevirt", true),
		Entry("archive content type", "archive", false),
	)

	It("should set event if upload pod failed", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		pvc.Annotations = make(map[string]string)
		pvc.Annotations[AnnPVCPrimeName] = PVCPrimeName(pvc)
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		pvcPrime.Annotations[cc.AnnPodPhase] = string(corev1.PodFailed)
		pvcPrime.Spec.VolumeName = "test-pv"
		pvcPrime.UID = pvcPrimeUID
		err = r.client.Update(context.TODO(), pvcPrime)
		Expect(err).ToNot(HaveOccurred())

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		updatedPV, err := getPV(r.client, pvcPrime.Spec.VolumeName)
		Expect(err).ToNot(HaveOccurred())

		//expect bind to remain to pvc'
		Expect(updatedPV.Spec.ClaimRef.Name).To(Equal(pvcPrime.Name))

		expectEvent(r, errUploadFailed)
	})

	It("should rebind PV to target PVC", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		pvc.Annotations = make(map[string]string)
		pvc.Annotations[AnnPVCPrimeName] = PVCPrimeName(pvc)
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		pvcPrime.Annotations[cc.AnnPodPhase] = string(corev1.PodSucceeded)
		pvcPrime.Spec.VolumeName = "test-pv"
		pvcPrime.UID = pvcPrimeUID
		err = r.client.Update(context.TODO(), pvcPrime)
		Expect(err).ToNot(HaveOccurred())

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		updatedPV, err := getPV(r.client, pvcPrime.Spec.VolumeName)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedPV.Spec.ClaimRef.Name).To(Equal("test-pvc"))

		//should remove the pvc prime annotation when upload completes
		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}, updatedPVC)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPVC.GetAnnotations()).To(BeNil())

		expectEvent(r, uploadSucceeded)
	})

	It("should clean PVCPrime when targetPVC bound", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		pvc.Spec.VolumeName = "test-pv"
		pvcPrime := newUploadPopulatorPVC(PVCPrimeName(pvc))

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, pvcPrime)

		pvcPrime = &corev1.PersistentVolumeClaim{}
		pvcPrimeKey := types.NamespacedName{Namespace: pvc.Namespace, Name: PVCPrimeName(pvc)}
		err := r.client.Get(context.TODO(), pvcPrimeKey, pvcPrime)
		Expect(err).ToNot(HaveOccurred())

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime = &corev1.PersistentVolumeClaim{}
		pvcPrimeKey = types.NamespacedName{Namespace: pvc.Namespace, Name: PVCPrimeName(pvc)}
		err = r.client.Get(context.TODO(), pvcPrimeKey, pvcPrime)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should wait for selected node annotation in case of wffc", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		pvc.Spec.StorageClassName = &scName
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		bindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		sc.VolumeBindingMode = &bindingMode

		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		// until added selected node annotation upload
		// process shouldn't start
		pvcPrime := &corev1.PersistentVolumeClaim{}
		pvcPrimeKey := types.NamespacedName{Namespace: pvc.Namespace, Name: PVCPrimeName(pvc)}
		err = r.client.Get(context.TODO(), pvcPrimeKey, pvcPrime)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		pvc.Annotations = make(map[string]string)
		pvc.Annotations[cc.AnnSelectedNode] = "node01"
		err = r.client.Update(context.TODO(), pvc)
		Expect(err).ToNot(HaveOccurred())

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())

		expectEvent(r, createdPVCPrimeSuccessfully)

		pvcPrime, err = r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvcPrime).ToNot(BeNil())
		Expect(pvcPrime.Annotations[cc.AnnSelectedNode]).To(Equal("node01"))
	})
})

func newUploadPopulatorPVC(name string) *corev1.PersistentVolumeClaim {
	apiGroup := cc.AnnAPIGroup
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1Gi"),
				},
			},
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     cdiv1.VolumeUploadSourceRef,
				Name:     testPopulatorName,
			},
		},
	}
}

func newUploadPopulatorCR(contentType string, preallocation bool) *cdiv1.VolumeUploadSource {
	return &cdiv1.VolumeUploadSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.VolumeUploadSourceSpec{
			ContentType:   cdiv1.DataVolumeContentType(contentType),
			Preallocation: &preallocation,
		},
	}
}

func uploadPV(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{
				Namespace: metav1.NamespaceDefault,
				Name:      PVCPrimeName(pvc),
				UID:       pvcPrimeUID,
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
		},
	}
}

func expectEvent(r *UploadPopulatorReconciler, expectedEvent string) {
	close(r.recorder.(*record.FakeRecorder).Events)
	found := false
	for event := range r.recorder.(*record.FakeRecorder).Events {
		By(event)
		if strings.Contains(event, expectedEvent) {
			found = true
		}
	}
	Expect(found).To(BeTrue())
}

func createUploadPopulatorReconciler(objects ...runtime.Object) *UploadPopulatorReconciler {
	cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, cc.MakeEmptyCDICR())

	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &UploadPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          uploadPopLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
			sourceKind: cdiv1.VolumeUploadSourceRef,
		},
	}
	return r
}

func getPV(c client.Client, name string) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name}, pv); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pv, nil
}
