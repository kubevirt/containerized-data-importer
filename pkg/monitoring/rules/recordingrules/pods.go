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
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_import_pods_high_restart",
			Help: "[Deprecated] The number of CDI import pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + importPodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_import_pods_high_restart:count",
			Help: "The number of CDI import pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + importPodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_upload_pods_high_restart",
			Help: "[Deprecated] The number of CDI upload server pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + uploadPodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_upload_pods_high_restart:count",
			Help: "The number of CDI upload server pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + uploadPodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_clone_pods_high_restart",
			Help: "[Deprecated] The number of CDI clone pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + clonePodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_clone_pods_high_restart:count",
			Help: "The number of CDI clone pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count(" + clonePodsHighRestart + ") or on() vector(0)",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "namespace:kubevirt_cdi_import_pods_high_restart:count",
			Help: "The number of CDI import pods with high restart count, per namespace",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count by (namespace) (" + importPodsHighRestart + ")",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "namespace:kubevirt_cdi_upload_pods_high_restart:count",
			Help: "The number of CDI upload server pods with high restart count, per namespace",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count by (namespace) (" + uploadPodsHighRestart + ")",
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "namespace:kubevirt_cdi_clone_pods_high_restart:count",
			Help: "The number of CDI clone pods with high restart count, per namespace",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			"count by (namespace) (" + clonePodsHighRestart + ")",
		),
	},
}
