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
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	imagev1 "github.com/openshift/api/image/v1"
	secv1 "github.com/openshift/api/security/v1"
	"github.com/pkg/errors"
	cronexpr "github.com/robfig/cron/v3"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	openapicommon "k8s.io/kube-openapi/pkg/common"
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
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
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
	digestSha256Prefix          = "sha256:"
	digestUIDPrefix             = "uid:"
	digestDvNameSuffixLength    = 12
	cronJobUIDSuffixLength      = 8
	defaultImportsToKeepPerCron = 3
)

var ErrNotManagedByCron = errors.New("DataSource is not managed by this DataImportCron")

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
	if isControllerPolledSource(dataImportCron) {
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

func (r *DataImportCronReconciler) pollSourceDigest(ctx context.Context, dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	nextTimeStr := dataImportCron.Annotations[AnnNextCronTime]
	if nextTimeStr == "" {
		return r.setNextCronTime(dataImportCron)
	}
	nextTime, err := time.Parse(time.RFC3339, nextTimeStr)
	if err != nil {
		return reconcile.Result{}, err
	}
	if nextTime.After(time.Now()) {
		return r.setNextCronTime(dataImportCron)
	}
	switch {
	case isImageStreamSource(dataImportCron):
		if err := r.updateImageStreamDesiredDigest(ctx, dataImportCron); err != nil {
			return reconcile.Result{}, err
		}
	case isPvcSource(dataImportCron):
		if err := r.updatePvcDesiredDigest(ctx, dataImportCron); err != nil {
			return reconcile.Result{}, err
		}
	case isNodePull(dataImportCron):
		if done, err := r.updateContainerImageDesiredDigest(ctx, dataImportCron); !done {
			return reconcile.Result{RequeueAfter: 3 * time.Second}, err
		} else if err != nil {
			return reconcile.Result{}, err
		}
	}

	return r.setNextCronTime(dataImportCron)
}

func (r *DataImportCronReconciler) setNextCronTime(dataImportCron *cdiv1.DataImportCron) (reconcile.Result, error) {
	now := time.Now()
	expr, err := cronexpr.ParseStandard(dataImportCron.Spec.Schedule)
	if err != nil {
		return reconcile.Result{}, err
	}
	nextTime := expr.Next(now)
	requeueAfter := nextTime.Sub(now)
	res := reconcile.Result{RequeueAfter: requeueAfter}
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

func isNodePull(cron *cdiv1.DataImportCron) bool {
	regSource, err := getCronRegistrySource(cron)
	return err == nil && regSource != nil && regSource.PullMethod != nil &&
		*regSource.PullMethod == cdiv1.RegistryPullNode
}

func getCronRegistrySource(cron *cdiv1.DataImportCron) (*cdiv1.DataVolumeSourceRegistry, error) {
	if !isCronRegistrySource(cron) {
		return nil, errors.Errorf("Cron has no registry source %s", cron.Name)
	}
	return cron.Spec.Template.Spec.Source.Registry, nil
}

func isCronRegistrySource(cron *cdiv1.DataImportCron) bool {
	source := cron.Spec.Template.Spec.Source
	return source != nil && source.Registry != nil
}

func getCronPvcSource(cron *cdiv1.DataImportCron) (*cdiv1.DataVolumeSourcePVC, error) {
	if !isPvcSource(cron) {
		return nil, errors.Errorf("Cron has no PVC source %s", cron.Name)
	}
	return cron.Spec.Template.Spec.Source.PVC, nil
}

func isPvcSource(cron *cdiv1.DataImportCron) bool {
	source := cron.Spec.Template.Spec.Source
	return source != nil && source.PVC != nil
}

func isControllerPolledSource(cron *cdiv1.DataImportCron) bool {
	return isImageStreamSource(cron) || isPvcSource(cron) || isNodePull(cron)
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
		if deleted, err := r.deleteOutdatedPendingPvc(ctx, pvc, desiredStorageClass.Name, dataImportCron.Name); deleted || err != nil {
			return res, err
		}
		currentSc, hasCurrent := dataImportCron.Annotations[AnnStorageClass]
		desiredSc := desiredStorageClass.Name
		if hasCurrent && currentSc != desiredSc {
			r.log.Info("Storage class changed, delete most recent source on the old sc as it's no longer the desired", "currentSc", currentSc, "desiredSc", desiredSc)
			if err := r.handleStorageClassChange(ctx, dataImportCron, desiredSc); err != nil {
				return res, err
			}
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}
		cc.AddAnnotation(dataImportCron, AnnStorageClass, desiredStorageClass.Name)
	}
	format, err := r.getSourceFormat(ctx, desiredStorageClass)
	if err != nil {
		return res, err
	}
	snapshot, err := r.getSnapshot(ctx, dataImportCron)
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

	switch {
	case dv != nil:
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
	case pvc != nil && pvc.Status.Phase == corev1.ClaimBound:
		if err := handlePopulatedPvc(); err != nil {
			return res, err
		}
	case snapshot != nil:
		if format == cdiv1.DataImportCronSourceFormatPvc {
			if err := r.client.Delete(ctx, snapshot); cc.IgnoreNotFound(err) != nil {
				return res, err
			}
			r.log.Info("Snapshot is around even though format switched to PVC, requeueing")
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}
		// Below k8s 1.29 there's no way to know the source volume mode
		// Let's at least expose this info on our own snapshots
		if _, ok := snapshot.Annotations[cc.AnnSourceVolumeMode]; !ok {
			volMode, err := inferVolumeModeForSnapshot(ctx, r.client, dataImportCron)
			if err != nil {
				return res, err
			}
			if volMode != nil {
				cc.AddAnnotation(snapshot, cc.AnnSourceVolumeMode, string(*volMode))
			}
		}
		// Copy labels found on dataSource to the existing snapshot in case of upgrades.
		dataSource, err := r.getDataSource(ctx, dataImportCron)
		if err != nil {
			if !k8serrors.IsNotFound(err) && !errors.Is(err, ErrNotManagedByCron) {
				return res, err
			}
		} else {
			cc.CopyAllowedLabels(dataSource.Labels, snapshot, true)
		}
		if err := r.updateSource(ctx, dataImportCron, snapshot); err != nil {
			return res, err
		}
		importSucceeded = true
	default:
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
	if isControllerPolledSource(dataImportCron) && dataImportCron.Spec.Schedule != "" {
		// We use the poll returned reconcile.Result for RequeueAfter if needed
		pollRes, err := r.pollSourceDigest(ctx, dataImportCron)
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
		if err := r.updateDataImportCronSuccessCondition(dataImportCron, format, snapshot); err != nil {
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
func (r *DataImportCronReconciler) getSnapshot(ctx context.Context, cron *cdiv1.DataImportCron) (*snapshotv1.VolumeSnapshot, error) {
	imports := cron.Status.CurrentImports
	if len(imports) == 0 {
		return nil, nil
	}

	snapName := imports[0].DataVolumeName
	snapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: cron.Namespace, Name: snapName}, snapshot); err != nil {
		if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return nil, err
		}
		return nil, nil
	}

	return snapshot, nil
}

func (r *DataImportCronReconciler) getDataSource(ctx context.Context, dataImportCron *cdiv1.DataImportCron) (*cdiv1.DataSource, error) {
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	dataSource := &cdiv1.DataSource{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: dataImportCron.Namespace, Name: dataSourceName}, dataSource); err != nil {
		return nil, err
	}
	if dataSource.Labels[common.DataImportCronLabel] != dataImportCron.Name {
		log := r.log.WithName("getCronManagedDataSource")
		log.Info("DataSource has no DataImportCron label or is not managed by cron, so it is not updated", "name", dataSourceName, "uid", dataSource.UID, "cron", dataImportCron.Name)
		return nil, ErrNotManagedByCron
	}
	return dataSource, nil
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
	if cond := dvc.FindConditionByType(cdiv1.DataVolumeRunning, dv.Status.Conditions); cond != nil {
		if cond.Status == corev1.ConditionFalse &&
			(cond.Reason == common.GenericError || cond.Reason == ImagePullFailedReason) {
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

func (r *DataImportCronReconciler) updateContainerImageDesiredDigest(ctx context.Context, cron *cdiv1.DataImportCron) (bool, error) {
	log := r.log.WithValues("name", cron.Name).WithValues("uid", cron.UID)
	podName := getPollerPodName(cron)
	ns := cron.Namespace
	nn := types.NamespacedName{Name: podName, Namespace: ns}
	pod := &corev1.Pod{}

	if err := r.client.Get(ctx, nn, pod); err == nil {
		digest, err := fetchContainerImageDigest(pod)
		if err != nil || digest == "" {
			return false, err
		}
		cc.AddAnnotation(cron, AnnLastCronTime, time.Now().Format(time.RFC3339))
		if cron.Annotations[AnnSourceDesiredDigest] != digest {
			log.Info("Updating DataImportCron", "digest", digest)
			cc.AddAnnotation(cron, AnnSourceDesiredDigest, digest)
		}
		return true, r.client.Delete(ctx, pod)
	} else if cc.IgnoreNotFound(err) != nil {
		return false, err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(ctx, r.client)
	if err != nil {
		return false, err
	}

	containerImage := strings.TrimPrefix(*cron.Spec.Template.Spec.Source.Registry.URL, "docker://")

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         cron.APIVersion,
					Kind:               cron.Kind,
					Name:               cron.Name,
					UID:                cron.UID,
					BlockOwnerDeletion: ptr.To[bool](true),
					Controller:         ptr.To[bool](true),
				},
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: ptr.To[int64](0),
			RestartPolicy:                 corev1.RestartPolicyNever,
			NodeSelector:                  workloadNodePlacement.NodeSelector,
			Tolerations:                   workloadNodePlacement.Tolerations,
			Affinity:                      workloadNodePlacement.Affinity,
			Volumes: []corev1.Volume{
				{
					Name: "shared-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:                     "init",
					Image:                    r.image,
					ImagePullPolicy:          corev1.PullPolicy(r.pullPolicy),
					Command:                  []string{"sh", "-c", "cp /usr/bin/cdi-containerimage-server /shared/server"},
					VolumeMounts:             []corev1.VolumeMount{{Name: "shared-volume", MountPath: "/shared"}},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				},
			},
			Containers: []corev1.Container{
				{
					Name:                     "image-container",
					Image:                    containerImage,
					ImagePullPolicy:          corev1.PullAlways,
					Command:                  []string{"/shared/server", "-h"},
					VolumeMounts:             []corev1.VolumeMount{{Name: "shared-volume", MountPath: "/shared"}},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				},
			},
		},
	}

	cc.SetRestrictedSecurityContext(&pod.Spec)
	if pod.Spec.SecurityContext != nil {
		pod.Spec.SecurityContext.FSGroup = nil
	}

	return false, r.client.Create(ctx, pod)
}

func fetchContainerImageDigest(pod *corev1.Pod) (string, error) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return "", nil
	}

	status := pod.Status.ContainerStatuses[0]
	if status.State.Waiting != nil {
		reason := status.State.Waiting.Reason
		switch reason {
		case "ImagePullBackOff", "ErrImagePull", "InvalidImageName":
			return "", errors.Errorf("%s %s: %s", common.ImagePullFailureText, status.Image, reason)
		}
		return "", nil
	}

	if status.State.Terminated == nil {
		return "", nil
	}

	imageID := status.ImageID
	if imageID == "" {
		return "", errors.Errorf("Container has no imageID")
	}
	idx := strings.Index(imageID, digestSha256Prefix)
	if idx < 0 {
		return "", errors.Errorf("Container image %s ID has no digest: %s", status.Image, imageID)
	}

	return imageID[idx:], nil
}

