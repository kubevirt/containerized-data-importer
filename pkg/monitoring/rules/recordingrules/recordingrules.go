package recordingrules

import (
	"fmt"
	"strconv"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

// MetricOpts represent CDI Prometheus Metrics
type MetricOpts struct {
	Name string
	Help string
	Type string
}

// RecordRulesDesc represent CDI Prometheus Record Rules
type RecordRulesDesc struct {
	Opts MetricOpts
	Expr string
}

// GetRecordRulesDesc returns CDI Prometheus Record Rules
func GetRecordRulesDesc(namespace string) []RecordRulesDesc {
	return []RecordRulesDesc{
		{
			MetricOpts{
				"kubevirt_cdi_operator_up",
				"CDI operator status",
				"Gauge",
			},
			fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
		},
		{
			MetricOpts{
				"kubevirt_cdi_import_pods_high_restart",
				"The number of CDI import pods with high restart count",
				"Gauge",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_upload_pods_high_restart",
				"The number of CDI upload server pods with high restart count",
				"Gauge",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s) or on() vector(0)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_clone_pods_high_restart",
				"The number of CDI clone pods with high restart count",
				"Gauge",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s) or on() vector(0)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
	}
}
