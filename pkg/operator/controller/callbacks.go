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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cdicontroller "kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
)

func addReconcileCallbacks(r *ReconcileCDI) {
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileServiceAccountRead)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileServiceAccounts)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileCreateSCC)
	r.reconciler.AddCallback(&corev1.ServiceAccount{}, reconcileSELinuxPerms)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileCreateRoute)
	r.reconciler.AddCallback(&appsv1.Deployment{}, reconcileDeleteSecrets)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileInitializeCRD)
	r.reconciler.AddCallback(&extv1.CustomResourceDefinition{}, reconcileSetConfigAuthority)
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

func reconcileCreateSCC(args *callbacks.ReconcileCallbackArgs) error {
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
	if err := ensureSCCExists(args.Logger, args.Client, args.Namespace, common.ControllerServiceAccountName); err != nil {
		args.Recorder.Event(cr, corev1.EventTypeWarning, createResourceFailed, fmt.Sprintf("Failed to ensure SecurityContextConstraint exists, %v", err))
		return err
	}
	args.Recorder.Event(cr, corev1.EventTypeNormal, createResourceSuccess, "Successfully ensured SecurityContextConstraint exists")

	return nil
}

func deleteWorkerResources(l logr.Logger, c client.Client) error {
	listTypes := []runtime.Object{&corev1.PodList{}, &corev1.ServiceList{}}

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
			obj := iv.Index(i).Addr().Interface().(runtime.Object)
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
