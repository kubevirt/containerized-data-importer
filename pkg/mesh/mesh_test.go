package mesh

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var _ = Describe("Mesh test", func() {
	It("Linkerd preStop hook", func() {
		ctr := &v1.Container{}
		ApplyPreStopHook(createObjectMeta(map[string]string{cc.AnnPodSidecarInjectionLinkerd: "enabled"}), ctr)
		Expect(ctr.Lifecycle).ToNot(BeNil())
		Expect(ctr.Lifecycle.PreStop).To(Equal(L5dPreStopHook()))
	})

	It("Istio preStop hook", func() {
		ctr := &v1.Container{}
		ApplyPreStopHook(createObjectMeta(map[string]string{cc.AnnPodSidecarInjectionIstio: "true"}), ctr)
		Expect(ctr.Lifecycle).ToNot(BeNil())
		Expect(ctr.Lifecycle.PreStop).To(Equal(IstioPreStopHook()))
	})

	It("Not meshed", func() {
		ctr := &v1.Container{}
		ApplyPreStopHook(createObjectMeta(map[string]string{}), ctr)
		Expect(ctr.Lifecycle).To(BeNil())
	})

	It("Pod linkerd preStop hook", func() {
		pod := &v1.Pod{
			ObjectMeta: createObjectMeta(map[string]string{cc.AnnPodSidecarInjectionLinkerd: "enabled"}),
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "test",
					},
				},
			},
		}
		ApplyPreStopHook(pod.ObjectMeta, &pod.Spec.Containers[0])
		Expect(pod.Spec.Containers[0].Lifecycle).ToNot(BeNil())
		Expect(pod.Spec.Containers[0].Lifecycle.PreStop).To(Equal(L5dPreStopHook()))
	})

	It("Pod istio preStop hook", func() {
		pod := &v1.Pod{
			ObjectMeta: createObjectMeta(map[string]string{cc.AnnPodSidecarInjectionIstio: "true"}),
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "test",
					},
				},
			},
		}
		ApplyPreStopHook(pod.ObjectMeta, &pod.Spec.Containers[0])
		Expect(pod.Spec.Containers[0].Lifecycle).ToNot(BeNil())
		Expect(pod.Spec.Containers[0].Lifecycle.PreStop).To(Equal(IstioPreStopHook()))
	})
})

func createObjectMeta(annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Annotations: annotations,
	}
}
