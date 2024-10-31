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

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	uploadProxyServiceName = "cdi-uploadproxy"
	uploadProxyRouteName   = uploadProxyServiceName
	uploadProxyCABundle    = "cdi-uploadproxy-signer-bundle"
)

func ensureUploadProxyRouteExists(ctx context.Context, logger logr.Logger, c client.Client, scheme *runtime.Scheme, owner metav1.Object) (bool, error) {
	namespace := owner.GetNamespace()
	if namespace == "" {
		return false, fmt.Errorf("cluster scoped owner not supported")
	}

	cert, err := getUploadProxyCABundle(ctx, c)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(3).Info("ensureUploadProxyRouteExists() upload proxy ca cert doesn't exist")
			return false, nil
		}
		return false, err
	}

	cr, err := cc.GetActiveCDI(ctx, c)
	if err != nil {
		return false, err
	}
	if cr == nil {
		return false, fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	desiredRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uploadProxyRouteName,
			Namespace: namespace,
			Labels: map[string]string{
				"cdi.kubevirt.io": "",
			},
			Annotations: map[string]string{
				// long timeout here to make sure client connection doesn't die during qcow->raw conversion
				"haproxy.router.openshift.io/timeout": "60m",
			},
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: uploadProxyServiceName,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				DestinationCACertificate:      cert,
			},
		},
	}
	util.SetRecommendedLabels(desiredRoute, installerLabels, "cdi-operator")

	currentRoute := &routev1.Route{}
	key := client.ObjectKey{Namespace: namespace, Name: uploadProxyRouteName}
	err = c.Get(ctx, key, currentRoute)
	if err == nil {
		if currentRoute.Spec.To.Kind != desiredRoute.Spec.To.Kind ||
			currentRoute.Spec.To.Name != desiredRoute.Spec.To.Name ||
			currentRoute.Spec.TLS == nil ||
			currentRoute.Spec.TLS.Termination != desiredRoute.Spec.TLS.Termination ||
			currentRoute.Spec.TLS.DestinationCACertificate != desiredRoute.Spec.TLS.DestinationCACertificate {
			currentRoute.Spec = desiredRoute.Spec
			if err := c.Update(ctx, currentRoute); err != nil {
				return false, err
			}

			return true, nil
		}

		return false, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	if err = controllerutil.SetControllerReference(owner, desiredRoute, scheme); err != nil {
		return false, err
	}

	if err := c.Create(ctx, desiredRoute); err != nil {
		return false, err
	}

	return true, nil
}

func updateUserRoutes(ctx context.Context, logger logr.Logger, c client.Client, recorder record.EventRecorder) error {
	cert, err := getUploadProxyCABundle(ctx, c)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(3).Info("updateUserRoutes() upload proxy ca cert doesn't exist")
			return nil
		}
		return err
	}

	routes := &routev1.RouteList{}
	err = c.List(ctx, routes, &client.ListOptions{
		Namespace: util.GetNamespace(),
	})
	if err != nil {
		return err
	}

	for _, r := range routes.Items {
		route := r.DeepCopy()
		if route.Annotations[annInjectUploadProxyCert] != "true" {
			continue
		}

		if route.Spec.TLS == nil {
			logger.V(1).Info("Route has no TLS config, skipping", "route", route.Name)
			continue
		}

		if route.Spec.TLS.DestinationCACertificate != cert {
			logger.V(1).Info("Updating route with new CA cert", "route", route.Name)
			route.Spec.TLS.DestinationCACertificate = cert
			if err := c.Update(ctx, route); err != nil {
				return err
			}
			recorder.Event(route, corev1.EventTypeNormal, updateUserRouteSuccess, "Successfully updated Route destination CA certificate")
		}
	}

	return nil
}

func getUploadProxyCABundle(ctx context.Context, c client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: util.GetNamespace(), Name: uploadProxyCABundle}
	if err := c.Get(ctx, key, cm); err != nil {
		return "", err
	}
	return cm.Data["ca-bundle.crt"], nil
}

func (r *ReconcileCDI) watchRoutes() error {
	if !r.haveRoutes {
		log.Info("Not watching Routes")
		return nil
	}
	var route client.Object = &routev1.Route{}
	return r.controller.Watch(source.Kind(r.getCache(), route, enqueueCDI(r.client)))
}

func haveRoutes(c client.Client) (bool, error) {
	err := c.List(context.TODO(), &routev1.RouteList{}, &client.ListOptions{
		Namespace: util.GetNamespace(),
		Limit:     1,
	})
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
