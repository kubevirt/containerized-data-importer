package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	eventInvolvedObjectField = "eventInvolvedObject"
)

var (
	// IncompleteProfileGauge is the metric we use to alert about incomplete storage profiles
	IncompleteProfileGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: monitoring.MetricOptsList[monitoring.IncompleteProfile].Name,
			Help: monitoring.MetricOptsList[monitoring.IncompleteProfile].Help,
		})

	accessModes = []v1.PersistentVolumeAccessMode{v1.ReadWriteMany, v1.ReadWriteOnce, v1.ReadOnlyMany}
	volumeModes = []v1.PersistentVolumeMode{v1.PersistentVolumeBlock, v1.PersistentVolumeFilesystem}
)

// StorageProfileReconciler members
type StorageProfileReconciler struct {
	client client.Client
	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient  client.Client
	scheme          *runtime.Scheme
	log             logr.Logger
	cdiNamespace    string
	installerLabels map[string]string
}

// Reconcile the reconcile.Reconciler implementation for the StorageProfileReconciler object.
func (r *StorageProfileReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("StorageProfile", req.NamespacedName)
	log.Info("reconciling StorageProfile")

	storageClass := &storagev1.StorageClass{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, storageClass); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, r.deleteStorageProfile(req.NamespacedName.Name, log)
		}
		return reconcile.Result{}, err
	} else if storageClass.GetDeletionTimestamp() != nil {
		return reconcile.Result{}, r.deleteStorageProfile(req.NamespacedName.Name, log)
	}

	if _, err := r.reconcileStorageProfile(storageClass); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.checkIncompleteProfiles()
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
	storageProfile.Status.CloneStrategy = r.reconcileCloneStrategy(sc, storageProfile.Spec.CloneStrategy)

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
		claimPropertySets, err = r.reconcilePropertySets(sc)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if len(claimPropertySets) > 0 {
		//FIXME: handle local storage
		storageProfile.Status.ClaimPropertySets = claimPropertySets
	} else if prevStorageProfile != nil {
		//FIXME: add pending annotation and poll only if > 0
		if err := r.pollPvcs(storageProfile); err != nil {
			return reconcile.Result{}, err
		}
	}

	util.SetRecommendedLabels(storageProfile, r.installerLabels, "cdi-controller")
	if err := r.updateStorageProfile(prevStorageProfile, storageProfile, log); err != nil {
		return reconcile.Result{}, err
	}

	if prevStorageProfile == nil && len(claimPropertySets) == 0 {
		if err := r.createDvs(storageProfile); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *StorageProfileReconciler) updateStorageProfile(prevStorageProfile runtime.Object, storageProfile *cdiv1.StorageProfile, log logr.Logger) error {
	if prevStorageProfile == nil {
		return r.client.Create(context.TODO(), storageProfile)
	}

	if !reflect.DeepEqual(prevStorageProfile, storageProfile) {
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
		if errors.IsNotFound(err) {
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

func (r *StorageProfileReconciler) reconcilePropertySets(sc *storagev1.StorageClass) ([]cdiv1.ClaimPropertySet, error) {
	claimPropertySets := []cdiv1.ClaimPropertySet{}
	capabilities, err := storagecapabilities.Get(r.client, sc)
	if err != nil {
		return []cdiv1.ClaimPropertySet{}, err
	}
	for i := range capabilities {
		claimPropertySet := cdiv1.ClaimPropertySet{
			AccessModes: []v1.PersistentVolumeAccessMode{capabilities[i].AccessMode},
			VolumeMode:  &capabilities[i].VolumeMode,
		}
		claimPropertySets = append(claimPropertySets, claimPropertySet)
	}
	return claimPropertySets, nil
}

func (r *StorageProfileReconciler) pollPvcs(storageProfile *cdiv1.StorageProfile) error {
	pvc := &v1.PersistentVolumeClaim{}
	claimPropertySets := storageProfile.Status.ClaimPropertySets

	for _, am := range accessModes {
		for _, vm := range volumeModes {
			pvcName := getDvName(storageProfile.Name, am, vm)
			pvcKey := types.NamespacedName{Name: pvcName, Namespace: r.cdiNamespace}
			if err := r.client.Get(context.TODO(), pvcKey, pvc); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return err
			}

			//FIXME: refactor
			done := false

			if pvc.DeletionTimestamp == nil {
				if pvc.Status.Phase == v1.ClaimBound {
					claimPropertySets = append(claimPropertySets, cdiv1.ClaimPropertySet{AccessModes: pvc.Spec.AccessModes, VolumeMode: pvc.Spec.VolumeMode})
					done = true
				} else {
					fieldSelector, _ := fields.ParseSelector(eventInvolvedObjectField + "=" + getKey(pvc.Namespace, pvc.Name))
					events := &v1.EventList{}
					if err := r.client.List(context.TODO(), events, &client.ListOptions{FieldSelector: fieldSelector}); err != nil {
						return err
					}
					for _, e := range events.Items {
						if e.Reason == "ProvisioningFailed" {
							done = true
							break
						}
					}
				}
			}

			if done {
				dv := &cdiv1.DataVolume{ObjectMeta: metav1.ObjectMeta{Name: pvc.Name, Namespace: pvc.Namespace}}
				if err := r.client.Delete(context.TODO(), dv); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}

		}
	}

	storageProfile.Status.ClaimPropertySets = sortUniqueClaimPropertySets(claimPropertySets)

	return nil
}

func sortUniqueClaimPropertySets(claimPropertySets []cdiv1.ClaimPropertySet) []cdiv1.ClaimPropertySet {
	sets := []cdiv1.ClaimPropertySet{}
	for _, am := range accessModes {
		for _, vm := range volumeModes {
			accessMode := am
			volumeMode := vm
			set := cdiv1.ClaimPropertySet{
				AccessModes: []v1.PersistentVolumeAccessMode{accessMode},
				VolumeMode:  &volumeMode,
			}
			if claimPropertySetExists(claimPropertySets, set) {
				sets = append(sets, set)
			}
		}
	}
	return sets
}

func claimPropertySetExists(claimPropertySets []cdiv1.ClaimPropertySet, claimPropertySet cdiv1.ClaimPropertySet) bool {
	for _, cpSet := range claimPropertySets {
		if reflect.DeepEqual(cpSet, claimPropertySet) {
			return true
		}
	}
	return false
}

// FIXME: try plain pvc or blank dv instead (handle wffc)
func (r *StorageProfileReconciler) createDvs(storageProfile *cdiv1.StorageProfile) error {
	for _, am := range accessModes {
		for _, vm := range volumeModes {
			if err := r.createDv(storageProfile, am, vm); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *StorageProfileReconciler) createDv(storageProfile *cdiv1.StorageProfile, accessMode v1.PersistentVolumeAccessMode, volumeMode v1.PersistentVolumeMode) error {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getDvName(storageProfile.Name, accessMode, volumeMode),
			Namespace: r.cdiNamespace,
			Annotations: map[string]string{
				cc.AnnImmediateBinding: "true",
			},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: "http://none",
				},
			},
			PVC: &v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{accessMode},
				VolumeMode:       &volumeMode,
				StorageClassName: &storageProfile.Name,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1"),
					},
				},
			},
		},
	}

	//FIXME: controllerutil.SetControllerReference() for deletion

	return r.client.Create(context.TODO(), dv)
}

