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
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ReconcileStatePreCreate is the state before a resource is created
	ReconcileStatePreCreate ReconcileState = "PRE_CREATE"
	// ReconcileStatePostCreate is the state sfter a resource is created
	ReconcileStatePostCreate ReconcileState = "POST_CREATE"

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

	// ReconcileStatePreCDIDelete is the state before the CDI CR is deleted
	ReconcileStatePreCDIDelete ReconcileState = "PRE_CDI_DELETE"

	// ReconcileStatePostCDIDelete is the state before the CDI CR is deleted
	// not sure this will always be called
	ReconcileStatePostCDIDelete ReconcileState = "POST_CDI_DELETE"
)

// ReconcileState is the current state of the reconcile for a particuar resource
type ReconcileState string

// ReconcileCallback is the callback function
type ReconcileCallback func(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error

func addReconcileCallbacks(r *ReconcileCDI) {
	r.addCallback(&corev1.ServiceAccount{}, reconcileServiceAccounts)
}

func reconcileServiceAccounts(l logr.Logger, c client.Client, s ReconcileState, desiredObj, currentObj runtime.Object) error {
	if s == ReconcileStatePreUpdate {
		// make sure currentObj only has SCCs defined in desiredObj
	}

	switch s {
	case ReconcileStatePreCreate, ReconcileStatePreUpdate, ReconcileStatePreDelete:
		// loop through all SCCs and and add/remove SA as appropriate
	}

	return nil
}
