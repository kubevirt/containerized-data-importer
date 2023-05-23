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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	dvPopulatorLog = logf.Log.WithName("import-populator-test")
)

var _ = Describe("Import populator tests", func() {
	var (
		reconciler *ImportPopulatorReconciler
	)

	const (
		samplePopulatorName = "import-populator-test"
		targetPvcName       = "test-import-populator-pvc"
		scName              = "testsc"
	)

	AfterEach(func() {
		if reconciler != nil && reconciler.recorder != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
		}
	})

	sc := CreateStorageClassWithProvisioner(scName, map[string]string{
		AnnDefaultStorageClass: "true",
	}, map[string]string{}, "csi-plugin")

	getVolumeImportSource := func(preallocation bool, namespace string) *cdiv1.VolumeImportSource {
		return &cdiv1.VolumeImportSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      samplePopulatorName,
				Namespace: namespace,
			},
			Spec: cdiv1.VolumeImportSourceSpec{
				ContentType:   cdiv1.DataVolumeKubeVirt,
				Preallocation: &preallocation,
				Source: &cdiv1.ImportSourceType{
					HTTP: &cdiv1.DataVolumeSourceHTTP{
						URL: "http://example.com/data",
					},
				},
			},
		}
	}

	getPVCPrime := func(pvc *corev1.PersistentVolumeClaim, annotations map[string]string) *corev1.PersistentVolumeClaim {
		pvcPrime := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        PVCPrimeName(pvc),
				Namespace:   pvc.Namespace,
				Annotations: annotations,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      pvc.Spec.AccessModes,
				Resources:        pvc.Spec.Resources,
				StorageClassName: pvc.Spec.StorageClassName,
				VolumeMode:       pvc.Spec.VolumeMode,
			},
		}
		pvcPrime.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(pvc, schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PersistentVolumeClaim",
			}),
		}
		return pvcPrime
	}

	// Import populator's DataSourceRef
	apiGroup := AnnAPIGroup
	dataSourceRef := &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     cdiv1.VolumeImportSourceRef,
		Name:     samplePopulatorName,
	}
	nsName := "test-import"
	namespacedDataSourceRef := &corev1.TypedObjectReference{
		APIGroup:  &apiGroup,
		Kind:      cdiv1.VolumeImportSourceRef,
		Name:      samplePopulatorName,
		Namespace: &nsName,
	}

	var _ = Describe("Import populator reconcile", func() {
		It("should trigger succeeded event when podPhase is succeeded during population", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			volumeImportSource := getVolumeImportSource(true, metav1.NamespaceDefault)
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodSucceeded)}
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
				},
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef: &corev1.ObjectReference{
						Namespace: pvcPrime.Namespace,
						Name:      pvcPrime.Name,
					},
				},
			}
			pvcPrime.Spec.VolumeName = pv.Name

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, pv, volumeImportSource, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, importSucceeded) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("should ignore namespaced dataSourceRefs", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = namespacedDataSourceRef
			volumeImportSource := getVolumeImportSource(true, nsName)
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodSucceeded)}

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, volumeImportSource, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Checking PVC was ignored")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, importSucceeded) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeFalse())
		})

		It("Should trigger failed import event when pod phase is podfailed", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			volumeImportSource := getVolumeImportSource(true, metav1.NamespaceDefault)
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodFailed)}

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, volumeImportSource, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, importFailed) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())
		})

		It("Should retrigger reconcile while import pod is running", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			volumeImportSource := getVolumeImportSource(true, metav1.NamespaceDefault)
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodRunning)}

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, volumeImportSource, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("Should create PVC Prime with proper import annotations", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			volumeImportSource := getVolumeImportSource(true, metav1.NamespaceDefault)

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, volumeImportSource, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Checking events recorded")
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			found := false
			for event := range reconciler.recorder.(*record.FakeRecorder).Events {
				if strings.Contains(event, createdPVCPrimeSuccessfully) {
					found = true
				}
			}
			reconciler.recorder = nil
			Expect(found).To(BeTrue())

			By("Checking PVC' annotations")
			pvcPrime, err := reconciler.getPVCPrime(targetPvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcPrime).ToNot(BeNil())
			Expect(pvcPrime.GetAnnotations()).ToNot(BeNil())
			Expect(pvcPrime.GetAnnotations()[AnnImmediateBinding]).To(Equal(""))
			Expect(pvcPrime.GetAnnotations()[AnnUploadRequest]).To(Equal(""))
			Expect(pvcPrime.GetAnnotations()[AnnPopulatorKind]).To(Equal(cdiv1.VolumeImportSourceRef))
			Expect(pvcPrime.GetAnnotations()[AnnPreallocationRequested]).To(Equal("true"))
			Expect(pvcPrime.GetAnnotations()[AnnEndpoint]).To(Equal("http://example.com/data"))
			Expect(pvcPrime.GetAnnotations()[AnnSource]).To(Equal(SourceHTTP))
		})

		It("shouldn't error when reconciling PVC with non-import DataSourceRef", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			targetPvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     "BadPopulator",
				Name:     "badPopulator",
			}

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))
		})

		It("shouldn't error when reconciling PVC without DataSourceRef", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-dv", Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))
		})

		It("Should just return when VolumeImportSource is not available", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			targetPvc.Spec.DataSourceRef = dataSourceRef

			By("Reconcile")
			reconciler = createImportPopulatorReconciler(targetPvc, sc)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))
		})
	})

	var _ = Describe("Import populator progress report", func() {
		It("should set 100.0% if pod phase is succeeded", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			pvcPrime := getPVCPrime(targetPvc, nil)

			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, sc)
			err := reconciler.updateImportProgress(string(corev1.PodSucceeded), targetPvc, pvcPrime)
			Expect(err).To(Not(HaveOccurred()))
			Expect(targetPvc.Annotations[AnnPopulatorProgress]).To(Equal("100.0%"))
		})

		It("should return error if no metrics in pod", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", common.ImporterPodName, targetPvc.Name)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime := getPVCPrime(targetPvc, nil)
			pod := CreateImporterTestPod(targetPvc, pvcPrime.Name, nil)
			pod.Spec.Containers[0].Ports = nil
			pod.Status.Phase = corev1.PodRunning

			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Metrics port not found in pod"))
		})

		It("should not error if no endpoint exists", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", common.ImporterPodName, targetPvc.Name)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime := getPVCPrime(targetPvc, nil)
			pod := CreateImporterTestPod(targetPvc, pvcPrime.Name, nil)
			pod.Spec.Containers[0].Ports[0].ContainerPort = 12345
			pod.Status.PodIP = "127.0.0.1"
			pod.Status.Phase = corev1.PodRunning

			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not error if pod is not running", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", common.ImporterPodName, targetPvc.Name)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime := getPVCPrime(targetPvc, nil)
			pod := CreateImporterTestPod(targetPvc, pvcPrime.Name, nil)

			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should report progress in target PVC if http endpoint returns matching data", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", common.ImporterPodName, targetPvc.Name)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			targetPvc.SetUID("b856691e-1038-11e9-a5ab-525500d15501")
			pvcPrime := getPVCPrime(targetPvc, nil)

			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(fmt.Sprintf("import_progress{ownerUID=\"%v\"} 13.45", targetPvc.GetUID())))
				w.WriteHeader(200)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.Atoi(ep.Port())
			Expect(err).ToNot(HaveOccurred())

			pod := CreateImporterTestPod(targetPvc, pvcPrime.Name, nil)
			pod.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			pod.Status.PodIP = ep.Hostname()
			pod.Status.Phase = corev1.PodRunning

			reconciler = createImportPopulatorReconciler(targetPvc, pvcPrime, pod)
			err = reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
			Expect(targetPvc.Annotations[AnnPopulatorProgress]).To(BeEquivalentTo("13.45%"))
		})
	})
})

func createImportPopulatorReconciler(objects ...runtime.Object) *ImportPopulatorReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createImportPopulatorReconcilerWithoutConfig(objs...)
}

func createImportPopulatorReconcilerWithoutConfig(objects ...runtime.Object) *ImportPopulatorReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	// Create a fake client to mock API calls.
	builder := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...)

	for _, ia := range getIndexArgs() {
		builder = builder.WithIndex(ia.obj, ia.field, ia.extractValue)
	}

	cl := builder.Build()

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ImportPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvPopulatorLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
			sourceKind: cdiv1.VolumeImportSourceRef,
		},
	}
	return r
}
