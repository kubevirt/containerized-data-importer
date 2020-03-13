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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	// AnnUploadRequest marks that a PVC should be made available for upload
	AnnUploadRequest = "cdi.kubevirt.io/storage.upload.target"

	// AnnUploadClientName is the TLS name uploadserver will accept requests from
	AnnUploadClientName = "cdi.kubevirt.io/uploadClientName"

	annCreatedByUpload = "cdi.kubevirt.io/storage.createdByUploadController"

	uploadServerClientName = "client.upload-server.cdi.kubevirt.io"

	uploadServerCertDuration = 365 * 24 * time.Hour

	// UploadSucceededPVC provides a const to indicate an import to the PVC failed
	UploadSucceededPVC = "UploadSucceeded"
)

// UploadReconciler members
type UploadReconciler struct {
	Client                 client.Client
	CdiClient              cdiclientset.Interface
	K8sClient              kubernetes.Interface
	recorder               record.EventRecorder
	Scheme                 *runtime.Scheme
	Log                    logr.Logger
	Image                  string
	Verbose                string
	PullPolicy             string
	UploadProxyServiceName string
	serverCertGenerator    generator.CertGenerator
	clientCAFetcher        fetcher.CertBundleFetcher
}

// UploadPodArgs are the parameters required to create an upload pod
type UploadPodArgs struct {
	Name                            string
	PVC                             *v1.PersistentVolumeClaim
	ScratchPVCName                  string
	ClientName                      string
	ServerCert, ServerKey, ClientCA []byte
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *UploadReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Upload PVCs")

	// Get the PVC.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
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

	// force cleanup if PVC pending delete and pod running or the upload/clone annotation was removed
	if (!isUpload && !isCloneTarget) || podSucceededFromPVC(pvc) || pvc.DeletionTimestamp != nil {
		log.V(1).Info("not doing anything with PVC", "isUpload", isUpload, "isCloneTarget", isCloneTarget, "podSucceededFromPVC",
			podSucceededFromPVC(pvc), "deletionTimeStamp set?", pvc.DeletionTimestamp != nil)
		if err := r.cleanup(pvc); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	log.Info("Calling Upload reconcile PVC")
	return r.reconcilePVC(log, pvc, isCloneTarget)
}

func (r *UploadReconciler) reconcilePVC(log logr.Logger, pvc *corev1.PersistentVolumeClaim, isCloneTarget bool) (reconcile.Result, error) {
	var uploadClientName, scratchPVCName string
	pvcCopy := pvc.DeepCopy()

	if isCloneTarget {
		source, err := r.getCloneRequestSourcePVC(pvc)
		if err != nil {
			return reconcile.Result{}, err
		}

		if err = ValidateCanCloneSourceAndTargetSpec(&source.Spec, &pvc.Spec); err != nil {
			log.Error(err, "Error validating clone spec, ignoring")
			return reconcile.Result{}, nil
		}

		uploadClientName = fmt.Sprintf("%s/%s-%s/%s", source.Namespace, source.Name, pvc.Namespace, pvc.Name)
		pvcCopy.Annotations[AnnUploadClientName] = uploadClientName
	} else {
		uploadClientName = uploadServerClientName

		// TODO revisit naming, could overflow
		scratchPVCName = pvc.Name + "-scratch"
	}

	resourceName := getUploadResourceName(pvc.Name)

	pod, err := r.getOrCreateUploadPod(pvc, resourceName, scratchPVCName, uploadClientName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if _, err = r.getOrCreateUploadService(pvc, resourceName); err != nil {
		return reconcile.Result{}, err
	}

	podPhase := pod.Status.Phase
	pvcCopy.Annotations[AnnPodPhase] = string(podPhase)
	pvcCopy.Annotations[AnnPodReady] = strconv.FormatBool(isPodReady(pod))

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

func (r *UploadReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim) error {
	r.Log.V(1).Info("Phase is now", "pvc.anno.Phase", pvc.GetAnnotations()[AnnPodPhase])
	if err := r.Client.Update(context.TODO(), pvc); err != nil {
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
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sourcePvc); err != nil {
		return nil, errors.Wrap(err, "error getting clone source PVC")
	}
	if sourcePvc.Spec.VolumeMode != nil {
		sourceVolumeMode = *sourcePvc.Spec.VolumeMode
	}
	if targetPvc.Spec.VolumeMode != nil {
		targetVolumeMode = *targetPvc.Spec.VolumeMode
	}
	if sourceVolumeMode != targetVolumeMode {
		return nil, errors.New("Source and target volume Modes do not match")
	}
	return sourcePvc, nil
}

func (r *UploadReconciler) cleanup(pvc *v1.PersistentVolumeClaim) error {
	resourceName := getUploadResourceName(pvc.Name)

	// delete service
	if err := r.deleteService(pvc.Namespace, resourceName); err != nil {
		return err
	}

	// delete pod
	pod := &corev1.Pod{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: resourceName, Namespace: pvc.Namespace}, pod); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if pod.DeletionTimestamp == nil {
		if err := r.Client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (r *UploadReconciler) getOrCreateUploadPod(pvc *v1.PersistentVolumeClaim, podName, scratchPVCName, clientName string) (*v1.Pod, error) {
	pod := &corev1.Pod{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: pvc.Namespace}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "error getting upload pod %s/%s", pvc.Namespace, podName)
		}

		serverCert, serverKey, err := r.serverCertGenerator.MakeServerCert(pvc.Namespace, podName, uploadServerCertDuration)
		if err != nil {
			return nil, err
		}

		clientCA, err := r.clientCAFetcher.BundleBytes()
		if err != nil {
			return nil, err
		}

		args := UploadPodArgs{
			Name:           podName,
			PVC:            pvc,
			ScratchPVCName: scratchPVCName,
			ClientName:     clientName,
			ServerCert:     serverCert,
			ServerKey:      serverKey,
			ClientCA:       clientCA,
		}

		r.Log.V(3).Info("Creating upload pod")
		pod, err = r.createUploadPod(args)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("%s pod not controlled by pvc %s", podName, pvc.Name)
	}

	// Always try to get or create the scratch PVC for a pod that is not successful yet, if it exists nothing happens otherwise attempt to create.
	if scratchPVCName != "" {
		_, err := r.getOrCreateScratchPvc(pvc, pod, scratchPVCName)
		if err != nil {
			return nil, err
		}
	}

	return pod, nil
}

func (r *UploadReconciler) getOrCreateScratchPvc(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, name string) (*v1.PersistentVolumeClaim, error) {
	scratchPvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: pvc.Namespace}, scratchPvc); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "error getting scratch PVC")
		}

		storageClassName := GetScratchPvcStorageClass(r.K8sClient, r.CdiClient, pvc)

		// Scratch PVC doesn't exist yet, create it.
		scratchPvc, err = CreateScratchPersistentVolumeClaim(r.K8sClient, pvc, pod, name, storageClassName)
		if err != nil {
			return nil, err
		}
	}

	if !metav1.IsControlledBy(scratchPvc, pod) {
		return nil, errors.Errorf("%s scratch PVC not controlled by pod %s", scratchPvc.Name, pod.Name)
	}

	return scratchPvc, nil
}

