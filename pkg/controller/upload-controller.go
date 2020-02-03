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
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = "cdi.kubevirt.io/storage.upload.target"

	// AnnUploadClientName is the TLS name uploadserver will accept requests from
	AnnUploadClientName = "cdi.kubevirt.io/uploadClientName"

	annCreatedByUpload = "cdi.kubevirt.io/storage.createdByUploadController"

	uploadServerClientName = "client.upload-server.cdi.kubevirt.io"

	uploadServerCertDuration = 365 * 24 * time.Hour
)

// UploadController members
type UploadController struct {
	client                                    kubernetes.Interface
	cdiClient                                 clientset.Interface
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
	uploadProxyServiceName                    string
	serverCertGenerator                       generator.CertGenerator
	clientCAFetcher                           fetcher.CertBundleFetcher
}

// GetUploadResourceName returns the name given to upload resources
func GetUploadResourceName(name string) string {
	// TODO revisit naming, could overflow
	return "cdi-upload-" + name
}

// UploadPossibleForPVC is called by the api server to see whether to return an upload token
func UploadPossibleForPVC(pvc *v1.PersistentVolumeClaim) error {
	if _, ok := pvc.Annotations[AnnUploadRequest]; !ok {
		return errors.Errorf("PVC %s is not an upload target", pvc.Name)
	}
	return nil
}

// GetUploadServerURL returns the url the proxy should post to for a particular pvc
func GetUploadServerURL(namespace, pvc, path string) string {
	return fmt.Sprintf("https://%s.%s.svc%s", GetUploadResourceName(pvc), namespace, path)
}

// NewUploadController returns a new UploadController
func NewUploadController(client kubernetes.Interface,
	cdiClientSet clientset.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	uploadServiceImage string,
	uploadProxyServiceName string,
	pullPolicy string,
	verbose string,
	serverCertGenerator generator.CertGenerator,
	clientCAFetcher fetcher.CertBundleFetcher) *UploadController {
	c := &UploadController{
		client:                 client,
		cdiClient:              cdiClientSet,
		queue:                  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		pvcInformer:            pvcInformer.Informer(),
		podInformer:            podInformer.Informer(),
		serviceInformer:        serviceInformer.Informer(),
		pvcLister:              pvcInformer.Lister(),
		podLister:              podInformer.Lister(),
		serviceLister:          serviceInformer.Lister(),
		pvcsSynced:             pvcInformer.Informer().HasSynced,
		podsSynced:             podInformer.Informer().HasSynced,
		servicesSynced:         serviceInformer.Informer().HasSynced,
		uploadServiceImage:     uploadServiceImage,
		uploadProxyServiceName: uploadProxyServiceName,
		pullPolicy:             pullPolicy,
		verbose:                verbose,
		serverCertGenerator:    serverCertGenerator,
		clientCAFetcher:        clientCAFetcher,
	}

	c.pvcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueObject,
		UpdateFunc: func(old, new interface{}) {
			c.enqueueObject(new)
		},
	})

	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*v1.Pod)
			oldPod := old.(*v1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	c.serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*v1.Service)
			oldService := old.(*v1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	return c
}

// Init does synchronous initialization before being considered "ready"
func (c *UploadController) Init() error {
	return nil
}

// Run sets up UploadController state and executes main event loop
func (c *UploadController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.V(2).Infoln("Starting cdi upload controller Run loop")

	if threadiness < 1 {
		return errors.Errorf("expected >0 threads, got %d", threadiness)
	}

	klog.V(3).Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.pvcsSynced, c.podsSynced, c.servicesSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	klog.V(3).Infoln("UploadController cache has synced")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *UploadController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(errors.New("error decoding object, invalid type"))
			return
		}

		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(errors.New("error decoding object tombstone, invalid type"))
			return
		}

		klog.V(3).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	klog.V(3).Infof("Processing object: %s", object.GetName())

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		_, createdByUs := object.GetAnnotations()[annCreatedByUpload]
		if ownerRef.Kind != "PersistentVolumeClaim" || !createdByUs {
			return
		}

		pvc, err := c.pvcLister.PersistentVolumeClaims(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(3).Infof("ignoring orphaned object '%s' of pvc '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		klog.V(3).Infof("queueing pvc %+v!!", pvc)
		c.enqueueObject(pvc)
	}
}

