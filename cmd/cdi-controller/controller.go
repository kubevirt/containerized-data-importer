package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	crdv1alpha1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	"github.com/pkg/errors"
	v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	crdinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	informers "kubevirt.io/containerized-data-importer/pkg/client/informers/externalversions"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	csiclientset "kubevirt.io/containerized-data-importer/pkg/snapshot-client/clientset/versioned"
	csiinformers "kubevirt.io/containerized-data-importer/pkg/snapshot-client/informers/externalversions"
)

const (
	readyFile = "/tmp/ready"
)

var (
	kubeconfig             string
	masterURL              string
	importerImage          string
	clonerImage            string
	uploadServerImage      string
	uploadProxyServiceName string
	configName             string
	pullPolicy             string
	verbose                string
	log                    = logf.Log.WithName("controller")
)

// The importer and cloner images are obtained here along with the supported flags. IMPORTER_IMAGE, CLONER_IMAGE, and UPLOADSERVICE_IMAGE
// are required by the controller and will cause it to fail if not defined.
// Note: kubeconfig hierarchy is 1) -kubeconfig flag, 2) $KUBECONFIG exported var. If neither is
//   specified we do an in-cluster config. For testing it's easiest to export KUBECONFIG.
func init() {
	// flags
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	klog.InitFlags(nil)
	flag.Parse()

	if flag.Lookup("kubeconfig") != nil {
		kubeconfig = flag.Lookup("kubeconfig").Value.String()
	}
	importerImage = getRequiredEnvVar("IMPORTER_IMAGE")
	clonerImage = getRequiredEnvVar("CLONER_IMAGE")
	uploadServerImage = getRequiredEnvVar("UPLOADSERVER_IMAGE")
	uploadProxyServiceName = getRequiredEnvVar("UPLOADPROXY_SERVICE")

	pullPolicy = common.DefaultPullPolicy
	if pp := os.Getenv(common.PullPolicy); len(pp) != 0 {
		pullPolicy = pp
	}
	configName = common.ConfigName

	// NOTE we used to have a constant here and we're now just passing in the level directly
	// that should be fine since it was a constant and not a mutable variable
	defVerbose := fmt.Sprintf("%d", 1) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		klog.V(1).Infof("Note: increase the -v level in the controller deployment for more detailed logging, eg. -v=%d or -v=%d\n", 2, 3)
	}

	klog.V(3).Infof("init: complete: cdi controller will create importer using image %q\n", importerImage)
}

func getRequiredEnvVar(name string) string {
	val := os.Getenv(name)
	if val == "" {
		klog.Fatalf("Environment Variable %q undefined\n", name)
	}
	return val
}

func start(cfg *rest.Config, stopCh <-chan struct{}) {
	klog.Info("Starting CDI controller components")

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	cdiClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	csiClient, err := csiclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building csi clientset: %s", err.Error())
	}

	extClient, err := extclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building extClient: %s", err.Error())
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		klog.Errorf("Unable to setup controller manager: %v", err)
		os.Exit(1)
	}

	cdiInformerFactory := informers.NewSharedInformerFactory(cdiClient, common.DefaultResyncPeriod)
	csiInformerFactory := csiinformers.NewFilteredSharedInformerFactory(csiClient, common.DefaultResyncPeriod, "", func(options *v1.ListOptions) {
		options.LabelSelector = common.CDILabelSelector
	})
	pvcInformerFactory := k8sinformers.NewSharedInformerFactory(client, common.DefaultResyncPeriod)
	podInformerFactory := k8sinformers.NewFilteredSharedInformerFactory(client, common.DefaultResyncPeriod, "", func(options *v1.ListOptions) {
		options.LabelSelector = common.CDILabelSelector
	})
	serviceInformerFactory := k8sinformers.NewFilteredSharedInformerFactory(client, common.DefaultResyncPeriod, "", func(options *v1.ListOptions) {
		options.LabelSelector = common.CDILabelSelector
	})
	crdInformerFactory := crdinformers.NewSharedInformerFactory(extClient, common.DefaultResyncPeriod)

	pvcInformer := pvcInformerFactory.Core().V1().PersistentVolumeClaims()
	podInformer := podInformerFactory.Core().V1().Pods()
	serviceInformer := serviceInformerFactory.Core().V1().Services()
	dataVolumeInformer := cdiInformerFactory.Cdi().V1alpha1().DataVolumes()
	snapshotInformer := csiInformerFactory.Snapshot().V1alpha1().VolumeSnapshots()
	crdInformer := crdInformerFactory.Apiextensions().V1beta1().CustomResourceDefinitions().Informer()

	dataVolumeController := controller.NewDataVolumeController(
		client,
		cdiClient,
		csiClient,
		extClient,
		pvcInformer,
		dataVolumeInformer)

	if _, err := controller.NewImportController(mgr, cdiClient, client, log, importerImage, pullPolicy, verbose); err != nil {
		klog.Errorf("Unable to setup import controller: %v", err)
		os.Exit(1)
	}

	cloneController := controller.NewCloneController(client,
		cdiClient,
		pvcInformer,
		podInformer,
		clonerImage,
		pullPolicy,
		verbose,
		getAPIServerPublicKey())

	smartCloneController := controller.NewSmartCloneController(client,
		cdiClient,
		csiClient,
		pvcInformer,
		snapshotInformer,
		dataVolumeInformer)

	uploadController := controller.NewUploadController(
		client,
		cdiClient,
		pvcInformer,
		podInformer,
		serviceInformer,
		uploadServerImage,
		uploadProxyServiceName,
		pullPolicy,
		verbose)

	if _, err := controller.NewConfigController(mgr, cdiClient, client, log, uploadProxyServiceName, configName); err != nil {
		klog.Errorf("Unable to setup config controller: %v", err)
		os.Exit(1)
	}

	klog.V(1).Infoln("created cdi controllers")

	err = uploadController.Init()
	if err != nil {
		klog.Fatalf("Error initializing upload controller: %+v", err)
	}

	go cdiInformerFactory.Start(stopCh)
	go pvcInformerFactory.Start(stopCh)
	go podInformerFactory.Start(stopCh)
	go serviceInformerFactory.Start(stopCh)
	go crdInformerFactory.Start(stopCh)

	addCrdInformerEventHandlers(crdInformer, extClient, csiInformerFactory, smartCloneController, stopCh)

	klog.V(1).Infoln("started informers")

	go func() {
		err = dataVolumeController.Run(3, stopCh)
		if err != nil {
			klog.Fatalf("Error running dataVolume controller: %+v", err)
		}
	}()

	go func() {
		err = cloneController.Run(1, stopCh)
		if err != nil {
			klog.Fatalf("Error running clone controller: %+v", err)
		}
	}()

	go func() {
		err = uploadController.Run(1, stopCh)
		if err != nil {
			klog.Fatalf("Error running upload controller: %+v", err)
		}
	}()

	startSmartController(extClient, csiInformerFactory, smartCloneController, stopCh)

	if err := mgr.Start(stopCh); err != nil {
		klog.Errorf("Error running manager: %v", err)
		os.Exit(1)
	}
}

