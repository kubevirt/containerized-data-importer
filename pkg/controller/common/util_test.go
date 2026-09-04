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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
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

var _ = Describe("UpdateHTTPAnnotations", func() {
	It("Should set AnnInsecureSkipVerify to true when DataVolumeSourceHTTP.InsecureSkipVerify is true", func() {
		insecureSkipVerify := true
		annotations := map[string]string{}
		UpdateHTTPAnnotations(annotations, &cdiv1.DataVolumeSourceHTTP{
			URL:                "http://example.com",
			InsecureSkipVerify: &insecureSkipVerify,
		})
		Expect(annotations[AnnInsecureSkipVerify]).To(Equal("true"))
	})

	It("Should not set AnnInsecureSkipVerify when DataVolumeSourceHTTP.InsecureSkipVerify is false", func() {
		insecureSkipVerify := false
		annotations := map[string]string{}
		UpdateHTTPAnnotations(annotations, &cdiv1.DataVolumeSourceHTTP{
			URL:                "http://example.com",
			InsecureSkipVerify: &insecureSkipVerify,
		})
		_, exists := annotations[AnnInsecureSkipVerify]
		Expect(exists).To(BeFalse())
	})

	It("Should not set AnnInsecureSkipVerify when DataVolumeSourceHTTP.InsecureSkipVerify is absent", func() {
		annotations := map[string]string{}
		UpdateHTTPAnnotations(annotations, &cdiv1.DataVolumeSourceHTTP{
			URL: "http://example.com",
		})
		_, exists := annotations[AnnInsecureSkipVerify]
		Expect(exists).To(BeFalse())
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
		testCdiDatasourceKey            = "cdi.kubevirt.io/storage.import.datasource-name"
		testCdiDatasourceKeyValue       = "testdatasource"
	)

	It("Should copy desired labels", func() {
		srcLabels := map[string]string{
			testKubevirtIoKey:             testKubevirtIoValue,
			testInstancetypeKubevirtIoKey: testInstancetypeKubevirtIoValue,
			testUndesiredKey:              "undesired.key",
			testCdiDatasourceKey:          testCdiDatasourceKeyValue,
		}
		ds := &cdiv1.DataSource{}
		CopyAllowedLabels(srcLabels, ds, false)
		Expect(ds.Labels).To(HaveKeyWithValue(testKubevirtIoKey, testKubevirtIoValue))
		Expect(ds.Labels).To(HaveKeyWithValue(testInstancetypeKubevirtIoKey, testInstancetypeKubevirtIoValue))
		Expect(ds.Labels).To(HaveKeyWithValue(testCdiDatasourceKey, testCdiDatasourceKeyValue))
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

var _ = Describe("sortEvents", func() {
	It("Should sort events by timestamp but prioritize longer messages", func() {
		events := &v1.EventList{
			Items: []v1.Event{
				{LastTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Second)), Message: "third"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Second)), Message: "second"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Second)), Message: "first"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Second)), Message: "first long message"},
			},
		}
		sortEvents(events, false, "")
		Expect(events.Items[0].Message).To(Equal("first long message"))
		Expect(events.Items[1].Message).To(Equal("first"))
		Expect(events.Items[2].Message).To(Equal("second"))
		Expect(events.Items[3].Message).To(Equal("third"))

	})

	It("Should sort events by timestamp but prioritize prime messages", func() {
		events := &v1.EventList{
			Items: []v1.Event{
				{LastTimestamp: metav1.NewTime(time.Now().Add(-4 * time.Second)), Message: "[primeName] second prime"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Second)), Message: "second"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Second)), Message: "[primeName] first prime"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Second)), Message: "[primeName] first prime but more interesting"},
				{LastTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Second)), Message: "first"},
			},
		}
		sortEvents(events, true, "primeName")
		Expect(events.Items[0].Message).To(Equal("[primeName] first prime but more interesting"))
		Expect(events.Items[1].Message).To(Equal("[primeName] first prime"))
		Expect(events.Items[2].Message).To(Equal("[primeName] second prime"))
		Expect(events.Items[3].Message).To(Equal("first"))
		Expect(events.Items[4].Message).To(Equal("second"))
	})
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

