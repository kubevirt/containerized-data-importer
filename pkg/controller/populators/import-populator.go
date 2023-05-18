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
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	importPopulatorName = "import-populator"

	// importFailed provides a const to indicate import has failed
	importFailed = "importFailed"
	// importSucceeded provides a const to indicate import has succeeded
	importSucceeded = "importSucceeded"

	// messageImportFailed provides a const to form import has failed message
	messageImportFailed = "import into %s failed"
	// messageImportSucceeded provides a const to form import has succeeded message
	messageImportSucceeded = "Successfully imported into %s"
)

// ImportPopulatorReconciler members
type ImportPopulatorReconciler struct {
	ReconcilerBase
}

// http client to get metrics
var httpClient *http.Client

// NewImportPopulator creates a new instance of the import-populator controller
func NewImportPopulator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &ImportPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(importPopulatorName),
			recorder:        mgr.GetEventRecorderFor(importPopulatorName),
			featureGates:    featuregates.NewFeatureGates(client),
			sourceKind:      cdiv1.VolumeImportSourceRef,
			installerLabels: installerLabels,
		},
	}

	importPopulator, err := controller.New(importPopulatorName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := addCommonPopulatorsWatches(mgr, importPopulator, log, cdiv1.VolumeImportSourceRef, &cdiv1.VolumeImportSource{}); err != nil {
		return nil, err
	}

	return importPopulator, nil
}

// Reconcile the reconcile loop for the PVC with DataSourceRef of VolumeImportSource kind
func (r *ImportPopulatorReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Import Source PVCs")
	return r.reconcile(req, r, log)
}

// Implementations of populatorController methods

