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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	"kubevirt.io/containerized-data-importer/pkg/controller/clone"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

const (
	namespace       = "ns"
	targetClaimName = "target"
	dataSourceName  = "datasource"
	sourceClaimName = "source"
)

var (
	storageClassName  = "sc"
	clonePopulatorLog = logf.Log.WithName("clone-populator-test")
)

var _ = Describe("Clone populator tests", func() {
	nn := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      targetClaimName,
		},
	}

	isDefaultResult := func(result reconcile.Result, err error) {
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).ToNot(HaveOccurred())
	}

	isRequeueResult := func(result reconcile.Result, err error) {
		Expect(result.RequeueAfter).ToNot(BeZero())
		Expect(err).ToNot(HaveOccurred())
	}

	targetAndDataSource := func() (*corev1.PersistentVolumeClaim, *cdiv1.VolumeCloneSource) {
		target := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      targetClaimName,
				UID:       types.UID("uid"),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				DataSourceRef: &corev1.TypedObjectReference{
					APIGroup: &cdiv1.SchemeGroupVersion.Group,
					Kind:     cdiv1.VolumeCloneSourceRef,
					Name:     dataSourceName,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
				StorageClassName: &storageClassName,
			},
		}
		source := &cdiv1.VolumeCloneSource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      dataSourceName,
			},
			Spec: cdiv1.VolumeCloneSourceSpec{
				Source: corev1.TypedLocalObjectReference{
					Kind: "PersistentVolumeClaim",
					Name: sourceClaimName,
				},
			},
		}
		return target, source
	}

	initinializedTargetAndDataSource := func() (*corev1.PersistentVolumeClaim, *cdiv1.VolumeCloneSource) {
		target, source := targetAndDataSource()
		target.Annotations = map[string]string{
			AnnClonePhase:   clone.PendingPhaseName,
			cc.AnnCloneType: "snapshot",
		}
		clone.AddCommonClaimLabels(target)
		target.Finalizers = []string{cloneFinalizer}
		return target, source
	}

	succeededTarget := func() *corev1.PersistentVolumeClaim {
		target, _ := initinializedTargetAndDataSource()
		target.Annotations[AnnClonePhase] = string(clone.SucceededPhaseName)
		target.Spec.VolumeName = "volume"
		return target
	}

	getTarget := func(c client.Client) *corev1.PersistentVolumeClaim {
		target, _ := targetAndDataSource()
		result := &corev1.PersistentVolumeClaim{}
		err := c.Get(context.Background(), client.ObjectKeyFromObject(target), result)
		Expect(err).ToNot(HaveOccurred())
		return result
	}

	storageClass := func() *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
		}
	}

	verifyPending := func(c client.Client) {
		target := getTarget(c)
		Expect(target.Annotations[AnnClonePhase]).To(Equal(string(clone.PendingPhaseName)))
	}

	It("should do nothing if PVC is not found", func() {
		reconciler := createClonePopulatorReconciler()
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
	})

	It("should do nothing if unexpected PVC", func() {
		target, _ := targetAndDataSource()
		target.Spec.DataSourceRef.Kind = "Unexpected"
		reconciler := createClonePopulatorReconciler(target)
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		target = getTarget(reconciler.client)
		Expect(target.Annotations).ToNot(HaveKey(AnnClonePhase))
	})

	It("should be pending storageclass is nil", func() {
		target, _ := targetAndDataSource()
		target.Spec.StorageClassName = nil
		reconciler := createClonePopulatorReconciler(target)
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		verifyPending(reconciler.client)
	})

	It("should error if storageclass is not found", func() {
		target, _ := targetAndDataSource()
		reconciler := createClonePopulatorReconciler(target)
		_, err := reconciler.Reconcile(context.Background(), nn)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("storage class sc not found"))
	})

	It("should be pending if WFFC", func() {
		target, _ := targetAndDataSource()
		sc := storageClass()
		bm := storagev1.VolumeBindingWaitForFirstConsumer
		sc.VolumeBindingMode = &bm
		reconciler := createClonePopulatorReconciler(target, sc)
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		verifyPending(reconciler.client)
	})

	It("should be pending if datasource is not found", func() {
		target, _ := targetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass())
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		verifyPending(reconciler.client)
	})

	It("should be pending if choosestrategy returns nil", func() {
		target, source := targetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{}
		result, err := reconciler.Reconcile(context.Background(), nn)
		isRequeueResult(result, err)
		verifyPending(reconciler.client)
	})

	It("should be pending if choosestrategy returns nil (cross namespace validation)", func() {
		target, source := targetAndDataSource()
		source.Namespace = "other"
		cc.AddAnnotation(target, AnnDataSourceNamespace, source.Namespace)
		cc.AddAnnotation(target, cc.AnnCloneToken, "foo")
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{}
		reconciler.multiTokenValidator = &cc.MultiTokenValidator{
			ShortTokenValidator: &cc.FakeValidator{
				Match:     "foo",
				Operation: token.OperationClone,
				Name:      source.Spec.Source.Name,
				Namespace: "other",
				Resource: metav1.GroupVersionResource{
					Resource: "persistentvolumeclaims",
				},
				Params: map[string]string{
					"targetName":      target.Name,
					"targetNamespace": target.Namespace,
				},
			},
		}
		result, err := reconciler.Reconcile(context.Background(), nn)
		isRequeueResult(result, err)
		verifyPending(reconciler.client)
	})

	It("should be pending and initialize target if choosestrategy returns something", func() {
		target, source := targetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		csr := cdiv1.CloneStrategySnapshot
		reconciler.planner = &fakePlanner{
			chooseStrategyResult: &csr,
		}
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal(string(clone.PendingPhaseName)))
		Expect(pvc.Annotations[cc.AnnCloneType]).To(Equal(string(csr)))
		Expect(pvc.Finalizers).To(ContainElement(cloneFinalizer))
	})

	It("should be in error phase if plan returns an error", func() {
		target, source := initinializedTargetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{
			planError: fmt.Errorf("plan error"),
		}
		_, err := reconciler.Reconcile(context.Background(), nn)
		Expect(err).To(HaveOccurred())
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal("Error"))
		Expect(pvc.Annotations[AnnCloneError]).To(Equal("plan error"))
	})

	It("should be in error phase if phase returns an error", func() {
		target, source := initinializedTargetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{
			planResult: []clone.Phase{
				&fakePhase{
					name: "phase1",
					err:  fmt.Errorf("phase error"),
				},
			},
		}
		_, err := reconciler.Reconcile(context.Background(), nn)
		Expect(err).To(HaveOccurred())
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal("Error"))
		Expect(pvc.Annotations[AnnCloneError]).To(Equal("phase error"))
	})

	It("should report phase name and progress", func() {
		target, source := initinializedTargetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{
			planResult: []clone.Phase{
				&fakePhase{
					name: "phase1",
				},
				&fakePhaseWithProgress{
					fakePhase: fakePhase{
						name:   "phase2",
						result: &reconcile.Result{},
					},
					progress: &clone.PhaseProgress{
						Progress: "50.0%",
						Annotations: map[string]string{
							"foo":                  "bar",
							cc.AnnRunningCondition: "true",
						},
					},
				},
			},
		}
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal("phase2"))
		Expect(pvc.Annotations[cc.AnnPopulatorProgress]).To(Equal("50.0%"))
		Expect(pvc.Annotations[cc.AnnRunningCondition]).To(Equal("true"))
		Expect(pvc.Annotations).ToNot(HaveKey("foo"))
	})

	It("should be in error phase if progress returns an error", func() {
		target, source := initinializedTargetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{
			planResult: []clone.Phase{
				&fakePhaseWithProgress{
					fakePhase: fakePhase{
						name:   "phase1",
						result: &reconcile.Result{},
					},
					proogressErr: fmt.Errorf("progress error"),
				},
			},
		}
		_, err := reconciler.Reconcile(context.Background(), nn)
		Expect(err).To(HaveOccurred())
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal("Error"))
		Expect(pvc.Annotations[AnnCloneError]).To(Equal("progress error"))
	})

	It("should go to succeeded phase if all phases are done", func() {
		target, source := initinializedTargetAndDataSource()
		reconciler := createClonePopulatorReconciler(target, storageClass(), source)
		reconciler.planner = &fakePlanner{
			planResult: []clone.Phase{
				&fakePhase{
					name: "phase1",
				},
				&fakePhase{
					name: "phase2",
				},
			},
		}
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		pvc := getTarget(reconciler.client)
		Expect(pvc.Annotations[AnnClonePhase]).To(Equal("Succeeded"))
	})

	It("should remove finalizer and call cleanup when succeeded", func() {
		target := succeededTarget()
		reconciler := createClonePopulatorReconciler(target)
		fp := &fakePlanner{}
		reconciler.planner = fp
		result, err := reconciler.Reconcile(context.Background(), nn)
		isDefaultResult(result, err)
		pvc := getTarget(reconciler.client)
		Expect(pvc.Finalizers).ToNot(ContainElement(cloneFinalizer))
		Expect(fp.cleanupCalled).To(BeTrue())
	})
})

