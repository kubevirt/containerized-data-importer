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

type MetricsKey string

const (
	ReadyGauge             MetricsKey = "readyGauge"
	IncompleteProfile      MetricsKey = "incompleteProfile"
	DataImportCronOutdated MetricsKey = "dataImportCronOutdated"
	CloneProgress          MetricsKey = "cloneProgress"
)

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
		Name: "kubevirt_cdi_incomplete_storageprofiles_total",
		Help: "Total number of incomplete and hence unusable StorageProfile",
		Type: "Gauge",
	},
	ReadyGauge: {
		Name: "kubevirt_cdi_cr_ready",
		Help: "CDI CR Ready",
		Type: "Gauge",
	},
}

// GetRecordRulesDesc returns CDI Prometheus Record Rules
func GetRecordRulesDesc(namespace string) []RecordRulesDesc {
	return []RecordRulesDesc{
		{
			MetricOpts{
				"kubevirt_cdi_operator_up_total",
				"CDI operator status",
				"Gauge",
			},
			fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
		},
		{
			MetricOpts{
				"kubevirt_cdi_import_dv_unusual_restartcount_total",
				"Total restart count in CDI Data Volume importer pod",
				"Counter",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.ImporterPodName, common.ImporterPodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_upload_dv_unusual_restartcount_total",
				"Total restart count in CDI Data Volume upload server pod",
				"Counter",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'%s-.*', container='%s'} > %s)", common.UploadPodName, common.UploadServerPodname, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_clone_dv_unusual_restartcount_total",
				"Total restart count in CDI Data Volume cloner pod",
				"Counter",
			},
			fmt.Sprintf("count(kube_pod_container_status_restarts_total{pod=~'.*%s', container='%s'} > %s)", common.ClonerSourcePodNameSuffix, common.ClonerSourcePodName, strconv.Itoa(common.UnusualRestartCountThreshold)),
		},
		{
			MetricOpts{
				"kubevirt_cdi_dataimportcron_outdated_total",
				"Total count of outdated DataImportCron imports",
				"Counter",
			},
			"sum(kubevirt_cdi_dataimportcron_outdated or vector(0))",
		},
	}
}
