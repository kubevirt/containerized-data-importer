package clone

import (
	"context"
	"fmt"
	"sort"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// IsDataSourcePVC checks for PersistentVolumeClaim source kind
func IsDataSourcePVC(kind string) bool {
	return kind == "PersistentVolumeClaim"
}

// IsDataSourceVolumeClone checks for VolumeCloneSourceRef source kind
func IsDataSourceVolumeClone(dataSource *corev1.TypedObjectReference) bool {
	return dataSource != nil && dataSource.Kind == cdiv1.VolumeCloneSourceRef
}

// GetVolumeCloneSource returns the VolumeCloneSource dataSourceRef
func GetVolumeCloneSource(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*cdiv1.VolumeCloneSource, error) {
	if !IsDataSourceVolumeClone(pvc.Spec.DataSourceRef) {
		return nil, fmt.Errorf("invalid dataSourceRef")
	}

	ns := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		ns = *pvc.Spec.DataSourceRef.Namespace
	}

	obj := &cdiv1.VolumeCloneSource{}
	exists, err := getResource(ctx, c, ns, pvc.Spec.DataSourceRef.Name, obj)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	return obj, nil
}

// AddCommonLabels adds common labels to a resource
func AddCommonLabels(obj metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	obj.GetLabels()[common.CDILabelKey] = common.CDILabelValue
}

// AddCommonClaimLabels adds common labels to a pvc
func AddCommonClaimLabels(pvc *corev1.PersistentVolumeClaim) {
	AddCommonLabels(pvc)
	if util.ResolveVolumeMode(pvc.Spec.VolumeMode) == corev1.PersistentVolumeFilesystem {
		pvc.GetLabels()[common.KubePersistentVolumeFillingUpSuppressLabelKey] = common.KubePersistentVolumeFillingUpSuppressLabelValue
	}
}

// AddOwnershipLabel adds owner label
func AddOwnershipLabel(label string, obj, owner metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	obj.GetLabels()[label] = string(owner.GetUID())
}

// IsSourceClaimReady checks that PVC exists, is bound, and is not being used
func IsSourceClaimReady(ctx context.Context, c client.Client, namespace, name string) (bool, error) {
	claim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, c, namespace, name, claim)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	if claim.Status.Phase != corev1.ClaimBound {
		return false, nil
	}

	pods, err := cc.GetPodsUsingPVCs(ctx, c, namespace, sets.New(name), true)
	if err != nil {
		return false, err
	}

	if len(pods) > 0 {
		return false, nil
	}

	return cdiv1.IsPopulated(claim, dataVolumeGetter(ctx, c))
}

// ValidateContentTypes validates source/target contenttypes are compatible
func ValidateContentTypes(sourceClaim *corev1.PersistentVolumeClaim, targetContentType cdiv1.DataVolumeContentType) error {
	sourceContentType := cdiv1.DataVolumeContentType(cc.GetPVCContentType(sourceClaim))
	if targetContentType == "" {
		targetContentType = cdiv1.DataVolumeKubeVirt
	}
	if sourceContentType == cdiv1.DataVolumeArchive && targetContentType == cdiv1.DataVolumeKubeVirt {
		return fmt.Errorf("archive source and kubevirt target are incompatible")
	}
	return nil
}

// GetGlobalCloneStrategyOverride returns the global clone strategy override
func GetGlobalCloneStrategyOverride(ctx context.Context, c client.Client) (*cdiv1.CDICloneStrategy, error) {
	cr, err := cc.GetActiveCDI(ctx, c)
	if err != nil {
		return nil, err
	}

	if cr == nil {
		return nil, fmt.Errorf("no active CDI")
	}

	if cr.Spec.CloneStrategyOverride == nil {
		return nil, nil
	}

	return cr.Spec.CloneStrategyOverride, nil
}

// GetStorageClassForClaim returns the storageclass for a PVC
func GetStorageClassForClaim(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*storagev1.StorageClass, error) {
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return nil, nil
	}

	sc := &storagev1.StorageClass{}
	exists, err := getResource(ctx, c, "", *pvc.Spec.StorageClassName, sc)
	if err != nil {
		return nil, err
	}

	if exists {
		return sc, nil
	}

	return nil, nil
}

// GetCompatibleVolumeSnapshotClass returns a VolumeSNapshotClass name that works for all PVCs
func GetCompatibleVolumeSnapshotClass(ctx context.Context, c client.Client, pvcs ...*corev1.PersistentVolumeClaim) (*string, error) {
	var storageClasses []*storagev1.StorageClass
	for _, pvc := range pvcs {
		sc, err := GetStorageClassForClaim(ctx, c, pvc)
		if err != nil {
			return nil, err
		}

		if sc == nil {
			return nil, nil
		}

		storageClasses = append(storageClasses, sc)
	}

	volumeSnapshotClasses := &snapshotv1.VolumeSnapshotClassList{}
	if err := c.List(ctx, volumeSnapshotClasses); err != nil {
		if meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}

	var candidates []string
	for _, vcs := range volumeSnapshotClasses.Items {
		matches := true
		for _, sc := range storageClasses {
			if sc.Provisioner != vcs.Driver {
				matches = false
				break
			}
		}
		if matches {
			candidates = append(candidates, vcs.Name)
		}
	}

	if len(candidates) > 0 {
		sort.Strings(candidates)
		return &candidates[0], nil
	}

	return nil, nil
}

// SameVolumeMode returns true if all pvcs have the same volume mode
func SameVolumeMode(pvc1 *corev1.PersistentVolumeClaim, others ...*corev1.PersistentVolumeClaim) bool {
	vm := util.ResolveVolumeMode(pvc1.Spec.VolumeMode)
	for _, pvc := range others {
		if util.ResolveVolumeMode(pvc.Spec.VolumeMode) != vm {
			return false
		}
	}
	return true
}

func getResource(ctx context.Context, c client.Client, namespace, name string, obj client.Object) (bool, error) {
	obj.SetNamespace(namespace)
	obj.SetName(name)

	err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func dataVolumeGetter(ctx context.Context, c client.Client) func(name, namespace string) (*cdiv1.DataVolume, error) {
	return func(name, namespace string) (*cdiv1.DataVolume, error) {
		obj := &cdiv1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}

		err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return nil, err
		}

		return obj, nil
	}
}

func checkQuotaExceeded(r record.EventRecorder, owner client.Object, err error) {
	if cc.ErrQuotaExceeded(err) {
		r.Event(owner, corev1.EventTypeWarning, cc.ErrExceededQuota, err.Error())
	}
}

func claimBoundOrWFFC(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if pvc.Spec.VolumeName != "" {
		return true, nil
	}

	sc, err := GetStorageClassForClaim(ctx, c, pvc)
	if err != nil {
		return false, err
	}

	if sc == nil {
		return false, fmt.Errorf("no storageclass for pvc")
	}

	if sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
		return true, nil
	}

	return false, nil
}
