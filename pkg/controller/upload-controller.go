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
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/cert/triple"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"kubevirt.io/containerized-data-importer/pkg/keys"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"reflect"
)

const (
	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = "cdi.kubevirt.io/storage.upload.target"

	// cert/key annotations
	uploadServerCASecret = "cdi-upload-server-ca-key"
	uploadServerCAName   = "server.upload.cdi.kubevirt.io"

	uploadServerClientCASecret = "cdi-upload-server-client-ca-key"
	uploadServerClientCAName   = "client.upload-server.cdi.kubevirt.io"

	uploadServerClientKeySecret = "cdi-upload-server-client-key"
	uploadProxyClientName       = "uploadproxy.client.upload-server.cdi.kebevirt.io"

	uploadProxyCASecret     = "cdi-upload-proxy-ca-key"
	uploadProxyServerSecret = "cdi-upload-proxy-server-key"
	uploadProxyCAName       = "proxy.upload.cdi.kubevirt.io"
)

var uploader = "uploader"

// UploadController members
type UploadController struct {
	Controller
	serviceInformer        cache.SharedIndexInformer
	serviceLister          corelisters.ServiceLister
	servicesSynced         cache.InformerSynced
	serverCAKeyPair        *triple.KeyPair
	clientCAKeyPair        *triple.KeyPair
	uploadProxyServiceName string
}

// GetUploadResourceName returns the name given to upload services/pods
func GetUploadResourceName(pvcName string) string {
	return "cdi-upload-" + pvcName
}

// UploadPossibleForPVC is called by the api server to see whether to return an upload token
func UploadPossibleForPVC(pvc *v1.PersistentVolumeClaim) error {
	if _, ok := pvc.Annotations[AnnUploadRequest]; !ok {
		return errors.Errorf("PVC %s is not an upload target", pvc.Name)
	}

	pvcPodStatus, ok := pvc.Annotations[AnnPodPhase]
	if !ok || v1.PodPhase(pvcPodStatus) != v1.PodRunning {
		return errors.Errorf("Upload Server pod not currently running for PVC %s", pvc.Name)
	}

	return nil
}

// NewUploadController returns a new UploadController
func NewUploadController(client kubernetes.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	image string,
	uploadProxyServiceName string,
	pullPolicy string,
	verbose string) *UploadController {
	c := &UploadController{
		Controller:             *NewController(client, pvcInformer, podInformer, image, pullPolicy, verbose, uploader),
		serviceInformer:        serviceInformer.Informer(),
		serviceLister:          serviceInformer.Lister(),
		servicesSynced:         serviceInformer.Informer().HasSynced,
		uploadProxyServiceName: uploadProxyServiceName,
	}

	// Bind the service SharedIndexInformer to the service queue
	c.serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*v1.Service)
			oldDepl := old.(*v1.Service)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same Servicess will always have different RVs.
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	return c
}

//Run is being called from cdi-controller (cmd)
func (uc *UploadController) Run(threadiness int, stopCh <-chan struct{}) error {
	uc.Controller.run(threadiness, stopCh, uc)
	return nil
}

func (uc *UploadController) initCerts() error {
	var err error

	// CA for Upload Servers
	uc.serverCAKeyPair, err = keys.GetOrCreateCA(uc.clientset, util.GetNamespace(), uploadServerCASecret, uploadServerCAName)
	if err != nil {
		return errors.Wrap(err, "Couldn't get/create server CA")
	}

	// CA for Upload Client
	uc.clientCAKeyPair, err = keys.GetOrCreateCA(uc.clientset, util.GetNamespace(), uploadServerClientCASecret, uploadServerClientCAName)
	if err != nil {
		return errors.Wrap(err, "Couldn't get/create client CA")
	}

	// Upload Server Client Cert
	_, err = keys.GetOrCreateClientKeyPairAndCert(uc.clientset,
		util.GetNamespace(),
		uploadServerClientKeySecret,
		uc.clientCAKeyPair,
		uc.serverCAKeyPair.Cert,
		uploadProxyClientName,
		[]string{},
		nil,
	)
	if err != nil {
		return errors.Wrap(err, "Couldn't get/create client cert")
	}

	uploadProxyCAKeyPair, err := keys.GetOrCreateCA(uc.clientset, util.GetNamespace(), uploadProxyCASecret, uploadProxyCAName)
	if err != nil {
		return errors.Wrap(err, "Couldn't create upload proxy server cert")
	}

	_, err = keys.GetOrCreateServerKeyPairAndCert(uc.clientset,
		util.GetNamespace(),
		uploadProxyServerSecret,
		uploadProxyCAKeyPair,
		nil,
		uc.uploadProxyServiceName+"."+util.GetNamespace(),
		uc.uploadProxyServiceName,
		nil,
	)
	if err != nil {
		return errors.Wrap(err, "Error creating upload proxy server key pair")
	}

	return nil
}

