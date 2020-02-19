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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// delete when we no longer support <= 1.12.0
func reconcileDeleteSecrets(args *ReconcileCallbackArgs) error {
	if args.State != ReconcileStatePostRead {
		return nil
	}

	deployment := args.CurrentObject.(*appsv1.Deployment)
	if !isControllerDeployment(deployment) {
		return nil
	}

	for _, s := range []string{"cdi-api-server-cert",
		"cdi-upload-proxy-ca-key",
		"cdi-upload-proxy-server-key",
		"cdi-upload-server-ca-key",
		"cdi-upload-server-client-ca-key",
		"cdi-upload-server-client-key",
	} {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: args.Namespace, Name: s}
		err := args.Client.Get(context.TODO(), key, secret)
		if errors.IsNotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		err = args.Client.Delete(context.TODO(), secret)
		if err != nil {
			return err
		}
	}

	return nil
}
