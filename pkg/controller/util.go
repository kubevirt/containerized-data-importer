package controller

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const DataVolName = "cdi-data-vol"

// return a pvc pointer based on the passed-in work queue key.
func (c *Controller) pvcFromKey(key interface{}) (*v1.PersistentVolumeClaim, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, fmt.Errorf("pvcFromKey: key object not of type string\n")
	}
	obj, ok, err := c.pvcInformer.GetIndexer().GetByKey(keyString)
	if err != nil {
		return nil, fmt.Errorf("pvcFromKey: Error getting key from cache: %q\n", keyString)
	}
	if !ok {
		return nil, fmt.Errorf("pvcFromKey: object %v not found", keyString)
	}
	pvc, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return nil, fmt.Errorf("pvcFromKey: Object not of type *v1.PersistentVolumeClaim\n")
	}
	return pvc, nil
}

func (c *Controller) podFromKey(key interface{}) (*v1.Pod, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, fmt.Errorf("podFromKey: keys is not of type string")
	}
	obj, ok, err := c.podInformer.GetIndexer().GetByKey(keyString)
	if err != nil {
		return nil, fmt.Errorf("podFromKey: error getting pod obj from store: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("podFromKey: pod not found in store")
	}
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil, fmt.Errorf("podFromKey: error casting object to type \"v1.Pod\"")
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
func (c *Controller) checkPVC(pvc *v1.PersistentVolumeClaim, get bool) (bool, error) {
	// check if we have proper AnnEndPoint annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, AnnEndpoint) {
		glog.Infof("checkPVC: annotation %q not found, skipping pvc\n", AnnEndpoint)
		return false, nil
	}
	//check if the pvc is being processed
	if metav1.HasAnnotation(pvc.ObjectMeta, AnnImportPod) {
		glog.Infof("checkPVC: pvc annotation %q exists indicating it is being or has been processed, skipping pvc\n", AnnImportPod)
		return false, nil
	}

	if !get {
		return true, nil // done checking this pvc, assume it's good to go
	}

	// get latest pvc object to help mitigate race and timing issues with latency between the
	// store and work queue to double check if we are already processing
	glog.Infof("checkPVC: getting latest version of pvc for in-process annotation")
	latest, err := c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		glog.Infof("checkPVC: pvc Get error: %v\n", err)
		return false, err
	}
	// check if we are processing this pvc now that we have the lastest copy
	if metav1.HasAnnotation(latest.ObjectMeta, AnnImportPod) {
		glog.Infof("checkPVC: pvc Get annotation %q exists indicating it is being or has been processed, skipping pvc\n", AnnImportPod)
		return false, nil
	}
	//continue to process pvc
	return true, nil
}

// returns the endpoint string which contains the full path URI of the target object to be copied.
func getEndpoint(pvc *v1.PersistentVolumeClaim) (string, error) {
	ep, found := pvc.Annotations[AnnEndpoint]
	if !found || ep == "" {
		verb := "empty"
		if !found {
			verb = "missing"
		}
		return ep, fmt.Errorf("getEndpoint: annotation %q in pvc \"%s/%s\" is %s\n", AnnEndpoint, pvc.Namespace, pvc.Name, verb)
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
		glog.Infof(msg+"\n", AnnSecret, ns, pvc.Name)
		return "", nil // importer pod will not contain secret credentials
	}
	glog.Infof("getEndpointSecret: retrieving Secret \"%s/%s\"\n", ns, name)
	_, err := c.clientset.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		glog.Infof("getEndpointSecret: secret %q defined in pvc \"%s/%s\" is missing. Importer pod will run once this secret is created\n", name, ns, pvc.Name)
		return name, nil
	}
	if err != nil {
		return "", fmt.Errorf("getEndpointSecret: error getting secret %q defined in pvc \"%s/%s\": %v\n", name, ns, pvc.Name, err)
	}
	return name, nil
}

func (c *Controller) patchPVC(oldData, newData []byte, pvc *v1.PersistentVolumeClaim, comp string) error {
	// patch the pvc clone
	patch, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.PersistentVolumeClaim{})
	if err != nil {
		return fmt.Errorf("error creating patch %v", err)
	}
	_, err = c.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(pvc.Name, types.StrategicMergePatchType, patch)
	if err != nil {
		return fmt.Errorf("error patching pvc %v", err)
	}
	return nil
}

func (c *Controller) clonePVC(claim *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, []byte, error) {
	pvcClone := claim.DeepCopy()
	data, err := json.Marshal(pvcClone)
	if err != nil {
		return pvcClone, nil, fmt.Errorf("marshal clone pvc data: %v\n", err)
	}
	return pvcClone, data, nil
}

