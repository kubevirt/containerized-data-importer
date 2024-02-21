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
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/containers/image/v5/docker/reference"
	"github.com/go-logr/logr"
	"github.com/gorhill/cronexpr"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	cdv "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	// ErrDataSourceAlreadyManaged provides a const to indicate DataSource already managed error
	ErrDataSourceAlreadyManaged = "ErrDataSourceAlreadyManaged"
	// MessageDataSourceAlreadyManaged provides a const to form DataSource already managed error message
	MessageDataSourceAlreadyManaged = "DataSource %s is already managed by DataImportCron %s"

	prometheusNsLabel       = "ns"
	prometheusCronNameLabel = "cron_name"
)

// DataImportCronReconciler members
type DataImportCronReconciler struct {
	client          client.Client
	uncachedClient  client.Client
	recorder        record.EventRecorder
	scheme          *runtime.Scheme
	log             logr.Logger
	image           string
	pullPolicy      string
	cdiNamespace    string
	installerLabels map[string]string
}

const (
	// AnnSourceDesiredDigest is the digest of the pending updated image
	AnnSourceDesiredDigest = cc.AnnAPIGroup + "/storage.import.sourceDesiredDigest"
	// AnnImageStreamDockerRef is the ImageStream Docker reference
	AnnImageStreamDockerRef = cc.AnnAPIGroup + "/storage.import.imageStreamDockerRef"
	// AnnNextCronTime is the next time stamp which satisfies the cron expression
	AnnNextCronTime = cc.AnnAPIGroup + "/storage.import.nextCronTime"
	// AnnLastCronTime is the cron last execution time stamp
	AnnLastCronTime = cc.AnnAPIGroup + "/storage.import.lastCronTime"
	// AnnLastUseTime is the PVC last use time stamp
	AnnLastUseTime = cc.AnnAPIGroup + "/storage.import.lastUseTime"
	// AnnStorageClass is the cron DV's storage class
	AnnStorageClass = cc.AnnAPIGroup + "/storage.import.storageClass"

	dataImportControllerName    = "dataimportcron-controller"
	digestPrefix                = "sha256:"
	digestDvNameSuffixLength    = 12
	cronJobUIDSuffixLength      = 8
	defaultImportsToKeepPerCron = 3
)

// Reconcile loop for DataImportCronReconciler
func (r *DataImportCronReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	dataImportCron := &cdiv1.DataImportCron{}
	if err := r.client.Get(ctx, req.NamespacedName, dataImportCron); cc.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	} else if err != nil || dataImportCron.DeletionTimestamp != nil {
		err := r.cleanup(ctx, req.NamespacedName)
		return reconcile.Result{}, err
	}
	shouldReconcile, err := r.shouldReconcileCron(ctx, dataImportCron)
	if !shouldReconcile || err != nil {
		return reconcile.Result{}, err
	}

	if err := r.initCron(ctx, dataImportCron); err != nil {
		return reconcile.Result{}, err
	}

	return r.update(ctx, dataImportCron)
}

func (r *DataImportCronReconciler) shouldReconcileCron(ctx context.Context, cron *cdiv1.DataImportCron) (bool, error) {
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: cron.Spec.ManagedDataSource}, dataSource); err != nil {
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	dataSourceCronLabel := dataSource.Labels[common.DataImportCronLabel]
	if dataSourceCronLabel == cron.Name || dataSourceCronLabel == "" {
		return true, nil
	}
	otherCron := &cdiv1.DataImportCron{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: dataSourceCronLabel}, otherCron); err != nil {
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if otherCron.Spec.ManagedDataSource == dataSource.Name {
		msg := fmt.Sprintf(MessageDataSourceAlreadyManaged, dataSource.Name, otherCron.Name)
		r.recorder.Event(cron, corev1.EventTypeWarning, ErrDataSourceAlreadyManaged, msg)
		r.log.V(3).Info(msg)
		return false, nil
	}
	return true, nil
}

func (r *DataImportCronReconciler) initCron(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	if dataImportCron.Spec.Schedule == "" {
		return nil
	}
	if isImageStreamSource(dataImportCron) {
		if dataImportCron.Annotations[AnnNextCronTime] == "" {
			cc.AddAnnotation(dataImportCron, AnnNextCronTime, time.Now().Format(time.RFC3339))
		}
		return nil
	}
	if !isURLSource(dataImportCron) {
		return nil
	}
	exists, err := r.cronJobExistsAndUpdated(ctx, dataImportCron)
	if exists || err != nil {
		return err
	}
	cronJob, err := r.newCronJob(dataImportCron)
	if err != nil {
		return err
	}
	if err := r.client.Create(ctx, cronJob); err != nil {
		return err
	}
	job, err := r.newInitialJob(dataImportCron, cronJob)
	if err != nil {
		return err
	}
	if err := r.client.Create(ctx, job); err != nil {
		return err
	}
	return nil
}

func (r *DataImportCronReconciler) getImageStream(ctx context.Context, imageStreamName, imageStreamNamespace string) (*imagev1.ImageStream, string, error) {
	if imageStreamName == "" || imageStreamNamespace == "" {
		return nil, "", errors.Errorf("Missing ImageStream name or namespace")
	}
	imageStream := &imagev1.ImageStream{}
	name, tag, err := splitImageStreamName(imageStreamName)
	if err != nil {
		return nil, "", err
	}
	imageStreamNamespacedName := types.NamespacedName{
		Namespace: imageStreamNamespace,
		Name:      name,
	}
	if err := r.client.Get(ctx, imageStreamNamespacedName, imageStream); err != nil {
		return nil, "", err
	}
	return imageStream, tag, nil
}

