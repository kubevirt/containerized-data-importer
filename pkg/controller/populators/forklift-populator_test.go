package populators

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

var (
	forkliftPopulatorLog = logf.Log.WithName("forklift-populator-test")
)

type fakeCertGenerator struct {
	cert, key []byte
	err       error

	serverCalls  int
	gotNamespace string
	gotName      string
	gotDuration  time.Duration
}

func (cg *fakeCertGenerator) MakeClientCert(name string, groups []string, duration time.Duration) ([]byte, []byte, error) {
	return cg.cert, cg.key, cg.err
}

func (cg *fakeCertGenerator) MakeServerCert(namespace, service string, duration time.Duration) ([]byte, []byte, error) {
	cg.serverCalls++
	cg.gotNamespace = namespace
	cg.gotName = service
	cg.gotDuration = duration
	return cg.cert, cg.key, cg.err
}

var _ = Describe("Forklift populator tests", func() {
	var (
		reconciler *ForkliftPopulatorReconciler
		recorder   *record.FakeRecorder
	)

	BeforeEach(func() {
		recorder = nil
	})

	AfterEach(func() {
		if recorder != nil {
			close(recorder.Events)
		}
	})

	const (
		samplePopulatorName = "forklift-populator-test"
		targetPvcName       = "test-forklift-populator-pvc"
		scName              = "testsc"
	)

	BeforeEach(func() {
		recorder = nil
	})

	AfterEach(func() {
		if recorder != nil {
			close(recorder.Events)
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
				Finalizers:  []string{"test/finalizer"},
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
				EnableServiceLinks: ptr.To(false),
			},
		}
	}

	// Forklift populator's DataSourceRef
	apiGroup := "forklift.cdi.kubevirt.io"
	dataSourceRef := &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     v1beta1.OvirtVolumePopulatorKind,
		Name:     samplePopulatorName,
	}

	openstackCr := &v1beta1.OpenstackVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      samplePopulatorName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1beta1.OpenstackVolumePopulatorSpec{
			IdentityURL: "https://openstack.example.com:5000/v3",
			SecretRef:   "openstack-secret",
			ImageID:     "abcdef01-2345-6789-abcd-ef0123456789",
		},
	}

	openstackDataSourceRef := &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     v1beta1.OpenstackVolumePopulatorKind,
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
					"apiVersion": "forklift.cdi.kubevirt.io",
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

	var _ = Describe("Forklift populator prometheus certificate", func() {
		const (
			certData = "the-cert"
			keyData  = "the-key"
		)

		serverCertConfig := &cdiv1.CertConfig{
			Duration:    &metav1.Duration{Duration: 12 * time.Hour},
			RenewBefore: &metav1.Duration{Duration: 3 * time.Hour},
		}

		newPendingPod := func(targetPvc, pvcPrime *corev1.PersistentVolumeClaim, phase corev1.PodPhase) *corev1.Pod {
			pod := getPopulatorPod(targetPvc, pvcPrime)
			pod.UID = "populator-pod-uid"
			pod.Status.Phase = phase
			return pod
		}

		DescribeTable("should create the certificate Secret owned by the pod",
			func(phase corev1.PodPhase) {
				targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
				targetPvc.Spec.DataSourceRef = dataSourceRef
				pvcPrime := getPVCPrime(targetPvc, make(map[string]string))
				pod := newPendingPod(targetPvc, pvcPrime, phase)
				cg := &fakeCertGenerator{cert: []byte(certData), key: []byte(keyData)}

				reconciler = createForkliftPopulatorReconcilerWithCertConfig(cg, serverCertConfig, targetPvc, pvcPrime, sc, ovirtCr, pod)

				result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(2 * time.Second))

				secret := &corev1.Secret{}
				Expect(reconciler.client.Get(context.TODO(), types.NamespacedName{
					Name:      PrometheusCertSecretName(pod.Name),
					Namespace: metav1.NamespaceDefault,
				}, secret)).To(Succeed())

				Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSCertKey, []byte(certData)))
				Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSPrivateKeyKey, []byte(keyData)))
				Expect(secret.OwnerReferences).To(HaveLen(1))
				Expect(secret.OwnerReferences[0].Name).To(Equal(pod.Name))
				Expect(secret.OwnerReferences[0].UID).To(Equal(pod.UID))

				Expect(cg.serverCalls).To(Equal(1))
				Expect(cg.gotNamespace).To(Equal(pod.Namespace))
				Expect(cg.gotName).To(Equal(pod.Name))
				Expect(cg.gotDuration).To(Equal(serverCertConfig.Duration.Duration))
			},
			Entry("when the pod is pending", corev1.PodPending),
			Entry("when the pod phase is empty", corev1.PodPhase("")),
		)

		It("should create the Secret and reach the pending branch when the pod has container statuses", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, map[string]string{AnnPodPhase: string(corev1.PodPending)})
			pod := newPendingPod(targetPvc, pvcPrime, corev1.PodPending)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "ContainerCreating",
					},
				},
			}}
			cg := &fakeCertGenerator{cert: []byte(certData), key: []byte(keyData)}

			reconciler = createForkliftPopulatorReconcilerWithCertConfig(cg, serverCertConfig, targetPvc, pvcPrime, sc, ovirtCr, pod)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(2 * time.Second))

			secret := &corev1.Secret{}
			Expect(reconciler.client.Get(context.TODO(), types.NamespacedName{
				Name:      PrometheusCertSecretName(pod.Name),
				Namespace: metav1.NamespaceDefault,
			}, secret)).To(Succeed())

			Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSCertKey, []byte(certData)))
			Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSPrivateKeyKey, []byte(keyData)))
			Expect(secret.OwnerReferences).To(HaveLen(1))
			Expect(secret.OwnerReferences[0].Name).To(Equal(pod.Name))
			Expect(secret.OwnerReferences[0].UID).To(Equal(pod.UID))

			Expect(cg.serverCalls).To(Equal(1))
			Expect(cg.gotNamespace).To(Equal(pod.Namespace))
			Expect(cg.gotName).To(Equal(pod.Name))
			Expect(cg.gotDuration).To(Equal(serverCertConfig.Duration.Duration))
		})

		It("should not create a Secret or invoke the generator for a running pod", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, map[string]string{AnnPodPhase: string(corev1.PodRunning)})
			pod := newPendingPod(targetPvc, pvcPrime, corev1.PodRunning)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{RestartCount: 0}}
			cg := &fakeCertGenerator{cert: []byte(certData), key: []byte(keyData)}

			reconciler = createForkliftPopulatorReconcilerWithCertConfig(cg, serverCertConfig, targetPvc, pvcPrime, sc, ovirtCr, pod)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			Expect(cg.serverCalls).To(Equal(0))
			secret := &corev1.Secret{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{
				Name:      PrometheusCertSecretName(pod.Name),
				Namespace: metav1.NamespaceDefault,
			}, secret)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("should propagate the generator error and not create a Secret", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, make(map[string]string))
			pod := newPendingPod(targetPvc, pvcPrime, corev1.PodPending)
			generatorErr := errors.New("certificate generation failed")
			cg := &fakeCertGenerator{err: generatorErr}

			reconciler = createForkliftPopulatorReconcilerWithCertConfig(cg, serverCertConfig, targetPvc, pvcPrime, sc, ovirtCr, pod)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(MatchError(generatorErr))

			secret := &corev1.Secret{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{
				Name:      PrometheusCertSecretName(pod.Name),
				Namespace: metav1.NamespaceDefault,
			}, secret)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		It("should preserve the data of an existing Secret", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, make(map[string]string))
			pod := newPendingPod(targetPvc, pvcPrime, corev1.PodPending)
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PrometheusCertSecretName(pod.Name),
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("existing-cert"),
					corev1.TLSPrivateKeyKey: []byte("existing-key"),
				},
			}
			cg := &fakeCertGenerator{cert: []byte(certData), key: []byte(keyData)}

			reconciler = createForkliftPopulatorReconcilerWithCertConfig(cg, serverCertConfig, targetPvc, pvcPrime, sc, ovirtCr, pod, existingSecret)

			_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(reconciler.client.Get(context.TODO(), types.NamespacedName{
				Name:      PrometheusCertSecretName(pod.Name),
				Namespace: metav1.NamespaceDefault,
			}, secret)).To(Succeed())
			Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSCertKey, []byte("existing-cert")))
			Expect(secret.Data).To(HaveKeyWithValue(corev1.TLSPrivateKeyKey, []byte("existing-key")))
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
		pvcPrime.Status = corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimLost}

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
				_, _ = w.Write([]byte(fmt.Sprintf("kubevirt_cdi_openstack_populator_progress_total{ownerUID=\"%v\"} 13.45", targetPvc.GetUID())))
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()
			ep, err := url.Parse(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			port, err := strconv.ParseInt(ep.Port(), 10, 32)
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

		It("should remove the populator pod after pvcPrime is marked for deletion", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			targetPvc.Spec.DataSourceRef = dataSourceRef
			pvcPrime := getPVCPrime(targetPvc, make(map[string]string))
			pvcPrime.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			populatorPod := getPopulatorPod(targetPvc, pvcPrime)

			By("Reconcile")
			reconciler = createForkliftPopulatorReconciler(targetPvc, pvcPrime, sc, populatorPod)

			result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: targetPvcName, Namespace: metav1.NamespaceDefault}})
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Not(BeNil()))

			By("Checking if the populator pod is deleted")
			pod := &corev1.Pod{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: populatorPod.Name, Namespace: populatorPod.Namespace}, pod)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})

		DescribeTable(
			"should create the populator pod with expected specifications",
			func(cr runtime.Object, ref *corev1.TypedObjectReference, expectedImage func() string, expectedCommand string, expectedArgs []string, volumeMode corev1.PersistentVolumeMode, expectedVolumePath string) {
				targetPvc := CreatePvcInStorageClass(
					targetPvcName,
					metav1.NamespaceDefault,
					&sc.Name,
					nil,
					nil,
					corev1.ClaimPending,
				)
				targetPvc.Spec.DataSourceRef = ref
				targetPvc.Spec.VolumeMode = ptr.To(volumeMode)

				pvcPrime := getPVCPrime(targetPvc, make(map[string]string))

				reconciler = createForkliftPopulatorReconciler(
					targetPvc,
					pvcPrime,
					sc,
					cr,
				)

				err := reconciler.createPopulatorPod(pvcPrime, targetPvc)
				Expect(err).NotTo(HaveOccurred())

				pod := &corev1.Pod{}
				podName := fmt.Sprintf("%s-%s", populatorPodPrefix, targetPvc.UID)

				Expect(reconciler.client.Get(
					context.TODO(),
					types.NamespacedName{
						Name:      podName,
						Namespace: targetPvc.Namespace,
					},
					pod,
				)).To(Succeed())

				Expect(pod.Spec.Containers).To(HaveLen(1))
				container := pod.Spec.Containers[0]

				Expect(container.Name).To(Equal(populatorContainerName))
				Expect(container.Image).To(Equal(expectedImage()))
				Expect(container.Command).To(Equal([]string{expectedCommand}))
				Expect(container.Args).To(ContainElements(append([]string{
					fmt.Sprintf("--owner-uid=%s", targetPvc.UID),
					fmt.Sprintf(
						"--pvc-size=%d",
						targetPvc.Spec.Resources.Requests.Storage().Value(),
					),
					expectedVolumePath,
				}, expectedArgs...)))

				if volumeMode == corev1.PersistentVolumeBlock {
					Expect(container.VolumeDevices).To(ContainElement(
						corev1.VolumeDevice{
							Name:       populatorPodVolumeName,
							DevicePath: devicePath,
						},
					))
				} else {
					Expect(container.VolumeMounts).To(ContainElement(
						And(
							HaveField("Name", Equal(populatorPodVolumeName)),
							HaveField("MountPath", Equal(mountPath)),
						),
					))
				}

				Expect(container).To(HaveField(
					"SecurityContext.ReadOnlyRootFilesystem",
					HaveValue(BeTrue()),
				))

				Expect(container.VolumeMounts).To(ContainElement(
					And(
						HaveField("Name", Equal(common.TmpVolumeName)),
						HaveField("MountPath", Equal(common.TmpMountPath)),
					),
				))

				Expect(container.VolumeMounts).To(ContainElement(
					And(
						HaveField("Name", Equal(common.PrometheusCertVolName)),
						HaveField("MountPath", Equal(common.PrometheusCertDir)),
						HaveField("ReadOnly", BeTrue()),
					),
				))

				Expect(pod.Spec.Volumes).To(ContainElement(
					And(
						HaveField("Name", Equal(common.TmpVolumeName)),
						HaveField("VolumeSource.EmptyDir", Not(BeNil())),
					),
				))

				Expect(pod.Spec.Volumes).To(ContainElement(
					And(
						HaveField("Name", Equal(populatorPodVolumeName)),
						HaveField("VolumeSource.PersistentVolumeClaim.ClaimName", Equal(pvcPrime.Name)),
					),
				))

				Expect(pod.Spec.Volumes).To(ContainElement(
					And(
						HaveField("Name", Equal(common.PrometheusCertVolName)),
						HaveField("VolumeSource.Secret.SecretName", Equal(PrometheusCertSecretName(pod.Name))),
					),
				))
			},
			Entry(
				"oVirt, filesystem volume mode",
				ovirtCr, dataSourceRef,
				func() string { return reconciler.ovirtPopulatorImage }, "ovirt-populator",
				[]string{
					"--secret-name=ovirt-engine-secret",
					"--disk-id=12345678-1234-1234-1234-123456789012",
					"--engine-url=https://ovirt-engine.example.com",
				},
				corev1.PersistentVolumeFilesystem,
				"--volume-path="+mountPath+"disk.img",
			),
			Entry(
				"oVirt, raw-block volume mode",
				ovirtCr, dataSourceRef,
				func() string { return reconciler.ovirtPopulatorImage }, "ovirt-populator",
				[]string{
					"--secret-name=ovirt-engine-secret",
					"--disk-id=12345678-1234-1234-1234-123456789012",
					"--engine-url=https://ovirt-engine.example.com",
				},
				corev1.PersistentVolumeBlock,
				"--volume-path="+devicePath,
			),
			Entry(
				"OpenStack, filesystem volume mode",
				openstackCr, openstackDataSourceRef,
				func() string { return reconciler.importerImage }, "openstack-populator",
				[]string{
					"--endpoint=https://openstack.example.com:5000/v3",
					"--secret-name=openstack-secret",
					"--image-id=abcdef01-2345-6789-abcd-ef0123456789",
				},
				corev1.PersistentVolumeFilesystem,
				"--volume-path="+mountPath+"disk.img",
			),
			Entry(
				"OpenStack, raw-block volume mode",
				openstackCr, openstackDataSourceRef,
				func() string { return reconciler.importerImage }, "openstack-populator",
				[]string{
					"--endpoint=https://openstack.example.com:5000/v3",
					"--secret-name=openstack-secret",
					"--image-id=abcdef01-2345-6789-abcd-ef0123456789",
				},
				corev1.PersistentVolumeBlock,
				"--volume-path="+devicePath,
			),
		)

		It("should correctly identify a PVC as Forklift kind", func() {
			validPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-pvc",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: &corev1.TypedObjectReference{
						APIGroup: &apiGroup,
						Kind:     "OvirtVolumePopulator",
						Name:     "sample-populator",
					},
				},
			}

			invalidPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-pvc",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: &corev1.TypedObjectReference{
						APIGroup: &apiGroup,
						Kind:     "UnknownPopulator",
						Name:     "sample-populator",
					},
				},
			}

			noDataSourceRefPVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-datasource-pvc",
					Namespace: metav1.NamespaceDefault,
				},
			}

			By("Validating PVC with correct DataSourceRef")
			isValid := isPVCForkliftKind(validPVC)
			Expect(isValid).To(BeTrue())

			By("Validating PVC with incorrect DataSourceRef kind")
			isInvalid := isPVCForkliftKind(invalidPVC)
			Expect(isInvalid).To(BeFalse())

			By("Validating PVC with no DataSourceRef")
			isNoDataSourceRef := isPVCForkliftKind(noDataSourceRefPVC)
			Expect(isNoDataSourceRef).To(BeFalse())
		})
		It("Should not reconcile APIGroup forklift.konveyor.io", func() {
			targetPvc := CreatePvcInStorageClass(targetPvcName, metav1.NamespaceDefault, &sc.Name, nil, nil, corev1.ClaimPending)
			apiGroup := "forklift.konveyor.io"
			dataSourceRef := &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     v1beta1.OvirtVolumePopulatorKind,
				Name:     samplePopulatorName,
			}
			targetPvc.Spec.DataSourceRef = dataSourceRef
			Expect(isPVCForkliftKind(targetPvc)).To(BeFalse())
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

func createForkliftPopulatorReconcilerWithCertConfig(certGenerator generator.CertGenerator, serverConfig *cdiv1.CertConfig, objects ...runtime.Object) *ForkliftPopulatorReconciler {
	cdiCR := MakeEmptyCDICR()
	cdiCR.Spec.CertConfig = &cdiv1.CDICertConfig{Server: serverConfig}

	reconciler := createForkliftPopulatorReconcilerWithObjects(append([]runtime.Object{cdiCR}, objects...)...)
	reconciler.serverCertGenerator = certGenerator
	return reconciler
}

func createForkliftPopulatorReconcilerWithoutConfig(objects ...runtime.Object) *ForkliftPopulatorReconciler {
	return createForkliftPopulatorReconcilerWithObjects(append([]runtime.Object{MakeEmptyCDICR()}, objects...)...)
}

func createForkliftPopulatorReconcilerWithObjects(objects ...runtime.Object) *ForkliftPopulatorReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)
	_ = v1beta1.AddToScheme(s)

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
		importerImage:       "importer-image",
		ovirtPopulatorImage: "ovirt-populator-image",
		serverCertGenerator: &fakeCertGenerator{},
	}

	return r
}
