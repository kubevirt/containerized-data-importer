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
	"sort"
	"strings"

	"github.com/go-logr/logr"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	//FIXME: use batchv1 or v2alpha1
	v1beta1 "k8s.io/api/batch/v1beta1"
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
)

// DataImportCronReconciler members
type DataImportCronReconciler struct {
	client         client.Client
	uncachedClient client.Client
	recorder       record.EventRecorder
	scheme         *runtime.Scheme
	log            logr.Logger
}

const (
	dataImportControllerName      = "dataimportcron-controller"
	annCronJobActiveJobName       = "cronJobActiveJobName"
	annCronJobDigest              = "cronJobDigest"
	cronJobImageStreamTermMessage = "imagestream"
	labelDataImportCronName       = "dataimportcron-name"
	recentImportsToKeepPerCronJob = 3
)

// Reconcile loop for DataImportCronReconciler
func (r *DataImportCronReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithName("Reconcile")
	dataImportCron := &cdiv1.DataImportCron{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, dataImportCron); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("DataImportCron not found", "name", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	return r.reconcileDataImportCron(dataImportCron)
}

// Reconcile DataImportCron
func (r *DataImportCronReconciler) reconcileDataImportCron(dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	cronJob, err := r.getCronJob(dataImportCron)
	if err != nil {
		return reconcile.Result{}, err
	}
	digest, err := r.getDigest(dataImportCron, cronJob)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := r.updateStatus(dataImportCron); err != nil {
		return reconcile.Result{}, err
	}
	if r.shouldImport(dataImportCron, digest) {
		if err := r.createImportDataVolume(dataImportCron, cronJob); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *DataImportCronReconciler) getCronJob(dataImportCron *cdiv1.DataImportCron) (*v1beta1.CronJob, error) {
	log := r.log.WithName("getCronJob")
	// If no k8s cron job found, create a new one
	cronJob := &v1beta1.CronJob{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dataImportCron.Name}, cronJob); err != nil {
		if k8serrors.IsNotFound(err) {
			cronJob := newCronJob(dataImportCron)
			if err := r.client.Create(context.TODO(), cronJob); err != nil {
				log.Error(err, "Unable to create CronJob")
				return nil, err
			}
			return cronJob, nil
		}
		return nil, err
	}
	return cronJob, nil
}

func (r *DataImportCronReconciler) getDigest(dataImportCron *cdiv1.DataImportCron, cronJob *v1beta1.CronJob) (string, error) {
	log := r.log.WithName("getDigest")
	activeJobName := cronJob.Annotations[annCronJobActiveJobName]
	if cronJob.Status.Active != nil {
		jobName := cronJob.Status.Active[0].Name
		if jobName == "" || jobName == activeJobName {
			return "", nil
		}
		log.Info("Cron job started", "name", jobName)
		cronJob.Annotations[annCronJobActiveJobName] = jobName
		if err := r.client.Update(context.TODO(), cronJob); err != nil {
			log.Error(err, "Unable to update cron job with the currently running job", "name", jobName)
			return "", err
		}
		return "", nil
	}
	if activeJobName == "" {
		return "", nil
	}
	cronJobDigest := cronJob.Annotations[annCronJobDigest]
	if cronJobDigest != "" {
		return cronJobDigest, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"job-name": activeJobName,
		},
	})
	if err != nil {
		return "", err
	}
	podList := &corev1.PodList{}
	if err := r.client.List(context.TODO(), podList, &client.ListOptions{Namespace: dataImportCron.Namespace, LabelSelector: selector}); err != nil {
		log.Error(err, "error listing pods")
		return "", err
	}
	podCount := len(podList.Items)
	if podCount == 0 {
		log.Info("No pods found for cron job %s", activeJobName)
		return "", nil
	}
	if podCount > 1 {
		return "", errors.Errorf("%d pods found for cron job %s", podCount, activeJobName)
	}
	cronPod := podList.Items[0]
	if cronPod.Status.ContainerStatuses != nil {
		term := cronPod.Status.ContainerStatuses[0].State.Terminated
		if term != nil {
			log.Info("CronJob ended", "name", activeJobName, "term.Message", term.Message)
			digest := term.Message
			if strings.HasPrefix(term.Message, cronJobImageStreamTermMessage) {
				digest, err = r.getImageStreamDigest(dataImportCron)
				if err != nil {
					return "", err
				}
				log.Info("ImageStream", "digest", digest)
			}
			cronJob.Annotations[annCronJobDigest] = digest
			cronJob.Annotations[annCronJobActiveJobName] = ""
			if err := r.client.Update(context.TODO(), cronJob); err != nil {
				log.Error(err, "Unable to update cron job with no currently running job")
				return "", err
			}
			return digest, nil
		}
	}
	return "", nil
}

