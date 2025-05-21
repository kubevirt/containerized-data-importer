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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	importMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
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
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}

	if err := addCommonPopulatorsWatches(mgr, importPopulator, log, cdiv1.VolumeImportSourceRef, &cdiv1.VolumeImportSource{}); err != nil {
		return nil, err
	}

	if err := addEventWatcher(mgr, importPopulator); err != nil {
		return nil, err
	}

	return importPopulator, nil
}

func addEventWatcher(mgr manager.Manager, c controller.Controller) error {
	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.Event{}, handler.TypedEnqueueRequestsFromMapFunc[*corev1.Event](
		func(_ context.Context, e *corev1.Event) []reconcile.Request {
			if e.InvolvedObject.Kind == "PersistentVolumeClaim" && strings.Contains(e.InvolvedObject.Name, "prime") {
				mgr.GetLogger().V(1).Info("DANNY: got prime event", "name", e.InvolvedObject.Name)
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{Name: e.Namespace},
				}}
			}
			return nil
		}),
	)); err != nil {
		return err
	}
	return nil
}

// Reconcile the reconcile loop for the PVC with DataSourceRef of VolumeImportSource kind
func (r *ImportPopulatorReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Import Source PVCs")
	return r.reconcile(req, r, log)
}

// Implementations of populatorController methods

