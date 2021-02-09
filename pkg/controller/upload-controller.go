/*
Copyright 2018 The CDI Authors.

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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = "cdi.kubevirt.io/storage.upload.target"

	// AnnUploadClientName is the TLS name uploadserver will accept requests from
	AnnUploadClientName = "cdi.kubevirt.io/uploadClientName"

	// AnnUploadPod name of the upload pod
	AnnUploadPod = "cdi.kubevirt.io/storage.uploadPodName"

	annCreatedByUpload = "cdi.kubevirt.io/storage.createdByUploadController"

	uploadServerClientName = "client.upload-server.cdi.kubevirt.io"

	uploadServerCertDuration = 365 * 24 * time.Hour

	// UploadSucceededPVC provides a const to indicate an import to the PVC failed
	UploadSucceededPVC = "UploadSucceeded"

	// UploadTargetInUse is reason for event created when an upload pvc is in use
	UploadTargetInUse = "UploadTargetInUse"
)

// UploadReconciler members
type UploadReconciler struct {
	client                 client.Client
	recorder               record.EventRecorder
	scheme                 *runtime.Scheme
	log                    logr.Logger
	image                  string
	verbose                string
	pullPolicy             string
	uploadProxyServiceName string
	serverCertGenerator    generator.CertGenerator
	clientCAFetcher        fetcher.CertBundleFetcher
	featureGates           featuregates.FeatureGates
}

// UploadPodArgs are the parameters required to create an upload pod
type UploadPodArgs struct {
	Name                            string
	PVC                             *v1.PersistentVolumeClaim
	ScratchPVCName                  string
	ClientName                      string
	FilesystemOverhead              string
	ServerCert, ServerKey, ClientCA []byte
	Preallocation                   string
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *UploadReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Upload PVCs")

	// Get the PVC.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	_, isUpload := pvc.Annotations[AnnUploadRequest]
	_, isCloneTarget := pvc.Annotations[AnnCloneRequest]

	if isUpload && isCloneTarget {
		log.V(1).Info("PVC has both clone and upload annotations")
		return reconcile.Result{}, errors.New("PVC has both clone and upload annotations")
	}
	shouldReconcile, err := r.shouldReconcile(isUpload, isCloneTarget, pvc, log)
	if err != nil {
		return reconcile.Result{}, err
	}
	// force cleanup if PVC pending delete and pod running or the upload/clone annotation was removed
	if !shouldReconcile || podSucceededFromPVC(pvc) || pvc.DeletionTimestamp != nil {
		log.V(1).Info("not doing anything with PVC",
			"isUpload", isUpload,
			"isCloneTarget", isCloneTarget,
			"isBound", isBound(pvc, log),
			"podSucceededFromPVC", podSucceededFromPVC(pvc),
			"deletionTimeStamp set?", pvc.DeletionTimestamp != nil)
		if err := r.cleanup(pvc); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	log.Info("Calling Upload reconcile PVC")
	return r.reconcilePVC(log, pvc, isCloneTarget)
}

func (r *UploadReconciler) shouldReconcile(isUpload bool, isCloneTarget bool, pvc *v1.PersistentVolumeClaim, log logr.Logger) (bool, error) {
	_, isImmediateBindingRequested := pvc.Annotations[AnnImmediateBinding]
	waitForFirstConsumerEnabled, err := isWaitForFirstConsumerEnabled(isImmediateBindingRequested, r.featureGates)
	if err != nil {
		return false, err
	}

	return (isUpload || isCloneTarget) &&
			shouldHandlePvc(pvc, waitForFirstConsumerEnabled, log),
		nil
}

func (r *UploadReconciler) reconcilePVC(log logr.Logger, pvc *corev1.PersistentVolumeClaim, isCloneTarget bool) (reconcile.Result, error) {
	var uploadClientName string
	pvcCopy := pvc.DeepCopy()
	anno := pvcCopy.Annotations

	if isCloneTarget {
		source, err := r.getCloneRequestSourcePVC(pvc)
		if err != nil {
			return reconcile.Result{}, err
		}
		contentType, err := ValidateCanCloneSourceAndTargetContentType(source, pvc)
		if err != nil {
			return reconcile.Result{}, err
		}
		if err = ValidateCanCloneSourceAndTargetSpec(&source.Spec, &pvc.Spec, contentType); err != nil {
			log.Error(err, "Error validating clone spec, ignoring")
			return reconcile.Result{}, nil
		}

		uploadClientName = fmt.Sprintf("%s/%s-%s/%s", source.Namespace, source.Name, pvc.Namespace, pvc.Name)
		anno[AnnUploadClientName] = uploadClientName
	} else {
		uploadClientName = uploadServerClientName
	}

	pod, err := r.findUploadPodForPvc(pvc, log)
	if err != nil {
		return reconcile.Result{}, err
	}

	if pod == nil {
		podsUsingPVC, err := GetPodsUsingPVCs(r.client, pvc.Namespace, sets.NewString(pvc.Name), false)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(podsUsingPVC) > 0 {
			for _, pod := range podsUsingPVC {
				r.log.V(1).Info("can't create upload pod, pvc in use by other pod",
					"namespace", pvc.Namespace, "name", pvc.Name, "pod", pod.Name)
				r.recorder.Eventf(pvc, corev1.EventTypeWarning, UploadTargetInUse,
					"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, pvc.Name)

			}
			return reconcile.Result{Requeue: true}, nil
		}

		podName, ok := pvc.Annotations[AnnUploadPod]
		scratchPVCName := createScratchPvcNameFromPvc(pvc, isCloneTarget)

		if !ok {
			podName = createUploadResourceName(pvc.Name)
			if err := r.updatePvcPodName(pvc, podName, log); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
		pod, err = r.createUploadPodForPvc(pvc, podName, scratchPVCName, uploadClientName)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Always try to get or create the scratch PVC for a pod that is not successful yet, if it exists nothing happens otherwise attempt to create.
	scratchPVCName, exists := getScratchNameFromPod(pod)
	if exists {
		_, err := r.getOrCreateScratchPvc(pvcCopy, pod, scratchPVCName)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	svcName := naming.GetServiceNameFromResourceName(pod.Name)
	if _, err = r.getOrCreateUploadService(pvc, svcName); err != nil {
		return reconcile.Result{}, err
	}

	podPhase := pod.Status.Phase
	anno[AnnPodPhase] = string(podPhase)
	anno[AnnPodReady] = strconv.FormatBool(isPodReady(pod))

	if pod.Status.ContainerStatuses != nil {
		// update pvc annotation tracking pod restarts only if the source pod restart count is greater
		// see the same in clone-controller
		pvcAnnPodRestarts, _ := strconv.Atoi(anno[AnnPodRestarts])
		podRestarts := int(pod.Status.ContainerStatuses[0].RestartCount)
		if podRestarts > pvcAnnPodRestarts {
			anno[AnnPodRestarts] = strconv.Itoa(podRestarts)
		}

		if pod.Status.ContainerStatuses[0].State.Terminated != nil &&
			pod.Status.ContainerStatuses[0].State.Terminated.ExitCode == 0 {
			if strings.Contains(pod.Status.ContainerStatuses[0].State.Terminated.Message, PreallocationApplied) {
				anno[AnnPreallocationApplied] = "true"
			}
		}
	}
	setConditionFromPodWithPrefix(anno, AnnRunningCondition, pod)

	if !reflect.DeepEqual(pvc, pvcCopy) {
		if err := r.updatePVC(pvcCopy); err != nil {
			return reconcile.Result{}, err
		}
		if podSucceededFromPVC(pvcCopy) && !isCloneTarget {
			// Upload completed, emit event. clone controller will emit clone complete.
			r.recorder.Event(pvc, corev1.EventTypeNormal, UploadSucceededPVC, "Upload Successful")
		}
	}

	return reconcile.Result{}, nil
}

func (r *UploadReconciler) updatePvcPodName(pvc *v1.PersistentVolumeClaim, podName string, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Updating PVC from pod")
	anno := pvc.GetAnnotations()
	anno[AnnUploadPod] = podName

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		if err := r.updatePVC(pvc); err != nil {
			return err
		}
		log.V(1).Info("Updated PVC", "pvc.anno.AnnImportPod", anno[AnnUploadPod])
	}
	return nil
}

func (r *UploadReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim) error {
	r.log.V(1).Info("Phase is now", "pvc.anno.Phase", pvc.GetAnnotations()[AnnPodPhase])
	if err := r.client.Update(context.TODO(), pvc); err != nil {
		return err
	}
	return nil
}

func (r *UploadReconciler) getCloneRequestSourcePVC(targetPvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	sourceVolumeMode := corev1.PersistentVolumeFilesystem
	targetVolumeMode := corev1.PersistentVolumeFilesystem

	exists, namespace, name := ParseCloneRequestAnnotation(targetPvc)
	if !exists {
		return nil, errors.New("error parsing clone request annotation")
	}
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sourcePvc); err != nil {
		return nil, errors.Wrap(err, "error getting clone source PVC")
	}
	if sourcePvc.Spec.VolumeMode != nil {
		sourceVolumeMode = *sourcePvc.Spec.VolumeMode
	}
	if targetPvc.Spec.VolumeMode != nil {
		targetVolumeMode = *targetPvc.Spec.VolumeMode
	}
	// Allow different source and target volume modes only on KubeVirt content type
	contentType, err := ValidateCanCloneSourceAndTargetContentType(sourcePvc, targetPvc)
	if err != nil {
		return nil, err
	}
	if sourceVolumeMode != targetVolumeMode && contentType != cdiv1.DataVolumeKubeVirt {
		return nil, errors.New("Source and target volume modes do not match, and content type is not kubevirt")
	}
	return sourcePvc, nil
}

func (r *UploadReconciler) cleanup(pvc *v1.PersistentVolumeClaim) error {
	resourceName := getUploadResourceNameFromPvc(pvc)
	svcName := naming.GetServiceNameFromResourceName(resourceName)

	// delete service
	if err := r.deleteService(pvc.Namespace, svcName); err != nil {
		return err
	}

	// delete pod
	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: resourceName, Namespace: pvc.Namespace}, pod); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if pod.DeletionTimestamp == nil {
		if err := r.client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
func (r *UploadReconciler) findUploadPodForPvc(pvc *v1.PersistentVolumeClaim, log logr.Logger) (*v1.Pod, error) {
	podName := getUploadResourceNameFromPvc(pvc)
	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: pvc.Namespace}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "error getting upload pod %s/%s", pvc.Namespace, podName)
		}
		return nil, nil
	}

	if !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("%s pod not controlled by pvc %s", podName, pvc.Name)
	}

	return pod, nil
}

func (r *UploadReconciler) createUploadPodForPvc(pvc *v1.PersistentVolumeClaim, podName, scratchPVCName, clientName string) (*v1.Pod, error) {
	pod := &corev1.Pod{}

	serverCert, serverKey, err := r.serverCertGenerator.MakeServerCert(pvc.Namespace, naming.GetServiceNameFromResourceName(podName), uploadServerCertDuration)
	if err != nil {
		return nil, err
	}

	clientCA, err := r.clientCAFetcher.BundleBytes()
	if err != nil {
		return nil, err
	}

	fsOverhead, err := GetFilesystemOverhead(r.client, pvc)
	if err != nil {
		return nil, err
	}

	preallocationRequested := false
	if preallocation, err := strconv.ParseBool(getValueFromAnnotation(pvc, AnnPreallocationRequested)); err == nil {
		preallocationRequested = preallocation
	}

	args := UploadPodArgs{
		Name:               podName,
		PVC:                pvc,
		ScratchPVCName:     scratchPVCName,
		ClientName:         clientName,
		FilesystemOverhead: string(fsOverhead),
		ServerCert:         serverCert,
		ServerKey:          serverKey,
		ClientCA:           clientCA,
		Preallocation:      strconv.FormatBool(preallocationRequested),
	}

	r.log.V(3).Info("Creating upload pod")
	pod, err = r.createUploadPod(args)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *UploadReconciler) getOrCreateScratchPvc(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name string) (*v1.PersistentVolumeClaim, error) {
	// Set condition, then check if need to override with scratch pvc message
	anno := pvc.Annotations
	scratchPvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: pvc.Namespace}, scratchPvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting scratch PVC")
		}

		storageClassName := GetScratchPvcStorageClass(r.client, pvc)

		anno[AnnBoundCondition] = "false"
		anno[AnnBoundConditionMessage] = "Creating scratch space"
		anno[AnnBoundConditionReason] = creatingScratch
		// Scratch PVC doesn't exist yet, create it.
		scratchPvc, err = CreateScratchPersistentVolumeClaim(r.client, pvc, pod, name, storageClassName)
		if err != nil {
			return nil, err
		}
	} else {
		if !metav1.IsControlledBy(scratchPvc, pod) {
			return nil, errors.Errorf("%s scratch PVC not controlled by pod %s", scratchPvc.Name, pod.Name)
		}
		setBoundConditionFromPVC(anno, AnnBoundCondition, scratchPvc)
	}

	return scratchPvc, nil
}

func (r *UploadReconciler) getOrCreateUploadService(pvc *v1.PersistentVolumeClaim, name string) (*v1.Service, error) {
	service := &corev1.Service{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: pvc.Namespace}, service); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting upload service")
		}
		service, err = r.createUploadService(name, pvc)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(service, pvc) {
		return nil, errors.Errorf("%s service not controlled by pvc %s", name, pvc.Name)
	}

	return service, nil
}

func (r *UploadReconciler) deleteService(namespace, serviceName string) error {
	service := &corev1.Service{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: namespace}, service); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if service.DeletionTimestamp == nil {
		if err := r.client.Delete(context.TODO(), service); IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "error deleting upload service")
		}
	}

	return nil
}

// createUploadService creates upload service service manifest and sends to server
func (r *UploadReconciler) createUploadService(name string, pvc *v1.PersistentVolumeClaim) (*v1.Service, error) {
	ns := pvc.Namespace
	service := r.makeUploadServiceSpec(name, pvc)

	if err := r.client.Create(context.TODO(), service); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, service); err != nil {
				return nil, errors.Wrap(err, "upload service should exist but couldn't retrieve it")
			}
		} else {
			return nil, errors.Wrap(err, "upload service API create errored")
		}
	}
	r.log.V(1).Info("upload service created\n", "Namespace", service.Namespace, "Name", service.Name)
	return service, nil
}

// makeUploadServiceSpec creates upload service service manifest
func (r *UploadReconciler) makeUploadServiceSpec(name string, pvc *v1.PersistentVolumeClaim) *v1.Service {
	blockOwnerDeletion := true
	isController := true
	service := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.UploadServerCDILabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8443,
					},
				},
			},
			Selector: map[string]string{
				common.UploadServerServiceLabel: name,
			},
		},
	}
	return service
}

// createUploadPod creates upload service pod manifest and sends to server
func (r *UploadReconciler) createUploadPod(args UploadPodArgs) (*v1.Pod, error) {
	ns := args.PVC.Namespace

	podResourceRequirements, err := GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := GetWorkloadNodePlacement(r.client)
	if err != nil {
		return nil, err
	}

	pod := r.makeUploadPodSpec(args, podResourceRequirements, workloadNodePlacement)

	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: args.Name, Namespace: ns}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "upload pod should exist but couldn't retrieve it")
		}
		if err := r.client.Create(context.TODO(), pod); err != nil {
			return nil, err
		}
	}

	r.log.V(1).Info("upload pod created\n", "Namespace", pod.Namespace, "Name", pod.Name, "Image name", r.image)
	return pod, nil
}

// NewUploadController creates a new instance of the upload controller.
func NewUploadController(mgr manager.Manager, log logr.Logger, uploadImage, pullPolicy, verbose string, serverCertGenerator generator.CertGenerator, clientCAFetcher fetcher.CertBundleFetcher) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &UploadReconciler{
		client:              client,
		scheme:              mgr.GetScheme(),
		log:                 log.WithName("upload-controller"),
		image:               uploadImage,
		verbose:             verbose,
		pullPolicy:          pullPolicy,
		recorder:            mgr.GetEventRecorderFor("upload-controller"),
		serverCertGenerator: serverCertGenerator,
		clientCAFetcher:     clientCAFetcher,
		featureGates:        featuregates.NewFeatureGates(client),
	}
	uploadController, err := controller.New("upload-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addUploadControllerWatches(mgr, uploadController); err != nil {
		return nil, err
	}

	return uploadController, nil
}

func addUploadControllerWatches(mgr manager.Manager, uploadController controller.Controller) error {
	// Setup watches
	if err := uploadController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := uploadController.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &corev1.PersistentVolumeClaim{},
		IsController: true,
	}); err != nil {
		return err
	}
	if err := uploadController.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &corev1.PersistentVolumeClaim{},
		IsController: true,
	}); err != nil {
		return err
	}

	return nil
}

func createScratchPvcNameFromPvc(pvc *v1.PersistentVolumeClaim, isCloneTarget bool) string {
	if isCloneTarget {
		return ""
	}

	return naming.GetResourceName(pvc.Name, common.ScratchNameSuffix)
}

// getUploadResourceName returns the name given to upload resources
func getUploadResourceNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	podName, ok := pvc.Annotations[AnnUploadPod]
	if ok {
		return podName
	}

	// fallback to legacy naming, in fact the following function is fully compatible with legacy
	// name concatenation "cdi-upload-{pvc.Name}" if the name length is under the size limits,
	return naming.GetResourceName("cdi-upload", pvc.Name)
}

// createUploadResourceName returns the name given to upload resources
func createUploadResourceName(name string) string {
	return naming.GetResourceName("cdi-upload", name)
}

// UploadPossibleForPVC is called by the api server to see whether to return an upload token
func UploadPossibleForPVC(pvc *v1.PersistentVolumeClaim) error {
	if _, ok := pvc.Annotations[AnnUploadRequest]; !ok {
		return errors.Errorf("PVC %s is not an upload target", pvc.Name)
	}
	return nil
}

// GetUploadServerURL returns the url the proxy should post to for a particular pvc
func GetUploadServerURL(namespace, pvc, uploadPath string) string {
	serviceName := createUploadServiceNameFromPvcName(pvc)
	return fmt.Sprintf("https://%s.%s.svc%s", serviceName, namespace, uploadPath)
}

// createUploadServiceName returns the name given to upload service shortened if needed
func createUploadServiceNameFromPvcName(pvc string) string {
	return naming.GetServiceNameFromResourceName(createUploadResourceName(pvc))
}

func (r *UploadReconciler) makeUploadPodSpec(args UploadPodArgs, resourceRequirements *v1.ResourceRequirements, workloadNodePlacement *sdkapi.NodePlacement) *v1.Pod {
	requestImageSize, _ := getRequestedImageSize(args.PVC)
	serviceName := naming.GetServiceNameFromResourceName(args.Name)
	fsGroup := common.QemuSubGid
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: args.PVC.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:              common.CDILabelValue,
				common.CDIComponentLabel:        common.UploadServerCDILabel,
				common.UploadServerServiceLabel: serviceName,
				common.UploadTargetLabel:        string(args.PVC.UID),
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePVCOwnerReference(args.PVC),
			},
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []v1.Container{
				{
					Name:            common.UploadServerPodname,
					Image:           r.image,
					ImagePullPolicy: v1.PullPolicy(r.pullPolicy),
					Env: []v1.EnvVar{
						{
							Name:  "TLS_KEY",
							Value: string(args.ServerKey),
						},
						{
							Name:  "TLS_CERT",
							Value: string(args.ServerCert),
						},
						{
							Name:  "CLIENT_CERT",
							Value: string(args.ClientCA),
						},
						{
							Name:  common.FilesystemOverheadVar,
							Value: args.FilesystemOverhead,
						},
						{
							Name:  common.UploadImageSize,
							Value: requestImageSize,
						},
						{
							Name:  "CLIENT_NAME",
							Value: args.ClientName,
						},
						{
							Name:  common.Preallocation,
							Value: args.Preallocation,
						},
					},
					Args: []string{"-v=" + r.verbose},
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 8080,
								},
							},
						},
						InitialDelaySeconds: 2,
						PeriodSeconds:       5,
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			Volumes: []v1.Volume{
				{
					Name: DataVolName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: args.PVC.Name,
							ReadOnly:  false,
						},
					},
				},
			},
			NodeSelector: workloadNodePlacement.NodeSelector,
			Tolerations:  workloadNodePlacement.Tolerations,
			Affinity:     workloadNodePlacement.Affinity,
		},
	}

	if !checkPVC(args.PVC, AnnCloneRequest, r.log.WithValues("Name", args.PVC.Name, "Namspace", args.PVC.Namespace)) {
		pod.Spec.SecurityContext.FSGroup = &fsGroup
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	if getVolumeMode(args.PVC) == v1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = []v1.VolumeDevice{
			{
				Name:       DataVolName,
				DevicePath: common.WriteBlockPath,
			},
		}
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "DESTINATION",
			Value: common.WriteBlockPath,
		})
	} else {
		pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.UploadServerDataDir,
			},
		}
	}

	if args.ScratchPVCName != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: ScratchVolName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: args.ScratchPVCName,
					ReadOnly:  false,
				},
			},
		})

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}
	SetPodPvcAnnotations(pod, args.PVC)
	return pod
}
