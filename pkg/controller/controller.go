package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	annEndpoint = "kubevirt.io/storage.import.endpoint"
	annSecret   = "kubevirt.io/storage.import.secretName"
	annStatus   = "kubevirt.io/storage.import.status"
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
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)
	pvc, err := c.pvcFromKey(key)
	if pvc == nil {
		c.queue.Forget(key)
		return true
	}
	glog.Infoln("processNextItem(): Next item to process: ", pvc.Name)
	if err != nil {
		glog.Errorf("processNextItem(): error converting key to pvc: %v", err)
		c.queue.Forget(key)
		return true
	}
	if !metav1.HasAnnotation(pvc.ObjectMeta, annEndpoint) {
		glog.Infoln("processNextItem(): annotation not found, skipping item")
		c.queue.Forget(key)
		return true
	}
	if err := c.processItem(pvc); err != nil {
		glog.Errorf("processNextItem(): error processing key: %v", err)
		c.queue.Forget(key)
	}
	return true
}

// Create the importer pod (and its secert if needed).
// Place a watch on the importer pod and annotate the pvc when the importer pod terminates.
func (c *Controller) processItem(pvc *v1.PersistentVolumeClaim) error {
	ep := getEndpoint(pvc)
	if ep == "" {
		return fmt.Errorf("import endpoint missing for pvc %q", pvc.Name)
	}
	epSecret := getEndpointSecret(c.clientset, pvc)
	if epSecret == nil {
		glog.Infof("processItem: no secret for endpoint %q\n", ep)
	}
	importPod, err := createImporterPod(ep, epSecret, pvc)
	if err != nil {
		return fmt.Errorf("processItem: error creating importer pod: %v\n", err)
	}
	//just reference pod to prevent compile error
	glog.Infof("importer pod %q created", importPod.Name)
	return nil
}
