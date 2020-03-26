package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	importControllerAgentName = "import-controller"

	// AnnSource provide a const for our PVC import source annotation
	AnnSource = AnnAPIGroup + "/storage.import.source"
	// AnnEndpoint provides a const for our PVC endpoint annotation
	AnnEndpoint = AnnAPIGroup + "/storage.import.endpoint"
	// AnnSecret provides a const for our PVC secretName annotation
	AnnSecret = AnnAPIGroup + "/storage.import.secretName"
	// AnnCertConfigMap is the name of a configmap containing tls certs
	AnnCertConfigMap = AnnAPIGroup + "/storage.import.certConfigMap"
	// AnnContentType provides a const for the PVC content-type
	AnnContentType = AnnAPIGroup + "/storage.contentType"
	// AnnImportPod provides a const for our PVC importPodName annotation
	AnnImportPod = AnnAPIGroup + "/storage.import.importPodName"
	// AnnRequiresScratch provides a const for our PVC requires scratch annotation
	AnnRequiresScratch = AnnAPIGroup + "/storage.import.requiresScratch"
	// AnnDiskID provides a const for our PVC diskId annotation
	AnnDiskID = AnnAPIGroup + "/storage.import.diskId"

	//LabelImportPvc is a pod label used to find the import pod that was created by the relevant PVC
	LabelImportPvc = AnnAPIGroup + "/storage.import.importPvcName"
	//AnnDefaultStorageClass is the annotation indicating that a storage class is the default one.
	AnnDefaultStorageClass = "storageclass.kubernetes.io/is-default-class"

	// ErrImportFailedPVC provides a const to indicate an import to the PVC failed
	ErrImportFailedPVC = "ErrImportFailed"
	// ImportSucceededPVC provides a const to indicate an import to the PVC failed
	ImportSucceededPVC = "ImportSucceeded"
)

// ImportReconciler members
type ImportReconciler struct {
	Client     client.Client
	CdiClient  cdiclientset.Interface
	K8sClient  kubernetes.Interface
	recorder   record.EventRecorder
	Scheme     *runtime.Scheme
	Log        logr.Logger
	Image      string
	Verbose    string
	PullPolicy string
}

type importPodEnvVar struct {
	ep, secretName, source, contentType, imageSize, certConfigMap, diskID string
	insecureTLS                                                           bool
}

// NewImportController creates a new instance of the import controller.
func NewImportController(mgr manager.Manager, cdiClient *cdiclientset.Clientset, k8sClient kubernetes.Interface, log logr.Logger, importerImage, pullPolicy, verbose string) (controller.Controller, error) {
	reconciler := &ImportReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		CdiClient:  cdiClient,
		K8sClient:  k8sClient,
		Log:        log.WithName("import-controller"),
		Image:      importerImage,
		Verbose:    verbose,
		PullPolicy: pullPolicy,
		recorder:   mgr.GetEventRecorderFor("import-controller"),
	}
	importController, err := controller.New("import-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addImportControllerWatches(mgr, importController); err != nil {
		return nil, err
	}
	return importController, nil
}

func addImportControllerWatches(mgr manager.Manager, importController controller.Controller) error {
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

	return nil
}

func shouldReconcilePVC(pvc *corev1.PersistentVolumeClaim) bool {
	return !isPVCComplete(pvc) && (checkPVC(pvc, AnnEndpoint) || checkPVC(pvc, AnnSource))
}