func getImageStreamDigest(imageStream *imagev1.ImageStream, imageStreamTag string) (string, string, error) {
	if imageStream == nil {
		return "", "", errors.Errorf("No ImageStream")
	}
	tags := imageStream.Status.Tags
	if len(tags) == 0 {
		return "", "", errors.Errorf("ImageStream %s has no tags", imageStream.Name)
	}

	tagIdx := 0
	if len(imageStreamTag) > 0 {
		tagIdx = -1
		for i, tag := range tags {
			if tag.Tag == imageStreamTag {
				tagIdx = i
				break
			}
		}
	}
	if tagIdx == -1 {
		return "", "", errors.Errorf("ImageStream %s has no tag %s", imageStream.Name, imageStreamTag)
	}

	if len(tags[tagIdx].Items) == 0 {
		return "", "", errors.Errorf("ImageStream %s tag %s has no items", imageStream.Name, imageStreamTag)
	}
	return tags[tagIdx].Items[0].Image, tags[tagIdx].Items[0].DockerImageReference, nil
}

func splitImageStreamName(imageStreamName string) (string, string, error) {
	if subs := strings.Split(imageStreamName, ":"); len(subs) == 1 {
		return imageStreamName, "", nil
	} else if len(subs) == 2 && len(subs[0]) > 0 && len(subs[1]) > 0 {
		return subs[0], subs[1], nil
	}
	return "", "", errors.Errorf("Illegal ImageStream name %s", imageStreamName)
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
	cc.AddAnnotation(dataImportCron, AnnNextCronTime, nextTime.Format(time.RFC3339))
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

func (r *DataImportCronReconciler) update(ctx context.Context, dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	res := reconcile.Result{}

	dv, pvc, err := r.getImportState(ctx, dataImportCron)
	if err != nil {
		return res, err
	}

	dataImportCronCopy := dataImportCron.DeepCopy()
	imports := dataImportCron.Status.CurrentImports
	importSucceeded := false

	dataVolume := dataImportCron.Spec.Template
	explicitScName := cc.GetStorageClassFromDVSpec(&dataVolume)
	desiredStorageClass, err := cc.GetStorageClassByNameWithVirtFallback(ctx, r.client, explicitScName, dataVolume.Spec.ContentType)
	if err != nil {
		return res, err
	}
	if desiredStorageClass != nil {
		cc.AddAnnotation(dataImportCron, AnnStorageClass, desiredStorageClass.Name)
	}
	format, err := r.getSourceFormat(ctx, desiredStorageClass)
	if err != nil {
		return res, err
	}
	snapshot, err := r.getSnapshot(ctx, dataImportCron, format)
	if err != nil {
		return res, err
	}

	handlePopulatedPvc := func() error {
		if pvc != nil {
			if err := r.updateSource(ctx, dataImportCron, pvc); err != nil {
				return err
			}
		}
		importSucceeded = true
		if err := r.handleCronFormat(ctx, dataImportCron, pvc, format, desiredStorageClass); err != nil {
			return err
		}

		return nil
	}

	if dv != nil {
		switch dv.Status.Phase {
		case cdiv1.Succeeded:
			if err := handlePopulatedPvc(); err != nil {
				return res, err
			}
		case cdiv1.ImportScheduled:
			updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronProgressing, corev1.ConditionFalse, "Import is scheduled", scheduled)
		case cdiv1.ImportInProgress:
			updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronProgressing, corev1.ConditionTrue, "Import is progressing", inProgress)
		default:
			dvPhase := string(dv.Status.Phase)
			updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronProgressing, corev1.ConditionFalse, fmt.Sprintf("Import DataVolume phase %s", dvPhase), dvPhase)
		}
	} else if pvc != nil {
		// TODO: with plain populator PVCs (no DataVolumes) we may need to wait for corev1.Bound
		if err := handlePopulatedPvc(); err != nil {
			return res, err
		}
	} else if snapshot != nil {
		if err := r.updateSource(ctx, dataImportCron, snapshot); err != nil {
			return res, err
		}
		importSucceeded = true
	} else {
		if len(imports) > 0 {
			imports = imports[1:]
			dataImportCron.Status.CurrentImports = imports
		}
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronProgressing, corev1.ConditionFalse, "No current import", noImport)
	}

	if importSucceeded {
		if err := updateDataImportCronOnSuccess(dataImportCron); err != nil {
			return res, err
		}
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronProgressing, corev1.ConditionFalse, "No current import", noImport)
		if err := r.garbageCollectOldImports(ctx, dataImportCron); err != nil {
			return res, err
		}
	}

	if err := r.updateDataSource(ctx, dataImportCron, format); err != nil {
		return res, err
	}

	// Skip if schedule is disabled
	if isImageStreamSource(dataImportCron) && dataImportCron.Spec.Schedule != "" {
		// We use the poll returned reconcile.Result for RequeueAfter if needed
		pollRes, err := r.pollImageStreamDigest(ctx, dataImportCron)
		if err != nil {
			return pollRes, err
		}
		res = pollRes
	}

	desiredDigest := dataImportCron.Annotations[AnnSourceDesiredDigest]
	digestUpdated := desiredDigest != "" && (len(imports) == 0 || desiredDigest != imports[0].Digest)
	if digestUpdated {
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionFalse, "Source digest updated since last import", outdated)
		if dv != nil {
			if err := r.deleteErroneousDataVolume(ctx, dataImportCron, dv); err != nil {
				return res, err
			}
		}
		if importSucceeded || len(imports) == 0 {
			if err := r.createImportDataVolume(ctx, dataImportCron); err != nil {
				return res, err
			}
		}
	} else if importSucceeded {
		if err := r.updateDataImportCronSuccessCondition(ctx, dataImportCron, format, snapshot); err != nil {
			return res, err
		}
	} else if len(imports) > 0 {
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionFalse, "Import is progressing", inProgress)
	} else {
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionFalse, "No source digest", noDigest)
	}

	if err := updateLastExecutionTimestamp(dataImportCron); err != nil {
		return res, err
	}

	if !reflect.DeepEqual(dataImportCron, dataImportCronCopy) {
		if err := r.client.Update(ctx, dataImportCron); err != nil {
			return res, err
		}
	}
	return res, nil
}

