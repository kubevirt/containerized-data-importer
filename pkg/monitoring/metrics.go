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
	CloneProgress          MetricsKey = "cloneProgress"
	DataImportCronOutdated MetricsKey = "dataImportCronOutdated"
	StorageProfileStatus   MetricsKey = "storageProfileStatus"
	ReadyGauge             MetricsKey = "readyGauge"
	DataVolumePending      MetricsKey = "dataVolumePending"
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
	StorageProfileStatus: {
		Name: "kubevirt_cdi_storageprofile_info",
		Help: "`StorageProfiles` info labels: " +
			"`storageclass`, `provisioner`, " +
			"`complete` indicates if all storage profiles recommended PVC settings are complete, " +
			"`default` indicates if it's the Kubernetes default storage class, " +
			"`virtdefault` indicates if it's the default virtualization storage class, " +
			"`rwx` indicates if the storage class supports `ReadWriteMany`, " +
			"`smartclone` indicates if it supports snapshot or CSI based clone",
		Type: "Gauge",
	},
	ReadyGauge: {
		Name: "kubevirt_cdi_cr_ready",
		Help: "CDI install ready",
		Type: "Gauge",
	},
	DataVolumePending: {
		Name: "kubevirt_cdi_datavolume_pending",
		Help: "Number of DataVolumes pending for default storage class to be configured",
		Type: "Gauge",
	},
}

// InternalMetricOptsList list all CDI metrics used for internal purposes only
var InternalMetricOptsList = map[MetricsKey]MetricOpts{
	CloneProgress: {
		Name: "clone_progress",
		Help: "The clone progress in percentage",
		Type: "Counter",
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
