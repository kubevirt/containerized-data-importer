/*
Copyright 2022 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"strings"
	"sync"
	"time"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiv1utils "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1/utils"
	"kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/scheme"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	// DataVolName provides a const to use for creating volumes in pod specs
	DataVolName = "cdi-data-vol"

	// ScratchVolName provides a const to use for creating scratch pvc volumes in pod specs
	ScratchVolName = "cdi-scratch-vol"

	// AnnAPIGroup is the APIGroup for CDI
	AnnAPIGroup = "cdi.kubevirt.io"
	// AnnCreatedBy is a pod annotation indicating if the pod was created by the PVC
	AnnCreatedBy = AnnAPIGroup + "/storage.createdByController"
	// AnnPodPhase is a PVC annotation indicating the related pod progress (phase)
	AnnPodPhase = AnnAPIGroup + "/storage.pod.phase"
	// AnnPodReady tells whether the pod is ready
	AnnPodReady = AnnAPIGroup + "/storage.pod.ready"
	// AnnPodRestarts is a PVC annotation that tells how many times a related pod was restarted
	AnnPodRestarts = AnnAPIGroup + "/storage.pod.restarts"
	// AnnPopulatedFor is a PVC annotation telling the datavolume controller that the PVC is already populated
	AnnPopulatedFor = AnnAPIGroup + "/storage.populatedFor"
	// AnnPrePopulated is a PVC annotation telling the datavolume controller that the PVC is already populated
	AnnPrePopulated = AnnAPIGroup + "/storage.prePopulated"
	// AnnPriorityClassName is PVC annotation to indicate the priority class name for importer, cloner and uploader pod
	AnnPriorityClassName = AnnAPIGroup + "/storage.pod.priorityclassname"

	// AnnDeleteAfterCompletion is PVC annotation for deleting DV after completion
	AnnDeleteAfterCompletion = AnnAPIGroup + "/storage.deleteAfterCompletion"
	// AnnPodRetainAfterCompletion is PVC annotation for retaining transfer pods after completion
	AnnPodRetainAfterCompletion = AnnAPIGroup + "/storage.pod.retainAfterCompletion"

	// AnnPreviousCheckpoint provides a const to indicate the previous snapshot for a multistage import
	AnnPreviousCheckpoint = AnnAPIGroup + "/storage.checkpoint.previous"
	// AnnCurrentCheckpoint provides a const to indicate the current snapshot for a multistage import
	AnnCurrentCheckpoint = AnnAPIGroup + "/storage.checkpoint.current"
	// AnnFinalCheckpoint provides a const to indicate whether the current checkpoint is the last one
	AnnFinalCheckpoint = AnnAPIGroup + "/storage.checkpoint.final"
	// AnnCheckpointsCopied is a prefix for recording which checkpoints have already been copied
	AnnCheckpointsCopied = AnnAPIGroup + "/storage.checkpoint.copied"

	// AnnCurrentPodID keeps track of the latest pod servicing this PVC
	AnnCurrentPodID = AnnAPIGroup + "/storage.checkpoint.pod.id"
	// AnnMultiStageImportDone marks a multi-stage import as totally finished
	AnnMultiStageImportDone = AnnAPIGroup + "/storage.checkpoint.done"

	// AnnPreallocationRequested provides a const to indicate whether preallocation should be performed on the PV
	AnnPreallocationRequested = AnnAPIGroup + "/storage.preallocation.requested"
	// AnnPreallocationApplied provides a const for PVC preallocation annotation
	AnnPreallocationApplied = AnnAPIGroup + "/storage.preallocation"

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

	// AnnVddkVersion shows the last VDDK library version used by a DV's importer pod
	AnnVddkVersion = AnnAPIGroup + "/storage.pod.vddk.version"
	// AnnVddkHostConnection shows the last ESX host that serviced a DV's importer pod
	AnnVddkHostConnection = AnnAPIGroup + "/storage.pod.vddk.host"
	// AnnVddkInitImageURL saves a per-DV VDDK image URL on the PVC
	AnnVddkInitImageURL = AnnAPIGroup + "/storage.pod.vddk.initimageurl"

	// AnnRequiresScratch provides a const for our PVC requires scratch annotation
	AnnRequiresScratch = AnnAPIGroup + "/storage.import.requiresScratch"

	// AnnContentType provides a const for the PVC content-type
	AnnContentType = AnnAPIGroup + "/storage.contentType"

	// AnnSource provide a const for our PVC import source annotation
	AnnSource = AnnAPIGroup + "/storage.import.source"
	// AnnEndpoint provides a const for our PVC endpoint annotation
	AnnEndpoint = AnnAPIGroup + "/storage.import.endpoint"

	// AnnSecret provides a const for our PVC secretName annotation
	AnnSecret = AnnAPIGroup + "/storage.import.secretName"
	// AnnCertConfigMap is the name of a configmap containing tls certs
	AnnCertConfigMap = AnnAPIGroup + "/storage.import.certConfigMap"
	// AnnRegistryImportMethod provides a const for registry import method annotation
	AnnRegistryImportMethod = AnnAPIGroup + "/storage.import.registryImportMethod"
	// AnnRegistryImageStream provides a const for registry image stream annotation
	AnnRegistryImageStream = AnnAPIGroup + "/storage.import.registryImageStream"
	// AnnImportPod provides a const for our PVC importPodName annotation
	AnnImportPod = AnnAPIGroup + "/storage.import.importPodName"
	// AnnDiskID provides a const for our PVC diskId annotation
	AnnDiskID = AnnAPIGroup + "/storage.import.diskId"
	// AnnUUID provides a const for our PVC uuid annotation
	AnnUUID = AnnAPIGroup + "/storage.import.uuid"
	// AnnBackingFile provides a const for our PVC backing file annotation
	AnnBackingFile = AnnAPIGroup + "/storage.import.backingFile"
	// AnnThumbprint provides a const for our PVC backing thumbprint annotation
	AnnThumbprint = AnnAPIGroup + "/storage.import.vddk.thumbprint"
	// AnnExtraHeaders provides a const for our PVC extraHeaders annotation
	AnnExtraHeaders = AnnAPIGroup + "/storage.import.extraHeaders"
	// AnnSecretExtraHeaders provides a const for our PVC secretExtraHeaders annotation
	AnnSecretExtraHeaders = AnnAPIGroup + "/storage.import.secretExtraHeaders"

	// AnnCloneToken is the annotation containing the clone token
	AnnCloneToken = AnnAPIGroup + "/storage.clone.token"
	// AnnExtendedCloneToken is the annotation containing the long term clone token
	AnnExtendedCloneToken = AnnAPIGroup + "/storage.extended.clone.token"
	// AnnPermissiveClone annotation allows the clone-controller to skip the clone size validation
	AnnPermissiveClone = AnnAPIGroup + "/permissiveClone"
	// AnnOwnerUID annotation has the owner UID
	AnnOwnerUID = AnnAPIGroup + "/ownerUID"

	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = AnnAPIGroup + "/storage.upload.target"

	//AnnDefaultStorageClass is the annotation indicating that a storage class is the default one.
	AnnDefaultStorageClass = "storageclass.kubernetes.io/is-default-class"

	// AnnOpenShiftImageLookup is the annotation for OpenShift image stream lookup
	AnnOpenShiftImageLookup = "alpha.image.policy.openshift.io/resolve-names"

	// AnnCloneRequest sets our expected annotation for a CloneRequest
	AnnCloneRequest = "k8s.io/CloneRequest"
	// AnnCloneOf is used to indicate that cloning was complete
	AnnCloneOf = "k8s.io/CloneOf"

	// AnnPodNetwork is used for specifying Pod Network
	AnnPodNetwork = "k8s.v1.cni.cncf.io/networks"
	// AnnPodMultusDefaultNetwork is used for specifying default Pod Network
	AnnPodMultusDefaultNetwork = "v1.multus-cni.io/default-network"
	// AnnPodSidecarInjection is used for enabling/disabling Pod istio/AspenMesh sidecar injection
	AnnPodSidecarInjection = "sidecar.istio.io/inject"
	// AnnPodSidecarInjectionDefault is the default value passed for AnnPodSidecarInjection
	AnnPodSidecarInjectionDefault = "false"

	// CloneUniqueID is used as a special label to be used when we search for the pod
	CloneUniqueID = "cdi.kubevirt.io/storage.clone.cloneUniqeId"

	// CloneSourceInUse is reason for event created when clone source pvc is in use
	CloneSourceInUse = "CloneSourceInUse"

	// CloneComplete message
	CloneComplete = "Clone Complete"

	cloneTokenLeeway = 10 * time.Second

	// Default value for preallocation option if not defined in DV or CDIConfig
	defaultPreallocation = false

	// ErrStartingPod provides a const to indicate that a pod wasn't able to start without providing sensitive information (reason)
	ErrStartingPod = "ErrStartingPod"
	// MessageErrStartingPod provides a const to indicate that a pod wasn't able to start without providing sensitive information (message)
	MessageErrStartingPod = "Error starting pod '%s': For more information, request access to cdi-deploy logs from your sysadmin"
	// ErrClaimNotValid provides a const to indicate a claim is not valid
	ErrClaimNotValid = "ErrClaimNotValid"
	// ErrExceededQuota provides a const to indicate the claim has exceeded the quota
	ErrExceededQuota = "ErrExceededQuota"
	// ErrIncompatiblePVC provides a const to indicate a clone is not possible due to an incompatible PVC
	ErrIncompatiblePVC = "ErrIncompatiblePVC"

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
	// SourceImageio is the source type ovirt-imageio
	SourceImageio = "imageio"
	// SourceVDDK is the source type of VDDK
	SourceVDDK = "vddk"

	// ClaimLost reason const
	ClaimLost = "ClaimLost"
	// NotFound reason const
	NotFound = "NotFound"
)

// Size-detection pod error codes
const (
	NoErr int = iota
	ErrBadArguments
	ErrInvalidFile
	ErrInvalidPath
	ErrBadTermFile
	ErrUnknown
)

var (
	// BlockMode is raw block device mode
	BlockMode = corev1.PersistentVolumeBlock
	// FilesystemMode is filesystem device mode
	FilesystemMode = corev1.PersistentVolumeFilesystem

	apiServerKeyOnce sync.Once
	apiServerKey     *rsa.PrivateKey
)

// FakeValidator is a fake token validator
type FakeValidator struct {
	Match     string
	Operation token.Operation
	Name      string
	Namespace string
	Resource  metav1.GroupVersionResource
	Params    map[string]string
}

// Validate is a fake token validation
func (v *FakeValidator) Validate(value string) (*token.Payload, error) {
	if value != v.Match {
		return nil, fmt.Errorf("Token does not match expected")
	}
	resource := metav1.GroupVersionResource{
		Resource: "persistentvolumeclaims",
	}
	return &token.Payload{
		Name:      v.Name,
		Namespace: v.Namespace,
		Operation: token.OperationClone,
		Resource:  resource,
		Params:    v.Params,
	}, nil
}

// NewCloneTokenValidator returns a new token validator
func NewCloneTokenValidator(issuer string, key *rsa.PublicKey) token.Validator {
	return token.NewValidator(issuer, key, cloneTokenLeeway)
}

// GetRequestedImageSize returns the PVC requested size
func GetRequestedImageSize(pvc *v1.PersistentVolumeClaim) (string, error) {
	pvcSize, found := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	if !found {
		return "", errors.Errorf("storage request is missing in pvc \"%s/%s\"", pvc.Namespace, pvc.Name)
	}
	return pvcSize.String(), nil
}

// GetVolumeMode returns the volumeMode from PVC handling default empty value
func GetVolumeMode(pvc *v1.PersistentVolumeClaim) v1.PersistentVolumeMode {
	return util.ResolveVolumeMode(pvc.Spec.VolumeMode)
}

// GetStorageClassByName looks up the storage class based on the name. If no storage class is found returns nil
func GetStorageClassByName(client client.Client, name *string) (*storagev1.StorageClass, error) {
	// look up storage class by name
	if name != nil {
		storageClass := &storagev1.StorageClass{}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: *name}, storageClass); err != nil {
			klog.V(3).Info("Unable to retrieve storage class", "storage class name", *name)
			return nil, errors.New("unable to retrieve storage class")
		}
		return storageClass, nil
	}
	// No storage class found, just return nil for storage class and let caller deal with it.
	return GetDefaultStorageClass(client)
}

// GetDefaultStorageClass returns the default storage class or nil if none found
func GetDefaultStorageClass(client client.Client) (*storagev1.StorageClass, error) {
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

	return nil, nil
}

// GetFilesystemOverheadForStorageClass determines the filesystem overhead defined in CDIConfig for the storageClass.
func GetFilesystemOverheadForStorageClass(client client.Client, storageClassName *string) (cdiv1.Percent, error) {
	cdiConfig := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			klog.V(1).Info("CDIConfig does not exist, pod will not start until it does")
			return "0", nil
		}

		return "0", err
	}

	targetStorageClass, err := GetStorageClassByName(client, storageClassName)
	if err != nil {
		klog.V(3).Info("Storage class", storageClassName, "not found, trying default storage class")
		targetStorageClass, err = GetStorageClassByName(client, nil)
		if err != nil {
			klog.V(3).Info("No default storage class found, continuing with global overhead")
			return cdiConfig.Status.FilesystemOverhead.Global, nil
		}
	}

	if cdiConfig.Status.FilesystemOverhead == nil {
		klog.Errorf("CDIConfig filesystemOverhead used before config controller ran reconcile. Hopefully this only happens during unit testing.")
		return "0", nil
	}

	if targetStorageClass == nil {
		klog.V(3).Info("Storage class", storageClassName, "not found, continuing with global overhead")
		return cdiConfig.Status.FilesystemOverhead.Global, nil
	}

	klog.V(3).Info("target storage class for overhead", targetStorageClass.GetName())

	perStorageConfig := cdiConfig.Status.FilesystemOverhead.StorageClass

	storageClassOverhead, found := perStorageConfig[targetStorageClass.GetName()]
	if found {
		return storageClassOverhead, nil
	}

	return cdiConfig.Status.FilesystemOverhead.Global, nil
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

// AddVolumeDevices returns VolumeDevice slice with one block device for pods using PV with block volume mode
func AddVolumeDevices() []v1.VolumeDevice {
	volumeDevices := []v1.VolumeDevice{
		{
			Name:       DataVolName,
			DevicePath: common.WriteBlockPath,
		},
	}
	return volumeDevices
}

// GetPodsUsingPVCs returns Pods currently using PVCs
func GetPodsUsingPVCs(c client.Client, namespace string, names sets.String, allowReadOnly bool) ([]v1.Pod, error) {
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
				names.Has(volume.PersistentVolumeClaim.ClaimName) {
				addPod := true
				if allowReadOnly {
					if !volume.VolumeSource.PersistentVolumeClaim.ReadOnly {
						onlyReadOnly := true
						for _, c := range pod.Spec.Containers {
							for _, vm := range c.VolumeMounts {
								if vm.Name == volume.Name && !vm.ReadOnly {
									onlyReadOnly = false
								}
							}
						}
						if onlyReadOnly {
							// no rw mounts
							addPod = false
						}
					} else {
						// all mounts must be ro
						addPod = false
					}
				}
				if addPod {
					pods = append(pods, pod)
					break
				}
			}
		}
	}

	return pods, nil
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
		return nil, fmt.Errorf("number of active CDI CRs > 1")
	}

	return &activeResources[0], nil
}

// IsPopulated returns if the passed in PVC has been populated according to the rules outlined in pkg/apis/core/<version>/utils.go
func IsPopulated(pvc *v1.PersistentVolumeClaim, c client.Client) (bool, error) {
	return cdiv1utils.IsPopulated(pvc, func(name, namespace string) (*cdiv1.DataVolume, error) {
		dv := &cdiv1.DataVolume{}
		err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, dv)
		return dv, err
	})
}

// GetPreallocation retuns the preallocation setting for DV, falling back to StorageClass and global setting (in this order)
func GetPreallocation(client client.Client, dataVolume *cdiv1.DataVolume) bool {
	// First, the DV's preallocation
	if dataVolume.Spec.Preallocation != nil {
		return *dataVolume.Spec.Preallocation
	}

	cdiconfig := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiconfig); err != nil {
		klog.Errorf("Unable to find CDI configuration, %v\n", err)
		return defaultPreallocation
	}

	return cdiconfig.Status.Preallocation
}

// GetPriorityClass gets PVC priority class
func GetPriorityClass(pvc *v1.PersistentVolumeClaim) string {
	anno := pvc.GetAnnotations()
	return anno[AnnPriorityClassName]
}

// ShouldDeletePod returns whether the PVC workload pod should be deleted
func ShouldDeletePod(pvc *v1.PersistentVolumeClaim) bool {
	return pvc.GetAnnotations()[AnnPodRetainAfterCompletion] != "true" || pvc.GetAnnotations()[AnnRequiresScratch] == "true" || pvc.DeletionTimestamp != nil
}

// AddFinalizer adds a finalizer to a resource
func AddFinalizer(obj metav1.Object, name string) {
	if HasFinalizer(obj, name) {
		return
	}

	obj.SetFinalizers(append(obj.GetFinalizers(), name))
}

// RemoveFinalizer removes a finalizer from a resource
func RemoveFinalizer(obj metav1.Object, name string) {
	if !HasFinalizer(obj, name) {
		return
	}

	var finalizers []string
	for _, f := range obj.GetFinalizers() {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}

	obj.SetFinalizers(finalizers)
}

// HasFinalizer returns true if a resource has a specific finalizer
func HasFinalizer(object metav1.Object, value string) bool {
	for _, f := range object.GetFinalizers() {
		if f == value {
			return true
		}
	}
	return false
}

// ValidateCloneTokenPVC validates clone token for source and target PVCs
func ValidateCloneTokenPVC(t string, v token.Validator, source, target *v1.PersistentVolumeClaim) error {
	if source.Namespace == target.Namespace {
		return nil
	}

	tokenData, err := v.Validate(t)
	if err != nil {
		return errors.Wrap(err, "error verifying token")
	}

	return validateTokenData(tokenData, source.Namespace, source.Name, target.Namespace, target.Name, string(target.UID))
}

// ValidateCloneTokenDV validates clone token for DV
func ValidateCloneTokenDV(validator token.Validator, dv *cdiv1.DataVolume) error {
	if dv.Spec.Source.PVC == nil || dv.Spec.Source.PVC.Namespace == "" || dv.Spec.Source.PVC.Namespace == dv.Namespace {
		return nil
	}

	tok, ok := dv.Annotations[AnnCloneToken]
	if !ok {
		return errors.New("clone token missing")
	}

	tokenData, err := validator.Validate(tok)
	if err != nil {
		return errors.Wrap(err, "error verifying token")
	}

	return validateTokenData(tokenData, dv.Spec.Source.PVC.Namespace, dv.Spec.Source.PVC.Name, dv.Namespace, dv.Name, "")
}

func validateTokenData(tokenData *token.Payload, srcNamespace, srcName, targetNamespace, targetName, targetUID string) error {
	uid := tokenData.Params["uid"]
	if tokenData.Operation != token.OperationClone ||
		tokenData.Name != srcName ||
		tokenData.Namespace != srcNamespace ||
		tokenData.Resource.Resource != "persistentvolumeclaims" ||
		tokenData.Params["targetNamespace"] != targetNamespace ||
		tokenData.Params["targetName"] != targetName ||
		(uid != "" && uid != targetUID) {
		return errors.New("invalid token")
	}

	return nil
}

// validateContentTypes compares the content type of a clone DV against its source PVC's one
func validateContentTypes(sourcePVC *v1.PersistentVolumeClaim, spec *cdiv1.DataVolumeSpec) (bool, cdiv1.DataVolumeContentType, cdiv1.DataVolumeContentType) {
	sourceContentType := cdiv1.DataVolumeContentType(GetContentType(sourcePVC))
	targetContentType := spec.ContentType
	if targetContentType == "" {
		targetContentType = cdiv1.DataVolumeKubeVirt
	}
	return sourceContentType == targetContentType, sourceContentType, targetContentType
}

// ValidateClone compares a clone spec against its source PVC to validate its creation
func ValidateClone(sourcePVC *v1.PersistentVolumeClaim, spec *cdiv1.DataVolumeSpec) error {
	var targetResources v1.ResourceRequirements

	valid, sourceContentType, targetContentType := validateContentTypes(sourcePVC, spec)
	if !valid {
		msg := fmt.Sprintf("Source contentType (%s) and target contentType (%s) do not match", sourceContentType, targetContentType)
		return errors.New(msg)
	}

	isSizelessClone := false
	explicitPvcRequest := spec.PVC != nil
	if explicitPvcRequest {
		targetResources = spec.PVC.Resources
	} else {
		targetResources = spec.Storage.Resources
		// The storage size in the target DV can be empty
		// when cloning using the 'Storage' API
		if _, ok := targetResources.Requests["storage"]; !ok {
			isSizelessClone = true
		}
	}

	// TODO: Spec.Storage API needs a better more complex check to validate clone size - to account for fsOverhead
	// simple size comparison will not work here
	if (!isSizelessClone && GetVolumeMode(sourcePVC) == v1.PersistentVolumeBlock) || explicitPvcRequest {
		if err := ValidateRequestedCloneSize(sourcePVC.Spec.Resources, targetResources); err != nil {
			return err
		}
	}

	return nil
}

// AddAnnotation adds an annotation to an object
func AddAnnotation(obj metav1.Object, key, value string) {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}
	obj.GetAnnotations()[key] = value
}

// HandleFailedPod handles pod-creation errors and updates the pod's PVC without providing sensitive information
func HandleFailedPod(err error, podName string, pvc *v1.PersistentVolumeClaim, recorder record.EventRecorder, c client.Client) error {
	if err == nil {
		return nil
	}
	// Generic reason and msg to avoid providing sensitive information
	reason := ErrStartingPod
	msg := fmt.Sprintf(MessageErrStartingPod, podName)

	// Error handling to fine-tune the event with pertinent info
	if ErrQuotaExceeded(err) {
		reason = ErrExceededQuota
	}

	recorder.Event(pvc, v1.EventTypeWarning, reason, msg)

	if isCloneSourcePod := CreateCloneSourcePodName(pvc) == podName; isCloneSourcePod {
		AddAnnotation(pvc, AnnSourceRunningCondition, "false")
		AddAnnotation(pvc, AnnSourceRunningConditionReason, reason)
		AddAnnotation(pvc, AnnSourceRunningConditionMessage, msg)
	} else {
		AddAnnotation(pvc, AnnRunningCondition, "false")
		AddAnnotation(pvc, AnnRunningConditionReason, reason)
		AddAnnotation(pvc, AnnRunningConditionMessage, msg)
	}

	AddAnnotation(pvc, AnnPodPhase, string(v1.PodFailed))
	if err := c.Update(context.TODO(), pvc); err != nil {
		return err
	}

	return err
}

// GetSource returns the source string which determines the type of source. If no source or invalid source found, default to http
func GetSource(pvc *corev1.PersistentVolumeClaim) string {
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
		SourceImageio,
		SourceVDDK:
	default:
		source = SourceHTTP
	}
	return source
}

// GetEndpoint returns the endpoint string which contains the full path URI of the target object to be copied.
func GetEndpoint(pvc *corev1.PersistentVolumeClaim) (string, error) {
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

// AddImportVolumeMounts is being called for pods using PV with filesystem volume mode
func AddImportVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      DataVolName,
			MountPath: common.ImporterDataDir,
		},
	}
	return volumeMounts
}

// ValidateRequestedCloneSize validates the clone size requirements on block
func ValidateRequestedCloneSize(sourceResources corev1.ResourceRequirements, targetResources corev1.ResourceRequirements) error {
	sourceRequest := sourceResources.Requests[corev1.ResourceStorage]
	targetRequest := targetResources.Requests[corev1.ResourceStorage]
	// Verify that the target PVC size is equal or larger than the source.
	if sourceRequest.Value() > targetRequest.Value() {
		return errors.New("target resources requests storage size is smaller than the source")
	}
	return nil
}

// CreateCloneSourcePodName creates clone source pod name
func CreateCloneSourcePodName(targetPvc *corev1.PersistentVolumeClaim) string {
	return string(targetPvc.GetUID()) + common.ClonerSourcePodNameSuffix
}

// IsPVCComplete returns true if a PVC is in 'Succeeded' phase, false if not
func IsPVCComplete(pvc *v1.PersistentVolumeClaim) bool {
	if pvc != nil {
		phase, exists := pvc.ObjectMeta.Annotations[AnnPodPhase]
		return exists && (phase == string(v1.PodSucceeded))
	}
	return false
}

// SetRestrictedSecurityContext sets the pod security params to be compatible with restricted PSA
func SetRestrictedSecurityContext(podSpec *v1.PodSpec) {
	hasVolumeMounts := false
	for _, containers := range [][]v1.Container{podSpec.InitContainers, podSpec.Containers} {
		for i := range containers {
			container := &containers[i]
			if container.SecurityContext == nil {
				container.SecurityContext = &v1.SecurityContext{}
			}
			container.SecurityContext.Capabilities = &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			}
			container.SecurityContext.SeccompProfile = &v1.SeccompProfile{
				Type: v1.SeccompProfileTypeRuntimeDefault,
			}
			container.SecurityContext.AllowPrivilegeEscalation = pointer.BoolPtr(false)
			container.SecurityContext.RunAsNonRoot = pointer.BoolPtr(true)
			container.SecurityContext.RunAsUser = pointer.Int64(common.QemuSubGid)
			if len(container.VolumeMounts) > 0 {
				hasVolumeMounts = true
			}
		}
	}

	if hasVolumeMounts {
		if podSpec.SecurityContext == nil {
			podSpec.SecurityContext = &v1.PodSecurityContext{}
		}
		podSpec.SecurityContext.FSGroup = pointer.Int64(common.QemuSubGid)
	}
}

// CreatePvc creates PVC
func CreatePvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return CreatePvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimBound)
}

// CreatePvcInStorageClass creates PVC with storgae class
func CreatePvcInStorageClass(name, ns string, storageClassName *string, annotations, labels map[string]string, phase v1.PersistentVolumeClaimPhase) *v1.PersistentVolumeClaim {
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
			UID:         types.UID(ns + "-" + name),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
				},
			},
			StorageClassName: storageClassName,
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: phase,
		},
	}
	pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
	return pvc
}

// GetAPIServerKey returns API server RSA key
func GetAPIServerKey() *rsa.PrivateKey {
	apiServerKeyOnce.Do(func() {
		apiServerKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	})
	return apiServerKey
}

// CreateStorageClass creates storage class CR
func CreateStorageClass(name string, annotations map[string]string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
}

// CreateImporterTestPod creates importer test pod CR
func CreateImporterTestPod(pvc *corev1.PersistentVolumeClaim, dvname string, scratchPvc *corev1.PersistentVolumeClaim) *corev1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s", common.ImporterPodName, pvc.Name)

	blockOwnerDeletion := true
	isController := true

	volumes := []corev1.Volume{
		{
			Name: dvname,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	if scratchPvc != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ScratchVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: scratchPvc.Name,
					ReadOnly:  false,
				},
			},
		})
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:        common.CDILabelValue,
				common.CDIComponentLabel:  common.ImporterPodName,
				common.PrometheusLabelKey: common.PrometheusLabelValue,
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
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            common.ImporterPodName,
					Image:           "test/myimage",
					ImagePullPolicy: corev1.PullPolicy("Always"),
					Args:            []string{"-v=5"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes:       volumes,
		},
	}

	ep, _ := GetEndpoint(pvc)
	source := GetSource(pvc)
	contentType := GetContentType(pvc)
	imageSize, _ := GetRequestedImageSize(pvc)
	volumeMode := GetVolumeMode(pvc)

	env := []corev1.EnvVar{
		{
			Name:  common.ImporterSource,
			Value: source,
		},
		{
			Name:  common.ImporterEndpoint,
			Value: ep,
		},
		{
			Name:  common.ImporterContentType,
			Value: contentType,
		},
		{
			Name:  common.ImporterImageSize,
			Value: imageSize,
		},
		{
			Name:  common.OwnerUID,
			Value: string(pvc.UID),
		},
		{
			Name:  common.InsecureTLSVar,
			Value: "false",
		},
	}
	pod.Spec.Containers[0].Env = env
	if volumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = AddVolumeDevices()
	} else {
		pod.Spec.Containers[0].VolumeMounts = AddImportVolumeMounts()
	}

	if scratchPvc != nil {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}

	return pod
}

// CreateStorageClassWithProvisioner creates CR of storage class with provisioner
func CreateStorageClassWithProvisioner(name string, annotations, labels map[string]string, provisioner string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		Provisioner: provisioner,
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
	}
}

// CreateClient creates a fake client
func CreateClient(objs ...runtime.Object) client.Client {
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	corev1.AddToScheme(s)
	storagev1.AddToScheme(s)
	ocpconfigv1.AddToScheme(s)
	return fake.NewFakeClientWithScheme(s, objs...)
}

// ErrQuotaExceeded checked is the error is of exceeded quota
func ErrQuotaExceeded(err error) bool {
	return strings.Contains(err.Error(), "exceeded quota:")
}

// GetContentType returns the content type of the source image. If invalid or not set, default to kubevirt
func GetContentType(pvc *corev1.PersistentVolumeClaim) string {
	contentType, found := pvc.Annotations[AnnContentType]
	if !found {
		return string(cdiv1.DataVolumeKubeVirt)
	}
	switch contentType {
	case
		string(cdiv1.DataVolumeKubeVirt),
		string(cdiv1.DataVolumeArchive):
	default:
		contentType = string(cdiv1.DataVolumeKubeVirt)
	}
	return contentType
}

// GetNamespace returns the given namespace if not empty, otherwise the default namespace
func GetNamespace(namespace, defaultNamespace string) string {
	if namespace == "" {
		return defaultNamespace
	}
	return namespace
}

// IsErrCacheNotStarted checked is the error is of cache not started
func IsErrCacheNotStarted(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*cache.ErrCacheNotStarted)
	return ok
}

// GetDataVolumeTTLSeconds gets the current DataVolume TTL in seconds if GC is enabled, or < 0 if GC is disabled
func GetDataVolumeTTLSeconds(config *cdiv1.CDIConfig) int32 {
	const defaultDataVolumeTTLSeconds = 0
	if config.Spec.DataVolumeTTLSeconds != nil {
		return *config.Spec.DataVolumeTTLSeconds
	}
	return defaultDataVolumeTTLSeconds
}

// NewImportDataVolume returns new import DataVolume CR
func NewImportDataVolume(name string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		TypeMeta: metav1.TypeMeta{APIVersion: cdiv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
			UID:       types.UID(metav1.NamespaceDefault + "-" + name),
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://example.com/data",
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
			PriorityClassName: "p0",
		},
	}
}
