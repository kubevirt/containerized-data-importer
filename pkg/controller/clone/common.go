package clone

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// PendingPhaseName is the phase when the clone is pending
	PendingPhaseName = "Pending"

	// SucceededPhaseName is the phase when the clone is succeeded
	SucceededPhaseName = "Succeeded"

	// ErrorPhaseName is the phase when the clone is in error
	ErrorPhaseName = "Error"
)

// IsDataSourcePVC checks for PersistentVolumeClaim source kind
func IsDataSourcePVC(kind string) bool {
	return kind == "PersistentVolumeClaim"
}

// IsDataSourceSnapshot checks for Snapshot source kind
func IsDataSourceSnapshot(kind string) bool {
	return kind == "VolumeSnapshot"
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

// IsSourceClaimReadyArgs are arguments for IsSourceClaimReady
type IsSourceClaimReadyArgs struct {
	Target          client.Object
	SourceNamespace string
	SourceName      string
	Client          client.Client
	Log             logr.Logger
	Recorder        record.EventRecorder
}

// IsSourceClaimReady checks that PVC exists, is bound, and is not being used
func IsSourceClaimReady(ctx context.Context, args *IsSourceClaimReadyArgs) (bool, error) {
	claim := &corev1.PersistentVolumeClaim{}
	exists, err := getResource(ctx, args.Client, args.SourceNamespace, args.SourceName, claim)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, nil
	}

	if claim.Status.Phase != corev1.ClaimBound {
		return false, nil
	}

	pods, err := cc.GetPodsUsingPVCs(ctx, args.Client, args.SourceNamespace, sets.New(args.SourceName), true)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		args.Log.V(1).Info("Source PVC is being used by pod", "namespace", args.SourceNamespace, "name", args.SourceName, "pod", pod.Name)
		args.Recorder.Eventf(args.Target, corev1.EventTypeWarning, cc.CloneSourceInUse,
			"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, args.SourceName)
	}

	if len(pods) > 0 {
		return false, nil
	}

	return cdiv1.IsPopulated(claim, dataVolumeGetter(ctx, args.Client))
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

func getSnapshotClassForClaim(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*string, error) {
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return nil, nil
	}

	sp := &cdiv1.StorageProfile{}
	exists, err := getResource(ctx, c, "", *pvc.Spec.StorageClassName, sp)
	if err != nil {
		return nil, err
	}
	if exists {
		return sp.Status.SnapshotClass, nil
	}

	return nil, nil
}

// GetDriverFromVolume returns the CSI driver name for a PVC
func GetDriverFromVolume(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*string, error) {
	if pvc.Spec.VolumeName == "" {
		return nil, nil
	}

	pv := &corev1.PersistentVolume{}
	exists, err := getResource(ctx, c, "", pvc.Spec.VolumeName, pv)
	if err != nil {
		return nil, err
	}

	if !exists || pv.Spec.ClaimRef == nil {
		return nil, nil
	}

	if pv.Spec.ClaimRef.Namespace != pvc.Namespace ||
		pv.Spec.ClaimRef.Name != pvc.Name {
		return nil, fmt.Errorf("pvc does not match volume claim ref")
	}

	if pv.Spec.CSI == nil {
		return nil, nil
	}

	return &pv.Spec.CSI.Driver, nil
}

// GetCommonDriver returns the name of the CSI driver shared by all PVCs
func GetCommonDriver(ctx context.Context, c client.Client, pvcs ...*corev1.PersistentVolumeClaim) (*string, error) {
	var result *string

	for _, pvc := range pvcs {
		driver, err := GetDriverFromVolume(ctx, c, pvc)
		if err != nil {
			return nil, err
		}

		if driver == nil {
			sc, err := GetStorageClassForClaim(ctx, c, pvc)
			if err != nil {
				return nil, err
			}

			if sc == nil {
				return nil, nil
			}

			driver = &sc.Provisioner
		}

		if result == nil {
			result = driver
		}

		if *result != *driver {
			return nil, nil
		}
	}

	return result, nil
}

func getCommonSnapshotClass(ctx context.Context, c client.Client, pvcs ...*corev1.PersistentVolumeClaim) (*string, error) {
	var result *string

	for _, pvc := range pvcs {
		sc, err := getSnapshotClassForClaim(ctx, c, pvc)
		if err != nil {
			return nil, err
		}
		if sc == nil {
			return nil, nil
		}
		if result == nil {
			result = sc
		} else if *result != *sc {
			return nil, nil
		}
	}

	return result, nil
}

// GetCompatibleVolumeSnapshotClass returns a VolumeSnapshotClass name that works for all PVCs
func GetCompatibleVolumeSnapshotClass(ctx context.Context, c client.Client, log logr.Logger, pvcs ...*corev1.PersistentVolumeClaim) (*string, error) {
	driver, err := GetCommonDriver(ctx, c, pvcs...)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, nil
	}

	snapshotClassName, err := getCommonSnapshotClass(ctx, c, pvcs...)
	if err != nil {
		return nil, err
	}

	return cc.GetVolumeSnapshotClass(context.TODO(), c, *driver, snapshotClassName, log)
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

func isClaimBoundOrWFFC(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	if cc.IsBound(pvc) {
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
