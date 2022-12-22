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
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CertVolName is the name of the volumecontaining certs
	CertVolName = "cdi-cert-vol"

	// AnnOwnerRef is used when owner is in a different namespace
	AnnOwnerRef = cc.AnnAPIGroup + "/storage.ownerRef"

	// AnnImmediateBinding provides a const to indicate whether immediate binding should be performed on the PV (overrides global config)
	AnnImmediateBinding = cc.AnnAPIGroup + "/storage.bind.immediate.requested"

	// PodRunningReason is const that defines the pod was started as a reason
	PodRunningReason = "Pod is running"

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

var (
	vddkInfoMatch = regexp.MustCompile(`((.*; )|^)VDDK: (?P<info>{.*})`)
)

func checkPVC(pvc *v1.PersistentVolumeClaim, annotation string, log logr.Logger) bool {
	// check if we have proper annotation
	if !metav1.HasAnnotation(pvc.ObjectMeta, annotation) {
		log.V(1).Info("PVC annotation not found, skipping pvc", "annotation", annotation)
		return false
	}

	return true
}

func isWaitForFirstConsumerEnabled(isImmediateBindingRequested bool, gates featuregates.FeatureGates) (bool, error) {
	// when PVC requests immediateBinding it cannot honor wffc logic
	pvcHonorWaitForFirstConsumer := !isImmediateBindingRequested
	globalHonorWaitForFirstConsumer, err := gates.HonorWaitForFirstConsumerEnabled()
	if err != nil {
		return false, err
	}

	return pvcHonorWaitForFirstConsumer && globalHonorWaitForFirstConsumer, nil
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

// createScratchPersistentVolumeClaim creates and returns a pointer to a scratch PVC which is created based on the passed-in pvc and storage class name.
func createScratchPersistentVolumeClaim(client client.Client, pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name, storageClassName string, installerLabels map[string]string, recorder record.EventRecorder) (*v1.PersistentVolumeClaim, error) {
	scratchPvcSpec := newScratchPersistentVolumeClaimSpec(pvc, pod, name, storageClassName)
	util.SetRecommendedLabels(scratchPvcSpec, installerLabels, "cdi-controller")
	if err := client.Create(context.TODO(), scratchPvcSpec); err != nil {
		if cc.ErrQuotaExceeded(err) {
			recorder.Event(pvc, v1.EventTypeWarning, cc.ErrExceededQuota, err.Error())
		}
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

// GetFilesystemOverhead determines the filesystem overhead defined in CDIConfig for this PVC's volumeMode and storageClass.
func GetFilesystemOverhead(client client.Client, pvc *v1.PersistentVolumeClaim) (cdiv1.Percent, error) {
	if cc.GetVolumeMode(pvc) != v1.PersistentVolumeFilesystem {
		return "0", nil
	}

	return cc.GetFilesystemOverheadForStorageClass(client, pvc.Spec.StorageClassName)
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

func podPhaseFromPVC(pvc *v1.PersistentVolumeClaim) v1.PodPhase {
	phase := pvc.ObjectMeta.Annotations[cc.AnnPodPhase]
	return v1.PodPhase(phase)
}

func podSucceededFromPVC(pvc *v1.PersistentVolumeClaim) bool {
	return podPhaseFromPVC(pvc) == v1.PodSucceeded
}

func setAnnotationsFromPodWithPrefix(anno map[string]string, pod *v1.Pod, prefix string) {
	if pod == nil || pod.Status.ContainerStatuses == nil {
		return
	}
	annPodRestarts, _ := strconv.Atoi(anno[cc.AnnPodRestarts])
	podRestarts := int(pod.Status.ContainerStatuses[0].RestartCount)
	if podRestarts >= annPodRestarts {
		anno[cc.AnnPodRestarts] = strconv.Itoa(podRestarts)
	}
	setVddkAnnotations(anno, pod)
	containerState := pod.Status.ContainerStatuses[0].State
	if containerState.Running != nil {
		anno[prefix] = "true"
		anno[prefix+".message"] = ""
		anno[prefix+".reason"] = PodRunningReason
	} else {
		anno[cc.AnnRunningCondition] = "false"
		if containerState.Waiting != nil && containerState.Waiting.Reason != "CrashLoopBackOff" {
			anno[prefix+".message"] = simplifyKnownMessage(containerState.Waiting.Message)
			anno[prefix+".reason"] = containerState.Waiting.Reason
		} else if containerState.Terminated != nil {
			anno[prefix+".message"] = simplifyKnownMessage(containerState.Terminated.Message)
			anno[prefix+".reason"] = containerState.Terminated.Reason
			if strings.Contains(containerState.Terminated.Message, common.PreallocationApplied) {
				anno[cc.AnnPreallocationApplied] = "true"
			}
		}
	}
}

func simplifyKnownMessage(msg string) string {
	if strings.Contains(msg, "is larger than the reported available") ||
		strings.Contains(msg, "no space left on device") ||
		strings.Contains(msg, "file largest block is bigger than maxblock") {
		return "DataVolume too small to contain image"
	}

	return msg
}

func setVddkAnnotations(anno map[string]string, pod *v1.Pod) {
	if pod.Status.ContainerStatuses[0].State.Terminated == nil {
		return
	}
	terminationMessage := pod.Status.ContainerStatuses[0].State.Terminated.Message
	klog.V(1).Info("Saving VDDK annotations from pod status message: ", "message", terminationMessage)

	var terminationInfo string
	matches := vddkInfoMatch.FindAllStringSubmatch(terminationMessage, -1)
	for index, matchName := range vddkInfoMatch.SubexpNames() {
		if matchName == "info" && len(matches) > 0 {
			terminationInfo = matches[0][index]
			break
		}
	}

	var vddkInfo util.VddkInfo
	err := json.Unmarshal([]byte(terminationInfo), &vddkInfo)
	if err != nil {
		return
	}
	if vddkInfo.Host != "" {
		anno[cc.AnnVddkHostConnection] = vddkInfo.Host
	}
	if vddkInfo.Version != "" {
		anno[cc.AnnVddkVersion] = vddkInfo.Version
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
		anno[prefix+".message"] = cc.ClaimLost
		anno[prefix+".reason"] = cc.ClaimLost
	default:
		anno[prefix] = "false"
		anno[prefix+".message"] = "Unknown"
		anno[prefix+".reason"] = "Unknown"
	}
}

func getScratchNameFromPod(pod *v1.Pod) (string, bool) {
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == cc.ScratchVolName {
			return vol.PersistentVolumeClaim.ClaimName, true
		}
	}

	return "", false
}

// setPodPvcAnnotations applies PVC annotations on the pod
func setPodPvcAnnotations(pod *v1.Pod, pvc *v1.PersistentVolumeClaim) {
	allowedAnnotations := map[string]string{
		cc.AnnPodNetwork:              "",
		cc.AnnPodSidecarInjection:     cc.AnnPodSidecarInjectionDefault,
		cc.AnnPodMultusDefaultNetwork: ""}
	for ann, def := range allowedAnnotations {
		val, ok := pvc.Annotations[ann]
		if !ok && def != "" {
			val = def
		}
		if val != "" {
			klog.V(1).Info("Applying PVC annotation on the pod", ann, val)
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[ann] = val
		}
	}
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

func createBlockPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	pvcDef := cc.CreatePvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimBound)
	volumeMode := v1.PersistentVolumeBlock
	pvcDef.Spec.VolumeMode = &volumeMode
	return pvcDef
}