func (r *DataImportCronReconciler) updatePvcDesiredDigest(ctx context.Context, dataImportCron *cdiv1.DataImportCron) error {
	log := r.log.WithValues("name", dataImportCron.Name).WithValues("uid", dataImportCron.UID)
	pvcSource, err := getCronPvcSource(dataImportCron)
	if err != nil {
		return err
	}
	ns := pvcSource.Namespace
	if ns == "" {
		ns = dataImportCron.Namespace
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: pvcSource.Name}, pvc); err != nil {
		return err
	}
	digest := fmt.Sprintf("%s%s", digestUIDPrefix, pvc.UID)
	cc.AddAnnotation(dataImportCron, AnnLastCronTime, time.Now().Format(time.RFC3339))
	if digest != "" && dataImportCron.Annotations[AnnSourceDesiredDigest] != digest {
		log.Info("Updating DataImportCron", "digest", digest)
		cc.AddAnnotation(dataImportCron, AnnSourceDesiredDigest, digest)
	}
	return nil
}

func (r *DataImportCronReconciler) updateDataSource(ctx context.Context, dataImportCron *cdiv1.DataImportCron, format cdiv1.DataImportCronSourceFormat) error {
	log := r.log.WithName("updateDataSource")
	dataSource, err := r.getDataSource(ctx, dataImportCron)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			dataSource = r.newDataSource(dataImportCron)
			if err := r.client.Create(ctx, dataSource); err != nil {
				return err
			}
			log.Info("DataSource created", "name", dataSource.Name, "uid", dataSource.UID)
		} else if errors.Is(err, ErrNotManagedByCron) {
			return nil
		} else {
			return err
		}
	}
	dataSourceCopy := dataSource.DeepCopy()
	r.setDataImportCronResourceLabels(dataImportCron, dataSource)

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

