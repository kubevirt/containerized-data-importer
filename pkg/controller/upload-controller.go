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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

const (
	// AnnUploadClientName is the TLS name uploadserver will accept requests from
	AnnUploadClientName = "cdi.kubevirt.io/uploadClientName"

	// AnnUploadPod name of the upload pod
	AnnUploadPod = "cdi.kubevirt.io/storage.uploadPodName"

	annCreatedByUpload = "cdi.kubevirt.io/storage.createdByUploadController"

	uploadServerClientName = "client.upload-server.cdi.kubevirt.io"

	// UploadSucceededPVC provides a const to indicate an import to the PVC failed
	UploadSucceededPVC = "UploadSucceeded"

	// UploadTargetInUse is reason for event created when an upload pvc is in use
	UploadTargetInUse = "UploadTargetInUse"

	certVolName = "tls-config"

	certMountPath = "/etc/tls/"

	serverCertFile = certMountPath + "tls.crt"

	serverKeyFile = certMountPath + "tls.key"

	clientCertFile = certMountPath + "ca.crt"
)

// UploadReconciler members
type UploadReconciler struct {
	client              client.Client
	recorder            record.EventRecorder
	scheme              *runtime.Scheme
	log                 logr.Logger
	image               string
	verbose             string
	pullPolicy          string
	serverCertGenerator generator.CertGenerator
	clientCAFetcher     fetcher.CertBundleFetcher
	featureGates        featuregates.FeatureGates
	installerLabels     map[string]string
}

// UploadPodArgs are the parameters required to create an upload pod
type UploadPodArgs struct {
	Name                            string
	PVC                             *corev1.PersistentVolumeClaim
	ScratchPVCName                  string
	ClientName                      string
	FilesystemOverhead              string
	ServerCert, ServerKey, ClientCA []byte
	Preallocation                   string
	CryptoEnvVars                   CryptoEnvVars
	Deadline                        *time.Time
}

