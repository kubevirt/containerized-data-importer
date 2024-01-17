package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"strings"
)

var _ = Describe("Patches", func() {
	namespace := "fake-namespace"

	getControllerDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      common.CDIControllerResourceName,
			},
			Spec: appsv1.DeploymentSpec{},
		}
	}

	Context("generically apply patches", func() {

		flags := &cdiv1.Flags{
			Controller: map[string]string{
				"v": "4",
			},
		}

		customizer, _ := NewCustomizer(cdiv1.CustomizeComponents{
			Patches: []cdiv1.CustomizeComponentsPatch{
				{
					ResourceName: common.CDIControllerResourceName,
					ResourceType: "Deployment",
					Patch:        `{"metadata":{"labels":{"new-key":"added-this-label"}}}`,
					Type:         cdiv1.StrategicMergePatchType,
				},
				{
					ResourceName: "*",
					ResourceType: "Deployment",
					Patch:        `{"spec":{"template":{"spec":{"imagePullSecrets":[{"name":"image-pull"}]}}}}`,
					Type:         cdiv1.StrategicMergePatchType,
				},
			},
			Flags: flags,
		})

		deployment := getControllerDeployment()

		It("should apply to deployments", func() {
			deployments := []*appsv1.Deployment{
				deployment,
			}

			err := customizer.GenericApplyPatches(deployments)
			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.ObjectMeta.Labels["new-key"]).To(Equal("added-this-label"))
			Expect(deployment.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("image-pull"))
			// check flags are applied
			expectedFlags := []string{common.CDIControllerResourceName}
			expectedFlags = append(expectedFlags, flagsToArray(flags.Controller)...)
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(Equal(expectedFlags))

			// check objects implement runtime.Object
			err = customizer.GenericApplyPatches([]string{"string"})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("apply patch", func() {

		It("should not error on empty patch", func() {
			err := applyPatch(nil, cdiv1.CustomizeComponentsPatch{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("get hash", func() {
		patch1 := cdiv1.CustomizeComponentsPatch{
			ResourceName: common.CDIControllerResourceName,
			ResourceType: "Deployment",
			Patch:        `{"metadata":{"labels":{"new-key":"added-this-label"}}}`,
			Type:         cdiv1.StrategicMergePatchType,
		}
		patch2 := cdiv1.CustomizeComponentsPatch{
			ResourceName: common.CDIApiServerResourceName,
			ResourceType: "Deployment",
			Patch:        `{"metadata":{"labels":{"my-custom-label":"custom-label"}}}`,
			Type:         cdiv1.StrategicMergePatchType,
		}
		patch3 := cdiv1.CustomizeComponentsPatch{
			ResourceName: common.CDIControllerResourceName,
			ResourceType: "Deployment",
			Patch:        `{"metadata":{"annotation":{"key":"value"}}}`,
			Type:         cdiv1.StrategicMergePatchType,
		}
		c1 := cdiv1.CustomizeComponents{
			Patches: []cdiv1.CustomizeComponentsPatch{patch1, patch2, patch3},
		}

		c2 := cdiv1.CustomizeComponents{
			Patches: []cdiv1.CustomizeComponentsPatch{patch2, patch1, patch3},
		}

		flags1 := &cdiv1.Flags{
			API: map[string]string{
				"v": "4",
			},
		}

		flags2 := &cdiv1.Flags{
			API: map[string]string{
				"v": "1",
			},
		}

		It("should be equal", func() {
			h1, err := getHash(c1)
			Expect(err).ToNot(HaveOccurred())
			h2, err := getHash(c2)
			Expect(err).ToNot(HaveOccurred())

			Expect(h1).To(Equal(h2))
		})

		It("should not be equal", func() {
			c1.Flags = flags1
			c2.Flags = flags2

			h1, err := getHash(c1)
			Expect(err).ToNot(HaveOccurred())
			h2, err := getHash(c2)
			Expect(err).ToNot(HaveOccurred())

			Expect(h1).ToNot(Equal(h2))
		})
	})

	DescribeTable("valueMatchesKey", func(value, key string, expected bool) {
		matches := valueMatchesKey(value, key)
		Expect(matches).To(Equal(expected))
	},
		Entry("should match wildcard", "*", "Deployment", true),
		Entry("should match with different cases", "deployment", "Deployment", true),
		Entry("should not match", "Service", "Deployment", false),
	)

	Describe("Config controller flags", func() {
		flags := map[string]string{
			"flag-one":  "1",
			"flag":      "3",
			"bool-flag": "",
		}
		resource := "Deployment"

		It("should return flags in the proper format", func() {
			fa := flagsToArray(flags)
			Expect(fa).To(HaveLen(5))

			Expect(strings.Join(fa, " ")).To(ContainSubstring("--flag-one 1"))
			Expect(strings.Join(fa, " ")).To(ContainSubstring("--flag 3"))
			Expect(strings.Join(fa, " ")).To(ContainSubstring("--bool-flag"))
		})

		It("should add flag patch", func() {
			patches := addFlagsPatch(common.CDIApiServerResourceName, resource, flags, []cdiv1.CustomizeComponentsPatch{})
			Expect(patches).To(HaveLen(1))
			patch := patches[0]

			Expect(patch.ResourceName).To(Equal(common.CDIApiServerResourceName))
			Expect(patch.ResourceType).To(Equal(resource))
		})

		It("should return empty patch", func() {
			patches := addFlagsPatch(common.CDIApiServerResourceName, resource, map[string]string{}, []cdiv1.CustomizeComponentsPatch{})
			Expect(patches).To(BeEmpty())
		})

		It("should chain patches", func() {
			patches := addFlagsPatch(common.CDIApiServerResourceName, resource, flags, []cdiv1.CustomizeComponentsPatch{})
			Expect(patches).To(HaveLen(1))

			patches = addFlagsPatch(common.CDIControllerResourceName, resource, flags, patches)
			Expect(patches).To(HaveLen(2))
		})

		It("should return all flag patches", func() {
			f := &cdiv1.Flags{
				API: flags,
			}

			patches := flagsToPatches(f)
			Expect(patches).To(HaveLen(1))
		})
	})
})
