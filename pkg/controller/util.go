/*
Copyright 2022 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
limitations under the License.
See the License for the specific language governing permissions and
*/

package controller

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

const (
	// CertVolName is the name of the volume containing certs
	CertVolName = "cdi-cert-vol"

	// SecretVolName is the name of the volume containing gcs key
	//nolint:gosec // This is not a real secret
	SecretVolName = "cdi-secret-vol"

	// AnnOwnerRef is used when owner is in a different namespace
	AnnOwnerRef = cc.AnnAPIGroup + "/storage.ownerRef"

	// PodRunningReason is const that defines the pod was started as a reason
	PodRunningReason = "Pod is running"

	// ScratchSpaceRequiredReason is a const that defines the pod exited due to a lack of scratch space
	ScratchSpaceRequiredReason = "Scratch space required"

	// ImagePullFailedReason is a const that defines the pod exited due to failure when pulling image
	ImagePullFailedReason = "ImagePullFailed"

	// ImportCompleteMessage is a const that defines the pod completeded the import successfully
	ImportCompleteMessage = "Import Complete"

	// ProxyCertVolName is the name of the volumecontaining certs
	ProxyCertVolName = "cdi-proxy-cert-vol"
	// ClusterWideProxyAPIGroup is the APIGroup for OpenShift Cluster Wide Proxy
	ClusterWideProxyAPIGroup = "config.openshift.io"
	// ClusterWideProxyAPIKind is the APIKind for OpenShift Cluster Wide Proxy
	ClusterWideProxyAPIKind = "Proxy"
	// ClusterWideProxyAPIVersion is the APIVersion for OpenShift Cluster Wide Proxy
	ClusterWideProxyAPIVersion = "v1"
	// ClusterWideProxyName is the OpenShift Cluster Wide Proxy object name. There is only one obj in the cluster.
	ClusterWideProxyName = "cluster"
	// ClusterWideProxyConfigMapName is the OpenShift Cluster Wide Proxy ConfigMap name for CA certificates.
	ClusterWideProxyConfigMapName = "user-ca-bundle"
	// ClusterWideProxyConfigMapNameSpace is the OpenShift Cluster Wide Proxy ConfigMap namespace for CA certificates.
	ClusterWideProxyConfigMapNameSpace = "openshift-config"
	// ClusterWideProxyConfigMapKey is the OpenShift Cluster Wide Proxy ConfigMap key name for CA certificates.
	ClusterWideProxyConfigMapKey = "ca-bundle.crt"
)

func checkPVC(pvc *corev1.PersistentVolumeClaim, annotation string, log logr.Logger) bool {
	// check if we have proper annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, annotation) {
		log.V(1).Info("PVC annotation not found, skipping pvc", "annotation", annotation)
		return false
	}

	return true
}

// - when the SkipWFFCVolumesEnabled is true, the CDI controller will only handle BOUND the PVC
// - when the SkipWFFCVolumesEnabled is false, the CDI controller will can handle it - it will create worker pods for the PVC (this will bind it)
func shouldHandlePvc(pvc *corev1.PersistentVolumeClaim, honorWaitForFirstConsumerEnabled bool, log logr.Logger) bool {
	if honorWaitForFirstConsumerEnabled {
		return isBound(pvc, log)
	}
	return true
}

func isBound(pvc *corev1.PersistentVolumeClaim, log logr.Logger) bool {
	if pvc.Status.Phase != corev1.ClaimBound {
		log.V(1).Info("PVC not bound, skipping pvc", "Phase", pvc.Status.Phase)
		return false
	}

	return true
}

// checks if particular label exists in pvc
func checkIfLabelExists(pvc *corev1.PersistentVolumeClaim, lbl string, val string) bool {
	value, exists := pvc.ObjectMeta.Labels[lbl]
	if exists && value == val {
		return true
	}
	return false
}

