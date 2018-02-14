package controller

import (
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/util/workqueue"
	"time"
)

type Controller struct {
	clientset      kubernetes.Interface
	queue          workqueue.RateLimitingInterface
	pvcInformer    cache.SharedIndexInformer
	pvcListWatcher cache.ListerWatcher
}

func NewController(client kubernetes.Interface) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	pvcInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	pvcListWatcher := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "persistentvolumeclaims", "", fields.Everything())
	return &Controller{
		clientset:      client,
		queue:          queue,
		pvcInformer:    pvcInformer,
		pvcListWatcher: pvcListWatcher,
	}
}

func (c *Controller) Start(configPath string) {}

func (c *Controller) Run(stopCh <-chan struct{}) {}

func (c *Controller) runWorker() {}

func (c *Controller) processNextItem() bool {}

func (c *Controller) processItem(key, kobj string) error {
	c.pvcInformer.GetIndexer().GetByKey(key)
}

func (c *Controller) HasSynced() bool {}
