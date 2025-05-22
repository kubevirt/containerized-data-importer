package common

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

var _ = Describe("GetRequestedImageSize", func() {
	It("Should return 1G if 1G provided", func() {
		result, err := GetRequestedImageSize(CreatePvc("testPVC", "default", nil, nil))
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("1G"))
	})

	It("Should return error and blank if no size provided", func() {
		result, err := GetRequestedImageSize(createPvcNoSize("testPVC", "default", nil, nil))
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("GetStorageClassByName", func() {
	It("Should return the default storage class name", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
		)
		sc, _ := GetStorageClassByNameWithK8sFallback(context.Background(), client, nil)
		Expect(sc.Name).To(Equal("test-storage-class-2"))
	})

	It("Should return nil if there's not default storage class", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", nil),
		)
		sc, _ := GetStorageClassByNameWithK8sFallback(context.Background(), client, nil)
		Expect(sc).To(BeNil())
	})

	It("Should return default virt class even if there's not default k8s storage class", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", map[string]string{
				AnnDefaultVirtStorageClass: "true",
			}),
		)
		sc, _ := GetStorageClassByNameWithVirtFallback(context.Background(), client, nil, cdiv1.DataVolumeKubeVirt)
		Expect(sc.Name).To(Equal("test-storage-class-2"))
	})

	DescribeTable("Should return newer default", func(annotation string) {
		olderSc := CreateStorageClass("test-storage-class-new", map[string]string{
			annotation: "true",
		})
		olderSc.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-1 * time.Second)))
		newerSc := CreateStorageClass("test-storage-class-old", map[string]string{
			annotation: "true",
		})
		newerSc.SetCreationTimestamp(metav1.NewTime(time.Now()))
		client := CreateClient(newerSc, olderSc)
		sc, _ := GetStorageClassByNameWithVirtFallback(context.Background(), client, nil, cdiv1.DataVolumeKubeVirt)
		Expect(sc.Name).To(Equal(newerSc.Name))
	},
		Entry("virt storage class", AnnDefaultVirtStorageClass),
		Entry("k8s storage class", AnnDefaultStorageClass),
	)

	DescribeTable("Should fall back to lexicographic order when same timestamp", func(annotation string) {
		firstSc := CreateStorageClass("test-storage-class-1", map[string]string{
			annotation: "true",
		})
		firstSc.SetCreationTimestamp(metav1.NewTime(time.Now()))
		secondSc := CreateStorageClass("test-storage-class-2", map[string]string{
			annotation: "true",
		})
		secondSc.SetCreationTimestamp(metav1.NewTime(time.Now()))
		client := CreateClient(firstSc, secondSc)
		sc, _ := GetStorageClassByNameWithVirtFallback(context.Background(), client, nil, cdiv1.DataVolumeKubeVirt)
		Expect(sc.Name).To(Equal(firstSc.Name))
	},
		Entry("virt storage class", AnnDefaultVirtStorageClass),
		Entry("k8s storage class", AnnDefaultStorageClass),
	)
})

var _ = Describe("Rebind", func() {
	It("Should return error if PV doesn't exist", func() {
		client := CreateClient()
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		err := Rebind(context.Background(), client, pvc, pvc)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("Should return error if bound to unexpected claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "anotherPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, pvc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("PV testPV bound to unexpected claim anotherPVC"))
	})
	It("Should return nil if bound to target claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		targetPVC := pvc.DeepCopy()
		targetPVC.Name = "targetPVC"
		targetPVC.UID = "uid"
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "targetPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, targetPVC)
		Expect(err).ToNot(HaveOccurred())
	})
	It("Should rebind pv to target claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		targetPVC := pvc.DeepCopy()
		targetPVC.Name = "targetPVC"
		pvc.UID = "uid"
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "testPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		AddAnnotation(pv, "someAnno", "somevalue")
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, targetPVC)
		Expect(err).ToNot(HaveOccurred())
		updatedPV := &v1.PersistentVolume{}
		key := types.NamespacedName{Name: pv.Name, Namespace: pv.Namespace}
		err = client.Get(context.TODO(), key, updatedPV)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPV.Spec.ClaimRef.Name).To(Equal(targetPVC.Name))
		//make sure annotations of pv from before rebind dont get deleted
		Expect(pv.Annotations["someAnno"]).To(Equal("somevalue"))
	})

	Context("GetActiveCDI tests", func() {
		createCDI := func(name string, phase sdkapi.Phase) *cdiv1.CDI {
			return &cdiv1.CDI{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Status: cdiv1.CDIStatus{
					Status: sdkapi.Status{
						Phase: phase,
					},
				},
			}
		}

		It("Should return nil if no CDI", func() {
			client := CreateClient()
			cdi, err := GetActiveCDI(context.Background(), client)
			Expect(err).ToNot(HaveOccurred())
			Expect(cdi).To(BeNil())
		})

		It("Should return single active", func() {
			client := CreateClient(
				createCDI("cdi1", sdkapi.PhaseDeployed),
			)
			cdi, err := GetActiveCDI(context.Background(), client)
			Expect(err).ToNot(HaveOccurred())
			Expect(cdi).ToNot(BeNil())
		})

		It("Should return success with single active one error", func() {
			client := CreateClient(
				createCDI("cdi1", sdkapi.PhaseDeployed),
				createCDI("cdi2", sdkapi.PhaseError),
			)
			cdi, err := GetActiveCDI(context.Background(), client)
			Expect(err).ToNot(HaveOccurred())
			Expect(cdi).ToNot(BeNil())
			Expect(cdi.Name).To(Equal("cdi1"))
		})

		It("Should return error if multiple CDIs are active", func() {
			client := CreateClient(
				createCDI("cdi1", sdkapi.PhaseDeployed),
				createCDI("cdi2", sdkapi.PhaseDeployed),
			)
			cdi, err := GetActiveCDI(context.Background(), client)
			Expect(err).To(HaveOccurred())
			Expect(cdi).To(BeNil())
		})

		It("Should return error if multiple CDIs are error", func() {
			client := CreateClient(
				createCDI("cdi1", sdkapi.PhaseError),
				createCDI("cdi2", sdkapi.PhaseError),
			)
			cdi, err := GetActiveCDI(context.Background(), client)
			Expect(err).To(HaveOccurred())
			Expect(cdi).To(BeNil())
		})

	})
})

