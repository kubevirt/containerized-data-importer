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
 * Copyright 2019 Red Hat, Inc.
 *
 */

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"kubevirt.io/containerized-data-importer/pkg/apiserver"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	certwatcher "kubevirt.io/containerized-data-importer/pkg/util/cert/watcher"
	"kubevirt.io/containerized-data-importer/pkg/version/verflag"
)

const (
	// Default port that api listens on.
	defaultPort = 8443

	// Default address api listens on.
	defaultHost = "0.0.0.0"

	certDir  = "/var/run/certs/cdi-apiserver-server-cert/"
	certFile = certDir + "tls.crt"
	keyFile  = certDir + "tls.key"
)

var (
	configPath string
	masterURL  string
	verbose    string
)

func init() {
	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	klog.InitFlags(nil)
	flag.Parse()

	// get the verbose level so it can be passed to the importer pod
	defVerbose := fmt.Sprintf("%d", 1) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		klog.V(1).Infof("Note: increase the -v level in the api deployment for more detailed logging, eg. -v=%d or -v=%d\n", 2, 3)
	}
}

func main() {
	defer klog.Flush()

	verflag.PrintAndExitIfRequested()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, configPath)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	aggregatorClient := aggregatorclient.NewForConfigOrDie(cfg)

	cdiClient := cdiclient.NewForConfigOrDie(cfg)

	ch := signals.SetupSignalHandler()

	authConfigWatcher := apiserver.NewAuthConfigWatcher(client, ch)

	authorizor, err := apiserver.NewAuthorizorFromConfig(cfg, authConfigWatcher)
	if err != nil {
		klog.Fatalf("Unable to create authorizor: %v\n", errors.WithStack(err))
	}

	certWatcher, err := certwatcher.New(certFile, keyFile)
	if err != nil {
		klog.Fatalf("Unable to create certwatcher: %v\n", errors.WithStack(err))
	}

	uploadApp, err := apiserver.NewCdiAPIServer(defaultHost,
		defaultPort,
		client,
		aggregatorClient,
		cdiClient,
		authorizor,
		authConfigWatcher,
		certWatcher)
	if err != nil {
		klog.Fatalf("Upload api failed to initialize: %v\n", errors.WithStack(err))
	}

	go certWatcher.Start(ch)

	err = uploadApp.Start(ch)
	if err != nil {
		klog.Fatalf("TLS server failed: %v\n", errors.WithStack(err))
	}
}
