package controller

import (
	"fmt"

	_ "github.com/golang/glog"
	"k8s.io/api/core/v1"
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
func getEndpoint(pvc *v1.PersistentVolumeClaim) string {

	return "" //TODO
}

// returns a pointer to the secret, containing endpoint credentials, consumed by the importer
// pod. Nil implies there are no credentials for the endpoint being used.
func getEndpointSecret(pvc *v1.PersistentVolumeClaim) *v1.Secret {

	return nil //TODO
}

// returns a pointer to a pod which is created based on the passed-in endpoint, secret, and pvc.
func createImporterPod(ep string, secret *v1.Secret, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {

	return nil, nil //TODO
}
