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
	"time"

	"github.com/go-logr/logr"
	"github.com/gorhill/cronexpr"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	v1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// DataImportCronReconciler members
type DataImportCronReconciler struct {
	client         client.Client
	uncachedClient client.Client
	recorder       record.EventRecorder
	scheme         *runtime.Scheme
	log            logr.Logger
	image          string
	pullPolicy     string
	cdiNamespace   string
}

const (
	// AnnSourceDesiredDigest is the digest of the pending updated image
	AnnSourceDesiredDigest = AnnAPIGroup + "/storage.import.sourceDesiredDigest"
	// AnnImageStreamDockerRef is the ImageStream Docker reference
	AnnImageStreamDockerRef = AnnAPIGroup + "/storage.import.imageStreamDockerRef"
	// AnnNextCronTime is the next time stamp which satisfies the cron expression
	AnnNextCronTime = AnnAPIGroup + "/storage.import.nextCronTime"

	// dataImportCronFinalizer ensures CronJob is deleted when DataImportCron is deleted, as there is no cross-namespace OwnerReference
	dataImportCronFinalizer = "cdi.kubevirt.io/dataImportCronFinalizer"

	dataImportControllerName    = "dataimportcron-controller"
	labelDataImportCronName     = "dataimportcron-name"
	digestPrefix                = "sha256:"
	digestDvNameSuffixLength    = 12
	cronJobUIDSuffixLength      = 8
	defaultImportsToKeepPerCron = 3
)

// Reconcile loop for DataImportCronReconciler
func (r *DataImportCronReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	dataImportCron := &cdiv1.DataImportCron{}
	if err := r.client.Get(ctx, req.NamespacedName, dataImportCron); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if dataImportCron.DeletionTimestamp != nil {
		err := r.cleanup(ctx, dataImportCron)
		return reconcile.Result{}, err
	}
	if err := r.initCron(ctx, dataImportCron); err != nil {
		return reconcile.Result{}, err
	}
	return r.update(ctx, dataImportCron)
}

func (r *DataImportCronReconciler) initCron(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	if dataImportCron.Annotations == nil {
		dataImportCron.Annotations = make(map[string]string)
	}
	if isURLSource(dataImportCron) && !r.cronJobExists(ctx, dataImportCron) {
		cronJob, err := r.newCronJob(dataImportCron)
		if err != nil {
			return err
		}
		AddFinalizer(dataImportCron, dataImportCronFinalizer)
		if err := r.client.Create(ctx, cronJob); err != nil {
			r.log.Error(err, "Unable to create CronJob")
			return err
		}
	}
	return nil
}

func (r *DataImportCronReconciler) getImageStream(ctx context.Context, imageStreamName, imageStreamNamespace string) (*imagev1.ImageStream, error) {
	if imageStreamName == "" || imageStreamNamespace == "" {
		return nil, errors.Errorf("Missing imagestream name or namespace")
	}
	imageStream := &imagev1.ImageStream{}
	imageStreamNamespacedName := types.NamespacedName{
		Namespace: imageStreamNamespace,
		Name:      imageStreamName,
	}
	if err := r.client.Get(ctx, imageStreamNamespacedName, imageStream); err != nil {
		return nil, err
	}
	return imageStream, nil
}

func getImageStreamDigest(imageStream *imagev1.ImageStream) (string, string, error) {
	if imageStream == nil {
		return "", "", errors.Errorf("No imagestream")
	}
	tags := imageStream.Status.Tags
	if len(tags) == 0 || len(tags[0].Items) == 0 {
		return "", "", errors.Errorf("No imagestream tag items %s", imageStream.Name)
	}
	return tags[0].Items[0].Image, tags[0].Items[0].DockerImageReference, nil
}

