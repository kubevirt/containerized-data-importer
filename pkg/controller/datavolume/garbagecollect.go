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
	"time"

	"github.com/go-logr/logr"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

func (r *ReconcilerBase) garbageCollect(syncState *dvSyncState, log logr.Logger) error {
	dataVolume := syncState.dv
	if dataVolume.Status.Phase != cdiv1.Succeeded {
		return nil
	}
	cdiConfig := &cdiv1.CDIConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		return err
	}
	dvTTL := cc.GetDataVolumeTTLSeconds(cdiConfig)
	if dvTTL < 0 {
		log.Info("Garbage Collection is disabled")
		return nil
	}
	if allowed, err := r.isGarbageCollectionAllowed(dataVolume, log); !allowed || err != nil {
		return err
	}
	// Current DV still has TTL, so reconcile will return with the needed RequeueAfter
	if delta := getDeltaTTL(dataVolume, dvTTL); delta > 0 {
		syncState.result = &reconcile.Result{RequeueAfter: delta}
		return nil
	}
	if err := r.detachPvcDeleteDv(syncState); err != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) isGarbageCollectionAllowed(dv *cdiv1.DataVolume, log logr.Logger) (bool, error) {
	dvDelete := dv.Annotations[cc.AnnDeleteAfterCompletion]
	if dvDelete != "true" {
		log.Info("DataVolume is not annotated to be garbage collected")
		return false, nil
	}
	for _, ref := range dv.OwnerReferences {
		if ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
			continue
		}
		if allowed, err := r.canUpdateFinalizers(ref); !allowed || err != nil {
			log.Info("DataVolume cannot be garbage collected, due to owner finalizers update RBAC")
			return false, err
		}
	}
	return true, nil
}

func (r *ReconcilerBase) canUpdateFinalizers(ownerRef metav1.OwnerReference) (bool, error) {
	gvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
	mapping, err := r.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return false, err
	}
	ssar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:       gvk.Group,
				Version:     gvk.Version,
				Resource:    mapping.Resource.Resource,
				Subresource: "finalizers",
				Verb:        "update",
			},
		},
	}
	if err := r.client.Create(context.TODO(), ssar); err != nil {
		return false, err
	}
	return ssar.Status.Allowed, nil
}

func (r *ReconcilerBase) detachPvcDeleteDv(syncState *dvSyncState) error {
	updatePvcOwnerRefs(syncState.pvc, syncState.dv)
	delete(syncState.pvc.Annotations, cc.AnnPopulatedFor)
	cc.AddAnnotation(syncState.pvc, cc.AnnGarbageCollected, "true")
	if err := r.updatePVC(syncState.pvc); err != nil {
		return err
	}
	if err := r.client.Delete(context.TODO(), syncState.dv); err != nil {
		return err
	}
	syncState.result = &reconcile.Result{}
	syncState.dv = nil
	syncState.dvMutated = nil
	return nil
}

func updatePvcOwnerRefs(pvc *corev1.PersistentVolumeClaim, dv *cdiv1.DataVolume) {
	refs := pvc.OwnerReferences
	if i := findOwnerRef(refs, dv.UID); i >= 0 {
		refs = append(refs[:i], refs[i+1:]...)
	}
	for _, ref := range dv.OwnerReferences {
		if findOwnerRef(refs, ref.UID) == -1 {
			refs = append(refs, ref)
		}
	}
	pvc.OwnerReferences = refs
}

func findOwnerRef(refs []metav1.OwnerReference, uid types.UID) int {
	for i, ref := range refs {
		if ref.UID == uid {
			return i
		}
	}
	return -1
}

func getDeltaTTL(dv *cdiv1.DataVolume, ttl int32) time.Duration {
	delta := time.Second * time.Duration(ttl)
	if cond := FindConditionByType(cdiv1.DataVolumeReady, dv.Status.Conditions); cond != nil {
		delta -= time.Since(cond.LastTransitionTime.Time)
	}
	return delta
}
