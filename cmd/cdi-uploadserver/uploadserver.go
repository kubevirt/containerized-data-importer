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

	ocpcrypto "github.com/openshift/library-go/pkg/crypto"

	"k8s.io/klog/v2"

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

	filesystemOverhead, _ := strconv.ParseFloat(os.Getenv(common.FilesystemOverheadVar), 64)
	preallocation, _ := strconv.ParseBool(os.Getenv(common.Preallocation))

	config := &uploadserver.Config{
		BindAddress:        listenAddress,
		BindPort:           listenPort,
		Destination:        destination,
		ServerKey:          os.Getenv("TLS_KEY"),
		ServerCert:         os.Getenv("TLS_CERT"),
		ClientCert:         os.Getenv("CLIENT_CERT"),
		ClientName:         os.Getenv("CLIENT_NAME"),
		ImageSize:          os.Getenv(common.UploadImageSize),
		FilesystemOverhead: filesystemOverhead,
		Preallocation:      preallocation,
		CryptoConfig:       cryptoConfig,
	}

	server := uploadserver.NewUploadServer(config)

	klog.Infof("Running server on %s:%d", listenAddress, listenPort)

	err := server.Run()
	if err != nil {
		klog.Errorf("UploadServer failed: %s", err)
		os.Exit(1)
	}

	// Check if cloning or uploading based on the existence of the scratch space. Clone won't have scratch space
	clone := false
	_, err = os.OpenFile(common.ScratchDataDir, os.O_RDONLY, 0600)
	if err != nil {
		// Cloning instead of uploading.
		clone = true
	}
	var message string
	if clone {
		message = "Clone Complete"
	} else {
		message = "Upload Complete"
	}
	if server.PreallocationApplied() {
		message += ", " + common.PreallocationApplied
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
