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

package datavolume

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"

	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ErrUnableToClone provides a const to indicate some errors are blocking the clone
	ErrUnableToClone = "ErrUnableToClone"

	// CloneScheduled provides a const to indicate clone is scheduled
	CloneScheduled = "CloneScheduled"
	// CloneInProgress provides a const to indicate clone is in progress
	CloneInProgress = "CloneInProgress"
	// SnapshotForSmartCloneInProgress provides a const to indicate snapshot creation for smart-clone is in progress
	SnapshotForSmartCloneInProgress = "SnapshotForSmartCloneInProgress"
	// CloneFromSnapshotSourceInProgress provides a const to indicate clone from snapshot source is in progress
	CloneFromSnapshotSourceInProgress = "CloneFromSnapshotSourceInProgress"
	// SnapshotForSmartCloneCreated provides a const to indicate snapshot creation for smart-clone has been completed
	SnapshotForSmartCloneCreated = "SnapshotForSmartCloneCreated"
	// SmartClonePVCInProgress provides a const to indicate snapshot creation for smart-clone is in progress
	SmartClonePVCInProgress = "SmartClonePVCInProgress"
	// SmartCloneSourceInUse provides a const to indicate a smart clone is being delayed because the source is in use
	SmartCloneSourceInUse = "SmartCloneSourceInUse"
	// CSICloneInProgress provides a const to indicate  csi volume clone is in progress
	CSICloneInProgress = "CSICloneInProgress"
	// CSICloneSourceInUse provides a const to indicate a csi volume clone is being delayed because the source is in use
	CSICloneSourceInUse = "CSICloneSourceInUse"
	// HostAssistedCloneSourceInUse provides a const to indicate a host-assisted clone is being delayed because the source is in use
	HostAssistedCloneSourceInUse = "HostAssistedCloneSourceInUse"
	// CloneFailed provides a const to indicate clone has failed
	CloneFailed = "CloneFailed"
	// CloneSucceeded provides a const to indicate clone has succeeded
	CloneSucceeded = "CloneSucceeded"

	// MessageCloneScheduled provides a const to form clone is scheduled message
	MessageCloneScheduled = "Cloning from %s/%s into %s/%s scheduled"
	// MessageCloneInProgress provides a const to form clone is in progress message
	MessageCloneInProgress = "Cloning from %s/%s into %s/%s in progress"
	// MessageCloneFailed provides a const to form clone has failed message
	MessageCloneFailed = "Cloning from %s/%s into %s/%s failed"
	// MessageCloneSucceeded provides a const to form clone has succeeded message
	MessageCloneSucceeded = "Successfully cloned from %s/%s into %s/%s"
	// MessageSmartCloneInProgress provides a const to form snapshot for smart-clone is in progress message
	MessageSmartCloneInProgress = "Creating snapshot for smart-clone is in progress (for pvc %s/%s)"
	// MessageCloneFromSnapshotSourceInProgress provides a const to form clone from snapshot source is in progress message
	MessageCloneFromSnapshotSourceInProgress = "Creating PVC from snapshot source is in progress (for snapshot %s/%s)"
	// MessageSmartClonePVCInProgress provides a const to form snapshot for smart-clone is in progress message
	MessageSmartClonePVCInProgress = "Creating PVC for smart-clone is in progress (for pvc %s/%s)"
	// MessageCsiCloneInProgress provides a const to form a CSI Volume Clone in progress message
	MessageCsiCloneInProgress = "CSI Volume clone in progress (for pvc %s/%s)"

	// ExpansionInProgress is const representing target PVC expansion
	ExpansionInProgress = "ExpansionInProgress"
	// MessageExpansionInProgress is a const for reporting target expansion
	MessageExpansionInProgress = "Expanding PersistentVolumeClaim for DataVolume %s/%s"
	// NamespaceTransferInProgress is const representing target PVC transfer
	NamespaceTransferInProgress = "NamespaceTransferInProgress"
	// MessageNamespaceTransferInProgress is a const for reporting target transfer
	MessageNamespaceTransferInProgress = "Transferring PersistentVolumeClaim for DataVolume %s/%s"
	// SizeDetectionPodCreated provides a const to indicate that the size-detection pod has been created (reason)
	SizeDetectionPodCreated = "SizeDetectionPodCreated"
	// MessageSizeDetectionPodCreated provides a const to indicate that the size-detection pod has been created (message)
	MessageSizeDetectionPodCreated = "Size-detection pod created"
	// SizeDetectionPodNotReady reports that the size-detection pod has not finished its exectuion (reason)
	SizeDetectionPodNotReady = "SizeDetectionPodNotReady"
	// MessageSizeDetectionPodNotReady reports that the size-detection pod has not finished its exectuion (message)
	MessageSizeDetectionPodNotReady = "The size detection pod is not finished yet"
	// ImportPVCNotReady reports that it's not yet possible to access the source PVC (reason)
	ImportPVCNotReady = "ImportPVCNotReady"
	// MessageImportPVCNotReady reports that it's not yet possible to access the source PVC (message)
	MessageImportPVCNotReady = "The source PVC is not fully imported"
	// CloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (reason)
	CloneValidationFailed = "CloneValidationFailed"
	// MessageCloneValidationFailed reports that a clone wasn't admitted by our validation mechanism (message)
	MessageCloneValidationFailed = "The clone doesn't meet the validation requirements"
	// CloneWithoutSource reports that the source of a clone doesn't exists (reason)
	CloneWithoutSource = "CloneWithoutSource"
	// MessageCloneWithoutSource reports that the source of a clone doesn't exists (message)
	MessageCloneWithoutSource = "The source %s %s doesn't exist"

	// AnnCSICloneRequest annotation associates object with CSI Clone Request
	AnnCSICloneRequest = "cdi.kubevirt.io/CSICloneRequest"

	// AnnVirtualImageSize annotation contains the Virtual Image size of a PVC used for host-assisted cloning
	AnnVirtualImageSize = "cdi.Kubervirt.io/virtualSize"

	// AnnSourceCapacity annotation contains the storage capacity of a PVC used for host-assisted cloning
	AnnSourceCapacity = "cdi.Kubervirt.io/sourceCapacity"

	crossNamespaceFinalizer = "cdi.kubevirt.io/dataVolumeFinalizer"

	annReadyForTransfer = "cdi.kubevirt.io/readyForTransfer"
)