// Returns the current import DV if exists, and the last imported PVC
func (r *DataImportCronReconciler) getImportState(ctx context.Context, cron *cdiv1.DataImportCron) (*cdiv1.DataVolume, *corev1.PersistentVolumeClaim, error) {
	imports := cron.Status.CurrentImports
	if len(imports) == 0 {
		return nil, nil, nil
	}

	dvName := imports[0].DataVolumeName
	dv := &cdiv1.DataVolume{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: dvName}, dv); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, err
		}
		dv = nil
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: dvName}, pvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, err
		}
		pvc = nil
	}
	return dv, pvc, nil
}

// Returns the current import DV if exists, and the last imported PVC
func (r *DataImportCronReconciler) getSnapshot(ctx context.Context, cron *cdiv1.DataImportCron, format cdiv1.DataImportCronSourceFormat) (*snapshotv1.VolumeSnapshot, error) {
	if format != cdiv1.DataImportCronSourceFormatSnapshot {
		return nil, nil
	}

	imports := cron.Status.CurrentImports
	if len(imports) == 0 {
		return nil, nil
	}

	snapName := imports[0].DataVolumeName
	snapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: snapName}, snapshot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}

	return snapshot, nil
}

func (r *DataImportCronReconciler) updateSource(ctx context.Context, cron *cdiv1.DataImportCron, obj client.Object) error {
	objCopy := obj.DeepCopyObject()
	cc.AddAnnotation(obj, AnnLastUseTime, time.Now().UTC().Format(time.RFC3339Nano))
	r.setDataImportCronResourceLabels(cron, obj)
	if !reflect.DeepEqual(obj, objCopy) {
		if err := r.client.Update(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

func (r *DataImportCronReconciler) deleteErroneousDataVolume(ctx context.Context, cron *cdiv1.DataImportCron, dv *cdiv1.DataVolume) error {
	log := r.log.WithValues("name", dv.Name).WithValues("uid", dv.UID)
	if cond := cdv.FindConditionByType(cdiv1.DataVolumeRunning, dv.Status.Conditions); cond != nil {
		if cond.Status == corev1.ConditionFalse && cond.Reason == common.GenericError {
			log.Info("Delete DataVolume and reset DesiredDigest due to error", "message", cond.Message)
			// Unlabel the DV before deleting it, to eliminate reconcile before DIC is updated
			dv.Labels[common.DataImportCronLabel] = ""
			if err := r.client.Update(ctx, dv); cc.IgnoreNotFound(err) != nil {
				return err
			}
			if err := r.client.Delete(ctx, dv); cc.IgnoreNotFound(err) != nil {
				return err
			}
			cron.Status.CurrentImports = nil
		}
	}
	return nil
}

func (r *DataImportCronReconciler) updateImageStreamDesiredDigest(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithValues("name", dataImportCron.Name).WithValues("uid", dataImportCron.UID)
	regSource, err := getCronRegistrySource(dataImportCron)
	if err != nil {
		return err
	}
	if regSource.ImageStream == nil {
		return nil
	}
	imageStream, imageStreamTag, err := r.getImageStream(ctx, *regSource.ImageStream, dataImportCron.Namespace)
	if err != nil {
		return err
	}
	digest, dockerRef, err := getImageStreamDigest(imageStream, imageStreamTag)
	if err != nil {
		return err
	}
	cc.AddAnnotation(dataImportCron, AnnLastCronTime, time.Now().Format(time.RFC3339))
	if digest != "" && dataImportCron.Annotations[AnnSourceDesiredDigest] != digest {
		log.Info("Updating DataImportCron", "digest", digest)
		cc.AddAnnotation(dataImportCron, AnnSourceDesiredDigest, digest)
		cc.AddAnnotation(dataImportCron, AnnImageStreamDockerRef, dockerRef)
	}
	return nil
}

func (r *DataImportCronReconciler) updateDataSource(ctx context.Context, dataImportCron *cdiv1.DataImportCron, format cdiv1.DataImportCronSourceFormat) error {
	log := r.log.WithName("updateDataSource")
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dataSourceName}, dataSource); err != nil {
		if k8serrors.IsNotFound(err) {
			dataSource = r.newDataSource(dataImportCron)
			if err := r.client.Create(ctx, dataSource); err != nil {
				return err
			}
			log.Info("DataSource created", "name", dataSourceName, "uid", dataSource.UID)
		} else {
			return err
		}
	}
	if dataSource.Labels[common.DataImportCronLabel] == "" {
		log.Info("DataSource has no DataImportCron label, so it is not updated", "name", dataSourceName, "uid", dataSource.UID)
		return nil
	}
	dataSourceCopy := dataSource.DeepCopy()
	r.setDataImportCronResourceLabels(dataImportCron, dataSource)

	for _, defaultInstanceTypeLabel := range cc.DefaultInstanceTypeLabels {
		passCronLabelToDataSource(dataImportCron, dataSource, defaultInstanceTypeLabel)
	}

	passCronLabelToDataSource(dataImportCron, dataSource, cc.LabelDynamicCredentialSupport)

	sourcePVC := dataImportCron.Status.LastImportedPVC
	populateDataSource(format, dataSource, sourcePVC)

	if !reflect.DeepEqual(dataSource, dataSourceCopy) {
		if err := r.client.Update(ctx, dataSource); err != nil {
			return err
		}
	}

	return nil
}

func populateDataSource(format cdiv1.DataImportCronSourceFormat, dataSource *cdiv1.DataSource, sourcePVC *cdiv1.DataVolumeSourcePVC) {
	if sourcePVC == nil {
		return
	}

	switch format {
	case cdiv1.DataImportCronSourceFormatPvc:
		dataSource.Spec.Source = cdiv1.DataSourceSource{
			PVC: sourcePVC,
		}
	case cdiv1.DataImportCronSourceFormatSnapshot:
		dataSource.Spec.Source = cdiv1.DataSourceSource{
			Snapshot: &cdiv1.DataVolumeSourceSnapshot{
				Namespace: sourcePVC.Namespace,
				Name:      sourcePVC.Name,
			},
		}
	}
}

func updateDataImportCronOnSuccess(dataImportCron *cdiv1.DataImportCron) error {
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

func updateLastExecutionTimestamp(cron *cdiv1.DataImportCron) error {
	lastTimeStr := cron.Annotations[AnnLastCronTime]
	if lastTimeStr == "" {
		return nil
	}
	lastTime, err := time.Parse(time.RFC3339, lastTimeStr)
	if err != nil {
		return err
	}
	if ts := cron.Status.LastExecutionTimestamp; ts == nil || ts.Time != lastTime {
		cron.Status.LastExecutionTimestamp = &metav1.Time{Time: lastTime}
	}
	return nil
}

func (r *DataImportCronReconciler) createImportDataVolume(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	digest := dataImportCron.Annotations[AnnSourceDesiredDigest]
	if digest == "" {
		return nil
	}
	dvName, err := createDvName(dataSourceName, digest)
	if err != nil {
		return err
	}
	dataImportCron.Status.CurrentImports = []cdiv1.ImportStatus{{DataVolumeName: dvName, Digest: digest}}

	sources := []client.Object{&snapshotv1.VolumeSnapshot{}, &corev1.PersistentVolumeClaim{}}
	for _, src := range sources {
		if err := r.client.Get(ctx, types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dvName}, src); err != nil {
			if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				return err
			}
		} else {
			if err := r.updateSource(ctx, dataImportCron, src); err != nil {
				return err
			}
			// If source exists don't create DV
			return nil
		}
	}

	dv := r.newSourceDataVolume(dataImportCron, dvName)
	if err := r.client.Create(ctx, dv); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *DataImportCronReconciler) handleCronFormat(ctx context.Context, dataImportCron *cdiv1.DataImportCron, pvc *corev1.PersistentVolumeClaim, format cdiv1.DataImportCronSourceFormat, desiredStorageClass *storagev1.StorageClass) error {
	switch format {
	case cdiv1.DataImportCronSourceFormatPvc:
		return nil
	case cdiv1.DataImportCronSourceFormatSnapshot:
		return r.handleSnapshot(ctx, dataImportCron, pvc, desiredStorageClass)
	default:
		return fmt.Errorf("unknown source format for snapshot")
	}
}

