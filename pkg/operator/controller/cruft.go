/*
Copyright 2018 The CDI Authors.

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

package controller

import (
	"context"
	"fmt"
	"reflect"

	secv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/apiserver"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
)

func reconcileDeleteDeprecatedResources(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) {
		return nil
	}

	namespace := deployment.GetNamespace()
	deprecatedResources := []client.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cdi-insecure-registries",
				Namespace: namespace,
			},
		},
	}

	for _, k := range deprecatedResources {
		cr := args.Resource.(runtime.Object)
		if err := args.Client.Delete(context.TODO(), k); err != nil {
			if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
				// Doesn't exist or CRD not installed, we're fine
				continue
			}
			args.Recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed to delete deprecated resource %s, %v", k.GetName(), err))
			return err
		}
		args.Recorder.Event(cr, corev1.EventTypeNormal, deleteResourceSuccess, fmt.Sprintf("Deleted deprecated resource %s successfully", k.GetName()))
	}

	return nil
}

func reconcileRemainingRelationshipLabels(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) || !sdk.CheckDeploymentReady(deployment) {
		return nil
	}
	namespace := deployment.GetNamespace()
	cr := args.Resource.(*cdiv1.CDI)
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)
	remainingResources := []client.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      operator.ConfigMapName,
				Namespace: namespace,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.CDIControllerLeaderElectionHelperName,
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apiserver.APISigningKeySecretName,
				Namespace: namespace,
			},
		},
		// &routev1.Route{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      uploadProxyRouteName,
		// 		Namespace: namespace,
		// 	},
		// },
		&secv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Name: sccName,
			},
		},
	}

	for _, k := range remainingResources {
		nn := client.ObjectKeyFromObject(k)
		if err := args.Client.Get(context.TODO(), nn, k); err != nil {
			if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
				// Doesn't exist or CRD not installed, we're fine
				continue
			}
			return err
		}
		// Exists, lets update labels if needed
		labelsCopy := util.MergeLabels(k.GetLabels(), map[string]string{})
		util.SetRecommendedLabels(k, installerLabels, "cdi-operator")
		if !reflect.DeepEqual(labelsCopy, k.GetLabels()) {
			if err := args.Client.Update(context.TODO(), k); err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete after we no longer want to include CDI CRD v1alpha1 version in release YAMLs
// Special code needed because we're not the owner of this object.
func (r *ReconcileCDI) watchCDICRD() error {
	if err := r.controller.Watch(source.Kind(r.getCache(), &extv1.CustomResourceDefinition{}, handler.TypedEnqueueRequestsFromMapFunc[*extv1.CustomResourceDefinition](
		func(ctx context.Context, obj *extv1.CustomResourceDefinition) []reconcile.Request {
			name := obj.GetName()
			if name != "cdis.cdi.kubevirt.io" {
				return nil
			}
			cr, err := cc.GetActiveCDI(ctx, r.client)
			if err != nil || cr == nil {
				return nil
			}
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "",
						Name:      cr.Name,
					},
				},
			}
		},
	))); err != nil {
		return err
	}

	return nil
}

// ReconcileCDICRD monitors the CDI CRD and removes the alpha version from it.
// Delete after we no longer want to include CDI CRD v1alpha1 version in release YAMLs
// Remove alpha as a version from the CDI CRD
func reconcileCDICRD(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}
	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) {
		return nil
	}

	crd := &extv1.CustomResourceDefinition{}
	crdKey := client.ObjectKey{Namespace: "", Name: "cdis.cdi.kubevirt.io"}
	err := args.Client.Get(context.TODO(), crdKey, crd)
	crdCopy := crd.DeepCopy()
	if err != nil {
		if errors.IsNotFound(err) {
			args.Logger.Info("CDI CRD does not exist")
			return nil
		}
		args.Logger.Error(err, "Failed to get CDI CRD")
		return err
	}

	desiredVersion := newestVersion(crd)
	if olderVersionsExist(desiredVersion, crd) {
		args.Logger.Info("Old version is in CDI CRD status.storedVersion, rewriting objects and removing...")
		if !desiredIsStorage(desiredVersion, crd) {
			return err
		}
		if err := rewriteOldObjects(args, desiredVersion, crd); err != nil {
			return err
		}
		if err := removeStoredVersion(args, desiredVersion, crd); err != nil {
			return err
		}
	} else {
		removeOldVersions(desiredVersion, crd)
		if !reflect.DeepEqual(crdCopy, crd) {
			args.Logger.Info("Old version not in CDI CRD status.storedVersion, removing also from CRD spec")
			if err := args.Client.Update(context.TODO(), crd); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete after we no longer want to include CDI CRD v1alpha1 version in release YAMLs
func removeOldVersions(desiredVersion string, crd *extv1.CustomResourceDefinition) *extv1.CustomResourceDefinition {
	newVersions := make([]extv1.CustomResourceDefinitionVersion, 0)
	for _, version := range crd.Spec.Versions {
		if version.Name == desiredVersion {
			newVersions = append(newVersions, version)
		}
	}
	crd.Spec.Versions = newVersions
	return crd
}