// CloneReconcilerBase members
type CloneReconcilerBase struct {
	ReconcilerBase
	clonerImage    string
	importerImage  string
	pullPolicy     string
	tokenValidator token.Validator
	tokenGenerator token.Generator
}

func (r *CloneReconcilerBase) ensureExtendedToken(pvc *corev1.PersistentVolumeClaim) error {
	_, ok := pvc.Annotations[cc.AnnExtendedCloneToken]
	if ok {
		return nil
	}

	token, ok := pvc.Annotations[cc.AnnCloneToken]
	if !ok {
		return fmt.Errorf("token missing")
	}

	payload, err := r.tokenValidator.Validate(token)
	if err != nil {
		return err
	}

	if payload.Params == nil {
		payload.Params = make(map[string]string)
	}
	payload.Params["uid"] = string(pvc.UID)

	newToken, err := r.tokenGenerator.Generate(payload)
	if err != nil {
		return err
	}

	pvc.Annotations[cc.AnnExtendedCloneToken] = newToken

	if err := r.updatePVC(pvc); err != nil {
		return err
	}

	return nil
}

// When the clone is finished some additional actions may be applied
// like namespaceTransfer Cleanup or size expansion
func (r *CloneReconcilerBase) finishClone(log logr.Logger, syncState *dvSyncState, transferName string) (reconcile.Result, error) {
	//DO Nothing, not yet ready
	if syncState.pvc.Annotations[cc.AnnCloneOf] != "true" {
		return reconcile.Result{}, nil
	}

	// expand for non-namespace case
	return r.expandPvcAfterClone(log, syncState, syncState.pvc, cdiv1.Succeeded, false)
}

func (r *CloneReconcilerBase) setCloneOfOnPvc(pvc *corev1.PersistentVolumeClaim) error {
	if v, ok := pvc.Annotations[cc.AnnCloneOf]; !ok || v != "true" {
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations[cc.AnnCloneOf] = "true"

		return r.updatePVC(pvc)
	}

	return nil
}