func (r *DataImportCronReconciler) handleSnapshot(ctx context.Context, dataImportCron *cdiv1.DataImportCron, pvc *corev1.PersistentVolumeClaim, desiredStorageClass *storagev1.StorageClass) error {
	if sc := pvc.Spec.StorageClassName; sc != nil && *sc != desiredStorageClass.Name {
		r.log.Info("Attempt to change storage class, will not try making a snapshot of the old PVC")
		return nil
	}
	storageProfile := &cdiv1.StorageProfile{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: desiredStorageClass.Name}, storageProfile); err != nil {
		return err
	}
	className, err := cc.GetSnapshotClassForSmartClone(pvc, &desiredStorageClass.Name, storageProfile.Status.SnapshotClass, r.log, r.client, r.recorder)
	if err != nil {
		return err
	}
	labels := map[string]string{
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: "",
	}
	desiredSnapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc.Name,
			Namespace: dataImportCron.Namespace,
			Labels:    labels,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
			VolumeSnapshotClassName: &className,
		},
	}
	r.setDataImportCronResourceLabels(dataImportCron, desiredSnapshot)

	currentSnapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredSnapshot), currentSnapshot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		cc.AddAnnotation(desiredSnapshot, AnnLastUseTime, time.Now().UTC().Format(time.RFC3339Nano))
		if err := r.client.Create(ctx, desiredSnapshot); err != nil {
			return err
		}
	} else {
		if cc.IsSnapshotReady(currentSnapshot) {
			// Clean up DV/PVC as they are not needed anymore
			r.log.Info("Deleting dv/pvc as snapshot is ready", "name", desiredSnapshot.Name)
			if err := r.deleteDvPvc(ctx, desiredSnapshot.Name, desiredSnapshot.Namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DataImportCronReconciler) updateDataImportCronSuccessCondition(ctx context.Context, dataImportCron *cdiv1.DataImportCron, format cdiv1.DataImportCronSourceFormat, snapshot *snapshotv1.VolumeSnapshot) error {
	dataImportCron.Status.SourceFormat = &format

	switch format {
	case cdiv1.DataImportCronSourceFormatPvc:
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionTrue, "Latest import is up to date", upToDate)
	case cdiv1.DataImportCronSourceFormatSnapshot:
		if snapshot == nil {
			// Snapshot create/update will trigger reconcile
			return nil
		}
		if cc.IsSnapshotReady(snapshot) {
			updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionTrue, "Latest import is up to date", upToDate)
		} else {
			updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionFalse, "Snapshot of imported data is progressing", inProgress)
		}
	default:
		return fmt.Errorf("unknown source format for snapshot")
	}

	return nil
}

