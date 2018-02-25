package controller

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// Nil pvc implies there are no credentials for the endpoint being used. A returned error
// causes processNextItem() to stop. If returned skipPVC is true this pvc is not processed but
// will be requeued.
func (c *Controller) getEndpointSecret(pvc *v1.PersistentVolumeClaim) (secret *v1.Secret, skipPVC bool, err error) {
	ns := pvc.Namespace
	secretName, found := pvc.Annotations[annSecret]
	if !found {
		glog.Infof("getEndpointSecret: annotation %q is missing in pvc %s/%s\n", annSecret, ns, pvc.Name)
		return nil, false, nil // no error
	}
	if secretName == "" {
		glog.Infof("getEndpointSecret: secret name is missing from annotation %q in pvc \"%s/%s\n", annSecret, ns, pvc.Name)
		return nil, false, nil // no error
	}
	glog.Infof("getEndpointSecret: retrieving Secret \"%s/%s\"\n", ns, secretName)
	secret, err = c.clientset.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		glog.Infof("getEndpointSecret: secret %q defined in pvc \"%s/%s\" is missing\n", secretName, ns, pvc.Name)
		return nil, true, nil // no error
	}
	if err != nil {
		return nil, true, fmt.Errorf("getEndpointSecret: error getting secret %q defined in pvc \"%s/%s\": %v\n", secretName, ns, pvc.Name, err)
	}
	return secret, false, nil
}

// set the pvc's "status" annotation to the passed-in value.
func (c *Controller) setPVCStatus(pvc *v1.PersistentVolumeClaim, status string) (*v1.PersistentVolumeClaim, error) {
	if val, ok := pvc.Annotations[annStatus]; ok && val == status {
		return pvc, nil // annotation already set
	}
	// don't mutate the original pvc since it's from the shared informer
	pvcClone := pvc.DeepCopy()
	metav1.SetMetaDataAnnotation(&pvcClone.ObjectMeta, annStatus, status)
	newPVC, err := c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(pvcClone)
	if err != nil {
		return nil, fmt.Errorf("error updating pvc %s/%s: %v\n", pvc.Namespace, pvc.Name, err)
	}
	return newPVC, nil
}

// return a pointer to a pod which is created based on the passed-in endpoint,
// secret, and pvc. A nil secret means the endpoint credentials are not passed
// to the importer pod.
func (c *Controller) createImporterPod(ep string, secret *v1.Secret, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	ns := pvc.Namespace
	pod := makeImporterPodSpec(ep, secret, pvc)
	var err error
	pod, err = c.clientset.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, fmt.Errorf("createImporterPod: Create failed: %v\n", err)
	}
	glog.Infof("importer pod \"%s/%s\" created\n", pod.Namespace, pod.Name)
	return pod, nil
}

// return the importer pod spec based on the passed-in endpoint, secret and pvc.
func makeImporterPodSpec(ep string, secret *v1.Secret, pvc *v1.PersistentVolumeClaim) *v1.Pod {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "importer-",
			Annotations: map[string]string{
				annCreatedBy: "yes",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "importer",
					Image:   "docker.io/jcoperh/importer:latest",
					ImagePullPolicy: v1.PullAlways,
					Env: []v1.EnvVar{
						{
							Name: "IMPORTER_ENDPOINT",
							Value: ep,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name: "data-path",
							MountPath: "/data",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "data-path",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
	if secret != nil {
	}
	return pod
}
