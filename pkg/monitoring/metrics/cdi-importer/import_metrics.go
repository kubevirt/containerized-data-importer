package cdiimporter

import (
	ioprometheusclient "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// ImportProgressMetricName is the name of the import progress metric
	ImportProgressMetricName = "kubevirt_cdi_import_progress_total"
	// ImportPhaseMetricName is the name of the import phase metric
	ImportPhaseMetricName = "kubevirt_cdi_import_phase_info"
)

var (
	importerMetrics = []operatormetrics.Metric{
		importProgress,
		importPhase,
	}

	importProgress = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: ImportProgressMetricName,
			Help: "The import progress in percentage",
		},
		[]string{"ownerUID"},
	)

	importPhase = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: ImportPhaseMetricName,
			Help: "The current processing phase of the import",
		},
		[]string{"ownerUID", "phase"},
	)
)

type ImportProgress struct {
	ownerUID string
}

func Progress(ownerUID string) *ImportProgress {
	return &ImportProgress{ownerUID}
}

// Add adds value to the importProgress metric
func (ip *ImportProgress) Add(value float64) {
	importProgress.WithLabelValues(ip.ownerUID).Add(value)
}

// Get returns the importProgress value
func (ip *ImportProgress) Get() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	if err := importProgress.WithLabelValues(ip.ownerUID).Write(dto); err != nil {
		return 0, err
	}
	return dto.Counter.GetValue(), nil
}

// Delete removes the importProgress metric with the passed label
func (ip *ImportProgress) Delete() {
	importProgress.DeleteLabelValues(ip.ownerUID)
}

type ImportPhase struct {
	ownerUID string
}

func Phase(ownerUID string) *ImportPhase {
	return &ImportPhase{ownerUID}
}

// Set sets the current phase metric
func (ip *ImportPhase) Set(phase string) {
	for _, p := range common.AllProcessingPhases {
		importPhase.WithLabelValues(ip.ownerUID, p).Set(0)
	}
	importPhase.WithLabelValues(ip.ownerUID, phase).Set(1)
}

// Get returns the current phase
func (ip *ImportPhase) Get() (string, error) {
	for _, p := range common.AllProcessingPhases {
		dto := &ioprometheusclient.Metric{}
		if err := importPhase.WithLabelValues(ip.ownerUID, string(p)).Write(dto); err != nil {
			return "", err
		}
		if dto.Gauge.GetValue() == 1 {
			return string(p), nil
		}
	}
	return "", nil
}

// Delete removes the importPhase metric with the passed label
func (ip *ImportPhase) Delete() {
	for _, p := range common.AllProcessingPhases {
		importPhase.DeleteLabelValues(ip.ownerUID, string(p))
	}
}
