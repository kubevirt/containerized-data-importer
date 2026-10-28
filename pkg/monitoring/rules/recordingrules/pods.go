package recordingrules

import (
	"fmt"
	"strconv"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

	"k8s.io/apimachinery/pkg/util/intstr"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

var (
	highRestartThreshold = strconv.Itoa(common.UnusualRestartCountThreshold)

	// Selectors for the containers of CDI workload pods that restarted more than
	// the threshold, shared by the cluster-wide and per-namespace rules.
	importPodsHighRestart = fmt.Sprintf("kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s", common.ImporterPodName, common.ImporterPodName, highRestartThreshold)
	uploadPodsHighRestart = fmt.Sprintf("kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s", common.UploadPodName, common.UploadServerPodname, highRestartThreshold)
	clonePodsHighRestart  = fmt.Sprintf("kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, highRestartThreshold)
)

var podsRecordingRules = []operatorrules.RecordingRule{
	gaugeRule(
		"kubevirt_cdi_import_pods_high_restart",
		"[Deprecated] The number of CDI import pods with high restart count",
		"count("+importPodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"cluster:kubevirt_cdi_import_pods_high_restart:count",
		"The number of CDI import pods with high restart count",
		"count("+importPodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"kubevirt_cdi_upload_pods_high_restart",
		"[Deprecated] The number of CDI upload server pods with high restart count",
		"count("+uploadPodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"cluster:kubevirt_cdi_upload_pods_high_restart:count",
		"The number of CDI upload server pods with high restart count",
		"count("+uploadPodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"kubevirt_cdi_clone_pods_high_restart",
		"[Deprecated] The number of CDI clone pods with high restart count",
		"count("+clonePodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"cluster:kubevirt_cdi_clone_pods_high_restart:count",
		"The number of CDI clone pods with high restart count",
		"count("+clonePodsHighRestart+") or on() vector(0)",
	),
	gaugeRule(
		"namespace:kubevirt_cdi_import_pods_high_restart:count",
		"The number of CDI import pods with high restart count, per namespace",
		"count by (namespace) ("+importPodsHighRestart+")",
	),
	gaugeRule(
		"namespace:kubevirt_cdi_upload_pods_high_restart:count",
		"The number of CDI upload server pods with high restart count, per namespace",
		"count by (namespace) ("+uploadPodsHighRestart+")",
	),
	gaugeRule(
		"namespace:kubevirt_cdi_clone_pods_high_restart:count",
		"The number of CDI clone pods with high restart count, per namespace",
		"count by (namespace) ("+clonePodsHighRestart+")",
	),
}

// gaugeRule returns a gauge recording rule.
func gaugeRule(name, help, expr string) operatorrules.RecordingRule {
	return operatorrules.RecordingRule{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: name,
			Help: help,
		},
		MetricType: operatormetrics.GaugeType,
		Expr:       intstr.FromString(expr),
	}
}
