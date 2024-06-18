# Containerized Data Importer metrics

### kubevirt_cdi_clone_pods_high_restart
The number of CDI clone pods with high restart count. Type: Gauge.

### kubevirt_cdi_cr_ready
CDI install ready. Type: Gauge.

### kubevirt_cdi_dataimportcron_outdated
DataImportCron has an outdated import. Type: Gauge.

### kubevirt_cdi_datavolume_pending
Number of DataVolumes pending for default storage class to be configured. Type: Gauge.

### kubevirt_cdi_import_pods_high_restart
The number of CDI import pods with high restart count. Type: Gauge.

### kubevirt_cdi_operator_up
CDI operator status. Type: Gauge.

### kubevirt_cdi_storageprofile_info
`StorageProfiles` info labels: `storageclass`, `provisioner`, `complete` indicates if all storage profiles recommended PVC settings are complete, `default` indicates if it's the Kubernetes default storage class, `virtdefault` indicates if it's the default virtualization storage class, `rwx` indicates if the storage class supports `ReadWriteMany`, `smartclone` indicates if it supports snapshot or CSI based clone, `degraded` indicates it is not optimal for virtualization. Type: Gauge.

### kubevirt_cdi_upload_pods_high_restart
The number of CDI upload server pods with high restart count. Type: Gauge.

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
