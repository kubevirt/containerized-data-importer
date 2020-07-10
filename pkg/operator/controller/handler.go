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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

func enqueueCDI(c client.Client) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			var rrs []reconcile.Request
			cdiList := &cdiv1.CDIList{}

			if err := c.List(context.TODO(), cdiList, &client.ListOptions{}); err != nil {
				log.Error(err, "Error listing all CDI objects")
				return nil
			}

			for _, cdi := range cdiList.Items {
				rr := reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: cdi.Namespace, Name: cdi.Name},
				}
				rrs = append(rrs, rr)
			}

			return rrs
		}),
	}
}
