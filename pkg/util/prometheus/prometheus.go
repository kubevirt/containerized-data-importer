package prometheus

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

const CertFileName = "tls.crt"
const KeyFileName = "tls.key"

// ProgressReader is a counting reader that reports progress to prometheus.
type ProgressReader struct {
	util.CountingReader
	metric ProgressMetric
	total  uint64
	final  bool
}

type ProgressMetric interface {
	Add(value float64)
	Get() (float64, error)
	Delete()
}

// NewProgressReader creates a new instance of a prometheus updating progress reader.
func NewProgressReader(r io.ReadCloser, metric ProgressMetric, total uint64) *ProgressReader {
	promReader := &ProgressReader{
		CountingReader: util.CountingReader{
			Reader:  r,
			Current: 0,
		},
		metric: metric,
		total:  total,
		final:  true,
	}

	return promReader
}

// StartTimedUpdate starts the update timer to automatically update every second.
func (r *ProgressReader) StartTimedUpdate() {
	// Start the progress update thread.
	go r.timedUpdateProgress()
}

func (r *ProgressReader) timedUpdateProgress() {
	cont := true
	for cont {
		// Update every second.
		time.Sleep(time.Second)
		cont = r.updateProgress()
	}
}

func (r *ProgressReader) updateProgress() bool {
	if r.total > 0 {
		finished := r.final && r.Done
		currentProgress := 100.0
		if !finished && r.Current < r.total {
			currentProgress = float64(r.Current) / float64(r.total) * 100.0
		}
		progress, err := r.metric.Get()
		if err != nil {
			klog.Errorf("updateProgress: failed to read metric; %v", err)
			return true // true ==> to try again // todo - how to avoid endless loop in case it's a constant error?
		}
		if currentProgress > progress {
			r.metric.Add(currentProgress - progress)
		}
		klog.V(1).Infoln(fmt.Sprintf("%.2f", currentProgress))
		return !finished
	}
	return false
}

// SetNextReader replaces the current counting reader with a new one,
// for tracking progress over multiple readers.
func (r *ProgressReader) SetNextReader(reader io.ReadCloser, final bool) {
	r.CountingReader = util.CountingReader{
		Reader:  reader,
		Current: r.Current,
		Done:    false,
	}
	r.final = final
}

// StartPrometheusEndpoint starts the prometheus HTTPS endpoint, reading the
// serving certificate and key from certsDirectory, where they must be named
// CertFileName (tls.crt) and KeyFileName (tls.key). It returns an error if
// either file is missing. The server is started asynchronously, so errors from
// serving are only logged by the goroutine, not returned.
func StartPrometheusEndpoint(certsDirectory string) error {
	certFile := path.Join(certsDirectory, CertFileName)
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		klog.Errorf("Cert file %s does not exist", certFile)
		return fmt.Errorf("cert file %s does not exist", certFile)
	}
	keyFile := path.Join(certsDirectory, KeyFileName)
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		klog.Errorf("Key file %s does not exist", keyFile)
		return fmt.Errorf("key file %s does not exist", keyFile)
	}

	go func() {
		server := &http.Server{
			Addr:              ":8443",
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           promhttp.Handler(),
		}

		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
			klog.Errorf("Prometheus endpoint failed: %v", err)
		}
	}()

	return nil
}