// CryptoEnvVars holds the TLS crypto-related configurables for the upload server
type CryptoEnvVars struct {
	Ciphers       string
	MinTLSVersion string
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *UploadReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
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

	_, isUpload := pvc.Annotations[cc.AnnUploadRequest]
	_, isCloneTarget := pvc.Annotations[cc.AnnCloneRequest]

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

func (r *UploadReconciler) shouldReconcile(isUpload bool, isCloneTarget bool, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (bool, error) {
	waitForFirstConsumerEnabled, err := cc.IsWaitForFirstConsumerEnabled(pvc, r.featureGates)
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
		if err = ValidateCanCloneSourceAndTargetSpec(context.TODO(), r.client, source, pvc, contentType); err != nil {
			log.Error(err, "Error validating clone spec, ignoring")
			r.recorder.Eventf(pvc, corev1.EventTypeWarning, cc.ErrIncompatiblePVC, err.Error())
			return reconcile.Result{}, nil
		}

		uploadClientName = fmt.Sprintf("%s/%s-%s/%s", source.Namespace, source.Name, pvc.Namespace, pvc.Name)
		anno[AnnUploadClientName] = uploadClientName
	} else {
		uploadClientName = uploadServerClientName
	}

	pod, err := r.findUploadPodForPvc(pvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	if pod == nil {
		podsUsingPVC, err := cc.GetPodsUsingPVCs(context.TODO(), r.client, pvc.Namespace, sets.New(pvc.Name), false)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(podsUsingPVC) > 0 {
			es, err := cc.GetAnnotatedEventSource(context.TODO(), r.client, pvc)
			if err != nil {
				return reconcile.Result{}, err
			}

			for _, pod := range podsUsingPVC {
				log.V(1).Info("can't create upload pod, pvc in use by other pod",
					"namespace", pvc.Namespace, "name", pvc.Name, "pod", pod.Name)
				r.recorder.Eventf(es, corev1.EventTypeWarning, UploadTargetInUse,
					"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, pvc.Name)
			}
			return reconcile.Result{Requeue: true}, nil
		}

		podName, ok := pvc.Annotations[AnnUploadPod]

		if !ok {
			podName = createUploadResourceName(pvc.Name)
			if err := r.updatePvcPodName(pvc, podName, log); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
		pod, err = r.createUploadPodForPvc(pvc, podName, uploadClientName, isCloneTarget)
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

	termMsg, err := parseTerminationMessage(pod)
	if err != nil {
		return reconcile.Result{}, err
	}

	deadlinePassed := termMsg != nil && termMsg.DeadlinePassed != nil && *termMsg.DeadlinePassed
	if deadlinePassed {
		if pod.DeletionTimestamp == nil {
			log.V(1).Info("Deleting pod because deadline exceeded")
			if err := r.client.Delete(context.TODO(), pod); err != nil {
				return reconcile.Result{}, err
			}
		}

		anno[cc.AnnPodPhase] = ""
		anno[cc.AnnPodReady] = "false"
	} else {
		anno[cc.AnnPodPhase] = string(pod.Status.Phase)
		anno[cc.AnnPodReady] = strconv.FormatBool(isPodReady(pod))
	}

	setAnnotationsFromPodWithPrefix(anno, pod, termMsg, cc.AnnRunningCondition)

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

func (r *UploadReconciler) updatePvcPodName(pvc *corev1.PersistentVolumeClaim, podName string, log logr.Logger) error {
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
	r.log.V(1).Info("Phase is now", "pvc.anno.Phase", pvc.GetAnnotations()[cc.AnnPodPhase])
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

func (r *UploadReconciler) cleanup(pvc *corev1.PersistentVolumeClaim) error {
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
	if pod.DeletionTimestamp == nil && cc.ShouldDeletePod(pvc) {
		if err := r.client.Delete(context.TODO(), pod); cc.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
func (r *UploadReconciler) findUploadPodForPvc(pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
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

func (r *UploadReconciler) createUploadPodForPvc(pvc *corev1.PersistentVolumeClaim, podName, clientName string, isCloneTarget bool) (*corev1.Pod, error) {
	certConfig, err := operator.GetCertConfigWithDefaults(context.TODO(), r.client)
	if err != nil {
		return nil, err
	}

	serverCert, serverKey, err := r.serverCertGenerator.MakeServerCert(
		pvc.Namespace,
		naming.GetServiceNameFromResourceName(podName),
		certConfig.Server.Duration.Duration,
	)
	if err != nil {
		return nil, err
	}

	clientCA, err := r.clientCAFetcher.BundleBytes()
	if err != nil {
		return nil, err
	}

	fsOverhead, err := GetFilesystemOverhead(context.TODO(), r.client, pvc)
	if err != nil {
		return nil, err
	}

	preallocationRequested := false
	if preallocation, err := strconv.ParseBool(getValueFromAnnotation(pvc, cc.AnnPreallocationRequested)); err == nil {
		preallocationRequested = preallocation
	}

	config := &cdiv1.CDIConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return nil, err
	}
	ciphers, minTLSVersion := cryptowatch.SelectCipherSuitesAndMinTLSVersion(config.Spec.TLSSecurityProfile)
	cryptoVars := CryptoEnvVars{
		Ciphers:       strings.Join(ciphers, ","),
		MinTLSVersion: string(minTLSVersion),
	}

	serverRefresh := certConfig.Server.Duration.Duration - certConfig.Server.RenewBefore.Duration
	clientRefresh := certConfig.Client.Duration.Duration - certConfig.Client.RenewBefore.Duration

	args := UploadPodArgs{
		Name:               podName,
		PVC:                pvc,
		ScratchPVCName:     createScratchPvcNameFromPvc(pvc, isCloneTarget),
		ClientName:         clientName,
		FilesystemOverhead: string(fsOverhead),
		ServerCert:         serverCert,
		ServerKey:          serverKey,
		ClientCA:           clientCA,
		Preallocation:      strconv.FormatBool(preallocationRequested),
		CryptoEnvVars:      cryptoVars,
		Deadline:           ptr.To(time.Now().Add(min(serverRefresh, clientRefresh))),
	}

	r.log.V(3).Info("Creating upload pod")
	pod, err := r.createUploadPod(args)
	// Check if pod has failed and, in that case, record an event with the error
	if podErr := cc.HandleFailedPod(err, podName, pvc, r.recorder, r.client); podErr != nil {
		return nil, podErr
	}

	if err := r.ensureCertSecret(args, pod); err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *UploadReconciler) getOrCreateScratchPvc(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, name string) (*corev1.PersistentVolumeClaim, error) {
	// Set condition, then check if need to override with scratch pvc message
	anno := pvc.Annotations
	scratchPvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: pvc.Namespace}, scratchPvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting scratch PVC")
		}

		storageClassName := GetScratchPvcStorageClass(r.client, pvc)

		anno[cc.AnnBoundCondition] = "false"
		anno[cc.AnnBoundConditionMessage] = "Creating scratch space"
		anno[cc.AnnBoundConditionReason] = creatingScratch
		// Scratch PVC doesn't exist yet, create it.
		scratchPvc, err = createScratchPersistentVolumeClaim(r.client, pvc, pod, name, storageClassName, map[string]string{}, r.recorder)
		if err != nil {
			return nil, err
		}
	} else {
		if !metav1.IsControlledBy(scratchPvc, pod) {
			return nil, errors.Errorf("%s scratch PVC not controlled by pod %s", scratchPvc.Name, pod.Name)
		}
		setBoundConditionFromPVC(anno, cc.AnnBoundCondition, scratchPvc)
	}

	return scratchPvc, nil
}

func (r *UploadReconciler) getOrCreateUploadService(pvc *corev1.PersistentVolumeClaim, name string) (*corev1.Service, error) {
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
		if err := r.client.Delete(context.TODO(), service); cc.IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "error deleting upload service")
		}
	}

	return nil
}

func isPodReady(pod *corev1.Pod) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}

	numReady := 0
	for _, s := range pod.Status.ContainerStatuses {
		if s.Ready {
			numReady++
		}
	}

	return numReady == len(pod.Status.ContainerStatuses)
}

// createUploadService creates an upload service manifest and sends it to server
func (r *UploadReconciler) createUploadService(name string, pvc *corev1.PersistentVolumeClaim) (*corev1.Service, error) {
	ns := pvc.Namespace
	service := r.makeUploadServiceSpec(name, pvc)
	util.SetRecommendedLabels(service, r.installerLabels, "cdi-controller")

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

// makeUploadServiceSpec creates upload service manifest
func (r *UploadReconciler) makeUploadServiceSpec(name string, pvc *corev1.PersistentVolumeClaim) *corev1.Service {
	blockOwnerDeletion := true
	isController := true
	service := &corev1.Service{
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
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
func (r *UploadReconciler) createUploadPod(args UploadPodArgs) (*corev1.Pod, error) {
	ns := args.PVC.Namespace

	podResourceRequirements, err := cc.GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	imagePullSecrets, err := cc.GetImagePullSecrets(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(context.TODO(), r.client)
	if err != nil {
		return nil, err
	}

	pod := r.makeUploadPodSpec(args, podResourceRequirements, imagePullSecrets, workloadNodePlacement)
	util.SetRecommendedLabels(pod, r.installerLabels, "cdi-controller")

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

func (r *UploadReconciler) ensureCertSecret(args UploadPodArgs, pod *corev1.Pod) error {
	if pod.Status.Phase == corev1.PodRunning {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: pod.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.UploadServerCDILabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePodOwnerReference(pod),
			},
		},
		Data: map[string][]byte{
			"tls.key": args.ServerKey,
			"tls.crt": args.ServerCert,
			"ca.crt":  args.ClientCA,
		},
	}

	util.SetRecommendedLabels(secret, r.installerLabels, "cdi-controller")

	err := r.client.Create(context.TODO(), secret)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "error creating cert secret")
	}

	return nil
}

// NewUploadController creates a new instance of the upload controller.
func NewUploadController(mgr manager.Manager, log logr.Logger, uploadImage, pullPolicy, verbose string, serverCertGenerator generator.CertGenerator, clientCAFetcher fetcher.CertBundleFetcher, installerLabels map[string]string) (controller.Controller, error) {
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
		installerLabels:     installerLabels,
	}
	uploadController, err := controller.New("upload-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
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
	if err := uploadController.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolumeClaim]{})); err != nil {
		return err
	}
	if err := uploadController.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, handler.TypedEnqueueRequestForOwner[*corev1.Pod](
		mgr.GetScheme(), mgr.GetClient().RESTMapper(), &corev1.PersistentVolumeClaim{}, handler.OnlyControllerOwner()))); err != nil {
		return err
	}
	if err := uploadController.Watch(source.Kind(mgr.GetCache(), &corev1.Service{}, handler.TypedEnqueueRequestForOwner[*corev1.Service](
		mgr.GetScheme(), mgr.GetClient().RESTMapper(), &corev1.PersistentVolumeClaim{}, handler.OnlyControllerOwner()))); err != nil {
		return err
	}

	return nil
}

