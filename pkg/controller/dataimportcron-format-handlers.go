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
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

var formatHandlers = map[cdiv1.DataImportCronSourceFormat]formatHandler{
	cdiv1.DataImportCronSourceFormatSnapshot: snapshotFormatHandler{},
	cdiv1.DataImportCronSourceFormatPvc:      pvcFormatHandler{},
}

type formatHandler interface {
	populateDataSourceSpec(*cdiv1.DataSourceSpec, *cdiv1.DataVolumeSourcePVC)
	handleSourceCreation(context.Context, *DataImportCronReconciler, *cdiv1.DataImportCron, *cdiv1.DataVolume, *storagev1.StorageClass) error
	updateOnSuccess(*cdiv1.DataImportCron, client.Object)
}

type snapshotFormatHandler struct{}

func (snapshotFormatHandler) populateDataSourceSpec(dataSourceSpec *cdiv1.DataSourceSpec, sourcePVC *cdiv1.DataVolumeSourcePVC) {
	dataSourceSpec.Source = cdiv1.DataSourceSource{
		Snapshot: &cdiv1.DataVolumeSourceSnapshot{
			Namespace: sourcePVC.Namespace,
			Name:      sourcePVC.Name,
		},
	}
}

func (snapshotFormatHandler) handleSourceCreation(ctx context.Context, r *DataImportCronReconciler, dataImportCron *cdiv1.DataImportCron, dataVolume *cdiv1.DataVolume, dvStorageClass *storagev1.StorageClass) error {
	dataSourceName := dataImportCron.Spec.ManagedDataSource
	digest := dataImportCron.Annotations[AnnSourceDesiredDigest]
	if digest == "" {
		return nil
	}
	dvName, err := createDvName(dataSourceName, digest)
	if err != nil {
		return err
	}

	className, err := cc.GetSnapshotClassForSmartClone(dataVolume.Name, &dvStorageClass.Name, r.log, r.client)
	if err != nil {
		return err
	}
	labels := map[string]string{
		common.CDILabelKey:       common.CDILabelValue,
		common.CDIComponentLabel: "",
	}
	desiredSnapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dvName,
			Namespace: dataImportCron.Namespace,
			Labels:    labels,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &dvName,
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
		return nil
	}
	if cc.IsSnapshotReady(currentSnapshot) {
		// Clean up DV/PVC as they are not needed anymore
		r.log.Info("Deleting dv/pvc as snapshot is ready", "name", desiredSnapshot.Name)
		if err := r.deleteDvPvc(ctx, desiredSnapshot.Name, desiredSnapshot.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func (snapshotFormatHandler) updateOnSuccess(dataImportCron *cdiv1.DataImportCron, sourceObj client.Object) {
	snapshot := sourceObj.(*snapshotv1.VolumeSnapshot)
	if snapshot == nil {
		// Snapshot create/update will trigger reconcile
		return
	}
	if cc.IsSnapshotReady(snapshot) {
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionTrue, "Latest import is up to date", upToDate)
	} else {
		updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionFalse, "Snapshot of imported data is progressing", inProgress)
	}
}

type pvcFormatHandler struct{}

func (pvcFormatHandler) populateDataSourceSpec(dataSourceSpec *cdiv1.DataSourceSpec, sourcePVC *cdiv1.DataVolumeSourcePVC) {
	dataSourceSpec.Source = cdiv1.DataSourceSource{
		PVC: sourcePVC,
	}
}

func (pvcFormatHandler) handleSourceCreation(context.Context, *DataImportCronReconciler, *cdiv1.DataImportCron, *cdiv1.DataVolume, *storagev1.StorageClass) error {
	// No extra steps needed
	return nil
}

func (pvcFormatHandler) updateOnSuccess(dataImportCron *cdiv1.DataImportCron, _ client.Object) {
	updateDataImportCronCondition(dataImportCron, cdiv1.DataImportCronUpToDate, corev1.ConditionTrue, "Latest import is up to date", upToDate)
}
