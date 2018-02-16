package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	clientset      kubernetes.Interface
	queue          workqueue.RateLimitingInterface
	pvcInformer    cache.SharedIndexInformer
	pvcListWatcher cache.ListerWatcher
}

func NewController(
	client kubernetes.Interface,
	queue workqueue.RateLimitingInterface,
	pvcInformer cache.SharedIndexInformer,
	pvcListWatcher cache.ListerWatcher,
) *Controller {
	return &Controller{
		clientset:      client,
		queue:          queue,
		pvcInformer:    pvcInformer,
		pvcListWatcher: pvcListWatcher,
	}
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer c.queue.ShutDown()
	glog.Infoln("Starting CDI controller loop")
	if threadiness < 1 {
		return fmt.Errorf("controller.Run(): expected >0 threads, got %d", threadiness)
	}
	go c.pvcInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced) {
		return fmt.Errorf("controller.Run(): Timeout waiting for cache sync")
	}
	glog.Infoln("Controller cache has synced")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorkers, time.Second, stopCh)
	}
	<-stopCh
	return nil
}

func (c *Controller) runWorkers() {
	glog.Infoln("DEBUG - controller.runWorkers()")
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	glog.Infoln("DEBUG -- controller.processNextItem()")
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	glog.Infof("Got Object: %v", key)
	defer c.queue.Done(key)
	if key, ok := key.(string); !ok {
		c.queue.Forget(key)
		glog.Errorf("controller.processNextItem(): key object failed string type assertion")
		return false
	}
	if err := c.processItem(key.(string)); err != nil {
		c.queue.Forget(key)
		glog.Errorf("controller.processNextItem(): error processing key: %v", err)
	}
	return true
}

func (c *Controller) processItem(key string) error {
	glog.Infof("DEBUG -- controller.processItem(): processing object %q", key)
	obj, ok, err := c.pvcInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("controller.processItem(): error getting object with key %s: %v", key, err)
	}
	if !ok {
		glog.Infof("Object with key %s does not exist\n", key)
	}
	c.queue.Forget(obj)
	return nil
}
