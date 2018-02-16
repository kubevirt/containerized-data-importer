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
	configPath string
	masterURL  string
)

// Note: kubeconfig hierarchy is 1) -kubeconfig flag, 2) $KUBECONFIG exported var. If neither is specified
//   we'll do an in-cluster config, so for testing it's easiest to export KUBECONFIG.
func init() {
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.Parse()
	glog.Infoln("CDI Controller is initialized.")
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
	// Bind the Index/Informer to the queue
	pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil && old != new {
				queue.AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
	})

	pvcListWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "persistentvolumeclaims", "", fields.Everything())
	cdiController := controller.NewController(client, queue, pvcInformer, pvcListWatcher)
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
