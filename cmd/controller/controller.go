package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
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
	kubeEnvVar string
)

func init() {
	home := os.Getenv("HOME")
	kubeEnvVar = os.Getenv("KUBERNETES_PORT") // KUBERNETES_PORT is always set in a pod environment
	flag.StringVar(&configPath, "kubeconfig", filepath.Join(home, ".kube", "config"), "(Optional) Absolute path to kubeconfig. (Default: $HOME/.kube/config)")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of api server (Default: localhost:443)")
	flag.Parse()
	glog.Infoln("CDI Controller is initialized.")
}

func main() {
	var client kubernetes.Interface
	if kubeEnvVar != "" {
		glog.Infoln("Detected in-cluster environment")
		client = util.GetInClusterClient()
	} else {
		glog.Infoln("Detected out-of-cluster environment")
		client = util.GetOutOfClusterClient(configPath, masterURL)
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	pvcInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
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