// fakePlanner implements Plan interface
type fakePlanner struct {
	chooseStrategyResult *cdiv1.CDICloneStrategy
	chooseStrategyError  error
	planResult           []clone.Phase
	planError            error
	cleanupCalled        bool
}

func (p *fakePlanner) ChooseStrategy(ctx context.Context, args *clone.ChooseStrategyArgs) (*cdiv1.CDICloneStrategy, error) {
	return p.chooseStrategyResult, p.chooseStrategyError
}

func (p *fakePlanner) Plan(ctx context.Context, args *clone.PlanArgs) ([]clone.Phase, error) {
	return p.planResult, p.planError
}

func (p *fakePlanner) Cleanup(ctx context.Context, log logr.Logger, owner client.Object) error {
	p.cleanupCalled = true
	return nil
}

type fakePhase struct {
	name   string
	result *reconcile.Result
	err    error
}

func (p *fakePhase) Name() string {
	return p.name
}

func (p *fakePhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	return p.result, p.err
}

type fakePhaseWithProgress struct {
	fakePhase
	progress     *clone.PhaseProgress
	proogressErr error
}

func (p *fakePhaseWithProgress) Progress(ctx context.Context) (*clone.PhaseProgress, error) {
	return p.progress, p.proogressErr
}

func createClonePopulatorReconciler(objects ...runtime.Object) *ClonePopulatorReconciler {
	cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createClonePopulatorReconcilerWithoutConfig(objs...)
}

func createClonePopulatorReconcilerWithoutConfig(objects ...runtime.Object) *ClonePopulatorReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)
	_ = snapshotv1.AddToScheme(s)
	_ = extv1.AddToScheme(s)

	objs = append(objs, cc.MakeEmptyCDICR())

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
	r := &ClonePopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          clonePopulatorLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
			sourceKind: cdiv1.VolumeCloneSourceRef,
		},
	}
	return r
}
