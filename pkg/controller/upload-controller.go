/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/cert/triple"
	"k8s.io/client-go/util/workqueue"
)

const (
	// pvc annotations

	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = "cdi.kubevirt.io/storage.upload.target"
	// AnnUploadPodPhase stores the status of the upload pod
	AnnUploadPodPhase = "cdi.kubevirt.io/storage.upload.pod.phase"

	// pod annotations

	// AnnCreatedByUpload marks that a particular resource was created by the upload controller
	AnnCreatedByUpload = "cdi.kubevirt.io/storage.createdByUploadController"

	// CASecret is the secret containing the ca private key
	CASecret = "cdi-upload-ca-key"
	// CAName is the name of the CA
	CAName = "upload.cdi.kubevirt.io"
)

// UploadController members
type UploadController struct {
	clientset                                 kubernetes.Interface
	queue                                     workqueue.RateLimitingInterface
	pvcInformer, podInformer, serviceInformer cache.SharedIndexInformer
	pvcLister                                 corelisters.PersistentVolumeClaimLister
	podLister                                 corelisters.PodLister
	serviceLister                             corelisters.ServiceLister
	pvcsSynced                                cache.InformerSynced
	podsSynced                                cache.InformerSynced
	servicesSynced                            cache.InformerSynced
	uploadServiceImage                        string
	pullPolicy                                string // Options: IfNotPresent, Always, or Never
	verbose                                   string // verbose levels: 1, 2, ...
	caKeyPair                                 *triple.KeyPair
}

// GetUploadResourceName returns the name given to upload services/pods
func GetUploadResourceName(pvcName string) string {
	return "cdi-upload-" + pvcName
}

// NewUploadController returns a new UploadController
func NewUploadController(client kubernetes.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	uploadServiceImage string,
	pullPolicy string,
	verbose string) *UploadController {
	c := &UploadController{
		clientset:          client,
		queue:              workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		pvcInformer:        pvcInformer.Informer(),
		podInformer:        podInformer.Informer(),
		serviceInformer:    serviceInformer.Informer(),
		pvcLister:          pvcInformer.Lister(),
		podLister:          podInformer.Lister(),
		serviceLister:      serviceInformer.Lister(),
		pvcsSynced:         pvcInformer.Informer().HasSynced,
		podsSynced:         podInformer.Informer().HasSynced,
		servicesSynced:     serviceInformer.Informer().HasSynced,
		uploadServiceImage: uploadServiceImage,
		pullPolicy:         pullPolicy,
		verbose:            verbose,
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
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*v1.Pod)
			oldDepl := old.(*v1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Pods.
				// Two different versions of the same PVCs will always have different RVs.
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	// Bind the pod SharedIndexInformer to the pod queue
	c.serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*v1.Service)
			oldDepl := old.(*v1.Service)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same PVCs will always have different RVs.
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	return c
}

// Run sets up UploadController state and executes main event loop
func (c *UploadController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	glog.V(2).Infoln("Getting/creating certs")

	if err := c.initCerts(); err != nil {
		runtime.HandleError(err)
		return errors.Wrap(err, "Error initializing certificates")
	}

	glog.V(2).Infoln("Starting cdi upload controller Run loop")

	if threadiness < 1 {
		return errors.Errorf("expected >0 threads, got %d", threadiness)
	}

	glog.V(3).Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.pvcsSynced, c.podsSynced, c.servicesSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	glog.V(3).Infoln("UploadController cache has synced")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c *UploadController) initCerts() error {
	var err error

	c.caKeyPair, err = GetOrCreateCA(c.clientset, GetNamespace(), CASecret, CAName, nil)
	if err != nil {
		return errors.Wrap(err, "Couldn't get/create CA")
	}

	return nil
}

