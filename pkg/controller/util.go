package controller

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	crdv1alpha1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

const (
	// DataVolName provides a const to use for creating volumes in pod specs
	DataVolName = "cdi-data-vol"

	// CertVolName is the name of the volumecontaining certs
	CertVolName = "cdi-cert-vol"

	// ScratchVolName provides a const to use for creating scratch pvc volumes in pod specs
	ScratchVolName = "cdi-scratch-vol"

	// ImagePathName provides a const to use for creating volumes in pod specs
	ImagePathName  = "image-path"
	socketPathName = "socket-path"

	// SourceHTTP is the source type HTTP, if unspecified or invalid, it defaults to SourceHTTP
	SourceHTTP = "http"
	// SourceS3 is the source type S3
	SourceS3 = "s3"
	// SourceGlance is the source type of glance
	SourceGlance = "glance"
	// SourceNone means there is no source.
	SourceNone = "none"
	// SourceRegistry is the source type of Registry
	SourceRegistry = "registry"
	// AnnAPIGroup is the APIGroup for CDI
	AnnAPIGroup = "cdi.kubevirt.io"
	// AnnCreatedBy is a pod annotation indicating if the pod was created by the PVC
	AnnCreatedBy = AnnAPIGroup + "/storage.createdByController"
	// AnnPodPhase is a PVC annotation indicating the related pod progress (phase)
	AnnPodPhase = AnnAPIGroup + "/storage.pod.phase"
	// AnnPodReady tells whether the pod is ready
	AnnPodReady = AnnAPIGroup + "/storage.pod.ready"
	// AnnOwnerRef is used when owner is in a different namespace
	AnnOwnerRef = AnnAPIGroup + "/storage.ownerRef"
	// SourceImageio is the source type ovirt-imageio
	SourceImageio = "imageio"
)

type podDeleteRequest struct {
	namespace string
	podName   string
	podLister corelisters.PodLister
	k8sClient kubernetes.Interface
}

func checkPVC(pvc *v1.PersistentVolumeClaim, annotation string) bool {
	// check if we have proper annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, annotation) {
		klog.V(2).Infof("pvc annotation %q not found, skipping pvc \"%s/%s\"\n", annotation, pvc.Namespace, pvc.Name)
		return false
	}

	return true
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

func getDiskID(pvc *v1.PersistentVolumeClaim) string {
	diskID, _ := pvc.Annotations[AnnDiskID]
	return diskID
}

func getRequestedImageSize(pvc *v1.PersistentVolumeClaim) (string, error) {
	pvcSize, found := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	if !found {
		return "", errors.Errorf("storage request is missing in pvc \"%s/%s\"", pvc.Namespace, pvc.Name)
	}
	return pvcSize.String(), nil
}

// returns the source string which determines the type of source. If no source or invalid source found, default to http
func getSource(pvc *v1.PersistentVolumeClaim) string {
	source, found := pvc.Annotations[AnnSource]
	if !found {
		source = ""
	}
	switch source {
	case
		SourceHTTP,
		SourceS3,
		SourceGlance,
		SourceNone,
		SourceRegistry,
		SourceImageio:
		klog.V(2).Infof("pvc source annotation found for pvc \"%s/%s\", value %s\n", pvc.Namespace, pvc.Name, source)
	default:
		klog.V(2).Infof("No valid source annotation found for pvc \"%s/%s\", default to http\n", pvc.Namespace, pvc.Name)
		source = SourceHTTP
	}
	return source
}

// returns the source string which determines the type of source. If no source or invalid source found, default to http
func getContentType(pvc *v1.PersistentVolumeClaim) string {
	contentType, found := pvc.Annotations[AnnContentType]
	if !found {
		contentType = ""
	}
	switch contentType {
	case
		string(cdiv1.DataVolumeKubeVirt),
		string(cdiv1.DataVolumeArchive):
		klog.V(2).Infof("pvc content type annotation found for pvc \"%s/%s\", value %s\n", pvc.Namespace, pvc.Name, contentType)
	default:
		klog.V(2).Infof("No content type annotation found for pvc \"%s/%s\", default to kubevirt\n", pvc.Namespace, pvc.Name)
		contentType = string(cdiv1.DataVolumeKubeVirt)
	}
	return contentType
}