func (r *DataImportCronReconciler) handleStorageClassChange(ctx context.Context, dataImportCron *cdiv1.DataImportCron, desiredStorageClass string) error {
	digest, ok := dataImportCron.Annotations[AnnSourceDesiredDigest]
	if !ok {
		// nothing to delete
		return nil
	}
	name, err := createDvName(dataImportCron.Spec.ManagedDataSource, digest)
	if err != nil {
		return err
	}
	om := metav1.ObjectMeta{Name: name, Namespace: dataImportCron.Namespace}
	sources := []client.Object{&snapshotv1.VolumeSnapshot{ObjectMeta: om}, &cdiv1.DataVolume{ObjectMeta: om}, &corev1.PersistentVolumeClaim{ObjectMeta: om}}
	for _, src := range sources {
		if err := r.client.Delete(ctx, src); cc.IgnoreNotFound(err) != nil {
			return err
		}
	}
	for _, src := range sources {
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(src), src); err == nil || !k8serrors.IsNotFound(err) {
			return fmt.Errorf("waiting for old sources to get cleaned up: %w", err)
		}
	}
	// Only update desired storage class once garbage collection went through
	annPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/annotations/%s","value":"%s" }]`, openapicommon.EscapeJsonPointer(AnnStorageClass), desiredStorageClass)
	err = r.client.Patch(ctx, dataImportCron, client.RawPatch(types.JSONPatchType, []byte(annPatch)))
	if err != nil {
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
	if pvc == nil {
		return nil
	}
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
	desiredSnapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc.Name,
			Namespace: dataImportCron.Namespace,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
			VolumeSnapshotClassName: &className,
		},
	}
	r.setDataImportCronResourceLabels(dataImportCron, desiredSnapshot)
	cc.CopyAllowedLabels(pvc.GetLabels(), desiredSnapshot, false)

	currentSnapshot := &snapshotv1.VolumeSnapshot{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredSnapshot), currentSnapshot); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		cc.AddAnnotation(desiredSnapshot, AnnLastUseTime, time.Now().UTC().Format(time.RFC3339Nano))
		if pvc.Spec.VolumeMode != nil {
			cc.AddAnnotation(desiredSnapshot, cc.AnnSourceVolumeMode, string(*pvc.Spec.VolumeMode))
		}
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

