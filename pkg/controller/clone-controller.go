package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	. "kubevirt.io/containerized-data-importer/pkg/common"
	expectations "kubevirt.io/containerized-data-importer/pkg/expectations"
)

const (
	// pvc annotations
	AnnCloneRequest = "k8s.io/CloneRequest"
	AnnCloneOf      = "k8s.io/CloneOf"
	// cloner pods annotations
	AnnCloningCreatedBy   = "cdi.kubevirt.io/storage.cloningCreatedByController"
	AnnClonePodPhase      = "cdi.kubevirt.io/storage.clone.pod.phase"
	CloneUniqueID         = "cdi.kubevirt.io/storage.clone.cloneUniqeId"
	AnnTargetPodNamespace = "cdi.kubevirt.io/storage.clone.targetPod.namespace"
)

type CloneController struct {
	clientset                kubernetes.Interface
	queue                    workqueue.RateLimitingInterface
	pvcInformer, podInformer cache.SharedIndexInformer
	pvcLister                corelisters.PersistentVolumeClaimLister
	podLister                corelisters.PodLister
	pvcsSynced               cache.InformerSynced
	podsSynced               cache.InformerSynced
	cloneImage               string
	pullPolicy               string // Options: IfNotPresent, Always, or Never
	verbose                  string // verbose levels: 1, 2, ...
	podExpectations          *expectations.UIDTrackingControllerExpectations
}

func NewCloneController(client kubernetes.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	cloneImage string,
	pullPolicy string,
	verbose string) *CloneController {
	c := &CloneController{
		clientset:       client,
		queue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		pvcInformer:     pvcInformer.Informer(),
		podInformer:     podInformer.Informer(),
		pvcLister:       pvcInformer.Lister(),
		podLister:       podInformer.Lister(),
		pvcsSynced:      pvcInformer.Informer().HasSynced,
		podsSynced:      podInformer.Informer().HasSynced,
		cloneImage:      cloneImage,
		pullPolicy:      pullPolicy,
		verbose:         verbose,
		podExpectations: expectations.NewUIDTrackingControllerExpectations(expectations.NewControllerExpectations()),
	}

	// Bind the pvc SharedIndexInformer to the pvc queue
	c.pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuePVC,
		UpdateFunc: func(old, new interface{}) {
			c.enqueuePVC(new)
		},
	})

	// Bind the pod SharedIndexInformer to the pod queue
	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handlePodAdd,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*v1.Pod)
			oldDepl := old.(*v1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known PVCs.
				// Two different versions of the same PVCs will always have different RVs.
				return
			}
			c.handlePodUpdate(new)
		},
		DeleteFunc: c.handlePodDelete,
	})

	return c
}

func (c *CloneController) handlePodAdd(obj interface{}) {
	c.handlePodObject(obj, "add")
}
func (c *CloneController) handlePodUpdate(obj interface{}) {
	c.handlePodObject(obj, "update")
}
func (c *CloneController) handlePodDelete(obj interface{}) {
	c.handlePodObject(obj, "delete")
}

func (c *CloneController) expectPodCreate(pvcKey string) {
	c.podExpectations.ExpectCreations(pvcKey, 1)
}
func (c *CloneController) observePodCreate(pvcKey string) {
	c.podExpectations.CreationObserved(pvcKey)
}

func (c *CloneController) handlePodObject(obj interface{}, verb string) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(Vdebug).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(Vdebug).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		_, createdByUs := object.GetAnnotations()[AnnCloningCreatedBy]

		if ownerRef.Kind != "PersistentVolumeClaim" {
			return
		} else if !createdByUs {
			return
		}

		pvc, err := c.pvcLister.PersistentVolumeClaims(object.GetAnnotations()[AnnTargetPodNamespace]).Get(ownerRef.Name)
		if err != nil {
			glog.V(Vdebug).Infof("ignoring orphaned object '%s' of pvc '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		if verb == "add" {
			pvcKey, err := cache.MetaNamespaceKeyFunc(pvc)
			if err != nil {
				runtime.HandleError(err)
				return
			}

			c.observePodCreate(pvcKey)
		}
		c.enqueuePVC(pvc)
		return
	}
}

func (c *CloneController) enqueuePVC(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

func (c *CloneController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer func() {
		c.queue.ShutDown()
	}()
	glog.V(Vadmin).Infoln("Starting clone controller Run loop")
	if threadiness < 1 {
		return errors.Errorf("expected >0 threads, got %d", threadiness)
	}

	if !cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced) {
		return errors.New("Timeout waiting for pvc cache sync")
	}
	if !cache.WaitForCacheSync(stopCh, c.podInformer.HasSynced) {
		return errors.New("Timeout waiting for pod cache sync")
	}
	glog.V(Vdebug).Infoln("CloneController cache has synced")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runPVCWorkers, time.Second, stopCh)
	}
	<-stopCh
	return nil
}

func (c *CloneController) runPVCWorkers() {
	for c.ProcessNextPvcItem() {
	}
}

func (c *CloneController) syncPvc(key string) error {
	pvc, err := c.pvcFromKey(key)
	if err != nil {
		return err
	}
	if pvc == nil {
		return nil
	}
	// filter pvc and decide if the cloning pods should be created
	if !checkClonePVC(pvc) {
		return nil
	}
	glog.V(Vdebug).Infof("ProcessNextPvcItem: next pvc to process: %s\n", key)
	return c.processPvcItem(pvc)
}

