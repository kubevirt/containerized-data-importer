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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller/clone"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
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
	// CSICloneInProgress provides a const to indicate  csi volume clone is in progress
	CSICloneInProgress = "CSICloneInProgress"
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
	MessageCloneFromSnapshotSourceInProgress = "Creating PVC from snapshot source is in progress (for %s %s/%s)"
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
	// PrepClaimInProgress is const representing target PVC prep
	PrepClaimInProgress = "PrepClaimInProgress"
	// MessagePrepClaimInProgress is a const for reporting target prep
	MessagePrepClaimInProgress = "Prepping PersistentVolumeClaim for DataVolume %s/%s"
	// RebindInProgress is const representing target PVC rebind
	RebindInProgress = "RebindInProgress"
	// MessageRebindInProgress is a const for reporting target rebind
	MessageRebindInProgress = "Rebinding PersistentVolumeClaim for DataVolume %s/%s"

	// AnnCSICloneRequest annotation associates object with CSI Clone Request
	AnnCSICloneRequest = "cdi.kubevirt.io/CSICloneRequest"

	// AnnVirtualImageSize annotation contains the Virtual Image size of a PVC used for host-assisted cloning
	AnnVirtualImageSize = "cdi.Kubervirt.io/virtualSize"

	// AnnSourceCapacity annotation contains the storage capacity of a PVC used for host-assisted cloning
	AnnSourceCapacity = "cdi.Kubervirt.io/sourceCapacity"

	crossNamespaceFinalizer = "cdi.kubevirt.io/dataVolumeFinalizer"
)

// CloneReconcilerBase members
type CloneReconcilerBase struct {
	ReconcilerBase
	clonerImage         string
	importerImage       string
	pullPolicy          string
	cloneSourceAPIGroup *string
	cloneSourceKind     string
	shortTokenValidator token.Validator
	longTokenValidator  token.Validator
	tokenGenerator      token.Generator
}

func (r *CloneReconcilerBase) addVolumeCloneSourceWatch(datavolumeController controller.Controller) error {
	return datavolumeController.Watch(&source.Kind{Type: &cdiv1.VolumeCloneSource{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			var err error
			var hasDataVolumeOwner bool
			var ownerNamespace, ownerName string
			ownerRef := metav1.GetControllerOf(obj)
			if ownerRef != nil && ownerRef.Kind == "DataVolume" {
				hasDataVolumeOwner = true
				ownerNamespace = obj.GetNamespace()
				ownerName = ownerRef.Name
			} else if hasAnnOwnedByDataVolume(obj) {
				hasDataVolumeOwner = true
				ownerNamespace, ownerName, err = getAnnOwnedByDataVolume(obj)
				if err != nil {
					return nil
				}
			}
			if !hasDataVolumeOwner {
				return nil
			}
			dv := &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ownerNamespace,
					Name:      ownerName,
				},
			}
			if err = r.client.Get(context.TODO(), client.ObjectKeyFromObject(dv), dv); err != nil {
				r.log.Info("Failed to get DataVolume", "error", err)
				return nil
			}
			if err := r.populateSourceIfSourceRef(dv); err != nil {
				r.log.Info("Failed to check DataSource", "error", err)
				return nil
			}
			if (r.cloneSourceKind == "PersistentVolumeClaim" && dv.Spec.Source.PVC != nil) ||
				(r.cloneSourceKind == "VolumeSnapshot" && dv.Spec.Source.Snapshot != nil) {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: ownerNamespace, Name: ownerName}}}
			}
			return nil
		}),
	)
}

func (r *CloneReconcilerBase) updatePVCForPopulation(dataVolume *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if dataVolume.Spec.Source.PVC == nil && dataVolume.Spec.Source.Snapshot == nil {
		return errors.Errorf("no source set for clone datavolume")
	}
	if err := addCloneToken(dataVolume, pvc); err != nil {
		return err
	}
	if err := cc.AddImmediateBindingAnnotationIfWFFCDisabled(pvc, r.featureGates); err != nil {
		return err
	}
	if isCrossNamespaceClone(dataVolume) {
		_, _, sourcNamespace := cc.GetCloneSourceInfo(dataVolume)
		cc.AddAnnotation(pvc, populators.AnnDataSourceNamespace, sourcNamespace)
	}
	apiGroup := cc.AnnAPIGroup
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     cdiv1.VolumeCloneSourceRef,
		Name:     volumeCloneSourceName(dataVolume),
	}
	return nil
}