func (r *DataImportCronReconciler) pollImageStreamDigest(ctx context.Context, dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	if nextTimeStr := dataImportCron.Annotations[AnnNextCronTime]; nextTimeStr != "" {
		nextTime, err := time.Parse(time.RFC3339, nextTimeStr)
		if err != nil {
			return reconcile.Result{}, err
		}
		if nextTime.Before(time.Now()) {
			if err := r.updateImageStreamDesiredDigest(ctx, dataImportCron); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	return r.setNextCronTime(dataImportCron)
}

func (r *DataImportCronReconciler) setNextCronTime(dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	now := time.Now()
	expr, err := cronexpr.Parse(dataImportCron.Spec.Schedule)
	if err != nil {
		return reconcile.Result{}, err
	}
	nextTime := expr.Next(now)
	diffSec := time.Duration(nextTime.Sub(now).Seconds()) + 1
	res := reconcile.Result{Requeue: true, RequeueAfter: diffSec * time.Second}
	dataImportCron.Annotations[AnnNextCronTime] = nextTime.Format(time.RFC3339)
	r.log.Info("setNextCronTime", "nextTime", nextTime)
	return res, err
}

func isImageStreamSource(dataImportCron *cdiv1.DataImportCron) bool {
	regSource, err := getCronRegistrySource(dataImportCron)
	return err == nil && regSource.ImageStream != nil
}

func isURLSource(dataImportCron *cdiv1.DataImportCron) bool {
	regSource, err := getCronRegistrySource(dataImportCron)
	return err == nil && regSource.URL != nil
}

func getCronRegistrySource(cron *cdiv1.DataImportCron) (*cdiv1.DataVolumeSourceRegistry, error) {
	source := cron.Spec.Template.Spec.Source
	if source == nil || source.Registry == nil {
		return nil, errors.Errorf("Cron with no registry source %s", cron.Name)
	}
	return source.Registry, nil
}

// FIXME: update all conditions
func (r *DataImportCronReconciler) update(ctx context.Context, dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	log := r.log.WithName("update")
	res := reconcile.Result{}
	//FIXME: add ImportStatus DataVolumePhase so we'll handle only on transition (to Succeeded etc.)
	importSucceeded := false
	imports := dataImportCron.Status.CurrentImports
	if imports != nil {
		// Get the currently imported DataVolume
		dataVolume := &cdiv1.DataVolume{}
		if err := r.client.Get(ctx, types.NamespacedName{Namespace: dataImportCron.Namespace, Name: imports[0].DataVolumeName}, dataVolume); err != nil {
			if !k8serrors.IsNotFound(err) {
				return res, err
			}
			log.Info("DataVolume not found for some reason, so let's recreate it", "name", imports[0].DataVolumeName)
			if err := r.createImportDataVolume(ctx, dataImportCron); err != nil {
				return res, err
			}
		}

		now := metav1.Now()
		dataImportCron.Status.LastExecutionTimestamp = &now

		if dataVolume.Status.Phase == cdiv1.Succeeded {
			importSucceeded = true
			if err := r.updateDataSourceOnSuccess(ctx, dataImportCron); err != nil {
				return res, err
			}
			if err := r.updateDataImportCronOnSuccess(ctx, dataImportCron); err != nil {
				return res, err
			}
			if err := r.garbageCollectOldImports(ctx, dataImportCron); err != nil {
				return res, err
			}
		}
	}

	// We use the poller returned reconcile.Result for RequeueAfter if needed
	var err error
	if isImageStreamSource(dataImportCron) {
		res, err = r.pollImageStreamDigest(ctx, dataImportCron)
		if err != nil {
			return res, err
		}
	}

	desiredDigest := dataImportCron.Annotations[AnnSourceDesiredDigest]
	if desiredDigest != "" && (imports == nil || (importSucceeded && desiredDigest != imports[0].Digest)) {
		if err := r.createImportDataVolume(ctx, dataImportCron); err != nil {
			return res, err
		}
	}

	if err := r.client.Update(ctx, dataImportCron); err != nil {
		log.Error(err, "Unable to update DataImportCron", "Name", dataImportCron.Name)
		return res, err
	}
	return res, nil
}

func (r *DataImportCronReconciler) updateImageStreamDesiredDigest(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	regSource, err := getCronRegistrySource(dataImportCron)
	if err != nil {
		return err
	}
	if regSource.ImageStream == nil {
		return nil
	}
	imageStream, err := r.getImageStream(ctx, *regSource.ImageStream, dataImportCron.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	digest, dockerRef, err := getImageStreamDigest(imageStream)
	if err != nil {
		return err
	}
	if digest != "" && dataImportCron.Annotations[AnnSourceDesiredDigest] != digest {
		r.log.Info("updated", "digest", digest, "cron", dataImportCron.Name)
		dataImportCron.Annotations[AnnSourceDesiredDigest] = digest
		dataImportCron.Annotations[AnnImageStreamDockerRef] = dockerRef
	}
	return nil
}

func (r *DataImportCronReconciler) updateDataSourceOnSuccess(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithName("update")
	if dataImportCron.Status.CurrentImports == nil {
		return errors.Errorf("No CurrentImports in cron %s", dataImportCron.Name)
	}
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dataSourceName}, dataSource); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("Create DataSource", "name", dataSourceName)
			dataSource = newDataSource(dataImportCron)
			if err := r.client.Create(ctx, dataSource); err != nil {
				log.Error(err, "Unable to create DataSource", "name", dataSourceName)
				return err
			}
		} else {
			log.Error(err, "Unable to get DataSource", "name", dataSourceName)
			return err
		}
	}
	if !isOwner(dataSource, dataImportCron) {
		dataSource.OwnerReferences = append(dataSource.OwnerReferences, metav1.OwnerReference{
			APIVersion: dataImportCron.APIVersion,
			Kind:       dataImportCron.Kind,
			Name:       dataImportCron.Name,
			UID:        dataImportCron.UID,
		})
	}
	sourcePVC := &cdiv1.DataVolumeSourcePVC{
		Namespace: dataImportCron.Namespace,
		Name:      dataImportCron.Status.CurrentImports[0].DataVolumeName,
	}
	if dataSource.Spec.Source.PVC == nil || *dataSource.Spec.Source.PVC != *sourcePVC {
		dataSource.Spec.Source.PVC = sourcePVC
		if err := r.client.Update(ctx, dataSource); err != nil {
			log.Error(err, "Unable to update DataSource with source PVC", "Name", dataSourceName, "PVC", sourcePVC)
			return err
		}
	}
	return nil
}

