package populators

import (
	"context"
	"fmt"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	forkliftPopulatorLog = logf.Log.WithName("forklift-populator-test")
)

var _ = Describe("Import populator tests", func() {
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
			EngineURL:        "https://ovirt-engine.example.com",
			EngineSecretName: "ovirt-engine-secret",
			DiskID:           "12345678-1234-1234-1234-123456789012",
		},
	}

	populatorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      samplePopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.PodSpec{},
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
			populatorPod.Status.Phase = corev1.PodSucceeded
			populatorPod.Spec.Containers = []corev1.Container{
				{Name: fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)},
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

		It("Should trigger failed import event when pod phase is podfailed", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, nil)
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodFailed)}

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
			pvcPrime.Annotations = map[string]string{AnnPodPhase: string(corev1.PodRunning)}

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
		reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, pv, sc, ovirtCr, populatorPod)
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
	}

	return r
}