var _ = Describe("GetMetricsURL", func() {
	makePod := func(ip string, withMetrics bool) *v1.Pod {
		pod := &v1.Pod{
			Status: v1.PodStatus{
				PodIP: ip,
			},
		}

		if !withMetrics {
			return pod
		}

		pod.Spec = v1.PodSpec{
			Containers: []v1.Container{
				{
					Ports: []v1.ContainerPort{
						{Name: "metrics", ContainerPort: 8080},
					},
				},
			},
		}

		return pod
	}

	It("Should succeed with IPv4", func() {
		pod := makePod("127.0.0.1", true)
		url, err := GetMetricsURL(pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(url).To(Equal("https://127.0.0.1:8080/metrics"))
	})

	It("Should succeed with IPv6", func() {
		pod := makePod("::1", true)
		url, err := GetMetricsURL(pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(url).To(Equal("https://[::1]:8080/metrics"))
	})

	It("Should fail when there is no metrics port", func() {
		pod := makePod("127.0.0.1", false)
		_, err := GetMetricsURL(pod)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("SetDefaultLables", func() {
	It("Should set default labels", func() {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"test": "test",
				},
			},
		}
		SetDefaultLabels(pod)
		Expect(pod.Labels).To(HaveKeyWithValue(LabelIstioAmbientDataPlaneMode, LabelIstioAmbientDatePlaneModeDefault))
	})
	It("Should not overwrite existing labels", func() {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LabelIstioAmbientDataPlaneMode: "ambient",
				},
			},
		}
		SetDefaultLabels(pod)
		Expect(pod.Labels).To(HaveKeyWithValue(LabelIstioAmbientDataPlaneMode, "ambient"))
	})
})

var _ = Describe("CopyAllowedLabels", func() {
	const (
		testKubevirtIoKey               = "test.kubevirt.io/test"
		testKubevirtIoValue             = "testvalue"
		testInstancetypeKubevirtIoKey   = "instancetype.kubevirt.io/default-preference"
		testInstancetypeKubevirtIoValue = "testpreference"
		testKubevirtIoKeyExisting       = "test.kubevirt.io/existing"
		testKubevirtIoValueExisting     = "existing"
		testKubevirtIoNewValueExisting  = "newvalue"
		testUndesiredKey                = "undesired.key"
	)

	It("Should copy desired labels", func() {
		srcLabels := map[string]string{
			testKubevirtIoKey:             testKubevirtIoValue,
			testInstancetypeKubevirtIoKey: testInstancetypeKubevirtIoValue,
			testUndesiredKey:              "undesired.key",
		}
		ds := &cdiv1.DataSource{}
		CopyAllowedLabels(srcLabels, ds, false)
		Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
		Expect(ds.Labels).To(HaveKeyWithValue(testInstancetypeKubevirtIoKey, testInstancetypeKubevirtIoValue))
		Expect(ds.Labels).ToNot(HaveKey(testUndesiredKey))
	})

	DescribeTable("Should overwrite existing labels", func(overwrite bool) {
		srcLabels := map[string]string{
			testKubevirtIoKeyExisting: testKubevirtIoNewValueExisting,
		}
		ds := &cdiv1.DataSource{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					testKubevirtIoKeyExisting: testKubevirtIoValueExisting,
				},
			},
		}
		CopyAllowedLabels(srcLabels, ds, overwrite)
		if overwrite {
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKeyExisting, testKubevirtIoNewValueExisting))
		} else {
			Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKeyExisting, testKubevirtIoValueExisting))
		}
	},
		Entry("when override enabled", true),
		Entry("not when override disabled", false),
	)
})

func createPvcNoSize(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
		},
	}
}