func (r *DataImportCronReconciler) updateDataImportCronOnSuccess(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	if dataImportCron.Status.CurrentImports == nil {
		return errors.Errorf("No CurrentImports in cron %s", dataImportCron.Name)
	}
	sourcePVC := &cdiv1.DataVolumeSourcePVC{
		Namespace: dataImportCron.Namespace,
		Name:      dataImportCron.Status.CurrentImports[0].DataVolumeName,
	}
	if dataImportCron.Status.LastImportedPVC == nil || *dataImportCron.Status.LastImportedPVC != *sourcePVC {
		dataImportCron.Status.LastImportedPVC = sourcePVC
		now := metav1.Now()
		dataImportCron.Status.LastImportTimestamp = &now
	}
	return nil
}

func (r *DataImportCronReconciler) createImportDataVolume(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithName("createImportDataVolume")
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	digest := dataImportCron.Annotations[AnnSourceDesiredDigest]
	dvName := createDvName(dataSourceName, digest)
	log.Info("Creating new source DataVolume", "name", dvName)
	dv := newSourceDataVolume(dataImportCron, dvName)
	if err := r.client.Create(ctx, dv); err != nil {
		log.Error(err, "Unable to create DataVolume", "name", dvName)
		return err
	}
	// Update references to current import
	dataImportCron.Status.CurrentImports = []cdiv1.ImportStatus{{DataVolumeName: dvName, Digest: digest}}
	return nil
}

