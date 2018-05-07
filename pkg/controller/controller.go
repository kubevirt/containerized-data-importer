package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	. "github.com/kubevirt/containerized-data-importer/pkg/util"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	AnnPodPhase  = "kubevirt.io/storage.import.pod.phase"
)

type Controller struct {
	clientset                kubernetes.Interface
	pvcQueue, podQueue       workqueue.RateLimitingInterface
	pvcInformer, podInformer cache.SharedIndexInformer
	importerImage            string
	pullPolicy               string // Options: IfNotPresent, Always, or Never
}

func NewController(client kubernetes.Interface, pvcInformer, podInformer cache.SharedIndexInformer, importerImage string, pullPolicy string) (*Controller, error) {
	c := &Controller{
		clientset:     client,
		pvcQueue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		podQueue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		pvcInformer:   pvcInformer,
		podInformer:   podInformer,
		importerImage: importerImage,
		pullPolicy:    pullPolicy,
	}

	// Bind the pvc SharedIndexInformer to the pvc queue
	c.pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.pvcQueue.AddRateLimited(key)
			}
		},
		// this is triggered by an update or it will also be
		// be triggered periodically even if no changes were made.
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.pvcQueue.AddRateLimited(key)
			}
		},
	})

	// Bind the pod SharedIndexInformer to the pod queue
	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.podQueue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.podQueue.AddRateLimited(key)
			}
		},
	})

	return c, nil
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer func() {
		c.pvcQueue.ShutDown()
		c.podQueue.ShutDown()
	}()
	glog.Infoln("Starting CDI controller loop")
	if threadiness < 1 {
		return Err("expected >0 threads, got %d", threadiness)
	}
	go c.pvcInformer.Run(stopCh)
	go c.podInformer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced) {
		return Err("Timeout waiting for pvc cache sync")
	}
	if !cache.WaitForCacheSync(stopCh, c.podInformer.HasSynced) {
		return Err("Timeout waiting for pod cache sync")
	}
	glog.Infoln("Controller cache has synced")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runPVCWorkers, time.Second, stopCh)
		go wait.Until(c.runPodWorkers, time.Second, stopCh)
	}
	<-stopCh
	return nil
}

func (c *Controller) runPodWorkers() {
	for c.ProcessNextPodItem() {
	}
}

func (c *Controller) runPVCWorkers() {
	for c.ProcessNextPvcItem() {
	}
}

// ProcessNextPodItem gets the next pod key from the queue and verifies that it was created
// by the CDI controller. If not, the key is discarded. Otherwise, the pod object is passed
// to processPodItem.
func (c *Controller) ProcessNextPodItem() bool {
	key, shutdown := c.podQueue.Get()
	if shutdown {
		return false
	}
	defer c.podQueue.Done(key)
	pod, err := c.podFromKey(key)
	if err != nil {
		c.forgetKey(key, fmt.Sprintf("Unable to get pod object: %v", err))
		return true
	}
	if !metav1.HasAnnotation(pod.ObjectMeta, AnnCreatedBy) {
		c.forgetKey(key, "Pod does not have annotation "+AnnCreatedBy)
		return true
	}
	if err := c.processPodItem(pod); err == nil {
		c.forgetKey(key, fmt.Sprintf("Processing Pod %q completed", pod.Name))
	}
	return true
}

// processPodItem verifies that the passed in pod is genuine and, if so, annotates the Phase
// of the pod in the PVC to indicate the status of the import process.
func (c *Controller) processPodItem(pod *v1.Pod) error {
	glog.Infof("processPodItem: processing pod named %q\n", pod.Name)

	// First get the pod's CDI-relative pvc name
	var pvcKey string
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == DataVolName {
			glog.Infof("processPodItem: Pod has volume matching CDI claim")
			pvcKey = fmt.Sprintf("%s/%s", pod.Namespace, vol.PersistentVolumeClaim.ClaimName)
			break
		}
	}
	if len(pvcKey) == 0 {
		// For some reason, no pvc matching the volume name was found.
		return fmt.Errorf("processPodItem: Pod does not contain volume %q", DataVolName)
	}
	glog.Infof("processPodItem: Getting PVC object for key %q", pvcKey)
	pvc, err := c.pvcFromKey(pvcKey)
	if err != nil {
		return fmt.Errorf("processPodItem: error getting pvc from key: %v", err)
	}
	err = c.setPVCAnnotation(pvc, AnnPodPhase, string(pod.Status.Phase))
	if err != nil {
		return fmt.Errorf("processPodItem: error setting PVC annotation: %v", err)
	}
	glog.Infof("processPodItem: Pod phase %q annotated in PVC %q", pod.Status.Phase, pvcKey)
	return nil
}

// Select only pvcs with the importer endpoint annotation and that are not being processed.
// We forget the key unless `processPvcItem` returns an error in which case the key can be
// retried. 
func (c *Controller) ProcessNextPvcItem() bool {
	key, shutdown := c.pvcQueue.Get()
	if shutdown {
		return false
	}
	defer c.pvcQueue.Done(key)

	pvc, err := c.pvcFromKey(key)
	if err != nil || pvc == nil {
		return c.forgetKey(key, fmt.Sprintf("ProcessNextPvcItem: error converting key %q to pvc: %v", key, err))
	}

	// filter pvc and decide if the importer pod should be created
	createPod, _ := c.checkPVC(pvc, false)
	if !createPod {
		return c.forgetKey(key, fmt.Sprintf("ProcessNextPvcItem: skipping pvc %q\n", key))
	}

	glog.Infof("ProcessNextPvcItem: next pvc to process: %s\n", key)
	err = c.processPvcItem(pvc)
	if err != nil {
		return true // and remember key
	}
	// we're done operating on this key; remove it from the queue
	return c.forgetKey(key, "")
}

// Create the importer pod based the pvc. The endpoint and optional secret are available to
// the importer pod as env vars. The pvc is checked (again) to ensure that we are not already
// processing this pvc, which would result in multiple importer pods for the same pvc.
func (c *Controller) processPvcItem(pvc *v1.PersistentVolumeClaim) error {
	e := func(err error, s string) error {
		if s == "" {
			return fmt.Errorf("processPvcItem: %v\n", err)
		}
		return fmt.Errorf("processPvcItem: %s: %v\n", s, err)
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
		glog.Infof("processPvcItem: no secret will be supplied to endpoint %q\n", ep)
	}

	// check our existing pvc one more time to ensure we should be working on it
	// and to help mitigate any race conditions. This time we get the latest pvc.
	createPod, err := c.checkPVC(pvc, true)
	if err != nil { // maybe an intermittent api error
		return e(err, "check pvc")
	}
	if !createPod { // don't create importer pod but not an error
		return nil // forget key; logging already done
	}

	// all checks passed, let's create the importer pod!
	pod, err := c.createImporterPod(ep, secretName, pvc)
	if err != nil {
		return e(err, "create pod")
	}
	err = c.setPVCAnnotation(pvc, AnnImportPod, pod.Name)
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

// forget the passed-in key for this event and optionally log a message.
func (c *Controller) forgetKey(key interface{}, msg string) bool {
	if len(msg) > 0 {
		glog.Info(msg)
	}
	c.pvcQueue.Forget(key)
	return true
}
