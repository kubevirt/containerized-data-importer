package controller

import (
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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/keys"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
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
)

type podDeleteRequest struct {
	namespace string
	podName   string
	podLister corelisters.PodLister
	k8sClient kubernetes.Interface
}

var createClientKeyAndCertFunc = createClientKeyAndCert

func createClientKeyAndCert(ca *triple.KeyPair, commonName string, organizations []string) ([]byte, []byte, error) {
	clientKeyPair, err := triple.NewClientKeyPair(ca, commonName, organizations)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error creating client key pair")
	}

	clientKeyBytes := cert.EncodePrivateKeyPEM(clientKeyPair.Key)
	clientCertBytes := cert.EncodeCertPEM(clientKeyPair.Cert)

	return clientKeyBytes, clientCertBytes, nil
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
		SourceRegistry:
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
	klog.V(3).Infof("getEndpointSecret: retrieving Secret \"%s/%s\"\n", ns, name)
	_, err := client.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		klog.V(1).Infof("secret %q defined in pvc \"%s/%s\" is missing. Importer pod will run once this secret is created\n", name, ns, pvc.Name)
		return name, nil
	}
	if err != nil {
		return "", errors.Wrapf(err, "error getting secret %q defined in pvc \"%s/%s\"", name, ns, pvc.Name)
	}
	klog.V(1).Infof("retrieved secret %q defined in pvc \"%s/%s\"\n", name, ns, pvc.Name)
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

	pvcDef := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Labels:    labels,
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
// 1. Defined value in CDI config map.
// 2. If 1 is not available use the 'default' storage class.
// 3. If 2 is not available use the storage class name of the original pvc that will own the scratch pvc.
// 4. If none of those are available, return blank.
func GetScratchPvcStorageClass(client kubernetes.Interface, cdiclient clientset.Interface, pvc *v1.PersistentVolumeClaim) string {
	config, err := cdiclient.CdiV1alpha1().CDIConfigs().Get(common.ConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to find CDI configuration, %v\n", err)
	}
	storageClassName := config.Status.ScratchSpaceStorageClass
	if storageClassName == "" {
		// Unable to determine default storage class, attempt to read the storage class from the pvc.
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
func GetDefaultPodResourceRequirements(cdiclient clientset.Interface) (*v1.ResourceRequirements, error) {
	config, err := cdiclient.CdiV1alpha1().CDIConfigs().Get(common.ConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Unable to find CDI configuration, %v\n", err)
		return nil, err
	}

	return config.Status.DefaultPodResourceRequirements, nil
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

// returns the CloneRequest string which contains the pvc name (and namespace) from which we want to clone the image.
func getCloneRequestSourcePVC(pvc *v1.PersistentVolumeClaim, pvcLister corelisters.PersistentVolumeClaimLister) (*v1.PersistentVolumeClaim, error) {
	exists, namespace, name := ParseCloneRequestAnnotation(pvc)
	if !exists {
		return nil, errors.New("error parsing clone request annotation")
	}
	pvc, err := pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		return nil, errors.Wrap(err, "error getting clone source PVC")
	}
	return pvc, nil
}

// ParseCloneRequestAnnotation parses the clone request annotation
func ParseCloneRequestAnnotation(pvc *v1.PersistentVolumeClaim) (exists bool, namespace, name string) {
	var ann string
	ann, exists = pvc.Annotations[AnnCloneRequest]
	if !exists {
		return
	}

	sp := strings.Split(ann, "/")
	if len(sp) != 2 {
		klog.V(1).Infof("Bad CloneRequest Annotation %s", ann)
		exists = false
		return
	}

	namespace, name = sp[0], sp[1]
	return
}

// ValidateCanCloneSourceAndTargetSpec validates the specs passed in are compatible for cloning.
func ValidateCanCloneSourceAndTargetSpec(sourceSpec, targetSpec *v1.PersistentVolumeClaimSpec) error {
	sourceRequest := sourceSpec.Resources.Requests[v1.ResourceStorage]
	targetRequest := targetSpec.Resources.Requests[v1.ResourceStorage]
	// Verify that the target PVC size is equal or larger than the source.
	if sourceRequest.Value() > targetRequest.Value() {
		return errors.New("target resources requests storage size is smaller than the source")
	}
	// Verify that the source and target volume modes are the same.
	sourceVolumeMode := v1.PersistentVolumeFilesystem
	if sourceSpec.VolumeMode != nil && *sourceSpec.VolumeMode == v1.PersistentVolumeBlock {
		sourceVolumeMode = v1.PersistentVolumeBlock
	}
	targetVolumeMode := v1.PersistentVolumeFilesystem
	if targetSpec.VolumeMode != nil && *targetSpec.VolumeMode == v1.PersistentVolumeBlock {
		targetVolumeMode = v1.PersistentVolumeBlock
	}
	if sourceVolumeMode != targetVolumeMode {
		return fmt.Errorf("source volumeMode (%s) and target volumeMode (%s) do not match",
			sourceVolumeMode, targetVolumeMode)
	}
	// Can clone.
	return nil
}

func validateCloneToken(validator token.Validator, source, target *v1.PersistentVolumeClaim) error {
	tok, ok := target.Annotations[AnnCloneToken]
	if !ok {
		return errors.New("clone token missing")
	}

	tokenData, err := validator.Validate(tok)
	if err != nil {
		return errors.Wrap(err, "error verifying token")
	}

	if tokenData.Operation != token.OperationClone ||
		tokenData.Name != source.Name ||
		tokenData.Namespace != source.Namespace ||
		tokenData.Resource.Resource != "persistentvolumeclaims" ||
		tokenData.Params["targetNamespace"] != target.Namespace ||
		tokenData.Params["targetName"] != target.Name {
		return errors.New("invalid token")
	}

	return nil
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

// CreateCloneSourcePod creates our cloning src pod which will be used for out of band cloning to read the contents of the src PVC
func CreateCloneSourcePod(client kubernetes.Interface, cdiClient clientset.Interface, image, pullPolicy, clientName string, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	exists, sourcePvcNamespace, sourcePvcName := ParseCloneRequestAnnotation(pvc)
	if !exists {
		return nil, errors.Errorf("bad CloneRequest Annotation")
	}

	ownerKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cache key")
	}

	serverCACertBytes, err := keys.GetKeyPairAndCertBytes(client, util.GetNamespace(), uploadServerCASecret)
	if err != nil {
		return nil, errors.Wrap(err, "error getting uploadserver server CA cert")
	}

	if serverCACertBytes == nil {
		return nil, errors.Errorf("secret %s does not exist", uploadServerCASecret)
	}

	clientCAKeyPair, err := keys.GetKeyPairAndCert(client, util.GetNamespace(), uploadServerClientCASecret)
	if err != nil {
		return nil, errors.Wrap(err, "error getting uploadserver client CA cert")
	}

	if clientCAKeyPair == nil {
		return nil, errors.Errorf("secret %s does not exist", uploadServerClientCASecret)
	}

	clientKeyBytes, clientCertBytes, err := createClientKeyAndCertFunc(&clientCAKeyPair.KeyPair, clientName, []string{})
	if err != nil {
		return nil, err
	}

	podResourceRequirements, err := GetDefaultPodResourceRequirements(cdiClient)
	if err != nil {
		return nil, err
	}

	pod := MakeCloneSourcePodSpec(image, pullPolicy, sourcePvcName, ownerKey,
		clientKeyBytes, clientCertBytes, serverCACertBytes.Cert, pvc, podResourceRequirements)

	pod, err = client.CoreV1().Pods(sourcePvcNamespace).Create(pod)
	if err != nil {
		return nil, errors.Wrap(err, "source pod API create errored")
	}

	klog.V(1).Infof("cloning source pod \"%s/%s\" (image: %q) created\n", pod.Namespace, pod.Name, image)

	return pod, nil
}

// MakeCloneSourcePodSpec creates and returns the clone source pod spec based on the target pvc.
func MakeCloneSourcePodSpec(image, pullPolicy, sourcePvcName, ownerRefAnno string,
	clientKey, clientCert, serverCACert []byte, pvc *v1.PersistentVolumeClaim, resourceRequirements *v1.ResourceRequirements) *v1.Pod {

	var ownerID string
	podName := fmt.Sprintf("%s-%s-", common.ClonerSourcePodName, sourcePvcName)
	id := string(pvc.GetUID())
	url := GetUploadServerURL(pvc.Namespace, pvc.Name)
	pvcOwner := metav1.GetControllerOf(pvc)
	if pvcOwner != nil && pvcOwner.Kind == "DataVolume" {
		ownerID = string(pvcOwner.UID)
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
				AnnOwnerRef:  ownerRefAnno,
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue, //filtered by the podInformer
				common.CDIComponentLabel: common.ClonerSourcePodName,
				// this label is used when searching for a pvc's cloner source pod.
				CloneUniqueID:          id + "-source-pod",
				common.PrometheusLabel: "",
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []v1.Container{
				{
					Name:            common.ClonerSourcePodName,
					Image:           image,
					ImagePullPolicy: v1.PullPolicy(pullPolicy),
					Env: []v1.EnvVar{
						/*
						 Easier to just stick key/certs in env vars directly no.
						 Maybe revisit when we fix the "naming things" problem.
						*/
						{
							Name:  "CLIENT_KEY",
							Value: string(clientKey),
						},
						{
							Name:  "CLIENT_CERT",
							Value: string(clientCert),
						},
						{
							Name:  "SERVER_CA_CERT",
							Value: string(serverCACert),
						},
						{
							Name:  "UPLOAD_URL",
							Value: url,
						},
						{
							Name:  common.OwnerUID,
							Value: ownerID,
						},
					},
					Ports: []v1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      v1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes: []v1.Volume{
				{
					Name: DataVolName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	var volumeMode v1.PersistentVolumeMode
	var addVars []v1.EnvVar

	if pvc.Spec.VolumeMode != nil {
		volumeMode = *pvc.Spec.VolumeMode
	} else {
		volumeMode = v1.PersistentVolumeFilesystem
	}

	if volumeMode == v1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		addVars = []v1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "block",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.WriteBlockPath,
			},
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
		addVars = []v1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "filesystem",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.ClonerMountPath,
			},
		}
	}

	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, addVars...)

	return pod
}

// UploadPodArgs are the parameters required to create an upload pod
type UploadPodArgs struct {
	Client         kubernetes.Interface
	CdiClient      clientset.Interface
	Image          string
	Verbose        string
	PullPolicy     string
	Name           string
	PVC            *v1.PersistentVolumeClaim
	ScratchPVCName string
	ClientName     string
}

// CreateUploadPod creates upload service pod manifest and sends to server
func CreateUploadPod(args UploadPodArgs) (*v1.Pod, error) {
	ns := args.PVC.Namespace
	commonName := args.Name + "." + ns
	secretName := args.Name + "-server-tls"

	podResourceRequirements, err := GetDefaultPodResourceRequirements(args.CdiClient)
	if err != nil {
		return nil, err
	}

	pod := makeUploadPodSpec(args.Image, args.Verbose, args.PullPolicy, args.Name,
		args.PVC, args.ScratchPVCName, secretName, args.ClientName, podResourceRequirements)

	pod, err = args.Client.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			pod, err = args.Client.CoreV1().Pods(ns).Get(args.Name, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "upload pod should exist but couldn't retrieve it")
			}
		} else {
			return nil, errors.Wrap(err, "upload pod API create errored")
		}
	}

	serverCAKeyPair, err := keys.GetKeyPairAndCert(args.Client, util.GetNamespace(), uploadServerCASecret)
	if err != nil {
		return nil, errors.Wrap(err, "error getting uploadserver server CA cert")
	}

	if serverCAKeyPair == nil {
		return nil, errors.Errorf("secret %s does not exist", uploadServerCASecret)
	}

	clientCAKeyPair, err := keys.GetKeyPairAndCert(args.Client, util.GetNamespace(), uploadServerClientCASecret)
	if err != nil {
		return nil, errors.Wrap(err, "error getting uploadserver client CA cert")
	}

	if clientCAKeyPair == nil {
		return nil, errors.Errorf("secret %s does not exist", uploadServerClientCASecret)
	}

	podOwner := MakePodOwnerReference(pod)

	_, err = keys.GetOrCreateServerKeyPairAndCert(args.Client, ns, secretName,
		&serverCAKeyPair.KeyPair, clientCAKeyPair.KeyPair.Cert, commonName, args.Name, &podOwner)
	if err != nil {
		// try to clean up
		args.Client.CoreV1().Pods(ns).Delete(pod.Name, &metav1.DeleteOptions{})

		return nil, errors.Wrap(err, "error creating server key pair")
	}

	klog.V(1).Infof("upload pod \"%s/%s\" (image: %q) created\n", pod.Namespace, pod.Name, args.Image)
	return pod, nil
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

