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

	. "github.com/onsi/ginkgo/v2"
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
		Expect(pvcPrime.GetLabels()[cc.LabelExcludeFromVeleroBackup]).To(Equal("true"))

		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}, updatedPVC)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPVC.GetAnnotations()).ToNot(BeNil())
		Expect(updatedPVC.GetAnnotations()[cc.AnnPVCPrimeName]).To(Equal(pvcPrime.Name))
	},
		Entry("kubevirt content type", "kubevirt", false),
		Entry("kubevirt content type with preallocation", "kubevirt", true),
		Entry("archive content type", "archive", false),
	)

	It("should set event if upload pod failed", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		cc.AddAnnotation(pvc, cc.AnnPVCPrimeName, PVCPrimeName(pvc))
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		cc.AddAnnotation(pvcPrime, cc.AnnPodPhase, string(corev1.PodFailed))
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
		cc.AddAnnotation(pvc, cc.AnnPVCPrimeName, PVCPrimeName(pvc))
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())

		cc.AddAnnotation(pvcPrime, cc.AnnPodPhase, string(corev1.PodSucceeded))
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
		Expect(updatedPVC.GetAnnotations()[cc.AnnPVCPrimeName]).To(BeEmpty())

		expectEvent(r, uploadSucceeded)
	})

	It("should clean PVCPrime when targetPVC bound and succeeded", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		pvc.Spec.VolumeName = "test-pv"
		cc.AddAnnotation(pvc, cc.AnnPodPhase, string(corev1.PodSucceeded))
		pvcPrime := newUploadPopulatorPVC(PVCPrimeName(pvc))
		pvc.Status = corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}
		pvcPrime.Status = corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimLost}

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

	It("should always remove the PVCPrimeName annotation once the pod succeeds", func() {
		pvc := newUploadPopulatorPVC("test-pvc")
		pvc.Spec.VolumeName = "test-pv"
		pvcPrime := newUploadPopulatorPVC(PVCPrimeName(pvc))
		cc.AddAnnotation(pvcPrime, cc.AnnPodPhase, string(corev1.PodSucceeded))
		cc.AddAnnotation(pvcPrime, cc.AnnPVCPrimeName, pvcPrime.Name)

		r := createUploadPopulatorReconciler(pvc, pvcPrime)
		pvc, err := r.updatePVCWithPVCPrimeAnnotations(pvc, pvcPrime, r.updateUploadAnnotations)
		Expect(err).ToNot(HaveOccurred())

		//should always remove the pvc prime annotation
		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, updatedPVC)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPVC.GetAnnotations()[cc.AnnPVCPrimeName]).To(BeEmpty())
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

		cc.AddAnnotation(pvc, cc.AnnSelectedNode, "node01")
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

	DescribeTable("should update target pvc with desired annotations from pvc prime", func(podPhase string) {
		pvc := newUploadPopulatorPVC("test-pvc")
		cc.AddAnnotation(pvc, cc.AnnPVCPrimeName, PVCPrimeName(pvc))
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)

		_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).ToNot(HaveOccurred())
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())

		pvcPrime.Spec.VolumeName = "test-pv"
		pvcPrime.UID = pvcPrimeUID
		for _, ann := range desiredAnnotations {
			cc.AddAnnotation(pvcPrime, ann, "somevalue")
		}
		cc.AddAnnotation(pvcPrime, cc.AnnPodPhase, podPhase)
		cc.AddAnnotation(pvcPrime, "undesiredAnn", "somevalue")
		err = r.client.Update(context.TODO(), pvcPrime)
		Expect(err).ToNot(HaveOccurred())

		By("Reconcile")
		result, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(Not(HaveOccurred()))
		Expect(result).ToNot(BeNil())

		updatedPVC := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}, updatedPVC)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPVC.GetAnnotations()).ToNot(BeNil())
		for _, ann := range desiredAnnotations {
			_, ok := updatedPVC.Annotations[ann]
			Expect(ok).To(BeTrue())
		}
		_, ok := updatedPVC.Annotations["undesiredAnn"]
		Expect(ok).To(BeFalse())
	},
		Entry("with pod running phase", string(corev1.PodRunning)),
		Entry("with pod succeeded phase", string(corev1.PodFailed)),
		Entry("with pod succeeded phase", string(corev1.PodSucceeded)),
	)

	DescribeTable("Should create PVC Prime with proper upload annotations", func(key, value, expectedValue string) {
		pvc := newUploadPopulatorPVC("test-pvc")
		cc.AddAnnotation(pvc, cc.AnnPVCPrimeName, PVCPrimeName(pvc))
		uploadPV := uploadPV(pvc)

		volumeUploadSourceCR := newUploadPopulatorCR("", false)
		scName := "test-sc"
		sc := cc.CreateStorageClassWithProvisioner(scName, map[string]string{cc.AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
		pvc.Annotations[key] = value

		By("Reconcile")
		r := createUploadPopulatorReconciler(pvc, volumeUploadSourceCR, sc, uploadPV)
		result, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-pvc", Namespace: metav1.NamespaceDefault}})
		Expect(err).To(Not(HaveOccurred()))
		Expect(result).To(Not(BeNil()))

		By("Checking events recorded")
		close(r.recorder.(*record.FakeRecorder).Events)
		found := false
		for event := range r.recorder.(*record.FakeRecorder).Events {
			if strings.Contains(event, createdPVCPrimeSuccessfully) {
				found = true
			}
		}
		r.recorder = nil
		Expect(found).To(BeTrue())

		By("Checking PVC' annotations")
		pvcPrime, err := r.getPVCPrime(pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvcPrime).ToNot(BeNil())
		// make sure we didnt inflate size
		Expect(pvcPrime.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
		Expect(pvcPrime.GetAnnotations()).ToNot(BeNil())
		Expect(pvcPrime.GetAnnotations()[cc.AnnImmediateBinding]).To(Equal(""))
		Expect(pvcPrime.GetAnnotations()[cc.AnnUploadRequest]).To(Equal(""))
		Expect(pvcPrime.GetAnnotations()[cc.AnnPopulatorKind]).To(Equal(cdiv1.VolumeUploadSourceRef))
		Expect(pvcPrime.Annotations[key]).To(Equal(expectedValue))
		Expect(pvcPrime.GetLabels()[cc.LabelExcludeFromVeleroBackup]).To(Equal("true"))
	},
		Entry("No extra annotations", "", "", ""),
		Entry("Invalid extra annotation is not passed", "invalid", "test", ""),
		Entry("Priority class is passed", cc.AnnPriorityClassName, "test", "test"),
		Entry("pod network is passed", cc.AnnPodNetwork, "test", "test"),
		Entry("istio side car injection is passed", cc.AnnPodSidecarInjectionIstio, cc.AnnPodSidecarInjectionIstioDefault, cc.AnnPodSidecarInjectionIstioDefault),
		Entry("linkerd side car injection is passed", cc.AnnPodSidecarInjectionLinkerd, cc.AnnPodSidecarInjectionLinkerdDefault, cc.AnnPodSidecarInjectionLinkerdDefault),
		Entry("multus default network is passed", cc.AnnPodMultusDefaultNetwork, "test", "test"),
	)
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
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
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
