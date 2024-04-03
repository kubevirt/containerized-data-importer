package mesh

import (
	v1 "k8s.io/api/core/v1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// L5dPreStopHook returns a lifecycle preStop hook to terminate linkerd proxy.
// See https://linkerd.io/2.15/tasks/graceful-shutdown/#graceful-shutdown-of-job-and-cronjob-resources.
func L5dPreStopHook() *v1.LifecycleHandler {
	return &v1.LifecycleHandler{
		Exec: &v1.ExecAction{
			Command: []string{
				"/usr/bin/bash",
				"-c",
				"curl -X POST http://localhost:4191/shutdown",
			},
		},
	}
}

// isMeshed checks if the annotation linkerd.io/inject is true
func isL5dMeshed(annotations map[string]string) bool {
	return annotations[cc.AnnPodSidecarInjectionLinkerd] == "enabled"
}
