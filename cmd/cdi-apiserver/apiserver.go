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
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	snapclient "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/apiserver"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	certwatcher "kubevirt.io/containerized-data-importer/pkg/util/cert/watcher"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
	"kubevirt.io/containerized-data-importer/pkg/version/verflag"
)

const (
	// Default port that api listens on.
	defaultPort = 8443

	// Default address api listens on.
	defaultHost = "0.0.0.0"
)

// semiCachingClient is a controller-runtime client that uses a cache for
// PersistentVolume, StorageClass and StorageProfile reads and direct API calls for everything else.
type semiCachingClient struct {
	crclient.Client
	cache cache.Cache
}

func (c *semiCachingClient) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object, opts ...crclient.GetOption) error {
	switch obj.(type) {
	case *corev1.PersistentVolume, *storagev1.StorageClass, *cdiv1.StorageProfile:
		return c.cache.Get(ctx, key, obj, opts...)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *semiCachingClient) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	switch list.(type) {
	case *corev1.PersistentVolumeList, *storagev1.StorageClassList, *cdiv1.StorageProfileList:
		return c.cache.List(ctx, list, opts...)
	}
	return c.Client.List(ctx, list, opts...)
}

// pvTransform replaces each PersistentVolume in the informer cache with a
// stripped copy that only contains the fields used by renderPvcSpecFromAvailablePv
// and CheckVolumeSatisfyClaim, reducing the memory footprint.
func pvTransform(obj interface{}) (interface{}, error) {
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return obj, nil
	}
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              pv.Name,
			ResourceVersion:   pv.ResourceVersion,
			DeletionTimestamp: pv.DeletionTimestamp,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: pv.Spec.StorageClassName,
			Capacity:         pv.Spec.Capacity,
			AccessModes:      pv.Spec.AccessModes,
			VolumeMode:       pv.Spec.VolumeMode,
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: pv.Status.Phase,
		},
	}, nil
}

var (
	configPath      string
	kubeURL         string
	verbose         string
	apiServerArgs   APIServerEnvs
	installerLabels map[string]string
)

// APIServerEnvs contains environment variables read for setting custom cert paths
type APIServerEnvs struct {
	CertFile string `default:"/var/run/certs/cdi-apiserver-server-cert/tls.crt" split_words:"true"`
	KeyFile  string `default:"/var/run/certs/cdi-apiserver-server-cert/tls.key" split_words:"true"`
}

func init() {
	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&kubeURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
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

	installerLabels = map[string]string{}
	if partOfVal := os.Getenv(common.InstallerPartOfLabel); len(partOfVal) != 0 {
		installerLabels[common.AppKubernetesPartOfLabel] = partOfVal
	}
	if versionVal := os.Getenv(common.InstallerVersionLabel); len(versionVal) != 0 {
		installerLabels[common.AppKubernetesVersionLabel] = versionVal
	}
}

func main() {
	defer klog.Flush()

	verflag.PrintAndExitIfRequested()

	err := envconfig.Process("", &apiServerArgs)
	if err != nil {
		klog.Fatalf("Unable to get environment variables: %v\n", errors.WithStack(err))
	}

	utilruntime.Must(corev1.AddToScheme(scheme.Scheme))
	utilruntime.Must(cdiv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme.Scheme))

	cfg, err := clientcmd.BuildConfigFromFlags(kubeURL, configPath)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	aggregatorClient := aggregatorclient.NewForConfigOrDie(cfg)

	cdiClient := cdiclient.NewForConfigOrDie(cfg)

	snapClient := snapclient.NewForConfigOrDie(cfg)

	uncachedClient, err := crclient.New(cfg, crclient.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		klog.Fatalf("Unable to create controller runtime client: %v\n", errors.WithStack(err))
	}

	semiCache, err := cache.New(cfg, cache.Options{
		Scheme: scheme.Scheme,
		ByObject: map[crclient.Object]cache.ByObject{
			&corev1.PersistentVolume{}: {
				Transform: pvTransform,
			},
			&storagev1.StorageClass{}: {},
			&cdiv1.StorageProfile{}:   {},
		},
	})
	if err != nil {
		klog.Fatalf("Unable to create informer cache: %v\n", errors.WithStack(err))
	}

	if err := dvc.CreateAvailablePersistentVolumeIndex(semiCache); err != nil {
		klog.Fatalf("Unable to create field index: %v\n", errors.WithStack(err))
	}

	controllerRuntimeClient := &semiCachingClient{
		Client: uncachedClient,
		cache:  semiCache,
	}

	ctx := signals.SetupSignalHandler()

	authConfigWatcher, err := apiserver.NewAuthConfigWatcher(ctx, client)
	if err != nil {
		klog.Fatalf("Unable to create authConfigWatcher: %v\n", errors.WithStack(err))
	}

	cdiConfigTLSWatcher, err := cryptowatch.NewCdiConfigTLSWatcher(ctx, cdiClient)
	if err != nil {
		klog.Fatalf("Unable to create cdiConfigTLSWatcher: %v\n", errors.WithStack(err))
	}

	authorizor, err := apiserver.NewAuthorizorFromConfig(cfg, authConfigWatcher)
	if err != nil {
		klog.Fatalf("Unable to create authorizor: %v\n", errors.WithStack(err))
	}

	certWatcher, err := certwatcher.New(apiServerArgs.CertFile, apiServerArgs.KeyFile)
	if err != nil {
		klog.Fatalf("Unable to create certwatcher: %v\n", errors.WithStack(err))
	}

	cdiAPIApp, err := apiserver.NewCdiAPIServer(defaultHost,
		defaultPort,
		client,
		aggregatorClient,
		cdiClient,
		snapClient,
		controllerRuntimeClient,
		authorizor,
		authConfigWatcher,
		cdiConfigTLSWatcher,
		certWatcher,
		installerLabels)
	if err != nil {
		klog.Fatalf("CDI API server failed to initialize: %v\n", errors.WithStack(err))
	}

	go func() {
		if err := semiCache.Start(ctx); err != nil {
			klog.Fatalf("cache failed: %v\n", errors.WithStack(err))
		}
	}()

	if !semiCache.WaitForCacheSync(ctx) {
		klog.Fatalf("Failed to sync cache\n")
	}

	go func() {
		if err := certWatcher.Start(ctx.Done()); err != nil {
			klog.Errorf("cert watcher failed: %v\n", errors.WithStack(err))
		}
	}()

	err = cdiAPIApp.Start(ctx.Done())
	if err != nil {
		klog.Fatalf("TLS server failed: %v\n", errors.WithStack(err))
	}
}
