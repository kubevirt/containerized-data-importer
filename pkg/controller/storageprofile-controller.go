package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	storagehelpers "k8s.io/component-helpers/storage/volume"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	storageProfileControllerName = "storageprofile-controller"
	counterLabelStorageClass     = "storageclass"
	counterLabelProvisioner      = "provisioner"
	counterLabelComplete         = "complete"
	counterLabelDefault          = "default"
	counterLabelVirtDefault      = "virtdefault"
	counterLabelRWX              = "rwx"
	counterLabelSmartClone       = "smartclone"
)

var (
	// StorageProfileStatusGaugeVec is the metric we use to alert about storage profile status
	StorageProfileStatusGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: monitoring.MetricOptsList[monitoring.StorageProfileStatus].Name,
			Help: monitoring.MetricOptsList[monitoring.StorageProfileStatus].Help,
		},
		[]string{
			counterLabelStorageClass,
			counterLabelProvisioner,
			counterLabelComplete,
			counterLabelDefault,
			counterLabelVirtDefault,
			counterLabelRWX,
			counterLabelSmartClone,
		},
	)
)

// StorageProfileReconciler members
type StorageProfileReconciler struct {
	client client.Client
	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient  client.Client
	scheme          *runtime.Scheme
	log             logr.Logger
	installerLabels map[string]string
}

// Reconcile the reconcile.Reconciler implementation for the StorageProfileReconciler object.
func (r *StorageProfileReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("StorageProfile", req.NamespacedName)
	log.Info("reconciling StorageProfile")

	storageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, storageClass); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, r.deleteStorageProfile(req.NamespacedName.Name, log)
		}
		return reconcile.Result{}, err
	} else if storageClass.GetDeletionTimestamp() != nil {
		return reconcile.Result{}, r.deleteStorageProfile(req.NamespacedName.Name, log)
	}

	return r.reconcileStorageProfile(storageClass)
}

func (r *StorageProfileReconciler) reconcileStorageProfile(sc *storagev1.StorageClass) (reconcile.Result, error) {
	log := r.log.WithValues("StorageProfile", sc.Name)

	storageProfile, prevStorageProfile, err := r.getStorageProfile(sc)
	if err != nil {
		log.Error(err, "Unable to create StorageProfile")
		return reconcile.Result{}, err
	}

	storageProfile.Status.StorageClass = &sc.Name
	storageProfile.Status.Provisioner = &sc.Provisioner
	snapClass, err := cc.GetSnapshotClassForSmartClone("", &sc.Name, r.log, r.client)
	if err != nil {
		return reconcile.Result{}, err
	}
	storageProfile.Status.CloneStrategy = r.reconcileCloneStrategy(sc, storageProfile.Spec.CloneStrategy, snapClass)
	storageProfile.Status.DataImportCronSourceFormat = r.reconcileDataImportCronSourceFormat(sc, storageProfile.Spec.DataImportCronSourceFormat, snapClass)

	var claimPropertySets []cdiv1.ClaimPropertySet

	if len(storageProfile.Spec.ClaimPropertySets) > 0 {
		for _, cps := range storageProfile.Spec.ClaimPropertySets {
			if len(cps.AccessModes) == 0 && cps.VolumeMode != nil {
				err = fmt.Errorf("must provide access mode for volume mode: %s", *cps.VolumeMode)
				log.Error(err, "Unable to update StorageProfile")
				return reconcile.Result{}, err
			}
		}
		claimPropertySets = storageProfile.Spec.ClaimPropertySets
	} else {
		claimPropertySets = r.reconcilePropertySets(sc)
	}

	storageProfile.Status.ClaimPropertySets = claimPropertySets

	util.SetRecommendedLabels(storageProfile, r.installerLabels, "cdi-controller")
	if err := r.updateStorageProfile(prevStorageProfile, storageProfile, log); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.computeMetrics(storageProfile, sc)
}

func (r *StorageProfileReconciler) updateStorageProfile(prevStorageProfile runtime.Object, storageProfile *cdiv1.StorageProfile, log logr.Logger) error {
	if prevStorageProfile == nil {
		return r.client.Create(context.TODO(), storageProfile)
	} else if !reflect.DeepEqual(prevStorageProfile, storageProfile) {
		// Updates have happened, update StorageProfile.
		log.Info("Updating StorageProfile", "StorageProfile.Name", storageProfile.Name, "storageProfile", storageProfile)
		return r.client.Update(context.TODO(), storageProfile)
	}

	return nil
}