// returns the volumeMode which determines if the PVC is block PVC or not.
func getVolumeMode(pvc *v1.PersistentVolumeClaim) v1.PersistentVolumeMode {
	if pvc.Spec.VolumeMode != nil {
		return *pvc.Spec.VolumeMode
	}
	return v1.PersistentVolumeFilesystem
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
		klog.V(2).Infof(msg+"\n", AnnSecret, ns, pvc.Name)
		return "", nil // importer pod will not contain secret credentials
	}
	return name, nil
}

// Update and return a copy of the passed-in pvc. Only one of the annotation or label maps is required though
// both can be passed.
// Note: the only pvc changes supported are annotations and labels.
func updatePVC(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, anno, label map[string]string) (*v1.PersistentVolumeClaim, error) {
	klog.V(3).Infof("updatePVC: updating pvc \"%s/%s\" with anno: %+v and label: %+v", pvc.Namespace, pvc.Name, anno, label)
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
		updtPvc, e = client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(pvcCopy)
		if e == nil {
			return true, nil // successful update
		}
		if k8serrors.IsConflict(e) { // pvc is likely stale
			klog.V(3).Infof("pvc %q is stale, re-trying\n", nsName)
			pvcCopy, e = client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
			if e == nil {
				return false, nil // retry update
			}
			// Get failed, start over
			pvcCopy = pvc.DeepCopy()
		}
		klog.Errorf("%q update/get error: %v\n", nsName, e)
		return false, nil // retry
	})

	if err == nil {
		klog.V(3).Infof("updatePVC: pvc %q updated", nsName)
		return updtPvc, nil
	}
	return pvc, errors.Wrapf(err, "error updating pvc %q\n", nsName)
}

// Sets an annotation `key: val` in the given pvc. Returns the updated pvc.
func setPVCAnnotation(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, key, val string) (*v1.PersistentVolumeClaim, error) {
	klog.V(3).Infof("setPVCAnnotation: adding annotation \"%s: %s\" to pvc \"%s/%s\"\n", key, val, pvc.Namespace, pvc.Name)
	return updatePVC(client, pvc, map[string]string{key: val}, nil)
}

// checks if annotation `key` has a value of `val`.
func checkIfAnnoExists(pvc *v1.PersistentVolumeClaim, key string, val string) bool {
	value, exists := pvc.ObjectMeta.Annotations[key]
	if exists && value == val {
		return true
	}
	return false
}

// checks if particular label exists in pvc
func checkIfLabelExists(pvc *v1.PersistentVolumeClaim, lbl string, val string) bool {
	value, exists := pvc.ObjectMeta.Labels[lbl]
	if exists && value == val {
		return true
	}
	return false
}

// newScratchPersistentVolumeClaimSpec creates a new PVC based on the size of the passed in PVC.
// It also sets the appropriate OwnerReferences on the resource
// which allows handleObject to discover the pod resource that 'owns' it, and clean up when needed.
func newScratchPersistentVolumeClaimSpec(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name, storageClassName string) *v1.PersistentVolumeClaim {
	labels := map[string]string{
		"cdi-controller": pod.Name,
		"app":            "containerized-data-importer",
		LabelImportPvc:   pvc.Name,
	}

	annotations := make(map[string]string, 0)
	// Copy kubevirt.io annotations, but NOT the CDI annotations as those will trigger another import/upload/clone on the scratchspace
	// pvc.
	if len(pvc.GetAnnotations()) > 0 {
		for k, v := range pvc.GetAnnotations() {
			if strings.Contains(k, common.KubeVirtAnnKey) && !strings.Contains(k, common.CDIAnnKey) {
				annotations[k] = v
			}
		}
	}
	pvcDef := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   pvc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				MakePodOwnerReference(pod),
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources:   pvc.Spec.Resources,
		},
	}
	if storageClassName != "" {
		pvcDef.Spec.StorageClassName = &storageClassName
	}
	return pvcDef
}