// Import-specific implementation of getPopulationSource
func (r *ImportPopulatorReconciler) getPopulationSource(namespace, name string) (client.Object, error) {
	volumeImportSource := &cdiv1.VolumeImportSource{}
	volumeImportSourceKey := types.NamespacedName{Namespace: namespace, Name: name}
	if err := r.client.Get(context.TODO(), volumeImportSourceKey, volumeImportSource); err != nil {
		// reconcile will be triggered once created
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return volumeImportSource, nil
}

// Import-specific implementation of reconcileTargetPVC
func (r *ImportPopulatorReconciler) reconcileTargetPVC(pvc, pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	pvcCopy := pvc.DeepCopy()
	phase := pvcPrime.Annotations[cc.AnnPodPhase]
	// TODO: do we want to prevent the other updates in case of failure
	// tp update the progress?
	if err := r.updateImportProgress(phase, pvcCopy, pvcPrime); err != nil {
		return reconcile.Result{}, err
	}
	updateImportSourceAnnotation(pvcCopy, pvcPrime)
	updateVddkAnnotations(pvcCopy, pvcPrime)

	switch phase {
	case string(corev1.PodRunning):
		err := r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime)
		// We requeue to keep reporting progress
		return reconcile.Result{RequeueAfter: 2 * time.Second}, err
	case string(corev1.PodFailed):
		// We'll get called later once it succeeds
		r.recorder.Eventf(pvc, corev1.EventTypeWarning, importFailed, messageImportFailed, pvc.Name)
	case string(corev1.PodSucceeded):
		// Once the import is succeeded, we rebind the PV from PVC' to target PVC
		if err := cc.Rebind(context.TODO(), r.client, pvcPrime, pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	err := r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cc.IsPVCComplete(pvcPrime) {
		r.recorder.Eventf(pvc, corev1.EventTypeNormal, importSucceeded, messageImportSucceeded, pvc.Name)
	}

	return reconcile.Result{}, nil
}

// Import-specific implementation of updatePVCForPopulation
func (r *ImportPopulatorReconciler) updatePVCForPopulation(pvc *corev1.PersistentVolumeClaim, source client.Object) {
	volumeImportSource := source.(*cdiv1.VolumeImportSource)
	annotations := pvc.Annotations
	annotations[cc.AnnPopulatorKind] = cdiv1.VolumeImportSourceRef
	annotations[cc.AnnContentType] = cc.GetContentType(string(volumeImportSource.Spec.ContentType))
	annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(context.TODO(), r.client, volumeImportSource.Spec.Preallocation))

	if http := volumeImportSource.Spec.Source.HTTP; http != nil {
		cc.UpdateHTTPAnnotations(annotations, http)
		return
	}
	if s3 := volumeImportSource.Spec.Source.S3; s3 != nil {
		cc.UpdateS3Annotations(annotations, s3)
		return
	}
	if gcs := volumeImportSource.Spec.Source.GCS; gcs != nil {
		cc.UpdateGCSAnnotations(annotations, gcs)
		return
	}
	if registry := volumeImportSource.Spec.Source.Registry; registry != nil {
		cc.UpdateRegistryAnnotations(annotations, registry)
		return
	}
	if imageio := volumeImportSource.Spec.Source.Imageio; imageio != nil {
		cc.UpdateImageIOAnnotations(annotations, imageio)
		return
	}
	if vddk := volumeImportSource.Spec.Source.VDDK; vddk != nil {
		cc.UpdateVDDKAnnotations(annotations, vddk)
		return
	}
	// Our webhook doesn't allow VolumeImportSources without source, so this should never happen.
	// Defaulting to Blank source anyway to avoid unexpected behavior.
	annotations[cc.AnnSource] = cc.SourceNone
}

func updateVddkAnnotations(pvc, pvcPrime *corev1.PersistentVolumeClaim) {
	if cc.GetSource(pvcPrime) != cc.SourceVDDK {
		return
	}
	if vddkHost := pvcPrime.Annotations[cc.AnnVddkHostConnection]; vddkHost != "" {
		cc.AddAnnotation(pvc, cc.AnnVddkHostConnection, vddkHost)
	}
	if vddkVersion := pvcPrime.Annotations[cc.AnnVddkVersion]; vddkVersion != "" {
		cc.AddAnnotation(pvc, cc.AnnVddkVersion, vddkVersion)
	}
}

func updateImportSourceAnnotation(pvc, pvcPrime *corev1.PersistentVolumeClaim) {
	pvc.Annotations[cc.AnnSource] = cc.GetSource(pvcPrime)
}

// Progress reporting

func (r *ImportPopulatorReconciler) updateImportProgress(podPhase string, pvc, pvcPrime *corev1.PersistentVolumeClaim) error {
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	// Just set 100.0% if pod is succeeded
	if podPhase == string(corev1.PodSucceeded) {
		pvc.Annotations[cc.AnnImportProgressReporting] = "100.0%"
		return nil
	}
	importPod, err := r.getImportPod(pvc)
	if err != nil {
		return err
	}
	// This will only work when the import pod is running
	if importPod != nil && importPod.Status.Phase != corev1.PodRunning {
		return nil
	}
	url, err := cc.GetMetricsURL(importPod)
	if err != nil {
		return err
	}
	if url == "" {
		return nil
	}
	// We fetch the import progress from the import pod metrics
	importRegExp := regexp.MustCompile("progress\\{ownerUID\\=\"" + string(pvc.UID) + "\"\\} (\\d{1,3}\\.?\\d*)")
	httpClient = cc.BuildHTTPClient(httpClient)
	progressReport, err := cc.GetProgressReportFromURL(url, importRegExp, httpClient)
	if err != nil {
		return err
	}
	if progressReport != "" {
		if f, err := strconv.ParseFloat(progressReport, 64); err == nil {
			pvc.Annotations[cc.AnnImportProgressReporting] = fmt.Sprintf("%.2f%%", f)
		}
	}

	return nil
}

func (r *ImportPopulatorReconciler) getImportPod(pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
	importPodName, ok := pvc.Annotations[cc.AnnImportPod]
	if !ok {
		return nil, nil
	}

	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: importPodName, Namespace: pvc.GetNamespace()}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	if !metav1.IsControlledBy(pod, pvc) && !cc.IsImageStream(pvc) {
		return nil, errors.Errorf("Pod is not owned by PVC")
	}
	return pod, nil
}
