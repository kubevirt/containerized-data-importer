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
	"os"
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	cdicerts "kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
	cdicluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/generate/install"
	cdinamespaced "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
	sdkr "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/reconciler"
)

const (
	finalizerName = "operator.cdi.kubevirt.io"

	createVersionLabel = "operator.cdi.kubevirt.io/createVersion"
	updateVersionLabel = "operator.cdi.kubevirt.io/updateVersion"
	// LastAppliedConfigAnnotation is the annotation that holds the last resource state which we put on resources under our governance
	LastAppliedConfigAnnotation = "operator.cdi.kubevirt.io/lastAppliedConfiguration"

	certPollInterval = 1 * time.Minute

	createResourceFailed  = "CreateResourceFailed"
	createResourceSuccess = "CreateResourceSuccess"

	deleteResourceFailed   = "DeleteResourceFailed"
	deleteResourceSuccess  = "DeleteResourceSuccess"
	dumpInstallStrategyKey = "DUMP_INSTALL_STRATEGY"

	annInjectUploadProxyCert = "operator.cdi.kubevirt.io/injectUploadProxyCert"
	updateUserRouteSuccess   = "UploadProxyRouteInjectSuccess"
)

var (
	log = logf.Log.WithName("cdi-operator")
)

func init() {
	// Setup metrics for our various controllers
	err := metrics.SetupMetrics()
	if err != nil {
		log.Info("Error setting up metrics: %v", err)
	}
	metrics.SetInit()
}

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
	dumpInstallStrategy := false
	if value, ok := os.LookupEnv(dumpInstallStrategyKey); ok {
		ret, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		dumpInstallStrategy = ret
		log.Info("Dump Install Strategy", "VARS", ret)
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

	err = rules.SetupRules(namespace)
	if err != nil {
		return nil, err
	}

	recorder := mgr.GetEventRecorderFor("operator-controller")

	haveRoutes, err := haveRoutes(uncachedClient)
	if err != nil {
		return nil, err
	}

	r := &ReconcileCDI{
		client:              restClient,
		uncachedClient:      uncachedClient,
		scheme:              scheme,
		getCache:            mgr.GetCache,
		recorder:            recorder,
		namespace:           namespace,
		clusterArgs:         clusterArgs,
		namespacedArgs:      &namespacedArgs,
		dumpInstallStrategy: dumpInstallStrategy,
		haveRoutes:          haveRoutes,
	}
	callbackDispatcher := callbacks.NewCallbackDispatcher(log, restClient, uncachedClient, scheme, namespace)
	r.reconciler = sdkr.NewReconciler(r, log, restClient, callbackDispatcher, scheme, mgr.GetCache, createVersionLabel, updateVersionLabel, LastAppliedConfigAnnotation, certPollInterval, finalizerName, false, recorder)

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
	getCache       func() cache.Cache
	recorder       record.EventRecorder
	controller     controller.Controller

	namespace      string
	clusterArgs    *cdicluster.FactoryArgs
	namespacedArgs *cdinamespaced.FactoryArgs

	certManager         CertManager
	reconciler          *sdkr.Reconciler
	dumpInstallStrategy bool
	haveRoutes          bool
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
func (r *ReconcileCDI) Reconcile(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CDI CR")
	operatorVersion := r.namespacedArgs.OperatorVersion
	cr := &cdiv1.CDI{}
	crKey := client.ObjectKey{Namespace: "", Name: request.NamespacedName.Name}
	err := r.client.Get(context.TODO(), crKey, cr)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("CDI CR does not exist")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get CDI object")
		return reconcile.Result{}, err
	}

	if r.dumpInstallStrategy {
		reqLogger.Info("Dumping Install Strategy")
		objects, err := r.GetAllResources(cr)
		if err != nil {
			reqLogger.Error(err, "Failed to get all CDI object")
			return reconcile.Result{}, err
		}
		var runtimeObjects []runtime.Object
		for _, obj := range objects {
			runtimeObjects = append(runtimeObjects, obj)
		}
		installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)
		err = install.DumpInstallStrategyToConfigMap(r.client, runtimeObjects, reqLogger, r.namespace, installerLabels)
		if err != nil {
			reqLogger.Error(err, "Failed to dump CDI object in configmap")
			return reconcile.Result{}, err
		}
	}

	// Ready metric so we can alert whenever we are not ready for a while
	if conditionsv1.IsStatusConditionTrue(cr.Status.Conditions, conditionsv1.ConditionAvailable) {
		metrics.SetReady()
	} else if !conditionsv1.IsStatusConditionTrue(cr.Status.Conditions, conditionsv1.ConditionProgressing) {
		// Not an issue if progress is still ongoing
		metrics.SetNotReady()
	}
	return r.reconciler.Reconcile(request, operatorVersion, reqLogger)
}

func (r *ReconcileCDI) add(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New("cdi-operator-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              r,
	})
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
				args.SignerDuration = &cdi.Spec.CertConfig.CA.Duration.Duration
			}

			if cdi.Spec.CertConfig.CA.RenewBefore != nil {
				args.SignerRenewBefore = &cdi.Spec.CertConfig.CA.RenewBefore.Duration
			}
		}

		if cdi.Spec.CertConfig.Server != nil {
			if cdi.Spec.CertConfig.Server.Duration != nil {
				args.ServerDuration = &cdi.Spec.CertConfig.Server.Duration.Duration
			}

			if cdi.Spec.CertConfig.Server.RenewBefore != nil {
				args.ServerRenewBefore = &cdi.Spec.CertConfig.Server.RenewBefore.Duration
			}
		}

		if cdi.Spec.CertConfig.Client != nil {
			if cdi.Spec.CertConfig.Client.Duration != nil {
				args.ClientDuration = &cdi.Spec.CertConfig.Client.Duration.Duration
			}

			if cdi.Spec.CertConfig.Client.RenewBefore != nil {
				args.ClientRenewBefore = &cdi.Spec.CertConfig.Client.RenewBefore.Duration
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
func (r *ReconcileCDI) createOperatorConfig(cr client.Object) error {
	cdiCR := cr.(*cdiv1.CDI)
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cdiCR)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operator.ConfigMapName,
			Namespace: r.namespace,
			Labels:    map[string]string{"operator.cdi.kubevirt.io": ""},
		},
	}
	util.SetRecommendedLabels(cm, installerLabels, "cdi-operator")

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), cm)
}
