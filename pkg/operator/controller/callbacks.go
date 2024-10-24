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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cdicontroller "kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
)

func addReconcileCallbacks(r *ReconcileCDI) {
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileSCC)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileCreatePrometheusInfra)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileRemainingRelationshipLabels)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteDeprecatedResources)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileCDICRD)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcilePvcMutatingWebhook)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileSetConfigAuthority)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileHandleOldVersion)
	if r.haveRoutes {
		r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileRoute)
	}
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

func reconcileRoute(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) || !sdk.CheckDeploymentReady(deployment) {
		return nil
	}

	cr := args.Resource.(runtime.Object)
	if err := ensureUploadProxyRouteExists(context.TODO(), args.Logger, args.Client, args.Scheme, deployment); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure upload proxy route exists, %v", err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, "Successfully ensured upload proxy route exists")

	if err := updateUserRoutes(context.TODO(), args.Logger, args.Client, args.Recorder); err != nil {
		return err
	}

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
	updated, err := ensureSCCExists(context.TODO(), args.Logger, args.Client, args.Namespace, common.ControllerServiceAccountName, common.CronJobServiceAccountName)
	if err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure SecurityContextConstraint exists, %v", err))
		return err
	}

	if updated {
		args.Recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, "Successfully ensured SecurityContextConstraint exists")
	}

	return nil
}

func reconcileCreatePrometheusInfra(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	// we don't check sdk.CheckDeploymentReady(deployment) since we want Prometheus to cover NotReady state as well
	if !isControllerDeployment(deployment) {
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
	if err := ensurePrometheusResourcesExist(context.TODO(), args.Client, args.Scheme, deployment); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure prometheus resources exists, %v", err))
		return err
	}

	return nil
}

func deleteWorkerResources(l logr.Logger, c client.Client) error {
	listTypes := []client.ObjectList{&corev1.PodList{}, &corev1.ServiceList{}}

	ls, err := labels.Parse(fmt.Sprintf("cdi.kubevirt.io in (%s, %s, %s)",
		common.ImporterPodName, common.UploadServerCDILabel, common.ClonerSourcePodName))
	if err != nil {
		return err
	}

	for _, lt := range listTypes {
		lo := &client.ListOptions{
			LabelSelector: ls,
		}

		l.V(1).Info("Deleting worker resources", "type", reflect.TypeOf(lt).Elem().Name())

		if err := cc.BulkDeleteResources(context.TODO(), c, lt, lo); err != nil {
			return err
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
		restoreOlderVersions(currentCrd, desiredCrd)
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
	return specVersion != nil && specVersion.Storage
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

func reconcilePvcMutatingWebhook(args *callbacks.ReconcileCallbackArgs) error {
	if args.State != callbacks.ReconcileStatePostRead {
		return nil
	}

	deployment, ok := args.DesiredObject.(*appsv1.Deployment)
	if !ok || deployment.Name != common.CDIApiServerResourceName {
		return nil
	}

	enabled, err := featuregates.IsWebhookPvcRenderingEnabled(args.Client)
	if err != nil {
		return cc.IgnoreNotFound(err)
	}

	whc := &admissionregistrationv1.MutatingWebhookConfiguration{}
	key := client.ObjectKey{Name: "cdi-api-pvc-mutate"}
	err = args.Client.Get(context.TODO(), key, whc)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	exists := err == nil
	if !enabled {
		if !exists {
			return nil
		}
		err = args.Client.Delete(context.TODO(), whc)
		return client.IgnoreNotFound(err)
	}

	if !exists {
		if err := initPvcMutatingWebhook(whc, args); err != nil {
			return err
		}
		return args.Client.Create(context.TODO(), whc)
	}

	whcCopy := whc.DeepCopy()
	if err := initPvcMutatingWebhook(whc, args); err != nil {
		return err
	}
	if !reflect.DeepEqual(whc, whcCopy) {
		return args.Client.Update(context.TODO(), whc)
	}

	return nil
}

func initPvcMutatingWebhook(whc *admissionregistrationv1.MutatingWebhookConfiguration, args *callbacks.ReconcileCallbackArgs) error {
	path := "/pvc-mutate"
	defaultServicePort := int32(443)
	allScopes := admissionregistrationv1.AllScopes
	exactPolicy := admissionregistrationv1.Exact
	failurePolicy := admissionregistrationv1.Fail
	defaultTimeoutSeconds := int32(10)
	reinvocationNever := admissionregistrationv1.NeverReinvocationPolicy
	sideEffect := admissionregistrationv1.SideEffectClassNone
	bundle := cluster.GetAPIServerCABundle(args.Namespace, args.Client, args.Logger)

	whc.Name = "cdi-api-pvc-mutate"
	whc.Labels = map[string]string{utils.CDILabel: cluster.APIServerServiceName}
	whc.Webhooks = []admissionregistrationv1.MutatingWebhook{
		{
			Name: "pvc-mutate.cdi.kubevirt.io",
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{corev1.SchemeGroupVersion.Group},
					APIVersions: []string{corev1.SchemeGroupVersion.Version},
					Resources:   []string{"persistentvolumeclaims"},
					Scope:       &allScopes,
				},
			}},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: args.Namespace,
					Name:      cluster.APIServerServiceName,
					Path:      &path,
					Port:      &defaultServicePort,
				},
				CABundle: bundle,
			},
			FailurePolicy:     &failurePolicy,
			SideEffects:       &sideEffect,
			MatchPolicy:       &exactPolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			TimeoutSeconds:    &defaultTimeoutSeconds,
			AdmissionReviewVersions: []string{
				"v1",
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.PvcApplyStorageProfileLabel: "true",
				},
			},
			ReinvocationPolicy: &reinvocationNever,
		},
	}

	cdi, err := cc.GetActiveCDI(context.TODO(), args.Client)
	if err != nil {
		return err
	}

	return controllerutil.SetControllerReference(cdi, whc, args.Scheme)
}