func (r *UploadReconciler) getOrCreateUploadService(pvc *v1.PersistentVolumeClaim, name string) (*v1.Service, error) {
	service := &corev1.Service{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: pvc.Namespace}, service); err != nil {
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
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: namespace}, service); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if service.DeletionTimestamp == nil {
		if err := r.Client.Delete(context.TODO(), service); IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "error deleting upload service")
		}
	}

	return nil
}

// createUploadService creates upload service service manifest and sends to server
func (r *UploadReconciler) createUploadService(name string, pvc *v1.PersistentVolumeClaim) (*v1.Service, error) {
	ns := pvc.Namespace
	service := r.makeUploadServiceSpec(name, pvc)

	if err := r.Client.Create(context.TODO(), service); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, service); err != nil {
				return nil, errors.Wrap(err, "upload service should exist but couldn't retrieve it")
			}
		} else {
			return nil, errors.Wrap(err, "upload service API create errored")
		}
	}
	r.Log.V(1).Info("upload service created\n", "Namespace", service.Namespace, "Name", service.Name)
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

	podResourceRequirements, err := GetDefaultPodResourceRequirements(r.Client)
	if err != nil {
		return nil, err
	}

	pod := r.makeUploadPodSpec(args, podResourceRequirements)

	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: args.Name, Namespace: ns}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "upload pod should exist but couldn't retrieve it")
		}
		if err := r.Client.Create(context.TODO(), pod); err != nil {
			return nil, err
		}
	}

	r.Log.V(1).Info("upload pod created\n", "Namespace", pod.Namespace, "Name", pod.Name, "Image name", r.Image)
	return pod, nil
}

// NewUploadController creates a new instance of the upload controller.
func NewUploadController(mgr manager.Manager, cdiClient *cdiclientset.Clientset, k8sClient kubernetes.Interface, log logr.Logger, uploadImage, pullPolicy, verbose string, serverCertGenerator generator.CertGenerator, clientCAFetcher fetcher.CertBundleFetcher) (controller.Controller, error) {
	reconciler := &UploadReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		CdiClient:           cdiClient,
		K8sClient:           k8sClient,
		Log:                 log.WithName("upload-controller"),
		Image:               uploadImage,
		Verbose:             verbose,
		PullPolicy:          pullPolicy,
		recorder:            mgr.GetEventRecorderFor("upload-controller"),
		serverCertGenerator: serverCertGenerator,
		clientCAFetcher:     clientCAFetcher,
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

func addUploadControllerWatches(mgr manager.Manager, importController controller.Controller) error {
	// Setup watches
	if err := importController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := importController.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &corev1.PersistentVolumeClaim{},
		IsController: true,
	}); err != nil {
		return err
	}
	if err := importController.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &corev1.PersistentVolumeClaim{},
		IsController: true,
	}); err != nil {
		return err
	}

	return nil
}

// getUploadResourceName returns the name given to upload resources
func getUploadResourceName(name string) string {
	// TODO revisit naming, could overflow
	return "cdi-upload-" + name
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
	return fmt.Sprintf("https://%s.%s.svc%s", getUploadResourceName(pvc), namespace, uploadPath)
}

func (r *UploadReconciler) makeUploadPodSpec(args UploadPodArgs, resourceRequirements *v1.ResourceRequirements) *v1.Pod {
	requestImageSize, _ := getRequestedImageSize(args.PVC)
	fsGroup := common.QemuSubGid
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Name,
			Namespace: args.PVC.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:              common.CDILabelValue,
				common.CDIComponentLabel:        common.UploadServerCDILabel,
				common.UploadServerServiceLabel: args.Name,
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
					Image:           r.Image,
					ImagePullPolicy: v1.PullPolicy(r.PullPolicy),
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
							Name:  common.UploadImageSize,
							Value: requestImageSize,
						},
						{
							Name:  "CLIENT_NAME",
							Value: args.ClientName,
						},
					},
					Args: []string{"-v=" + r.Verbose},
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
		},
	}

	if !checkPVC(args.PVC, AnnCloneRequest) {
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

	return pod
}
