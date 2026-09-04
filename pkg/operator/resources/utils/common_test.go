package utils

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operator Resources Utils Suite")
}

func verifyRestrictedSecurityContext(sc *corev1.SecurityContext) {
	ExpectWithOffset(1, sc).NotTo(BeNil())
	ExpectWithOffset(1, sc.ReadOnlyRootFilesystem).NotTo(BeNil())
	ExpectWithOffset(1, *sc.ReadOnlyRootFilesystem).To(BeTrue())
	ExpectWithOffset(1, sc.AllowPrivilegeEscalation).NotTo(BeNil())
	ExpectWithOffset(1, *sc.AllowPrivilegeEscalation).To(BeFalse())
	ExpectWithOffset(1, sc.RunAsNonRoot).NotTo(BeNil())
	ExpectWithOffset(1, *sc.RunAsNonRoot).To(BeTrue())
	ExpectWithOffset(1, sc.Capabilities).NotTo(BeNil())
	ExpectWithOffset(1, sc.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
	ExpectWithOffset(1, sc.SeccompProfile).NotTo(BeNil())
	ExpectWithOffset(1, sc.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
}

var _ = DescribeTable("container factory SecurityContext",
	func(create func() corev1.Container) {
		container := create()
		verifyRestrictedSecurityContext(container.SecurityContext)
	},
	Entry("CreateContainer", func() corev1.Container {
		return CreateContainer("test", "img:latest", "1", "Always")
	}),
	Entry("CreatePortsContainer", func() corev1.Container {
		return CreatePortsContainer("test", "img:latest", "Always", []corev1.ContainerPort{{ContainerPort: 8080}})
	}),
)