func getDvName(storageClassName string, accessMode v1.PersistentVolumeAccessMode, volumeMode v1.PersistentVolumeMode) string {
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", storageClassName, accessMode, volumeMode))
}

func (r *StorageProfileReconciler) reconcileCloneStrategy(sc *storagev1.StorageClass, clonestrategy *cdiv1.CDICloneStrategy) *cdiv1.CDICloneStrategy {

	if clonestrategy == nil {
		if sc.Annotations["cdi.kubevirt.io/clone-strategy"] == "copy" {
			strategy := cdiv1.CloneStrategyHostAssisted
			return &strategy
		} else if sc.Annotations["cdi.kubevirt.io/clone-strategy"] == "snapshot" {
			strategy := cdiv1.CloneStrategySnapshot
			return &strategy
		} else if sc.Annotations["cdi.kubevirt.io/clone-strategy"] == "csi-clone" {
			strategy := cdiv1.CloneStrategyCsiClone
			return &strategy
		} else {
			return clonestrategy
		}
	}
	return clonestrategy
}

func (r *StorageProfileReconciler) createEmptyStorageProfile(sc *storagev1.StorageClass) (*cdiv1.StorageProfile, error) {
	storageProfile := MakeEmptyStorageProfileSpec(sc.Name)
	util.SetRecommendedLabels(storageProfile, r.installerLabels, "cdi-controller")
	// uncachedClient is used to directly get the resource, SetOwnerRuntime requires some cluster-scoped resources
	// normal/cached client does list resource, a cdi user might not have the rights to list cluster scope resource
	if err := operator.SetOwnerRuntime(r.uncachedClient, storageProfile); err != nil {
		return nil, err
	}
	return storageProfile, nil
}

