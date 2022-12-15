/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package populator_machinery

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
)

const (
	subSystem   = "volume_populator"
	labelResult = "result"
)

type metricsManager struct {
	mu               sync.Mutex
	srv              *http.Server
	cache            map[types.UID]time.Time
	registry         k8smetrics.KubeRegistry
	opLatencyMetrics *k8smetrics.HistogramVec
	opInFlight       *k8smetrics.Gauge
}

var metricBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600}
var inFlightCheckInterval = 30 * time.Second

func initMetrics() *metricsManager {

	m := new(metricsManager)
	m.cache = make(map[types.UID]time.Time)
	m.registry = k8smetrics.NewKubeRegistry()

	m.opLatencyMetrics = k8smetrics.NewHistogramVec(
		&k8smetrics.HistogramOpts{
			Subsystem: subSystem,
			Name:      "operation_seconds",
			Help:      "Time taken by each populator operation",
			Buckets:   metricBuckets,
		},
		[]string{labelResult},
	)
	m.opInFlight = k8smetrics.NewGauge(
		&k8smetrics.GaugeOpts{
			Subsystem: subSystem,
			Name:      "operations_in_flight",
			Help:      "Total number of operations in flight",
		},
	)

	k8smetrics.RegisterProcessStartTime(m.registry.Register)
	m.registry.MustRegister(m.opLatencyMetrics)
	m.registry.MustRegister(m.opInFlight)

	go m.scheduleOpsInFlightMetric()

	return m
}

func (m *metricsManager) scheduleOpsInFlightMetric() {
	for range time.Tick(inFlightCheckInterval) {
		func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.opInFlight.Set(float64(len(m.cache)))
		}()
	}
}

type promklog struct{}

func (pl promklog) Println(v ...interface{}) {
	klog.Error(v...)
}

func (m *metricsManager) startListener(httpEndpoint, metricsPath string) {
	if "" == httpEndpoint || "" == metricsPath {
		return
	}

	mux := http.NewServeMux()
	mux.Handle(metricsPath, k8smetrics.HandlerFor(
		m.registry,
		k8smetrics.HandlerOpts{
			ErrorLog:      promklog{},
			ErrorHandling: k8smetrics.ContinueOnError,
		}))

	klog.Infof("Metrics path successfully registered at %s", metricsPath)

	l, err := net.Listen("tcp", httpEndpoint)
	if err != nil {
		klog.Fatalf("failed to listen on address[%s], error[%v]", httpEndpoint, err)
	}
	m.srv = &http.Server{Addr: l.Addr().String(), Handler: mux}
	go func() {
		if err := m.srv.Serve(l); err != http.ErrServerClosed {
			klog.Fatalf("failed to start endpoint at:%s/%s, error: %v", httpEndpoint, metricsPath, err)
		}
	}()
	klog.Infof("Metrics http server successfully started on %s, %s", httpEndpoint, metricsPath)
}

func (m *metricsManager) stopListener() {
	if m.srv == nil {
		return
	}

	err := m.srv.Shutdown(context.Background())
	if err != nil {
		klog.Errorf("Failed to shutdown metrics server: %s", err.Error())
	}

	klog.Infof("Metrics server successfully shutdown")
}

// operationStart starts a new operation
func (m *metricsManager) operationStart(pvcUID types.UID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cache[pvcUID]; !exists {
		m.cache[pvcUID] = time.Now()
	}
	m.opInFlight.Set(float64(len(m.cache)))
}

// dropOperation drops an operation
func (m *metricsManager) dropOperation(pvcUID types.UID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, pvcUID)
	m.opInFlight.Set(float64(len(m.cache)))
}

// recordMetrics emits operation metrics
func (m *metricsManager) recordMetrics(pvcUID types.UID, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	startTime, exists := m.cache[pvcUID]
	if !exists {
		// the operation has not been cached, return directly
		return
	}

	operationDuration := time.Since(startTime).Seconds()
	m.opLatencyMetrics.WithLabelValues(result).Observe(operationDuration)

	delete(m.cache, pvcUID)
	m.opInFlight.Set(float64(len(m.cache)))
}