// newScratchPersistentVolumeClaimSpec creates a new PVC based on the size of the passed in PVC.
// It also sets the appropriate OwnerReferences on the resource
// which allows handleObject to discover the pod resource that 'owns' it, and clean up when needed.
func newScratchPersistentVolumeClaimSpec(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, name, storageClassName string) *corev1.PersistentVolumeClaim {
	labels := map[string]string{
		"app": "containerized-data-importer",
	}

	annotations := make(map[string]string)
	// Copy kubevirt.io annotations, but NOT the CDI annotations as those will trigger another import/upload/clone on the scratchspace
	// pvc.
	if len(pvc.GetAnnotations()) > 0 {
		for k, v := range pvc.GetAnnotations() {
			if strings.Contains(k, common.KubeVirtAnnKey) && !strings.Contains(k, common.CDIAnnKey) {
				annotations[k] = v
			}
		}
	}
	// When the original PVC is being handled by a populator, copy AnnSelectedNode to avoid issues with k8s scheduler
	_, isPopulator := pvc.Annotations[cc.AnnPopulatorKind]
	selectedNode := pvc.Annotations[cc.AnnSelectedNode]
	if isPopulator && selectedNode != "" {
		annotations[cc.AnnSelectedNode] = selectedNode
	}

	pvcDef := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   pvc.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				MakePodOwnerReference(pod),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources:   *pvc.Spec.Resources.DeepCopy(),
		},
	}
	if storageClassName != "" {
		pvcDef.Spec.StorageClassName = &storageClassName
	}
	return pvcDef
}

// createScratchPersistentVolumeClaim creates and returns a pointer to a scratch PVC which is created based on the passed-in pvc and storage class name.
func createScratchPersistentVolumeClaim(client client.Client, pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, name, storageClassName string, installerLabels map[string]string, recorder record.EventRecorder) (*corev1.PersistentVolumeClaim, error) {
	scratchPvcSpec := newScratchPersistentVolumeClaimSpec(pvc, pod, name, storageClassName)

	sizeRequest := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	scratchFsOverhead, err := GetFilesystemOverhead(context.TODO(), client, scratchPvcSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get filesystem overhead for scratch PVC")
	}
	scratchFsOverheadFloat, _ := strconv.ParseFloat(string(scratchFsOverhead), 64)
	pvcFsOverhead, err := GetFilesystemOverhead(context.TODO(), client, pvc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get filesystem overhead for original PVC")
	}
	pvcFsOverheadFloat, _ := strconv.ParseFloat(string(pvcFsOverhead), 64)

	// Calculate the expected usable space for the scratch PVC based on both original PVC size and its fs overhead.
	expectedVirtualSize := util.GetUsableSpace(pvcFsOverheadFloat, sizeRequest.Value())
	// Now we add the fs overhead for the scratch PVC.
	// TODO: Should we allow using a smaller overhead for scratch PVCs?
	usableSpaceRaw := util.GetRequiredSpace(scratchFsOverheadFloat, expectedVirtualSize)

	// Since GetUsableSpace rounds down to the nearest block size to account for qemu-img calculations, and GetRequiredSpace rounds up before applying overhead,
	// it's good practice round up here to ensure we don't end up with a scratch PVC that is smaller than the original PVC.
	usableSpaceRaw = util.RoundUp(usableSpaceRaw, util.DefaultAlignBlockSize)

	scratchPvcSpec.Spec.Resources.Requests[corev1.ResourceStorage] = *resource.NewScaledQuantity(usableSpaceRaw, 0)

	util.SetRecommendedLabels(scratchPvcSpec, installerLabels, "cdi-controller")
	cc.AddLabel(scratchPvcSpec, cc.LabelExcludeFromVeleroBackup, "true")
	if err := client.Create(context.TODO(), scratchPvcSpec); err != nil {
		if cc.ErrQuotaExceeded(err) {
			recorder.Event(pvc, corev1.EventTypeWarning, cc.ErrExceededQuota, err.Error())
		}
		if !k8serrors.IsAlreadyExists(err) {
			return nil, errors.Wrap(err, "scratch PVC API create errored")
		}
	}
	scratchPvc := &corev1.PersistentVolumeClaim{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: scratchPvcSpec.Name, Namespace: pvc.Namespace}, scratchPvc); err != nil {
		klog.Errorf("Unable to get scratch space pvc, %v\n", err)
		return nil, err
	}
	klog.V(3).Infof("scratch PVC \"%s/%s\" created\n", scratchPvc.Namespace, scratchPvc.Name)
	return scratchPvc, nil
}