func (r *StorageProfileReconciler) deleteStorageProfile(name string, log logr.Logger) error {
	log.Info("Cleaning up StorageProfile that corresponds to deleted StorageClass", "StorageClass.Name", name)
	storageProfileObj := &cdiv1.StorageProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.client.Delete(context.TODO(), storageProfileObj); cc.IgnoreNotFound(err) != nil {
		return err
	}

	return r.checkIncompleteProfiles()
}

func isNoProvisioner(name string, cl client.Client) bool {
	storageClass := &storagev1.StorageClass{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name}, storageClass); err != nil {
		return false
	}
	return storageClass.Provisioner == "kubernetes.io/no-provisioner"
}

func (r *StorageProfileReconciler) checkIncompleteProfiles() error {
	numIncomplete := 0
	storageProfileList := &cdiv1.StorageProfileList{}
	if err := r.client.List(context.TODO(), storageProfileList); err != nil {
		return err
	}
	for _, profile := range storageProfileList.Items {
		if profile.Status.Provisioner == nil {
			continue
		}
		// We don't count explicitly unsupported provisioners as incomplete
		_, found := storagecapabilities.UnsupportedProvisioners[*profile.Status.Provisioner]
		if !found && isIncomplete(profile.Status.ClaimPropertySets) {
			numIncomplete++
		}
	}
	IncompleteProfileGauge.Set(float64(numIncomplete))

	return nil
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
		cdiNamespace:    util.GetNamespace(),
		installerLabels: installerLabels,
	}

	storageProfileController, err := controller.New(
		"storageprofile-controller",
		mgr,
		controller.Options{Reconciler: reconciler})
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

	cdiNamespace := util.GetNamespace()

	//FIXME: watch only the relevant PVCs using annotation index
	if err := c.Watch(&source.Kind{Type: &v1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			if obj.GetNamespace() != cdiNamespace {
				return []reconcile.Request{}
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: *obj.(*v1.PersistentVolumeClaim).Spec.StorageClassName},
			}}
		},
	)); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &v1.Event{}, eventInvolvedObjectField, func(obj client.Object) []string {
		involvedObj := obj.(*v1.Event).InvolvedObject
		return []string{getKey(involvedObj.Namespace, involvedObj.Name)}
	}); err != nil {
		return err
	}

	//FIXME: watch only the relevant PVC Events
	if err := c.Watch(&source.Kind{Type: &v1.Event{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			involvedObj := obj.(*v1.Event).InvolvedObject
			if involvedObj.Kind != "PersistentVolumeClaim" || involvedObj.Namespace != cdiNamespace {
				return []reconcile.Request{}
			}
			pvc := &v1.PersistentVolumeClaim{}
			if err := mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: involvedObj.Name, Namespace: involvedObj.Namespace}, pvc); err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, "Unable to get CDI namespace PVC", "PVC name", involvedObj.Name)
				}
				return []reconcile.Request{}
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: *pvc.Spec.StorageClassName},
			}}
		},
	)); err != nil {
		return err
	}
	return nil
}

func getKey(namespace, name string) string {
	return namespace + "/" + name
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