// temporary pvc is used when the clone src and tgt are in two distinct namespaces
func (r *CloneReconcilerBase) expandPvcAfterClone(
	log logr.Logger,
	syncState *dvSyncState,
	pvc *corev1.PersistentVolumeClaim,
	nextPhase cdiv1.DataVolumePhase,
	isTemp bool) (reconcile.Result, error) {

	done, err := r.expand(log, syncState, pvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !done {
		return reconcile.Result{Requeue: true},
			r.syncCloneStatusPhase(syncState, cdiv1.ExpansionInProgress, pvc)
	}

	if isTemp {
		// trigger transfer and next reconcile should have pvcExists == true
		pvc.Annotations[annReadyForTransfer] = "true"
		pvc.Annotations[cc.AnnPopulatedFor] = syncState.dvMutated.Name
		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, r.syncCloneStatusPhase(syncState, nextPhase, pvc)
}

func (r *CloneReconcilerBase) doCrossNamespaceClone(log logr.Logger,
	syncState *dvSyncState,
	pvcName, sourceNamespace string,
	returnWhenCloneInProgress bool) (*reconcile.Result, error) {

	initialized, err := r.initTransfer(log, syncState, pvcName, sourceNamespace)
	if err != nil {
		return &reconcile.Result{}, err
	}

	// get reconciled again v soon
	if !initialized {
		return &reconcile.Result{}, r.syncCloneStatusPhase(syncState, cdiv1.CloneScheduled, nil)
	}

	tmpPVC := &corev1.PersistentVolumeClaim{}
	nn := types.NamespacedName{Namespace: sourceNamespace, Name: pvcName}
	if err := r.client.Get(context.TODO(), nn, tmpPVC); err != nil {
		if !k8serrors.IsNotFound(err) {
			return &reconcile.Result{}, err
		}
	} else if tmpPVC.Annotations[cc.AnnCloneOf] == "true" {
		result, err := r.expandPvcAfterClone(log, syncState, tmpPVC, cdiv1.NamespaceTransferInProgress, true)
		return &result, err
	} else {
		// AnnCloneOf != true, so cloneInProgress
		if returnWhenCloneInProgress {
			return &reconcile.Result{}, nil
		}
	}

	return nil, nil
}

func (r *CloneReconcilerBase) initTransfer(log logr.Logger, syncState *dvSyncState, name, namespace string) (bool, error) {
	initialized := true
	dv := syncState.dvMutated

	log.Info("Initializing transfer")

	if !cc.HasFinalizer(dv, crossNamespaceFinalizer) {
		cc.AddFinalizer(dv, crossNamespaceFinalizer)
		initialized = false
	}

	ot := &cdiv1.ObjectTransfer{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name}, ot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		if err := cc.ValidateCloneTokenDV(r.tokenValidator, dv); err != nil {
			return false, err
		}

		ot = &cdiv1.ObjectTransfer{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					common.CDILabelKey:       common.CDILabelValue,
					common.CDIComponentLabel: "",
				},
			},
			Spec: cdiv1.ObjectTransferSpec{
				Source: cdiv1.TransferSource{
					Kind:      "PersistentVolumeClaim",
					Namespace: namespace,
					Name:      name,
					RequiredAnnotations: map[string]string{
						annReadyForTransfer: "true",
					},
				},
				Target: cdiv1.TransferTarget{
					Namespace: &dv.Namespace,
					Name:      &dv.Name,
				},
			},
		}
		util.SetRecommendedLabels(ot, r.installerLabels, "cdi-controller")

		if err := setAnnOwnedByDataVolume(ot, dv); err != nil {
			return false, err
		}

		if err := r.client.Create(context.TODO(), ot); err != nil {
			return false, err
		}

		initialized = false
	}

	return initialized, nil
}

func expansionPodName(pvc *corev1.PersistentVolumeClaim) string {
	return "cdi-expand-" + string(pvc.UID)
}

