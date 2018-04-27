package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// pvc annotations
	AnnEndpoint  = "kubevirt.io/storage.import.endpoint"
	AnnSecret    = "kubevirt.io/storage.import.secretName"
	AnnImportPod = "kubevirt.io/storage.import.importPodName"
	// importer pod annotations
	AnnCreatedBy = "kubevirt.io/storage.createdByController"
)

type Controller struct {
	clientset     kubernetes.Interface
	queue         workqueue.RateLimitingInterface
	pvcInformer   cache.SharedIndexInformer
	importerImage string
}

func NewController(client kubernetes.Interface, pvcInformer cache.SharedIndexInformer, importerImage string) (*Controller, error) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Controller{
		clientset:     client,
		queue:         queue,
		pvcInformer:   pvcInformer,
		importerImage: importerImage,
	}

	// Bind the Index/Informer to the queue only for new pvcs
	c.pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
		// this is triggered by an update or it will also be
		// be triggered periodically even if no changes were made.
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
	})

	return c, nil
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
	for c.ProcessNextItem() {
	}
}

// Select pvcs with AnnEndpoint
// Note: only new and updated pvcs will trigger an add to the work queue, Deleted pvcs
//  are ignored.
func (c *Controller) ProcessNextItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	pvc, err := c.pvcFromKey(key)
	if pvc == nil {
		return c.forgetKey("", key)
	}
	if err != nil {
		return c.forgetKey(fmt.Sprintf("processNextItem: error converting key to pvc: %v", err), key)
	}

	// check to see if we have our endpoint and we are not already processing this pvc
	if !c.checkIfShouldQueuePVC(pvc, "processNextItem") {
		return c.forgetKey(fmt.Sprintf("processNextItem: annotation %q not found or pvc %s is already being worked, skipping pvc\n", AnnEndpoint, pvc.Name), key)
	}
	glog.Infof("processNextItem: next pvc to process: %s\n", key)

	// all checks have passed, let's process it!
	if err := c.processItem(pvc); err == nil {
		// If the proceess succeeds, we're done operating on this key; remove it from the queue
		return c.forgetKey(fmt.Sprintf("processNextItem: error processing key %q: %v", key, err), key)
	}
	return true
}

func (c *Controller) forgetKey(msg string, key interface{}) bool {
	if len(msg) > 0 {
		glog.Info(msg)
	}
	c.queue.Forget(key)
	return true
}

// Create the importer pod with the pvc and optional secret.
func (c *Controller) processItem(pvc *v1.PersistentVolumeClaim) error {
	e := func(err error, s string) error {
		if s == "" {
			return fmt.Errorf("processItem: %v\n", err)
		}
		return fmt.Errorf("processItem: %s: %v\n", s, err)
	}

	ep, err := getEndpoint(pvc)
	if err != nil {
		return e(err, "")
	}
	secretName, err := c.getSecretName(pvc)
	if err != nil {
		return e(err, "")
	}
	if secretName == "" {
		glog.Infof("processItem: no secret will be supplied to endpoint %q\n", ep)
	}

	// check our existing pvc one more time to ensure we should be working on it
	// and to help mitigate any unforeseen race conditions.
	if !c.checkIfShouldQueuePVC(pvc, "processItem") {
		return e(nil, "pvc is already being processed")
	}

	// all checks passed, let's create the importer pod!
	pod, err := c.createImporterPod(ep, secretName, pvc)
	if err != nil {
		return e(err, "create pod")
	}
	err = c.setAnnoImportPod(pvc, pod.Name)
	if err != nil {
		return e(err, "set annotation")
	}
	// Add the label if it doesn't exist
	// it should be noted that the label may actually exist but not
	// recognized due to patched timing issues but since this is a
	// simple map there is no harm in adding it again if we don't find it.
	if !c.checkIfLabelExists(pvc, common.CDI_LABEL_KEY, common.CDI_LABEL_VALUE) {
		glog.Infof("adding label \"%s\" to pvc, it does not exist", common.CDI_LABEL_SELECTOR)
		err = c.setCdiLabel(pvc)
		if err != nil {
			glog.Infof("error adding label %v", err)
			return e(err, "set label")
		}
	}
	return nil
}
