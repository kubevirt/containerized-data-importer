package main

import (
	"flag"
	"os"
	"os/signal"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/controller"
	"github.com/kubevirt/containerized-data-importer/pkg/util"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	configPath string
	masterURL  string
)

func init() {
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Absolute path to kubeconfig. (Default: $KUBECONFIG)")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.Parse()
	glog.Infoln("CDI Controller is initialized.")
}

func main() {
	var client kubernetes.Interface
	// Build client relevant to runtime environment
	if os.Getenv("KUBERNETES_PORT") != "" {
		glog.Infoln("Detected in-cluster environment")
		client = util.GetInClusterClient()
	} else {
		glog.Infoln("Detected out-of-cluster environment")
		client = util.GetOutOfClusterClient(configPath, masterURL)
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
	err := cdiController.Run(1, stopCh)
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
