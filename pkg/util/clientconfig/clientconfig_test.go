package clientconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/rest"
)

var _ = Describe("ApplyRateLimits", func() {
	It("should keep existing values when zero values are given", func() {
		cfg := &rest.Config{QPS: 20, Burst: 30}
		ApplyRateLimits(cfg, 0, 0)
		Expect(cfg.QPS).To(Equal(float32(20)))
		Expect(cfg.Burst).To(Equal(30))
	})

	It("should keep unset values unset when zero values are given", func() {
		cfg := &rest.Config{}
		ApplyRateLimits(cfg, 0, 0)
		Expect(cfg.QPS).To(BeZero())
		Expect(cfg.Burst).To(BeZero())
	})

	It("should override both values", func() {
		cfg := &rest.Config{QPS: 20, Burst: 30}
		ApplyRateLimits(cfg, 100, 200)
		Expect(cfg.QPS).To(Equal(float32(100)))
		Expect(cfg.Burst).To(Equal(200))
	})

	It("should override only QPS", func() {
		cfg := &rest.Config{QPS: 20, Burst: 30}
		ApplyRateLimits(cfg, 50.5, 0)
		Expect(cfg.QPS).To(Equal(float32(50.5)))
		Expect(cfg.Burst).To(Equal(30))
	})

	It("should override only burst", func() {
		cfg := &rest.Config{QPS: 20, Burst: 30}
		ApplyRateLimits(cfg, 0, 60)
		Expect(cfg.QPS).To(Equal(float32(20)))
		Expect(cfg.Burst).To(Equal(60))
	})

	It("should ignore negative values", func() {
		cfg := &rest.Config{QPS: 20, Burst: 30}
		ApplyRateLimits(cfg, -1, -1)
		Expect(cfg.QPS).To(Equal(float32(20)))
		Expect(cfg.Burst).To(Equal(30))
	})
})