func isPVCComplete(pvc *corev1.PersistentVolumeClaim) bool {
	phase, exists := pvc.ObjectMeta.Annotations[AnnPodPhase]
	return exists && (phase == string(corev1.PodSucceeded))
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *ImportReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Import PVCs")

	// Get the PVC.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !shouldReconcilePVC(pvc) {
		log.V(1).Info("Should not reconcile this PVC", "pvc.annotation.phase.complete", isPVCComplete(pvc),
			"pvc.annotations.endpoint", checkPVC(pvc, AnnEndpoint), "pvc.annotations.source", checkPVC(pvc, AnnSource))
		return reconcile.Result{}, nil
	}

	// In case this is a request to create a blank disk on a block device, we do not create a pod.
	// we just mark the DV as successful
	volumeMode := getVolumeMode(pvc)
	if volumeMode == corev1.PersistentVolumeBlock && pvc.GetAnnotations()[AnnSource] == SourceNone {
		log.V(1).Info("attempting to create blank disk for block mode, this is a no-op, marking pvc with pod-phase succeeded")
		if pvc.GetAnnotations() == nil {
			pvc.SetAnnotations(make(map[string]string, 0))
		}
		pvc.GetAnnotations()[AnnPodPhase] = string(corev1.PodSucceeded)
		if err := r.updatePVC(pvc, log); err != nil {
			return reconcile.Result{}, errors.WithMessage(err, fmt.Sprintf("could not update pvc %q annotation and/or label", pvc.Name))
		}
		return reconcile.Result{}, nil
	}
	return r.reconcilePvc(pvc, log)
}

func (r *ImportReconciler) findImporterPod(pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*corev1.Pod, error) {
	podName := importPodNameFromPvc(pvc)
	pod := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: pvc.GetNamespace()}, pod)

	if k8serrors.IsNotFound(err) {
		return nil, nil
	}

	if !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("Pod is not owned by PVC")
	}

	log.V(1).Info("Pod is owned by PVC", pod.Name, pvc.Name)
	return pod, nil
}