func (c *UploadController) handleObject(obj interface{}) {
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
		glog.V(3).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(3).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		_, createdByUs := object.GetAnnotations()[AnnCreatedByUpload]

		if ownerRef.Kind != "PersistentVolumeClaim" || !createdByUs {
			return
		}

		pvc, err := c.pvcLister.PersistentVolumeClaims(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(3).Infof("ignoring orphaned object '%s' of pvc '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		glog.V(3).Infof("queueing pvc %+v!!", pvc)

		c.enqueuePVC(pvc)
		return
	}
}

func (c *UploadController) enqueuePVC(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

func (c *UploadController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *UploadController) processNextWorkItem() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj)
			runtime.HandleError(errors.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			return errors.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.queue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil

	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *UploadController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	pvc, err := c.pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("PVC '%s' in work queue no longer exists", key))
			return nil
		}
		return errors.Wrapf(err, "error getting PVC %s", key)
	}

	resourceName := GetUploadResourceName(pvc.Name)

	if _, exists := pvc.ObjectMeta.Annotations[AnnUploadRequest]; !exists {
		// delete everything

		// delete service
		err = c.deleteService(pvc.Namespace, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error deleting upload service for pvc: %s", key)
		}

		// delete pod
		err = c.deletePod(pvc.Namespace, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error deleting upload pod for pvc: %s", key)
		}

		return nil
	}

	podPhaseFromPVC := func(pvc *v1.PersistentVolumeClaim) string {
		phase, _ := pvc.ObjectMeta.Annotations[AnnUploadPodPhase]
		return phase
	}

	podSucceededFromPVC := func(pvc *v1.PersistentVolumeClaim) bool {
		return (podPhaseFromPVC(pvc) == string(v1.PodSucceeded))
	}

	if !podSucceededFromPVC(pvc) {
		pod, err := c.getOrCreateUploadPod(pvc, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error creating upload pod for pvc: %s", key)
		}

		podPhase := string(pod.Status.Phase)
		if podPhase != podPhaseFromPVC(pvc) {
			var labels map[string]string
			annotations := map[string]string{AnnUploadPodPhase: podPhase}
			pvc, err = updatePVC(c.clientset, pvc, annotations, labels)
			if err != nil {
				return errors.Wrapf(err, "Error updating pvc %s, pod phase %s", key, podPhase)
			}
		}
	}

	if podSucceededFromPVC(pvc) {
		// delete service
		if err = c.deleteService(pvc.Namespace, resourceName); err != nil {
			return errors.Wrapf(err, "Error deleting upload service for pvc %s", key)
		}
	} else {
		// make sure the service exists
		if _, err = c.getOrCreateUploadService(pvc, resourceName); err != nil {
			return errors.Wrapf(err, "Error getting/creating service resource for PVC %s", key)
		}
	}

	return nil
}

func (c *UploadController) getOrCreateUploadPod(pvc *v1.PersistentVolumeClaim, name string) (*v1.Pod, error) {
	pod, err := c.podLister.Pods(pvc.Namespace).Get(name)

	if apierrs.IsNotFound(err) {
		pod, err = CreateUploadPod(c.clientset, c.caKeyPair, c.uploadServiceImage, c.verbose, c.pullPolicy, name, pvc)
	}

	if pod != nil && !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("%s pod not controlled by pvc %s", name, pvc.Name)
	}

	return pod, err
}

func (c *UploadController) getOrCreateUploadService(pvc *v1.PersistentVolumeClaim, name string) (*v1.Service, error) {
	service, err := c.serviceLister.Services(pvc.Namespace).Get(name)

	if apierrs.IsNotFound(err) {
		service, err = CreateUploadService(c.clientset, name, pvc)
	}

	if service != nil && !metav1.IsControlledBy(service, pvc) {
		return nil, errors.Errorf("%s service not controlled by pvc %s", name, pvc.Name)
	}

	return service, err
}

func (c *UploadController) deletePod(namespace, name string) error {
	_, err := c.podLister.Pods(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		return nil
	}
	if err == nil {
		err = c.clientset.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}

func (c *UploadController) deleteService(namespace, name string) error {
	_, err := c.serviceLister.Services(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		return nil
	}
	if err == nil {
		err = c.clientset.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}
