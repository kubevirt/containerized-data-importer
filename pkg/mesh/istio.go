package mesh

import (
	v1 "k8s.io/api/core/v1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// IstioPrestopHook returns a preStop hook to terminate istio envoy proxy.
// See https://discuss.istio.io/t/best-practices-for-jobs/4968.
func IstioPreStopHook() *v1.LifecycleHandler {
	return &v1.LifecycleHandler{
		Exec: &v1.ExecAction{
			Command: []string{
				"/usr/bin/bash",
				"-c",
				"curl -X POST http://localhost:15000/quitquitquit",
			},
		},
	}
}

// isIstioMeshed checks if the annotation sidecar.istio.io/inject is true
func isIstioMeshed(annotations map[string]string) bool {
	return annotations[cc.AnnPodSidecarInjectionIstio] == "true"
}