func (r *DataImportCronReconciler) updateDataImportCronSuccessCondition(dataImportCron *cdiv1.DataImportCron, format cdiv1.DataImportCronSourceFormat, snapshot *snapshotv1.VolumeSnapshot) error {
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
	metrics.DeleteDataImportCronOutdated(getPrometheusCronLabels(cron.Namespace, cron.Name))

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
	deleteOpts := client.DeleteOptions{PropagationPolicy: ptr.To[metav1.DeletionPropagation](metav1.DeletePropagationBackground)}
	selector, err := getSelector(map[string]string{common.DataImportCronNsLabel: cron.Namespace, common.DataImportCronLabel: cron.Name})
	if err != nil {
		return err
	}
	opts := &client.DeleteAllOfOptions{ListOptions: client.ListOptions{Namespace: r.cdiNamespace, LabelSelector: selector}, DeleteOptions: deleteOpts}
	if err := r.client.DeleteAllOf(ctx, &batchv1.CronJob{}, opts); err != nil {
		return err
	}
	if err := r.client.DeleteAllOf(ctx, &batchv1.Job{}, opts); err != nil {
		return err
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
	if err := addDataImportCronControllerWatches(mgr, dataImportCronController); err != nil {
		return nil, err
	}
	log.Info("Initialized DataImportCron controller")
	return dataImportCronController, nil
}

func getCronName(obj client.Object) string {
	return obj.GetLabels()[common.DataImportCronLabel]
}

func getCronNs(obj client.Object) string {
	return obj.GetLabels()[common.DataImportCronNsLabel]
}

func mapSourceObjectToCron[T client.Object](_ context.Context, obj T) []reconcile.Request {
	if cronName := getCronName(obj); cronName != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: cronName, Namespace: obj.GetNamespace()}}}
	}
	return nil
}

