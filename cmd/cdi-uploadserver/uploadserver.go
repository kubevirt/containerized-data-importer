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

	"k8s.io/klog"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/uploadserver"
	"kubevirt.io/containerized-data-importer/pkg/util"
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

	destination := getDestination()

	server := uploadserver.NewUploadServer(
		listenAddress,
		listenPort,
		destination,
		os.Getenv("TLS_KEY"),
		os.Getenv("TLS_CERT"),
		os.Getenv("CLIENT_CERT"),
		os.Getenv("CLIENT_NAME"),
		os.Getenv(common.UploadImageSize),
	)

	klog.Infof("Upload destination: %s", destination)

	klog.Infof("Running server on %s:%d", listenAddress, listenPort)

	err := server.Run()
	if err != nil {
		klog.Errorf("UploadServer failed: %s", err)
		os.Exit(1)
	}

	// Check if cloning or uploading based on the existance of the scratch space. Clone won't have scratch space
	clone := false
	_, err = os.OpenFile(common.ScratchDataDir, os.O_RDONLY, 0600)
	if err != nil {
		// Cloning instead of uploading.
		clone = true
	}
	if clone {
		err = util.WriteTerminationMessage("Clone Complete")
	} else {
		err = util.WriteTerminationMessage("Upload Complete")
	}
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

func getDestination() string {
	destination := defaultDestination

	if val := os.Getenv("DESTINATION"); len(val) > 0 {
		destination = val
	}

	return destination
}