// CreateScratchPersistentVolumeClaim creates and returns a pointer to a scratch PVC which is created based on the passed-in pvc and storage class name.
func CreateScratchPersistentVolumeClaim(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name, storageClassName string) (*v1.PersistentVolumeClaim, error) {
	ns := pvc.Namespace
	scratchPvcSpec := newScratchPersistentVolumeClaimSpec(pvc, pod, name, storageClassName)
	scratchPvc, err := client.CoreV1().PersistentVolumeClaims(ns).Create(scratchPvcSpec)
	if err != nil {
		return nil, errors.Wrap(err, "scratch PVC API create errored")
	}
	klog.V(3).Infof("scratch PVC \"%s/%s\" created\n", scratchPvc.Namespace, scratchPvc.Name)
	return scratchPvc, nil
}

// GetScratchPvcStorageClass tries to determine which storage class to use for use with a scratch persistent
// volume claim. The order of preference is the following:
// 1. Defined value in CDI Config field scratchSpaceStorageClass.
// 2. If 1 is not available, use the storage class name of the original pvc that will own the scratch pvc.
// 3. If none of those are available, return blank.
func GetScratchPvcStorageClass(client kubernetes.Interface, cdiclient clientset.Interface, pvc *v1.PersistentVolumeClaim) string {
	config, err := cdiclient.CdiV1alpha1().CDIConfigs().Get(common.ConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to find CDI configuration, %v\n", err)
	}
	storageClassName := config.Status.ScratchSpaceStorageClass
	if storageClassName == "" {
		// Unable to determine scratch storage class, attempt to read the storage class from the pvc.
		if pvc.Spec.StorageClassName != nil {
			storageClassName = *pvc.Spec.StorageClassName
			if storageClassName != "" {
				return storageClassName
			}
		}
	} else {
		return storageClassName
	}
	return ""
}

// GetDefaultPodResourceRequirements gets default pod resource requirements from cdi config status
func GetDefaultPodResourceRequirements(client client.Client) (*v1.ResourceRequirements, error) {
	cdiconfig := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiconfig); err != nil {
		klog.Errorf("Unable to find CDI configuration, %v\n", err)
		return nil, err
	}

	return cdiconfig.Status.DefaultPodResourceRequirements, nil
}

