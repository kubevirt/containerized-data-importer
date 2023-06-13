/*
Copyright 2020 The CDI Authors.

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
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	storagehelpers "k8s.io/component-helpers/storage/volume"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

const (
	// AnnOwnedByDataVolume annotation has the owner DataVolume name
	AnnOwnedByDataVolume = "cdi.kubevirt.io/ownedByDataVolume"
)

// renderPvcSpec creates a new PVC Spec based on either the dv.spec.pvc or dv.spec.storage section
func renderPvcSpec(client client.Client, recorder record.EventRecorder, log logr.Logger, dv *cdiv1.DataVolume, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaimSpec, error) {
	if dv.Spec.PVC != nil {
		return dv.Spec.PVC.DeepCopy(), nil
	} else if dv.Spec.Storage != nil {
		return pvcFromStorage(client, recorder, log, dv, pvc)
	}

	return nil, errors.Errorf("datavolume one of {pvc, storage} field is required")
}

func pvcFromStorage(client client.Client, recorder record.EventRecorder, log logr.Logger, dv *cdiv1.DataVolume, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaimSpec, error) {
	var pvcSpec *v1.PersistentVolumeClaimSpec

	if pvc == nil {
		pvcSpec = copyStorageAsPvc(log, dv.Spec.Storage)
		if err := renderPvcSpecVolumeModeAndAccessModes(client, recorder, log, dv, pvcSpec); err != nil {
			return nil, err
		}
	} else {
		pvcSpec = pvc.Spec.DeepCopy()
	}

	if err := renderPvcSpecVolumeSize(client, dv.Spec, pvcSpec); err != nil {
		return nil, err
	}

	return pvcSpec, nil
}

func renderPvcSpecVolumeModeAndAccessModes(client client.Client, recorder record.EventRecorder, log logr.Logger, dv *cdiv1.DataVolume, pvcSpec *v1.PersistentVolumeClaimSpec) error {
	if dv.Spec.ContentType == cdiv1.DataVolumeArchive {
		if pvcSpec.VolumeMode != nil && *pvcSpec.VolumeMode == v1.PersistentVolumeBlock {
			log.V(1).Info("DataVolume with ContentType Archive cannot have block volumeMode", "namespace", dv.Namespace, "name", dv.Name)
			recorder.Eventf(dv, v1.EventTypeWarning, cc.ErrClaimNotValid, "DataVolume with ContentType Archive cannot have block volumeMode")
			return errors.Errorf("DataVolume with ContentType Archive cannot have block volumeMode")
		}
		volumeMode := v1.PersistentVolumeFilesystem
		pvcSpec.VolumeMode = &volumeMode
	}

	storageClass, err := cc.GetStorageClassByName(context.TODO(), client, dv.Spec.Storage.StorageClassName)
	if err != nil {
		return err
	}
	if storageClass == nil {
		if err := renderPvcSpecFromAvailablePv(client, pvcSpec); err != nil {
			return err
		}
		// Not even default storageClass on the cluster, cannot apply the defaults, verify spec is ok
		if len(pvcSpec.AccessModes) == 0 {
			log.V(1).Info("Cannot set accessMode for new pvc", "namespace", dv.Namespace, "name", dv.Name)
			recorder.Eventf(dv, v1.EventTypeWarning, cc.ErrClaimNotValid, "DataVolume.storage spec is missing accessMode and no storageClass to choose profile")
			return errors.Errorf("DataVolume spec is missing accessMode")
		}
		return nil
	}

	pvcSpec.StorageClassName = &storageClass.Name
	// given storageClass we can apply defaults if needed
	if (pvcSpec.VolumeMode == nil || *pvcSpec.VolumeMode == "") && (len(pvcSpec.AccessModes) == 0) {
		accessModes, volumeMode, err := getDefaultVolumeAndAccessMode(client, storageClass)
		if err != nil {
			log.V(1).Info("Cannot set accessMode and volumeMode for new pvc", "namespace", dv.Namespace, "name", dv.Name, "Error", err)
			recorder.Eventf(dv, v1.EventTypeWarning, cc.ErrClaimNotValid,
				fmt.Sprintf("DataVolume.storage spec is missing accessMode and volumeMode, cannot get access mode from StorageProfile %s", getName(storageClass)))
			return err
		}
		pvcSpec.AccessModes = append(pvcSpec.AccessModes, accessModes...)
		pvcSpec.VolumeMode = volumeMode
	} else if len(pvcSpec.AccessModes) == 0 {
		accessModes, err := getDefaultAccessModes(client, storageClass, pvcSpec.VolumeMode)
		if err != nil {
			log.V(1).Info("Cannot set accessMode for new pvc", "namespace", dv.Namespace, "name", dv.Name, "Error", err)
			recorder.Eventf(dv, v1.EventTypeWarning, cc.ErrClaimNotValid,
				fmt.Sprintf("DataVolume.storage spec is missing accessMode and cannot get access mode from StorageProfile %s", getName(storageClass)))
			return err
		}
		pvcSpec.AccessModes = append(pvcSpec.AccessModes, accessModes...)
	} else if pvcSpec.VolumeMode == nil || *pvcSpec.VolumeMode == "" {
		volumeMode, err := getDefaultVolumeMode(client, storageClass, pvcSpec.AccessModes)
		if err != nil {
			return err
		}
		pvcSpec.VolumeMode = volumeMode
	}

	return nil
}

func renderPvcSpecVolumeSize(client client.Client, dvSpec cdiv1.DataVolumeSpec, pvcSpec *v1.PersistentVolumeClaimSpec) error {
	requestedVolumeSize, err := resolveVolumeSize(client, dvSpec, pvcSpec)
	if err != nil {
		return err
	}
	if pvcSpec.Resources.Requests == nil {
		pvcSpec.Resources.Requests = v1.ResourceList{}
	}
	pvcSpec.Resources.Requests[v1.ResourceStorage] = *requestedVolumeSize
	return nil
}

func getName(storageClass *storagev1.StorageClass) string {
	if storageClass != nil {
		return storageClass.Name
	}
	return ""
}

func copyStorageAsPvc(log logr.Logger, storage *cdiv1.StorageSpec) *v1.PersistentVolumeClaimSpec {
	input := storage.DeepCopy()
	pvcSpec := &v1.PersistentVolumeClaimSpec{
		AccessModes:      input.AccessModes,
		Selector:         input.Selector,
		Resources:        input.Resources,
		VolumeName:       input.VolumeName,
		StorageClassName: input.StorageClassName,
		VolumeMode:       input.VolumeMode,
		DataSource:       input.DataSource,
		DataSourceRef:    input.DataSourceRef,
	}

	return pvcSpec
}

// Renders the PVC spec VolumeMode and AccessModes from an available satisfying PV
func renderPvcSpecFromAvailablePv(c client.Client, pvcSpec *v1.PersistentVolumeClaimSpec) error {
	if pvcSpec.StorageClassName == nil {
		return nil
	}

	pvList := &v1.PersistentVolumeList{}
	fields := client.MatchingFields{claimStorageClassNameField: *pvcSpec.StorageClassName}
	if err := c.List(context.TODO(), pvList, fields); err != nil {
		return err
	}
	for _, pv := range pvList.Items {
		if pv.Status.Phase == v1.VolumeAvailable {
			pvc := &v1.PersistentVolumeClaim{Spec: *pvcSpec}
			if err := checkVolumeSatisfyClaim(&pv, pvc); err == nil {
				pvcSpec.VolumeMode = pv.Spec.VolumeMode
				if len(pvcSpec.AccessModes) == 0 {
					pvcSpec.AccessModes = pv.Spec.AccessModes
				}
				return nil
			}
		}
	}

	return nil
}

func getDefaultVolumeAndAccessMode(c client.Client, storageClass *storagev1.StorageClass) ([]v1.PersistentVolumeAccessMode, *v1.PersistentVolumeMode, error) {
	if storageClass == nil {
		return nil, nil, errors.Errorf("no accessMode defined on DV and no StorageProfile")
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get StorageProfile")
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		return accessModes, volumeMode, nil
	}

	// no accessMode configured on storageProfile
	return nil, nil, errors.Errorf("no accessMode defined DV nor on StorageProfile for %s StorageClass", storageClass.Name)
}

func getDefaultVolumeMode(c client.Client, storageClass *storagev1.StorageClass, pvcAccessModes []v1.PersistentVolumeAccessMode) (*v1.PersistentVolumeMode, error) {
	if storageClass == nil {
		// fallback to k8s defaults
		return nil, nil
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get StorageProfile")
	}
	if len(storageProfile.Status.ClaimPropertySets) > 0 {
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		if len(pvcAccessModes) == 0 {
			return volumeMode, nil
		}
		// check for volume mode matching with given pvc access modes
		for _, cps := range storageProfile.Status.ClaimPropertySets {
			for _, accessMode := range cps.AccessModes {
				for _, pvcAccessMode := range pvcAccessModes {
					if accessMode == pvcAccessMode {
						return cps.VolumeMode, nil
					}
				}
			}
		}
		// if not found return default volume mode for the storage class
		return volumeMode, nil
	}

	// since volumeMode is optional - > gracefully fallback to k8s defaults,
	return nil, nil
}

func getDefaultAccessModes(c client.Client, storageClass *storagev1.StorageClass, pvcVolumeMode *v1.PersistentVolumeMode) ([]v1.PersistentVolumeAccessMode, error) {
	if storageClass == nil {
		return nil, errors.Errorf("no accessMode defined on DV, no StorageProfile ")
	}

	storageProfile := &cdiv1.StorageProfile{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: storageClass.Name}, storageProfile)
	if err != nil {
		return nil, errors.Wrap(err, "no accessMode defined on DV, cannot get StorageProfile")
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 {
		// check for access modes matching with given pvc volume mode
		defaultAccessModes := []v1.PersistentVolumeAccessMode{}
		for _, cps := range storageProfile.Status.ClaimPropertySets {
			if cps.VolumeMode != nil && pvcVolumeMode != nil && *cps.VolumeMode == *pvcVolumeMode {
				if len(cps.AccessModes) > 0 {
					return cps.AccessModes, nil
				}
			} else if len(cps.AccessModes) > 0 && len(defaultAccessModes) == 0 {
				defaultAccessModes = cps.AccessModes
			}
		}
		// if not found return default access modes for the storage profile
		if len(defaultAccessModes) > 0 {
			return defaultAccessModes, nil
		}
	}

	// no accessMode configured on storageProfile
	return nil, errors.Errorf("no accessMode defined on StorageProfile for %s StorageClass", storageClass.Name)
}

func resolveVolumeSize(c client.Client, dvSpec cdiv1.DataVolumeSpec, pvcSpec *v1.PersistentVolumeClaimSpec) (*resource.Quantity, error) {
	// resources.requests[storage] - just copy it to pvc,
	requestedSize, found := dvSpec.Storage.Resources.Requests[v1.ResourceStorage]

	if !found {
		// Storage size can be empty when cloning
		isClone := dvSpec.Source.PVC != nil
		if isClone {
			return &requestedSize, nil
		}
		return nil, errors.Errorf("Datavolume Spec is not valid - missing storage size")
	}

	// disk or image size, inflate it with overhead
	requestedSize, err := cc.InflateSizeWithOverhead(context.TODO(), c, requestedSize.Value(), pvcSpec)

	return &requestedSize, err
}

// storageClassCSIDriverExists returns true if the passed storage class has CSI drivers available
func storageClassCSIDriverExists(client client.Client, log logr.Logger, storageClassName *string) (bool, error) {
	log = log.WithName("storageClassCSIDriverExists").V(3)

	storageClass, err := cc.GetStorageClassByName(context.TODO(), client, storageClassName)
	if err != nil {
		return false, err
	}
	if storageClass == nil {
		log.Info("Target PVC's Storage Class not found")
		return false, nil
	}

	csiDriver := &storagev1.CSIDriver{}

	if err := client.Get(context.TODO(), types.NamespacedName{Name: storageClass.Provisioner}, csiDriver); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

// CheckPVCUsingPopulators returns true if pvc has dataSourceRef and has
// the usePopulator annotation
func CheckPVCUsingPopulators(pvc *v1.PersistentVolumeClaim) (bool, error) {
	if pvc.Spec.DataSourceRef == nil {
		return false, nil
	}
	usePopulator, ok := pvc.Annotations[cc.AnnUsePopulator]
	if !ok {
		return false, nil
	}
	boolUsePopulator, err := strconv.ParseBool(usePopulator)
	if err != nil {
		return false, err
	}
	return boolUsePopulator, nil
}

func updateDataVolumeUseCDIPopulator(syncState *dvSyncState) {
	cc.AddAnnotation(syncState.dvMutated, cc.AnnUsePopulator, strconv.FormatBool(syncState.usePopulator))
}

func checkDVUsingPopulators(dv *cdiv1.DataVolume) (bool, error) {
	usePopulator, ok := dv.Annotations[cc.AnnUsePopulator]
	if !ok {
		return false, nil
	}
	boolUsePopulator, err := strconv.ParseBool(usePopulator)
	if err != nil {
		return false, err
	}
	return boolUsePopulator, nil
}

func dvBoundOrPopulationInProgress(dataVolume *cdiv1.DataVolume, boundCond *cdiv1.DataVolumeCondition) bool {
	usePopulator, err := checkDVUsingPopulators(dataVolume)
	if err != nil {
		return false
	}
	return boundCond.Status == v1.ConditionTrue ||
		(usePopulator && dataVolume.Status.Phase != cdiv1.Pending && dataVolume.Status.Phase != cdiv1.PendingPopulation)
}

func createStorageProfile(name string,
	accessModes []v1.PersistentVolumeAccessMode,
	volumeMode v1.PersistentVolumeMode) *cdiv1.StorageProfile {
	claimPropertySets := []cdiv1.ClaimPropertySet{{
		AccessModes: accessModes,
		VolumeMode:  &volumeMode,
	}}
	return createStorageProfileWithClaimPropertySets(name, claimPropertySets)
}

func createStorageProfileWithClaimPropertySets(name string,
	claimPropertySets []cdiv1.ClaimPropertySet) *cdiv1.StorageProfile {
	return createStorageProfileWithCloneStrategy(name, claimPropertySets, nil)
}

func createStorageProfileWithCloneStrategy(name string,
	claimPropertySets []cdiv1.ClaimPropertySet,
	cloneStrategy *cdiv1.CDICloneStrategy) *cdiv1.StorageProfile {

	return &cdiv1.StorageProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: cdiv1.StorageProfileStatus{
			StorageClass:      &name,
			ClaimPropertySets: claimPropertySets,
			CloneStrategy:     cloneStrategy,
		},
	}
}

func hasAnnOwnedByDataVolume(obj metav1.Object) bool {
	_, ok := obj.GetAnnotations()[AnnOwnedByDataVolume]
	return ok
}

func getAnnOwnedByDataVolume(obj metav1.Object) (string, string, error) {
	val := obj.GetAnnotations()[AnnOwnedByDataVolume]
	return cache.SplitMetaNamespaceKey(val)
}

func setAnnOwnedByDataVolume(dest, obj metav1.Object) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	if dest.GetAnnotations() == nil {
		dest.SetAnnotations(make(map[string]string))
	}
	dest.GetAnnotations()[AnnOwnedByDataVolume] = key

	return nil
}

// adapted from k8s.io/kubernetes/pkg/controller/volume/persistentvolume/pv_controller.go
// checkVolumeSatisfyClaim checks if the volume requested by the claim satisfies the requirements of the claim
func checkVolumeSatisfyClaim(volume *v1.PersistentVolume, claim *v1.PersistentVolumeClaim) error {
	requestedQty := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestedSize := requestedQty.Value()

	// check if PV's DeletionTimeStamp is set, if so, return error.
	if volume.ObjectMeta.DeletionTimestamp != nil {
		return fmt.Errorf("the volume is marked for deletion %q", volume.Name)
	}

	volumeQty := volume.Spec.Capacity[v1.ResourceStorage]
	volumeSize := volumeQty.Value()
	if volumeSize < requestedSize {
		return fmt.Errorf("requested PV is too small")
	}

	// this differs from pv_controller, be loose on storage class if not specified
	requestedClass := storagehelpers.GetPersistentVolumeClaimClass(claim)
	if requestedClass != "" && storagehelpers.GetPersistentVolumeClass(volume) != requestedClass {
		return fmt.Errorf("storageClassName does not match")
	}

	if storagehelpers.CheckVolumeModeMismatches(&claim.Spec, &volume.Spec) {
		return fmt.Errorf("incompatible volumeMode")
	}

	if !storagehelpers.CheckAccessModes(claim, volume) {
		return fmt.Errorf("incompatible accessMode")
	}

	return nil
}

func getReconcileRequest(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: client.ObjectKeyFromObject(obj)}
}