func (r *CloneReconcilerBase) expand(log logr.Logger, syncState *dvSyncState, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if pvc.Status.Phase != corev1.ClaimBound {
		return false, fmt.Errorf("cannot expand volume in %q phase", pvc.Status.Phase)
	}

	requestedSize, hasRequested := syncState.pvcSpec.Resources.Requests[corev1.ResourceStorage]
	currentSize, hasCurrent := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	actualSize, hasActual := pvc.Status.Capacity[corev1.ResourceStorage]
	if !hasRequested || !hasCurrent || !hasActual {
		return false, fmt.Errorf("PVC sizes missing")
	}

	expansionRequired := actualSize.Cmp(requestedSize) < 0
	updateRequestSizeRequired := currentSize.Cmp(requestedSize) < 0

	log.V(3).Info("Expand sizes", "req", requestedSize, "cur", currentSize, "act", actualSize, "exp", expansionRequired)

	if updateRequestSizeRequired {
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = requestedSize
		if err := r.updatePVC(pvc); err != nil {
			return false, err
		}

		return false, nil
	}

	podName := expansionPodName(pvc)
	podExists := true
	pod := &corev1.Pod{}
	nn := types.NamespacedName{Namespace: pvc.Namespace, Name: podName}
	if err := r.client.Get(context.TODO(), nn, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		podExists = false
	}

	if !expansionRequired && !podExists {
		// finally done
		return true, nil
	}

	hasPendingResizeCondition := false
	for _, cond := range pvc.Status.Conditions {
		if cond.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			hasPendingResizeCondition = true
			break
		}
	}

	if !podExists && !hasPendingResizeCondition {
		// wait for resize condition
		return false, nil
	}

	if !podExists {
		var err error
		pod, err = r.createExpansionPod(pvc, syncState.dvMutated, podName)
		// Check if pod has failed and, in that case, record an event with the error
		if podErr := cc.HandleFailedPod(err, podName, pvc, r.recorder, r.client); podErr != nil {
			return false, podErr
		}
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		if err := r.client.Delete(context.TODO(), pod); err != nil {
			if k8serrors.IsNotFound(err) {
				return true, err
			}

			return false, err
		}

		return false, nil
	}

	return false, nil
}

func (r *CloneReconcilerBase) createExpansionPod(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume, podName string) (*corev1.Pod, error) {
	resourceRequirements, err := cc.GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	imagePullSecrets, err := cc.GetImagePullSecrets(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(context.TODO(), r.client)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "cdi-expander",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "dummy",
					Image:           r.clonerImage,
					ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
					Command:         []string{"/bin/bash"},
					Args:            []string{"-c", "echo", "'hello cdi'"},
				},
			},
			ImagePullSecrets: imagePullSecrets,
			RestartPolicy:    corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: cc.DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
			NodeSelector: workloadNodePlacement.NodeSelector,
			Tolerations:  workloadNodePlacement.Tolerations,
			Affinity:     workloadNodePlacement.Affinity,
		},
	}
	util.SetRecommendedLabels(pod, r.installerLabels, "cdi-controller")

	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = cc.AddVolumeDevices()
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      cc.DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	if err := setAnnOwnedByDataVolume(pod, dv); err != nil {
		return nil, err
	}

	cc.SetRestrictedSecurityContext(&pod.Spec)

	if err := r.client.Create(context.TODO(), pod); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	return pod, nil
}