func createScratchPvcNameFromPvc(pvc *corev1.PersistentVolumeClaim, isCloneTarget bool) string {
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
	return naming.GetResourceName(common.UploadPodName, pvc.Name)
}

// createUploadResourceName returns the name given to upload resources
func createUploadResourceName(name string) string {
	return naming.GetResourceName(common.UploadPodName, name)
}

// UploadPossibleForPVC is called by the api server to see whether to return an upload token
func UploadPossibleForPVC(pvc *corev1.PersistentVolumeClaim) error {
	if _, ok := pvc.Annotations[cc.AnnUploadRequest]; !ok {
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

func (r *UploadReconciler) makeUploadPodSpec(args UploadPodArgs, resourceRequirements *corev1.ResourceRequirements, imagePullSecrets []corev1.LocalObjectReference, workloadNodePlacement *sdkapi.NodePlacement) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: args.PVC.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:              common.CDILabelValue,
				common.CDIComponentLabel:        common.UploadServerCDILabel,
				common.UploadServerServiceLabel: naming.GetServiceNameFromResourceName(args.Name),
				common.UploadTargetLabel:        string(args.PVC.UID),
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePVCOwnerReference(args.PVC),
			},
		},
		Spec: corev1.PodSpec{
			Containers:        r.makeUploadPodContainers(args, resourceRequirements),
			Volumes:           r.makeUploadPodVolumes(args),
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			NodeSelector:      workloadNodePlacement.NodeSelector,
			Tolerations:       workloadNodePlacement.Tolerations,
			Affinity:          workloadNodePlacement.Affinity,
			PriorityClassName: cc.GetPriorityClass(args.PVC),
			ImagePullSecrets:  imagePullSecrets,
		},
	}

	cc.CopyAllowedAnnotations(args.PVC, pod)
	cc.SetNodeNameIfPopulator(args.PVC, &pod.Spec)
	cc.SetRestrictedSecurityContext(&pod.Spec)

	return pod
}

