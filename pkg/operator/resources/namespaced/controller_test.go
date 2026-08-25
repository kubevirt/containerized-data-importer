package namespaced

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operator Resources Controller Suite")
}

var _ = Describe("cdi-controller Deployment", func() {
	var deployment = createControllerDeployment(
		"controller:latest", "importer:latest", "cloner:latest",
		"ovirt-populator:latest", "uploadserver:latest",
		"1", "Always", nil, "", nil, 1,
	)

	It("should set ReadOnlyRootFilesystem to true", func() {
		containers := deployment.Spec.Template.Spec.Containers
		Expect(containers).NotTo(BeEmpty())
		for _, c := range containers {
			Expect(c.SecurityContext).NotTo(BeNil(), "container %s", c.Name)
			Expect(c.SecurityContext.ReadOnlyRootFilesystem).NotTo(BeNil(), "container %s", c.Name)
			Expect(*c.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue(), "container %s", c.Name)
		}
	})

	It("should have an emptyDir volume for /tmp", func() {
		var found bool
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Name == common.TmpVolumeName {
				Expect(vol.VolumeSource.EmptyDir).NotTo(BeNil())
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "expected volume %s to be present", common.TmpVolumeName)
	})

	It("should mount /tmp on all containers", func() {
		for _, c := range deployment.Spec.Template.Spec.Containers {
			var hasTmpMount bool
			for _, m := range c.VolumeMounts {
				if m.Name == common.TmpVolumeName && m.MountPath == common.TmpMountPath {
					hasTmpMount = true
					break
				}
			}
			Expect(hasTmpMount).To(BeTrue(), "container %s should have /tmp mount", c.Name)
		}
	})

	It("should use /tmp/ready in readiness probe", func() {
		container := deployment.Spec.Template.Spec.Containers[0]
		Expect(container.ReadinessProbe).NotTo(BeNil())
		Expect(container.ReadinessProbe.Exec).NotTo(BeNil())
		Expect(container.ReadinessProbe.Exec.Command).To(ContainElement("/tmp/ready"))
	})
})

var _ = DescribeTable("deployment SecurityContext",
	func(create func() corev1.SecurityContext) {
		sc := create()
		Expect(sc.ReadOnlyRootFilesystem).NotTo(BeNil())
		Expect(*sc.ReadOnlyRootFilesystem).To(BeTrue())
		Expect(sc.AllowPrivilegeEscalation).NotTo(BeNil())
		Expect(*sc.AllowPrivilegeEscalation).To(BeFalse())
	},
	Entry("cdi-apiserver", func() corev1.SecurityContext {
		d := createAPIServerDeployment("img:latest", "1", "Always", nil, "", nil, 1)
		return *d.Spec.Template.Spec.Containers[0].SecurityContext
	}),
	Entry("cdi-uploadproxy", func() corev1.SecurityContext {
		d := createUploadProxyDeployment("img:latest", "1", "Always", nil, "", nil, 1)
		return *d.Spec.Template.Spec.Containers[0].SecurityContext
	}),
)