// Select only pvcs with the 'CloneRequest' annotation and that are not being processed.
// We forget the key unless `processPvcItem` returns an error in which case the key can be
// retried.
func (c *CloneController) ProcessNextPvcItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncPvc(key.(string))
	if err != nil { // processPvcItem errors may not have been logged so log here
		glog.Errorf("error processing pvc %q: %v", key, err)
		return true
	}
	return c.forgetKey(key, fmt.Sprintf("ProcessNextPvcItem: processing pvc %q completed", key))
}

func (c *CloneController) findClonePodsFromCache(pvc *v1.PersistentVolumeClaim) (*v1.Pod, *v1.Pod, error) {
	var sourcePod, targetPod *v1.Pod
	annCloneRequest := pvc.GetAnnotations()[AnnCloneRequest]
	if annCloneRequest != "" {
		sourcePvcNamespace, _ := ParseSourcePvcAnnotation(annCloneRequest, "/")
		if sourcePvcNamespace == "" {
			return nil, nil, errors.Errorf("Bad CloneRequest Annotation")
		}
		//find the source pod
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{CloneUniqueID: pvc.Name + "-source-pod"}})
		if err != nil {
			return nil, nil, err
		}
		podList, err := c.podLister.Pods(sourcePvcNamespace).List(selector)
		if err != nil {
			return nil, nil, err
		}
		if len(podList) == 0 {
			return nil, nil, nil
		} else if len(podList) > 1 {
			return nil, nil, errors.Errorf("multiple source pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
		}
		sourcePod = podList[0]
		//find target pod
		selector, err = metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{CloneUniqueID: pvc.Name + "-target-pod"}})
		if err != nil {
			return nil, nil, err
		}
		podList, err = c.podLister.Pods(pvc.Namespace).List(selector)
		if err != nil {
			return nil, nil, err
		}
		if len(podList) == 0 {
			return nil, nil, nil
		} else if len(podList) > 1 {
			return nil, nil, errors.Errorf("multiple target pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
		}
		targetPod = podList[0]
	}
	return sourcePod, targetPod, nil
}

// Create the cloning source and target pods based the pvc. The pvc is checked (again) to ensure that we are not already
// processing this pvc, which would result in multiple pods for the same pvc.
func (c *CloneController) processPvcItem(pvc *v1.PersistentVolumeClaim) error {
	anno := map[string]string{}

	// find cloning source and target Pods
	sourcePod, targetPod, err := c.findClonePodsFromCache(pvc)
	if err != nil {
		return err
	}
	pvcKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return err
	}

	// Pods must be controlled by this PVC
	if sourcePod != nil && !metav1.IsControlledBy(sourcePod, pvc) {
		return errors.Errorf("found pod %s/%s not owned by pvc %s/%s", sourcePod.Namespace, sourcePod.Name, pvc.Namespace, pvc.Name)
	}
	if targetPod != nil && !metav1.IsControlledBy(sourcePod, pvc) {
		return errors.Errorf("found pod %s/%s not owned by pvc %s/%s", targetPod.Namespace, targetPod.Name, pvc.Namespace, pvc.Name)
	}

	// expectations prevent us from creating multiple pods. An expectation forces
	// us to observe a pod's creation in the cache.
	needsSync := c.podExpectations.SatisfiedExpectations(pvcKey)

	// make sure not to reprocess a PVC that has already completed successfully,
	// even if the pod no longer exists
	phase, exists := pvc.ObjectMeta.Annotations[AnnClonePodPhase]
	if exists && (phase == string(v1.PodSucceeded)) {
		needsSync = false
	}

	if needsSync && (sourcePod == nil || targetPod == nil) {
		//create random string to be used for pod labeling and hostpath name
		if sourcePod == nil {
			cr, err := getCloneRequestPVC(pvc)
			if err != nil {
				return err
			}
			// all checks passed, let's create the cloner pods!
			c.expectPodCreate(pvcKey)
			//create the source pod
			sourcePod, err = CreateCloneSourcePod(c.clientset, c.cloneImage, c.verbose, c.pullPolicy, cr, pvc)
			if err != nil {
				c.observePodCreate(pvcKey)
				return err
			}
		}
		if targetPod == nil {
			c.expectPodCreate(pvcKey)
			//create the target pod
			targetPod, err = CreateCloneTargetPod(c.clientset, c.cloneImage, c.verbose, c.pullPolicy, pvc, sourcePod.ObjectMeta.Namespace)
			if err != nil {
				c.observePodCreate(pvcKey)
				return err
			}
		}
		return nil
	}

	// update pvc with cloner pod name and optional cdi label
	//we update the target PVC according to the target pod. Only the target pods indicates the real status of the cloning.
	anno[AnnClonePodPhase] = string(targetPod.Status.Phase)
	//add the following annotation only if the pod pahse is succeeded, meaning job is completed
	if phase == string(v1.PodSucceeded) {
		anno[AnnCloneOf] = "true"
	}
	var lab map[string]string
	if !checkIfLabelExists(pvc, CDI_LABEL_KEY, CDI_LABEL_VALUE) {
		lab = map[string]string{CDI_LABEL_KEY: CDI_LABEL_VALUE}
	}
	pvc, err = updatePVC(c.clientset, pvc, anno, lab)
	if err != nil {
		return errors.WithMessage(err, "could not update pvc %q annotation and/or label")
	}
	return nil
}

// forget the passed-in key for this event and optionally log a message.
func (c *CloneController) forgetKey(key interface{}, msg string) bool {
	if len(msg) > 0 {
		glog.V(Vdebug).Info(msg)
	}
	c.queue.Forget(key)
	return true
}
