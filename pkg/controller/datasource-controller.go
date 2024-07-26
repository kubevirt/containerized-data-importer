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
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
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
	ready                    = "Ready"
	noSource                 = "NoSource"
	dataSourceControllerName = "datasource-controller"
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
	if !reflect.DeepEqual(dataSource.Status.Source, dataSource.Spec.Source) {
		dataSource.Spec.Source.DeepCopyInto(&dataSource.Status.Source)
		dataSource.Status.Conditions = nil
	}
	dataSourceCopy := dataSource.DeepCopy()
	if sourcePVC := dataSource.Spec.Source.PVC; sourcePVC != nil {
		if err := r.handlePvcSource(ctx, sourcePVC, dataSource); err != nil {
			return err
		}
	} else if sourceSnapshot := dataSource.Spec.Source.Snapshot; sourceSnapshot != nil {
		if err := r.handleSnapshotSource(ctx, sourceSnapshot, dataSource); err != nil {
			return err
		}
	} else {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "No source PVC set", noSource)
	}

	if !reflect.DeepEqual(dataSource, dataSourceCopy) {
		if err := r.client.Update(ctx, dataSource); err != nil {
			return err
		}
	}
	return nil
}

func (r *DataSourceReconciler) handlePvcSource(ctx context.Context, sourcePVC *cdiv1.DataVolumeSourcePVC, dataSource *cdiv1.DataSource) error {
	dv := &cdiv1.DataVolume{}
	ns := cc.GetNamespace(sourcePVC.Namespace, dataSource.Namespace)
	isReady := false
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: sourcePVC.Name}, dv); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: sourcePVC.Name}, pvc); err != nil {
			if !k8serrors.IsNotFound(err) {
				return err
			}
			r.log.Info("PVC not found", "name", sourcePVC.Name)
			updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "PVC not found", cc.NotFound)
		} else {
			isReady = true
		}
	} else if dv.Status.Phase == cdiv1.Succeeded {
		isReady = true
	} else {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, fmt.Sprintf("Import DataVolume phase %s", dv.Status.Phase), string(dv.Status.Phase))
	}
	if isReady {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionTrue, "DataSource is ready to be consumed", ready)
	}

	return nil
}

func (r *DataSourceReconciler) handleSnapshotSource(ctx context.Context, sourceSnapshot *cdiv1.DataVolumeSourceSnapshot, dataSource *cdiv1.DataSource) error {
	snapshot := &snapshotv1.VolumeSnapshot{}
	ns := cc.GetNamespace(sourceSnapshot.Namespace, dataSource.Namespace)
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: sourceSnapshot.Name}, snapshot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		r.log.Info("Snapshot not found", "name", sourceSnapshot.Name)
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "Snapshot not found", cc.NotFound)
	} else if cc.IsSnapshotReady(snapshot) {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionTrue, "DataSource is ready to be consumed", ready)
	} else {
		updateDataSourceCondition(dataSource, cdiv1.DataSourceReady, corev1.ConditionFalse, "Snapshot phase is not ready", "SnapshotNotReady")
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
		recorder:        mgr.GetEventRecorderFor(dataSourceControllerName),
		scheme:          mgr.GetScheme(),
		log:             log.WithName(dataSourceControllerName),
		installerLabels: installerLabels,
	}
	DataSourceController, err := controller.New(dataSourceControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addDataSourceControllerWatches(mgr, DataSourceController, log); err != nil {
		return nil, err
	}
	log.Info("Initialized DataSource controller")
	return DataSourceController, nil
}