func (r *StorageProfileReconciler) getStorageProfile(sc *storagev1.StorageClass) (*cdiv1.StorageProfile, runtime.Object, error) {
	var prevStorageProfile runtime.Object
	storageProfile := &cdiv1.StorageProfile{}

	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: sc.Name}, storageProfile); err != nil {
		if k8serrors.IsNotFound(err) {
			storageProfile, err = r.createEmptyStorageProfile(sc)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	} else {
		prevStorageProfile = storageProfile.DeepCopyObject()
	}

	return storageProfile, prevStorageProfile, nil
}

func (r *StorageProfileReconciler) reconcilePropertySets(sc *storagev1.StorageClass) []cdiv1.ClaimPropertySet {
	claimPropertySets := []cdiv1.ClaimPropertySet{}
	capabilities, found := storagecapabilities.GetCapabilities(r.client, sc)
	if found {
		for i := range capabilities {
			claimPropertySet := cdiv1.ClaimPropertySet{
				AccessModes: []v1.PersistentVolumeAccessMode{capabilities[i].AccessMode},
				VolumeMode:  &capabilities[i].VolumeMode,
			}
			claimPropertySets = append(claimPropertySets, claimPropertySet)
		}
	}
	return claimPropertySets
}

func (r *StorageProfileReconciler) reconcileCloneStrategy(sc *storagev1.StorageClass, desiredCloneStrategy *cdiv1.CDICloneStrategy, snapClass string) *cdiv1.CDICloneStrategy {
	if desiredCloneStrategy != nil {
		return desiredCloneStrategy
	}

	if annStrategyVal, ok := sc.Annotations["cdi.kubevirt.io/clone-strategy"]; ok {
		return r.getCloneStrategyFromStorageClass(annStrategyVal)
	}

	// Default to trying snapshot clone unless volume snapshot class missing
	hostAssistedStrategy := cdiv1.CloneStrategyHostAssisted
	strategy := hostAssistedStrategy
	if snapClass != "" {
		strategy = cdiv1.CloneStrategySnapshot
	}

	if knownStrategy, ok := storagecapabilities.GetAdvisedCloneStrategy(sc); ok {
		strategy = knownStrategy
	}

	if strategy == cdiv1.CloneStrategySnapshot && snapClass == "" {
		r.log.Info("No VolumeSnapshotClass found for storage class, falling back to host assisted cloning", "StorageClass.Name", sc.Name)
		return &hostAssistedStrategy
	}

	return &strategy
}

func (r *StorageProfileReconciler) getCloneStrategyFromStorageClass(annStrategyVal string) *cdiv1.CDICloneStrategy {
	var strategy cdiv1.CDICloneStrategy

	switch annStrategyVal {
	case "copy":
		strategy = cdiv1.CloneStrategyHostAssisted
	case "snapshot":
		strategy = cdiv1.CloneStrategySnapshot
	case "csi-clone":
		strategy = cdiv1.CloneStrategyCsiClone
	}

	return &strategy
}

func (r *StorageProfileReconciler) reconcileDataImportCronSourceFormat(sc *storagev1.StorageClass, desiredFormat *cdiv1.DataImportCronSourceFormat, snapClass string) *cdiv1.DataImportCronSourceFormat {
	if desiredFormat != nil {
		return desiredFormat
	}

	// This can be changed later on
	// for example, if at some point we're confident snapshot sources should be the default
	pvcFormat := cdiv1.DataImportCronSourceFormatPvc
	format := pvcFormat

	if knownFormat, ok := storagecapabilities.GetAdvisedSourceFormat(sc); ok {
		format = knownFormat
	}

	if format == cdiv1.DataImportCronSourceFormatSnapshot && snapClass == "" {
		// No point using snapshots without a corresponding snapshot class
		r.log.Info("No VolumeSnapshotClass found for storage class, falling back to pvc sources for DataImportCrons", "StorageClass.Name", sc.Name)
		return &pvcFormat
	}

	return &format
}