func (r *UploadReconciler) makeUploadPodContainers(args UploadPodArgs, resourceRequirements *corev1.ResourceRequirements) []corev1.Container {
	requestImageSize, _ := cc.GetRequestedImageSize(args.PVC)
	containers := []corev1.Container{
		{
			Name:            common.UploadServerPodname,
			Image:           r.image,
			ImagePullPolicy: corev1.PullPolicy(r.pullPolicy),
			Env: []corev1.EnvVar{
				{
					Name:  "TLS_KEY_FILE",
					Value: serverKeyFile,
				},
				{
					Name:  "TLS_CERT_FILE",
					Value: serverCertFile,
				},
				{
					Name:  "CLIENT_CERT_FILE",
					Value: clientCertFile,
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
				{
					Name:  common.CiphersTLSVar,
					Value: args.CryptoEnvVars.Ciphers,
				},
				{
					Name:  common.MinVersionTLSVar,
					Value: args.CryptoEnvVars.MinTLSVersion,
				},
			},
			Args: []string{"-v=" + r.verbose},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
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
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      certVolName,
					MountPath: certMountPath,
				},
			},
		},
	}
	if args.Deadline != nil {
		containers[0].Env = append(containers[0].Env, corev1.EnvVar{
			Name:  "DEADLINE",
			Value: args.Deadline.Format(time.RFC3339),
		})
	}
	if cc.GetVolumeMode(args.PVC) == corev1.PersistentVolumeBlock {
		containers[0].VolumeDevices = append(containers[0].VolumeDevices, corev1.VolumeDevice{
			Name:       cc.DataVolName,
			DevicePath: common.WriteBlockPath,
		})
		containers[0].Env = append(containers[0].Env, corev1.EnvVar{
			Name:  "DESTINATION",
			Value: common.WriteBlockPath,
		})
	} else {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      cc.DataVolName,
			MountPath: common.UploadServerDataDir,
		})
	}
	if args.ScratchPVCName != "" {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      cc.ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}
	if resourceRequirements != nil {
		containers[0].Resources = *resourceRequirements
	}
	return containers
}

func (r *UploadReconciler) makeUploadPodVolumes(args UploadPodArgs) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: certVolName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: args.Name,
				},
			},
		},
		{
			Name: cc.DataVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: args.PVC.Name,
					ReadOnly:  false,
				},
			},
		},
	}
	if args.ScratchPVCName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: cc.ScratchVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: args.ScratchPVCName,
					ReadOnly:  false,
				},
			},
		})
	}
	return volumes
}
