/*
Copyright 2023 The CDI Authors.

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

package populators

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

const (
	uploadPopulatorName = "upload-populator"

	// errUploadFailed provides a const to indicate upload has failed
	errUploadFailed = "UploadFailed"
	// uploadSucceeded provides a const to indicate upload has succeeded
	uploadSucceeded = "UploadSucceeded"

	// messageUploadFailed provides a const to form upload has failed message
	messageUploadFailed = "Upload into %s failed"
	// messageUploadSucceeded provides a const to form upload has succeeded message
	messageUploadSucceeded = "Successfully uploaded into %s"
)

// UploadPopulatorReconciler members
type UploadPopulatorReconciler struct {
	ReconcilerBase
}

// NewUploadPopulator creates a new instance of the upload populator
func NewUploadPopulator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &UploadPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(uploadPopulatorName),
			recorder:        mgr.GetEventRecorderFor(uploadPopulatorName),
			featureGates:    featuregates.NewFeatureGates(client),
			installerLabels: installerLabels,
			sourceKind:      cdiv1.VolumeUploadSourceRef,
		},
	}

	uploadPopulator, err := controller.New(uploadPopulatorName, mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := addCommonPopulatorsWatches(mgr, uploadPopulator, log, cdiv1.VolumeUploadSourceRef, &cdiv1.VolumeUploadSource{}); err != nil {
		return nil, err
	}

	return uploadPopulator, nil
}

// Reconcile the reconcile loop for the PVC with DataSourceRef of VolumeUploadSource kind
func (r *UploadPopulatorReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Upload Source PVCs")
	return r.reconcile(req, r, log)
}

// upload-specific implementation of getPopulationSource
func (r *UploadPopulatorReconciler) getPopulationSource(pvc *corev1.PersistentVolumeClaim) (client.Object, error) {
	uploadSource := &cdiv1.VolumeUploadSource{}
	uploadSourceKey := types.NamespacedName{Namespace: getPopulationSourceNamespace(pvc), Name: pvc.Spec.DataSourceRef.Name}
	if err := r.client.Get(context.TODO(), uploadSourceKey, uploadSource); err != nil {
		// reconcile will be triggered once created
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return uploadSource, nil
}

// Upload-specific implementation of updatePVCForPopulation
func (r *UploadPopulatorReconciler) updatePVCForPopulation(pvc *corev1.PersistentVolumeClaim, source client.Object) {
	pvc.Annotations[cc.AnnUploadRequest] = ""
	uploadSource := source.(*cdiv1.VolumeUploadSource)
	pvc.Annotations[cc.AnnContentType] = string(cc.GetContentType(uploadSource.Spec.ContentType))
	pvc.Annotations[cc.AnnPopulatorKind] = cdiv1.VolumeUploadSourceRef
	pvc.Annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(context.TODO(), r.client, uploadSource.Spec.Preallocation))
}

func (r *UploadPopulatorReconciler) updateUploadAnnotations(pvc *corev1.PersistentVolumeClaim, pvcPrime *corev1.PersistentVolumeClaim) {
	if _, ok := pvc.Annotations[cc.AnnPVCPrimeName]; !ok {
		return
	}
	// Delete the PVC Prime annotation once the pod is succeeded
	if pvcPrime.Annotations[cc.AnnPodPhase] == string(corev1.PodSucceeded) {
		delete(pvc.Annotations, cc.AnnPVCPrimeName)
	}
}

func (r *UploadPopulatorReconciler) reconcileTargetPVC(pvc, pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	pvcCopy := pvc.DeepCopy()
	phase := pvcPrime.Annotations[cc.AnnPodPhase]

	if phase != string(corev1.PodSucceeded) {
		updated, err := r.updatePVCPrimeNameAnnotation(pvcCopy, pvcPrime.Name)
		if updated || err != nil {
			// wait for the annotation to be updated
			return reconcile.Result{}, err
		}
	}

	// Wait upload completes
	switch phase {
	case string(corev1.PodFailed):
		// We'll get called later once it succeeds
		r.recorder.Eventf(pvc, corev1.EventTypeWarning, errUploadFailed, fmt.Sprintf(messageUploadFailed, pvc.Name))
	case string(corev1.PodSucceeded):
		if cc.IsPVCComplete(pvcPrime) && cc.IsUnbound(pvc) {
			// Once the upload is succeeded, we rebind the PV from PVC' to target PVC
			if err := cc.Rebind(context.TODO(), r.client, pvcPrime, pvcCopy); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	_, err := r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime, r.updateUploadAnnotations)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cc.IsPVCComplete(pvcPrime) {
		r.recorder.Eventf(pvc, corev1.EventTypeNormal, uploadSucceeded, fmt.Sprintf(messageUploadSucceeded, pvc.Name))
	}

	return reconcile.Result{}, nil
}