func addDataImportCronControllerWatches(mgr manager.Manager, c controller.Controller) error {
	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataImportCron{}, &handler.TypedEnqueueRequestForObject[*cdiv1.DataImportCron]{})); err != nil {
		return err
	}

	mapStorageProfileToCron := func(ctx context.Context, obj *cdiv1.StorageProfile) []reconcile.Request {
		// TODO: Get rid of this after at least one version; use indexer on storage class annotation instead
		// Otherwise we risk losing the storage profile event
		var crons cdiv1.DataImportCronList
		if err := mgr.GetClient().List(ctx, &crons); err != nil {
			c.GetLogger().Error(err, "Unable to list DataImportCrons")
			return nil
		}
		// Storage profiles are 1:1 to storage classes
		scName := obj.GetName()
		var reqs []reconcile.Request
		for _, cron := range crons.Items {
			dataVolume := cron.Spec.Template
			explicitScName := cc.GetStorageClassFromDVSpec(&dataVolume)
			templateSc, err := cc.GetStorageClassByNameWithVirtFallback(ctx, mgr.GetClient(), explicitScName, dataVolume.Spec.ContentType)
			if err != nil || templateSc == nil {
				c.GetLogger().Error(err, "Unable to get storage class", "templateSc", templateSc)
				return reqs
			}
			if templateSc.Name == scName {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cron.Namespace, Name: cron.Name}})
			}
		}
		return reqs
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataVolume{},
		handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.DataVolume](mapSourceObjectToCron),
		predicate.TypedFuncs[*cdiv1.DataVolume]{
			CreateFunc: func(event.TypedCreateEvent[*cdiv1.DataVolume]) bool { return false },
			UpdateFunc: func(e event.TypedUpdateEvent[*cdiv1.DataVolume]) bool { return getCronName(e.ObjectNew) != "" },
			DeleteFunc: func(e event.TypedDeleteEvent[*cdiv1.DataVolume]) bool { return getCronName(e.Object) != "" },
		},
	)); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.DataSource{},
		handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.DataSource](mapSourceObjectToCron),
		predicate.TypedFuncs[*cdiv1.DataSource]{
			CreateFunc: func(event.TypedCreateEvent[*cdiv1.DataSource]) bool { return false },
			UpdateFunc: func(e event.TypedUpdateEvent[*cdiv1.DataSource]) bool { return getCronName(e.ObjectNew) != "" },
			DeleteFunc: func(e event.TypedDeleteEvent[*cdiv1.DataSource]) bool { return getCronName(e.Object) != "" },
		},
	)); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{},
		handler.TypedEnqueueRequestsFromMapFunc[*corev1.PersistentVolumeClaim](mapSourceObjectToCron),
		predicate.TypedFuncs[*corev1.PersistentVolumeClaim]{
			CreateFunc: func(event.TypedCreateEvent[*corev1.PersistentVolumeClaim]) bool { return false },
			UpdateFunc: func(event.TypedUpdateEvent[*corev1.PersistentVolumeClaim]) bool { return false },
			DeleteFunc: func(e event.TypedDeleteEvent[*corev1.PersistentVolumeClaim]) bool { return getCronName(e.Object) != "" },
		},
	)); err != nil {
		return err
	}

	if err := addDefaultStorageClassUpdateWatch(mgr, c); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &cdiv1.StorageProfile{},
		handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.StorageProfile](mapStorageProfileToCron),
		predicate.TypedFuncs[*cdiv1.StorageProfile]{
			CreateFunc: func(event.TypedCreateEvent[*cdiv1.StorageProfile]) bool { return true },
			DeleteFunc: func(event.TypedDeleteEvent[*cdiv1.StorageProfile]) bool { return false },
			UpdateFunc: func(e event.TypedUpdateEvent[*cdiv1.StorageProfile]) bool {
				return e.ObjectOld.Status.DataImportCronSourceFormat != e.ObjectNew.Status.DataImportCronSourceFormat
			},
		},
	)); err != nil {
		return err
	}

	mapCronJobToCron := func(_ context.Context, obj *batchv1.CronJob) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: getCronNs(obj), Name: getCronName(obj)}}}
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &batchv1.CronJob{},
		handler.TypedEnqueueRequestsFromMapFunc[*batchv1.CronJob](mapCronJobToCron),
		predicate.TypedFuncs[*batchv1.CronJob]{
			CreateFunc: func(e event.TypedCreateEvent[*batchv1.CronJob]) bool {
				return getCronName(e.Object) != "" && getCronNs(e.Object) != ""
			},
			DeleteFunc: func(event.TypedDeleteEvent[*batchv1.CronJob]) bool { return false },
			UpdateFunc: func(event.TypedUpdateEvent[*batchv1.CronJob]) bool { return false },
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
		handler.TypedEnqueueRequestsFromMapFunc[*snapshotv1.VolumeSnapshot](mapSourceObjectToCron),
		predicate.TypedFuncs[*snapshotv1.VolumeSnapshot]{
			CreateFunc: func(event.TypedCreateEvent[*snapshotv1.VolumeSnapshot]) bool { return false },
			UpdateFunc: func(event.TypedUpdateEvent[*snapshotv1.VolumeSnapshot]) bool { return false },
			DeleteFunc: func(e event.TypedDeleteEvent[*snapshotv1.VolumeSnapshot]) bool { return getCronName(e.Object) != "" },
		},
	)); err != nil {
		return err
	}

	return nil
}