// Sets an annotation `key: val` in the given PVC.
// Note: Patch() is used instead of Update() to handle version related field changes.
func (c *Controller) setPVCAnnotation(pvc *v1.PersistentVolumeClaim, key, val string) error {
	const funcTrace = "setPVCAnnotation"
	glog.Infof("Adding annotation \"%s: %s\" to pvc \"%s/%s\"\n", key, val, pvc.Namespace, pvc.Name)

	// don't mutate the original pvc since it's from the shared informer
	// make copies of old pvc
	pvcClone, oldData, err := c.clonePVC(pvc)
	if err != nil {
		return fmt.Errorf("%s: marshal clone pvc data: %v\n", funcTrace, err)
	}

	// add annotation to update pvc
	metav1.SetMetaDataAnnotation(&pvcClone.ObjectMeta, key, val)

	//make copies of new pvc
	newData, err := json.Marshal(pvcClone)
	if err != nil {
		return fmt.Errorf("%s: marshal new pvc data: %v\n", funcTrace, err)
	}

	//patch and merge the old and new pvc
	err = c.patchPVC(oldData, newData, pvc, funcTrace)
	if err != nil {
		return fmt.Errorf("%s: %v", funcTrace, err)
	}
	return nil
}

// checks if particular label exists in pvc
func (c *Controller) checkIfLabelExists(pvc *v1.PersistentVolumeClaim, lbl string, val string) bool {
	glog.Info("checkIfLabelExists")
	value, exists := pvc.ObjectMeta.Labels[lbl]
	if exists && value == val {
		return true
	}
	return false
}

// set the pvc's cdi label.
// Note: Patch() is used instead of Update() to handle version related field changes.
func (c *Controller) setCdiLabel(pvc *v1.PersistentVolumeClaim) error {
	const funcTrace = "setCdiLabel"
	glog.Infof("%s: adding label \"%s: %s\" to pvc %s\n", funcTrace, common.CDI_LABEL_KEY, common.CDI_LABEL_VALUE, pvc.Name)

	// don't mutate the original pvc since it's from the shared informer
	// make copies of old pvc
	pvcClone, oldData, err := c.clonePVC(pvc)
	if err != nil {
		return fmt.Errorf("%s: marshal clone pvc data: %v\n", funcTrace, err)
	}

	// add label
	setPvcMetaDataLabel(&pvcClone.ObjectMeta, common.CDI_LABEL_KEY, common.CDI_LABEL_VALUE)

	// make copy of updated pvc
	newData, err := json.Marshal(pvcClone)
	if err != nil {
		return fmt.Errorf("%s: error marshal new pvc data: %v\n", funcTrace, err)
	}

	// patch the pvc clone
	err = c.patchPVC(oldData, newData, pvc, funcTrace)
	if err != nil {
		return fmt.Errorf("%s: %v", funcTrace, err)
	}
	return nil
}

func setPvcMetaDataLabel(obj *metav1.ObjectMeta, key string, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// return a pointer to a pod which is created based on the passed-in endpoint, secret
// name, and pvc. A nil secret means the endpoint credentials are not passed to the
// importer pod.
func (c *Controller) createImporterPod(ep, secretName string, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	ns := pvc.Namespace
	pod := c.makeImporterPodSpec(ep, secretName, pvc)

	pod, err := c.clientset.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, fmt.Errorf("createImporterPod: Create failed: %v\n", err)
	}
	glog.Infof("importer pod \"%s/%s\" (image: %q) created\n", pod.Namespace, pod.Name, c.importerImage)
	return pod, nil
}

// return the importer pod spec based on the passed-in endpoint, secret and pvc.
func (c *Controller) makeImporterPodSpec(ep, secret string, pvc *v1.PersistentVolumeClaim) *v1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s-", common.IMPORTER_PODNAME, pvc.Name)

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
				common.CDI_LABEL_KEY: common.CDI_LABEL_VALUE,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            common.IMPORTER_PODNAME,
					Image:           c.importerImage,
					ImagePullPolicy: v1.PullPolicy(c.pullPolicy),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: common.IMPORTER_DATA_DIR,
						},
					},
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
			Name:  common.IMPORTER_ENDPOINT,
			Value: endpoint,
		},
	}
	if secret != "" {
		env = append(env, v1.EnvVar{
			Name: common.IMPORTER_ACCESS_KEY_ID,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
					},
					Key: common.KeyAccess,
				},
			},
		}, v1.EnvVar{
			Name: common.IMPORTER_SECRET_KEY,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secret,
					},
					Key: common.KeySecret,
				},
			},
		})
	}
	return env
}
