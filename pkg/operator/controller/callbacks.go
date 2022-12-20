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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cdicontroller "kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
)

func addReconcileCallbacks(r *ReconcileCDI) {
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileServiceAccountRead)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileServiceAccounts)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileSCC)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileCreateRoute)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileCreatePrometheusInfra)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileRemainingRelationshipLabels)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteSecrets)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileInitializeCRD)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileSetConfigAuthority)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileHandleOldVersion)
}

func isControllerDeployment(d *appsv1.Deployment) bool {
	return d.Name == "cdi-deployment"
}

func reconcileDeleteControllerDeployment(args *callbacks.ReconcileCallbackArgs) error {
	switch args.State {
	case callbacks.ReconcileStatePostDelete, callbacks.ReconcileStateOperatorDelete:
	default:
		return nil
	}

	var deployment *appsv1.Deployment
	if args.DesiredObject != nil {
		deployment = args.DesiredObject.(*appsv1.Deployment)
	} else if args.CurrentObject != nil {
		deployment = args.CurrentObject.(*appsv1.Deployment)
	} else {
		args.Logger.Info("Received callback with no desired/current object")
		return nil
	}

	if !isControllerDeployment(deployment) {
		return nil
	}

	args.Logger.Info("Deleting CDI deployment and all import/upload/clone pods/services")
	err := args.Client.Delete(context.TODO(), deployment, &client.DeleteOptions{
		PropagationPolicy: &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
	})
	cr := args.Resource.(runtime.Object)
	if err != nil && !errors.IsNotFound(err) {
		args.Logger.Error(err, "Error deleting cdi controller deployment")
		args.Recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed to delete deployment %s, %v", deployment.Name, err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, deleteResourceSuccess, fmt.Sprintf("Deleted deployment %s successfully", deployment.Name))

	if err = deleteWorkerResources(args.Logger, args.Client); err != nil {
		args.Logger.Error(err, "Error deleting worker resources")
		args.Recorder.Event(cr, corev1.EventTypeWarning, deleteResourceFailed, fmt.Sprintf("Failed to deleted worker resources %v", err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, deleteResourceSuccess, "Deleted worker resources successfully")

	return nil
}

func reconcileCreateRoute(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) || !sdk.CheckDeploymentReady(deployment) {
		return nil
	}

	cr := args.Resource.(runtime.Object)
	if err := ensureUploadProxyRouteExists(args.Logger, args.Client, args.Scheme, deployment); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure upload proxy route exists, %v", err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, "Successfully ensured upload proxy route exists")

	return nil
}

func reconcileSCC(args *callbacks.ReconcileCallbackArgs) error {
	switch args.State {
	case callbacks.ReconcileStatePreCreate, callbacks.ReconcileStatePostRead:
	default:
		return nil
	}

	sa := args.DesiredObject.(*corev1.ServiceAccount)
	if sa.Name != common.ControllerServiceAccountName {
		return nil
	}

	cr := args.Resource.(runtime.Object)
	if err := ensureSCCExists(args.Logger, args.Client, args.Namespace, common.ControllerServiceAccountName, common.CronJobServiceAccountName); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure SecurityContextConstraint exists, %v", err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, "Successfully ensured SecurityContextConstraint exists")

	return nil
}

func reconcileCreatePrometheusInfra(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) || !sdk.CheckDeploymentReady(deployment) {
		return nil
	}

	cr := args.Resource.(runtime.Object)
	namespace := deployment.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("cluster scoped owner not supported")
	}

	if deployed, err := isPrometheusDeployed(args.Logger, args.Client, namespace); err != nil {
		return err
	} else if !deployed {
		return nil
	}
	if err := ensurePrometheusResourcesExist(args.Client, args.Scheme, deployment); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure prometheus resources exists, %v", err))
		return err
	}

	return nil
}

func deleteWorkerResources(l logr.Logger, c client.Client) error {
	listTypes := []client.ObjectList{&corev1.PodList{}, &corev1.ServiceList{}}

	for _, lt := range listTypes {
		ls, err := labels.Parse(fmt.Sprintf("cdi.kubevirt.io in (%s, %s, %s)",
			common.ImporterPodName, common.UploadServerCDILabel, common.ClonerSourcePodName))
		if err != nil {
			return err
		}

		lo := &client.ListOptions{
			LabelSelector: ls,
		}

		if err := c.List(context.TODO(), lt, lo); err != nil {
			l.Error(err, "Error listing resources")
			return err
		}

		sv := reflect.ValueOf(lt).Elem()
		iv := sv.FieldByName("Items")

		for i := 0; i < iv.Len(); i++ {
			obj := iv.Index(i).Addr().Interface().(client.Object)
			l.Info("Deleting", "type", reflect.TypeOf(obj), "obj", obj)
			if err := c.Delete(context.TODO(), obj); err != nil {
				l.Error(err, "Error deleting a resource")
				return err
			}
		}
	}

	return nil
}