//RunWorkers is declared in Controller WorkerRunner interface
func (uc *UploadController) RunWorkers() {
	for uc.processNextWorkItem() {
	}
}

//WaitForCacheSync is declared in Controller WorkerRunner interface
func (uc *UploadController) WaitForCacheSync(stopCh <-chan struct{}, c *Controller, v reflect.Value, controller interface{}) error {
	glog.V(2).Infoln("Getting/creating certs")
	//ownerController := controller.(*UploadController)
	if err := uc.initCerts(); err != nil {
		runtime.HandleError(err)
		return errors.Wrap(err, "Error initializing certificates")
	}
	waitForCacheSync("UploadController", stopCh, c, v, controller, uc)
	return nil
}

func (uc *UploadController) processNextWorkItem() bool {
	obj, shutdown := uc.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer uc.queue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			uc.queue.Forget(obj)
			runtime.HandleError(errors.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := uc.syncHandler(key); err != nil {
			return errors.Errorf("error syncing '%s': %s", key, err.Error())
		}

		uc.queue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil

	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (uc *UploadController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	pvc, err := uc.pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("PVC '%s' in work queue no longer exists", key))
			return nil
		}
		return errors.Wrapf(err, "error getting PVC %s", key)
	}

	resourceName := GetUploadResourceName(pvc.Name)

	if _, exists := pvc.ObjectMeta.Annotations[AnnUploadRequest]; !exists {
		// delete everything

		// delete service
		err = uc.deleteService(pvc.Namespace, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error deleting upload service for pvc: %s", key)
		}

		// delete pod
		err = uc.deletePod(pvc.Namespace, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error deleting upload pod for pvc: %s", key)
		}

		return nil
	}

	podPhaseFromPVC := func(pvc *v1.PersistentVolumeClaim) v1.PodPhase {
		phase, _ := pvc.ObjectMeta.Annotations[AnnPodPhase]
		return v1.PodPhase(phase)
	}

	podSucceededFromPVC := func(pvc *v1.PersistentVolumeClaim) bool {
		return (podPhaseFromPVC(pvc) == v1.PodSucceeded)
	}

	var pod *v1.Pod
	if !podSucceededFromPVC(pvc) {
		pod, err = uc.getOrCreateUploadPod(pvc, resourceName)
		if err != nil {
			return errors.Wrapf(err, "Error creating upload pod for pvc: %s", key)
		}

		podPhase := pod.Status.Phase
		if podPhase != podPhaseFromPVC(pvc) {
			var labels map[string]string
			annotations := map[string]string{AnnPodPhase: string(podPhase)}
			pvc, err = updatePVC(uc.clientset, pvc, annotations, labels)
			if err != nil {
				return errors.Wrapf(err, "Error updating pvc %s, pod phase %s", key, podPhase)
			}
		}
	}

	if podSucceededFromPVC(pvc) {
		// delete service
		if err = uc.deleteService(pvc.Namespace, resourceName); err != nil {
			return errors.Wrapf(err, "Error deleting upload service for pvc %s", key)
		}
	} else {
		// make sure the service exists
		if _, err = uc.getOrCreateUploadService(pvc, resourceName); err != nil {
			return errors.Wrapf(err, "Error getting/creating service resource for PVC %s", key)
		}
	}

	return nil
}

func (uc *UploadController) getOrCreateUploadPod(pvc *v1.PersistentVolumeClaim, name string) (*v1.Pod, error) {
	pod, err := uc.podLister.Pods(pvc.Namespace).Get(name)

	if k8serrors.IsNotFound(err) {
		pod, err = CreateUploadPod(uc.clientset, uc.serverCAKeyPair, uc.clientCAKeyPair.Cert, uc.image, uc.verbose, uc.pullPolicy, name, pvc)
	}

	if pod != nil && !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("%s pod not controlled by pvc %s", name, pvc.Name)
	}

	return pod, err
}

func (uc *UploadController) getOrCreateUploadService(pvc *v1.PersistentVolumeClaim, name string) (*v1.Service, error) {
	service, err := uc.serviceLister.Services(pvc.Namespace).Get(name)

	if k8serrors.IsNotFound(err) {
		service, err = CreateUploadService(uc.clientset, name, pvc)
	}

	if service != nil && !metav1.IsControlledBy(service, pvc) {
		return nil, errors.Errorf("%s service not controlled by pvc %s", name, pvc.Name)
	}

	return service, err
}

func (uc *UploadController) deletePod(namespace, name string) error {
	pod, err := uc.podLister.Pods(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err == nil && pod.DeletionTimestamp == nil {
		err = uc.clientset.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func (uc *UploadController) deleteService(namespace, name string) error {
	service, err := uc.serviceLister.Services(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err == nil && service.DeletionTimestamp == nil {
		err = uc.clientset.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}