// Import-specific implementation of getPopulationSource
func (r *ImportPopulatorReconciler) getPopulationSource(pvc *corev1.PersistentVolumeClaim) (client.Object, error) {
	volumeImportSourceKey := types.NamespacedName{Namespace: getPopulationSourceNamespace(pvc), Name: pvc.Spec.DataSourceRef.Name}
	volumeImportSource := &cdiv1.VolumeImportSource{}
	if err := r.client.Get(context.TODO(), volumeImportSourceKey, volumeImportSource); err != nil {
		// reconcile will be triggered once created
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	targetClaim := volumeImportSource.Spec.TargetClaim
	if targetClaim != nil && *targetClaim != "" && *targetClaim != pvc.Name {
		r.log.V(1).Info("volumeImportSource is meant for a different PVC, ignoring")
		return nil, nil
	}
	return volumeImportSource, nil
}

// Import-specific implementation of reconcileTargetPVC
func (r *ImportPopulatorReconciler) reconcileTargetPVC(pvc, pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	r.log.V(1).Info("DANNY: IN REC TARGET PVC")
	pvcCopy := pvc.DeepCopy()
	phase := pvcPrime.Annotations[cc.AnnPodPhase]
	source, err := r.getPopulationSource(pvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	// copy over any new events from pvcPrime to pvc
	CopyEvents(pvcPrime, pvc, r.client, r.log, r.recorder)

	switch phase {
	case string(corev1.PodRunning):
		if err = cc.MaybeSetPvcMultiStageAnnotation(pvcPrime, r.getCheckpointArgs(source)); err != nil {
			return reconcile.Result{}, err
		}
		if _, err = r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime, r.updateImportAnnotations); err != nil {
			return reconcile.Result{}, err
		}
		// We requeue to keep reporting progress
		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	case string(corev1.PodFailed):
		// We'll get called later once it succeeds
		r.recorder.Eventf(pvc, corev1.EventTypeWarning, importFailed, messageImportFailed, pvc.Name)
	case string(corev1.PodSucceeded):
		if cc.IsMultiStageImportInProgress(pvcPrime) {
			if err := cc.UpdatesMultistageImportSucceeded(pvcPrime, r.getCheckpointArgs(source)); err != nil {
				return reconcile.Result{}, err
			}
			r.recorder.Eventf(pvc, corev1.EventTypeNormal, cc.ImportPaused, cc.MessageImportPaused, pvc.Name)
			break
		}

		if cc.IsPVCComplete(pvcPrime) && cc.IsUnbound(pvc) {
			// Once the import is succeeded, we copy annotations and labels and rebind the PV from PVC to target PVC
			if pvcCopy, err = r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime, r.updateImportAnnotations); err != nil {
				return reconcile.Result{}, err
			}
			if pvcCopy, err = r.updatePVCWithPVCPrimeLabels(pvcCopy, pvcPrime.GetLabels()); err != nil {
				return reconcile.Result{}, err
			}
			if err := cc.Rebind(context.TODO(), r.client, pvcPrime, pvcCopy); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if _, err = r.updatePVCWithPVCPrimeAnnotations(pvcCopy, pvcPrime, r.updateImportAnnotations); err != nil {
		return reconcile.Result{}, err
	}
	if cc.IsPVCComplete(pvcPrime) && !cc.IsMultiStageImportInProgress(pvc) {
		r.recorder.Eventf(pvc, corev1.EventTypeNormal, importSucceeded, messageImportSucceeded, pvc.Name)
	}

	return reconcile.Result{}, nil
}

func CopyEvents(srcObj, destObj client.Object, c client.Client, log logr.Logger, recorder record.EventRecorder) {
	copyingToDv := false
	log.V(1).Info("DANNY: Kind = ", "kind", destObj.GetObjectKind().GroupVersionKind().Kind)
	if destObj.GetObjectKind().GroupVersionKind().Kind == "DataVolume" {
		copyingToDv = true
	}
	newEvents := &corev1.EventList{}
	err := c.List(context.TODO(), newEvents,
		client.InNamespace(srcObj.GetNamespace()),
		client.MatchingFields{"involvedObject.name": srcObj.GetName(),
			"involvedObject.uid": string(srcObj.GetUID())},
	)

	if err != nil {
		log.Error(err, "Could not retrieve destObj Prime list of Events")
	}

	oldEvents := &corev1.EventList{}
	err = c.List(context.TODO(), oldEvents,
		client.InNamespace(destObj.GetNamespace()),
		client.MatchingFields{"involvedObject.name": destObj.GetName(),
			"involvedObject.uid": string(destObj.GetUID())},
	)

	if err != nil {
		log.Error(err, "Could not retrieve PVC list of Events")
	}

	log.V(1).Info("List size", "srcObj", len(newEvents.Items), "regularPvc", len(oldEvents.Items))

	// Sort event lists by most recent
	sort.Slice(oldEvents.Items, func(i, j int) bool {
		return oldEvents.Items[i].FirstTimestamp.Time.After(oldEvents.Items[j].FirstTimestamp.Time)
	})

	sort.Slice(newEvents.Items, func(i, j int) bool {
		return newEvents.Items[i].FirstTimestamp.Time.After(newEvents.Items[j].FirstTimestamp.Time)
	})

	emitEvent := true
	for idx, newEvent := range newEvents.Items {
		emitEvent = true
		currTime := newEvent.FirstTimestamp.Unix()
		for _, oldEvent := range oldEvents.Items {
			log.V(1).Info("DANNY: comparing  ====== ")
			log.V(1).Info("old event", "event: ", oldEvent.Message)
			log.V(1).Info("new event", "event: ", newEvent.Message)
			// only want to emit new events from primePvc
			// since lists are sorted by time, if we find one we've emitted (prefixed with the primePVC name)
			// then all subsequent events have also been emitted
			if strings.Contains(oldEvent.Message, newEvent.Message) {
				// sometimes events have the equal timestamps, so evaulate next one before quitting
				if len(newEvents.Items) > idx+1 && currTime == newEvents.Items[idx+1].FirstTimestamp.Unix() {
					log.V(1).Info("Next Timstamp is equal, don't quit")
					emitEvent = false
					break
				} else {
					log.V(1).Info("DANNY: REJECTING")
					return
				}
			}
		}
		if emitEvent {
			message := ""
			// if we are copying to a DV, we only want to copy over events with pvcPrime prefix
			if copyingToDv {
				if !strings.Contains(newEvent.Message, "prime") {
					continue
				} else {
					message = newEvent.Message
				}
			} else {
				// only want to add pvcPrime prefix if we are copying to another PVC
				message = "[" + srcObj.GetName() + "] : " + newEvent.Message
			}
			recorder.Event(destObj, newEvent.Type, newEvent.Reason, message)
			log.V(1).Info("DANNY: EMITTING", "message", message)
		}
	}
	log.V(1).Info("DANNY: Ending Copy")
}

// Import-specific implementation of updatePVCForPopulation
func (r *ImportPopulatorReconciler) updatePVCForPopulation(pvc *corev1.PersistentVolumeClaim, source client.Object) {
	volumeImportSource := source.(*cdiv1.VolumeImportSource)
	annotations := pvc.Annotations
	annotations[cc.AnnPopulatorKind] = cdiv1.VolumeImportSourceRef
	annotations[cc.AnnContentType] = string(cc.GetContentType(volumeImportSource.Spec.ContentType))
	annotations[cc.AnnPreallocationRequested] = strconv.FormatBool(cc.GetPreallocation(context.TODO(), r.client, volumeImportSource.Spec.Preallocation))

	if checkpoint := cc.GetNextCheckpoint(pvc, r.getCheckpointArgs(source)); checkpoint != nil {
		annotations[cc.AnnCurrentCheckpoint] = checkpoint.Current
		annotations[cc.AnnPreviousCheckpoint] = checkpoint.Previous
		annotations[cc.AnnFinalCheckpoint] = strconv.FormatBool(checkpoint.IsFinal)
	}

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

func (r *ImportPopulatorReconciler) updateImportAnnotations(pvc, pvcPrime *corev1.PersistentVolumeClaim) {
	phase := pvcPrime.Annotations[cc.AnnPodPhase]
	if err := r.updateImportProgress(phase, pvc, pvcPrime); err != nil {
		r.log.Error(err, fmt.Sprintf("Failed to update import progress for pvc %s/%s", pvc.Namespace, pvc.Name))
	}
	updateVddkAnnotations(pvc, pvcPrime)
}

// Progress reporting

func (r *ImportPopulatorReconciler) updateImportProgress(podPhase string, pvc, pvcPrime *corev1.PersistentVolumeClaim) error {
	// Just set 100.0% if pod is succeeded
	if podPhase == string(corev1.PodSucceeded) {
		cc.AddAnnotation(pvc, cc.AnnPopulatorProgress, "100.0%")
		return nil
	}

	importPodName, ok := pvcPrime.Annotations[cc.AnnImportPod]
	if !ok {
		return nil
	}

	importPod, err := r.getImportPod(pvcPrime, importPodName)
	if err != nil {
		return err
	}

	if importPod == nil {
		_, ok := pvc.Annotations[cc.AnnPopulatorProgress]
		// Initialize the progress once PVC Prime is bound
		if !ok && pvcPrime.Status.Phase == corev1.ClaimBound {
			cc.AddAnnotation(pvc, cc.AnnPopulatorProgress, "N/A")
		}
		return nil
	}

	// This will only work when the import pod is running
	if importPod.Status.Phase != corev1.PodRunning {
		return nil
	}

	url, err := cc.GetMetricsURL(importPod)
	if url == "" || err != nil {
		return err
	}

	// We fetch the import progress from the import pod metrics
	httpClient = cc.BuildHTTPClient(httpClient)
	progressReport, err := cc.GetProgressReportFromURL(context.TODO(), url, httpClient, importMetrics.ImportProgressMetricName, string(pvc.UID))
	if err != nil {
		return err
	}
	if progressReport != "" {
		if strings.HasPrefix(progressReport, "100") {
			// Hold on with reporting 100% since that may not be accounting for resize/convert etc
			return nil
		}
		if f, err := strconv.ParseFloat(progressReport, 64); err == nil {
			cc.AddAnnotation(pvc, cc.AnnPopulatorProgress, fmt.Sprintf("%.2f%%", f))
		}
	}

	return nil
}

func (r *ImportPopulatorReconciler) getImportPod(pvc *corev1.PersistentVolumeClaim, importPodName string) (*corev1.Pod, error) {
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

func (r *ImportPopulatorReconciler) getCheckpointArgs(source client.Object) *cc.CheckpointArgs {
	isFinal := false
	checkpoints := []cdiv1.DataVolumeCheckpoint{}
	// We attempt to allow finishing the population
	// even if the VolumeImportSource is deleted.
	// The VolumeImportSource would only be required in multi-stage imports.
	if source != nil {
		volumeImportSource := source.(*cdiv1.VolumeImportSource)
		if volumeImportSource.Spec.FinalCheckpoint != nil {
			isFinal = *volumeImportSource.Spec.FinalCheckpoint
		}
		checkpoints = volumeImportSource.Spec.Checkpoints
	}
	return &cc.CheckpointArgs{
		Checkpoints: checkpoints,
		IsFinal:     isFinal,
		Client:      r.client,
		Log:         r.log,
	}
}