// addDefaultStorageClassUpdateWatch watches for default/virt default storage class updates
func addDefaultStorageClassUpdateWatch(mgr manager.Manager, c controller.Controller) error {
	if err := c.Watch(source.Kind(mgr.GetCache(), &storagev1.StorageClass{},
		handler.TypedEnqueueRequestsFromMapFunc[*storagev1.StorageClass](
			func(ctx context.Context, obj *storagev1.StorageClass) []reconcile.Request {
				log := c.GetLogger().WithName("DefaultStorageClassUpdateWatch")
				log.Info("Update", "sc", obj.GetName(),
					"default", obj.GetAnnotations()[cc.AnnDefaultStorageClass] == "true",
					"defaultVirt", obj.GetAnnotations()[cc.AnnDefaultVirtStorageClass] == "true")
				reqs, err := getReconcileRequestsForDicsWithoutExplicitStorageClass(ctx, mgr.GetClient())
				if err != nil {
					log.Error(err, "Failed getting DataImportCrons with pending PVCs")
				}
				return reqs
			},
		),
		predicate.TypedFuncs[*storagev1.StorageClass]{
			CreateFunc: func(event.TypedCreateEvent[*storagev1.StorageClass]) bool { return false },
			DeleteFunc: func(event.TypedDeleteEvent[*storagev1.StorageClass]) bool { return false },
			UpdateFunc: func(e event.TypedUpdateEvent[*storagev1.StorageClass]) bool {
				return (e.ObjectNew.Annotations[cc.AnnDefaultStorageClass] != e.ObjectOld.Annotations[cc.AnnDefaultStorageClass]) ||
					(e.ObjectNew.Annotations[cc.AnnDefaultVirtStorageClass] != e.ObjectOld.Annotations[cc.AnnDefaultVirtStorageClass])
			},
		},
	)); err != nil {
		return err
	}

	return nil
}

func getReconcileRequestsForDicsWithoutExplicitStorageClass(ctx context.Context, c client.Client) ([]reconcile.Request, error) {
	dicList := &cdiv1.DataImportCronList{}
	if err := c.List(ctx, dicList); err != nil {
		return nil, err
	}
	reqs := []reconcile.Request{}
	for _, dic := range dicList.Items {
		if cc.GetStorageClassFromDVSpec(&dic.Spec.Template) != nil {
			continue
		}

		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: dic.Name, Namespace: dic.Namespace}})
	}

	return reqs, nil
}

