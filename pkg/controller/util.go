package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const DataVolName = "cdi-data-vol"

// return a pvc pointer based on the passed-in work queue key.
func (c *Controller) pvcFromKey(key interface{}) (*v1.PersistentVolumeClaim, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, errors.New("key object not of type string")
	}
	obj, ok, err := c.pvcInformer.GetIndexer().GetByKey(keyString)
	if err != nil {
		return nil, errors.Wrapf(err, "Error getting key %q from cache", keyString)
	}
	if !ok {
		return nil, errors.Errorf("key %q not found in cache", keyString)
	}
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return nil, errors.New("Object not of type *v1.PersistentVolumeClaim")
	}
	return pvc, nil
}

func (c *Controller) podFromKey(key interface{}) (*v1.Pod, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, errors.New("keys is not of type string")
	}
	obj, ok, err := c.podInformer.GetIndexer().GetByKey(keyString)
	if err != nil {
		return nil, errors.Wrap(err, "error getting pod obj from store")
	}
	if !ok {
		return nil, errors.New("pod not found in store")
	}
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil, errors.New("error casting object to type \"v1.Pod\"")
	}
	return pod, nil
}

// checkPVC verifies that the passed-in pvc is one we care about. Specifically, it must have the
// endpoint annotation and it must not already be "in-progress". If the pvc passes these filters
// then true is returned and the importer pod will be created. `AnnEndPoint` indicates that the
// pvc is targeted for the importer pod. `AnnImportPod` indicates the  pvc is being processed.
// Note: there is a race condition where the AnnImportPod annotation is not seen in time and as
//   a result the importer pod can be created twice (or more, presumably). To reduce this window
//   a Get api call can be requested in order to get the latest copy of the pvc before verifying
//   its annotations.
func (c *Controller) checkPVC(pvc *v1.PersistentVolumeClaim, get bool) (ok bool, newPvc *v1.PersistentVolumeClaim, err error) {
	// check if we have proper AnnEndPoint annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, AnnEndpoint) {
		glog.V(Vadmin).Infof("pvc annotation %q not found, skipping pvc\n", AnnEndpoint)
		return false, pvc, nil
	}
	//check if the pvc is being processed
	if metav1.HasAnnotation(pvc.ObjectMeta, AnnImportPod) {
		glog.V(Vadmin).Infof("pvc annotation %q exists indicating in-progress or completed, skipping pvc\n", AnnImportPod)
		return false, pvc, nil
	}

	if !get {
		return true, pvc, nil // done checking this pvc, assume it's good to go
	}

	// get latest pvc object to help mitigate race and timing issues with latency between the
	// store and work queue to double check if we are already processing
	glog.V(Vdebug).Infof("checkPVC: getting latest version of pvc for in-process annotation")
	newPvc, err = c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		glog.Infof("checkPVC: pvc %q Get error: %v\n", pvc.Name, err)
		return false, pvc, err
	}
	// check if we are processing this pvc now that we have the lastest copy
	if metav1.HasAnnotation(newPvc.ObjectMeta, AnnImportPod) {
		glog.V(Vadmin).Infof("latest pvc annotation %q exists indicating in-progress or completed, skipping pvc\n", AnnImportPod)
		return false, newPvc, nil
	}
	//continue to process pvc
	return true, newPvc, nil
}

// returns the endpoint string which contains the full path URI of the target object to be copied.
func getEndpoint(pvc *v1.PersistentVolumeClaim) (string, error) {
	ep, found := pvc.Annotations[AnnEndpoint]
	if !found || ep == "" {
		verb := "empty"
		if !found {
			verb = "missing"
		}
		return ep, errors.Errorf("annotation %q in pvc \"%s/%s\" is %s\n", AnnEndpoint, pvc.Namespace, pvc.Name, verb)
	}
	return ep, nil
}

// returns the name of the secret containing endpoint credentials consumed by the importer pod.
// A value of "" implies there are no credentials for the endpoint being used. A returned error
// causes processNextItem() to stop.
func (c *Controller) getSecretName(pvc *v1.PersistentVolumeClaim) (string, error) {
	ns := pvc.Namespace
	name, found := pvc.Annotations[AnnSecret]
	if !found || name == "" {
		msg := "getEndpointSecret: "
		if !found {
			msg += "annotation %q is missing in pvc \"%s/%s\""
		} else {
			msg += "secret name is missing from annotation %q in pvc \"%s/%s\""
		}
		glog.V(Vadmin).Infof(msg+"\n", AnnSecret, ns, pvc.Name)
		return "", nil // importer pod will not contain secret credentials
	}
	glog.V(Vdebug).Infof("getEndpointSecret: retrieving Secret \"%s/%s\"\n", ns, name)
	_, err := c.clientset.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		glog.V(Vuser).Infof("secret %q defined in pvc \"%s/%s\" is missing. Importer pod will run once this secret is created\n", name, ns, pvc.Name)
		return name, nil
	}
	if err != nil {
		return "", errors.Wrapf(err, "error getting secret %q defined in pvc \"%s/%s\"", name, ns, pvc.Name)
	}
	glog.V(Vuser).Infof("retrieved secret %q defined in pvc \"%s/%s\"\n", name, ns, pvc.Name)
	return name, nil
}

