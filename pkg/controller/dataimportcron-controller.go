/*
Copyright 2021 The CDI Authors.

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
	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// DataImportCronReconciler members
type DataImportCronReconciler struct {
	client         client.Client
	uncachedClient client.Client
	recorder       record.EventRecorder
	scheme         *runtime.Scheme
	log            logr.Logger
	cron           *cron.Cron
	// Maps DataImportCron name to its dataImportCronJob
	importJobs map[string]*dataImportCronJob
}

const (
	recentImportsToKeepPerCronJob = 3
)

type dataImportCronJob struct {
	client         client.Client
	log            logr.Logger
	cron           *cdiv1.DataImportCron
	dataVolumeName string
	recentImports  []string
}

func (d *dataImportCronJob) Run() {
	log := d.log.WithName("dataImportCronJobRun")
	// Look for DataImportCron
	dataImportCron := &cdiv1.DataImportCron{}
	if err := d.client.Get(context.TODO(), types.NamespacedName{Name: d.cron.Name, Namespace: d.cron.Namespace}, dataImportCron); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("DataImportCron not found, hopefully will be soon deleted from cron runner", "name", d.cron.Name)
			return
		}
		log.Error(err, "Unable to get DataImportCron", "name", d.cron.Name)
		return
	}
	// Look for the cron DataSource
	// Note DataSource source PVC is updated only in Reconcile on DataVolume success
	dataSource := &cdiv1.DataSource{}
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	if err := d.client.Get(context.TODO(), types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dataSourceName}, dataSource); err != nil {
		// If DataSource not found, create it
		if k8serrors.IsNotFound(err) {
			log.Info("DataSource not found, creating one with no source PVC yet")
			dataSource = newDataSource(dataImportCron)
			if err := d.client.Create(context.TODO(), dataSource); err != nil {
				log.Error(err, "Unable to create DataSource", "name", dataSourceName)
				return
			}
		} else {
			log.Error(err, "Unable to get DataSource", "name", dataSourceName)
			return
		}
	}
	// If cron job currently has no active imports
	// FIXME: Pass DV annotation with last imported image SHA256 for conditional import, so DV is created only if source image updated
	if dataSource.Spec.Source.PVC == nil || dataSource.Spec.Source.PVC.Name == d.dataVolumeName {
		// Keep DataVolume name for status update in Reconcile
		dvName := getTimeStampedName(d.cron.Spec.ManagedDataSource)
		log.Info("Creating new source DataVolume", "name", dvName)
		dv := newSourceDataVolume(dataImportCron, dvName)
		if err := d.client.Create(context.TODO(), dv); err != nil {
			log.Error(err, "Unable to create DataVolume", "name", dvName)
			return
		}
		// Add the last import dv name to the recent imports queue
		d.recentImports = append(d.recentImports, d.dataVolumeName)
		d.garbageCollectOldImports()
		// Update references to current import
		d.dataVolumeName = dvName
	}
}

func (d *dataImportCronJob) garbageCollectOldImports() {
	log := d.log.WithName("garbageCollectOldImports")
	log.Info("recentImports", "count", len(d.recentImports))
	for len(d.recentImports) > recentImportsToKeepPerCronJob {
		oldDvName := d.recentImports[0]
		oldDv := &cdiv1.DataVolume{}
		if err := d.client.Get(context.TODO(), types.NamespacedName{Namespace: d.cron.Namespace, Name: oldDvName}, oldDv); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("DataVolume not found, deleting from recentImports", "name", oldDvName)
				d.recentImports = d.recentImports[1:]
				continue
			}
			log.Error(err, "Unable to get DataVolume", "name", oldDvName)
			return
		}
		if err := d.client.Delete(context.TODO(), oldDv); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("DataVolume not found, deleting from recentImports", "name", oldDvName)
				d.recentImports = d.recentImports[1:]
				continue
			}
			log.Error(err, "Unable to delete DataVolume", "name", oldDvName)
			return
		}
		log.Info("DataVolume deleted", "name", oldDvName)
		d.recentImports = d.recentImports[1:]
	}
}

// Reconcile loop for DataImportCronReconciler
func (r *DataImportCronReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithName("Reconcile")
	dataImportCron := &cdiv1.DataImportCron{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, dataImportCron); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("DataImportCron not found", "name", req.NamespacedName)
			if r.importJobs[req.NamespacedName.Name] != nil {
				delete(r.importJobs, req.NamespacedName.Name)
				r.restartCron()
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	return r.reconcileDataImportCron(dataImportCron)
}

// Reconcile DataImportCron
func (r *DataImportCronReconciler) reconcileDataImportCron(dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	log := r.log.WithName("reconcileDataImportCron")
	// If cron job is not already in the map, add new one to the map and the cron job runner
	importJob := r.importJobs[dataImportCron.Name]
	if importJob == nil {
		log.Info("Add CronJob", "Spec", dataImportCron.Spec)
		importJob := &dataImportCronJob{client: r.client, log: r.log, cron: dataImportCron}
		r.importJobs[dataImportCron.Name] = importJob
		r.cron.AddJob(dataImportCron.Spec.Schedule, importJob)
		return reconcile.Result{}, nil
	}
	dvName := importJob.dataVolumeName
	dataVolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dvName}, dataVolume); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("DataVolume not found", "name", dvName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Update DataSource and DataImportCron PVC on successful completion
	//FIXME: update all needed status fields and conditions
	//FIXME: delete dv on failure?
	if dataVolume.Status.Phase == cdiv1.Succeeded {
		dataSource := &cdiv1.DataSource{}
		dataSourceName := dataImportCron.Spec.ManagedDataSource
		if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: dataVolume.Namespace, Name: dataSourceName}, dataSource); err != nil {
			log.Error(err, "Unable to get DataSource", "Name", dataSourceName)
			return reconcile.Result{}, err
		}
		// Update DataSource and DataImportCron status if needed
		sourcePVC := &cdiv1.DataVolumeSourcePVC{Namespace: dataVolume.Namespace, Name: dataVolume.Name}
		if dataSource.Spec.Source.PVC == nil || *dataSource.Spec.Source.PVC != *sourcePVC {
			dataSource.Spec.Source.PVC = sourcePVC
			if err := r.client.Update(context.TODO(), dataSource); err != nil {
				log.Error(err, "Unable to update DataSource with source PVC", "Name", dataSourceName, "PVC", sourcePVC)
				return reconcile.Result{}, err
			}
			dataImportCron.Status.LastImportedPVC = sourcePVC
			if err := r.client.Update(context.TODO(), dataImportCron); err != nil {
				log.Error(err, "Unable to update DataImportCron with last imported PVC", "Name", dataVolume.Name, "PVC", sourcePVC)
				return reconcile.Result{}, err
			}
		}
	}
	return reconcile.Result{}, nil
}

func (r *DataImportCronReconciler) restartCron() {
	log := r.log.WithName("restartCron")
	log.Info("Restart cron job runner")
	r.cron.Stop()
	r.cron = cron.New()
	for cronName, importJob := range r.importJobs {
		log.Info("Add cron job", "cronName", cronName, "Spec", importJob.cron.Spec)
		r.cron.AddJob(importJob.cron.Spec.Schedule, importJob)
	}
	r.cron.Start()
}

func getTimeStampedName(prefix string) string {
	return prefix + "-" + time.Now().Format("20060102150405")
}

// NewDataImportCronController creates a new instance of the DataImportCron controller
func NewDataImportCronController(mgr manager.Manager, log logr.Logger) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	reconciler := &DataImportCronReconciler{
		client:         mgr.GetClient(),
		uncachedClient: uncachedClient,
		scheme:         mgr.GetScheme(),
		log:            log.WithName("dataimportcron-controller"),
		cron:           cron.New(),
		importJobs:     make(map[string]*dataImportCronJob),
	}
	dataImportCronController, err := controller.New("dataimportcron-controller", mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return nil, err
	}
	if err := addDataImportCronControllerWatches(mgr, dataImportCronController, log); err != nil {
		return nil, err
	}
	// Start the cron scheduler
	reconciler.cron.Start()
	log.Info("Initialized DataImportCron controller")
	return dataImportCronController, nil
}

func addDataImportCronControllerWatches(mgr manager.Manager, c controller.Controller, log logr.Logger) error {
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &cdiv1.DataImportCron{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := c.Watch(&source.Kind{Type: &cdiv1.DataVolume{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataImportCron{},
		IsController: true,
	}); err != nil {
		return err
	}
	return nil
}

// FIXME: add registry SecretRef & CertConfigMap
func newSourceDataVolume(cron *cdiv1.DataImportCron, dataVolumeName string) *cdiv1.DataVolume {
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataVolumeName,
			Namespace: cron.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cron, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataImportCron",
				}),
			},
			Annotations: map[string]string{
				//AnnPodRetainAfterCompletion: "true",
			},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: cron.Spec.Source.Registry.URL,
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("5Gi"), //FIXME
					},
				},
			},
		},
	}
	return dataVolume
}

// Create new DataSource for DataImportCron
//FIXME: Labels, Annotations?
func newDataSource(cron *cdiv1.DataImportCron) *cdiv1.DataSource {
	dataSource := &cdiv1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cron.Spec.ManagedDataSource,
			Namespace: cron.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cron, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataImportCron",
				}),
			},
		},
	}
	return dataSource
}