func addDataSourceControllerWatches(mgr manager.Manager, c controller.Controller, log logr.Logger) error {
	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataSource{}, &handler.TypedEnqueueRequestForObject[*cdiv1.DataSource]{},
		predicate.TypedFuncs[*cdiv1.DataSource]{
			CreateFunc: func(e event.TypedCreateEvent[*cdiv1.DataSource]) bool { return true },
			DeleteFunc: func(e event.TypedDeleteEvent[*cdiv1.DataSource]) bool { return true },
			UpdateFunc: func(e event.TypedUpdateEvent[*cdiv1.DataSource]) bool {
				return !sameSourceSpec(e.ObjectOld, e.ObjectNew)
			},
		},
	)); err != nil {
		return err
	}

	const dataSourcePvcField = "spec.source.pvc"
	const dataSourceSnapshotField = "spec.source.snapshot"

	getKey := func(namespace, name string) string {
		return namespace + "/" + name
	}

	appendMatchingDataSourceRequests := func(ctx context.Context, indexingKey string, obj client.Object, reqs []reconcile.Request) []reconcile.Request {
		var dataSources cdiv1.DataSourceList
		matchingFields := client.MatchingFields{indexingKey: getKey(obj.GetNamespace(), obj.GetName())}
		if err := mgr.GetClient().List(ctx, &dataSources, matchingFields); err != nil {
			log.Error(err, "Unable to list DataSources", "matchingFields", matchingFields)
			return reqs
		}
		for _, ds := range dataSources.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ds.Namespace, Name: ds.Name}})
		}
		return reqs
	}

	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataSource{}, dataSourcePvcField, func(obj client.Object) []string {
		if pvc := obj.(*cdiv1.DataSource).Spec.Source.PVC; pvc != nil {
			ns := cc.GetNamespace(pvc.Namespace, obj.GetNamespace())
			return []string{getKey(ns, pvc.Name)}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.DataSource{}, dataSourceSnapshotField, func(obj client.Object) []string {
		if snapshot := obj.(*cdiv1.DataSource).Spec.Source.Snapshot; snapshot != nil {
			ns := cc.GetNamespace(snapshot.Namespace, obj.GetNamespace())
			return []string{getKey(ns, snapshot.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	mapToDataSource := func(ctx context.Context, obj client.Object) []reconcile.Request {
		reqs := appendMatchingDataSourceRequests(ctx, dataSourcePvcField, obj, nil)
		return appendMatchingDataSourceRequests(ctx, dataSourceSnapshotField, obj, reqs)
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataVolume{},
		handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.DataVolume](func(ctx context.Context, obj *cdiv1.DataVolume) []reconcile.Request {
			return mapToDataSource(ctx, obj)
		}),
		predicate.TypedFuncs[*cdiv1.DataVolume]{
			CreateFunc: func(e event.TypedCreateEvent[*cdiv1.DataVolume]) bool { return true },
			DeleteFunc: func(e event.TypedDeleteEvent[*cdiv1.DataVolume]) bool { return true },
			// Only DV status phase update is interesting to reconcile
			UpdateFunc: func(e event.TypedUpdateEvent[*cdiv1.DataVolume]) bool {
				return e.ObjectOld.Status.Phase != e.ObjectNew.Status.Phase
			},
		},
	)); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{},
		handler.TypedEnqueueRequestsFromMapFunc[*corev1.PersistentVolumeClaim](func(ctx context.Context, obj *corev1.PersistentVolumeClaim) []reconcile.Request {
			return mapToDataSource(ctx, obj)
		}),
		predicate.TypedFuncs[*corev1.PersistentVolumeClaim]{
			CreateFunc: func(e event.TypedCreateEvent[*corev1.PersistentVolumeClaim]) bool { return true },
			DeleteFunc: func(e event.TypedDeleteEvent[*corev1.PersistentVolumeClaim]) bool { return true },
			UpdateFunc: func(e event.TypedUpdateEvent[*corev1.PersistentVolumeClaim]) bool {
				return e.ObjectOld.Status.Phase != e.ObjectNew.Status.Phase
			},
		},
	)); err != nil {
		return err
	}

	if err := mgr.GetClient().List(context.TODO(), &snapshotv1.VolumeSnapshotList{}); err != nil {
		if meta.IsNoMatchError(err) {
			// Back out if there's no point to attempt watch
			return nil
		}
		if !cc.IsErrCacheNotStarted(err) {
			return err
		}
	}
	if err := c.Watch(source.Kind(mgr.GetCache(), &snapshotv1.VolumeSnapshot{},
		handler.TypedEnqueueRequestsFromMapFunc[*snapshotv1.VolumeSnapshot](func(ctx context.Context, obj *snapshotv1.VolumeSnapshot) []reconcile.Request {
			return mapToDataSource(ctx, obj)
		}),
		predicate.TypedFuncs[*snapshotv1.VolumeSnapshot]{
			CreateFunc: func(e event.TypedCreateEvent[*snapshotv1.VolumeSnapshot]) bool { return true },
			DeleteFunc: func(e event.TypedDeleteEvent[*snapshotv1.VolumeSnapshot]) bool { return true },
			UpdateFunc: func(e event.TypedUpdateEvent[*snapshotv1.VolumeSnapshot]) bool {
				return !reflect.DeepEqual(e.ObjectOld.Status, e.ObjectNew.Status)
			},
		},
	)); err != nil {
		return err
	}

	return nil
}

func sameSourceSpec(objOld, objNew client.Object) bool {
	dsOld, okOld := objOld.(*cdiv1.DataSource)
	dsNew, okNew := objNew.(*cdiv1.DataSource)

	if !okOld || !okNew {
		return false
	}
	if dsOld.Spec.Source.PVC != nil {
		return reflect.DeepEqual(dsOld.Spec.Source.PVC, dsNew.Spec.Source.PVC)
	}
	if dsOld.Spec.Source.Snapshot != nil {
		return reflect.DeepEqual(dsOld.Spec.Source.Snapshot, dsNew.Spec.Source.Snapshot)
	}

	return false
}
