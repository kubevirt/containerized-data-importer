package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

type engineConfig struct {
	URL      string
	username string
	password string
	cacert   string
	insecure bool
}

type TransferProgress struct {
	Transferred uint64  `json:"transferred"`
	Description string  `json:"description"`
	Size        *uint64 `json:"size,omitempty"`
	Elapsed     float64 `json:"elapsed"`
}

func main() {
	var engineUrl, secretName, diskID, volPath string
	// Populate args
	flag.StringVar(&engineUrl, "engine-url", "", "ovirt-engine url (https//engine.fqdn)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing oVirt credentials")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&volPath, "volume-path", "", "Volume path to populate")

	// Other args
	flag.Parse()

	populate(engineUrl, diskID, volPath)
}

func populate(engineURL, diskID, volPath string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)
	progressGague := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "volume_populators",
			Name:      "ovirt_volume_populator",
			Help:      "Amount of data transferred",
		},
		[]string{"disk_id"},
	)
	if err := prometheus.Register(progressGague); err != nil {
		klog.Error("Prometheus progress gauge not registered:", err)
	} else {
		klog.Info("Prometheus progress gauge registered.")
	}

	engineConfig := loadEngineConfig(engineURL)

	// Write credentials to files
	ovirtPass, err := os.Create("/tmp/ovirt.pass")
	if err != nil {
		klog.Fatalf("Failed to create ovirt.pass %v", err)
	}

	defer ovirtPass.Close()
	_, err = ovirtPass.Write([]byte(engineConfig.password))
	if err != nil {
		klog.Fatalf("Failed to write password to file: %v", err)
	}

	var args []string
	//for secure connection use the ca cert
	if !engineConfig.insecure {
		cert, err := os.Create("/tmp/ca.pem")
		if err != nil {
			klog.Fatalf("Failed to create ca.pem %v", err)
		}

		defer cert.Close()
		_, err = cert.Write([]byte(engineConfig.cacert))
		if err != nil {
			klog.Fatalf("Failed to write CA to file: %v", err)
		}

		args = []string{
			"download-disk",
			"--output", "json",
			"--engine-url=" + engineConfig.URL,
			"--username=" + engineConfig.username,
			"--password-file=/tmp/ovirt.pass",
			"--cafile=" + "/tmp/ca.pem",
			"-f", "raw",
			diskID,
			volPath,
		}
	} else {
		args = []string{
			"download-disk",
			"--output", "json",
			"--engine-url=" + engineConfig.URL,
			"--username=" + engineConfig.username,
			"--password-file=/tmp/ovirt.pass",
			"--insecure",
			"-f", "raw",
			diskID,
			volPath,
		}
	}
	cmd := exec.Command("/usr/bin/ovirt-img", args...)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)
	klog.Info(fmt.Sprintf("Running command: %s", cmd.String()))

	go func() {
		for scanner.Scan() {
			progressOutput := TransferProgress{}
			text := scanner.Text()
			klog.Info(text)
			err = json.Unmarshal([]byte(text), &progressOutput)
			if err != nil {
				var syntaxError *json.SyntaxError
				if !errors.As(err, &syntaxError) {
					klog.Error(err)
				}
			}

			progressGague.WithLabelValues(diskID).Set(float64(progressOutput.Transferred))
		}

		done <- struct{}{}
	}()

	err = cmd.Start()
	if err != nil {
		klog.Fatal(err)
	}

	<-done
	err = cmd.Wait()
	if err != nil {
		klog.Fatal(err)
	}
}

func loadEngineConfig(engineURL string) engineConfig {
	user, err := os.ReadFile("/etc/secret-volume/user")
	if err != nil {
		klog.Fatal(err.Error())
	}
	pass, err := os.ReadFile("/etc/secret-volume/password")
	if err != nil {
		klog.Fatal(err.Error())
	}

	var insecureSkipVerify []byte
	_, err = os.Stat("/etc/secret-volume/insecureSkipVerify")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			insecureSkipVerify = []byte("false")
		} else {
			klog.Fatal(err.Error())
		}
	} else {
		insecureSkipVerify, err = os.ReadFile("/etc/secret-volume/insecureSkipVerify")
		if err != nil {
			klog.Fatal(err.Error())
		}
	}

	insecure, err := strconv.ParseBool(string(insecureSkipVerify))
	if err != nil {
		klog.Fatal(err.Error())
	}
	// If the insecure option is set, the ca file field in the secret is not required.
	var cacert []byte
	if insecure {
		cacert = []byte("")
	} else {
		cacert, err = os.ReadFile("/etc/secret-volume/cacert")
		if err != nil {
			klog.Error(err.Error())
		}
	}

	return engineConfig{
		URL:      engineURL,
		username: string(user),
		password: string(pass),
		cacert:   string(cacert),
		insecure: insecure,
	}
}