func (r *DataImportCronReconciler) garbageCollectOldImports(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithName("garbageCollectOldImports")
	if dataImportCron.Spec.GarbageCollect == nil || *dataImportCron.Spec.GarbageCollect != cdiv1.DataImportCronGarbageCollectOutdated {
		return nil
	}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			labelDataImportCronName: dataImportCron.Name,
		},
	})
	if err != nil {
		return err
	}
	dvList := &cdiv1.DataVolumeList{}
	if err := r.client.List(ctx, dvList, &client.ListOptions{Namespace: dataImportCron.Namespace, LabelSelector: selector}); err != nil {
		log.Error(err, "error listing dvs")
		return err
	}
	maxDvs := defaultImportsToKeepPerCron
	importsToKeep := dataImportCron.Spec.ImportsToKeep
	if importsToKeep != nil && *importsToKeep >= 0 {
		maxDvs = int(*importsToKeep)
	}
	if len(dvList.Items) <= maxDvs {
		return nil
	}
	sort.Slice(dvList.Items, func(i, j int) bool {
		return dvList.Items[i].CreationTimestamp.Time.After(dvList.Items[j].CreationTimestamp.Time)
	})
	for _, dv := range dvList.Items[maxDvs:] {
		if err := r.client.Delete(ctx, &dv); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("DataVolume not found for deletion", "name", dv.Name)
			} else {
				log.Error(err, "Unable to delete DataVolume", "name", dv.Name)
			}
		} else {
			log.Info("DataVolume deleted", "name", dv.Name)
		}
	}
	return nil
}

func (r *DataImportCronReconciler) cleanup(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	if !HasFinalizer(dataImportCron, dataImportCronFinalizer) {
		return nil
	}
	cronJob := &v1beta1.CronJob{ObjectMeta: metav1.ObjectMeta{Namespace: r.cdiNamespace, Name: GetCronJobName(dataImportCron)}}
	if err := r.client.Delete(ctx, cronJob); IgnoreNotFound(err) != nil {
		return err
	}
	RemoveFinalizer(dataImportCron, dataImportCronFinalizer)
	if err := r.client.Update(ctx, dataImportCron); err != nil {
		return err
	}
	return nil
}

// NewDataImportCronController creates a new instance of the DataImportCron controller
func NewDataImportCronController(mgr manager.Manager, log logr.Logger, importerImage, pullPolicy string) (controller.Controller, error) {
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
		image:          importerImage,
		pullPolicy:     pullPolicy,
		cdiNamespace:   util.GetNamespace(),
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
	return nil
}

func (r *DataImportCronReconciler) cronJobExists(ctx context.Context, cron *cdiv1.DataImportCron) bool {
	var cronJob v1beta1.CronJob
	cronJobNamespacedName := types.NamespacedName{Namespace: r.cdiNamespace, Name: GetCronJobName(cron)}
	return r.uncachedClient.Get(ctx, cronJobNamespacedName, &cronJob) == nil
}

