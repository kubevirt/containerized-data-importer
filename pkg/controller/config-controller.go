package controller

import (
	"context"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	routev1 "github.com/openshift/api/route/v1"
	storagev1 "k8s.io/api/storage/v1"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// CDIConfigReconciler members
type CDIConfigReconciler struct {
	client client.Client
	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient         client.Client
	scheme                 *runtime.Scheme
	log                    logr.Logger
	uploadProxyServiceName string
	configName             string
	cdiNamespace           string
}

func isErrCacheNotStarted(err error) bool {
	if err == nil {
		return false
	}
	return err.(*cache.ErrCacheNotStarted) != nil
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *CDIConfigReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("CDIConfig", req.NamespacedName)
	log.Info("reconciling CDIConfig")

	config, err := r.createCDIConfig()
	if err != nil {
		log.Error(err, "Unable to create CDIConfig")
		return reconcile.Result{}, err
	}
	// Keep a copy of the original for comparison later.
	currentConfigCopy := config.DeepCopyObject()

	if err := r.reconcileUploadProxyURL(config, log); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileStorageClass(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileDefaultPodResourceRequirements(config); err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(currentConfigCopy, config) {
		// Updates have happened, update CDIConfig.
		log.Info("Updating CDIConfig", "CDIConfig.Name", config.Name, "config", config)
		if err := r.client.Update(context.TODO(), config); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *CDIConfigReconciler) reconcileUploadProxyURL(config *cdiv1.CDIConfig, log logr.Logger) error {
	config.Status.UploadProxyURL = config.Spec.UploadProxyURLOverride
	// No override, try Ingress
	if config.Status.UploadProxyURL == nil {
		if err := r.reconcileIngress(config); err != nil {
			log.Error(err, "Unable to reconcile Ingress")
			return err
		}
	}
	// No override or Ingress, try Route
	if config.Status.UploadProxyURL == nil {
		if err := r.reconcileRoute(config); err != nil {
			log.Error(err, "Unable to reconcile Routes")
			return err
		}
	}
	return nil
}

func (r *CDIConfigReconciler) reconcileIngress(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("IngressReconcile")
	ingressList := &extensionsv1beta1.IngressList{}
	if err := r.client.List(context.TODO(), ingressList, &client.ListOptions{}); IgnoreIsNoMatchError(err) != nil {
		return err
	}
	for _, ingress := range ingressList.Items {
		ingressURL := getURLFromIngress(&ingress, r.uploadProxyServiceName)
		if ingressURL != "" {
			log.Info("Setting upload proxy url", "IngressURL", ingressURL)
			config.Status.UploadProxyURL = &ingressURL
			return nil
		}
	}
	log.Info("No ingress found, setting to blank", "IngressURL", "")
	config.Status.UploadProxyURL = nil
	return nil
}

func (r *CDIConfigReconciler) reconcileRoute(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("RouteReconcile")
	routeList := &routev1.RouteList{}
	if err := r.client.List(context.TODO(), routeList, &client.ListOptions{}); IgnoreIsNoMatchError(err) != nil {
		return err
	}
	for _, route := range routeList.Items {
		routeURL := getURLFromRoute(&route, r.uploadProxyServiceName)
		if routeURL != "" {
			log.Info("Setting upload proxy url", "RouteURL", routeURL)
			config.Status.UploadProxyURL = &routeURL
			return nil
		}
	}
	log.Info("No route found, setting to blank", "RouteURL", "")
	config.Status.UploadProxyURL = nil
	return nil
}

func (r *CDIConfigReconciler) reconcileStorageClass(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("StorageClassReconcile")
	storageClassList := &storagev1.StorageClassList{}
	if err := r.client.List(context.TODO(), storageClassList, &client.ListOptions{}); err != nil {
		return err
	}

	// Check config for scratch space class
	if config.Spec.ScratchSpaceStorageClass != nil {
		for _, storageClass := range storageClassList.Items {
			if storageClass.Name == *config.Spec.ScratchSpaceStorageClass {
				log.Info("Setting scratch space to override", "storageClass.Name", storageClass.Name)
				config.Status.ScratchSpaceStorageClass = storageClass.Name
				return nil
			}
		}
	}
	// Check for default storage class.
	for _, storageClass := range storageClassList.Items {
		if defaultClassValue, ok := storageClass.Annotations[AnnDefaultStorageClass]; ok {
			if defaultClassValue == "true" {
				log.Info("Setting scratch space to default", "storageClass.Name", storageClass.Name)
				config.Status.ScratchSpaceStorageClass = storageClass.Name
				return nil
			}
		}
	}
	log.Info("No default storage class found, setting scratch space to blank")
	// No storage class found, blank it out.
	config.Status.ScratchSpaceStorageClass = ""
	return nil
}

func (r *CDIConfigReconciler) reconcileDefaultPodResourceRequirements(config *cdiv1.CDIConfig) error {
	config.Status.DefaultPodResourceRequirements = &v1.ResourceRequirements{
		Limits: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
			v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
		Requests: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:    *resource.NewQuantity(0, resource.DecimalSI),
			v1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI)},
	}

	if config.Spec.PodResourceRequirements != nil {
		if config.Spec.PodResourceRequirements.Limits != nil {
			if cpu, exist := config.Spec.PodResourceRequirements.Limits[v1.ResourceCPU]; exist {
				config.Status.DefaultPodResourceRequirements.Limits[v1.ResourceCPU] = cpu
			}

			if memory, exist := config.Spec.PodResourceRequirements.Limits[v1.ResourceMemory]; exist {
				config.Status.DefaultPodResourceRequirements.Limits[v1.ResourceMemory] = memory
			}
		}

		if config.Spec.PodResourceRequirements.Requests != nil {
			if cpu, exist := config.Spec.PodResourceRequirements.Requests[v1.ResourceCPU]; exist {
				config.Status.DefaultPodResourceRequirements.Requests[v1.ResourceCPU] = cpu
			}

			if memory, exist := config.Spec.PodResourceRequirements.Requests[v1.ResourceMemory]; exist {
				config.Status.DefaultPodResourceRequirements.Requests[v1.ResourceMemory] = memory
			}
		}
	}

	return nil
}

// createCDIConfig creates a new instance of the CDIConfig object if it doesn't exist already, and returns the existing one if found.
// It also sets the operator to be the owner of the CDIConfig object.
func (r *CDIConfigReconciler) createCDIConfig() (*cdiv1.CDIConfig, error) {
	config := &cdiv1.CDIConfig{}
	if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: r.configName}, config); err != nil {
		if errors.IsNotFound(err) {
			config = MakeEmptyCDIConfigSpec(r.configName)
			if err := operator.SetOwnerRuntime(r.uncachedClient, config); err != nil {
				return nil, err
			}
			if err := r.client.Create(context.TODO(), config); err != nil {
				if errors.IsAlreadyExists(err) {
					config := &cdiv1.CDIConfig{}
					if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: r.configName}, config); err == nil {
						return config, nil
					}
					return nil, err
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return config, nil
}

// Init initializes a CDIConfig object.
func (r *CDIConfigReconciler) Init() error {
	_, err := r.createCDIConfig()
	return err
}

// NewConfigController creates a new instance of the config controller.
func NewConfigController(mgr manager.Manager, log logr.Logger, uploadProxyServiceName, configName string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	reconciler := &CDIConfigReconciler{
		client:                 mgr.GetClient(),
		uncachedClient:         uncachedClient,
		scheme:                 mgr.GetScheme(),
		log:                    log.WithName("config-controller"),
		uploadProxyServiceName: uploadProxyServiceName,
		configName:             configName,
		cdiNamespace:           util.GetNamespace(),
	}

	configController, err := controller.New("config-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addConfigControllerWatches(mgr, configController, reconciler.cdiNamespace, configName, uploadProxyServiceName); err != nil {
		return nil, err
	}
	if err := reconciler.Init(); err != nil {
		log.Error(err, "Unable to initalize CDIConfig")
	}
	log.Info("Initialized CDI Config object")
	return configController, nil
}

// addConfigControllerWatches sets up the watches used by the config controller.
func addConfigControllerWatches(mgr manager.Manager, configController controller.Controller, cdiNamespace, configName, uploadProxyServiceName string) error {
	// Add schemes.
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := storagev1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := extensionsv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	// Setup watches
	if err := configController.Watch(&source.Kind{Type: &cdiv1.CDIConfig{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	err := configController.Watch(&source.Kind{Type: &storagev1.StorageClass{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		}),
	})
	if err != nil {
		return err
	}
	err = configController.Watch(&source.Kind{Type: &extensionsv1beta1.Ingress{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		}),
	}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return "" != getURLFromIngress(e.Object.(*extensionsv1beta1.Ingress), uploadProxyServiceName) &&
				e.Object.(*extensionsv1beta1.Ingress).GetNamespace() == cdiNamespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return "" != getURLFromIngress(e.ObjectNew.(*extensionsv1beta1.Ingress), uploadProxyServiceName) &&
				e.ObjectNew.(*extensionsv1beta1.Ingress).GetNamespace() == cdiNamespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return "" != getURLFromIngress(e.Object.(*extensionsv1beta1.Ingress), uploadProxyServiceName) &&
				e.Object.(*extensionsv1beta1.Ingress).GetNamespace() == cdiNamespace
		},
	})

	// check if routes exist
	err = mgr.GetClient().List(context.TODO(), &routev1.RouteList{})
	if meta.IsNoMatchError(err) {
		return nil
	}

	if err != nil && !isErrCacheNotStarted(err) {
		return err
	}

	err = configController.Watch(&source.Kind{Type: &routev1.Route{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		}),
	}, predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return "" != getURLFromRoute(e.Object.(*routev1.Route), uploadProxyServiceName) &&
				e.Object.(*routev1.Route).GetNamespace() == cdiNamespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return "" != getURLFromRoute(e.ObjectNew.(*routev1.Route), uploadProxyServiceName) &&
				e.ObjectNew.(*routev1.Route).GetNamespace() == cdiNamespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return "" != getURLFromRoute(e.Object.(*routev1.Route), uploadProxyServiceName) &&
				e.Object.(*routev1.Route).GetNamespace() == cdiNamespace
		},
	})

	return nil
}

func getURLFromIngress(ing *extensionsv1beta1.Ingress, uploadProxyServiceName string) string {
	if ing.Spec.Backend != nil {
		if ing.Spec.Backend.ServiceName != uploadProxyServiceName {
			return ""
		}
		return ing.Spec.Rules[0].Host
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.ServiceName == uploadProxyServiceName {
				if rule.Host != "" {
					return rule.Host
				}
			}
		}
	}
	return ""

}

func getURLFromRoute(route *routev1.Route, uploadProxyServiceName string) string {
	if route.Spec.To.Name == uploadProxyServiceName {
		if len(route.Status.Ingress) > 0 {
			return route.Status.Ingress[0].Host
		}
	}
	return ""

}
