package recordingrules

import (
	"fmt"
	"strconv"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

	"k8s.io/apimachinery/pkg/util/intstr"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

var podsRecordingRules = []operatorrules.RecordingRule{
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_import_pods_high_restart",
			Help: "[Deprecated] The number of CDI import pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_import_pods_high_restart:count",
			Help: "The number of CDI import pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_upload_pods_high_restart",
			Help: "[Deprecated] The number of CDI upload server pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_upload_pods_high_restart:count",
			Help: "The number of CDI upload server pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_clone_pods_high_restart",
			Help: "[Deprecated] The number of CDI clone pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s) or on() vector(0)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
	{
		MetricsOpts: operatormetrics.MetricOpts{
			Name: "cluster:kubevirt_cdi_clone_pods_high_restart:count",
			Help: "The number of CDI clone pods with high restart count",
		},
		MetricType: operatormetrics.GaugeType,
		Expr: intstr.FromString(
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s) or on() vector(0)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		),
	},
}
