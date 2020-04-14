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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// ReconcileStatePreCreate is the state before a resource is created
	ReconcileStatePreCreate ReconcileState = "PRE_CREATE"
	// ReconcileStatePostCreate is the state sfter a resource is created
	ReconcileStatePostCreate ReconcileState = "POST_CREATE"

	// ReconcileStatePostRead is the state sfter a resource is read
	ReconcileStatePostRead ReconcileState = "POST_READ"

	// ReconcileStatePreUpdate is the state before a resource is updated
	ReconcileStatePreUpdate ReconcileState = "PRE_UPDATE"
	// ReconcileStatePostUpdate is the state after a resource is updated
	ReconcileStatePostUpdate ReconcileState = "POST_UPDATE"

	// ReconcileStatePreDelete is the state before a resource is explicitly deleted (probably during upgrade)
	// don't count on this always being called for your resource
	// ideally we just let garbage collection do it's thing
	ReconcileStatePreDelete ReconcileState = "PRE_DELETE"
	// ReconcileStatePostDelete is the state after a resource is explicitly deleted (probably during upgrade)
	// don't count on this always being called for your resource
	// ideally we just let garbage collection do it's thing
	ReconcileStatePostDelete ReconcileState = "POST_DELETE"

	// ReconcileStateCDIDelete is called during CDI finalizer
	ReconcileStateCDIDelete ReconcileState = "CDI_DELETE"
)

// ReconcileState is the current state of the reconcile for a particuar resource
type ReconcileState string

// ReconcileCallbackArgs contains the data of a ReconcileCallback
type ReconcileCallbackArgs struct {
	Logger    logr.Logger
	Client    client.Client
	Scheme    *runtime.Scheme
	Namespace string
	Resource  *cdiv1alpha1.CDI

	State         ReconcileState
	DesiredObject runtime.Object
	CurrentObject runtime.Object
}

// ReconcileCallback is the callback function
type ReconcileCallback func(args *ReconcileCallbackArgs) error

func addReconcileCallbacks(r *ReconcileCDI) {
	r.addCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
	r.addCallback(&corev1.ServiceAccount{}, reconcileServiceAccountRead)
	r.addCallback(&corev1.ServiceAccount{}, reconcileServiceAccounts)
	r.addCallback(&corev1.ServiceAccount{}, reconcileCreateSCC)
	r.addCallback(&appsv1.Deployment{}, reconcileCreateRoute)
	r.addCallback(&appsv1.Deployment{}, reconcileDeleteSecrets)
}

func isControllerDeployment(d *appsv1.Deployment) bool {
	return d.Name == "cdi-deployment"
}

func reconcileDeleteControllerDeployment(args *ReconcileCallbackArgs) error {
	switch args.State {
	case ReconcileStatePostDelete, ReconcileStateCDIDelete:
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
	if err != nil && !errors.IsNotFound(err) {
		args.Logger.Error(err, "Error deleting cdi controller deployment")
		return err
	}

	if err = deleteWorkerResources(args.Logger, args.Client); err != nil {
		args.Logger.Error(err, "Error deleting worker resources")
		return err
	}

	return nil
}

func reconcileCreateRoute(args *ReconcileCallbackArgs) error {
	if args.State != ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) || !checkDeploymentReady(deployment) {
		return nil
	}

	if err := ensureUploadProxyRouteExists(args.Logger, args.Client, args.Scheme, deployment); err != nil {
		return err
	}

	return nil
}

func reconcileCreateSCC(args *ReconcileCallbackArgs) error {
	switch args.State {
	case ReconcileStatePreCreate, ReconcileStatePostRead:
	default:
		return nil
	}

	sa := args.DesiredObject.(*corev1.ServiceAccount)
	if sa.Name != common.ControllerServiceAccountName {
		return nil
	}

	if err := ensureSCCExists(
		args.Logger,
		args.Client,
		args.Scheme,
		args.Resource,
		args.Namespace,
		common.ControllerServiceAccountName,
	); err != nil {
		return err
	}

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
