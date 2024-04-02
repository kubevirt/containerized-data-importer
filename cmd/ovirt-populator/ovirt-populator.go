package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

type engineConfig struct {
	URL      string
	username string
	password string
	cacert   string
	insecure bool
}

type transferProgress struct {
	Transferred uint64  `json:"transferred"`
	Description string  `json:"description"`
	Size        *uint64 `json:"size,omitempty"`
	Elapsed     float64 `json:"elapsed"`
}

func main() {
	var engineURL, diskID, volPath, secretName, crName, crNamespace, ownerUID string
	var pvcSize *int64

	flag.StringVar(&engineURL, "engine-url", "", "ovirt-engine url (https://engine.fqdn)")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&volPath, "volume-path", "", "Volume path to populate")
	flag.StringVar(&secretName, "secret-name", "", "Name of secret containing ovirt credentials")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")
	flag.StringVar(&ownerUID, "owner-uid", "", "Owner UID (usually PVC UID)")
	pvcSize = flag.Int64("pvc-size", 0, "Size of pvc (in bytes)")

	flag.Parse()

	if pvcSize == nil || *pvcSize <= 0 {
		klog.Fatal("pvc-size must be greater than 0")
	}

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatal(err)
	}

	prometheusutil.StartPrometheusEndpoint(certsDirectory)

	populate(engineURL, diskID, volPath, ownerUID, *pvcSize)
}

func populate(engineURL, diskID, volPath, ownerUID string, pvcSize int64) {
	config := loadEngineConfig(engineURL)
	prepareCredentials(config)
	executePopulationProcess(config, diskID, volPath, ownerUID, pvcSize)
}

func prepareCredentials(config *engineConfig) {
	writeFile("/tmp/ovirt.pass", config.password)
	if !config.insecure {
		writeFile("/tmp/ca.pem", config.cacert)
	}
}

func writeFile(filename, content string) {
	file, err := os.Create(filename)
	if err != nil {
		klog.Fatalf("Failed to create %s: %v", filename, err)
	}
	defer file.Close()

	if _, err := file.Write([]byte(content)); err != nil {
		klog.Fatalf("Failed to write to %s: %v", filename, err)
	}
}

func executePopulationProcess(config *engineConfig, diskID, volPath, ownerUID string, pvcSize int64) {
	args := createCommandArguments(config, diskID, volPath)
	cmd := exec.Command("ovirt-img", args...)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)
	klog.Info(fmt.Sprintf("Running command: %s", cmd.String()))

	go monitorProgress(scanner, ownerUID, pvcSize, done)

	if err := cmd.Start(); err != nil {
		klog.Fatal(err)
	}

	<-done
	if err := cmd.Wait(); err != nil {
		klog.Fatal(err)
	}
}

func monitorProgress(scanner *bufio.Scanner, ownerUID string, pvcSize int64, done chan struct{}) {
	progress := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ovirt_progress",
			Help: "Progress of volume population",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progress); err != nil {
		klog.Error("Prometheus progress gauge not registered:", err)
		return
	}

	var currentProgress float64
	total := pvcSize
	metric := &dto.Metric{}

	for scanner.Scan() {
		progressOutput := transferProgress{}
		text := scanner.Text()
		klog.Info(text)
		if err := json.Unmarshal([]byte(text), &progressOutput); err != nil {
			var syntaxError *json.SyntaxError
			if !errors.As(err, &syntaxError) {
				klog.Error(err)
			}
		}
		if total > 0 {
			currentProgress = (float64(progressOutput.Transferred) / float64(total)) * 100
			if err := progress.WithLabelValues(ownerUID).Write(metric); err != nil {
				klog.Error(err)
			} else if currentProgress > metric.Counter.GetValue() {
				progress.WithLabelValues(ownerUID).Add(currentProgress - metric.Counter.GetValue())
			}
		}
	}

	if err := progress.WithLabelValues(ownerUID).Write(metric); err != nil {
		klog.Error(err)
	}

	remaining := 100 - int64(metric.Counter.GetValue())
	if remaining > 0 {
		progress.WithLabelValues(ownerUID).Add(float64(remaining))
	}

	done <- struct{}{}
}

func createCommandArguments(config *engineConfig, diskID, volPath string) []string {
	var args []string
	args = append(args, "download-disk", "--output", "json", "--engine-url="+config.URL, "--username="+config.username, "--password-file=/tmp/ovirt.pass")

	if config.insecure {
		args = append(args, "--insecure")
	} else {
		args = append(args, "--cafile=/tmp/ca.pem")
	}

	args = append(args, "-f", "raw", diskID, volPath)
	return args
}

func loadEngineConfig(engineURL string) *engineConfig {
	insecure := getEnvAsBool("insecureSkipVerify", false)
	return &engineConfig{
		URL:      engineURL,
		username: os.Getenv("user"),
		password: os.Getenv("password"),
		cacert:   os.Getenv("cacert"),
		insecure: insecure,
	}
}

func getEnvAsBool(key string, defaultVal bool) bool {
	val, found := os.LookupEnv(key)
	if !found {
		return defaultVal
	}
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		klog.Fatal("Invalid boolean value for", key, ":", val)
	}
	return boolVal
}
