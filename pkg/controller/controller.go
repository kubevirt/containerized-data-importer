package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	informerCoreV1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listercorev1 "k8s.io/client-go/listers/core/v1"
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
	Clientset     kubernetes.Interface
	Queue         workqueue.RateLimitingInterface
	PvcInformer   cache.SharedIndexInformer
	PodInformer   cache.SharedIndexInformer
	PvcLister     listercorev1.PersistentVolumeClaimLister
	PodLister     listercorev1.PodLister
	ImporterImage string
}

func NewController(
	client kubernetes.Interface,
	queue workqueue.RateLimitingInterface,
	pvcIndexInformer informerCoreV1.PersistentVolumeClaimInformer,
	podIndexInformer informerCoreV1.PodInformer,
	importerImage string,
) *Controller {

	c := &Controller{
		Clientset:     client,
		Queue:         queue,
		PvcInformer:   pvcIndexInformer.Informer(),
		PvcLister:     pvcIndexInformer.Lister(),
		PodInformer:   podIndexInformer.Informer(),
		PodLister:     podIndexInformer.Lister(),
		ImporterImage: importerImage,
	}

	c.PvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.AddRateLimited(key)
			}
		},
	})

	c.PodInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				queue.Add(key)
			}
		},
	})

	return c
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer c.Queue.ShutDown()
	glog.Infoln("Starting CDI controller loop")
	if threadiness < 1 {
		return fmt.Errorf("controller.Run: expected >0 threads, got %d", threadiness)
	}
	go c.PvcInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.PvcInformer.HasSynced) {
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
// Note: only new pvcs trigger an addition to the work queue. Updated and deleted pvcs
//  are ignored.
func (c *Controller) ProcessNextItem() bool {
	key, shutdown := c.Queue.Get()
	if shutdown {
		return false
	}
	defer c.Queue.Done(key)
	pvc, err := c.pvcFromKey(key)
	if pvc == nil {
		c.Queue.Forget(key)
		return true
	}
	glog.Infof("processNextItem: next pvc to process: %s\n", key)
	if err != nil {
		glog.Errorf("processNextItem: error converting key to pvc: %v", err)
		c.Queue.Forget(key)
		return true
	}
	if !metav1.HasAnnotation(pvc.ObjectMeta, AnnEndpoint) {
		glog.Infof("processNextItem: annotation %q not found, skipping pvc\n", AnnEndpoint)
		c.Queue.Forget(key)
		return true
	}
	if err := c.processItem(pvc); err != nil {
		glog.Errorf("processNextItem: error processing key %q: %v", key, err)
		c.Queue.Forget(key)
	}
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
