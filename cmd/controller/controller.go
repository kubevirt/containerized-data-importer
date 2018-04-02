package main

import (
	"flag"
	"os"
	"os/signal"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/controller"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

var (
	configPath  string
	masterURL   string
	importerTag string
)

// The optional importer image tag are obtained here along with the supported flags.
// Note: kubeconfig hierarchy is 1) -kubeconfig flag, 2) $KUBECONFIG exported var. If neither is
//   specified we do an in-cluster config. For testing it's easiest to export KUBECONFIG.
func init() {
	const (
		// optional, importer image tag, default is "latest"
		IMPORTER_TAG = "IMPORTER_TAG"
	)
	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.Parse()
	// env variables
	importerTag = os.Getenv(IMPORTER_TAG)
	if importerTag == "" {
		importerTag = "latest"
	}
	glog.Infof("init: complete: CDI controller will create the %q version of the importer\n", importerTag)
}

func main() {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, configPath)
	if err != nil {
		glog.Fatalf("Error getting kube config: %v\n", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error getting kube client: %v\n", err)
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	pvcInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	// Bind the Index/Informer to the queue only for new pvcs
	pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
	})

	// watch pvcs in all namespaces
	pvcListWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "persistentvolumeclaims", "", fields.Everything())
	cdiController := controller.NewController(client, queue, pvcInformer, pvcListWatcher, importerTag)
	glog.Infoln("main: created CDI Controller")
	stopCh := handleSignals()
	err = cdiController.Run(1, stopCh)
	if err != nil {
		glog.Fatalln(err)
	}
}

// Shutdown gracefully on system signals
func handleSignals() <-chan struct{} {
	sigCh := make(chan os.Signal)
	stopCh := make(chan struct{})
	go func() {
		signal.Notify(sigCh)
		<-sigCh
		close(stopCh)
		os.Exit(1)
	}()
	return stopCh
}