func (r *StorageProfileReconciler) createEmptyStorageProfile(sc *storagev1.StorageClass) (*cdiv1.StorageProfile, error) {
	storageProfile := MakeEmptyStorageProfileSpec(sc.Name)
	util.SetRecommendedLabels(storageProfile, r.installerLabels, "cdi-controller")
	// uncachedClient is used to directly get the config map
	// the controller runtime client caches objects that are read once, and thus requires a list/watch
	// should be cheaper than watching
	if err := operator.SetOwnerRuntime(r.uncachedClient, storageProfile); err != nil {
		return nil, err
	}
	return storageProfile, nil
}

func (r *StorageProfileReconciler) deleteStorageProfile(name string, log logr.Logger) error {
	log.Info("Cleaning up StorageProfile that corresponds to deleted StorageClass", "StorageClass.Name", name)
	profile := &cdiv1.StorageProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.client.Delete(context.TODO(), profile); cc.IgnoreNotFound(err) != nil {
		return err
	}

	labels := prometheus.Labels{
		counterLabelStorageClass: name,
	}
	StorageProfileStatusGaugeVec.DeletePartialMatch(labels)
	return nil
}

func isNoProvisioner(name string, cl client.Client) bool {
	storageClass := &storagev1.StorageClass{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name}, storageClass); err != nil {
		return false
	}
	return storageClass.Provisioner == storagehelpers.NotSupportedProvisioner
}

func (r *StorageProfileReconciler) computeMetrics(profile *cdiv1.StorageProfile, sc *storagev1.StorageClass) error {
	if profile.Status.StorageClass == nil || profile.Status.Provisioner == nil {
		return nil
	}

	storageClass := *profile.Status.StorageClass
	provisioner := *profile.Status.Provisioner

	// We don't count explicitly unsupported provisioners as incomplete
	_, found := storagecapabilities.UnsupportedProvisioners[*profile.Status.Provisioner]
	isComplete := found || !isIncomplete(profile.Status.ClaimPropertySets)
	isDefault := sc.Annotations[cc.AnnDefaultStorageClass] == "true"
	isVirtDefault := false
	isRWX := hasRWX(profile.Status.ClaimPropertySets)
	isSmartClone, err := r.hasSmartClone(profile)
	if err != nil {
		return err
	}

	// Setting the labeled Gauge to 1 will not delete older metric, so we need to explicitly delete them
	scLabels := prometheus.Labels{counterLabelStorageClass: storageClass, counterLabelProvisioner: provisioner}
	metricsDeleted := StorageProfileStatusGaugeVec.DeletePartialMatch(scLabels)
	scLabels = createLabels(storageClass, provisioner, isComplete, isDefault, isVirtDefault, isRWX, isSmartClone)
	StorageProfileStatusGaugeVec.With(scLabels).Set(float64(1))
	r.log.Info(fmt.Sprintf("Set metric:%s complete:%t default:%t vdefault:%t rwx:%t smartclone:%t (deleted %d)",
		storageClass, isComplete, isDefault, isVirtDefault, isRWX, isSmartClone, metricsDeleted))

	return nil
}

func (r *StorageProfileReconciler) hasSmartClone(sp *cdiv1.StorageProfile) (bool, error) {
	strategy := sp.Status.CloneStrategy
	provisioner := sp.Status.Provisioner

	if strategy != nil {
		if *strategy == cdiv1.CloneStrategyHostAssisted {
			return false, nil
		}
		if *strategy == cdiv1.CloneStrategyCsiClone && provisioner != nil {
			driver := &storagev1.CSIDriver{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: *provisioner}, driver); err != nil {
				return false, cc.IgnoreNotFound(err)
			}
			return true, nil
		}
	}

	if (strategy == nil || *strategy == cdiv1.CloneStrategySnapshot) && provisioner != nil {
		vscs := &snapshotv1.VolumeSnapshotClassList{}
		if err := r.client.List(context.TODO(), vscs); err != nil {
			return false, err
		}
		return hasDriver(vscs, *provisioner), nil
	}

	return false, nil
}

