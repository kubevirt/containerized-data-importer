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
	annEndpoint  = "kubevirt.io/storage.import.endpoint"
	annSecret    = "kubevirt.io/storage.import.secretName"
	annImportPod = "kubevirt.io/storage.import.importPodName"
	// importer pod annotations
	annCreatedBy = "kubevirt.io/storage.createdByController"
)

type Controller struct {
	clientset        kubernetes.Interface
	queue            workqueue.RateLimitingInterface
	pvcInformer      cache.SharedIndexInformer
	pvcListWatcher   cache.ListerWatcher
	importerImageTag string
}

func NewController(
	client kubernetes.Interface,
	queue workqueue.RateLimitingInterface,
	pvcInformer cache.SharedIndexInformer,
	pvcListWatcher cache.ListerWatcher,
	importerTag string,
) *Controller {
	return &Controller{
		clientset:        client,
		queue:            queue,
		pvcInformer:      pvcInformer,
		pvcListWatcher:   pvcListWatcher,
		importerImageTag: importerTag,
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

// Select pvcs with annEndpoint
// Note: only new pvcs trigger an addition to the work queue. Updated and deleted pvcs
//  are ignored.
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
	if err := c.processItem(pvc); err != nil {
		glog.Errorf("processNextItem: error processing key %q: %v", key, err)
		c.queue.Forget(key)
	}
	return true
}

// Create the importer pod with the pvc and optional secret.
func (c *Controller) processItem(pvc *v1.PersistentVolumeClaim) error {
	ep, err := getEndpoint(pvc)
	if err != nil {
		return fmt.Errorf("processItem: get endpoint: %v\n", err)
	}
	secretName, err := c.getSecretName(pvc)
	if err != nil {
		return fmt.Errorf("processItem: get secert: %v\n", err)
	}
	if secretName == "" {
		glog.Infof("processItem: no secret will be supplied to endpoint %q\n", ep)
	}
	pod, err := c.createImporterPod(ep, secretName, pvc)
	if err != nil {
		return fmt.Errorf("processItem: create pod: %v\n", err)
	}
	err = c.setAnnoImportPod(pvc, pod.Name)
	if err != nil {
		return fmt.Errorf("processItem: set anno: %v\n", err)
	}
	return nil
}
