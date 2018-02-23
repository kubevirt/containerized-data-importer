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
		return nil, fmt.Errorf("pvcFromKey: key object not of type string\n")
	}
	obj, ok, err := c.pvcInformer.GetIndexer().GetByKey(keyString)
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pvcFromKey: Error getting key from cache: %q\n", keyString)
	}
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return nil, fmt.Errorf("pvcFromKey: Object not of type *v1.PersistentVolumeClaim\n")
	}
	return pvc, nil
}

// returns the endpoint string which contains the full path URI of the target object to be copied.
func getEndpoint(pvc *v1.PersistentVolumeClaim) (string, error) {
	ep, found := pvc.Annotations[annEndpoint]
	if !found || ep == "" {
		// annotation was present and is now missing or is blank
		return ep, fmt.Errorf("getEndpoint: annotation %q in pvc %s/%s is missing or is blank\n", annEndpoint, pvc.Namespace, pvc.Name)
	}
	return ep, nil
}

// returns a pointer to the secret containing endpoint credentials consumed by the importer pod.
// Nil implies there are no credentials for the endpoint being used.
func getEndpointSecret(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) *v1.Secret {
	ns := pvc.Namespace
	secretName, found := pvc.Annotations[annSecret]
	if !found || secretName == "" {
		glog.Infof("secret %q is missing in pvc %s/%s\n", annSecret, ns, pvc.Name)
		return nil
	}
	glog.Infof("retrieving Secret %s/%s\n", ns, secretName)
	secret, err := client.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		glog.Errorf("getEndpointSecret: secret %q defined in pvc %s/%s does not exist\n", secretName, ns, pvc.Name)
		return nil
	}
	if err != nil {
		glog.Errorf("getEndpointSecret: error getting secret %q defined in pvc %s/%s: %v\n", secretName, ns, pvc.Name, err)
		return nil
	}
	return secret
}

// returns a pointer to a pod which is created based on the passed-in endpoint,
// secret, and pvc.
func createImporterPod(ep string, secret *v1.Secret, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {

	return &v1.Pod{}, nil //TODO
}
