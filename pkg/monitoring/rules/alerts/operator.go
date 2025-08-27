package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var operatorAlerts = []promv1.Rule{
	{
		Alert: "CDIOperatorDown",
		Expr:  intstr.FromString("kubevirt_cdi_operator_up == 0"),
		For:   (*promv1.Duration)(ptr.To("10m")),
		Annotations: map[string]string{
			"summary": "CDI operator is down",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "critical",
			operatorHealthImpactLabelKey: "critical",
		},
	},
	{
		Alert: "CDINotReady",
		Expr:  intstr.FromString("kubevirt_cdi_cr_ready == 0"),
		For:   (*promv1.Duration)(ptr.To("10m")),
		Annotations: map[string]string{
			"summary": "CDI is not available to use",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "critical",
			operatorHealthImpactLabelKey: "critical",
		},
	},
	{
		Alert: "CDIDataVolumeUnusualRestartCount",
		Expr:  intstr.FromString("kubevirt_cdi_import_pods_high_restart > 0 or kubevirt_cdi_upload_pods_high_restart > 0 or kubevirt_cdi_clone_pods_high_restart > 0"),
		For:   (*promv1.Duration)(ptr.To("5m")),
		Annotations: map[string]string{
			"summary": "Some CDI population workloads have an unusual restart count, meaning they are probably failing and need to be investigated",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "warning",
			operatorHealthImpactLabelKey: "warning",
		},
	},
	{
		Alert: "CDIStorageProfilesIncomplete",
		Expr:  intstr.FromString(`sum by(storageclass,provisioner) ((kubevirt_cdi_storageprofile_info{complete="false"}>0))`),
		For:   (*promv1.Duration)(ptr.To("5m")),
		Annotations: map[string]string{
			"summary": "Incomplete StorageProfile {{ $labels.storageclass }}, accessMode/volumeMode cannot be inferred by CDI for PVC population request",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "info",
			operatorHealthImpactLabelKey: "none",
		},
	},
	{
		Alert: "CDIDataImportCronOutdated",
		Expr:  intstr.FromString(`sum by(namespace,cron_name) (kubevirt_cdi_dataimportcron_outdated{pending="false"}) > 0`),
		For:   (*promv1.Duration)(ptr.To("15m")),
		Annotations: map[string]string{
			"summary": "DataImportCron (recurring polling of VM templates disk image sources, also known as golden images) PVCs are not being updated on the defined schedule",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "warning",
			operatorHealthImpactLabelKey: "none",
		},
	},
	{
		Alert: "CDINoDefaultStorageClass",
		Expr: intstr.FromString(`sum(kubevirt_cdi_storageprofile_info{default="true"} or on() vector(0)) +
								 sum(kubevirt_cdi_storageprofile_info{virtdefault="true"} or on() vector(0)) +
								 (count(kubevirt_cdi_datavolume_pending == 0) or on() vector(0)) == 0`),
		For: (*promv1.Duration)(ptr.To("5m")),
		Annotations: map[string]string{
			"summary": "No default StorageClass or virtualization StorageClass, and a DataVolume is pending for one",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "warning",
			operatorHealthImpactLabelKey: "none",
		},
	},
	{
		Alert: "CDIMultipleDefaultVirtStorageClasses",
		Expr:  intstr.FromString(`sum(kubevirt_cdi_storageprofile_info{virtdefault="true"} or on() vector(0)) > 1`),
		For:   (*promv1.Duration)(ptr.To("5m")),
		Annotations: map[string]string{
			"summary": "More than one default virtualization StorageClass detected",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "warning",
			operatorHealthImpactLabelKey: "none",
		},
	},
	{
		Alert: "CDIDefaultStorageClassDegraded",
		Expr: intstr.FromString(`sum(kubevirt_cdi_storageprofile_info{default="true",degraded="false"} or on() vector(0)) +
								 sum(kubevirt_cdi_storageprofile_info{virtdefault="true",degraded="false"} or on() vector(0)) +
								 on () (0*(sum(kubevirt_cdi_storageprofile_info{default="true"}) or sum(kubevirt_cdi_storageprofile_info{virtdefault="true"}))) == 0`),
		For: (*promv1.Duration)(ptr.To("5m")),
		Annotations: map[string]string{
			"summary": "Default storage class has no smart clone or ReadWriteMany",
		},
		Labels: map[string]string{
			severityAlertLabelKey:        "warning",
			operatorHealthImpactLabelKey: "none",
		},
	},
}