func reconcileSetConfigAuthority(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	crd := args.CurrentObject.(*extv1.CustomResourceDefinition)
	if crd.Name != "cdiconfigs.cdi.kubevirt.io" {
		return nil
	}

	cdi, ok := args.Resource.(*cdiv1.CDI)
	if !ok {
		return nil
	}

	if _, ok = cdi.Annotations[cdicontroller.AnnConfigAuthority]; ok {
		return nil
	}

	if cdi.Spec.Config == nil {
		cl := &cdiv1.CDIConfigList{}
		err := args.Client.List(context.TODO(), cl)
		if err != nil {
			if meta.IsNoMatchError(err) {
				return nil
			}

			return err
		}

		if len(cl.Items) != 1 {
			return nil
		}

		cs := cl.Items[0].Spec.DeepCopy()
		if !reflect.DeepEqual(cs, &cdiv1.CDIConfigSpec{}) {
			cdi.Spec.Config = cs
		}
	}

	if cdi.Annotations == nil {
		cdi.Annotations = map[string]string{}
	}
	cdi.Annotations[cdicontroller.AnnConfigAuthority] = ""

	return args.Client.Update(context.TODO(), cdi)
}

func getSpecVersion(version string, crd *extv1.CustomResourceDefinition) *extv1.CustomResourceDefinitionVersion {
	for _, v := range crd.Spec.Versions {
		if v.Name == version {
			return &v
		}
	}
	return nil
}

func rewriteOldObjects(args *callbacks.ReconcileCallbackArgs, version string, crd *extv1.CustomResourceDefinition) error {
	args.Logger.Info("Rewriting old objects")
	kind := crd.Spec.Names.Kind
	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: version,
		Kind:    kind,
	}
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(gvk)
	err := args.Client.List(context.TODO(), ul, &client.ListOptions{})
	if err != nil {
		return err
	}
	for _, item := range ul.Items {
		nn := client.ObjectKey{Namespace: item.GetNamespace(), Name: item.GetName()}
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(item.GetObjectKind().GroupVersionKind())
		err = args.Client.Get(context.TODO(), nn, u)
		if err != nil {
			return err
		}
		err = args.Client.Update(context.TODO(), u)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeStoredVersion(args *callbacks.ReconcileCallbackArgs, desiredVersion string, crd *extv1.CustomResourceDefinition) error {
	args.Logger.Info("Removing stored version")
	crd.Status.StoredVersions = []string{desiredVersion}
	return args.Client.Status().Update(context.TODO(), crd)
}

// Handle upgrade from clusters that had v1alpha1 as a storage version
// and remove it from all CRDs managed by us
func reconcileHandleOldVersion(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}
	currentCrd := args.CurrentObject.(*extv1.CustomResourceDefinition)
	desiredCrd := args.DesiredObject.(*extv1.CustomResourceDefinition)
	desiredVersion := newestVersion(desiredCrd)

	if olderVersionsExist(desiredVersion, currentCrd) {
		desiredCrd = restoreOlderVersions(currentCrd, desiredCrd)
		if !desiredIsStorage(desiredVersion, currentCrd) {
			// Let kubernetes add it
			return nil
		}
		if err := rewriteOldObjects(args, desiredVersion, currentCrd); err != nil {
			return err
		}
		if err := removeStoredVersion(args, desiredVersion, currentCrd); err != nil {
			return err
		}
	}
	return nil
}

func olderVersionsExist(desiredVersion string, crd *extv1.CustomResourceDefinition) bool {
	for _, version := range crd.Status.StoredVersions {
		if version != desiredVersion {
			return true
		}
	}
	return false
}

func desiredIsStorage(desiredVersion string, crd *extv1.CustomResourceDefinition) bool {
	specVersion := getSpecVersion(desiredVersion, crd)
	return specVersion != nil && specVersion.Storage == true
}

func newestVersion(crd *extv1.CustomResourceDefinition) string {
	orderedVersions := []string{"v1", "v1beta1", "v1alpha1"}
	for _, version := range orderedVersions {
		specVersion := getSpecVersion(version, crd)
		if specVersion != nil {
			return version
		}
	}
	return ""
}

// Merge both old and new versions into new CRD, so we have both in the desiredCrd object
func restoreOlderVersions(currentCrd, desiredCrd *extv1.CustomResourceDefinition) *extv1.CustomResourceDefinition {
	for _, version := range currentCrd.Status.StoredVersions {
		specVersion := getSpecVersion(version, desiredCrd)
		if specVersion == nil {
			// Not available in desired CRD, restore from current CRD
			specVersion := getSpecVersion(version, currentCrd)
			// We are only allowed one storage version
			// The desired CRD already has one
			specVersion.Storage = false

			desiredCrd.Spec.Versions = append(desiredCrd.Spec.Versions, *specVersion)
		}
	}
	return desiredCrd
}