func (r *DataImportCronReconciler) getImageStreamDigest(dataImportCron *cdiv1.DataImportCron) (string, error) {
	imageStream := &imagev1.ImageStream{}
	imageStreamName := types.NamespacedName{
		Namespace: dataImportCron.Namespace,
		Name:      *dataImportCron.Spec.Source.Registry.ImageStream,
	}
	if err := r.client.Get(context.TODO(), imageStreamName, imageStream); err != nil {
		return "", err
	}
	tags := imageStream.Status.Tags
	if len(tags) == 0 || len(tags[0].Items) == 0 {
		return "", errors.Errorf("No imagestream tag items %v", imageStreamName)
	}
	return tags[0].Items[0].Image, nil
}

// Update DataSource and DataImportCron PVC on successful completion
// FIXME: update all needed status fields and conditions
func (r *DataImportCronReconciler) updateStatus(dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithName("updateStatus")
	dvName := dataImportCron.Status.CurrentImportDataVolumeName
	if dvName == "" {
		return nil
	}
	// Get the currently imported DataVolume
	dataVolume := &cdiv1.DataVolume{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dvName}, dataVolume); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("DataVolume not found", "name", dvName)
		}
		return err
	}

	now := metav1.Now()
	dataImportCron.Status.LastExecutionTimestamp = &now
	if dataVolume.Status.Phase != cdiv1.Succeeded {
		if err := r.client.Update(context.TODO(), dataImportCron); err != nil {
			log.Error(err, "Unable to update DataImportCron with last execution timestamp", "Name", dataImportCron.Name)
			return err
		}
		return nil
	}
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: dataVolume.Namespace, Name: dataSourceName}, dataSource); err != nil {
		// DataSource was not found, so create it
		if k8serrors.IsNotFound(err) {
			log.Info("Create DataSource", "name", dataSourceName)
			dataSource = newDataSource(dataImportCron)
			if err := r.client.Create(context.TODO(), dataSource); err != nil {
				log.Error(err, "Unable to create DataSource", "name", dataSourceName)
				return err
			}
		} else {
			log.Error(err, "Unable to get DataSource", "name", dataSourceName)
			return err
		}
	}

	// Update DataSource and DataImportCron status if needed
	sourcePVC := &cdiv1.DataVolumeSourcePVC{Namespace: dataVolume.Namespace, Name: dataVolume.Name}
	isDataSourceOwner := isOwner(dataSource, dataImportCron)
	if dataSource.Spec.Source.PVC == nil || *dataSource.Spec.Source.PVC != *sourcePVC || !isDataSourceOwner {
		dataSource.Spec.Source.PVC = sourcePVC
		if !isDataSourceOwner {
			dataSource.OwnerReferences = append(dataSource.OwnerReferences, metav1.OwnerReference{
				APIVersion: dataImportCron.APIVersion,
				Kind:       dataImportCron.Kind,
				Name:       dataImportCron.Name,
				UID:        dataImportCron.UID,
			})
		}
		if err := r.client.Update(context.TODO(), dataSource); err != nil {
			log.Error(err, "Unable to update DataSource with source PVC", "Name", dataSourceName, "PVC", sourcePVC)
			return err
		}
		dataImportCron.Status.LastImportedPVC = sourcePVC
		dataImportCron.Status.LastImportTimestamp = &now
	}
	// Mark no current import
	dataImportCron.Status.CurrentImportDataVolumeName = ""
	if err := r.client.Update(context.TODO(), dataImportCron); err != nil {
		log.Error(err, "Unable to update DataImportCron with last imported PVC", "Name", dataImportCron.Name, "PVC", sourcePVC)
		return err
	}
	// Garbage collection
	if err := r.garbageCollectOldImports(dataImportCron); err != nil {
		return err
	}
	return nil
}

//FIXME: do we want immediate initial import (may cause storming) or on first cron job execution?
func (r *DataImportCronReconciler) shouldImport(dataImportCron *cdiv1.DataImportCron, digest string) bool {
	log := r.log.WithName("shouldImport")
	currentDigest := dataImportCron.Status.CurrentImportDigest
	currentImportDvName := dataImportCron.Status.CurrentImportDataVolumeName
	log.Info("Checking", "currentImportDvName", currentImportDvName, "currentDigest", currentDigest, "newDigest", digest)
	return currentImportDvName == "" && digest != "" && currentDigest != digest
}

