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

package callbacks

import (
	"context"
	"reflect"

	"k8s.io/client-go/tools/record"

	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// ReconcileStateOperatorDelete is called during CR finalizer
	ReconcileStateOperatorDelete ReconcileState = "OPERATOR_DELETE"
)

// ReconcileState is the current state of the reconcile for a particuar resource
type ReconcileState string

// ReconcileCallbackArgs contains the data of a ReconcileCallback
type ReconcileCallbackArgs struct {
	Logger    logr.Logger
	Client    client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Resource  interface{}

	State         ReconcileState
	DesiredObject runtime.Object
	CurrentObject runtime.Object
}

// CallbackDispatcher manages and executes resource callbacks
type CallbackDispatcher struct {
	log       logr.Logger
	callbacks map[reflect.Type][]ReconcileCallback
	// This Client, initialized using mgr.client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	client client.Client

	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient client.Client
	scheme         *runtime.Scheme

	namespace string
}

// ReconcileCallback is the callback function
type ReconcileCallback func(args *ReconcileCallbackArgs) error

// NewCallbackDispatcher creates new callback dispatcher
func NewCallbackDispatcher(log logr.Logger, client, uncachedClient client.Client, scheme *runtime.Scheme, namespace string) *CallbackDispatcher {
	return &CallbackDispatcher{
		log:            log,
		client:         client,
		uncachedClient: uncachedClient,
		scheme:         scheme,
		namespace:      namespace,
		callbacks:      make(map[reflect.Type][]ReconcileCallback),
	}
}

// AddCallback registers a callback for given object type
func (cd *CallbackDispatcher) AddCallback(obj runtime.Object, cb ReconcileCallback) {
	t := reflect.TypeOf(obj)
	cbs := cd.callbacks[t]
	cd.callbacks[t] = append(cbs, cb)
}

// InvokeCallbacks executes callbacks for desired/current object type
func (cd *CallbackDispatcher) InvokeCallbacks(l logr.Logger, cr interface{}, s ReconcileState, desiredObj, currentObj runtime.Object, recorder record.EventRecorder) error {
	var t reflect.Type

	if desiredObj != nil {
		t = reflect.TypeOf(desiredObj)
	} else if currentObj != nil {
		t = reflect.TypeOf(currentObj)
	}

	// callbacks with nil key always get invoked
	cbs := append(cd.callbacks[t], cd.callbacks[nil]...)

	for _, cb := range cbs {
		if s != ReconcileStatePreCreate && currentObj == nil {
			metaObj := desiredObj.(metav1.Object)
			key := client.ObjectKey{
				Namespace: metaObj.GetNamespace(),
				Name:      metaObj.GetName(),
			}

			currentObj = sdk.NewDefaultInstance(desiredObj)
			if err := cd.client.Get(context.TODO(), key, currentObj); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
				currentObj = nil
			}
		}
		args := ReconcileCallbackArgs{
			Logger:        l,
			Client:        cd.uncachedClient,
			Scheme:        cd.scheme,
			Recorder:      recorder,
			Namespace:     cd.namespace,
			State:         s,
			DesiredObject: desiredObj,
			CurrentObject: currentObj,
			Resource:      cr,
		}

		cd.log.V(3).Info("Invoking callbacks for", "type", t)
		if err := cb(&args); err != nil {
			cd.log.Error(err, "error invoking callback for", "type", t)
			return err
		}
	}

	return nil
}
