package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/informers/internalinterfaces"
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
	clientset             kubernetes.Interface
	queue                 workqueue.RateLimitingInterface
	sharedInformerFactory internalinterfaces.SharedInformerFactory
	pvcInformer           cache.SharedIndexInformer
	importerImage         string
}

func NewController(client kubernetes.Interface, importerImage string) *Controller {

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Setup all informers to only watch objects with the LabelSelector `app=containerized-data-importer`
	informerFactory := informers.NewFilteredSharedInformerFactory(client, 10*time.Minute, "", func(options *metav1.ListOptions) {
		options.LabelSelector = common.CDI_SELECTOR_LABEL
	})
	pvcInformerFactory := informerFactory.Core().V1().PersistentVolumeClaims()

	c := &Controller{
		clientset:     client,
		queue:         queue,
		pvcInformer:   pvcInformerFactory.Informer(),
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

	return c
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer c.queue.ShutDown()
	glog.Infoln("Starting CDI controller loop")
	if threadiness < 1 {
		return fmt.Errorf("controller.Run: expected >0 threads, got %d", threadiness)
	}
	go c.sharedInformerFactory.Start(stopCh)
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
	glog.Infof("processNextItem: next pvc to process: %s\n", key)
	if err != nil {
		return c.forgetKey(fmt.Sprintf("processNextItem: error converting key to pvc: %v", err), key)
	}
	if !metav1.HasAnnotation(pvc.ObjectMeta, AnnEndpoint) {
		return c.forgetKey(fmt.Sprintf("processNextItem: annotation %q not found, skipping pvc\n", AnnEndpoint), key)
	}
	if metav1.HasAnnotation(pvc.ObjectMeta, AnnImportPod) {
		// The pvc may have reached our queue due to a normal update process
		// however, based on the annotation of an importer pod, we know this was already processed.
		return c.forgetKey(fmt.Sprintf("processNextItem: annotation %q was found but annotation %q exists indicating this was already processed once, skipping pvc\n", AnnEndpoint, AnnImportPod), key)
	}
	if err := c.processItem(pvc); err != nil {
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
	pod, err := c.createImporterPod(ep, secretName, pvc)
	if err != nil {
		return e(err, "create pod")
	}
	err = c.setAnnoImportPod(pvc, pod.Name)
	if err != nil {
		return e(err, "set annotation")
	}
	return nil
}
