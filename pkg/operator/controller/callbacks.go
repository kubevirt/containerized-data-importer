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
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"

	secv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
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

// ReconcileCallback is the callback function
type ReconcileCallback func(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error

func addReconcileCallbacks(r *ReconcileCDI) {
	r.addCallback(&appsv1.Deployment{}, reconcileDeleteControllerDeployment)
	r.addCallback(&corev1.ServiceAccount{}, reconcileServiceAccountRead)
	r.addCallback(&corev1.ServiceAccount{}, reconcileServiceAccounts)
}

func reconcileDeleteControllerDeployment(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error {
	switch s {
	case ReconcileStatePostDelete, ReconcileStateCDIDelete:
	default:
		return nil
	}

	deployment := desiredObj.(*appsv1.Deployment)
	if deployment.Name != "cdi-deployment" {
		return nil
	}

	l.Info("Deleting CDI deployment and all import/upload/clone pods/services")

	err := c.Delete(context.TODO(), deployment, func(opts *client.DeleteOptions) {
		p := metav1.DeletePropagationForeground
		opts.PropagationPolicy = &p
	})
	if err != nil && !errors.IsNotFound(err) {
		l.Error(err, "Error deleting cdi controller deployment")
		return err
	}

	if err = deleteWorkerResources(l, c); err != nil {
		l.Error(err, "Error deleting worker resources")
		return err
	}

	return nil
}

func reconcileServiceAccountRead(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error {
	if s != ReconcileStatePostRead {
		return nil
	}

	do := desiredObj.(*corev1.ServiceAccount)
	co := currentObj.(*corev1.ServiceAccount)

	val, exists := do.Annotations[utils.SCCAnnotation]
	if exists {
		if co.Annotations == nil {
			co.Annotations = make(map[string]string)
		}
		co.Annotations[utils.SCCAnnotation] = val
	}

	return nil
}

func reconcileServiceAccounts(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error {
	switch s {
	case ReconcileStatePreCreate, ReconcileStatePreUpdate, ReconcileStatePostDelete, ReconcileStateCDIDelete:
	default:
		return nil
	}

	do := desiredObj.(*corev1.ServiceAccount)

	desiredSCCs := []string{}
	saName := fmt.Sprintf("system:serviceaccount:%s:%s", do.Namespace, do.Name)

	switch s {
	case ReconcileStatePreCreate, ReconcileStatePreUpdate:
		val, exists := do.Annotations[utils.SCCAnnotation]
		if exists {
			if err := json.Unmarshal([]byte(val), &desiredSCCs); err != nil {
				l.Error(err, "Error unmarshalling data")
				return err
			}
		}
	default:
		// want desiredSCCs empty because deleting resource/CDI
	}

	listObj := &secv1.SecurityContextConstraintsList{}
	if err := c.List(context.TODO(), &client.ListOptions{}, listObj); err != nil {
		if meta.IsNoMatchError(err) {
			// not openshift
			return nil
		}
		l.Error(err, "Error listing SCCs")
		return err
	}

	for _, scc := range listObj.Items {
		desiredUsers := []string{}
		add := containsValue(desiredSCCs, scc.Name)
		seenUser := false

		for _, u := range scc.Users {
			if u == saName {
				seenUser = true
				if !add {
					continue
				}
			}
			desiredUsers = append(desiredUsers, u)
		}

		if add && !seenUser {
			desiredUsers = append(desiredUsers, saName)
		}

		if !reflect.DeepEqual(desiredUsers, scc.Users) {
			l.Info("Doing SCC update", "name", scc.Name, "desired", desiredUsers, "current", scc.Users)
			scc.Users = desiredUsers
			if err := c.Update(context.TODO(), &scc); err != nil {
				l.Error(err, "Error updating SCC")
				return err
			}
		}
	}

	return nil
}

func deleteWorkerResources(l logr.Logger, c client.Client) error {
	listTypes := []runtime.Object{&corev1.PodList{}, &corev1.ServiceList{}}

	for _, lt := range listTypes {
		lo := &client.ListOptions{}
		lo.SetLabelSelector(fmt.Sprintf("cdi.kubevirt.io in (%s, %s, %s)",
			common.ImporterPodName, common.UploadServerCDILabel, common.ClonerSourcePodName))

		if err := c.List(context.TODO(), lo, lt); err != nil {
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

func containsValue(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