func (r *ImportReconciler) reconcilePvc(pvc *corev1.PersistentVolumeClaim, log logr.Logger) (reconcile.Result, error) {
	// See if we have a pod associated with the PVC, we know the PVC has the needed annotations.
	pod, err := r.findImporterPod(pvc, log)
	if err != nil {
		return reconcile.Result{}, err
	}
	if pod == nil {
		if isPVCComplete(pvc) {
			// Don't create the POD if the PVC is completed already
			log.V(1).Info("PVC is already complete")
		} else if pvc.DeletionTimestamp == nil {
			// Create importer pod, make sure the PVC owns it.
			if err := r.createImporterPod(pvc); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		if pvc.DeletionTimestamp != nil {
			log.V(1).Info("PVC being terminated, delete pods", "pod.Name", pod.Name)
			if err := r.Client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		// Pod exists, we need to update the PVC status.
		if err := r.updatePvcFromPod(pvc, pod, log); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *ImportReconciler) updatePvcFromPod(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, log logr.Logger) error {
	// Keep a copy of the original for comparison later.
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Updating PVC from pod")
	anno := pvc.GetAnnotations()
	scratchExitCode := false
	if pod.Status.ContainerStatuses != nil && pod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode > 0 {
		log.Info("Pod termination code", "pod.Name", pod.Name, "ExitCode", pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode)
		if pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode == common.ScratchSpaceNeededExitCode {
			log.V(1).Info("Pod requires scratch space, terminating pod, and restarting with scratch space", "pod.Name", pod.Name)
			scratchExitCode = true
			anno[AnnRequiresScratch] = "true"
		} else {
			r.recorder.Event(pvc, corev1.EventTypeWarning, ErrImportFailedPVC, pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message)
		}
	}

	anno[AnnImportPod] = string(pod.Name)
	// Even if scratch space is needed, the pod state will still remain running, until the new pod is started.
	anno[AnnPodPhase] = string(pod.Status.Phase)

	// Check if the POD is waiting for scratch space, if so create some.
	if pod.Status.Phase == corev1.PodPending && r.requiresScratchSpace(pvc) {
		if err := r.createScratchPvcForPod(pvc, pod); err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return err
			}
		}
	}
	if !checkIfLabelExists(pvc, common.CDILabelKey, common.CDILabelValue) {
		if pvc.GetLabels() == nil {
			pvc.SetLabels(make(map[string]string, 0))
		}
		pvc.GetLabels()[common.CDILabelKey] = common.CDILabelValue
	}

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		if err := r.updatePVC(pvc, log); err != nil {
			return err
		}
		log.V(1).Info("Updated PVC", "pvc.anno.Phase", anno[AnnPodPhase])
	}

	if isPVCComplete(pvc) || scratchExitCode {
		if !scratchExitCode {
			r.recorder.Event(pvc, corev1.EventTypeNormal, ImportSucceededPVC, "Import Successful")
			log.V(1).Info("Completed successfully, deleting POD", "pod.Name", pod.Name)
		}
		if err := r.Client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (r *ImportReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	log.V(1).Info("Phase is now", "pvc.anno.Phase", pvc.GetAnnotations()[AnnPodPhase])
	if err := r.Client.Update(context.TODO(), pvc); err != nil {
		return err
	}
	return nil
}

func (r *ImportReconciler) createImporterPod(pvc *corev1.PersistentVolumeClaim) error {
	r.Log.V(1).Info("Creating importer POD for PVC", "pvc.Name", pvc.Name)
	var scratchPvcName *string
	var err error

	requiresScratch := r.requiresScratchSpace(pvc)
	if requiresScratch {
		name := scratchNameFromPvc(pvc)
		scratchPvcName = &name
	}

	podEnvVar, err := createImportEnvVar(r.K8sClient, pvc)
	if err != nil {
		return err
	}

	// all checks passed, let's create the importer pod!
	pod, err := createImporterPod(r.Log, r.Client, r.CdiClient, r.Image, r.Verbose, r.PullPolicy, podEnvVar, pvc, scratchPvcName)

	if err != nil {
		return err
	}
	r.Log.V(1).Info("Created POD", "pod.Name", pod.Name)
	if requiresScratch {
		r.Log.V(1).Info("POD requires scratch space")
		return r.createScratchPvcForPod(pvc, pod)
	}
	return nil
}

func (r *ImportReconciler) requiresScratchSpace(pvc *corev1.PersistentVolumeClaim) bool {
	scratchRequired := false
	contentType := getContentType(pvc)
	// All archive requires scratch space.
	if contentType == "archive" {
		scratchRequired = true
	} else {
		switch getSource(pvc) {
		case SourceGlance:
			scratchRequired = true
		case SourceRegistry:
			scratchRequired = true
		}
	}
	value, ok := pvc.Annotations[AnnRequiresScratch]
	if ok {
		boolVal, _ := strconv.ParseBool(value)
		scratchRequired = scratchRequired || boolVal
	}
	return scratchRequired
}

func (r *ImportReconciler) createScratchPvcForPod(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod) error {
	scratchPvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Namespace: pvc.GetNamespace(), Name: scratchNameFromPvc(pvc)}, scratchPvc)
	if IgnoreNotFound(err) != nil {
		return err
	}
	if k8serrors.IsNotFound(err) {
		scratchPVCName := scratchNameFromPvc(pvc)
		storageClassName := GetScratchPvcStorageClass(r.K8sClient, r.CdiClient, pvc)
		// Scratch PVC doesn't exist yet, create it. Determine which storage class to use.
		_, err = CreateScratchPersistentVolumeClaim(r.K8sClient, pvc, pod, scratchPVCName, storageClassName)
		if err != nil {
			return err
		}
	}
	return nil
}

func importPodNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-%s", common.ImporterPodName, pvc.Name)
}

func scratchNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-scratch", pvc.Name)
}

// createImporterPod creates and returns a pointer to a pod which is created based on the passed-in endpoint, secret
// name, and pvc. A nil secret means the endpoint credentials are not passed to the
// importer pod.
func createImporterPod(log logr.Logger, client client.Client, cdiClient cdiclientset.Interface, image, verbose, pullPolicy string, podEnvVar *importPodEnvVar, pvc *corev1.PersistentVolumeClaim, scratchPvcName *string) (*v1.Pod, error) {
	podResourceRequirements, err := GetDefaultPodResourceRequirements(client)
	if err != nil {
		return nil, err
	}

	pod := makeImporterPodSpec(pvc.Namespace, image, verbose, pullPolicy, podEnvVar, pvc, scratchPvcName, podResourceRequirements)

	if err := client.Create(context.TODO(), pod); err != nil {
		return nil, err
	}
	log.V(3).Info("importer pod created\n", "pod.Name", pod.Name, "pod.Namespace", pod.Namespace, "image name", image)
	return pod, nil
}

