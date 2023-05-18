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

package clone

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("Planner test", func() {
	log := logf.Log.WithName("planner-test")

	const (
		namespace  = "ns"
		sourceName = "source"
		targetName = "target"
		ownerLabel = "label"
	)

	var (
		storageClassName = "sc"

		small  = resource.MustParse("5Gi")
		medium = resource.MustParse("10Gi")
		large  = resource.MustParse("15Gi")
	)

	createPlanner := func(objects ...runtime.Object) *Planner {
		s := scheme.Scheme
		_ = cdiv1.AddToScheme(s)
		_ = snapshotv1.AddToScheme(s)

		objects = append(objects, cc.MakeEmptyCDICR())

		// Create a fake client to mock API calls.
		builder := fake.NewClientBuilder().
			WithScheme(s).
			WithRuntimeObjects(objects...)

		cl := builder.Build()

		rec := record.NewFakeRecorder(10)

		return &Planner{
			RootObjectType: &corev1.PersistentVolumeClaimList{},
			OwnershipLabel: ownerLabel,
			UIDField:       "uid",
			Image:          "image",
			PullPolicy:     corev1.PullIfNotPresent,
			InstallerLabels: map[string]string{
				"key": "value",
			},
			Client:            cl,
			Recorder:          rec,
			watchingSnapshots: true,
		}
	}

	createDataSource := func() *cdiv1.VolumeCloneSource {
		return &cdiv1.VolumeCloneSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vcs",
				Namespace: namespace,
			},
			Spec: cdiv1.VolumeCloneSourceSpec{
				Source: corev1.TypedLocalObjectReference{
					Kind: "PersistentVolumeClaim",
					Name: sourceName,
				},
			},
		}
	}

	createClaim := func(name string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				UID:       types.UID(name + "-uid"),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: medium,
					},
				},
				StorageClassName: &storageClassName,
			},
		}
	}

	createTargetClaim := func() *corev1.PersistentVolumeClaim {
		return createClaim(targetName)
	}

	createSourceClaim := func() *corev1.PersistentVolumeClaim {
		s := createClaim(sourceName)
		s.Spec.VolumeName = "sourceVolume"
		s.Status.Capacity = corev1.ResourceList{
			corev1.ResourceStorage: s.Spec.Resources.Requests[corev1.ResourceStorage],
		}
		return s
	}

	createStorageClass := func() *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          "provisioner",
			AllowVolumeExpansion: pointer.Bool(true),
		}
	}

	createVolumeSnapshotClass := func() *snapshotv1.VolumeSnapshotClass {
		return &snapshotv1.VolumeSnapshotClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vsc",
			},
			Driver: createStorageClass().Provisioner,
		}
	}

	Context("ChooseStrategy tests", func() {

		It("should error if unsupported kind", func() {
			source := createDataSource()
			source.Spec.Source.Kind = "UnsupportedKind"
			args := &ChooseStrategyArgs{
				DataSource: source,
				Log:        log,
			}
			planner := createPlanner()
			_, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported datasource"))
		})

		It("should return nil if no storageclass name", func() {
			tc := createTargetClaim()
			tc.Spec.StorageClassName = nil
			args := &ChooseStrategyArgs{
				TargetClaim: tc,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner()
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).To(BeNil())
		})

		It("should error if emptystring storageclass name", func() {
			tc := createTargetClaim()
			tc.Spec.StorageClassName = pointer.String("")
			args := &ChooseStrategyArgs{
				TargetClaim: tc,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner()
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("claim has emptystring storageclass, will not work"))
			Expect(strategy).To(BeNil())
		})

		It("should error if no storageclass exists", func() {
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner()
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no storageclass for pvc"))
			Expect(strategy).To(BeNil())
		})

		It("should return nil if no source", func() {
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).To(BeNil())
		})

		It("should return nil if source not bound", func() {
			source := createSourceClaim()
			source.Spec.VolumeName = ""
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), source)
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).To(BeNil())
		})

		It("should fail target smaller", func() {
			source := createSourceClaim()
			target := createTargetClaim()
			target.Spec.Resources.Requests[corev1.ResourceStorage] = small
			args := &ChooseStrategyArgs{
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), source)
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("target resources requests storage size is smaller than the source"))
			Expect(strategy).To(BeNil())
		})

		It("should return host assisted with no volumesnapshotclass", func() {
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), createSourceClaim())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
		})

		It("should return snapshot with volumesnapshotclass", func() {
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), createSourceClaim(), createVolumeSnapshotClass())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategySnapshot))
		})

		It("should returnsnapshot with bigger target", func() {
			target := createTargetClaim()
			target.Spec.Resources.Requests[corev1.ResourceStorage] = large
			args := &ChooseStrategyArgs{
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), createSourceClaim(), createVolumeSnapshotClass())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategySnapshot))
		})

		It("should return host assisted with bigger target and no volumeexpansion", func() {
			storageClass := createStorageClass()
			storageClass.AllowVolumeExpansion = nil
			target := createTargetClaim()
			target.Spec.Resources.Requests[corev1.ResourceStorage] = large
			args := &ChooseStrategyArgs{
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(storageClass, createSourceClaim())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
		})

		It("should return host assisted with non matching volume modes", func() {
			bm := corev1.PersistentVolumeBlock
			source := createSourceClaim()
			source.Spec.VolumeMode = &bm
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), source)
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
		})

		It("should return csi-clone if global override is set", func() {
			cs := cdiv1.CloneStrategyCsiClone
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(createStorageClass(), createSourceClaim())
			cdi := &cdiv1.CDI{}
			err := planner.Client.Get(context.Background(), client.ObjectKeyFromObject(cc.MakeEmptyCDICR()), cdi)
			Expect(err).ToNot(HaveOccurred())
			cdi.Spec.CloneStrategyOverride = &cs
			err = planner.Client.Update(context.Background(), cdi)
			Expect(err).ToNot(HaveOccurred())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategyCsiClone))
		})

		It("should return csi-clone if storage profile is set", func() {
			cs := cdiv1.CloneStrategyCsiClone
			args := &ChooseStrategyArgs{
				TargetClaim: createTargetClaim(),
				DataSource:  createDataSource(),
				Log:         log,
			}
			sp := &cdiv1.StorageProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: storageClassName,
				},
				Status: cdiv1.StorageProfileStatus{
					CloneStrategy: &cs,
				},
			}
			planner := createPlanner(sp, createStorageClass(), createSourceClaim())
			strategy, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(strategy).ToNot(BeNil())
			Expect(*strategy).To(Equal(cdiv1.CloneStrategyCsiClone))
		})
	})

	Context("Plan tests", func() {
		cdiConfig := &cdiv1.CDIConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "config",
			},
			Status: cdiv1.CDIConfigStatus{
				FilesystemOverhead: &cdiv1.FilesystemOverhead{
					Global: "0.05",
				},
			},
		}

		validateHostClonePhase := func(planner *Planner, args *PlanArgs, p Phase) {
			hc := p.(*HostClonePhase)
			Expect(hc).ToNot(BeNil())
			Expect(hc.Owner).To(Equal(args.TargetClaim))
			Expect(hc.Namespace).To(Equal(namespace))
			Expect(hc.SourceName).To(Equal(sourceName))
			Expect(hc.ImmediateBind).To(BeTrue())
			Expect(hc.OwnershipLabel).To(Equal(planner.OwnershipLabel))
			desiredSize := hc.DesiredClaim.Spec.Resources.Requests[corev1.ResourceStorage]
			requestedSize := args.TargetClaim.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(desiredSize.Cmp(requestedSize)).To(Equal(1))
		}

		tmpClaimName := func(uid types.UID) string {
			return "tmp-pvc-" + string(uid)
		}

		tmpSnapshotName := func(uid types.UID) string {
			return "tmp-snapshot-" + string(uid)
		}

		validateRebindPhase := func(planner *Planner, args *PlanArgs, p Phase) {
			rb := p.(*RebindPhase)
			Expect(rb).ToNot(BeNil())
			Expect(rb.SourceNamespace).To(Equal(namespace))
			Expect(rb.SourceName).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			Expect(rb.TargetNamespace).To(Equal(namespace))
			Expect(rb.TargetName).To(Equal(targetName))
		}

		validateSnapshotPhase := func(planner *Planner, args *PlanArgs, p Phase) {
			sp := p.(*SnapshotPhase)
			Expect(sp).ToNot(BeNil())
			Expect(sp.Owner).To(Equal(args.TargetClaim))
			Expect(sp.SourceNamespace).To(Equal(namespace))
			Expect(sp.SourceName).To(Equal(sourceName))
			Expect(sp.TargetName).To(Equal(tmpSnapshotName(args.TargetClaim.UID)))
			Expect(sp.OwnershipLabel).To(Equal(planner.OwnershipLabel))
			Expect(sp.VolumeSnapshotClass).To(Equal("vsc"))
		}

		validateSnapshotClonePhase := func(planner *Planner, args *PlanArgs, p Phase) {
			scp := p.(*SnapshotClonePhase)
			Expect(scp).ToNot(BeNil())
			Expect(scp.Owner).To(Equal(args.TargetClaim))
			Expect(scp.Namespace).To(Equal(namespace))
			Expect(scp.SourceName).To(Equal(tmpSnapshotName(args.TargetClaim.UID)))
			Expect(scp.DesiredClaim.Name).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			Expect(scp.OwnershipLabel).To(Equal(planner.OwnershipLabel))
		}

		validatePrepClaimPhase := func(planner *Planner, args *PlanArgs, p Phase) {
			pcp := p.(*PrepClaimPhase)
			Expect(pcp).ToNot(BeNil())
			Expect(pcp.Owner).To(Equal(args.TargetClaim))
			Expect(pcp.DesiredClaim.Name).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			Expect(pcp.Image).To(Equal(planner.Image))
			Expect(pcp.PullPolicy).To(Equal(planner.PullPolicy))
			Expect(pcp.InstallerLabels).To(Equal(planner.InstallerLabels))
			Expect(pcp.OwnershipLabel).To(Equal(planner.OwnershipLabel))
		}

		validateCSIClonePhase := func(planner *Planner, args *PlanArgs, p Phase) {
			ccp := p.(*CSIClonePhase)
			Expect(ccp).ToNot(BeNil())
			Expect(ccp.Owner).To(Equal(args.TargetClaim))
			Expect(ccp.Namespace).To(Equal(namespace))
			Expect(ccp.SourceName).To(Equal(sourceName))
			Expect(ccp.DesiredClaim.Name).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			Expect(ccp.OwnershipLabel).To(Equal(planner.OwnershipLabel))
		}

		It("should plan host assited", func() {
			source := createSourceClaim()
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategyHostAssisted,
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(cdiConfig, createStorageClass(), source)
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(2))
			validateHostClonePhase(planner, args, plan[0])
			validateRebindPhase(planner, args, plan[1])
		})

		It("should plan snapshot", func() {
			source := createSourceClaim()
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategySnapshot,
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(cdiConfig, createStorageClass(), createVolumeSnapshotClass(), source)
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(4))
			validateSnapshotPhase(planner, args, plan[0])
			validateSnapshotClonePhase(planner, args, plan[1])
			validatePrepClaimPhase(planner, args, plan[2])
			validateRebindPhase(planner, args, plan[3])
		})

		It("should plan csi-clone", func() {
			source := createSourceClaim()
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategyCsiClone,
				TargetClaim: target,
				DataSource:  createDataSource(),
				Log:         log,
			}
			planner := createPlanner(cdiConfig, createStorageClass(), source)
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(3))
			validateCSIClonePhase(planner, args, plan[0])
			validatePrepClaimPhase(planner, args, plan[1])
			validateRebindPhase(planner, args, plan[2])
		})
	})

	Context("Cleanup tests", func() {
		tempResources := func() []runtime.Object {
			target := createTargetClaim()
			return []runtime.Object{
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "tmpClaim",
						Labels: map[string]string{
							ownerLabel: string(target.UID),
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "tmpPod",
						Labels: map[string]string{
							ownerLabel: string(target.UID),
						},
					},
				},
				&snapshotv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "tmpSnapshot",
						Labels: map[string]string{
							ownerLabel: string(target.UID),
						},
					},
				},
			}
		}

		It("should cleanup tmp resources", func() {
			tempObjs := tempResources()
			target := createTargetClaim()
			planner := createPlanner(tempObjs...)
			err := planner.Cleanup(context.Background(), log, target)
			Expect(err).ToNot(HaveOccurred())
			for _, r := range tempResources() {
				co := r.(client.Object)
				err = planner.Client.Get(context.Background(), client.ObjectKeyFromObject(co), co)
				Expect(err).To(HaveOccurred())
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			}
		})
	})
})
