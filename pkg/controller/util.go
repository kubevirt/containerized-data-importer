package controller

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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

func pvcFromKey2(informer cache.SharedIndexInformer, key interface{}) (*v1.PersistentVolumeClaim, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, errors.New("key object not of type string")
	}
	obj, ok, err := informer.GetIndexer().GetByKey(keyString)
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

func podFromKey2(informer cache.SharedIndexInformer, key interface{}) (*v1.Pod, error) {
	keyString, ok := key.(string)
	if !ok {
		return nil, errors.New("keys is not of type string")
	}
	obj, ok, err := informer.GetIndexer().GetByKey(keyString)
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
func checkPVC(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, get bool) (bool, error) {
	// check if we have proper AnnEndPoint annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, AnnEndpoint) {
		glog.V(Vdebug).Infof("checkPVC: annotation %q not found, skipping pvc\n", AnnEndpoint)
		return false, nil
	}
	//check if the pvc is being processed
	if metav1.HasAnnotation(pvc.ObjectMeta, AnnImportPod) {
		glog.V(Vadmin).Infof("pvc annotation %q exists indicating it is being or has been processed, skipping pvc\n", AnnImportPod)
		return false, nil
	}

	if !get {
		return true, nil // done checking this pvc, assume it's good to go
	}

	// get latest pvc object to help mitigate race and timing issues with latency between the
	// store and work queue to double check if we are already processing
	glog.V(Vdebug).Infof("checkPVC: getting latest version of pvc for in-process annotation")
	latest, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		glog.Infof("checkPVC: pvc Get error: %v\n", err)
		return false, err
	}
	// check if we are processing this pvc now that we have the lastest copy
	if metav1.HasAnnotation(latest.ObjectMeta, AnnImportPod) {
		glog.V(Vadmin).Infof("pvc Get annotation %q exists indicating it is being or has been processed, skipping pvc\n", AnnImportPod)
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
		return ep, errors.Errorf("annotation %q in pvc \"%s/%s\" is %s\n", AnnEndpoint, pvc.Namespace, pvc.Name, verb)
	}
	return ep, nil
}

// returns the name of the secret containing endpoint credentials consumed by the importer pod.
// A value of "" implies there are no credentials for the endpoint being used. A returned error
// causes processNextItem() to stop.
func getSecretName(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) (string, error) {
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
	_, err := client.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
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

func PatchPVC(client kubernetes.Interface, oldData, newData []byte, pvc *v1.PersistentVolumeClaim) error {
	// patch the pvc clone
	patch, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.PersistentVolumeClaim{})
	if err != nil {
		return errors.Wrap(err, "error creating pvc patch")
	}
	_, err = client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(pvc.Name, types.StrategicMergePatchType, patch)
	if err != nil {
		return errors.Wrapf(err, "error patching pvc %q", pvc.Name)
	}
	return nil
}

func ClonePVC(claim *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, []byte, error) {
	pvcClone := claim.DeepCopy()
	data, err := json.Marshal(pvcClone)
	if err != nil {
		return pvcClone, nil, errors.Wrap(err, "error marshalling pvc object")
	}
	return pvcClone, data, nil
}

// Sets an annotation `key: val` in the given PVC.
// Note: Patch() is used instead of Update() to handle version related field changes.
func SetPVCAnnotation(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, key, val string) error {
	glog.V(Vdebug).Infof("Adding annotation \"%s: %s\" to pvc \"%s/%s\"\n", key, val, pvc.Namespace, pvc.Name)

	// don't mutate the original pvc since it's from the shared informer
	// make copies of old pvc
	// pvcClone, oldData, err := c.clonePVC(pvc)
	pvcClone, oldData, err := ClonePVC(pvc)
	if err != nil {
		return err
	}

	// add annotation to update pvc
	metav1.SetMetaDataAnnotation(&pvcClone.ObjectMeta, key, val)

	//make copies of new pvc
	newData, err := json.Marshal(pvcClone)
	if err != nil {
		return errors.Wrap(err, "error marshalling pvc object")
	}

	//patch and merge the old and new pvc
	// err = c.patchPVC(oldData, newData, pvc)
	err = PatchPVC(client, oldData, newData, pvc)
	if err != nil {
		return err
	}
	return nil
}

// checks if particular label exists in pvc
func CheckIfLabelExists(pvc *v1.PersistentVolumeClaim, lbl string, val string) bool {
	value, exists := pvc.ObjectMeta.Labels[lbl]
	if exists && value == val {
		return true
	}
	return false
}

// set the pvc's cdi label.
// Note: Patch() is used instead of Update() to handle version related field changes.
func SetCdiLabel(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) error {
	const funcTrace = "setCdiLabel"
	glog.V(Vdebug).Infof("%s: adding label \"%s: %s\" to pvc %s\n", funcTrace, CDI_LABEL_KEY, CDI_LABEL_VALUE, pvc.Name)

	// don't mutate the original pvc since it's from the shared informer
	// make copies of old pvc
	pvcClone, oldData, err := ClonePVC(pvc)
	if err != nil {
		return err
	}

	// add label
	setPvcMetaDataLabel(&pvcClone.ObjectMeta, CDI_LABEL_KEY, CDI_LABEL_VALUE)

	// make copy of updated pvc
	newData, err := json.Marshal(pvcClone)
	if err != nil {
		return errors.Wrap(err, "error marshalling new pvc data")
	}

	// patch the pvc clone
	err = PatchPVC(client, oldData, newData, pvc)
	if err != nil {
		return err
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
func createImporterPod(client kubernetes.Interface, image, verbose, pullPolicy, ep, secretName string, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	ns := pvc.Namespace
	pod := makeImporterPodSpec(client, image, verbose, pullPolicy, ep, secretName, pvc)
	//pod := c.makeImporterPodSpec(ep, secretName, pvc)


	// pod, err := c.clientset.CoreV1().Pods(ns).Create(pod)
	pod, err := client.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, errors.Wrap(err, "importer pod API create errored")
	}
	glog.V(Vuser).Infof("importer pod \"%s/%s\" (image: %q) created\n", pod.Namespace, pod.Name, image)
	return pod, nil
}

// return the importer pod spec based on the passed-in endpoint, secret and pvc.
func makeImporterPodSpec(client kubernetes.Interface, image, verbose, pullPolicy, ep, secret string, pvc *v1.PersistentVolumeClaim) *v1.Pod {
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
					Image:           image,
					ImagePullPolicy: v1.PullPolicy(pullPolicy),
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      DataVolName,
							MountPath: IMPORTER_DATA_DIR,
						},
					},
					Args: []string{"-v=" + verbose},
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
