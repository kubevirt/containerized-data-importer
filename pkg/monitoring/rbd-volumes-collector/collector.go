package pvkrbdrxbounce

import (
	"fmt"
	"kubevirt.io/containerized-data-importer/pkg/monitoring"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	log = logf.Log.WithName("rbd-volumes-collector")

	// RbdPVMetricOpts contains options for RBD PersistentVolume metric
	RbdPVMetricOpts = monitoring.MetricOpts{
		Name: "kubevirt_cdi_rbd_volume",
		Help: "RBD mounted volume",
		Type: "Gauge",
	}

	rbdPVMetricDesc = prometheus.NewDesc(
		RbdPVMetricOpts.Name,
		RbdPVMetricOpts.Help,
		[]string{"pvc_name", "namespace", "pv_name", "volume_mode", "rxbounce_enabled"},
		nil,
	)
)

// PersistentVolumeInfoCollector collects information about PersistentVolumes managed by CDI
type PersistentVolumeInfoCollector struct {
	pvcInformer cache.SharedIndexInformer
	pvInformer  cache.SharedIndexInformer
}

// SetupCollector registers PersistentVolumeInfoCollector with Prometheus
func SetupCollector(pvcInformer, pvInformer cache.SharedIndexInformer) *PersistentVolumeInfoCollector {
	c := PersistentVolumeInfoCollector{
		pvcInformer: pvcInformer,
		pvInformer:  pvInformer,
	}
	prometheus.MustRegister(c)
	return &c
}

// Collect implements prometheus.Collector interface
func (c PersistentVolumeInfoCollector) Collect(ch chan<- prometheus.Metric) {
	c.collectPersistentVolumeInfo(ch)
}

// Describe implements prometheus.Collector interface
func (c PersistentVolumeInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rbdPVMetricDesc
}

func (c PersistentVolumeInfoCollector) collectPersistentVolumeInfo(ch chan<- prometheus.Metric) {
	pvcs, err := c.getPVCsManagedByCDI()
	if err != nil {
		log.Error(err, "Failed to get PVCs managed by CDI")
		return
	}

	for _, pvc := range pvcs {
		c.createVolumeInfoMetric(ch, pvc)
	}
}

func (c PersistentVolumeInfoCollector) getPVCsManagedByCDI() ([]v1.PersistentVolumeClaim, error) {
	pvcs := make([]v1.PersistentVolumeClaim, 0)

	list := c.pvcInformer.GetStore().List()
	log.Info("Found PVCs", "count", len(list))
	for _, v := range list {
		pvc, ok := v.(*v1.PersistentVolumeClaim)
		if !ok {
			return nil, fmt.Errorf("failed to cast object to PersistentVolumeClaim")
		}

		log.Info("Found PVC", "name", pvc.Name, "namespace", pvc.Namespace)
		pvcs = append(pvcs, *pvc)
	}

	return pvcs, nil
}

func (c PersistentVolumeInfoCollector) createVolumeInfoMetric(ch chan<- prometheus.Metric, pvc v1.PersistentVolumeClaim) {
	v, exists, err := c.pvInformer.GetStore().GetByKey(pvc.Spec.VolumeName)
	if err != nil {
		log.Error(err, "Failed to get PV", "name", pvc.Spec.VolumeName)
		return
	}
	if !exists {
		log.Error(err, fmt.Sprintf("PV %s referenced by PVC %s does not exist", pvc.Spec.VolumeName, pvc.Name))
		return
	}
	pv, ok := v.(*v1.PersistentVolume)
	if !ok {
		log.Error(err, "Failed to cast object to PersistentVolume", "name", pvc.Spec.VolumeName)
		return
	}
	if pv.Spec.PersistentVolumeSource.CSI == nil {
		return
	}

	driver := pv.Spec.PersistentVolumeSource.CSI.Driver
	if !strings.Contains(driver, "rbd.csi.ceph.com") {
		return
	}

	mounter, rxbounceEnabled := getAttributes(pv)
	if mounter != "" && mounter != "rbd" {
		return
	}

	log.Info("Found RBD PV", "name", pv.Name, "namespace", pv.Namespace, "rxbounce_enabled", rxbounceEnabled)
	m, err := prometheus.NewConstMetric(
		rbdPVMetricDesc,
		prometheus.GaugeValue,
		1,
		pvc.Name, pvc.Namespace,
		pvc.Spec.VolumeName,
		string(*pvc.Spec.VolumeMode),
		fmt.Sprintf("%t", rxbounceEnabled),
	)
	if err != nil {
		log.Error(err, "Failed to create metric", "metric", rbdPVMetricDesc)
		return
	}
	ch <- m
}

func getAttributes(pv *v1.PersistentVolume) (string, bool) {
	mounter := ""
	rxbounceEnabled := false

	for k, v := range pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes {
		if k == "mounter" {
			mounter = v
		}

		if k == "mapOptions" && strings.Contains(v, "krbd:rxbounce") {
			rxbounceEnabled = true
		}
	}

	return mounter, rxbounceEnabled
}