func (r *DataImportCronReconciler) getSourceFormat(ctx context.Context, desiredStorageClass *storagev1.StorageClass) (cdiv1.DataImportCronSourceFormat, error) {
	format := cdiv1.DataImportCronSourceFormatPvc
	if desiredStorageClass == nil {
		return format, nil
	}

	storageProfile := &cdiv1.StorageProfile{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: desiredStorageClass.Name}, storageProfile); err != nil {
		return format, err
	}
	if storageProfile.Status.DataImportCronSourceFormat != nil {
		format = *storageProfile.Status.DataImportCronSourceFormat
	}

	return format, nil
}

func (r *DataImportCronReconciler) garbageCollectOldImports(ctx context.Context, cron *cdiv1.DataImportCron) error {
	if cron.Spec.GarbageCollect != nil && *cron.Spec.GarbageCollect != cdiv1.DataImportCronGarbageCollectOutdated {
		return nil
	}
	selector, err := getSelector(map[string]string{common.DataImportCronLabel: cron.Name})
	if err != nil {
		return err
	}

	maxImports := defaultImportsToKeepPerCron

	if cron.Spec.ImportsToKeep != nil && *cron.Spec.ImportsToKeep >= 0 {
		maxImports = int(*cron.Spec.ImportsToKeep)
	}

	if err := r.garbageCollectPVCs(ctx, cron.Namespace, cron.Name, selector, maxImports); err != nil {
		return err
	}
	if err := r.garbageCollectSnapshots(ctx, cron.Namespace, selector, maxImports); err != nil {
		return err
	}

	return nil
}

