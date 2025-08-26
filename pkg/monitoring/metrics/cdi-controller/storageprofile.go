package cdicontroller

import (
	"github.com/prometheus/client_golang/prometheus"
	ioprometheusclient "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

const (
	counterLabelStorageClass = "storageclass"
	counterLabelProvisioner  = "provisioner"
	counterLabelComplete     = "complete"
	counterLabelDefault      = "default"
	counterLabelVirtDefault  = "virtdefault"
	counterLabelRWX          = "rwx"
	counterLabelSmartClone   = "smartclone"
	counterLabelDegraded     = "degraded"
)

var (
	storageMetrics = []operatormetrics.Metric{
		storageProfileStatus,
	}

	storageProfileStatus = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_cdi_storageprofile_info",
			Help: "`StorageProfiles` info labels: " +
				"`storageclass`, `provisioner`, " +
				"`complete` indicates if all storage profiles recommended PVC settings are complete, " +
				"`default` indicates if it's the Kubernetes default storage class, " +
				"`virtdefault` indicates if it's the default virtualization storage class, " +
				"`rwx` indicates if the storage class supports `ReadWriteMany`, " +
				"`smartclone` indicates if it supports snapshot or CSI based clone, " +
				"`degraded` indicates it is not optimal for virtualization",
		},
		[]string{
			counterLabelStorageClass,
			counterLabelProvisioner,
			counterLabelComplete,
			counterLabelDefault,
			counterLabelVirtDefault,
			counterLabelRWX,
			counterLabelSmartClone,
			counterLabelDegraded,
		},
	)
)

// SetStorageProfileStatus sets storageProfileStatus value
func SetStorageProfileStatus(labels prometheus.Labels, status int) {
	storageProfileStatus.With(labels).Set(float64(status))
}

// GetStorageProfileStatus returns the storageProfileStatus value
func GetStorageProfileStatus(labels prometheus.Labels) float64 {
	dto := &ioprometheusclient.Metric{}
	_ = storageProfileStatus.With(labels).Write(dto)
	return dto.Gauge.GetValue()
}

// DeleteStorageProfileStatus deletes metrics by their labels, and return the number of deleted metrics
func DeleteStorageProfileStatus(labels prometheus.Labels) int {
	return storageProfileStatus.DeletePartialMatch(labels)
}