func (r *DataImportCronReconciler) deleteOutdatedPendingPvc(ctx context.Context, pvc *corev1.PersistentVolumeClaim, desiredStorageClass, cronName string) (bool, error) {
	if pvc == nil || pvc.Status.Phase != corev1.ClaimPending || pvc.Labels[common.DataImportCronLabel] != cronName {
		return false, nil
	}

	sc := pvc.Spec.StorageClassName
	if sc == nil || *sc == desiredStorageClass {
		return false, nil
	}

	r.log.Info("Delete pending pvc", "name", pvc.Name, "ns", pvc.Namespace, "sc", *sc)
	if err := r.client.Delete(ctx, pvc); cc.IgnoreNotFound(err) != nil {
		return false, err
	}

	return true, nil
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
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
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
	// No need for specifid uid/fsgroup here since this doesn't write or use qemu
	if podSpec.SecurityContext != nil {
		podSpec.SecurityContext.FSGroup = nil
	}
	if podSpec.Containers[0].SecurityContext != nil {
		podSpec.Containers[0].SecurityContext.RunAsUser = nil
	}

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
	cc.AddAnnotation(&jobSpec.Template, secv1.RequiredSCCAnnotation, common.RestrictedSCCName)

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
	labels[common.DataImportCronNsLabel] = cron.Namespace
	labels[common.DataImportCronLabel] = cron.Name
	obj.SetLabels(labels)
	return nil
}

func (r *DataImportCronReconciler) newSourceDataVolume(cron *cdiv1.DataImportCron, dataVolumeName string) *cdiv1.DataVolume {
	dv := cron.Spec.Template.DeepCopy()
	if isCronRegistrySource(cron) {
		var digestedURL string
		if isURLSource(cron) {
			digestedURL = untagDigestedDockerURL(*dv.Spec.Source.Registry.URL + "@" + cron.Annotations[AnnSourceDesiredDigest])
		} else if isImageStreamSource(cron) {
			// No way to import image stream by name when we want specific digest, so we use its docker reference
			digestedURL = "docker://" + cron.Annotations[AnnImageStreamDockerRef]
			dv.Spec.Source.Registry.ImageStream = nil
		}
		dv.Spec.Source.Registry.URL = &digestedURL
	}
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

// Create DataVolume name based on the DataSource name + prefix of the digest
func createDvName(prefix, digest string) (string, error) {
	digestPrefix := ""
	if strings.HasPrefix(digest, digestSha256Prefix) {
		digestPrefix = digestSha256Prefix
	} else if strings.HasPrefix(digest, digestUIDPrefix) {
		digestPrefix = digestUIDPrefix
	} else {
		return "", errors.Errorf("Digest has no supported prefix")
	}
	fromIdx := len(digestPrefix)
	toIdx := fromIdx + digestDvNameSuffixLength
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

func getPollerPodName(cron *cdiv1.DataImportCron) string {
	return naming.GetResourceName("poller-"+cron.Name, string(cron.UID)[:8])
}

func getSelector(matchLabels map[string]string) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: matchLabels})
}

func inferVolumeModeForSnapshot(ctx context.Context, client client.Client, cron *cdiv1.DataImportCron) (*corev1.PersistentVolumeMode, error) {
	dv := &cron.Spec.Template

	if explicitVolumeMode := getVolumeModeFromDVSpec(dv); explicitVolumeMode != nil {
		return explicitVolumeMode, nil
	}

	accessModes := getAccessModesFromDVSpec(dv)
	inferredPvc := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: cc.GetStorageClassFromDVSpec(dv),
			AccessModes:      accessModes,
			VolumeMode:       ptr.To(cdiv1.PersistentVolumeFromStorageProfile),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					// Doesn't matter
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	if err := dvc.RenderPvc(ctx, client, inferredPvc); err != nil {
		return nil, err
	}

	return inferredPvc.Spec.VolumeMode, nil
}

// getVolumeModeFromDVSpec returns the volume mode from DataVolume PVC or Storage spec
func getVolumeModeFromDVSpec(dv *cdiv1.DataVolume) *corev1.PersistentVolumeMode {
	if dv.Spec.PVC != nil {
		return dv.Spec.PVC.VolumeMode
	}

	if dv.Spec.Storage != nil {
		return dv.Spec.Storage.VolumeMode
	}

	return nil
}

// getAccessModesFromDVSpec returns the access modes from DataVolume PVC or Storage spec
func getAccessModesFromDVSpec(dv *cdiv1.DataVolume) []corev1.PersistentVolumeAccessMode {
	if dv.Spec.PVC != nil {
		return dv.Spec.PVC.AccessModes
	}

	if dv.Spec.Storage != nil {
		return dv.Spec.Storage.AccessModes
	}

	return nil
}
