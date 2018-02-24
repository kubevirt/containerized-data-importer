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
	// pvc annotations
	annEndpoint = "kubevirt.io/storage.import.endpoint"
	annSecret   = "kubevirt.io/storage.import.secretName"
	annStatus   = "kubevirt.io/storage.import.status"
	// importer pod annotations
	annCreatedBy = "kubevirt.io/storage.createdByController"
	// pvc statuses
	pvcStatusInProcess = "In-process"
	pvcStatusSuccess   = "Success"
	pvcStatusFailed	   = "Failed"
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
		return fmt.Errorf("controller.Run: expected >0 threads, got %d", threadiness)
	}
	go c.pvcInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced) {
		return fmt.Errorf("controller.Run: Timeout waiting for cache sync")
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

// Select pvcs with annEndpoint and without annStatus.
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
	glog.Infof("processNextItem: next pvc to process: %s\n", key)
	if err != nil {
		glog.Errorf("processNextItem: error converting key to pvc: %v", err)
		c.queue.Forget(key)
		return true
	}
	if !metav1.HasAnnotation(pvc.ObjectMeta, annEndpoint) {
		glog.Infof("processNextItem: annotation %q not found, skipping pvc\n", annEndpoint)
		c.queue.Forget(key)
		return true
	}
	if metav1.HasAnnotation(pvc.ObjectMeta, annStatus) {
		glog.Infof("processNextItem: annotation %q present, skipping pvc\n", annStatus)
		c.queue.Forget(key)
		return true
	}
	if err := c.processItem(pvc); err != nil {
		glog.Errorf("processNextItem: error processing key %q: %v", key, err)
		c.queue.Forget(key)
	}
	return true
}

// Create the importer pod and its secert if needed.
// Place a watch on the importer pod and annotate the pvc when the importer pod terminates.
func (c *Controller) processItem(pvc *v1.PersistentVolumeClaim) error {
	ep, err := getEndpoint(pvc)
	if err != nil {
		return fmt.Errorf("processItem: %v\n", err)
	}
	epSecret, skip, err := c.getEndpointSecret(pvc)
	if err != nil {
		return fmt.Errorf("processItem: %v\n", err)
	}
	if skip {
		// skip this pvc and try again. Note: annStatus has not been set yet
		c.enqueue(pvc) // re-queue pvc
		return nil
	}
	if epSecret == nil {
		glog.Infof("processItem: no secret will be supplied to endpoint %q\n", ep)
	}
	locPVC, err := c.setPVCStatus(pvc, pvcStatusInProcess)
	if err != nil {
		return fmt.Errorf("processItem: %v\n", err)
	}
	importPod, err := c.createImporterPod(ep, epSecret, locPVC)
	if err != nil {
		return fmt.Errorf("processItem: error creating importer pod: %v\n", err)
	}
	//just reference pod to prevent compile error
	glog.Infof("importer pod %q will be created...", importPod.Name)
	return nil
}

// re-queue the pvc.
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err == nil {
		c.queue.AddRateLimited(key)
	}
}
