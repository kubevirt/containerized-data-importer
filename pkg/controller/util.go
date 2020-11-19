package controller

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
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
	// AnnPodRestarts is a PVC annotation that tells how many times a related pod was restarted
	AnnPodRestarts = AnnAPIGroup + "/storage.pod.restarts"
	// AnnPopulatedFor is a PVC annotation telling the datavolume controller that the PVC is already populated
	AnnPopulatedFor = AnnAPIGroup + "/storage.populatedFor"
	// AnnPrePopulated is a PVC annotation telling the datavolume controller that the PVC is already populated
	AnnPrePopulated = AnnAPIGroup + "/storage.prePopulated"

	// AnnRunningCondition provides a const for the running condition
	AnnRunningCondition = AnnAPIGroup + "/storage.condition.running"
	// AnnRunningConditionMessage provides a const for the running condition
	AnnRunningConditionMessage = AnnAPIGroup + "/storage.condition.running.message"
	// AnnRunningConditionReason provides a const for the running condition
	AnnRunningConditionReason = AnnAPIGroup + "/storage.condition.running.reason"

	// AnnBoundCondition provides a const for the running condition
	AnnBoundCondition = AnnAPIGroup + "/storage.condition.bound"
	// AnnBoundConditionMessage provides a const for the running condition
	AnnBoundConditionMessage = AnnAPIGroup + "/storage.condition.bound.message"
	// AnnBoundConditionReason provides a const for the running condition
	AnnBoundConditionReason = AnnAPIGroup + "/storage.condition.bound.reason"

	// AnnSourceRunningCondition provides a const for the running condition
	AnnSourceRunningCondition = AnnAPIGroup + "/storage.condition.source.running"
	// AnnSourceRunningConditionMessage provides a const for the running condition
	AnnSourceRunningConditionMessage = AnnAPIGroup + "/storage.condition.source.running.message"
	// AnnSourceRunningConditionReason provides a const for the running condition
	AnnSourceRunningConditionReason = AnnAPIGroup + "/storage.condition.source.running.reason"

	// PodRunningReason is const that defines the pod was started as a reason
	podRunningReason = "Pod is running"
)

func checkPVC(pvc *v1.PersistentVolumeClaim, annotation string, log logr.Logger) bool {
	// check if we have proper annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, annotation) {
		log.V(1).Info("PVC annotation not found, skipping pvc", "annotation", annotation)
		return false
	}

	return true
}

// - when the SkipWFFCVolumesEnabled is true, the CDI controller will only handle BOUND the PVC
// - when the SkipWFFCVolumesEnabled is false, the CDI controller will can handle it - it will create worker pods for the PVC (this will bind it)
func shouldHandlePvc(pvc *v1.PersistentVolumeClaim, honorWaitForFirstConsumerEnabled bool, log logr.Logger) bool {
	if honorWaitForFirstConsumerEnabled {
		return isBound(pvc, log)
	}
	return true
}

func isBound(pvc *v1.PersistentVolumeClaim, log logr.Logger) bool {
	if pvc.Status.Phase != v1.ClaimBound {
		log.V(1).Info("PVC not bound, skipping pvc", "Phase", pvc.Status.Phase)
		return false
	}

	return true
}

func isPvcUsedByAnyPod(c client.Client, pvc *v1.PersistentVolumeClaim, log logr.Logger) (bool, error) {
	pods := &v1.PodList{}
	if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: pvc.Namespace}); err != nil {
		return false, errors.Wrap(err, "error listing pods")
	}

	for _, pod := range pods.Items {
		if isPvcUsedByPod(pod, pvc.Name) {
			return true, nil
		}
	}

	return false, nil
}

func isPvcUsedByPod(pod v1.Pod, pvcName string) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.VolumeSource.PersistentVolumeClaim != nil &&
			volume.PersistentVolumeClaim.ClaimName == pvcName {
			return true
		}
	}
	return false
}

func getRequestedImageSize(pvc *v1.PersistentVolumeClaim) (string, error) {
	pvcSize, found := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	if !found {
		return "", errors.Errorf("storage request is missing in pvc \"%s/%s\"", pvc.Namespace, pvc.Name)
	}
	return pvcSize.String(), nil
}

