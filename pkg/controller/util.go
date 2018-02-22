package controller

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

// return a pvc pointer based on the passed-in work queue key.
func (c *Controller) pvcFromKey(key interface{}) (*v1.PersistentVolumeClaim, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, fmt.Errorf("pvcFromKey(): key object not of type string\n")
	}
	obj, ok, err := c.pvcInformer.GetIndexer().GetByKey(keyString)
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pvcFromKey(): Error getting key from cache: %q\n", keyString)
	}
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return nil, fmt.Errorf("pvcFromKey(): Object not of type *v1.PersistentVolumeClaim\n")
	}
	return pvc, nil
}

// returns the endpoint string which contains the full path URI of the target object to be copied.
func getEndpoint(pvc *v1.PersistentVolumeClaim) (ep string) {
	ep, found := pvc.Annotations[annEndpoint]
	if !found || ep == "" {
		glog.Fatalf("getEndpoint: annotation %q in pvc %s/%s is missing!\n", annEndpoint, pvc.Namespace, pvc.Name)
	}
	return ep
}

// returns a pointer to the secret containing endpoint credentials consumed by the importer pod.
// Nil implies there are no credentials for the endpoint being used.
func getEndpointSecret(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) *v1.Secret {
	secretName, found := pvc.Annotations[annSecret]
	if !found || secretName == "" {
		return nil
	}
	ns := pvc.Namespace
	secret, err := client.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		glog.Errorf("getEndpointSecret: secret %q from pvc %s/%s does not exist. Secret ignored/n", secretName, ns, pvc.Name)
		return nil
	}
	if err != nil {
		glog.Errorf("getEndpointSecret: error getting secret %q from pvc %s/%s. Secret ignored: %v/n", secretName, ns, pvc.Name, err)
		return nil
	}
	return secret
}

// returns a pointer to a pod which is created based on the passed-in endpoint, secret, and pvc.
func createImporterPod(ep string, secret *v1.Secret, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {

	return nil, nil //TODO
}