func main() {
	defer klog.Flush()
	debug := false
	if i, err := strconv.Atoi(verbose); err == nil && i > 1 {
		debug = true
	}
	logf.SetLogger(logf.ZapLogger(debug))
	logf.Log.WithName("main").Info("Verbosity level", "verbose", verbose)

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}

	stopCh := signals.SetupSignalHandler()

	err = startLeaderElection(context.TODO(), cfg, func() {
		start(cfg, stopCh)
	})

	if err != nil {
		klog.Fatalf("Unable to start leader election: %v\n", errors.WithStack(err))
	}

	if err = createReadyFile(); err != nil {
		klog.Fatalf("Error creating ready file: %+v", err)
	}

	<-stopCh

	deleteReadyFile()

	klog.V(2).Infoln("cdi controller exited")
}

func createReadyFile() error {
	f, err := os.Create(readyFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func deleteReadyFile() {
	os.Remove(readyFile)
}

func addCrdInformerEventHandlers(crdInformer cache.SharedIndexInformer, extClient extclientset.Interface,
	csiInformerFactory csiinformers.SharedInformerFactory, smartCloneController *controller.SmartCloneController,
	stopCh <-chan struct{}) {
	crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crd := obj.(*v1beta1.CustomResourceDefinition)
			crdName := crd.Name

			vsClass := crdv1alpha1.VolumeSnapshotClassResourcePlural + "." + crdv1alpha1.GroupName
			vsContent := crdv1alpha1.VolumeSnapshotContentResourcePlural + "." + crdv1alpha1.GroupName
			vs := crdv1alpha1.VolumeSnapshotResourcePlural + "." + crdv1alpha1.GroupName

			switch crdName {
			case vsClass:
				fallthrough
			case vsContent:
				fallthrough
			case vs:
				startSmartController(extClient, csiInformerFactory, smartCloneController, stopCh)
			}
		},
	})
}

func startSmartController(extclient extclientset.Interface, csiInformerFactory csiinformers.SharedInformerFactory,
	smartCloneController *controller.SmartCloneController, stopCh <-chan struct{}) {
	if controller.IsCsiCrdsDeployed(extclient) {
		go csiInformerFactory.Start(stopCh)
		go func() {
			err := smartCloneController.Run(1, stopCh)
			if err != nil {
				klog.Fatalf("Error running smart clone controller: %+v", err)
			}
		}()
	}
}

func getAPIServerPublicKey() *rsa.PublicKey {
	keyBytes, err := ioutil.ReadFile(controller.APIServerPublicKeyPath)
	if err != nil {
		klog.Fatalf("Error reading apiserver public key")
	}

	key, err := controller.DecodePublicKey(keyBytes)
	if err != nil {
		klog.Fatalf("Error decoding public key")
	}

	return key
}