// Update and return a copy of the passed-in pvc. Only one of the annotation or label maps is requred though
// both can be passed.
// Note: the only pvc changes supported are annotations and labels.
func (c *Controller) updatePVC(pvc *v1.PersistentVolumeClaim, anno, label map[string]string) (*v1.PersistentVolumeClaim, error) {
	glog.V(Vdebug).Infof("updatePVC: updating pvc \"%s/%s\" with anno: %+v and label: %+v", pvc.Namespace, pvc.Name, anno, label)

	applyUpdt := func(claim *v1.PersistentVolumeClaim, a, l map[string]string) {
		if a != nil {
			claim.ObjectMeta.Annotations = addToMap(claim.ObjectMeta.Annotations, a)
		}
		if l != nil {
			claim.ObjectMeta.Labels = addToMap(claim.ObjectMeta.Labels, l)
		}
	}

	var updtPvc *v1.PersistentVolumeClaim
	nsName := fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)
	// don't mutate the passed-in pvc since it's likely from the shared informer
	pvcCopy := pvc.DeepCopy()

	// loop a few times in case the pvc is stale
	err := wait.PollImmediate(time.Second*1, time.Second*10, func() (bool, error) {
		var e error
		applyUpdt(pvcCopy, anno, label)
		updtPvc, e = c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(pvcCopy)
		if e == nil {
			return true, nil // successful update
		}
		if apierrs.IsConflict(e) { // pvc is likely stale
			glog.V(Vdebug).Infof("pvc %q is stale, re-trying\n", nsName)
			pvcCopy, e = c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
			if e == nil {
				return false, nil // retry update
			}
			// Get failed, start over
			pvcCopy = pvc.DeepCopy()
		}
		glog.Errorf("%q update/get error: %v\n", nsName, e)
		return false, nil // retry
	})

	if err == nil {
		glog.V(Vdebug).Infof("updatePVC: pvc %q updated", nsName)
		return updtPvc, nil
	}
	return pvc, errors.Wrapf(err, "error updating pvc %q\n", nsName)
}

// Sets an annotation `key: val` in the given pvc. Returns the updated pvc.
func (c *Controller) setPVCAnnotation(pvc *v1.PersistentVolumeClaim, key, val string) (*v1.PersistentVolumeClaim, error) {
	glog.V(Vdebug).Infof("setPVCAnnotation: adding annotation \"%s: %s\" to pvc \"%s/%s\"\n", key, val, pvc.Namespace, pvc.Name)
	return c.updatePVC(pvc, map[string]string{key: val}, nil)
}

// checks if annotation `key` has a value of `val`.
func (c *Controller) checkIfAnnoExists(pvc *v1.PersistentVolumeClaim, key string, val string) bool {
	value, exists := pvc.ObjectMeta.Annotations[key]
	if exists && value == val {
		return true
	}
	return false
}

// checks if particular label exists in pvc
func (c *Controller) checkIfLabelExists(pvc *v1.PersistentVolumeClaim, lbl string, val string) bool {
	value, exists := pvc.ObjectMeta.Labels[lbl]
	if exists && value == val {
		return true
	}
	return false
}

// return a pointer to a pod which is created based on the passed-in endpoint, secret
// name, and pvc. A nil secret means the endpoint credentials are not passed to the
// importer pod.
func (c *Controller) createImporterPod(ep, secretName string, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	ns := pvc.Namespace
	pod := c.makeImporterPodSpec(ep, secretName, pvc)

	pod, err := c.clientset.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, errors.Wrap(err, "importer pod API create errored")
	}
	glog.V(Vuser).Infof("importer pod \"%s/%s\" (image: %q) created\n", pod.Namespace, pod.Name, c.importerImage)
	return pod, nil
}

// return the importer pod spec based on the passed-in endpoint, secret and pvc.
func (c *Controller) makeImporterPodSpec(ep, secret string, pvc *v1.PersistentVolumeClaim) *v1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s-", IMPORTER_PODNAME, pvc.Name)

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				CDI_LABEL_KEY: CDI_LABEL_VALUE,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            IMPORTER_PODNAME,
					Image:           c.importerImage,
					ImagePullPolicy: v1.PullPolicy(c.pullPolicy),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: IMPORTER_DATA_DIR,
						},
					},
					Args: []string{"-v=" + c.verbose},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: DataVolName,
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
	pod.Spec.Containers[0].Env = makeEnv(ep, secret)
	return pod
}

// return the Env portion for the importer container.
func makeEnv(endpoint, secret string) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  IMPORTER_ENDPOINT,
			Value: endpoint,
		},
	}
	if secret != "" {
		env = append(env, v1.EnvVar{
			Name: IMPORTER_ACCESS_KEY_ID,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
					},
					Key: KeyAccess,
				},
			},
		}, v1.EnvVar{
			Name: IMPORTER_SECRET_KEY,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
					},
					Key: KeySecret,
				},
			},
		})
	}
	return env
}

// Return a new map consisting of map1 with map2 added. In general, map2 is expected to have a single key. eg
// a single annotation or label. If map1 has the same key as map2 then map2's value is used.
func addToMap(m1, m2 map[string]string) map[string]string {
	if m1 == nil {
		m1 = make(map[string]string)
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}