func (r *DataImportCronReconciler) newCronJob(cron *cdiv1.DataImportCron) (*v1beta1.CronJob, error) {
	regSource, err := getCronRegistrySource(cron)
	if err != nil {
		return nil, err
	}
	if regSource.URL == nil {
		return nil, errors.Errorf("No URL source in cron %s", cron.Name)
	}
	cdiConfig := &cdiv1.CDIConfig{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		return nil, err
	}
	insecureTLS, err := IsInsecureTLS(*regSource.URL, cdiConfig, r.uncachedClient, r.log)
	if err != nil {
		return nil, err
	}
	container := corev1.Container{
		Name:  "cdi-source-update-poller",
		Image: r.image,
		Command: []string{
			"/usr/bin/cdi-source-update-poller",
			"-ns", cron.Namespace,
			"-cron", cron.Name,
			"-url", *regSource.URL,
		},
		ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
	}

	hasCertConfigMap := regSource.CertConfigMap != nil && *regSource.CertConfigMap != ""
	if hasCertConfigMap {
		vm := corev1.VolumeMount{
			Name:      CertVolName,
			MountPath: common.ImporterCertDir,
		}
		container.VolumeMounts = []corev1.VolumeMount{vm}
		container.Command = append(container.Command, "-certdir", common.ImporterCertDir)
	}

	if regSource.SecretRef != nil && *regSource.SecretRef != "" {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name: common.ImporterAccessKeyID,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *regSource.SecretRef,
						},
						Key: common.KeyAccess,
					},
				},
			},
			corev1.EnvVar{
				Name: common.ImporterSecretKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *regSource.SecretRef,
						},
						Key: common.KeySecret,
					},
				},
			},
		)
	}

	if insecureTLS {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  common.InsecureTLSVar,
				Value: "true",
			},
		)
	}

	var successfulJobsHistoryLimit int32 = 0
	var failedJobsHistoryLimit int32 = 0

	job := &v1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetCronJobName(cron),
			Namespace: r.cdiNamespace,
		},
		Spec: v1beta1.CronJobSpec{
			Schedule:                   cron.Spec.Schedule,
			ConcurrencyPolicy:          v1beta1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			JobTemplate: v1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyNever,
							Containers:         []corev1.Container{container},
							ServiceAccountName: "cdi-cronjob",
						},
					},
				},
			},
		},
	}

	if hasCertConfigMap {
		vol := corev1.Volume{
			Name: CertVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *regSource.CertConfigMap,
					},
				},
			},
		}
		job.Spec.JobTemplate.Spec.Template.Spec.Volumes = []corev1.Volume{vol}
	}

	return job, nil
}

func newSourceDataVolume(cron *cdiv1.DataImportCron, dataVolumeName string) *cdiv1.DataVolume {
	var digestedURL string
	dv := cron.Spec.Template.DeepCopy()
	if isURLSource(cron) {
		digestedURL = *dv.Spec.Source.Registry.URL + "@" + cron.Annotations[AnnSourceDesiredDigest]
	} else if isImageStreamSource(cron) {
		// No way to import image stream by name when we want speciific digest, so we use its docker reference
		digestedURL = "docker://" + cron.Annotations[AnnImageStreamDockerRef]
		dv.Spec.Source.Registry.ImageStream = nil
	}
	dv.Spec.Source.Registry.URL = &digestedURL
	dv.Name = dataVolumeName
	dv.Namespace = cron.Namespace
	dv.OwnerReferences = append(dv.OwnerReferences,
		*metav1.NewControllerRef(cron, schema.GroupVersionKind{
			Group:   cdiv1.SchemeGroupVersion.Group,
			Version: cdiv1.SchemeGroupVersion.Version,
			Kind:    "DataImportCron",
		}))
	if dv.Labels == nil {
		dv.Labels = make(map[string]string)
	}
	dv.Labels[labelDataImportCronName] = cron.Name
	passCronAnnotationToDv(cron, dv, AnnImmediateBinding)
	passCronAnnotationToDv(cron, dv, AnnPodRetainAfterCompletion)
	return dv
}

func passCronAnnotationToDv(cron *cdiv1.DataImportCron, dv *cdiv1.DataVolume, ann string) {
	if val := cron.Annotations[ann]; val != "" {
		if dv.Annotations == nil {
			dv.Annotations = make(map[string]string)
		}
		dv.Annotations[ann] = val
	}
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

// Create DataVolume name based on the DataSource name + prefix of the digest sha256
func createDvName(prefix, digest string) string {
	fromIdx := len(digestPrefix)
	toIdx := fromIdx + digestDvNameSuffixLength
	return prefix + "-" + digest[fromIdx:toIdx]
}

// GetCronJobName get CronJob name based on cron name and UID
func GetCronJobName(cron *cdiv1.DataImportCron) string {
	return cron.Name + "-" + string(cron.UID)[:cronJobUIDSuffixLength]
}
