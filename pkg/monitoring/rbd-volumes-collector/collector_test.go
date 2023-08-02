package pvkrbdrxbounce

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	io_prometheus_client "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	framework "k8s.io/client-go/tools/cache/testing"
)

var _ = Describe("PersistentVolumeInfoCollector", func() {
	var collector *PersistentVolumeInfoCollector

	BeforeEach(func() {
		if collector != nil {
			prometheus.Unregister(collector)
		}
	})

	var setupCollector = func(driver, mounter, mapOptions string) {
		volumeMode := v1.PersistentVolumeBlock
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test-namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "test-pv",
				VolumeMode: &volumeMode,
			},
		}

		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pv",
			},
			Spec: v1.PersistentVolumeSpec{
				PersistentVolumeSource: v1.PersistentVolumeSource{
					CSI: &v1.CSIPersistentVolumeSource{
						Driver: driver,
						VolumeAttributes: map[string]string{
							"clusterID":     "test-cluster",
							"mounter":       mounter,
							"imageFeatures": "layering,deep-flatten,exclusive-lock",
							"mapOptions":    mapOptions,
						},
					},
				},
			},
		}

		pvcInformer := newFakeInformerFor(&v1.PersistentVolumeClaim{})
		pvInformer := newFakeInformerFor(&v1.PersistentVolume{})

		err := pvcInformer.GetStore().Add(pvc)
		Expect(err).ToNot(HaveOccurred())
		err = pvInformer.GetStore().Add(pv)
		Expect(err).ToNot(HaveOccurred())

		collector = SetupCollector(pvcInformer, pvInformer)
	}

	var getKubevirtCdiRbdVolumeMetric = func(metricsChan chan prometheus.Metric) *io_prometheus_client.Metric {
		m := <-metricsChan
		dto := &io_prometheus_client.Metric{}
		err := m.Write(dto)
		Expect(err).ToNot(HaveOccurred())

		Expect(m.Desc().String()).To(ContainSubstring("kubevirt_cdi_rbd_volume"))
		Expect(dto.Label).To(HaveLen(5))

		return dto
	}

	Describe("Collector", func() {
		It("should collect volume with rbd driver and default mounter", func() {
			metricsChan := make(chan prometheus.Metric)
			setupCollector("openshift-storage.rbd.csi.ceph.com", "", "krbd:rxbounce")
			go collector.Collect(metricsChan)

			dto := getKubevirtCdiRbdVolumeMetric(metricsChan)

			err := whereKeyExpectValue(dto.Label, "volume_mode", "Block")
			Expect(err).ToNot(HaveOccurred())
			err = whereKeyExpectValue(dto.Label, "rxbounce_enabled", "true")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should collect volume with rbd driver and rbd mounter", func() {
			metricsChan := make(chan prometheus.Metric)
			setupCollector("openshift-storage.rbd.csi.ceph.com", "rbd", "krbd:rxbounce")
			go collector.Collect(metricsChan)

			dto := getKubevirtCdiRbdVolumeMetric(metricsChan)

			err := whereKeyExpectValue(dto.Label, "volume_mode", "Block")
			Expect(err).ToNot(HaveOccurred())
			err = whereKeyExpectValue(dto.Label, "rxbounce_enabled", "true")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not collect volume with non-rbd driver", func() {
			metricsChan := make(chan prometheus.Metric)
			setupCollector("random", "rbd", "krbd:rxbounce")
			go collector.Collect(metricsChan)
			Consistently(metricsChan).ShouldNot(Receive())
		})

		It("should not collect volume with non-rbd mounter", func() {
			metricsChan := make(chan prometheus.Metric)
			setupCollector("openshift-storage.rbd.csi.ceph.com", "random", "krbd:rxbounce")
			go collector.Collect(metricsChan)
			Consistently(metricsChan).ShouldNot(Receive())
		})

		It("should set rxbounce_enabled to false when mapOptions does not contain krbd:rxbounce", func() {
			metricsChan := make(chan prometheus.Metric)
			setupCollector("openshift-storage.rbd.csi.ceph.com", "rbd", "random")
			go collector.Collect(metricsChan)

			dto := getKubevirtCdiRbdVolumeMetric(metricsChan)

			err := whereKeyExpectValue(dto.Label, "volume_mode", "Block")
			Expect(err).ToNot(HaveOccurred())
			err = whereKeyExpectValue(dto.Label, "rxbounce_enabled", "false")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func newFakeInformerFor(obj runtime.Object) cache.SharedIndexInformer {
	objSource := framework.NewFakeControllerSource()
	objInformer := cache.NewSharedIndexInformer(objSource, obj, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return objInformer
}

func whereKeyExpectValue(labels []*io_prometheus_client.LabelPair, key string, value string) error {
	for _, label := range labels {
		if *label.Name == key {
			if *label.Value == value {
				return nil
			}
			return fmt.Errorf("expected value %s for key %s, got %s", value, key, *label.Value)
		}
	}

	return fmt.Errorf("expected key %s not found", key)
}