func makeUploadPodSpec(image, verbose, pullPolicy, name string,
	pvc *v1.PersistentVolumeClaim, scratchName, secretName, clientName string, resourceRequirements *v1.ResourceRequirements) *v1.Pod {
	requestImageSize, _ := getRequestedImageSize(pvc)
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:              common.CDILabelValue,
				common.CDIComponentLabel:        common.UploadServerCDILabel,
				common.UploadServerServiceLabel: name,
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePVCOwnerReference(pvc),
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []v1.Container{
				{
					Name:            common.UploadServerPodname,
					Image:           image,
					ImagePullPolicy: v1.PullPolicy(pullPolicy),
					Env: []v1.EnvVar{
						{
							Name: "TLS_KEY",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSKeyFile,
								},
							},
						},
						{
							Name: "TLS_CERT",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSCertFile,
								},
							},
						},
						{
							Name: "CLIENT_CERT",
							ValueFrom: &v1.EnvVarSource{
								SecretKeyRef: &v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: secretName,
									},
									Key: keys.KeyStoreTLSCAFile,
								},
							},
						},
						{
							Name:  common.UploadImageSize,
							Value: requestImageSize,
						},
						{
							Name:  "CLIENT_NAME",
							Value: clientName,
						},
					},
					Args: []string{"-v=" + verbose},
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 8080,
								},
							},
						},
						InitialDelaySeconds: 2,
						PeriodSeconds:       5,
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
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

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	if getVolumeMode(pvc) == v1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevicesForUpload()
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "DESTINATION",
			Value: common.WriteBlockPath,
		})
	} else {
		pod.Spec.Containers[0].VolumeMounts = addVolumeMountsForUpload()
	}

	if scratchName != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: ScratchVolName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: scratchName,
					ReadOnly:  false,
				},
			},
		})

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}

	return pod
}

