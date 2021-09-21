package controller

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
		return reconcile.Result{}, err
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
	storageProfile.Status.CloneStrategy = storageProfile.Spec.CloneStrategy

	storageProfile.Status.ClaimPropertySets = storageProfile.Spec.ClaimPropertySets

	var claimPropertySet *cdiv1.ClaimPropertySet
	// right now the CDI supports only a single property set
	if len(storageProfile.Status.ClaimPropertySets) > 0 {
		claimPropertySet = &storageProfile.Status.ClaimPropertySets[0]
	} else {
		claimPropertySet = &cdiv1.ClaimPropertySet{}
	}

	r.reconcileAccessModes(sc, claimPropertySet)
	r.reconcileVolumeMode(sc, claimPropertySet)

	if !isClaimPropertySetEmpty(claimPropertySet) {
		storageProfile.Status.ClaimPropertySets = []cdiv1.ClaimPropertySet{*claimPropertySet}
	}

	if err := r.updateStorageProfile(prevStorageProfile, storageProfile, log); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
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

func isClaimPropertySetEmpty(set *cdiv1.ClaimPropertySet) bool {
	return set == nil ||
		(len(set.AccessModes) == 0 && set.VolumeMode == nil)
}

func (r *StorageProfileReconciler) reconcileVolumeMode(sc *storagev1.StorageClass, claimPropertySet *cdiv1.ClaimPropertySet) {
	if claimPropertySet.VolumeMode == nil {
		capabilities, found := storagecapabilities.Get(sc)
		if found {
			claimPropertySet.VolumeMode = &capabilities.VolumeMode
		}
	}
}

func (r *StorageProfileReconciler) reconcileAccessModes(sc *storagev1.StorageClass, claimPropertySet *cdiv1.ClaimPropertySet) {
	// reconcile accessModes
	if len(claimPropertySet.AccessModes) == 0 {
		capabilities, found := storagecapabilities.Get(sc)
		if found {
			claimPropertySet.AccessModes = []v1.PersistentVolumeAccessMode{capabilities.AccessMode}
		}
	}
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
	// Add schemes.
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := storagev1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &storagev1.StorageClass{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &cdiv1.StorageProfile{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	return nil
}
