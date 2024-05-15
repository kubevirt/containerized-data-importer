package populators

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

var (
	forkliftPopulatorLog = logf.Log.WithName("forklift-populator-test")
)

var _ = Describe("Forklift populator tests", func() {
	var (
		reconciler *ForkliftPopulatorReconciler
	)

	const (
		samplePopulatorName = "forklift-populator-test"
		targetPvcName       = "test-forklift-populator-pvc"
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

	ovirtCr := &v1beta1.OvirtVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      samplePopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1beta1.OvirtVolumePopulatorSpec{
			EngineURL: "https://ovirt-engine.example.com",
			SecretRef: "ovirt-engine-secret",
			DiskID:    "12345678-1234-1234-1234-123456789012",
		},
	}

	getPopulatorPod := func(pvc, pvcPrime *corev1.PersistentVolumeClaim) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", populatorPodPrefix, pvc.UID),
				Namespace: metav1.NamespaceDefault,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(pvcPrime, schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "PersistentVolumeClaim",
					}),
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-populate",
						Image: "test-image",
						Ports: []corev1.ContainerPort{
							{
								Name:          "metrics",
								ContainerPort: 12345,
							},
						},
					},
				},
			},
		}
	}

	// Forklift populator's DataSourceRef
	apiGroup := "forklift.konveyor.io"
	dataSourceRef := &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     v1beta1.OvirtVolumePopulatorKind,
		Name:     samplePopulatorName,
	}

	var _ = Describe("Forklift populator reconcile", func() {
		It("should trigger succeeded event when podPhase is succeeded during population", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, make(map[string]string))
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
			populatorPod := getPopulatorPod(targetPvc, pvcPrime)

			populatorPod.Status.Phase = corev1.PodSucceeded
			populatorPod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					RestartCount: 0,
				},
			}
			populatorPod.Spec.Containers = []corev1.Container{
				{Name: "test-populate"},
			}

			By("Reconcile")
			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pv, sc, populatorPod, ovirtCr)

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

		It("Should trigger failed import event when pod phase is pod failed", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: ""}
			populatorPod := getPopulatorPod(targetPvc, pvcPrime)
			populatorPod.Status.Phase = corev1.PodFailed
			populatorPod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					RestartCount: 0,
				},
			}

			By("Reconcile")
			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc, ovirtCr, populatorPod)
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
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{
				AnnPodPhase: "",
			}

			populatorPod := getPopulatorPod(targetPvc, pvcPrime)
			populatorPod.Status.Phase = corev1.PodRunning

			By("Reconcile")
			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc, ovirtCr, populatorPod)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})

		It("Should fail on CR invalid CR kind", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			pvcPrime := getPVCPrime(targetPvc, nil)

			badCr := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       "BadPopulator",
					"apiVersion": "forklift.konveyor.io",
					"metadata": map[string]interface{}{
						"name":      "bad-pop",
						"namespace": metav1.NamespaceDefault,
					},
					"spec": map[string]interface{}{},
				},
			}

			targetPvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     "BadPopulator",
				Name:     "bad-pop",
			}

			By("Reconcile")
			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc, badCr)
			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(HaveOccurred())
			Expect(result).To(Not(BeNil()))
		})
	})

	It("should trigger appropriate event when using AnnPodRetainAfterCompletion", func() {
		targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name,
			map[string]string{AnnPodPhase: string(corev1.PodSucceeded)}, nil, corev1.ClaimPending)
		targetPvc.Spec.DataSourceRef = dataSourceRef
		targetPvc.Spec.VolumeName = "pv"
		pvcPrime := getPVCPrime(targetPvc, nil)
		pvcPrime.Annotations = map[string]string{
			AnnPodRetainAfterCompletion: "true",
			AnnPodPhase:                 string(corev1.PodSucceeded),
		}
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
		reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pv, sc, ovirtCr, getPopulatorPod(targetPvc, pvcPrime))
		result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
		Expect(err).To(Not(HaveOccurred()))
		Expect(result).To(Not(BeNil()))

		By("Checking events recorded")
		close(reconciler.recorder.(*record.FakeRecorder).Events)
		found := false
		for event := range reconciler.recorder.(*record.FakeRecorder).Events {
			if strings.Contains(event, retainedPVCPrime) {
				found = true
			}
		}
		reconciler.recorder = nil
		Expect(found).To(BeTrue())
	})

	var _ = Describe("Forklift populator progress report", func() {
		It("should set 100.0% if pod phase is succeeded", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			pvcPrime := getPVCPrime(targetPvc, nil)

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc)
			err := reconciler.updateImportProgress(string(corev1.PodSucceeded), targetPvc, pvcPrime)
			Expect(err).To(Not(HaveOccurred()))
			Expect(targetPvc.Annotations[AnnPopulatorProgress]).To(Equal("100.0%"))
		})

		It("should set N/A once PVC Prime is bound", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			pvcPrime := getPVCPrime(targetPvc, nil)
			importPodName := fmt.Sprintf("%s-%s", common.ImporterPodName, pvcPrime.Name)
			pvcPrime.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime.Status.Phase = corev1.ClaimBound

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc)
			err := reconciler.updateImportProgress("", targetPvc, pvcPrime)
			Expect(err).To(Not(HaveOccurred()))
			Expect(targetPvc.Annotations[AnnPopulatorProgress]).To(Equal("N/A"))
		})

		It("should return error if no metrics in pod", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			pvcPrime := getPVCPrime(targetPvc, nil)
			importPodName := fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)
			pvcPrime.Annotations = map[string]string{AnnImportPod: importPodName}
			pod := getPopulatorPod(targetPvc, pvcPrime)
			pod.Spec.Containers[0].Ports = nil
			pod.Status.Phase = corev1.PodRunning
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					RestartCount: 0,
				},
			}

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Metrics port not found in pod"))
		})

		It("should not error if no endpoint exists", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime := getPVCPrime(targetPvc, nil)
			pod := getPopulatorPod(targetPvc, pvcPrime)
			pod.Spec.Containers[0].Ports[0].ContainerPort = 12345
			pod.Status.PodIP = "127.0.0.1"
			pod.Status.Phase = corev1.PodRunning

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not error if pod is not running", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimBound)
			importPodName := fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)
			targetPvc.Annotations = map[string]string{AnnImportPod: importPodName}
			pvcPrime := getPVCPrime(targetPvc, nil)
			pod := getPopulatorPod(targetPvc, pvcPrime)

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pod)
			err := reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should report progress in target PVC if http endpoint returns matching data", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.SetUID("b856691e-1038-11e9-a5ab-525500d15501")
			pvcPrime := getPVCPrime(targetPvc, nil)
			importPodName := fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)
			pvcPrime.Annotations = map[string]string{AnnImportPod: importPodName}

			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(fmt.Sprintf("openstack_populator_progress{ownerUID=\"%v\"} 13.45", targetPvc.GetUID())))
				w.WriteHeader(200)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.Atoi(ep.Port())
			Expect(err).ToNot(HaveOccurred())

			pod := getPopulatorPod(targetPvc, pvcPrime)
			pod.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			pod.Status.PodIP = ep.Hostname()
			pod.Status.Phase = corev1.PodRunning

			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pod)
			err = reconciler.updateImportProgress(string(corev1.PodRunning), targetPvc, pvcPrime)
			Expect(err).ToNot(HaveOccurred())
			Expect(targetPvc.Annotations[AnnPopulatorProgress]).To(BeEquivalentTo("13.45%"))
		})
	})
})

func createForkliftPopulatorReconciler(objects ...runtime.Object) *ForkliftPopulatorReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createForkliftPopulatorReconcilerWithoutConfig(objs...)
}

func createForkliftPopulatorReconcilerWithoutConfig(objects ...runtime.Object) *ForkliftPopulatorReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)
	_ = v1beta1.AddToScheme(s)

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
	r := &ForkliftPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          forkliftPopulatorLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
		},
		ovirtPopulatorImage: "ovirt-populator-image",
	}

	return r
}