func addVolumeDevicesForUpload() []v1.VolumeDevice {
	volumeDevices := []v1.VolumeDevice{
		{
			Name:       DataVolName,
			DevicePath: common.WriteBlockPath,
		},
	}
	return volumeDevices
}

func addVolumeMountsForUpload() []v1.VolumeMount {
	volumeMounts := []v1.VolumeMount{
		{
			Name:      DataVolName,
			MountPath: common.UploadServerDataDir,
		},
	}
	return volumeMounts
}

// CreateUploadService creates upload service service manifest and sends to server
func CreateUploadService(client kubernetes.Interface, name string, pvc *v1.PersistentVolumeClaim) (*v1.Service, error) {
	ns := pvc.Namespace
	service := MakeUploadServiceSpec(name, pvc)

	service, err := client.CoreV1().Services(ns).Create(service)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			service, err = client.CoreV1().Services(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "upload service should exist but couldn't retrieve it")
			}
		} else {
			return nil, errors.Wrap(err, "upload service API create errored")
		}
	}
	klog.V(1).Infof("upload service \"%s/%s\" created\n", service.Namespace, service.Name)
	return service, nil
}

// MakeUploadServiceSpec creates upload service service manifest
func MakeUploadServiceSpec(name string, pvc *v1.PersistentVolumeClaim) *v1.Service {
	blockOwnerDeletion := true
	isController := true
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.UploadServerCDILabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8443,
					},
				},
			},
			Selector: map[string]string{
				common.UploadServerServiceLabel: name,
			},
		},
	}
	return service
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