func (r *DataImportCronReconciler) garbageCollectPVCs(ctx context.Context, namespace, cronName string, selector labels.Selector, maxImports int) error {
	pvcList := &corev1.PersistentVolumeClaimList{}

	if err := r.client.List(ctx, pvcList, &client.ListOptions{Namespace: namespace, LabelSelector: selector}); err != nil {
		return err
	}
	if len(pvcList.Items) > maxImports {
		sort.Slice(pvcList.Items, func(i, j int) bool {
			return pvcList.Items[i].Annotations[AnnLastUseTime] > pvcList.Items[j].Annotations[AnnLastUseTime]
		})
		for _, pvc := range pvcList.Items[maxImports:] {
			r.log.Info("Deleting dv/pvc", "name", pvc.Name, "pvc.uid", pvc.UID)
			if err := r.deleteDvPvc(ctx, pvc.Name, pvc.Namespace); err != nil {
				return err
			}
		}
	}

	dvList := &cdiv1.DataVolumeList{}
	if err := r.client.List(ctx, dvList, &client.ListOptions{Namespace: namespace, LabelSelector: selector}); err != nil {
		return err
	}

	if len(dvList.Items) > maxImports {
		for _, dv := range dvList.Items {
			pvc := &corev1.PersistentVolumeClaim{}
			if err := r.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dv.Name}, pvc); err != nil {
				return err
			}

			if pvc.Labels[common.DataImportCronLabel] != cronName {
				r.log.Info("Deleting old version dv/pvc", "name", pvc.Name, "pvc.uid", pvc.UID)
				if err := r.deleteDvPvc(ctx, dv.Name, dv.Namespace); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// deleteDvPvc deletes DV or PVC if DV was GCed
func (r *DataImportCronReconciler) deleteDvPvc(ctx context.Context, name, namespace string) error {
	om := metav1.ObjectMeta{Name: name, Namespace: namespace}
	dv := &cdiv1.DataVolume{ObjectMeta: om}
	if err := r.client.Delete(ctx, dv); err == nil || !k8serrors.IsNotFound(err) {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: om}
	if err := r.client.Delete(ctx, pvc); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *DataImportCronReconciler) garbageCollectSnapshots(ctx context.Context, namespace string, selector labels.Selector, maxImports int) error {
	snapList := &snapshotv1.VolumeSnapshotList{}

	if err := r.client.List(ctx, snapList, &client.ListOptions{Namespace: namespace, LabelSelector: selector}); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	if len(snapList.Items) > maxImports {
		sort.Slice(snapList.Items, func(i, j int) bool {
			return snapList.Items[i].Annotations[AnnLastUseTime] > snapList.Items[j].Annotations[AnnLastUseTime]
		})
		for _, snap := range snapList.Items[maxImports:] {
			r.log.Info("Deleting snapshot", "name", snap.Name, "uid", snap.UID)
			if err := r.client.Delete(ctx, &snap); err != nil && !k8serrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (r *DataImportCronReconciler) cleanup(ctx context.Context, cron types.NamespacedName) error {
	// Don't keep alerting over a cron thats being deleted, will get set back to 1 again by reconcile loop if needed.
	metrics.DeleteDataImportCronOutdated(getPrometheusCronLabels(cron))
	if err := r.deleteJobs(ctx, cron); err != nil {
		return err
	}
	selector, err := getSelector(map[string]string{common.DataImportCronLabel: cron.Name, common.DataImportCronCleanupLabel: "true"})
	if err != nil {
		return err
	}
	opts := &client.DeleteAllOfOptions{ListOptions: client.ListOptions{Namespace: cron.Namespace, LabelSelector: selector}}
	if err := r.client.DeleteAllOf(ctx, &cdiv1.DataSource{}, opts); err != nil {
		return err
	}
	if err := r.client.DeleteAllOf(ctx, &cdiv1.DataVolume{}, opts); err != nil {
		return err
	}
	if err := r.client.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, opts); err != nil {
		return err
	}
	if err := r.client.DeleteAllOf(ctx, &snapshotv1.VolumeSnapshot{}, opts); cc.IgnoreIsNoMatchError(err) != nil {
		return err
	}
	return nil
}

func (r *DataImportCronReconciler) deleteJobs(ctx context.Context, cron types.NamespacedName) error {
	deletePropagationBackground := metav1.DeletePropagationBackground
	deleteOpts := &client.DeleteOptions{PropagationPolicy: &deletePropagationBackground}
	selector, err := getSelector(map[string]string{common.DataImportCronLabel: GetCronJobLabelValue(cron.Namespace, cron.Name)})
	if err != nil {
		return err
	}
	cronJobList := &batchv1.CronJobList{}
	if err := r.client.List(ctx, cronJobList, &client.ListOptions{Namespace: r.cdiNamespace, LabelSelector: selector}); err != nil {
		return err
	}
	for _, cronJob := range cronJobList.Items {
		if err := r.client.Delete(ctx, &cronJob, deleteOpts); cc.IgnoreNotFound(err) != nil {
			return err
		}
	}
	jobList := &batchv1.JobList{}
	if err := r.client.List(ctx, jobList, &client.ListOptions{Namespace: r.cdiNamespace, LabelSelector: selector}); err != nil {
		return err
	}
	for _, job := range jobList.Items {
		if err := r.client.Delete(ctx, &job, deleteOpts); cc.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// NewDataImportCronController creates a new instance of the DataImportCron controller
func NewDataImportCronController(mgr manager.Manager, log logr.Logger, importerImage, pullPolicy string, installerLabels map[string]string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	reconciler := &DataImportCronReconciler{
		client:          mgr.GetClient(),
		uncachedClient:  uncachedClient,
		recorder:        mgr.GetEventRecorderFor(dataImportControllerName),
		scheme:          mgr.GetScheme(),
		log:             log.WithName(dataImportControllerName),
		image:           importerImage,
		pullPolicy:      pullPolicy,
		cdiNamespace:    util.GetNamespace(),
		installerLabels: installerLabels,
	}
	dataImportCronController, err := controller.New(dataImportControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
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
	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataImportCron{}), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	getCronName := func(obj client.Object) string {
		return obj.GetLabels()[common.DataImportCronLabel]
	}
	mapSourceObjectToCron := func(_ context.Context, obj client.Object) []reconcile.Request {
		if cronName := getCronName(obj); cronName != "" {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: cronName, Namespace: obj.GetNamespace()}}}
		}
		return nil
	}

	mapStorageProfileToCron := func(ctx context.Context, obj client.Object) (reqs []reconcile.Request) {
		// TODO: Get rid of this after at least one version; use indexer on storage class annotation instead
		// Otherwise we risk losing the storage profile event
		var crons cdiv1.DataImportCronList
		if err := mgr.GetClient().List(ctx, &crons); err != nil {
			c.GetLogger().Error(err, "Unable to list DataImportCrons")
			return
		}
		// Storage profiles are 1:1 to storage classes
		scName := obj.GetName()
		for _, cron := range crons.Items {
			dataVolume := cron.Spec.Template
			explicitScName := cc.GetStorageClassFromDVSpec(&dataVolume)
			templateSc, err := cc.GetStorageClassByNameWithVirtFallback(ctx, mgr.GetClient(), explicitScName, dataVolume.Spec.ContentType)
			if err != nil || templateSc == nil {
				c.GetLogger().Error(err, "Unable to get storage class", "templateSc", templateSc)
				return
			}
			if templateSc.Name == scName {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cron.Namespace, Name: cron.Name}})
			}
		}
		return
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataVolume{}),
		handler.EnqueueRequestsFromMapFunc(mapSourceObjectToCron),
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool { return getCronName(e.ObjectNew) != "" },
			DeleteFunc: func(e event.DeleteEvent) bool { return getCronName(e.Object) != "" },
		},
	); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataSource{}),
		handler.EnqueueRequestsFromMapFunc(mapSourceObjectToCron),
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool { return getCronName(e.ObjectNew) != "" },
			DeleteFunc: func(e event.DeleteEvent) bool { return getCronName(e.Object) != "" },
		},
	); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.StorageProfile{}),
		handler.EnqueueRequestsFromMapFunc(mapStorageProfileToCron),
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return true },
			DeleteFunc: func(event.DeleteEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				profileOld, okOld := e.ObjectOld.(*cdiv1.StorageProfile)
				profileNew, okNew := e.ObjectNew.(*cdiv1.StorageProfile)
				return okOld && okNew && profileOld.Status.DataImportCronSourceFormat != profileNew.Status.DataImportCronSourceFormat
			},
		},
	); err != nil {
		return err
	}

	return nil
}