func (c *UploadController) enqueueObject(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
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
			return errors.Errorf("expected string in workqueue but got %#v", obj)
		}

		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.queue.AddRateLimited(key)
			return err
		}

		c.queue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
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
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("PVC '%s' in work queue no longer exists", key))
			return nil
		}
		return errors.Wrapf(err, "error getting PVC %s", key)
	}

	_, isUpload := pvc.Annotations[AnnUploadRequest]
	_, isCloneTarget := pvc.Annotations[AnnCloneRequest]

	if isUpload && isCloneTarget {
		runtime.HandleError(errors.Errorf("PVC has both clone and upload annotations"))
		return nil
	}

	// force cleanup if PVC pending delete and pod running or the upload/clone annotation was removed
	if (!isUpload && !isCloneTarget) || podSucceededFromPVC(pvc) || pvc.DeletionTimestamp != nil {
		klog.V(3).Infof("%s/%s not doing anything with: upload=%t, clone=%t, succeeded=%t, deleted=%t",
			pvc.Namespace, pvc.Name, isUpload, isCloneTarget, podSucceededFromPVC(pvc), pvc.DeletionTimestamp == nil)
		if err = c.cleanup(pvc); err != nil {
			return err
		}
		return nil
	}

	var uploadClientName, scratchPVCName string
	pvcCopy := pvc.DeepCopy()

	if isCloneTarget {
		source, err := getCloneRequestSourcePVC(pvc, c.pvcLister)
		if err != nil {
			return err
		}

		if err = ValidateCanCloneSourceAndTargetSpec(&source.Spec, &pvc.Spec); err != nil {
			klog.Errorf("Error %s validating clone spec, ignoring", err)
			return nil
		}

		uploadClientName = fmt.Sprintf("%s/%s-%s/%s", source.Namespace, source.Name, pvc.Namespace, pvc.Name)
		pvcCopy.Annotations[AnnUploadClientName] = uploadClientName
	} else {
		uploadClientName = uploadServerClientName

		// TODO revisit naming, could overflow
		scratchPVCName = pvc.Name + "-scratch"
	}

	resourceName := GetUploadResourceName(pvc.Name)

	pod, err := c.getOrCreateUploadPod(pvc, resourceName, scratchPVCName, uploadClientName)
	if err != nil {
		return err
	}

	if _, err = c.getOrCreateUploadService(pvc, resourceName); err != nil {
		return err
	}

	podPhase := pod.Status.Phase
	pvcCopy.Annotations[AnnPodPhase] = string(podPhase)
	pvcCopy.Annotations[AnnPodReady] = strconv.FormatBool(isPodReady(pod))

	if !reflect.DeepEqual(pvc, pvcCopy) {
		pvc, err = c.client.CoreV1().PersistentVolumeClaims(pvcCopy.Namespace).Update(pvcCopy)
		if err != nil {
			return errors.Wrapf(err, "error updating pvc %s, pod phase %s", key, podPhase)
		}
	}

	return nil
}

func (c *UploadController) cleanup(pvc *v1.PersistentVolumeClaim) error {
	resourceName := GetUploadResourceName(pvc.Name)

	// delete service
	if err := c.deleteService(pvc.Namespace, resourceName); err != nil {
		return err
	}

	// delete pod
	// we're using a req struct for now until we can normalize the controllers a bit more and share things like lister, client etc
	// this way it's easy to stuff everything into an easy request struct, and can extend aditional behaviors if we want going forward
	dReq := podDeleteRequest{
		namespace: pvc.Namespace,
		podName:   resourceName,
		podLister: c.podLister,
		k8sClient: c.client,
	}

	if err := deletePod(dReq); err != nil {
		return err
	}

	return nil
}

func (c *UploadController) getOrCreateUploadPod(pvc *v1.PersistentVolumeClaim, podName, scratchPVCName, clientName string) (*v1.Pod, error) {
	pod, err := c.podLister.Pods(pvc.Namespace).Get(podName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "error getting upload pod %s/%s", pvc.Namespace, podName)
		}

		serverCert, serverKey, err := c.serverCertGenerator.MakeServerCert(pvc.Namespace, podName, uploadServerCertDuration)
		if err != nil {
			return nil, err
		}

		clientCA, err := c.clientCAFetcher.BundleBytes()
		if err != nil {
			return nil, err
		}

		args := UploadPodArgs{
			Client:         c.client,
			CdiClient:      c.cdiClient,
			Image:          c.uploadServiceImage,
			Verbose:        c.verbose,
			PullPolicy:     c.pullPolicy,
			Name:           podName,
			PVC:            pvc,
			ScratchPVCName: scratchPVCName,
			ClientName:     clientName,
			ServerCert:     serverCert,
			ServerKey:      serverKey,
			ClientCA:       clientCA,
		}

		pod, err = CreateUploadPod(args)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("%s pod not controlled by pvc %s", podName, pvc.Name)
	}

	// Always try to get or create the scratch PVC for a pod that is not successful yet, if it exists nothing happens otherwise attempt to create.
	if scratchPVCName != "" {
		_, err = c.getOrCreateScratchPvc(pvc, pod, scratchPVCName)
		if err != nil {
			return nil, err
		}
	}

	return pod, nil
}

func (c *UploadController) getOrCreateScratchPvc(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name string) (*v1.PersistentVolumeClaim, error) {
	scratchPvc, err := c.pvcLister.PersistentVolumeClaims(pvc.Namespace).Get(name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting scratch PVC")
		}

		storageClassName := GetScratchPvcStorageClass(c.client, c.cdiClient, pvc)

		// Scratch PVC doesn't exist yet, create it.
		scratchPvc, err = CreateScratchPersistentVolumeClaim(c.client, pvc, pod, name, storageClassName)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(scratchPvc, pod) {
		return nil, errors.Errorf("%s scratch PVC not controlled by pod %s", scratchPvc.Name, pod.Name)
	}

	return scratchPvc, nil
}

func (c *UploadController) getOrCreateUploadService(pvc *v1.PersistentVolumeClaim, name string) (*v1.Service, error) {
	service, err := c.serviceLister.Services(pvc.Namespace).Get(name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting upload service")
		}

		service, err = CreateUploadService(c.client, name, pvc)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(service, pvc) {
		return nil, errors.Errorf("%s service not controlled by pvc %s", name, pvc.Name)
	}

	return service, nil
}

func (c *UploadController) deleteService(namespace, name string) error {
	service, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "error getting upload service")
	}

	if service.DeletionTimestamp == nil {
		err = c.client.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return errors.Wrap(err, "error deleting upload service")
		}
	}

	return nil
}