// GetFilesystemOverhead determines the filesystem overhead defined in CDIConfig for this PVC's volumeMode and storageClass.
func GetFilesystemOverhead(ctx context.Context, client client.Client, pvc *corev1.PersistentVolumeClaim) (cdiv1.Percent, error) {
	if cc.GetVolumeMode(pvc) != corev1.PersistentVolumeFilesystem {
		return "0", nil
	}

	return cc.GetFilesystemOverheadForStorageClass(ctx, client, pvc.Spec.StorageClassName)
}

// GetScratchPvcStorageClass tries to determine which storage class to use for use with a scratch persistent
// volume claim. The order of preference is the following:
// 1. Defined value in CDI Config field scratchSpaceStorageClass.
// 2. If 1 is not available, use the storage class name of the original pvc that will own the scratch pvc.
// 3. If none of those are available, return blank.
func GetScratchPvcStorageClass(client client.Client, pvc *corev1.PersistentVolumeClaim) string {
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
func MakePVCOwnerReference(pvc *corev1.PersistentVolumeClaim) metav1.OwnerReference {
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
func MakePodOwnerReference(pod *corev1.Pod) metav1.OwnerReference {
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

func podPhaseFromPVC(pvc *corev1.PersistentVolumeClaim) corev1.PodPhase {
	phase := pvc.ObjectMeta.Annotations[cc.AnnPodPhase]
	return corev1.PodPhase(phase)
}

func podSucceededFromPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return podPhaseFromPVC(pvc) == corev1.PodSucceeded
}

func setAnnotationsFromPodWithPrefix(anno map[string]string, pod *corev1.Pod, termMsg *common.TerminationMessage, prefix string) {
	if pod == nil || pod.Status.ContainerStatuses == nil {
		return
	}
	annPodRestarts, _ := strconv.Atoi(anno[cc.AnnPodRestarts])
	podRestarts := int(pod.Status.ContainerStatuses[0].RestartCount)
	if podRestarts >= annPodRestarts {
		anno[cc.AnnPodRestarts] = strconv.Itoa(podRestarts)
	}

	containerState := pod.Status.ContainerStatuses[0].State
	if containerState.Running != nil {
		anno[prefix] = "true"
		anno[prefix+".message"] = ""
		anno[prefix+".reason"] = PodRunningReason
		return
	}

	anno[cc.AnnRunningCondition] = "false"

	for _, status := range pod.Status.ContainerStatuses {
		if status.Started != nil && !(*status.Started) && status.State.Waiting != nil {
			switch status.State.Waiting.Reason {
			case "ImagePullBackOff", "ErrImagePull", "InvalidImageName":
				anno[prefix+".message"] = fmt.Sprintf("%s: %s", common.ImagePullFailureText, status.Image)
				anno[prefix+".reason"] = ImagePullFailedReason
				return
			}
		}
	}

	if containerState.Waiting != nil && containerState.Waiting.Reason != "CrashLoopBackOff" {
		anno[prefix+".message"] = simplifyKnownMessage(containerState.Waiting.Message)
		anno[prefix+".reason"] = containerState.Waiting.Reason
		return
	}

	if containerState.Terminated != nil {
		if termMsg != nil {
			if termMsg.ScratchSpaceRequired != nil && *termMsg.ScratchSpaceRequired {
				anno[cc.AnnRequiresScratch] = "true"
				anno[prefix+".message"] = common.ScratchSpaceRequired
				anno[prefix+".reason"] = ScratchSpaceRequiredReason
				return
			}
			// Handle extended termination message
			if termMsg.Message != nil {
				anno[prefix+".message"] = *termMsg.Message
			}
			if termMsg.VddkInfo != nil {
				if termMsg.VddkInfo.Host != "" {
					anno[cc.AnnVddkHostConnection] = termMsg.VddkInfo.Host
				}
				if termMsg.VddkInfo.Version != "" {
					anno[cc.AnnVddkVersion] = termMsg.VddkInfo.Version
				}
			}
			if termMsg.PreallocationApplied != nil && *termMsg.PreallocationApplied {
				anno[cc.AnnPreallocationApplied] = "true"
			}
		} else {
			// Handle plain termination message (legacy)
			anno[prefix+".message"] = simplifyKnownMessage(containerState.Terminated.Message)
			if strings.Contains(containerState.Terminated.Message, common.PreallocationApplied) {
				anno[cc.AnnPreallocationApplied] = "true"
			}

			if strings.Contains(containerState.Terminated.Message, common.ImagePullFailureText) {
				anno[prefix+".reason"] = ImagePullFailedReason
				return
			}
		}
		anno[prefix+".reason"] = containerState.Terminated.Reason
	}
}

func addLabelsFromTerminationMessage(labels map[string]string, termMsg *common.TerminationMessage) map[string]string {
	newLabels := make(map[string]string, 0)
	for k, v := range labels {
		newLabels[k] = v
	}
	if termMsg != nil {
		for k, v := range termMsg.Labels {
			if _, found := newLabels[k]; !found {
				newLabels[k] = v
			}
		}
	}
	return newLabels
}

func simplifyKnownMessage(msg string) string {
	if strings.Contains(msg, "is larger than the reported available") ||
		strings.Contains(msg, "no space left on device") ||
		strings.Contains(msg, "file largest block is bigger than maxblock") ||
		strings.Contains(msg, "disk quota exceeded") {
		return "DataVolume too small to contain image"
	}

	return msg
}

func parseTerminationMessage(pod *corev1.Pod) (*common.TerminationMessage, error) {
	if pod == nil || pod.Status.ContainerStatuses == nil {
		return nil, nil
	}

	state := pod.Status.ContainerStatuses[0].State
	if state.Terminated == nil || state.Terminated.ExitCode != 0 {
		return nil, nil
	}

	termMsg := &common.TerminationMessage{}
	if err := json.Unmarshal([]byte(state.Terminated.Message), termMsg); err != nil {
		return nil, err
	}

	return termMsg, nil
}

func setBoundConditionFromPVC(anno map[string]string, prefix string, pvc *corev1.PersistentVolumeClaim) {
	switch pvc.Status.Phase {
	case corev1.ClaimBound:
		anno[prefix] = "true"
		anno[prefix+".message"] = ""
		anno[prefix+".reason"] = ""
	case corev1.ClaimPending:
		anno[prefix] = "false"
		anno[prefix+".message"] = "Claim Pending"
		anno[prefix+".reason"] = "Claim Pending"
	case corev1.ClaimLost:
		anno[prefix] = "false"
		anno[prefix+".message"] = cc.ClaimLost
		anno[prefix+".reason"] = cc.ClaimLost
	default:
		anno[prefix] = "false"
		anno[prefix+".message"] = "Unknown"
		anno[prefix+".reason"] = "Unknown"
	}
}

func getScratchNameFromPod(pod *corev1.Pod) (string, bool) {
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == cc.ScratchVolName {
			return vol.PersistentVolumeClaim.ClaimName, true
		}
	}

	return "", false
}

func podUsingPVC(pvc *corev1.PersistentVolumeClaim, readOnly bool) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pvc.Namespace,
			Name:      pvc.Name + "-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:     "v1",
							ReadOnly: readOnly,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "v1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  readOnly,
						},
					},
				},
			},
		},
	}
}

func createBlockPvc(name, ns string, annotations, labels map[string]string) *corev1.PersistentVolumeClaim {
	pvcDef := cc.CreatePvcInStorageClass(name, ns, nil, annotations, labels, corev1.ClaimBound)
	volumeMode := corev1.PersistentVolumeBlock
	pvcDef.Spec.VolumeMode = &volumeMode
	return pvcDef
}