func (r *DataImportCronReconciler) cronJobExistsAndUpdated(ctx context.Context, cron *cdiv1.DataImportCron) (bool, error) {
	cronJob := &batchv1.CronJob{}
	cronJobKey := types.NamespacedName{Namespace: r.cdiNamespace, Name: GetCronJobName(cron)}
	if err := r.client.Get(ctx, cronJobKey, cronJob); err != nil {
		return false, cc.IgnoreNotFound(err)
	}

	cronJobCopy := cronJob.DeepCopy()
	if err := r.initCronJob(cron, cronJobCopy); err != nil {
		return false, err
	}

	if !reflect.DeepEqual(cronJob, cronJobCopy) {
		r.log.Info("Updating CronJob", "name", cronJob.GetName())
		if err := r.client.Update(ctx, cronJobCopy); err != nil {
			return false, cc.IgnoreNotFound(err)
		}
	}
	return true, nil
}

func (r *DataImportCronReconciler) newCronJob(cron *cdiv1.DataImportCron) (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetCronJobName(cron),
			Namespace: r.cdiNamespace,
		},
	}
	if err := r.initCronJob(cron, cronJob); err != nil {
		return nil, err
	}
	return cronJob, nil
}

// InitPollerPodSpec inits poller PodSpec
func InitPollerPodSpec(c client.Client, cron *cdiv1.DataImportCron, podSpec *corev1.PodSpec, image string, pullPolicy corev1.PullPolicy, log logr.Logger) error {
	regSource, err := getCronRegistrySource(cron)
	if err != nil {
		return err
	}
	if regSource.URL == nil {
		return errors.Errorf("No URL source in cron %s", cron.Name)
	}
	cdiConfig := &cdiv1.CDIConfig{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		return err
	}
	insecureTLS, err := IsInsecureTLS(*regSource.URL, cdiConfig, log)
	if err != nil {
		return err
	}
	container := corev1.Container{
		Name:  "cdi-source-update-poller",
		Image: image,
		Command: []string{
			"/usr/bin/cdi-source-update-poller",
			"-ns", cron.Namespace,
			"-cron", cron.Name,
			"-url", *regSource.URL,
		},
		ImagePullPolicy:          pullPolicy,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
	}

	var volumes []corev1.Volume
	hasCertConfigMap := regSource.CertConfigMap != nil && *regSource.CertConfigMap != ""
	if hasCertConfigMap {
		vm := corev1.VolumeMount{
			Name:      CertVolName,
			MountPath: common.ImporterCertDir,
		}
		container.VolumeMounts = append(container.VolumeMounts, vm)
		container.Command = append(container.Command, "-certdir", common.ImporterCertDir)
		volumes = append(volumes, createConfigMapVolume(CertVolName, *regSource.CertConfigMap))
	}

	if volName, _ := GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName); volName != "" {
		vm := corev1.VolumeMount{
			Name:      ProxyCertVolName,
			MountPath: common.ImporterProxyCertDir,
		}
		container.VolumeMounts = append(container.VolumeMounts, vm)
		volumes = append(volumes, createConfigMapVolume(ProxyCertVolName, volName))
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

	addEnvVar := func(varName, value string) {
		container.Env = append(container.Env, corev1.EnvVar{Name: varName, Value: value})
	}

	if insecureTLS {
		addEnvVar(common.InsecureTLSVar, "true")
	}

	addEnvVarFromImportProxyConfig := func(varName string) {
		if value, err := GetImportProxyConfig(cdiConfig, varName); err == nil {
			addEnvVar(varName, value)
		}
	}

	addEnvVarFromImportProxyConfig(common.ImportProxyHTTP)
	addEnvVarFromImportProxyConfig(common.ImportProxyHTTPS)
	addEnvVarFromImportProxyConfig(common.ImportProxyNoProxy)

	imagePullSecrets, err := cc.GetImagePullSecrets(c)
	if err != nil {
		return err
	}
	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(context.TODO(), c)
	if err != nil {
		return err
	}

	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.TerminationGracePeriodSeconds = ptr.To[int64](0)
	podSpec.Containers = []corev1.Container{container}
	podSpec.ServiceAccountName = common.CronJobServiceAccountName
	podSpec.Volumes = volumes
	podSpec.ImagePullSecrets = imagePullSecrets
	podSpec.NodeSelector = workloadNodePlacement.NodeSelector
	podSpec.Tolerations = workloadNodePlacement.Tolerations
	podSpec.Affinity = workloadNodePlacement.Affinity

	cc.SetRestrictedSecurityContext(podSpec)

	return nil
}

func (r *DataImportCronReconciler) initCronJob(cron *cdiv1.DataImportCron, cronJob *batchv1.CronJob) error {
	cronJobSpec := &cronJob.Spec
	cronJobSpec.Schedule = cron.Spec.Schedule
	cronJobSpec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	cronJobSpec.SuccessfulJobsHistoryLimit = ptr.To[int32](1)
	cronJobSpec.FailedJobsHistoryLimit = ptr.To[int32](1)

	jobSpec := &cronJobSpec.JobTemplate.Spec
	jobSpec.BackoffLimit = ptr.To[int32](2)
	jobSpec.TTLSecondsAfterFinished = ptr.To[int32](10)

	podSpec := &jobSpec.Template.Spec
	if err := InitPollerPodSpec(r.client, cron, podSpec, r.image, corev1.PullPolicy(r.pullPolicy), r.log); err != nil {
		return err
	}
	if err := r.setJobCommon(cron, cronJob); err != nil {
		return err
	}
	return nil
}