func (r *CloneReconcilerBase) ensureExtendedTokenDV(dv *cdiv1.DataVolume) (bool, error) {
	if !isCrossNamespaceClone(dv) {
		return false, nil
	}

	_, ok := dv.Annotations[cc.AnnExtendedCloneToken]
	if ok {
		return false, nil
	}

	token, ok := dv.Annotations[cc.AnnCloneToken]
	if !ok {
		return false, fmt.Errorf("token missing")
	}

	payload, err := r.shortTokenValidator.Validate(token)
	if err != nil {
		return false, err
	}

	if payload.Params == nil {
		payload.Params = make(map[string]string)
	}
	payload.Params["uid"] = string(dv.UID)

	newToken, err := r.tokenGenerator.Generate(payload)
	if err != nil {
		return false, err
	}

	dv.Annotations[cc.AnnExtendedCloneToken] = newToken

	return true, nil
}

func (r *CloneReconcilerBase) ensureExtendedTokenPVC(dv *cdiv1.DataVolume, pvc *corev1.PersistentVolumeClaim) error {
	if !isCrossNamespaceClone(dv) {
		return nil
	}

	_, ok := pvc.Annotations[cc.AnnExtendedCloneToken]
	if ok {
		return nil
	}

	token, ok := dv.Annotations[cc.AnnExtendedCloneToken]
	if !ok {
		return fmt.Errorf("token missing")
	}

	payload, err := r.longTokenValidator.Validate(token)
	if err != nil {
		return err
	}

	if payload.Params["uid"] != string(dv.UID) {
		return fmt.Errorf("token uid mismatch")
	}

	// now use pvc uid
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

func (r *CloneReconcilerBase) reconcileVolumeCloneSourceCR(syncState *dvSyncState) error {
	dv := syncState.dvMutated
	volumeCloneSource := &cdiv1.VolumeCloneSource{}
	volumeCloneSourceName := volumeCloneSourceName(dv)
	_, sourceName, sourceNamespace := cc.GetCloneSourceInfo(dv)
	deletedOrSucceeded := dv.DeletionTimestamp != nil || dv.Status.Phase == cdiv1.Succeeded
	exists, err := cc.GetResource(context.TODO(), r.client, sourceNamespace, volumeCloneSourceName, volumeCloneSource)
	if err != nil {
		return err
	}

	if deletedOrSucceeded || exists {
		if deletedOrSucceeded && exists {
			if err := r.client.Delete(context.TODO(), volumeCloneSource); err != nil {
				if !k8serrors.IsNotFound(err) {
					return err
				}
			}
		}

		if deletedOrSucceeded {
			cc.RemoveFinalizer(dv, crossNamespaceFinalizer)
		}

		return nil
	}

	volumeCloneSource = &cdiv1.VolumeCloneSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeCloneSourceName,
			Namespace: sourceNamespace,
		},
		Spec: cdiv1.VolumeCloneSourceSpec{
			Source: corev1.TypedLocalObjectReference{
				APIGroup: r.cloneSourceAPIGroup,
				Kind:     r.cloneSourceKind,
				Name:     sourceName,
			},
			Preallocation: dv.Spec.Preallocation,
		},
	}

	if dv.Spec.PriorityClassName != "" {
		volumeCloneSource.Spec.PriorityClassName = &dv.Spec.PriorityClassName
	}

	if sourceNamespace == dv.Namespace {
		if err := controllerutil.SetControllerReference(dv, volumeCloneSource, r.scheme); err != nil {
			return err
		}
	} else {
		if err := setAnnOwnedByDataVolume(volumeCloneSource, dv); err != nil {
			return err
		}
	}

	if err := r.client.Create(context.TODO(), volumeCloneSource); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (r *CloneReconcilerBase) syncCloneStatusPhase(syncState *dvSyncState, phase cdiv1.DataVolumePhase, pvc *corev1.PersistentVolumeClaim) error {
	var event Event
	dataVolume := syncState.dvMutated
	r.setEventForPhase(dataVolume, phase, &event)
	return r.syncDataVolumeStatusPhaseWithEvent(syncState, phase, pvc, event)
}