func createLabels(storageClass, provisioner string, isComplete, isDefault, isVirtDefault, isRWX, isSmartClone bool) prometheus.Labels {
	return prometheus.Labels{
		counterLabelStorageClass: storageClass,
		counterLabelProvisioner:  provisioner,
		counterLabelComplete:     strconv.FormatBool(isComplete),
		counterLabelDefault:      strconv.FormatBool(isDefault),
		counterLabelVirtDefault:  strconv.FormatBool(isVirtDefault),
		counterLabelRWX:          strconv.FormatBool(isRWX),
		counterLabelSmartClone:   strconv.FormatBool(isSmartClone),
	}
}

// MakeEmptyStorageProfileSpec creates StorageProfile manifest
func MakeEmptyStorageProfileSpec(name string) *cdiv1.StorageProfile {
	return &cdiv1.StorageProfile{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageProfile",
			APIVersion: "cdi.kubevirt.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
	}
}

// NewStorageProfileController creates a new instance of the StorageProfile controller.
func NewStorageProfileController(mgr manager.Manager, log logr.Logger, installerLabels map[string]string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	reconciler := &StorageProfileReconciler{
		client:          mgr.GetClient(),
		uncachedClient:  uncachedClient,
		scheme:          mgr.GetScheme(),
		log:             log.WithName("storageprofile-controller"),
		installerLabels: installerLabels,
	}

	storageProfileController, err := controller.New(
		"storageprofile-controller",
		mgr,
		controller.Options{Reconciler: reconciler, MaxConcurrentReconciles: 3})
	if err != nil {
		return nil, err
	}
	if err := addStorageProfileControllerWatches(mgr, storageProfileController, log); err != nil {
		return nil, err
	}

	log.Info("Initialized StorageProfile controller")
	return storageProfileController, nil
}

func addStorageProfileControllerWatches(mgr manager.Manager, c controller.Controller, log logr.Logger) error {
	if err := c.Watch(&source.Kind{Type: &storagev1.StorageClass{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &cdiv1.StorageProfile{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &v1.PersistentVolume{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: scName(obj)},
			}}
		},
	),
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return isNoProvisioner(scName(e.Object), mgr.GetClient()) },
			UpdateFunc: func(e event.UpdateEvent) bool { return isNoProvisioner(scName(e.ObjectNew), mgr.GetClient()) },
			DeleteFunc: func(e event.DeleteEvent) bool { return isNoProvisioner(scName(e.Object), mgr.GetClient()) },
		}); err != nil {
		return err
	}

	mapSnapshotClassToProfile := func(obj client.Object) (reqs []reconcile.Request) {
		var scList storagev1.StorageClassList
		if err := mgr.GetClient().List(context.TODO(), &scList); err != nil {
			c.GetLogger().Error(err, "Unable to list StorageClasses")
			return
		}
		vsc := obj.(*snapshotv1.VolumeSnapshotClass)
		for _, sc := range scList.Items {
			if sc.Provisioner == vsc.Driver {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: sc.Name}})
			}
		}
		return
	}
	if err := mgr.GetClient().List(context.TODO(), &snapshotv1.VolumeSnapshotClassList{}, &client.ListOptions{Limit: 1}); err != nil {
		if meta.IsNoMatchError(err) {
			// Back out if there's no point to attempt watch
			return nil
		}
		if !cc.IsErrCacheNotStarted(err) {
			return err
		}
	}
	if err := c.Watch(&source.Kind{Type: &snapshotv1.VolumeSnapshotClass{}},
		handler.EnqueueRequestsFromMapFunc(mapSnapshotClassToProfile),
	); err != nil {
		return err
	}

	return nil
}

func scName(obj client.Object) string {
	return obj.(*v1.PersistentVolume).Spec.StorageClassName
}

func isIncomplete(sets []cdiv1.ClaimPropertySet) bool {
	if len(sets) > 0 {
		for _, cps := range sets {
			if len(cps.AccessModes) == 0 || cps.VolumeMode == nil {
				return true
			}
		}
	} else {
		return true
	}

	return false
}

func hasRWX(cpSets []cdiv1.ClaimPropertySet) bool {
	for _, cpSet := range cpSets {
		for _, am := range cpSet.AccessModes {
			if am == v1.ReadWriteMany {
				return true
			}
		}
	}
	return false
}

func hasDriver(vscs *snapshotv1.VolumeSnapshotClassList, driver string) bool {
	for i := range vscs.Items {
		vsc := vscs.Items[i]
		if vsc.Driver == driver {
			return true
		}
	}
	return false
}
