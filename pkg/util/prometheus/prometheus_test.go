package prometheus

import (
	"bytes"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

var (
	progress *prometheus.CounterVec
	ownerUID string
)

func init() {
	progress = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_progress",
			Help: "The test progress in percentage",
		},
		[]string{"ownerUID"},
	)
	ownerUID = "1111-1111-111"
}

var _ = Describe("Timed update", func() {

	It("Should start and stop when finished", func() {
		r := ioutil.NopCloser(bytes.NewReader([]byte("hello world")))
		progressReader := NewProgressReader(r, uint64(11), progress, ownerUID)
		progressReader.StartTimedUpdate()
		_, err := ioutil.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("Update Progress", func() {
	BeforeEach(func() {
		progress = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_progress",
				Help: "The test progress in percentage",
			},
			[]string{"ownerUID"},
		)
	})

	It("Parse valid progress update", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Add(0)
		metric := &dto.Metric{}
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(45),
			},
			total:    uint64(100),
			progress: progress,
			ownerUID: ownerUID,
		}
		result := promReader.updateProgress()
		Expect(true).To(Equal(result))
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(45)))
	})

	It("0 total should return 0", func() {
		metric := &dto.Metric{}
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(45),
			},
			total:    uint64(0),
			progress: progress,
			ownerUID: ownerUID,
		}
		result := promReader.updateProgress()
		Expect(false).To(Equal(result))
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
	})

	It("current and total equals should return false", func() {
		metric := &dto.Metric{}
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(1000),
				Done:    true,
			},
			total:    uint64(1000),
			progress: progress,
			ownerUID: ownerUID,
		}
		result := promReader.updateProgress()
		Expect(false).To(Equal(result))
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(100)))
	})

})
