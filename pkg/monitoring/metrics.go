package monitoring

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

// MetricsKey creates variables for metric reference
type MetricsKey string

// All metrics names for reference
const (
	ReadyGauge             MetricsKey = "readyGauge"
	IncompleteProfile      MetricsKey = "incompleteProfile"
	DataImportCronOutdated MetricsKey = "dataImportCronOutdated"
	CloneProgress          MetricsKey = "cloneProgress"
)

// MetricOptsList list all CDI metrics
var MetricOptsList = map[MetricsKey]MetricOpts{
	CloneProgress: {
		Name: "clone_progress",
		Help: "The clone progress in percentage",
		Type: "Counter",
	},
	DataImportCronOutdated: {
		Name: "kubevirt_cdi_dataimportcron_outdated",
		Help: "DataImportCron has an outdated import",
		Type: "Gauge",
	},
	IncompleteProfile: {
		Name: "kubevirt_cdi_incomplete_storageprofiles",
		Help: "Total number of incomplete and hence unusable StorageProfile",
		Type: "Gauge",
	},
	ReadyGauge: {
		Name: "kubevirt_cdi_cr_ready",
		Help: "CDI install ready",
		Type: "Gauge",
	},
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
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_upload_pods_high_restart",
				"The number of CDI upload server pods with high restart count",
				"Gauge",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_clone_pods_high_restart",
				"The number of CDI clone pods with high restart count",
				"Gauge",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_dataimportcron_outdated_aggregated",
				"Total count of outdated DataImportCron imports",
				"Gauge",
			},
			"sum(kubevirt_cdi_dataimportcron_outdated or vector(0))",
		},
	}
}