func (r *DataImportCronReconciler) newInitialJob(cron *cdiv1.DataImportCron, cronJob *batchv1.CronJob) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetInitialJobName(cron),
			Namespace: cronJob.Namespace,
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}
	if err := r.setJobCommon(cron, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *DataImportCronReconciler) setJobCommon(cron *cdiv1.DataImportCron, obj metav1.Object) error {
	if err := operator.SetOwnerRuntime(r.uncachedClient, obj); err != nil {
		return err
	}
	util.SetRecommendedLabels(obj, r.installerLabels, common.CDIControllerName)
	labels := obj.GetLabels()
	labels[common.DataImportCronLabel] = GetCronJobLabelValue(cron.Namespace, cron.Name)
	obj.SetLabels(labels)
	return nil
}

func (r *DataImportCronReconciler) newSourceDataVolume(cron *cdiv1.DataImportCron, dataVolumeName string) *cdiv1.DataVolume {
	var digestedURL string
	dv := cron.Spec.Template.DeepCopy()
	if isURLSource(cron) {
		digestedURL = untagDigestedDockerURL(*dv.Spec.Source.Registry.URL + "@" + cron.Annotations[AnnSourceDesiredDigest])
	} else if isImageStreamSource(cron) {
		// No way to import image stream by name when we want specific digest, so we use its docker reference
		digestedURL = "docker://" + cron.Annotations[AnnImageStreamDockerRef]
		dv.Spec.Source.Registry.ImageStream = nil
	}
	dv.Spec.Source.Registry.URL = &digestedURL
	dv.Name = dataVolumeName
	dv.Namespace = cron.Namespace
	r.setDataImportCronResourceLabels(cron, dv)
	cc.AddAnnotation(dv, cc.AnnImmediateBinding, "true")
	cc.AddAnnotation(dv, AnnLastUseTime, time.Now().UTC().Format(time.RFC3339Nano))
	passCronAnnotationToDv(cron, dv, cc.AnnPodRetainAfterCompletion)

	for _, defaultInstanceTypeLabel := range cc.DefaultInstanceTypeLabels {
		passCronLabelToDv(cron, dv, defaultInstanceTypeLabel)
	}

	passCronLabelToDv(cron, dv, cc.LabelDynamicCredentialSupport)

	return dv
}

func (r *DataImportCronReconciler) setDataImportCronResourceLabels(cron *cdiv1.DataImportCron, obj metav1.Object) {
	util.SetRecommendedLabels(obj, r.installerLabels, common.CDIControllerName)
	labels := obj.GetLabels()
	labels[common.DataImportCronLabel] = cron.Name
	if cron.Spec.RetentionPolicy != nil && *cron.Spec.RetentionPolicy == cdiv1.DataImportCronRetainNone {
		labels[common.DataImportCronCleanupLabel] = "true"
	}
	obj.SetLabels(labels)
}

func untagDigestedDockerURL(dockerURL string) string {
	if u, err := url.Parse(dockerURL); err == nil {
		url := u.Host + u.Path
		subs := reference.ReferenceRegexp.FindStringSubmatch(url)
		// Check for tag
		if len(subs) > 2 && len(subs[2]) > 0 {
			if untaggedRef, err := reference.ParseDockerRef(url); err == nil {
				return u.Scheme + "://" + untaggedRef.String()
			}
		}
	}
	return dockerURL
}

func passCronLabelToDv(cron *cdiv1.DataImportCron, dv *cdiv1.DataVolume, ann string) {
	if val := cron.Labels[ann]; val != "" {
		cc.AddLabel(dv, ann, val)
	}
}

func passCronAnnotationToDv(cron *cdiv1.DataImportCron, dv *cdiv1.DataVolume, ann string) {
	if val := cron.Annotations[ann]; val != "" {
		cc.AddAnnotation(dv, ann, val)
	}
}

func passCronLabelToDataSource(cron *cdiv1.DataImportCron, ds *cdiv1.DataSource, ann string) {
	if val := cron.Labels[ann]; val != "" {
		cc.AddLabel(ds, ann, val)
	}
}

func (r *DataImportCronReconciler) newDataSource(cron *cdiv1.DataImportCron) *cdiv1.DataSource {
	dataSource := &cdiv1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cron.Spec.ManagedDataSource,
			Namespace: cron.Namespace,
		},
	}
	util.SetRecommendedLabels(dataSource, r.installerLabels, common.CDIControllerName)
	dataSource.Labels[common.DataImportCronLabel] = cron.Name
	return dataSource
}

// Create DataVolume name based on the DataSource name + prefix of the digest sha256
func createDvName(prefix, digest string) (string, error) {
	fromIdx := len(digestPrefix)
	toIdx := fromIdx + digestDvNameSuffixLength
	if !strings.HasPrefix(digest, digestPrefix) {
		return "", errors.Errorf("Digest has no supported prefix")
	}
	if len(digest) < toIdx {
		return "", errors.Errorf("Digest is too short")
	}
	return naming.GetResourceName(prefix, digest[fromIdx:toIdx]), nil
}

// GetCronJobName get CronJob name based on cron name and UID
func GetCronJobName(cron *cdiv1.DataImportCron) string {
	return naming.GetResourceName(cron.Name, string(cron.UID)[:cronJobUIDSuffixLength])
}

// GetInitialJobName get initial job name based on cron name and UID
func GetInitialJobName(cron *cdiv1.DataImportCron) string {
	return naming.GetResourceName("initial-job", GetCronJobName(cron))
}

func getSelector(matchLabels map[string]string) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: matchLabels})
}

// GetCronJobLabelValue gets CronJob label based on the cron name and namespace
func GetCronJobLabelValue(cronNamespace, cronName string) string {
	const maxLen = validation.DNS1035LabelMaxLength
	label := cronNamespace + "." + cronName
	if len(label) > maxLen {
		return label[:maxLen]
	}
	return label
}
