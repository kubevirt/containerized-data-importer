package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// watch registers CDI-specific watches
func (r *ReconcileCDI) watch() error {
	if err := r.reconciler.WatchResourceTypes(&corev1.ConfigMap{}, &corev1.Secret{}); err != nil {
		return err
	}

	if err := r.watchRoutes(); err != nil {
		return err
	}

	if err := r.watchSecurityContextConstraints(); err != nil {
		return err
	}
	return nil
}

// preCreate creates the operator config map
func (r *ReconcileCDI) preCreate(cr controllerutil.Object) error {
	// claim the configmap
	if err := r.createOperatorConfig(cr); err != nil {
		return err
	}
	return nil
}

// checkSanity verifies whether config map exists and is in proper relation with the cr
func (r *ReconcileCDI) checkSanity(cr controllerutil.Object, reqLogger logr.Logger) (*reconcile.Result, error) {
	configMap, err := r.getConfigMap()
	if err != nil {
		return &reconcile.Result{}, err
	}
	if !metav1.IsControlledBy(configMap, cr) {
		ownerDeleted, err := r.configMapOwnerDeleted(configMap)
		if err != nil {
			return &reconcile.Result{}, err
		}

		if ownerDeleted || configMap.DeletionTimestamp != nil {
			reqLogger.Info("Waiting for cdi-config to be deleted before reconciling", "CDI", cr.GetName())
			return &reconcile.Result{RequeueAfter: time.Second}, nil
		}

		reqLogger.Info("Reconciling to error state, unwanted CDI object")
		result, err := r.reconciler.ReconcileError(cr, "Reconciling to error state, unwanted CDI object")
		return &result, err
	}
	return nil, nil
}

// sync syncs certificates used by CDU
func (r *ReconcileCDI) sync(cr controllerutil.Object, logger logr.Logger) error {
	cdi := cr.(*cdiv1.CDI)
	return r.certManager.Sync(r.getCertificateDefinitions(cdi))
}

func (r *ReconcileCDI) configMapOwnerDeleted(cm *corev1.ConfigMap) (bool, error) {
	ownerRef := metav1.GetControllerOf(cm)
	if ownerRef != nil {
		if ownerRef.Kind != "CDI" {
			return false, fmt.Errorf("unexpected configmap owner kind %q", ownerRef.Kind)
		}

		owner := &cdiv1.CDI{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: ownerRef.Name}, owner); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, err
		}

		if owner.DeletionTimestamp == nil && owner.UID == ownerRef.UID {
			return false, nil
		}
	}

	return true, nil
}

func (r *ReconcileCDI) registerHooks() {
	r.reconciler.
		WithPreCreateHook(r.preCreate).
		WithWatchRegistrator(r.watch).
		WithSanityChecker(r.checkSanity).
		WithPerishablesSynchronizer(r.sync)
}
