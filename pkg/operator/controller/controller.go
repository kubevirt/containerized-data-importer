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
	"time"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	cdicerts "kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
	cdicluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
	sdkr "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/reconciler"
)

const (
	finalizerName = "operator.cdi.kubevirt.io"

	createVersionLabel          = "operator.cdi.kubevirt.io/createVersion"
	updateVersionLabel          = "operator.cdi.kubevirt.io/updateVersion"
	lastAppliedConfigAnnotation = "operator.cdi.kubevirt.io/lastAppliedConfiguration"

	certPollInterval = 1 * time.Minute

	createResourceFailed  = "CreateResourceFailed"
	createResourceSuccess = "CreateResourceSuccess"

	deleteResourceFailed  = "DeleteResourceFailed"
	deleteResourceSuccess = "DeleteResourceSuccess"
)

var log = logf.Log.WithName("cdi-operator")

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
	restClient := mgr.GetClient()
	clusterArgs := &cdicluster.FactoryArgs{
		Namespace: namespace,
		Client:    restClient,
		Logger:    log,
	}

	err := envconfig.Process("", &namespacedArgs)
	if err != nil {
		return nil, err
	}

	namespacedArgs.Namespace = namespace

	log.Info("", "VARS", fmt.Sprintf("%+v", namespacedArgs))

	scheme := mgr.GetScheme()
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: scheme,
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}

	recorder := mgr.GetEventRecorderFor("operator-controller")

	r := &ReconcileCDI{
		client:         restClient,
		uncachedClient: uncachedClient,
		scheme:         scheme,
		recorder:       recorder,
		namespace:      namespace,
		clusterArgs:    clusterArgs,
		namespacedArgs: &namespacedArgs,
	}
	callbackDispatcher := callbacks.NewCallbackDispatcher(log, restClient, uncachedClient, scheme, namespace)
	r.reconciler = sdkr.NewReconciler(r, log, restClient, callbackDispatcher, scheme, createVersionLabel, updateVersionLabel, lastAppliedConfigAnnotation, certPollInterval, finalizerName, recorder)

	r.registerHooks()
	addReconcileCallbacks(r)

	return r, nil
}

var _ reconcile.Reconciler = &ReconcileCDI{}

// ReconcileCDI reconciles a CDI object
type ReconcileCDI struct {
	// This Client, initialized using mgr.client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	client client.Client

	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient client.Client
	scheme         *runtime.Scheme
	recorder       record.EventRecorder
	controller     controller.Controller

	namespace      string
	clusterArgs    *cdicluster.FactoryArgs
	namespacedArgs *cdinamespaced.FactoryArgs

	certManager CertManager
	reconciler  *sdkr.Reconciler
}

// SetController sets the controller dependency
func (r *ReconcileCDI) SetController(controller controller.Controller) {
	r.controller = controller
	r.reconciler.WithController(controller)
}

// Reconcile reads that state of the cluster for a CDI object and makes changes based on the state read
// and what is in the CDI.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCDI) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	operatorVersion := r.namespacedArgs.OperatorVersion
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CDI")
	return r.reconciler.Reconcile(request, operatorVersion, reqLogger)
}

func (r *ReconcileCDI) add(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("cdi-operator-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	r.SetController(c)

	if err = r.reconciler.WatchCR(); err != nil {
		return err
	}

	cm, err := NewCertManager(mgr, r.namespace)
	if err != nil {
		return err
	}

	r.certManager = cm

	return nil
}

func (r *ReconcileCDI) getCertificateDefinitions(cdi *cdiv1.CDI) []cdicerts.CertificateDefinition {
	args := &cdicerts.FactoryArgs{Namespace: r.namespace}

	if cdi != nil && cdi.Spec.CertConfig != nil {
		if cdi.Spec.CertConfig.CA != nil {
			if cdi.Spec.CertConfig.CA.Duration != nil {
				args.SignerValidity = &cdi.Spec.CertConfig.CA.Duration.Duration
			}

			if cdi.Spec.CertConfig.CA.RenewBefore != nil {
				args.SignerRefresh = &cdi.Spec.CertConfig.CA.RenewBefore.Duration
			}
		}

		if cdi.Spec.CertConfig.Server != nil {
			if cdi.Spec.CertConfig.Server.Duration != nil {
				args.TargetValidity = &cdi.Spec.CertConfig.Server.Duration.Duration
			}

			if cdi.Spec.CertConfig.Server.RenewBefore != nil {
				args.TargetRefresh = &cdi.Spec.CertConfig.Server.RenewBefore.Duration
			}
		}
	}

	return cdicerts.CreateCertificateDefinitions(args)
}

func (r *ReconcileCDI) getConfigMap() (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Name: operator.ConfigMapName, Namespace: r.namespace}

	if err := r.client.Get(context.TODO(), key, cm); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return cm, nil
}

// createOperatorConfig creates operator config map
func (r *ReconcileCDI) createOperatorConfig(cr controllerutil.Object) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operator.ConfigMapName,
			Namespace: r.namespace,
			Labels:    map[string]string{"operator.cdi.kubevirt.io": ""},
		},
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), cm)
}
