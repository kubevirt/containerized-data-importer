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

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kelseyhightower/envconfig"

	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdicluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var log = logf.Log.WithName("controller_cdi")

// Add creates a new CDI Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return r.add(mgr)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileCDI, error) {
	var namespacedArgs cdinamespaced.FactoryArgs
	namespace := util.GetNamespace()
	clusterArgs := &cdicluster.FactoryArgs{Namespace: namespace}

	err := envconfig.Process("", &namespacedArgs)
	if err != nil {
		return nil, err
	}

	namespacedArgs.Namespace = namespace

	log.Info("", "VARS", fmt.Sprintf("%+v", namespacedArgs))

	r := &ReconcileCDI{
		client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		clusterArgs:    clusterArgs,
		namespacedArgs: &namespacedArgs,
	}
	return r, nil
}

var _ reconcile.Reconciler = &ReconcileCDI{}

// ReconcileCDI reconciles a CDI object
type ReconcileCDI struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	clusterArgs    *cdicluster.FactoryArgs
	namespacedArgs *cdinamespaced.FactoryArgs
}

// Reconcile reads that state of the cluster for a CDI object and makes changes based on the state read
// and what is in the CDI.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCDI) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CDI")

	// Fetch the CDI instance
	// check at cluster level
	instance := &cdiv1alpha1.CDI{}
	instanceKey := client.ObjectKey{Namespace: "", Name: request.NamespacedName.Name}
	if err := r.client.Get(context.TODO(), instanceKey, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("CR marked for deleteion, let garbage collection clean up")
		return reconcile.Result{}, nil
	}

	resources, err := r.getAllResources()
	if err != nil {
		return reconcile.Result{}, nil
	}

	for _, desiredRuntimeObj := range resources {
		desiredMetaObj, ok := desiredRuntimeObj.(metav1.Object)
		if !ok {
			reqLogger.Info("Bad resource type", "type", fmt.Sprintf("%T", desiredMetaObj))
			continue
		}

		// use reflection to create default instance of desiredRuntimeObj type
		typ := reflect.ValueOf(desiredRuntimeObj).Elem().Type()
		currentRuntimeObj := reflect.New(typ).Interface().(runtime.Object)

		key := client.ObjectKey{
			Namespace: desiredMetaObj.GetNamespace(),
			Name:      desiredMetaObj.GetName(),
		}
		err = r.client.Get(context.TODO(), key, currentRuntimeObj)

		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			if err = controllerutil.SetControllerReference(instance, desiredMetaObj, r.scheme); err != nil {
				return reconcile.Result{}, err
			}

			if err = r.client.Create(context.TODO(), desiredRuntimeObj); err != nil {
				reqLogger.Error(err, "")
				return reconcile.Result{}, err
			}

			reqLogger.Info("Resource created",
				"namespace", desiredMetaObj.GetNamespace(),
				"name", desiredMetaObj.GetName(),
				"Kind", desiredRuntimeObj.GetObjectKind())
		} else {
			currentMetaObj := currentRuntimeObj.(metav1.Object)

			if !metav1.IsControlledBy(currentMetaObj, instance) {
				// This can happen if multiple CRs exist
				log.Info("Ignoring resource for CR")
				continue
			}

			// allow users to add new annotations (but not change ours)
			mergeLabelsAndAnnotations(currentMetaObj, desiredMetaObj)

			desiredBytes, err := json.Marshal(desiredRuntimeObj)
			if err != nil {
				return reconcile.Result{}, err
			}

			if err = json.Unmarshal(desiredBytes, currentRuntimeObj); err != nil {
				return reconcile.Result{}, err
			}

			if err = r.client.Update(context.TODO(), currentRuntimeObj); err != nil {
				return reconcile.Result{}, err
			}

			reqLogger.Info("Resource updated",
				"namespace", desiredMetaObj.GetNamespace(),
				"name", desiredMetaObj.GetName(),
				"Kind", desiredRuntimeObj.GetObjectKind())
		}
	}

	reqLogger.Info("Reconcile completed successfully")

	return reconcile.Result{}, nil
}

func (r *ReconcileCDI) add(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("cdi-operator-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return r.watch(c)
}

func (r *ReconcileCDI) watch(c controller.Controller) error {
	// Watch for changes to CR
	if err := c.Watch(&source.Kind{Type: &cdiv1alpha1.CDI{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	resources, err := r.getAllResources()
	if err != nil {
		return err
	}

	return r.watchTypes(c, resources)
}

func (r *ReconcileCDI) getAllResources() ([]runtime.Object, error) {
	var resources []runtime.Object

	if deployClusterResources() {
		crs, err := cdicluster.CreateAllResources(r.clusterArgs)
		if err != nil {
			return nil, err
		}

		resources = append(resources, crs...)
	}

	nsrs, err := cdinamespaced.CreateAllResources(r.namespacedArgs)
	if err != nil {
		return nil, err
	}

	resources = append(resources, nsrs...)

	return resources, nil
}

func (r *ReconcileCDI) watchTypes(c controller.Controller, resources []runtime.Object) error {
	types := map[string]bool{}

	for _, resource := range resources {
		t := fmt.Sprintf("%T", resource)
		if types[t] {
			continue
		}

		eventHandler := &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &cdiv1alpha1.CDI{},
		}

		if err := c.Watch(&source.Kind{Type: resource}, eventHandler); err != nil {
			return err
		}

		log.Info("Watching", "type", t)

		types[t] = true
	}

	return nil
}

func mergeLabelsAndAnnotations(src, dest metav1.Object) {
	// allow users to add labels but not change ours
	for k, v := range src.GetLabels() {
		if dest.GetLabels() == nil {
			dest.SetLabels(map[string]string{})
		}
		_, exists := dest.GetLabels()[k]
		if !exists {
			dest.GetLabels()[k] = v
		}
	}

	// same for annotations
	for k, v := range src.GetAnnotations() {
		if dest.GetAnnotations() == nil {
			dest.SetAnnotations(map[string]string{})
		}
		_, exists := dest.GetAnnotations()[k]
		if !exists {
			dest.GetAnnotations()[k] = v
		}
	}
}

func deployClusterResources() bool {
	return strings.ToLower(os.Getenv("DEPLOY_CLUSTER_RESOURCES")) != "false"
}