func (r *DataImportCronReconciler) createImportDataVolume(dataImportCron *cdiv1.DataImportCron, cronJob *v1beta1.CronJob) error {
	log := r.log.WithName("createImportDataVolume")
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	digest := cronJob.Annotations[annCronJobDigest]
	dvName := getDvName(dataSourceName, digest)
	log.Info("Creating new source DataVolume", "name", dvName)
	dv := newSourceDataVolume(dataImportCron, dvName)
	if err := r.client.Create(context.TODO(), dv); err != nil {
		log.Error(err, "Unable to create DataVolume", "name", dvName)
		return err
	}
	// Update references to current import
	dataImportCron.Status.CurrentImportDataVolumeName = dvName
	dataImportCron.Status.CurrentImportDigest = digest
	if err := r.client.Update(context.TODO(), dataImportCron); err != nil {
		log.Error(err, "Unable to update DataImportCron annotations", "Name", dataImportCron.Name)
		return err
	}
	// Mark no current import
	cronJob.Annotations[annCronJobDigest] = ""
	if err := r.client.Update(context.TODO(), cronJob); err != nil {
		log.Error(err, "Unable to update cronJob annotations", "Name", cronJob.Name)
		return err
	}
	return nil
}

func (r *DataImportCronReconciler) garbageCollectOldImports(dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithName("garbageCollectOldImports")
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			labelDataImportCronName: dataImportCron.Name,
		},
	})
	if err != nil {
		return err
	}
	dvList := &cdiv1.DataVolumeList{}
	if err := r.uncachedClient.List(context.TODO(), dvList, &client.ListOptions{Namespace: dataImportCron.Namespace, LabelSelector: selector}); err != nil {
		log.Error(err, "error listing dvs")
		return err
	}
	recentImports := len(dvList.Items)
	log.Info("recentImports", "count", recentImports)
	if recentImports <= recentImportsToKeepPerCronJob {
		return nil
	}
	sort.Slice(dvList.Items, func(i, j int) bool {
		return dvList.Items[i].CreationTimestamp.Time.Before(dvList.Items[j].CreationTimestamp.Time)
	})
	for _, dv := range dvList.Items {
		if recentImports <= recentImportsToKeepPerCronJob {
			break
		}
		if err := r.client.Delete(context.TODO(), &dv); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("DataVolume not found for deletion", "name", dv.Name)
				continue
			}
			log.Error(err, "Unable to delete DataVolume", "name", dv.Name)
			return err
		}
		log.Info("DataVolume deleted", "name", dv.Name)
		recentImports--
	}
	return nil
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
		recorder:       mgr.GetEventRecorderFor(dataImportControllerName),
		scheme:         mgr.GetScheme(),
		log:            log.WithName(dataImportControllerName),
	}
	dataImportCronController, err := controller.New(dataImportControllerName, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return nil, err
	}
	if err := addDataImportCronControllerWatches(mgr, dataImportCronController, log); err != nil {
		return nil, err
	}
	log.Info("Initialized DataImportCron controller")
	return dataImportCronController, nil
}

func addDataImportCronControllerWatches(mgr manager.Manager, c controller.Controller, log logr.Logger) error {
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := imagev1.AddToScheme(mgr.GetScheme()); err != nil {
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
	if err := c.Watch(&source.Kind{Type: &v1beta1.CronJob{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &cdiv1.DataImportCron{},
		IsController: true,
	}); err != nil {
		return err
	}
	return nil
}

func newCronJob(cron *cdiv1.DataImportCron) *v1beta1.CronJob {
	var container corev1.Container
	if cron.Spec.Source.Registry.URL != nil {
		container = corev1.Container{
			Name:  "skopeo",
			Image: "quay.io/skopeo/stable",
			Command: []string{"/usr/bin/sh", "-c",
				"skopeo inspect " + *cron.Spec.Source.Registry.URL +
					" | awk -F'\"' '/Digest/{print $4}' > /dev/termination-log",
			},
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
	} else if cron.Spec.Source.Registry.ImageStream != nil {
		container = corev1.Container{
			Name:  "busybox",
			Image: "busybox",
			Command: []string{"sh", "-c",
				"echo " + cronJobImageStreamTermMessage + " > /dev/termination-log",
			},
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
	}

	job := &v1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cron.Name,
			Namespace: cron.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cron, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataImportCron",
				}),
			},
			Annotations: map[string]string{
				annCronJobActiveJobName: "",
			},
		},
		Spec: v1beta1.CronJobSpec{
			Schedule:          cron.Spec.Schedule,
			ConcurrencyPolicy: v1beta1.ForbidConcurrent,
			JobTemplate: v1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers:    []corev1.Container{container},
						},
					},
				},
			},
		},
	}
	return job
}

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
			Labels: map[string]string{
				labelDataImportCronName: cron.Name,
			},
			Annotations: map[string]string{
				//AnnPodRetainAfterCompletion: "true",
			},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Registry: cron.Spec.Source.Registry,
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

func isOwner(obj, owner metav1.Object) bool {
	refs := obj.GetOwnerReferences()
	for i := range refs {
		if refs[i].UID == owner.GetUID() {
			return true
		}
	}
	return false
}

func getDvName(prefix, digest string) string {
	fromIdx := len("sha256:")
	toIdx := fromIdx + 12
	return prefix + "-" + digest[fromIdx:toIdx]
}
