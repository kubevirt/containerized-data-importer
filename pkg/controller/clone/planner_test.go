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
	"strings"

	. "github.com/onsi/ginkgo/v2"
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
	"k8s.io/utils/ptr"

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
		volumeName = "sourceVolume"
		driverName = "driver"
	)

	var (
		planner *Planner

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

	createPVCDataSource := func() *cdiv1.VolumeCloneSource {
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

	createSnapshotDataSource := func() *cdiv1.VolumeCloneSource {
		return &cdiv1.VolumeCloneSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vcs-snapshot",
				Namespace: namespace,
			},
			Spec: cdiv1.VolumeCloneSourceSpec{
				Source: corev1.TypedLocalObjectReference{
					APIGroup: &snapshotv1.SchemeGroupVersion.Group,
					Kind:     "VolumeSnapshot",
					Name:     sourceName,
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

	createSourceSnapshot := func(name, volumeSnapshotContentName, snapClassName string) *snapshotv1.VolumeSnapshot {
		pvcName := "some-pvc-that-was-snapshotted"
		size := resource.MustParse("1G")

		return &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
				VolumeSnapshotClassName: &snapClassName,
			},
			Status: &snapshotv1.VolumeSnapshotStatus{
				RestoreSize:                    &size,
				BoundVolumeSnapshotContentName: &volumeSnapshotContentName,
			},
		}
	}

	createDefaultVolumeSnapshotContent := func() *snapshotv1.VolumeSnapshotContent {
		return &snapshotv1.VolumeSnapshotContent{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-snapshot-content-name",
			},
			Spec: snapshotv1.VolumeSnapshotContentSpec{
				Driver: driverName,
			},
		}
	}

	createSourceClaim := func() *corev1.PersistentVolumeClaim {
		s := createClaim(sourceName)
		s.Spec.VolumeName = volumeName
		s.Status.Capacity = corev1.ResourceList{
			corev1.ResourceStorage: s.Spec.Resources.Requests[corev1.ResourceStorage],
		}
		return s
	}

	createSourceVolume := func() *corev1.PersistentVolume {
		return &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: volumeName,
			},
			Spec: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: small,
				},
				StorageClassName: storageClassName,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: driverName,
					},
				},
				ClaimRef: &corev1.ObjectReference{
					Namespace: namespace,
					Name:      sourceName,
				},
			},
		}
	}

	createStorageClass := func() *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          driverName,
			AllowVolumeExpansion: ptr.To[bool](true),
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

	expectEvent := func(planner *Planner, event string) {
		close(planner.Recorder.(*record.FakeRecorder).Events)
		found := false
		for e := range planner.Recorder.(*record.FakeRecorder).Events {
			if strings.Contains(e, event) {
				found = true
			}
		}
		planner.Recorder = nil
		Expect(found).To(BeTrue())
	}

	AfterEach(func() {
		if planner != nil && planner.Recorder != nil {
			close(planner.Recorder.(*record.FakeRecorder).Events)
			planner = nil
		}
	})

	Context("ChooseStrategy tests", func() {

		It("should error if unsupported kind", func() {
			source := createPVCDataSource()
			source.Spec.Source.Kind = "UnsupportedKind"
			args := &ChooseStrategyArgs{
				DataSource: source,
				Log:        log,
			}
			planner = createPlanner()
			_, err := planner.ChooseStrategy(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unsupported datasource"))
		})

		Context("PVC source", func() {
			It("should return nil if no storageclass name", func() {
				tc := createTargetClaim()
				tc.Spec.StorageClassName = nil
				args := &ChooseStrategyArgs{
					TargetClaim: tc,
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner()
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).To(BeNil())
			})

			It("should error if emptystring storageclass name", func() {
				tc := createTargetClaim()
				tc.Spec.StorageClassName = ptr.To[string]("")
				args := &ChooseStrategyArgs{
					TargetClaim: tc,
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner()
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("claim has emptystring storageclass, will not work"))
				Expect(csr).To(BeNil())
			})

			It("should error if no storageclass exists", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner()
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("target storage class not found"))
				Expect(csr).To(BeNil())
			})

			It("should return nil if no source", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).To(BeNil())
				expectEvent(planner, CloneWithoutSource)
			})

			It("should fail target smaller", func() {
				source := createSourceClaim()
				target := createTargetClaim()
				target.Spec.Resources.Requests[corev1.ResourceStorage] = small
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), source)
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(HavePrefix("target resources requests storage size is smaller than the source"))
				Expect(csr).To(BeNil())
				expectEvent(planner, CloneValidationFailed)
			})

			It("should return host assisted with no volumesnapshotclass", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createSourceClaim())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageNoVolumeSnapshotClass))
				expectEvent(planner, NoVolumeSnapshotClass)
			})

			It("should return snapshot with volumesnapshotclass", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createSourceClaim(), createVolumeSnapshotClass(), createSourceVolume())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategySnapshot))
			})

			It("should return snapshot with volumesnapshotclass (no source vol)", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createSourceClaim(), createVolumeSnapshotClass())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategySnapshot))
			})

			It("should return snapshot with volumesnapshotclass and source storageclass does not exist but same driver", func() {
				sourceClaim := createSourceClaim()
				sourceVolume := createSourceVolume()
				sourceVolume.Spec.StorageClassName = "baz"
				sourceClaim.Spec.StorageClassName = ptr.To[string](sourceVolume.Spec.StorageClassName)
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createVolumeSnapshotClass(), sourceClaim, sourceVolume)
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategySnapshot))
			})

			It("should returnsnapshot with bigger target", func() {
				target := createTargetClaim()
				target.Spec.Resources.Requests[corev1.ResourceStorage] = large
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createSourceClaim(), createVolumeSnapshotClass())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategySnapshot))
			})

			It("should return host assisted with bigger target and no volumeexpansion", func() {
				storageClass := createStorageClass()
				storageClass.AllowVolumeExpansion = nil
				target := createTargetClaim()
				target.Spec.Resources.Requests[corev1.ResourceStorage] = large
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(storageClass, createSourceClaim(), createVolumeSnapshotClass())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageNoVolumeExpansion))
				expectEvent(planner, NoVolumeExpansion)
			})

			It("should return host assisted with non matching volume modes", func() {
				bm := corev1.PersistentVolumeBlock
				source := createSourceClaim()
				source.Spec.VolumeMode = &bm
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createVolumeSnapshotClass(), source)
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageIncompatibleVolumeModes))
				expectEvent(planner, IncompatibleVolumeModes)
			})

			It("should return csi-clone if global override is set", func() {
				cs := cdiv1.CloneStrategyCsiClone
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), createSourceClaim())
				cdi := &cdiv1.CDI{}
				err := planner.Client.Get(context.Background(), client.ObjectKeyFromObject(cc.MakeEmptyCDICR()), cdi)
				Expect(err).ToNot(HaveOccurred())
				cdi.Spec.CloneStrategyOverride = &cs
				err = planner.Client.Update(context.Background(), cdi)
				Expect(err).ToNot(HaveOccurred())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyCsiClone))
			})

			It("should return csi-clone if storage profile is set", func() {
				cs := cdiv1.CloneStrategyCsiClone
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
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
				planner = createPlanner(sp, createStorageClass(), createSourceClaim())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyCsiClone))
			})

			It("should return host assisted if csi-clone in storage profile is set but source is different driver", func() {
				cs := cdiv1.CloneStrategyCsiClone
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createPVCDataSource(),
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
				sourceClaim := createSourceClaim()
				sourceClaim.Spec.StorageClassName = ptr.To[string]("foo")
				sourceVolume := createSourceVolume()
				sourceVolume.Spec.StorageClassName = "foo"
				sourceVolume.Spec.PersistentVolumeSource.CSI.Driver = "baz"
				planner = createPlanner(sp, createStorageClass(), sourceClaim, sourceVolume)
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageIncompatibleProvisioners))
			})
		})

		Context("Snapshot source", func() {
			createDefaultVolumeSnapshotContent := func(driver string) *snapshotv1.VolumeSnapshotContent {
				return &snapshotv1.VolumeSnapshotContent{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-snapshot-content-name",
					},
					Spec: snapshotv1.VolumeSnapshotContentSpec{
						Driver: driver,
					},
				}
			}

			It("should return nil if no source", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass())
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).To(BeNil())
				expectEvent(planner, CloneWithoutSource)
			})

			It("should fail when target storage class does not exist", func() {
				args := &ChooseStrategyArgs{
					TargetClaim: createTargetClaim(),
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner()
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("target storage class not found"))
				Expect(csr).To(BeNil())
			})

			It("should fail when snapshot doesn't have populated volumeSnapshotContent name", func() {
				source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				source.Status = nil
				target := createTargetClaim()
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), source)
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("volumeSnapshotContent name not found"))
				Expect(csr).To(BeNil())
			})

			It("should fallback to host-assisted when snapshot and storage class provisioners differ", func() {
				source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				target := createTargetClaim()
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), source, createDefaultVolumeSnapshotContent("test"))
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageNoProvisionerMatch))
				expectEvent(planner, MessageNoProvisionerMatch)
			})

			It("should fail if snapshot doesn't have restore size", func() {
				source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				source.Status.RestoreSize = nil
				target := createTargetClaim()
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), source, createDefaultVolumeSnapshotContent("driver"))
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("snapshot has no RestoreSize"))
				Expect(csr).To(BeNil())
			})

			It("should fallback to host-assisted when expansion is not supported", func() {
				source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				target := createTargetClaim()
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				sc := createStorageClass()
				sc.AllowVolumeExpansion = nil
				planner = createPlanner(sc, source, createDefaultVolumeSnapshotContent("driver"))
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategyHostAssisted))
				Expect(csr.FallbackReason).ToNot(BeNil())
				Expect(*csr.FallbackReason).To(Equal(MessageNoVolumeExpansion))
				expectEvent(planner, NoVolumeExpansion)
			})

			It("should do smart clone when meeting all prerequisites", func() {
				source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				target := createTargetClaim()
				args := &ChooseStrategyArgs{
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				planner = createPlanner(createStorageClass(), source, createDefaultVolumeSnapshotContent("driver"))
				csr, err := planner.ChooseStrategy(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(csr).ToNot(BeNil())
				Expect(csr.Strategy).To(Equal(cdiv1.CloneStrategySnapshot))
			})
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

		tmpClaimName := func(uid types.UID) string {
			return "tmp-pvc-" + string(uid)
		}

		tmpSourceClaimName := func(uid types.UID) string {
			return "tmp-source-pvc-" + string(uid)
		}

		tmpSnapshotName := func(uid types.UID) string {
			return "tmp-snapshot-" + string(uid)
		}

		validateHostClonePhase := func(planner *Planner, args *PlanArgs, p Phase) {
			hc := p.(*HostClonePhase)
			Expect(hc).ToNot(BeNil())
			Expect(hc.Owner).To(Equal(args.TargetClaim))
			Expect(hc.Namespace).To(Equal(namespace))
			if IsDataSourceSnapshot(args.DataSource.Spec.Source.Kind) && args.Strategy == cdiv1.CloneStrategyHostAssisted {
				Expect(hc.SourceName).To(Equal(tmpSourceClaimName(args.TargetClaim.UID)))
			} else {
				Expect(hc.SourceName).To(Equal(sourceName))
			}
			Expect(hc.ImmediateBind).To(BeTrue())
			Expect(hc.OwnershipLabel).To(Equal(planner.OwnershipLabel))
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
			if IsDataSourcePVC(args.DataSource.Spec.Source.Kind) {
				Expect(scp.SourceName).To(Equal(tmpSnapshotName(args.TargetClaim.UID)))
			} else {
				Expect(scp.SourceName).To(Equal(args.DataSource.Spec.Source.Name))
			}
			if IsDataSourceSnapshot(args.DataSource.Spec.Source.Kind) && args.Strategy == cdiv1.CloneStrategyHostAssisted {
				Expect(scp.DesiredClaim.Name).To(Equal(tmpSourceClaimName(args.TargetClaim.UID)))
			} else {
				Expect(scp.DesiredClaim.Name).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			}
			Expect(scp.OwnershipLabel).To(Equal(planner.OwnershipLabel))
		}

		validatePrepClaimPhase := func(planner *Planner, args *PlanArgs, p Phase) {
			pcp := p.(*PrepClaimPhase)
			Expect(pcp).ToNot(BeNil())
			Expect(pcp.Owner).To(Equal(args.TargetClaim))
			if IsDataSourceSnapshot(args.DataSource.Spec.Source.Kind) && args.Strategy == cdiv1.CloneStrategyHostAssisted {
				Expect(pcp.DesiredClaim.Name).To(Equal(tmpSourceClaimName(args.TargetClaim.UID)))
			} else {
				Expect(pcp.DesiredClaim.Name).To(Equal(tmpClaimName(args.TargetClaim.UID)))
			}
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
				DataSource:  createPVCDataSource(),
				Log:         log,
			}
			planner = createPlanner(cdiConfig, createStorageClass(), source)
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
				DataSource:  createPVCDataSource(),
				Log:         log,
			}
			planner = createPlanner(cdiConfig, createStorageClass(), createVolumeSnapshotClass(), source)
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
				DataSource:  createPVCDataSource(),
				Log:         log,
			}
			planner = createPlanner(cdiConfig, createStorageClass(), source)
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(3))
			validateCSIClonePhase(planner, args, plan[0])
			validatePrepClaimPhase(planner, args, plan[1])
			validateRebindPhase(planner, args, plan[2])
		})

		It("should plan smart clone from snapshot", func() {
			source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategySnapshot,
				TargetClaim: target,
				DataSource:  createSnapshotDataSource(),
				Log:         log,
			}
			planner = createPlanner(cdiConfig, createStorageClass(), source, createVolumeSnapshotClass(), createDefaultVolumeSnapshotContent())
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(3))
			validateSnapshotClonePhase(planner, args, plan[0])
			validatePrepClaimPhase(planner, args, plan[1])
			validateRebindPhase(planner, args, plan[2])
		})

		It("should plan host-assisted clone from snapshot", func() {
			source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategyHostAssisted,
				TargetClaim: target,
				DataSource:  createSnapshotDataSource(),
				Log:         log,
			}
			planner = createPlanner(cdiConfig, createStorageClass(), source, createVolumeSnapshotClass(), createDefaultVolumeSnapshotContent())
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).ToNot(BeNil())
			Expect(plan).To(HaveLen(4))
			validateSnapshotClonePhase(planner, args, plan[0])
			validatePrepClaimPhase(planner, args, plan[1])
			validateHostClonePhase(planner, args, plan[2])
			validateRebindPhase(planner, args, plan[3])
		})

		Context("temp host assisted source pvc spec", func() {
			volumeSnapshotContentWithSourceVolumeMode := func() *snapshotv1.VolumeSnapshotContent {
				vsc := createDefaultVolumeSnapshotContent()
				vsc.Spec.SourceVolumeMode = ptr.To[corev1.PersistentVolumeMode]("dummy")
				return vsc
			}

			snapWithSourceVolumeModeAnnotation := func() *snapshotv1.VolumeSnapshot {
				snap := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
				cc.AddAnnotation(snap, cc.AnnSourceVolumeMode, "dummyfromann")
				return snap
			}

			DescribeTable("should pick correct spec for temp host assisted source in clone from snapshot", func(objs []runtime.Object, expectedVolumeMode corev1.PersistentVolumeMode, source *snapshotv1.VolumeSnapshot) {
				target := createTargetClaim()
				target.Spec.VolumeMode = ptr.To[corev1.PersistentVolumeMode]("dummytargetvolmode")
				args := &PlanArgs{
					Strategy:    cdiv1.CloneStrategyHostAssisted,
					TargetClaim: target,
					DataSource:  createSnapshotDataSource(),
					Log:         log,
				}
				runtimeObjs := []runtime.Object{cdiConfig, source, createVolumeSnapshotClass()}
				runtimeObjs = append(runtimeObjs, objs...)
				planner = createPlanner(runtimeObjs...)
				plan, err := planner.Plan(context.Background(), args)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan).ToNot(BeNil())
				Expect(plan).To(HaveLen(4))
				validateSnapshotClonePhase(planner, args, plan[0])
				validatePrepClaimPhase(planner, args, plan[1])
				validateHostClonePhase(planner, args, plan[2])
				validateRebindPhase(planner, args, plan[3])
				Expect(plan[0].(*SnapshotClonePhase).DesiredClaim.Spec.VolumeMode).To(HaveValue(Equal(expectedVolumeMode)))
				Expect(plan[0].(*SnapshotClonePhase).DesiredClaim.Spec.AccessModes).To(ConsistOf(
					[]corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
				))
				Expect(plan[0].(*SnapshotClonePhase).DesiredClaim.Spec.DataSource).To(BeNil())
				Expect(plan[0].(*SnapshotClonePhase).DesiredClaim.Spec.DataSourceRef).To(BeNil())
			},
				Entry("when volumesnapshotcontent has source volume mode", []runtime.Object{volumeSnapshotContentWithSourceVolumeMode(), createStorageClass()}, corev1.PersistentVolumeMode("dummy"), createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")),
				Entry("when volumesnapshotcontent has no source volume mode but annotated with AnnSourceVolumeMode", []runtime.Object{createDefaultVolumeSnapshotContent(), createStorageClass()}, corev1.PersistentVolumeMode("dummyfromann"), snapWithSourceVolumeModeAnnotation()),
				Entry("when neither source volume mode on volumesnapshotcontent nor AnnSourceVolumeMode annotation", []runtime.Object{createDefaultVolumeSnapshotContent(), createStorageClass()}, corev1.PersistentVolumeMode("dummytargetvolmode"), createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")),
			)
		})

		It("should fail planning host-assisted clone from snapshot when no valid storage class for source PVC is found", func() {
			source := createSourceSnapshot(sourceName, "test-snapshot-content-name", "vsc")
			target := createTargetClaim()
			args := &PlanArgs{
				Strategy:    cdiv1.CloneStrategyHostAssisted,
				TargetClaim: target,
				DataSource:  createSnapshotDataSource(),
				Log:         log,
			}
			sc := createStorageClass()
			sc.Provisioner = "test-error"
			planner = createPlanner(cdiConfig, sc, source, createVolumeSnapshotClass(), createDefaultVolumeSnapshotContent())
			plan, err := planner.Plan(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to find a valid storage class for the temporal source claim"))
			Expect(plan).To(BeNil())
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
			planner = createPlanner(tempObjs...)
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
