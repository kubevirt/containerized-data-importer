/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2018 Red Hat, Inc.
 *
 */

package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	ocpcrypto "github.com/openshift/library-go/pkg/crypto"

	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/uploadserver"
	"kubevirt.io/containerized-data-importer/pkg/util"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
)

const (
	defaultListenPort    = 8443
	defaultListenAddress = "0.0.0.0"

	defaultDestination = common.ImporterWritePath
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
}

func main() {
	defer klog.Flush()

	listenAddress, listenPort := getListenAddressAndPort()

	cryptoConfig := getCryptoConfig()
	destination := getDestination()
	deadline := getDeadline()

	filesystemOverhead, _ := strconv.ParseFloat(os.Getenv(common.FilesystemOverheadVar), 64)
	preallocation, _ := strconv.ParseBool(os.Getenv(common.Preallocation))

	config := &uploadserver.Config{
		BindAddress:        listenAddress,
		BindPort:           listenPort,
		Destination:        destination,
		ServerKeyFile:      os.Getenv("TLS_KEY_FILE"),
		ServerCertFile:     os.Getenv("TLS_CERT_FILE"),
		ClientCertFile:     os.Getenv("CLIENT_CERT_FILE"),
		ClientName:         os.Getenv("CLIENT_NAME"),
		ImageSize:          os.Getenv(common.UploadImageSize),
		FilesystemOverhead: filesystemOverhead,
		Preallocation:      preallocation,
		CryptoConfig:       cryptoConfig,
		Deadline:           deadline,
	}

	server := uploadserver.NewUploadServer(config)

	klog.Infof("Running server on %s:%d", listenAddress, listenPort)

	result, err := server.Run()
	if err != nil {
		klog.Errorf("UploadServer failed: %s", err)
		os.Exit(1)
	}

	termMsg := &common.TerminationMessage{}

	if !result.DeadlinePassed {
		if result.CloneTarget {
			termMsg.Message = ptr.To("Clone Complete")
		} else {
			termMsg.Message = ptr.To("Upload Complete")
		}
		termMsg.PreallocationApplied = ptr.To(result.PreallocationApplied)
	} else {
		termMsg.Message = ptr.To("Deadline Passed")
		termMsg.DeadlinePassed = ptr.To(true)
	}

	message, err := termMsg.String()
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}

	err = util.WriteTerminationMessage(message)
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}

	klog.Info("UploadServer successfully exited")
}

func getListenAddressAndPort() (string, int) {
	addr, port := defaultListenAddress, defaultListenPort

	// empty value okay here
	if val, exists := os.LookupEnv("LISTEN_ADDRESS"); exists {
		addr = val
	}

	// not okay here
	if val := os.Getenv("LISTEN_PORT"); len(val) > 0 {
		n, err := strconv.ParseUint(val, 10, 16)
		if err == nil {
			port = int(n)
		}
	}

	return addr, port
}

func getCryptoConfig() cryptowatch.CryptoConfig {
	ciphersNames := strings.Split(os.Getenv(common.CiphersTLSVar), ",")
	ciphers := cryptowatch.CipherSuitesIDs(ciphersNames)
	minTLSVersion, _ := ocpcrypto.TLSVersion(os.Getenv(common.MinVersionTLSVar))

	return cryptowatch.CryptoConfig{
		CipherSuites: ciphers,
		MinVersion:   minTLSVersion,
	}
}

func getDestination() string {
	destination := defaultDestination

	if val := os.Getenv("DESTINATION"); len(val) > 0 {
		destination = val
	}

	return destination
}

func getDeadline() *time.Time {
	dl := os.Getenv("DEADLINE")
	if dl != "" {
		result, err := time.Parse(time.RFC3339, dl)
		if err != nil {
			klog.Fatalf("Failed to parse deadline: %v", err)
		}
		return &result
	}
	return nil
}