func (r *CloneReconcilerBase) setEventForPhase(dataVolume *cdiv1.DataVolume, phase cdiv1.DataVolumePhase, event *Event) {
	sourceType, sourceName, sourceNamespace := cc.GetCloneSourceInfo(dataVolume)
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
		event.message = fmt.Sprintf(MessageCloneFromSnapshotSourceInProgress, sourceType, sourceNamespace, sourceName)
	case cdiv1.CSICloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = CSICloneInProgress
		event.message = fmt.Sprintf(MessageCsiCloneInProgress, sourceNamespace, sourceName)
	case cdiv1.Succeeded:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneSucceeded
		event.message = fmt.Sprintf(MessageCloneSucceeded, sourceNamespace, sourceName, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.CloneInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = CloneInProgress
		event.message = fmt.Sprintf(MessageCloneInProgress, sourceNamespace, sourceName, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.PrepClaimInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = PrepClaimInProgress
		event.message = fmt.Sprintf(MessagePrepClaimInProgress, dataVolume.Namespace, dataVolume.Name)
	case cdiv1.RebindInProgress:
		event.eventType = corev1.EventTypeNormal
		event.reason = RebindInProgress
		event.message = fmt.Sprintf(MessageRebindInProgress, dataVolume.Namespace, dataVolume.Name)
	default:
		r.log.V(3).Info("No event set for phase", "phase", phase)
	}
}

var populatorPhaseMap = map[string]cdiv1.DataVolumePhase{
	"":                           cdiv1.CloneScheduled,
	clone.PendingPhaseName:       cdiv1.CloneScheduled,
	clone.SucceededPhaseName:     cdiv1.Succeeded,
	clone.CSIClonePhaseName:      cdiv1.CSICloneInProgress,
	clone.HostClonePhaseName:     cdiv1.CloneInProgress,
	clone.PrepClaimPhaseName:     cdiv1.PrepClaimInProgress,
	clone.RebindPhaseName:        cdiv1.RebindInProgress,
	clone.SnapshotClonePhaseName: cdiv1.CloneFromSnapshotSourceInProgress,
	clone.SnapshotPhaseName:      cdiv1.SnapshotForSmartCloneInProgress,
	//clone.ErrorPhaseName:         cdiv1.Error, // Want to hold off on this for now
}

func (r *CloneReconcilerBase) updateStatusPhaseForPopulator(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	popPhase := pvc.Annotations[populators.AnnClonePhase]
	dvPhase, ok := populatorPhaseMap[popPhase]
	if !ok {
		r.log.V(1).Info("Unknown populator phase", "phase", popPhase)
		//dataVolumeCopy.Status.Phase = cdiv1.Unknown // hold off on this for now
		return nil
	}
	dataVolumeCopy.Status.Phase = dvPhase
	r.setEventForPhase(dataVolumeCopy, dvPhase, event)
	return nil
}

func (r *CloneReconcilerBase) updateStatusPhase(pvc *corev1.PersistentVolumeClaim, dataVolumeCopy *cdiv1.DataVolume, event *Event) error {
	if err := r.populateSourceIfSourceRef(dataVolumeCopy); err != nil {
		return err
	}
	_, sourceName, sourceNamespace := cc.GetCloneSourceInfo(dataVolumeCopy)

	usePopulator, err := CheckPVCUsingPopulators(pvc)
	if err != nil {
		return err
	}
	if usePopulator {
		return r.updateStatusPhaseForPopulator(pvc, dataVolumeCopy, event)
	}

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

func isCrossNamespaceClone(dv *cdiv1.DataVolume) bool {
	_, _, sourceNamespace := cc.GetCloneSourceInfo(dv)

	return sourceNamespace != "" && sourceNamespace != dv.Namespace
}

// addCloneWithoutSourceWatch reconciles clones created without source once the matching PVC is created
func addCloneWithoutSourceWatch(mgr manager.Manager, datavolumeController controller.Controller, typeToWatch client.Object, indexingKey string) error {
	getKey := func(namespace, name string) string {
		return namespace + "/" + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataVolume{}, indexingKey, func(obj client.Object) []string {
		dv := obj.(*cdiv1.DataVolume)
		if source := dv.Spec.Source; source != nil {
			_, sourceName, sourceNamespace := cc.GetCloneSourceInfo(dv)
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