// this is being called for pods using PV with block volume mode
func addVolumeDevices() []v1.VolumeDevice {
	volumeDevices := []v1.VolumeDevice{
		{
			Name:       DataVolName,
			DevicePath: common.WriteBlockPath,
		},
	}
	return volumeDevices
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

// DecodePublicKey turns a bunch of bytes into a public key
func DecodePublicKey(keyBytes []byte) (*rsa.PublicKey, error) {
	keys, err := cert.ParsePublicKeysPEM(keyBytes)
	if err != nil {
		return nil, err
	}

	if len(keys) != 1 {
		return nil, errors.New("unexected number of pulic keys")
	}

	key, ok := keys[0].(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("PEM does not contain RSA key")
	}

	return key, nil
}

// MakePVCOwnerReference makes owner reference from a PVC
func MakePVCOwnerReference(pvc *v1.PersistentVolumeClaim) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true
	return metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "PersistentVolumeClaim",
		Name:               pvc.Name,
		UID:                pvc.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

// MakePodOwnerReference makes owner reference from a Pod
func MakePodOwnerReference(pod *v1.Pod) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true
	return metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               pod.Name,
		UID:                pod.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

func deletePod(req podDeleteRequest) error {
	pod, err := req.podLister.Pods(req.namespace).Get(req.podName)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err == nil && pod.DeletionTimestamp == nil {
		err = req.k8sClient.CoreV1().Pods(req.namespace).Delete(req.podName, &metav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}
	if err != nil {
		klog.V(1).Infof("error encountered deleting pod (%s): %s", req.podName, err.Error())
	}
	return errors.Wrapf(err, "error deleting pod %s/%s", req.namespace, req.podName)
}

func createImportEnvVar(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) (*importPodEnvVar, error) {
	podEnvVar := &importPodEnvVar{}
	podEnvVar.source = getSource(pvc)
	podEnvVar.contentType = getContentType(pvc)

	var err error
	if podEnvVar.source != SourceNone {
		podEnvVar.ep, err = getEndpoint(pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.secretName, err = getSecretName(client, pvc)
		if err != nil {
			return nil, err
		}
		if podEnvVar.secretName == "" {
			klog.V(2).Infof("no secret will be supplied to endpoint %q\n", podEnvVar.ep)
		}
		podEnvVar.certConfigMap, err = getCertConfigMap(client, pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.insecureTLS, err = isInsecureTLS(client, pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.diskID = getDiskID(pvc)
	}
	//get the requested image size.
	podEnvVar.imageSize, err = getRequestedImageSize(pvc)
	if err != nil {
		return nil, err
	}
	return podEnvVar, nil
}

func getCertConfigMap(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) (string, error) {
	value, ok := pvc.Annotations[AnnCertConfigMap]
	if !ok || value == "" {
		return "", nil
	}

	_, err := client.CoreV1().ConfigMaps(pvc.Namespace).Get(value, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warningf("Configmap %s does not exist, pod will not start until it does", value)
			return value, nil
		}

		return "", err
	}

	return value, nil
}

//IsOpenshift checks if we are on OpenShift platform
func IsOpenshift(client kubernetes.Interface) bool {
	//OpenShift 3.X check
	result := client.Discovery().RESTClient().Get().AbsPath("/oapi/v1").Do()
	var statusCode int
	result.StatusCode(&statusCode)

	if result.Error() == nil {
		// It is OpenShift
		if statusCode == http.StatusOK {
			return true
		}
	} else {
		// Got 404 so this is not Openshift 3.X, let's check OpenShift 4
		result = client.Discovery().RESTClient().Get().AbsPath("/apis/route.openshift.io").Do()
		var statusCode int
		result.StatusCode(&statusCode)

		if result.Error() == nil {
			// It is OpenShift
			if statusCode == http.StatusOK {
				return true
			}
		}
	}

	return false
}

func isInsecureTLS(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim) (bool, error) {
	var configMapName string

	value, ok := pvc.Annotations[AnnEndpoint]
	if !ok || value == "" {
		return false, nil
	}

	url, err := url.Parse(value)
	if err != nil {
		return false, err
	}

	switch url.Scheme {
	case "docker":
		configMapName = common.InsecureRegistryConfigMap
	default:
		return false, nil
	}

	klog.V(3).Infof("Checking configmap %s for host %s", configMapName, url.Host)

	cm, err := client.CoreV1().ConfigMaps(util.GetNamespace()).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warningf("Configmap %s does not exist", configMapName)
			return false, nil
		}

		return false, err
	}

	for key, value := range cm.Data {
		klog.V(3).Infof("Checking %q against %q: %q", url.Host, key, value)

		if value == url.Host {
			return true, nil
		}
	}

	return false, nil
}

// IsCsiCrdsDeployed checks whether the CSI snapshotter CRD are deployed
func IsCsiCrdsDeployed(c extclientset.Interface) bool {
	vsClass := crdv1alpha1.VolumeSnapshotClassResourcePlural + "." + crdv1alpha1.GroupName
	vsContent := crdv1alpha1.VolumeSnapshotContentResourcePlural + "." + crdv1alpha1.GroupName
	vs := crdv1alpha1.VolumeSnapshotResourcePlural + "." + crdv1alpha1.GroupName

	return isCrdDeployed(c, vsClass) &&
		isCrdDeployed(c, vsContent) &&
		isCrdDeployed(c, vs)
}

func isCrdDeployed(c extclientset.Interface, name string) bool {
	obj, err := c.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false
		}
		return false
	}
	return obj != nil
}

func isPodReady(pod *v1.Pod) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}

	numReady := 0
	for _, s := range pod.Status.ContainerStatuses {
		if s.Ready {
			numReady++
		}
	}

	return numReady == len(pod.Status.ContainerStatuses)
}

func podPhaseFromPVC(pvc *v1.PersistentVolumeClaim) v1.PodPhase {
	phase := pvc.ObjectMeta.Annotations[AnnPodPhase]
	return v1.PodPhase(phase)
}

func podSucceededFromPVC(pvc *v1.PersistentVolumeClaim) bool {
	return (podPhaseFromPVC(pvc) == v1.PodSucceeded)
}
