# Containerized Data Importer metrics

| Name | Kind | Type | Description |
|------|------|------|-------------|
| kubevirt_cdi_clone_progress_total | Metric | Counter | The clone progress in percentage |
| kubevirt_cdi_cr_ready | Metric | Gauge | CDI install ready |
| kubevirt_cdi_dataimportcron_outdated | Metric | Gauge | DataImportCron has an outdated import |
| kubevirt_cdi_datavolume_pending | Metric | Gauge | Number of DataVolumes pending for default storage class to be configured |
| kubevirt_cdi_import_phase_info | Metric | Gauge | The current processing phase of the import |
| kubevirt_cdi_import_progress_total | Metric | Counter | The import progress in percentage |
| kubevirt_cdi_openstack_populator_progress_total | Metric | Counter | Progress of volume population |
| kubevirt_cdi_ovirt_progress_total | Metric | Counter | Progress of volume population |
| kubevirt_cdi_storageprofile_info | Metric | Gauge | `StorageProfiles` info labels: `storageclass`, `provisioner`, `complete` indicates if all storage profiles recommended PVC settings are complete, `default` indicates if it's the Kubernetes default storage class, `virtdefault` indicates if it's the default virtualization storage class, `rwx` indicates if the storage class supports `ReadWriteMany`, `smartclone` indicates if it supports snapshot or CSI based clone, `degraded` indicates it is not optimal for virtualization |
| cluster:kubevirt_cdi_clone_pods_high_restart:count | Recording rule | Gauge | The number of CDI clone pods with high restart count |
| cluster:kubevirt_cdi_import_pods_high_restart:count | Recording rule | Gauge | The number of CDI import pods with high restart count |
| cluster:kubevirt_cdi_operator_up:sum | Recording rule | Gauge | The number of CDI pods that are up |
| cluster:kubevirt_cdi_upload_pods_high_restart:count | Recording rule | Gauge | The number of CDI upload server pods with high restart count |
| kubevirt_cdi_clone_pods_high_restart | Recording rule | Gauge | [Deprecated] The number of CDI clone pods with high restart count |
| kubevirt_cdi_import_pods_high_restart | Recording rule | Gauge | [Deprecated] The number of CDI import pods with high restart count |
| kubevirt_cdi_operator_up | Recording rule | Gauge | [Deprecated] CDI operator status |
| kubevirt_cdi_upload_pods_high_restart | Recording rule | Gauge | [Deprecated] The number of CDI upload server pods with high restart count |

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
