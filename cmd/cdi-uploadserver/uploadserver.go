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

	"github.com/golang/glog"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/uploadserver"
)

const (
	defaultListenPort    = uint16(8443)
	defaultListenAddress = "0.0.0.0"

	defaultPVCDir      = common.IMPORTER_WRITE_DIR
	defaultDestination = common.IMPORTER_WRITE_PATH
)

type tlsInfo struct {
	keyFile, certFile, caCertFile string
}

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()

	listenAddress, listenPort := getListenAddressAndPort()

	pvcDir, destination := getPVCDirAndDestination()

	tls := newTLSInfo()

	server := uploadserver.NewUploadServer(
		listenAddress,
		listenPort,
		pvcDir,
		destination,
		tls.keyFile,
		tls.certFile,
		tls.caCertFile,
	)

	glog.Infof("PVC dir: %s, destination: %s", pvcDir, destination)

	glog.Infof("Running server on %s:%d", listenAddress, listenPort)

	err := server.Run()
	if err != nil {
		glog.Error("UploadServer failed: %s", err)
		os.Exit(1)
	}

	glog.Info("UploadServer successfully exited")
}

func getListenAddressAndPort() (string, uint16) {
	addr, port := defaultListenAddress, defaultListenPort

	// empty value okay here
	if val, exists := os.LookupEnv("LISTEN_ADDRESS"); exists {
		addr = val
	}

	// not okay here
	if val := os.Getenv("LISTEN_PORT"); len(val) > 0 {
		n, err := strconv.ParseUint(val, 10, 16)
		if err == nil {
			port = uint16(n)
		}
	}

	return addr, port
}

func getPVCDirAndDestination() (string, string) {
	pvcDir, destination := defaultPVCDir, defaultDestination

	if val := os.Getenv("PVC_DIR"); len(val) > 0 {
		pvcDir = val
	}

	if val := os.Getenv("DESTINATION"); len(val) > 0 {
		destination = val
	}

	return pvcDir, destination
}

func newTLSInfo() *tlsInfo {
	return &tlsInfo{
		keyFile:    os.Getenv("TLS_KEY_FILE"),
		certFile:   os.Getenv("TLS_CERT_FILE"),
		caCertFile: os.Getenv("TLS_CA_FILE"),
	}
}