var _ = Describe("IsWebhookPvcRenderingEnabled", func() {
	var createCDIConfigClient = func(policy cdiv1.WebhookPvcRenderingPolicy) *fake.ClientBuilder {
		s := scheme.Scheme
		Expect(cdiv1.AddToScheme(s)).To(Succeed())

		cdiConfig := &cdiv1.CDIConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ConfigName,
			},
			Spec: cdiv1.CDIConfigSpec{
				WebhookPvcRendering: policy,
			},
		}
		return fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(cdiConfig)
	}

	DescribeTable("should reflect WebhookPvcRendering config", func(policy cdiv1.WebhookPvcRenderingPolicy, expectedEnabled bool) {
		cl := createCDIConfigClient(policy).Build()
		enabled, err := IsWebhookPvcRenderingEnabled(cl)
		Expect(err).ToNot(HaveOccurred())
		Expect(enabled).To(Equal(expectedEnabled))
	},
		Entry("enabled by default when field is empty", cdiv1.WebhookPvcRenderingPolicy(""), true),
		Entry("enabled when explicitly set to Enabled", cdiv1.WebhookPvcRenderingEnabled, true),
		Entry("disabled when set to Disabled", cdiv1.WebhookPvcRenderingDisabled, false),
	)

	It("should return error when CDIConfig is not found", func() {
		s := scheme.Scheme
		Expect(cdiv1.AddToScheme(s)).To(Succeed())
		cl := fake.NewClientBuilder().WithScheme(s).Build()

		_, err := IsWebhookPvcRenderingEnabled(cl)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("SetRestrictedSecurityContext", func() {
	var podSpec *v1.PodSpec

	BeforeEach(func() {
		podSpec = &v1.PodSpec{
			Containers: []v1.Container{
				{Name: "main"},
			},
			InitContainers: []v1.Container{
				{Name: "init"},
			},
		}
	})

	DescribeTable("should enforce container-level SecurityContext fields",
		func(check func(sc *v1.SecurityContext)) {
			SetRestrictedSecurityContext(podSpec)
			for _, c := range append(podSpec.Containers, podSpec.InitContainers...) {
				Expect(c.SecurityContext).NotTo(BeNil(), "container %s", c.Name)
				check(c.SecurityContext)
			}
		},
		Entry("ReadOnlyRootFilesystem=true", func(sc *v1.SecurityContext) {
			Expect(sc.ReadOnlyRootFilesystem).NotTo(BeNil())
			Expect(*sc.ReadOnlyRootFilesystem).To(BeTrue())
		}),
		Entry("AllowPrivilegeEscalation=false", func(sc *v1.SecurityContext) {
			Expect(sc.AllowPrivilegeEscalation).NotTo(BeNil())
			Expect(*sc.AllowPrivilegeEscalation).To(BeFalse())
		}),
		Entry("RunAsNonRoot=true", func(sc *v1.SecurityContext) {
			Expect(sc.RunAsNonRoot).NotTo(BeNil())
			Expect(*sc.RunAsNonRoot).To(BeTrue())
		}),
		Entry("RunAsUser=QemuSubGid", func(sc *v1.SecurityContext) {
			Expect(sc.RunAsUser).NotTo(BeNil())
			Expect(*sc.RunAsUser).To(Equal(common.QemuSubGid))
		}),
		Entry("drops ALL capabilities", func(sc *v1.SecurityContext) {
			Expect(sc.Capabilities).NotTo(BeNil())
			Expect(sc.Capabilities.Drop).To(ContainElement(v1.Capability("ALL")))
		}),
		Entry("SeccompProfile=RuntimeDefault", func(sc *v1.SecurityContext) {
			Expect(sc.SeccompProfile).NotTo(BeNil())
			Expect(sc.SeccompProfile.Type).To(Equal(v1.SeccompProfileTypeRuntimeDefault))
		}),
	)

	It("should set pod-level SeccompProfile to RuntimeDefault", func() {
		SetRestrictedSecurityContext(podSpec)
		Expect(podSpec.SecurityContext).NotTo(BeNil())
		Expect(podSpec.SecurityContext.SeccompProfile).NotTo(BeNil())
		Expect(podSpec.SecurityContext.SeccompProfile.Type).To(Equal(v1.SeccompProfileTypeRuntimeDefault))
	})

	It("should enforce ReadOnlyRootFilesystem=true even if previously set to false", func() {
		podSpec.Containers[0].SecurityContext = &v1.SecurityContext{
			ReadOnlyRootFilesystem: ptr.To(false),
		}
		SetRestrictedSecurityContext(podSpec)
		Expect(*podSpec.Containers[0].SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
	})

	Context("FSGroup", func() {
		It("should set FSGroup when containers have VolumeMounts", func() {
			podSpec.Containers[0].VolumeMounts = []v1.VolumeMount{
				{Name: "data", MountPath: "/data"},
			}
			SetRestrictedSecurityContext(podSpec)
			Expect(podSpec.SecurityContext.FSGroup).NotTo(BeNil())
			Expect(*podSpec.SecurityContext.FSGroup).To(Equal(common.QemuSubGid))
		})

		It("should not set FSGroup when no VolumeMounts are present", func() {
			SetRestrictedSecurityContext(podSpec)
			if podSpec.SecurityContext.FSGroup != nil {
				Expect(*podSpec.SecurityContext.FSGroup).NotTo(Equal(common.QemuSubGid))
			}
		})
	})
})

var _ = Describe("AppendPrometheusCertVolume", func() {
	It("should add prometheus cert volume and mount to pod spec", func() {
		podSpec := &v1.PodSpec{
			Containers: []v1.Container{
				{Name: "test-container"},
			},
		}
		AppendPrometheusCertVolume(podSpec, "my-pod")

		Expect(podSpec.Volumes).To(HaveLen(1))
		Expect(podSpec.Volumes[0].Name).To(Equal(common.PrometheusCertVolName))
		Expect(podSpec.Volumes[0].VolumeSource.Secret).NotTo(BeNil())
		Expect(podSpec.Volumes[0].VolumeSource.Secret.SecretName).To(Equal(PrometheusCertSecretName("my-pod")))

		Expect(podSpec.Containers[0].VolumeMounts).To(HaveLen(1))
		Expect(podSpec.Containers[0].VolumeMounts[0].Name).To(Equal(common.PrometheusCertVolName))
		Expect(podSpec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.PrometheusCertDir))
		Expect(podSpec.Containers[0].VolumeMounts[0].ReadOnly).To(BeTrue())
	})
})

var _ = Describe("AppendTmpVolume", func() {
	It("should add emptyDir tmp volume and mount to pod spec", func() {
		podSpec := &v1.PodSpec{
			Containers: []v1.Container{
				{Name: "test-container"},
			},
		}
		AppendTmpVolume(podSpec)

		Expect(podSpec.Volumes).To(HaveLen(1))
		Expect(podSpec.Volumes[0].Name).To(Equal(common.TmpVolumeName))
		Expect(podSpec.Volumes[0].VolumeSource.EmptyDir).NotTo(BeNil())

		Expect(podSpec.Containers[0].VolumeMounts).To(HaveLen(1))
		Expect(podSpec.Containers[0].VolumeMounts[0].Name).To(Equal(common.TmpVolumeName))
		Expect(podSpec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.TmpMountPath))
	})
})

var _ = Describe("PrometheusCertSecretName", func() {
	It("should return correct secret name", func() {
		Expect(PrometheusCertSecretName("importer-pod")).To(Equal("importer-pod" + PrometheusCertSecretSuffix))
	})
})

var _ = Describe("GeneratePrometheusCertBytes", func() {
	It("should generate valid cert and key bytes", func() {
		certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("test-pod", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(certBytes).NotTo(BeEmpty())
		Expect(keyBytes).NotTo(BeEmpty())
	})
})

var _ = Describe("CreatePrometheusCertSecret and SetPrometheusCertSecretOwnerRef", func() {
	var pod *v1.Pod

	BeforeEach(func() {
		pod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				UID:       types.UID("test-uid"),
			},
		}
	})

	It("should create a secret with correct name and data (no OwnerRef)", func() {
		s := scheme.Scheme
		Expect(v1.AddToScheme(s)).To(Succeed())
		cl := fake.NewClientBuilder().WithScheme(s).Build()

		err := CreatePrometheusCertSecret(context.TODO(), cl, pod.Name, pod.Namespace, nil)
		Expect(err).NotTo(HaveOccurred())

		secret := &v1.Secret{}
		err = cl.Get(context.TODO(), types.NamespacedName{
			Name:      PrometheusCertSecretName(pod.Name),
			Namespace: pod.Namespace,
		}, secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data).To(HaveKey("tls.crt"))
		Expect(secret.Data).To(HaveKey("tls.key"))
		Expect(secret.OwnerReferences).To(BeEmpty())
	})

	It("should set OwnerReference after pod exists", func() {
		s := scheme.Scheme
		Expect(v1.AddToScheme(s)).To(Succeed())
		cl := fake.NewClientBuilder().WithScheme(s).Build()

		err := CreatePrometheusCertSecret(context.TODO(), cl, pod.Name, pod.Namespace, nil)
		Expect(err).NotTo(HaveOccurred())

		err = SetPrometheusCertSecretOwnerRef(context.TODO(), cl, pod)
		Expect(err).NotTo(HaveOccurred())

		secret := &v1.Secret{}
		err = cl.Get(context.TODO(), types.NamespacedName{
			Name:      PrometheusCertSecretName(pod.Name),
			Namespace: pod.Namespace,
		}, secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.OwnerReferences).To(HaveLen(1))
		Expect(secret.OwnerReferences[0].Name).To(Equal(pod.Name))
		Expect(secret.OwnerReferences[0].UID).To(Equal(pod.UID))
	})

	It("should not error if secret already exists", func() {
		s := scheme.Scheme
		Expect(v1.AddToScheme(s)).To(Succeed())

		existingSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      PrometheusCertSecretName(pod.Name),
				Namespace: pod.Namespace,
			},
		}
		cl := fake.NewClientBuilder().WithScheme(s).WithObjects(existingSecret).Build()

		err := CreatePrometheusCertSecret(context.TODO(), cl, pod.Name, pod.Namespace, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should apply installer labels when provided", func() {
		s := scheme.Scheme
		Expect(v1.AddToScheme(s)).To(Succeed())
		cl := fake.NewClientBuilder().WithScheme(s).Build()

		labels := map[string]string{
			"app.kubernetes.io/part-of": "testing",
			"app.kubernetes.io/version": "v1.0.0",
		}
		err := CreatePrometheusCertSecret(context.TODO(), cl, pod.Name, pod.Namespace, labels)
		Expect(err).NotTo(HaveOccurred())

		secret := &v1.Secret{}
		err = cl.Get(context.TODO(), types.NamespacedName{
			Name:      PrometheusCertSecretName(pod.Name),
			Namespace: pod.Namespace,
		}, secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", "testing"))
	})
})
