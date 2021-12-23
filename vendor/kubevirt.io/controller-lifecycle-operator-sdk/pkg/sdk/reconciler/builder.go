package reconciler

import (
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewReconciler creates new Reconciler instance configured with given parameters
func NewReconciler(crManager CrManager, log logr.Logger, client client.Client, callbackDispatcher CallbackDispatcher, scheme *runtime.Scheme, createVersionLabel string, updateVersionLabel string, lastAppliedConfigAnnotation string, perishablesSyncInterval time.Duration, finalizerName string, subresourceEnabled bool, recorder record.EventRecorder) *Reconciler {
	return &Reconciler{
		crManager:                     crManager,
		log:                           log,
		client:                        client,
		callbackDispatcher:            callbackDispatcher,
		scheme:                        scheme,
		recorder:                      recorder,
		createVersionLabel:            createVersionLabel,
		updateVersionLabel:            updateVersionLabel,
		lastAppliedConfigAnnotation:   lastAppliedConfigAnnotation,
		perishablesSyncInterval:       perishablesSyncInterval,
		finalizerName:                 finalizerName,
		syncPerishables:               syncPerishables,
		updateControllerConfiguration: updateControllerConfiguration,
		checkSanity:                   checkSanity,
		watch:                         watch,
		preCreate:                     preCreate,
		subresourceEnabled:            subresourceEnabled,
	}
}

// WithController sets controller
func (r *Reconciler) WithController(controller controller.Controller) *Reconciler {
	r.controller = controller
	return r
}

// WithNamespacedCR informs the Reconciler that the configuration CR is namespaced
func (r *Reconciler) WithNamespacedCR() *Reconciler {
	r.namespacedCR = true
	return r
}

// WithWatching sets watching flag - for testing
func (r *Reconciler) WithWatching(watching bool) *Reconciler {
	r.watching = watching
	return r
}

// WithPerishablesSynchronizer sets PerishablesSynchronizer, which must not be nil
func (r *Reconciler) WithPerishablesSynchronizer(syncPerishables PerishablesSynchronizer) *Reconciler {
	r.syncPerishables = syncPerishables
	return r
}

// WithControllerConfigUpdater sets ControllerConfigUpdater
func (r *Reconciler) WithControllerConfigUpdater(updateConfig ControllerConfigUpdater) *Reconciler {
	r.updateControllerConfiguration = updateConfig
	return r
}

// WithSanityChecker sets SanityChecker
func (r *Reconciler) WithSanityChecker(checkSanity SanityChecker) *Reconciler {
	r.checkSanity = checkSanity
	return r
}

// WithWatchRegistrator sets WatchRegistrator
func (r *Reconciler) WithWatchRegistrator(watch WatchRegistrator) *Reconciler {
	r.watch = watch
	return r
}

// WithPreCreateHook sets PreCreateHook
func (r *Reconciler) WithPreCreateHook(preCreate PreCreateHook) *Reconciler {
	if preCreate == nil {
		panic("Pre create hook mustn't be nil")
	}
	r.preCreate = preCreate
	return r
}

func preCreate(_ client.Object) error {
	return nil
}

func watch() error {
	return nil
}

func checkSanity(_ client.Object, _ logr.Logger) (*reconcile.Result, error) {
	return nil, nil
}

func updateControllerConfiguration(_ client.Object) error {
	return nil
}

func syncPerishables(cr client.Object, logger logr.Logger) error {
	return nil
}