// returns the volumeMode which determines if the PVC is block PVC or not.
func getVolumeMode(pvc *v1.PersistentVolumeClaim) v1.PersistentVolumeMode {
	if pvc.Spec.VolumeMode != nil {
		return *pvc.Spec.VolumeMode
	}
	return v1.PersistentVolumeFilesystem
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
		"app": "containerized-data-importer",
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
func CreateScratchPersistentVolumeClaim(client client.Client, pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name, storageClassName string) (*v1.PersistentVolumeClaim, error) {
	scratchPvcSpec := newScratchPersistentVolumeClaimSpec(pvc, pod, name, storageClassName)
	if err := client.Create(context.TODO(), scratchPvcSpec); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nil, errors.Wrap(err, "scratch PVC API create errored")
		}
	}
	scratchPvc := &v1.PersistentVolumeClaim{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: scratchPvcSpec.Name, Namespace: pvc.Namespace}, scratchPvc); err != nil {
		klog.Errorf("Unable to get scratch space pvc, %v\n", err)
		return nil, err
	}
	klog.V(3).Infof("scratch PVC \"%s/%s\" created\n", scratchPvc.Namespace, scratchPvc.Name)
	return scratchPvc, nil
}

// GetStorageClassByName looks up the storage class based on the name. If no storage class is found returns nil
func GetStorageClassByName(client client.Client, name *string) (*storagev1.StorageClass, error) {
	// look up storage class by name
	if name == nil {
		storageClasses := &storagev1.StorageClassList{}
		if err := client.List(context.TODO(), storageClasses); err != nil {
			klog.V(3).Info("Unable to retrieve available storage classes")
			return nil, errors.New("unable to retrieve storage classes")
		}
		for _, storageClass := range storageClasses.Items {
			if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				return &storageClass, nil
			}
		}
	} else {
		storageClass := &storagev1.StorageClass{}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: *name}, storageClass); err != nil {
			klog.V(3).Info("Unable to retrieve storage class", "storage class name", *name)
			return nil, errors.New("unable to retrieve storage class")
		}
		return storageClass, nil
	}
	// No storage class found, just return nil for storage class and let caller deal with it.
	return nil, nil
}

// GetFilesystemOverhead determines the filesystem overhead defined in CDIConfig for this PVC's volumeMode and storageClass.
func GetFilesystemOverhead(client client.Client, pvc *v1.PersistentVolumeClaim) (cdiv1.Percent, error) {
	klog.V(1).Info("GetFilesystemOverhead with PVC", pvc)
	if getVolumeMode(pvc) != v1.PersistentVolumeFilesystem {
		return "0", nil
	}

	cdiConfig := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			klog.V(1).Info("CDIConfig does not exist, pod will not start until it does")
			return "0", nil
		}

		return "0", err
	}

	targetStorageClass, err := GetStorageClassByName(client, pvc.Spec.StorageClassName)
	if err != nil {
		klog.V(1).Info("Storage class", pvc.Spec.StorageClassName, "not found, trying default storage class")
		targetStorageClass, err = GetStorageClassByName(client, nil)
		if err != nil {
			klog.V(1).Info("No default storage class found, continuing with global overhead")
			return cdiConfig.Status.FilesystemOverhead.Global, nil
		}
	}

	klog.V(1).Info("target storage class for overhead", targetStorageClass)

	if cdiConfig.Status.FilesystemOverhead == nil {
		klog.Errorf("CDIConfig filesystemOverhead used before config controller ran reconcile. Hopefully this only happens during unit testing.")
		return "0", nil
	}

	if targetStorageClass == nil {
		klog.V(1).Info("Storage class", pvc.Spec.StorageClassName, "not found, continuing with global overhead")
		return cdiConfig.Status.FilesystemOverhead.Global, nil
	}

	perStorageConfig := cdiConfig.Status.FilesystemOverhead.StorageClass

	storageClassOverhead, found := perStorageConfig[targetStorageClass.GetName()]
	if found {
		return storageClassOverhead, nil
	}

	return cdiConfig.Status.FilesystemOverhead.Global, nil
}

