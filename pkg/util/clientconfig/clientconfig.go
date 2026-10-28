package clientconfig

import (
	"k8s.io/client-go/rest"
)

// ApplyRateLimits overrides the client-side rate limits (QPS and burst) of the given rest config.
// A zero value leaves the corresponding setting untouched, so the defaults of the
// client library in use keep applying unless explicitly overridden.
func ApplyRateLimits(cfg *rest.Config, qps float64, burst int) {
	if qps > 0 {
		cfg.QPS = float32(qps)
	}
	if burst > 0 {
		cfg.Burst = burst
	}
}