// makeImporterPodSpec creates and return the importer pod spec based on the passed-in endpoint, secret and pvc.
func makeImporterPodSpec(namespace, image, verbose, pullPolicy string, podEnvVar *importPodEnvVar, pvc *corev1.PersistentVolumeClaim, scratchPvcName *string, podResourceRequirements *v1.ResourceRequirements) *corev1.Pod {
	// importer pod name contains the pvc name
	podName := importPodNameFromPvc(pvc)

	blockOwnerDeletion := true
	isController := true

	volumes := []corev1.Volume{
		{
			Name: DataVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	if scratchPvcName != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ScratchVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: *scratchPvcName,
					ReadOnly:  false,
				},
			},
		})
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.ImporterPodName,
				// this label is used when searching for a pvc's import pod.
				LabelImportPvc:         pvc.Name,
				common.PrometheusLabel: "",
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
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            common.ImporterPodName,
					Image:           image,
					ImagePullPolicy: corev1.PullPolicy(pullPolicy),
					Args:            []string{"-v=" + verbose},
					Ports: []corev1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes:       volumes,
		},
	}

	if podResourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *podResourceRequirements
	}

	ownerUID := pvc.UID
	if len(pvc.OwnerReferences) == 1 {
		ownerUID = pvc.OwnerReferences[0].UID
	}

	if getVolumeMode(pvc) == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &[]int64{0}[0],
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = addImportVolumeMounts()
	}

	if scratchPvcName != nil {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}

	pod.Spec.Containers[0].Env = makeImportEnv(podEnvVar, ownerUID)

	if podEnvVar.certConfigMap != "" {
		vm := corev1.VolumeMount{
			Name:      CertVolName,
			MountPath: common.ImporterCertDir,
		}

		vol := corev1.Volume{
			Name: CertVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: podEnvVar.certConfigMap,
					},
				},
			},
		}

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, vm)
		pod.Spec.Volumes = append(pod.Spec.Volumes, vol)
	}

	if podEnvVar.contentType == string(cdiv1.DataVolumeKubeVirt) {
		// Set the fsGroup on the security context to the QemuSubGid
		if pod.Spec.SecurityContext == nil {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}
		fsGroup := common.QemuSubGid
		pod.Spec.SecurityContext.FSGroup = &fsGroup
	}
	return pod
}

// this is being called for pods using PV with filesystem volume mode
func addImportVolumeMounts() []v1.VolumeMount {
	volumeMounts := []v1.VolumeMount{
		{
			Name:      DataVolName,
			MountPath: common.ImporterDataDir,
		},
	}
	return volumeMounts
}

// return the Env portion for the importer container.
func makeImportEnv(podEnvVar *importPodEnvVar, uid types.UID) []v1.EnvVar {
	env := []v1.EnvVar{
		{
			Name:  common.ImporterSource,
			Value: podEnvVar.source,
		},
		{
			Name:  common.ImporterEndpoint,
			Value: podEnvVar.ep,
		},
		{
			Name:  common.ImporterContentType,
			Value: podEnvVar.contentType,
		},
		{
			Name:  common.ImporterImageSize,
			Value: podEnvVar.imageSize,
		},
		{
			Name:  common.OwnerUID,
			Value: string(uid),
		},
		{
			Name:  common.InsecureTLSVar,
			Value: strconv.FormatBool(podEnvVar.insecureTLS),
		},
		{
			Name:  common.ImporterDiskID,
			Value: podEnvVar.diskID,
		},
	}
	if podEnvVar.secretName != "" {
		env = append(env, v1.EnvVar{
			Name: common.ImporterAccessKeyID,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeyAccess,
				},
			},
		}, v1.EnvVar{
			Name: common.ImporterSecretKey,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeySecret,
				},
			},
		})

	}
	if podEnvVar.certConfigMap != "" {
		env = append(env, v1.EnvVar{
			Name:  common.ImporterCertDirVar,
			Value: common.ImporterCertDir,
		})
	}
	return env
}