func (r *CloneReconcilerBase) syncCloneStatusPhase(syncState *dvSyncState, phase cdiv1.DataVolumePhase, pvc *corev1.PersistentVolumeClaim) error {
	var event Event
	dataVolume := syncState.dvMutated
	sourceName, sourceNamespace := cc.GetCloneSourceNameAndNamespace(dataVolume)

	switch phase {
	case cdiv1.CloneScheduled:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneScheduled
		event.message = fmt.Sprintf(MessageCloneScheduled, sourceNamespace, sourceName, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.SnapshotForSmartCloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = SnapshotForSmartCloneInProgress
		event.message = fmt.Sprintf(MessageSmartCloneInProgress, sourceNamespace, sourceName)
	case cdiv1.CloneFromSnapshotSourceInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneFromSnapshotSourceInProgress
		event.message = fmt.Sprintf(MessageCloneFromSnapshotSourceInProgress, sourceNamespace, sourceName)
	case cdiv1.CSICloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = string(cdiv1.CSICloneInProgress)
		event.message = fmt.Sprintf(MessageCsiCloneInProgress, sourceNamespace, sourceName)
	case cdiv1.ExpansionInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = ExpansionInProgress
		event.message = fmt.Sprintf(MessageExpansionInProgress, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.NamespaceTransferInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = NamespaceTransferInProgress
		event.message = fmt.Sprintf(MessageNamespaceTransferInProgress, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.Succeeded:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, sourceNamespace, sourceName, dataVolume.Namespace, dataVolume.Name)
	}

	return r.syncDataVolumeStatusPhaseWithEvent(syncState, phase, pvc, event)
}

func (r *CloneReconcilerBase) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	phase, ok := pvc.Annotations[cc.AnnPodPhase]
	if phase != string(corev1.PodSucceeded) {
		_, ok = pvc.Annotations[cc.AnnCloneRequest]
		if !ok || pvc.Status.Phase != corev1.ClaimBound || pvcIsPopulated(pvc, dataVolumeCopy) {
			return nil
		}
		dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
	}
	if !ok {
		return nil
	}

	if err := r.populateSourceIfSourceRef(dataVolumeCopy); err != nil {
		return err
	}
	sourceName, sourceNamespace := cc.GetCloneSourceNameAndNamespace(dataVolumeCopy)

	switch phase {
	case string(corev1.PodPending):
		dataVolumeCopy.Status.Phase = cdiv1.CloneScheduled
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneScheduled
		event.message = fmt.Sprintf(MessageCloneScheduled, sourceNamespace, sourceName, pvc.Namespace, pvc.Name)
	case string(corev1.PodRunning):
		dataVolumeCopy.Status.Phase = cdiv1.CloneInProgress
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneInProgress
		event.message = fmt.Sprintf(MessageCloneInProgress, sourceNamespace, sourceName, pvc.Namespace, pvc.Name)
	case string(corev1.PodFailed):
		event.eventType = corev1.EventTypeWarning
		event.reason = CloneFailed
		event.message = fmt.Sprintf(MessageCloneFailed, sourceNamespace, sourceName, pvc.Namespace, pvc.Name)
	case string(corev1.PodSucceeded):
		dataVolumeCopy.Status.Phase = cdiv1.Succeeded
		dataVolumeCopy.Status.Progress = cdiv1.DataVolumeProgress("100.0%")
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, sourceNamespace, sourceName, pvc.Namespace, pvc.Name)
	}
	return nil
}

// If SourceRef is set, populate spec.Source with data from the DataSource
// Note that when the controller actually updates the DV (updateDataVolume), we nil out spec.Source when SourceRef is set
func (r *CloneReconcilerBase) populateSourceIfSourceRef(dv *cdiv1.DataVolume) error {
	if dv.Spec.SourceRef == nil {
		return nil
	}
	if dv.Spec.SourceRef.Kind != cdiv1.DataVolumeDataSource {
		return errors.Errorf("Unsupported sourceRef kind %s, currently only %s is supported", dv.Spec.SourceRef.Kind, cdiv1.DataVolumeDataSource)
	}
	ns := dv.Namespace
	if dv.Spec.SourceRef.Namespace != nil && *dv.Spec.SourceRef.Namespace != "" {
		ns = *dv.Spec.SourceRef.Namespace
	}
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: dv.Spec.SourceRef.Name, Namespace: ns}, dataSource); err != nil {
		return err
	}
	dv.Spec.Source = &cdiv1.DataVolumeSource{
		PVC:      dataSource.Spec.Source.PVC,
		Snapshot: dataSource.Spec.Source.Snapshot,
	}
	return nil
}