func addFinalizer(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, name string) (*v1.PersistentVolumeClaim, error) {
	if hasFinalizer(pvc, name) {
		return pvc, nil
	}

	cpy := pvc.DeepCopy()
	cpy.Finalizers = append(cpy.Finalizers, name)
	pvc, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(cpy)
	if err != nil {
		return nil, errors.Wrap(err, "error updating PVC")
	}

	return pvc, nil
}

func removeFinalizer(client kubernetes.Interface, pvc *v1.PersistentVolumeClaim, name string) (*v1.PersistentVolumeClaim, error) {
	if !hasFinalizer(pvc, name) {
		return pvc, nil
	}

	var finalizers []string
	for _, f := range pvc.Finalizers {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}

	cpy := pvc.DeepCopy()
	cpy.Finalizers = finalizers
	pvc, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(cpy)
	if err != nil {
		return nil, errors.Wrap(err, "error updating PVC")
	}

	return pvc, nil
}

func hasFinalizer(object metav1.Object, value string) bool {
	for _, f := range object.GetFinalizers() {
		if f == value {
			return true
		}
	}
	return false
}

func podPhaseFromPVC(pvc *v1.PersistentVolumeClaim) v1.PodPhase {
	phase := pvc.ObjectMeta.Annotations[AnnPodPhase]
	return v1.PodPhase(phase)
}

func podSucceededFromPVC(pvc *v1.PersistentVolumeClaim) bool {
	return (podPhaseFromPVC(pvc) == v1.PodSucceeded)
}
