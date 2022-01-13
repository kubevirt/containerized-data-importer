/*
Copyright 2022 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
limitations under the License.
See the License for the specific language governing permissions and
*/

package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DataSourceReconciler members
type DataSourceReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	installerLabels map[string]string
}

const (
	ready = "Ready"
	noPvc = "NoPvc"
)

// Reconcile loop for DataSourceReconciler
func (r *DataSourceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(ctx, req.NamespacedName, dataSource); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := r.update(ctx, dataSource); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *DataSourceReconciler) update(ctx context.Context, dataSource *cdiv1.DataSource) error {
	dataSourceCopy := dataSource.DeepCopy()
	sourcePVC := dataSource.Spec.Source.PVC
	if sourcePVC != nil {
		dv := &cdiv1.DataVolume{}
		if err := r.client.Get(ctx, types.NamespacedName{Namespace: sourcePVC.Namespace, Name: sourcePVC.Name}, dv); err != nil {
			if k8serrors.IsNotFound(err) {
				r.log.Info("DataVolume not found", "name", sourcePVC.Name)
				updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "DataVolume not found", notFound)
			} else {
				return err
			}
		} else if dv.Status.Phase == cdiv1.Succeeded {
			updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionTrue, "DataSource is ready to be consumed", ready)
		} else {
			updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, fmt.Sprintf("Import DataVolume phase %s", dv.Status.Phase), string(dv.Status.Phase))
		}
	} else {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "No source PVC set", noPvc)
	}
	if !reflect.DeepEqual(dataSource, dataSourceCopy) {
		if err := r.client.Update(ctx, dataSource); err != nil {
			return err
		}
	}
	return nil
}

func updateDataSourceCondition(ds *cdiv1.DataSource, conditionType cdiv1.DataSourceConditionType, status corev1.ConditionStatus, message, reason string) {
	if condition := FindDataSourceConditionByType(ds, conditionType); condition != nil {
		updateConditionState(&condition.ConditionState, status, message, reason)
	} else {
		condition = &cdiv1.DataSourceCondition{Type: conditionType}
		updateConditionState(&condition.ConditionState, status, message, reason)
		ds.Status.Conditions = append(ds.Status.Conditions, *condition)
	}
}

// FindDataSourceConditionByType finds DataSourceCondition by condition type
func FindDataSourceConditionByType(ds *cdiv1.DataSource, conditionType cdiv1.DataSourceConditionType) *cdiv1.DataSourceCondition {
	for i, condition := range ds.Status.Conditions {
		if condition.Type == conditionType {
			return &ds.Status.Conditions[i]
		}
	}
	return nil
}

// NewDataSourceController creates a new instance of the DataSource controller
func NewDataSourceController(mgr manager.Manager, log logr.Logger, installerLabels map[string]string) (controller.Controller, error) {
	reconciler := &DataSourceReconciler{
		client:          mgr.GetClient(),
		recorder:        mgr.GetEventRecorderFor(dataImportControllerName),
		scheme:          mgr.GetScheme(),
		log:             log.WithName(dataImportControllerName),
		installerLabels: installerLabels,
	}
	DataSourceController, err := controller.New(dataImportControllerName, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return nil, err
	}
	if err := addDataSourceControllerWatches(mgr, DataSourceController); err != nil {
		return nil, err
	}
	log.Info("Initialized DataSource controller")
	return DataSourceController, nil
}

func addDataSourceControllerWatches(mgr manager.Manager, c controller.Controller) error {
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &cdiv1.DataSource{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return true },
			DeleteFunc: func(e event.DeleteEvent) bool { return true },
			UpdateFunc: func(e event.UpdateEvent) bool { return !sameDataSourcePvc(e.ObjectOld, e.ObjectNew) },
		},
	); err != nil {
		return err
	}

	const dataSourcePvcField = "spec.source.pvc"

	getKey := func(namespace, name string) string {
		return namespace + "." + name
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataSource{}, dataSourcePvcField, func(obj client.Object) []string {
		if pvc := obj.(*cdiv1.DataSource).Spec.Source.PVC; pvc != nil {
			return []string{getKey(pvc.Namespace, pvc.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	mapToDataSource := func(obj client.Object) (reqs []reconcile.Request) {
		var dataSources cdiv1.DataSourceList
		matchingFields := client.MatchingFields{dataSourcePvcField: getKey(obj.GetNamespace(), obj.GetName())}
		mgr.GetClient().List(context.TODO(), &dataSources, matchingFields)
		for _, ds := range dataSources.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ds.Namespace, Name: ds.Name}})
		}
		return
	}

	if err := c.Watch(&source.Kind{Type: &cdiv1.DataVolume{}},
		handler.EnqueueRequestsFromMapFunc(mapToDataSource),
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return true },
			DeleteFunc: func(e event.DeleteEvent) bool { return true },
			// Only DV status phase update is interesting to reconcile
			UpdateFunc: func(e event.UpdateEvent) bool {
				dvOld, okOld := e.ObjectOld.(*cdiv1.DataVolume)
				dvNew, okNew := e.ObjectNew.(*cdiv1.DataVolume)
				return okOld && okNew && dvOld.Status.Phase != dvNew.Status.Phase
			},
		},
	); err != nil {
		return err
	}
	return nil
}

func sameDataSourcePvc(objOld, objNew client.Object) bool {
	dsOld, okOld := objOld.(*cdiv1.DataSource)
	dsNew, okNew := objNew.(*cdiv1.DataSource)
	return okOld && okNew && reflect.DeepEqual(dsOld.Spec.Source.PVC, dsNew.Spec.Source.PVC)
}