func (r *CloneReconcilerBase) cleanupTransfer(dv *cdiv1.DataVolume) error {
	transferName := getTransferName(dv)
	if !cc.HasFinalizer(dv, crossNamespaceFinalizer) {
		return nil
	}

	r.log.V(1).Info("Doing cleanup of transfer")

	if dv.DeletionTimestamp != nil && dv.Status.Phase != cdiv1.Succeeded {
		// delete all potential PVCs that may not have owner refs
		namespaces := []string{dv.Namespace}
		names := []string{dv.Name}
		namespaces, names = appendTmpPvcIfNeeded(dv, namespaces, names, transferName)

		for i := range namespaces {
			pvc := &corev1.PersistentVolumeClaim{}
			nn := types.NamespacedName{Namespace: namespaces[i], Name: names[i]}
			if err := r.client.Get(context.TODO(), nn, pvc); err != nil {
				if !k8serrors.IsNotFound(err) {
					return err
				}
			} else {
				pod := &corev1.Pod{}
				nn := types.NamespacedName{Namespace: namespaces[i], Name: expansionPodName(pvc)}
				if err := r.client.Get(context.TODO(), nn, pod); err != nil {
					if !k8serrors.IsNotFound(err) {
						return err
					}
				} else {
					if err := r.client.Delete(context.TODO(), pod); err != nil {
						if !k8serrors.IsNotFound(err) {
							return err
						}
					}
				}

				if err := r.client.Delete(context.TODO(), pvc); err != nil {
					if !k8serrors.IsNotFound(err) {
						return err
					}
				}
			}
		}
	}

	ot := &cdiv1.ObjectTransfer{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: transferName}, ot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		if ot.DeletionTimestamp == nil {
			if err := r.client.Delete(context.TODO(), ot); err != nil {
				if !k8serrors.IsNotFound(err) {
					return err
				}
			}
		}
		return fmt.Errorf("waiting for ObjectTransfer %s to delete", transferName)
	}

	cc.RemoveFinalizer(dv, crossNamespaceFinalizer)
	return nil
}

func appendTmpPvcIfNeeded(dv *cdiv1.DataVolume, namespaces, names []string, pvcName string) ([]string, []string) {
	_, sourceNamespace := cc.GetCloneSourceNameAndNamespace(dv)

	if sourceNamespace != "" && sourceNamespace != dv.Namespace {
		namespaces = append(namespaces, sourceNamespace)
		names = append(names, pvcName)
	}

	return namespaces, names
}

func isCrossNamespaceClone(dv *cdiv1.DataVolume) bool {
	_, sourceNamespace := cc.GetCloneSourceNameAndNamespace(dv)

	return sourceNamespace != "" && sourceNamespace != dv.Namespace
}

func getTransferName(dv *cdiv1.DataVolume) string {
	return fmt.Sprintf("cdi-tmp-%s", dv.UID)
}

// addCloneWithoutSourceWatch reconciles clones created without source once the matching PVC is created
func addCloneWithoutSourceWatch(mgr manager.Manager, datavolumeController controller.Controller, typeToWatch client.Object, indexingKey string) error {
	getKey := func(namespace, name string) string {
		return namespace + "/" + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, indexingKey, func(obj client.Object) []string {
		dv := obj.(*cdiv1.DataVolume)
		if source := dv.Spec.Source; source != nil {
			sourceName, sourceNamespace := cc.GetCloneSourceNameAndNamespace(dv)
			if sourceName != "" {
				ns := cc.GetNamespace(sourceNamespace, obj.GetNamespace())
				return []string{getKey(ns, sourceName)}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Function to reconcile DVs that match the selected fields
	dataVolumeMapper := func(obj client.Object) (reqs []reconcile.Request) {
		dvList := &cdiv1.DataVolumeList{}
		namespacedName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
		matchingFields := client.MatchingFields{indexingKey: namespacedName.String()}
		if err := mgr.GetClient().List(context.TODO(), dvList, matchingFields); err != nil {
			return
		}
		for _, dv := range dvList.Items {
			op := getDataVolumeOp(mgr.GetLogger(), &dv, mgr.GetClient())
			if op == dataVolumePvcClone || op == dataVolumeSnapshotClone {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name}})
			}
		}
		return
	}

	if err := datavolumeController.Watch(&source.Kind{Type: typeToWatch},
		handler.EnqueueRequestsFromMapFunc(dataVolumeMapper),
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return true },
			DeleteFunc: func(e event.DeleteEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool { return false },
		}); err != nil {
		return err
	}

	return nil
}