// GetScratchPvcStorageClass tries to determine which storage class to use for use with a scratch persistent
// volume claim. The order of preference is the following:
// 1. Defined value in CDI Config field scratchSpaceStorageClass.
// 2. If 1 is not available, use the storage class name of the original pvc that will own the scratch pvc.
// 3. If none of those are available, return blank.
func GetScratchPvcStorageClass(client client.Client, pvc *v1.PersistentVolumeClaim) string {
	config := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return ""
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

// IsCsiCrdsDeployed checks whether the CSI snapshotter CRD are deployed
func IsCsiCrdsDeployed(c extclientset.Interface) bool {
	version := "v1beta1"
	vsClass := "volumesnapshotclasses." + snapshotv1.GroupName
	vsContent := "volumesnapshotcontents." + snapshotv1.GroupName
	vs := "volumesnapshots." + snapshotv1.GroupName

	return isCrdDeployed(c, vsClass, version) &&
		isCrdDeployed(c, vsContent, version) &&
		isCrdDeployed(c, vs, version)
}

func isCrdDeployed(c extclientset.Interface, name, version string) bool {
	obj, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false
	}

	for _, v := range obj.Spec.Versions {
		if v.Name == version && v.Served {
			return true
		}
	}

	return false
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

func setConditionFromPodWithPrefix(anno map[string]string, prefix string, pod *v1.Pod) {
	if pod.Status.ContainerStatuses != nil {
		if pod.Status.ContainerStatuses[0].State.Running != nil {
			anno[prefix] = "true"
			anno[prefix+".message"] = ""
			anno[prefix+".reason"] = podRunningReason
		} else {
			anno[AnnRunningCondition] = "false"
			if pod.Status.ContainerStatuses[0].State.Waiting != nil {
				anno[prefix+".message"] = pod.Status.ContainerStatuses[0].State.Waiting.Message
				anno[prefix+".reason"] = pod.Status.ContainerStatuses[0].State.Waiting.Reason
			} else if pod.Status.ContainerStatuses[0].State.Terminated != nil {
				anno[prefix+".message"] = pod.Status.ContainerStatuses[0].State.Terminated.Message
				anno[prefix+".reason"] = pod.Status.ContainerStatuses[0].State.Terminated.Reason
			}
		}
	}
}

func setBoundConditionFromPVC(anno map[string]string, prefix string, pvc *v1.PersistentVolumeClaim) {
	switch pvc.Status.Phase {
	case v1.ClaimBound:
		anno[prefix] = "true"
		anno[prefix+".message"] = ""
		anno[prefix+".reason"] = ""
	case v1.ClaimPending:
		anno[prefix] = "false"
		anno[prefix+".message"] = "Claim Pending"
		anno[prefix+".reason"] = "Claim Pending"
	case v1.ClaimLost:
		anno[prefix] = "false"
		anno[prefix+".message"] = claimLost
		anno[prefix+".reason"] = claimLost
	default:
		anno[prefix] = "false"
		anno[prefix+".message"] = "Unknown"
		anno[prefix+".reason"] = "Unknown"
	}
}

func getScratchNameFromPod(pod *v1.Pod) (string, bool) {
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == ScratchVolName {
			return vol.PersistentVolumeClaim.ClaimName, true
		}
	}

	return "", false
}

func createScratchNameFromPvc(pvc *v1.PersistentVolumeClaim) string {
	return naming.GetResourceName(pvc.Name, common.ScratchNameSuffix)
}

func getPodsUsingPVCs(c client.Client, namespace string, names sets.String, allowReadOnly bool) ([]v1.Pod, error) {
	pl := &v1.PodList{}
	// hopefully using cached client here
	err := c.List(context.TODO(), pl, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	var pods []v1.Pod
	for _, pod := range pl.Items {
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.VolumeSource.PersistentVolumeClaim != nil &&
				names.Has(volume.PersistentVolumeClaim.ClaimName) &&
				(!allowReadOnly || !volume.PersistentVolumeClaim.ReadOnly) {
				pods = append(pods, pod)
				break
			}
		}
	}

	return pods, nil
}

func filterCloneSourcePods(input []v1.Pod) []v1.Pod {
	var output []v1.Pod

	for _, pod := range input {
		if pod.Labels[common.CDIComponentLabel] != common.ClonerSourcePodName {
			output = append(output, pod)
		}
	}

	return output
}

// GetWorkloadNodePlacement extracts the workload-specific nodeplacement values from the CDI CR
func GetWorkloadNodePlacement(c client.Client) (*sdkapi.NodePlacement, error) {
	cr, err := GetActiveCDI(c)
	if err != nil {
		return nil, err
	}

	if cr == nil {
		return nil, fmt.Errorf("no active CDI")
	}

	return &cr.Spec.Workloads, nil
}

// GetActiveCDI returns the active CDI CR
func GetActiveCDI(c client.Client) (*cdiv1.CDI, error) {
	crList := &cdiv1.CDIList{}
	if err := c.List(context.TODO(), crList, &client.ListOptions{}); err != nil {
		return nil, err
	}

	var activeResources []cdiv1.CDI
	for _, cr := range crList.Items {
		if cr.Status.Phase != sdkapi.PhaseError {
			activeResources = append(activeResources, cr)
		}
	}

	if len(activeResources) == 0 {
		return nil, nil
	}

	if len(activeResources) > 1 {
		return nil, fmt.Errorf("Number of active CDI CRs > 1")
	}

	return &activeResources[0], nil
}

// IsPopulated returns if the passed in PVC has been populated according to the rules outlined in pkg/apis/core/<version>/utils.go
func IsPopulated(pvc *v1.PersistentVolumeClaim, c client.Client) (bool, error) {
	return cdiv1.IsPopulated(pvc, func(name, namespace string) (*cdiv1.DataVolume, error) {
		dv := &cdiv1.DataVolume{}
		err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, dv)
		return dv, err
	})
}
