package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	importControllerAgentName = "import-controller"

	// SourceHTTP is the source type HTTP, if unspecified or invalid, it defaults to SourceHTTP
	SourceHTTP = "http"
	// SourceS3 is the source type S3
	SourceS3 = "s3"
	// SourceGlance is the source type of glance
	SourceGlance = "glance"
	// SourceNone means there is no source.
	SourceNone = "none"
	// SourceRegistry is the source type of Registry
	SourceRegistry = "registry"
	// SourceImageio is the source type ovirt-imageio
	SourceImageio = "imageio"
	// SourceVDDK is the source type of VDDK
	SourceVDDK = "vddk"

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
	// AnnUUID provides a const for our PVC uuid annotation
	AnnUUID = AnnAPIGroup + "/storage.import.uuid"
	// AnnBackingFile provides a const for our PVC backing file annotation
	AnnBackingFile = AnnAPIGroup + "/storage.import.backingFile"
	// AnnThumbprint provides a const for our PVC backing thumbprint annotation
	AnnThumbprint = AnnAPIGroup + "/storage.import.vddk.thumbprint"
	// AnnPreallocationApplied provides a const for PVC preallocation annotation
	AnnPreallocationApplied = AnnAPIGroup + "/storage.preallocation"

	//LabelImportPvc is a pod label used to find the import pod that was created by the relevant PVC
	LabelImportPvc = AnnAPIGroup + "/storage.import.importPvcName"
	//AnnDefaultStorageClass is the annotation indicating that a storage class is the default one.
	AnnDefaultStorageClass = "storageclass.kubernetes.io/is-default-class"

	// ErrImportFailedPVC provides a const to indicate an import to the PVC failed
	ErrImportFailedPVC = "ErrImportFailed"
	// ImportSucceededPVC provides a const to indicate an import to the PVC failed
	ImportSucceededPVC = "ImportSucceeded"

	// creatingScratch provides a const to indicate scratch is being created.
	creatingScratch = "CreatingScratchSpace"

	// ImportTargetInUse is reason for event created when an import pvc is in use
	ImportTargetInUse = "ImportTargetInUse"
)

var (
	vddkInfoMatch = regexp.MustCompile(`((.*; )|^)VDDK: (?P<info>{.*})`)
)

// ImportReconciler members
type ImportReconciler struct {
	client             client.Client
	uncachedClient     client.Client
	recorder           record.EventRecorder
	scheme             *runtime.Scheme
	log                logr.Logger
	image              string
	verbose            string
	pullPolicy         string
	filesystemOverhead string
	featureGates       featuregates.FeatureGates
}

type importPodEnvVar struct {
	ep                 string
	secretName         string
	source             string
	contentType        string
	imageSize          string
	certConfigMap      string
	diskID             string
	uuid               string
	backingFile        string
	thumbprint         string
	filesystemOverhead string
	insecureTLS        bool
	currentCheckpoint  string
	previousCheckpoint string
	finalCheckpoint    string
	preallocation      bool
	httpProxy          string
	httpsProxy         string
	noProxy            string
	certConfigMapProxy string
}

// NewImportController creates a new instance of the import controller.
func NewImportController(mgr manager.Manager, log logr.Logger, importerImage, pullPolicy, verbose string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	client := mgr.GetClient()
	reconciler := &ImportReconciler{
		client:         client,
		uncachedClient: uncachedClient,
		scheme:         mgr.GetScheme(),
		log:            log.WithName("import-controller"),
		image:          importerImage,
		verbose:        verbose,
		pullPolicy:     pullPolicy,
		recorder:       mgr.GetEventRecorderFor("import-controller"),
		featureGates:   featuregates.NewFeatureGates(client),
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

func (r *ImportReconciler) shouldReconcilePVC(pvc *corev1.PersistentVolumeClaim,
	log logr.Logger) (bool, error) {
	_, isImmediateBindingRequested := pvc.Annotations[AnnImmediateBinding]
	waitForFirstConsumerEnabled, err := isWaitForFirstConsumerEnabled(isImmediateBindingRequested, r.featureGates)

	if err != nil {
		return false, err
	}
	return !isPVCComplete(pvc) &&
			(checkPVC(pvc, AnnEndpoint, log) || checkPVC(pvc, AnnSource, log)) &&
			shouldHandlePvc(pvc, waitForFirstConsumerEnabled, log),
		nil
}

func isPVCComplete(pvc *corev1.PersistentVolumeClaim) bool {
	phase, exists := pvc.ObjectMeta.Annotations[AnnPodPhase]
	return exists && (phase == string(corev1.PodSucceeded))
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *ImportReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Import PVCs")

	// Get the PVC.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shouldReconcile, err := r.shouldReconcilePVC(pvc, log)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !shouldReconcile {
		log.V(1).Info("Should not reconcile this PVC",
			"pvc.annotation.phase.complete", isPVCComplete(pvc),
			"pvc.annotations.endpoint", checkPVC(pvc, AnnEndpoint, log),
			"pvc.annotations.source", checkPVC(pvc, AnnSource, log),
			"isBound", isBound(pvc, log))
		return reconcile.Result{}, nil
	}

	// In case this is a request to create a blank disk on a block device, we do not create a pod.
	// we just mark the DV as successful
	volumeMode := getVolumeMode(pvc)
	if volumeMode == corev1.PersistentVolumeBlock && pvc.GetAnnotations()[AnnSource] == SourceNone && pvc.GetAnnotations()[AnnPreallocationRequested] != "true" {
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
	podName := getImportPodNameFromPvc(pvc)
	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: pvc.GetNamespace()}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "error getting import pod %s/%s", pvc.Namespace, podName)
		}
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
			podsUsingPVC, err := GetPodsUsingPVCs(r.client, pvc.Namespace, sets.NewString(pvc.Name), false)
			if err != nil {
				return reconcile.Result{}, err
			}

			if len(podsUsingPVC) > 0 {
				for _, pod := range podsUsingPVC {
					r.log.V(1).Info("can't create import pod, pvc in use by other pod",
						"namespace", pvc.Namespace, "name", pvc.Name, "pod", pod.Name)
					r.recorder.Eventf(pvc, corev1.EventTypeWarning, ImportTargetInUse,
						"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, pvc.Name)

				}
				return reconcile.Result{Requeue: true}, nil
			}

			if _, ok := pvc.Annotations[AnnImportPod]; ok {
				// Create importer pod, make sure the PVC owns it.
				if err := r.createImporterPod(pvc); err != nil {
					return reconcile.Result{}, err
				}
			} else {
				// Create importer pod Name and store in PVC?
				if err := r.initPvcPodName(pvc, log); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		if pvc.DeletionTimestamp != nil {
			log.V(1).Info("PVC being terminated, delete pods", "pod.Name", pod.Name)
			if err := r.client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
				return reconcile.Result{}, err
			}
		} else {
			// Pod exists, we need to update the PVC status.
			if err := r.updatePvcFromPod(pvc, pod, log); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if !isPVCComplete(pvc) {
		// We are not done yet, force a re-reconcile in 2 seconds to get an update.
		log.V(1).Info("Force Reconcile pvc import not finished", "pvc.Name", pvc.Name)

		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ImportReconciler) initPvcPodName(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Init pod name on PVC")
	anno := pvc.GetAnnotations()

	anno[AnnImportPod] = createImportPodNameFromPvc(pvc)

	requiresScratch := r.requiresScratchSpace(pvc)
	if requiresScratch {
		anno[AnnRequiresScratch] = "true"
	}

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		if err := r.updatePVC(pvc, log); err != nil {
			return err
		}
		log.V(1).Info("Updated PVC", "pvc.anno.AnnImportPod", anno[AnnImportPod])
	}
	return nil
}

func (r *ImportReconciler) updatePvcFromPod(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, log logr.Logger) error {
	// Keep a copy of the original for comparison later.
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Updating PVC from pod")
	anno := pvc.GetAnnotations()
	setConditionFromPodWithPrefix(anno, AnnRunningCondition, pod)

	scratchExitCode := false
	if pod.Status.ContainerStatuses != nil &&
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
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
	if pod.Status.ContainerStatuses != nil &&
		pod.Status.ContainerStatuses[0].State.Terminated != nil &&
		pod.Status.ContainerStatuses[0].State.Terminated.ExitCode == 0 {
	}
	if pod.Status.ContainerStatuses != nil &&
		pod.Status.ContainerStatuses[0].State.Terminated != nil {
		if getSource(pvc) == SourceVDDK {
			r.saveVddkAnnotations(pvc, pod, log)
		}
	}

	if anno[AnnCurrentCheckpoint] != "" {
		anno[AnnCurrentPodID] = string(pod.ObjectMeta.UID)
	}

	if pod.Status.ContainerStatuses != nil {
		anno[AnnPodRestarts] = strconv.Itoa(int(pod.Status.ContainerStatuses[0].RestartCount))
	}

	anno[AnnImportPod] = string(pod.Name)
	if !scratchExitCode {
		// No scratch exit code, update the phase based on the pod. If we do have scratch exit code we don't want to update the
		// phase, because the pod might terminate cleanly and mistakenly mark the import complete.
		anno[AnnPodPhase] = string(pod.Status.Phase)
	}

	// Check if the POD is waiting for scratch space, if so create some.
	if pod.Status.Phase == corev1.PodPending && r.requiresScratchSpace(pvc) {
		if err := r.createScratchPvcForPod(pvc, pod); err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return err
			}
		}
	} else {
		// No scratch space, or scratch space is bound, remove annotation
		delete(anno, AnnBoundCondition)
		delete(anno, AnnBoundConditionMessage)
		delete(anno, AnnBoundConditionReason)
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
		log.V(1).Info("Updated PVC", "pvc.anno.Phase", anno[AnnPodPhase], "pvc.anno.Restarts", anno[AnnPodRestarts])
	}

	if isPVCComplete(pvc) || scratchExitCode {
		if !scratchExitCode {
			r.recorder.Event(pvc, corev1.EventTypeNormal, ImportSucceededPVC, "Import Successful")
			log.V(1).Info("Completed successfully, deleting POD", "pod.Name", pod.Name)
		}
		if err := r.client.Delete(context.TODO(), pod); IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (r *ImportReconciler) saveVddkAnnotations(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, log logr.Logger) {
	terminationMessage := pod.Status.ContainerStatuses[0].State.Terminated.Message
	log.V(1).Info("Saving VDDK annotations from pod status message: ", "message", terminationMessage)

	var terminationInfo string
	matches := vddkInfoMatch.FindAllStringSubmatch(terminationMessage, -1)
	for index, matchName := range vddkInfoMatch.SubexpNames() {
		if matchName == "info" && len(matches) > 0 {
			terminationInfo = matches[0][index]
			break
		}
	}

	var vddkInfo util.VddkInfo
	err := json.Unmarshal([]byte(terminationInfo), &vddkInfo)
	if err == nil {
		if vddkInfo.Host != "" {
			pvc.Annotations[AnnVddkHostConnection] = vddkInfo.Host
		}
		if vddkInfo.Version != "" {
			pvc.Annotations[AnnVddkVersion] = vddkInfo.Version
		}
	}
}

func (r *ImportReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	log.V(1).Info("Annotations are now", "pvc.anno", pvc.GetAnnotations())
	if err := r.client.Update(context.TODO(), pvc); err != nil {
		return err
	}
	return nil
}

func (r *ImportReconciler) createImporterPod(pvc *corev1.PersistentVolumeClaim) error {
	r.log.V(1).Info("Creating importer POD for PVC", "pvc.Name", pvc.Name)
	var scratchPvcName *string
	var vddkImageName *string
	var err error

	requiresScratch := r.requiresScratchSpace(pvc)
	if requiresScratch {
		name := createScratchNameFromPvc(pvc)
		scratchPvcName = &name
	}

	if getSource(pvc) == SourceVDDK {
		r.log.V(1).Info("Pod requires VDDK sidecar for VMware transfer")
		vddkImageName, err = r.getVddkImageName()
		if err != nil {
			anno := pvc.GetAnnotations()
			anno[AnnBoundCondition] = "false"
			anno[AnnBoundConditionMessage] = fmt.Sprintf("waiting for %s configmap for VDDK image", common.VddkConfigMap)
			anno[AnnBoundConditionReason] = common.AwaitingVDDK
			if updateErr := r.updatePVC(pvc, r.log); updateErr != nil {
				return updateErr
			}
			return err
		}
	}

	podEnvVar, err := r.createImportEnvVar(pvc)
	if err != nil {
		return err
	}
	// all checks passed, let's create the importer pod!
	pod, err := createImporterPod(r.log, r.client, r.image, r.verbose, r.pullPolicy, podEnvVar, pvc, scratchPvcName, vddkImageName, getPriorityClass(pvc))

	if err != nil {
		return err
	}
	r.log.V(1).Info("Created POD", "pod.Name", pod.Name)
	if requiresScratch {
		r.log.V(1).Info("Pod requires scratch space")
		return r.createScratchPvcForPod(pvc, pod)
	}

	return nil
}

func (r *ImportReconciler) createImportEnvVar(pvc *corev1.PersistentVolumeClaim) (*importPodEnvVar, error) {
	podEnvVar := &importPodEnvVar{}
	podEnvVar.source = getSource(pvc)
	podEnvVar.contentType = GetContentType(pvc)

	var err error
	if podEnvVar.source != SourceNone {
		podEnvVar.ep, err = getEndpoint(pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.secretName = r.getSecretName(pvc)
		if podEnvVar.secretName == "" {
			r.log.V(2).Info("no secret will be supplied to endpoint", "endPoint", podEnvVar.ep)
		}
		//get the CDIConfig to extract the proxy configuration to be used to import an image
		cdiConfig := &cdiv1.CDIConfig{}
		r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		podEnvVar.certConfigMap, err = r.getCertConfigMap(pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.insecureTLS, err = r.isInsecureTLS(pvc, cdiConfig)
		if err != nil {
			return nil, err
		}
		podEnvVar.diskID = getValueFromAnnotation(pvc, AnnDiskID)
		podEnvVar.backingFile = getValueFromAnnotation(pvc, AnnBackingFile)
		podEnvVar.uuid = getValueFromAnnotation(pvc, AnnUUID)
		podEnvVar.thumbprint = getValueFromAnnotation(pvc, AnnThumbprint)
		podEnvVar.previousCheckpoint = getValueFromAnnotation(pvc, AnnPreviousCheckpoint)
		podEnvVar.currentCheckpoint = getValueFromAnnotation(pvc, AnnCurrentCheckpoint)
		podEnvVar.finalCheckpoint = getValueFromAnnotation(pvc, AnnFinalCheckpoint)

		var field string
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP); err != nil {
			r.log.V(3).Info("no proxy http url will be supplied:", err.Error())
		}
		podEnvVar.httpProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS); err != nil {
			r.log.V(3).Info("no proxy https url will be supplied:", err.Error())
		}
		podEnvVar.httpsProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy); err != nil {
			r.log.V(3).Info("the noProxy field will not be supplied:", err.Error())
		}
		podEnvVar.noProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName); err != nil {
			r.log.V(3).Info("no proxy CA certiticate will be supplied:", err.Error())
		}
		podEnvVar.certConfigMapProxy = field
	}

	fsOverhead, err := GetFilesystemOverhead(r.client, pvc)
	if err != nil {
		return nil, err
	}
	podEnvVar.filesystemOverhead = string(fsOverhead)

	if preallocation, err := strconv.ParseBool(getValueFromAnnotation(pvc, AnnPreallocationRequested)); err == nil {
		podEnvVar.preallocation = preallocation
	} // else use the default "false"

	//get the requested image size.
	podEnvVar.imageSize, err = getRequestedImageSize(pvc)
	if err != nil {
		return nil, err
	}
	return podEnvVar, nil
}

func (r *ImportReconciler) isInsecureTLS(pvc *corev1.PersistentVolumeClaim, cdiConfig *cdiv1.CDIConfig) (bool, error) {
	value, ok := pvc.Annotations[AnnEndpoint]
	if !ok || value == "" {
		return false, nil
	}

	url, err := url.Parse(value)
	if err != nil {
		return false, err
	}

	if url.Scheme != "docker" {
		return false, nil
	}

	for _, value := range cdiConfig.Spec.InsecureRegistries {
		r.log.V(1).Info("Checking host against value", "host", url.Host, "value", value)
		if value == url.Host {
			return true, nil
		}
	}

	// ConfigMap is obsoleted and supported only for upgrade. It won't be refered anymore by future releases.
	configMapName := common.InsecureRegistryConfigMap
	r.log.V(1).Info("Checking configmap for host", "configMapName", configMapName, "host URL", url.Host)

	cm := &corev1.ConfigMap{}
	if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: util.GetNamespace()}, cm); err != nil {
		if k8serrors.IsNotFound(err) {
			r.log.V(1).Info("Configmap does not exist", "configMapName", configMapName)
			return false, nil
		}
		return false, err
	}

	for _, value := range cm.Data {
		r.log.V(1).Info("Checking host against value", "host", url.Host, "value", value)
		if value == url.Host {
			return true, nil
		}
	}

	return false, nil
}

func (r *ImportReconciler) getCertConfigMap(pvc *corev1.PersistentVolumeClaim) (string, error) {
	value, ok := pvc.Annotations[AnnCertConfigMap]
	if !ok || value == "" {
		return "", nil
	}

	configMap := &corev1.ConfigMap{}
	if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: value, Namespace: pvc.Namespace}, configMap); err != nil {
		if k8serrors.IsNotFound(err) {
			r.log.V(1).Info("Configmap does not exist, pod will not start until it does", "configMapName", value)
			return value, nil
		}

		return "", err
	}

	return value, nil
}

// returns the name of the secret containing endpoint credentials consumed by the importer pod.
// A value of "" implies there are no credentials for the endpoint being used. A returned error
// causes processNextItem() to stop.
func (r *ImportReconciler) getSecretName(pvc *corev1.PersistentVolumeClaim) string {
	ns := pvc.Namespace
	name, found := pvc.Annotations[AnnSecret]
	if !found || name == "" {
		msg := "getEndpointSecret: "
		if !found {
			msg += fmt.Sprintf("annotation %q is missing in pvc \"%s/%s\"", AnnSecret, ns, pvc.Name)
		} else {
			msg += fmt.Sprintf("secret name is missing from annotation %q in pvc \"%s/%s\"", AnnSecret, ns, pvc.Name)
		}
		r.log.V(2).Info(msg)
		return "" // importer pod will not contain secret credentials
	}
	return name
}

func (r *ImportReconciler) requiresScratchSpace(pvc *corev1.PersistentVolumeClaim) bool {
	scratchRequired := false
	contentType := GetContentType(pvc)
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
	scratchPVCName, exists := getScratchNameFromPod(pod)
	if !exists {
		return errors.New("Scratch Volume not configured for pod")
	}
	anno := pvc.GetAnnotations()
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: pvc.GetNamespace(), Name: scratchPVCName}, scratchPvc)
	if IgnoreNotFound(err) != nil {
		return err
	}
	if k8serrors.IsNotFound(err) {
		r.log.V(1).Info("Creating scratch space for POD and PVC", "pod.Name", pod.Name, "pvc.Name", pvc.Name)

		storageClassName := GetScratchPvcStorageClass(r.client, pvc)
		// Scratch PVC doesn't exist yet, create it. Determine which storage class to use.
		_, err = CreateScratchPersistentVolumeClaim(r.client, pvc, pod, scratchPVCName, storageClassName)
		if err != nil {
			return err
		}
		anno[AnnBoundCondition] = "false"
		anno[AnnBoundConditionMessage] = "Creating scratch space"
		anno[AnnBoundConditionReason] = creatingScratch
	} else {
		setBoundConditionFromPVC(anno, AnnBoundCondition, scratchPvc)
	}
	return nil
}

// Get path to VDDK image from 'v2v-vmware' ConfigMap
func (r *ImportReconciler) getVddkImageName() (*string, error) {
	namespace := util.GetNamespace()

	cm := &corev1.ConfigMap{}
	err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: common.VddkConfigMap, Namespace: namespace}, cm)
	if k8serrors.IsNotFound(err) {
		return nil, errors.Errorf("No %s ConfigMap present in namespace %s", common.VddkConfigMap, namespace)
	}

	image, found := cm.Data[common.VddkConfigDataKey]
	if found {
		msg := fmt.Sprintf("Found %s ConfigMap in namespace %s, VDDK image path is: ", common.VddkConfigMap, namespace)
		r.log.V(1).Info(msg, common.VddkConfigDataKey, image)
		return &image, nil
	}

	return nil, errors.Errorf("Found %s ConfigMap in namespace %s, but it does not contain a '%s' entry.", common.VddkConfigMap, namespace, common.VddkConfigDataKey)
}

// returns the source string which determines the type of source. If no source or invalid source found, default to http
func getSource(pvc *corev1.PersistentVolumeClaim) string {
	source, found := pvc.Annotations[AnnSource]
	if !found {
		source = ""
	}
	switch source {
	case
		SourceHTTP,
		SourceS3,
		SourceGlance,
		SourceNone,
		SourceRegistry,
		SourceImageio,
		SourceVDDK:
	default:
		source = SourceHTTP
	}
	return source
}

// GetContentType returns the content type of the source image. If invalid or not set, default to kubevirt
func GetContentType(pvc *corev1.PersistentVolumeClaim) string {
	contentType, found := pvc.Annotations[AnnContentType]
	if !found {
		return string(cdiv1.DataVolumeKubeVirt)
	}
	switch contentType {
	case
		string(cdiv1.DataVolumeKubeVirt),
		string(cdiv1.DataVolumeArchive):
	default:
		contentType = string(cdiv1.DataVolumeKubeVirt)
	}
	return contentType
}

// returns the endpoint string which contains the full path URI of the target object to be copied.
func getEndpoint(pvc *corev1.PersistentVolumeClaim) (string, error) {
	ep, found := pvc.Annotations[AnnEndpoint]
	if !found || ep == "" {
		verb := "empty"
		if !found {
			verb = "missing"
		}
		return ep, errors.Errorf("annotation %q in pvc \"%s/%s\" is %s\n", AnnEndpoint, pvc.Namespace, pvc.Name, verb)
	}
	return ep, nil
}

// getValueFromAnnotation returns the value of an annotation
func getValueFromAnnotation(pvc *corev1.PersistentVolumeClaim, annotation string) string {
	value, _ := pvc.Annotations[annotation]
	return value
}

func getImportPodNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	podName, ok := pvc.Annotations[AnnImportPod]
	if ok {
		return podName
	}
	// fallback to legacy naming, in fact the following function is fully compatible with legacy
	// name concatenation "importer-{pvc.Name}" if the name length is under the size limits,
	return naming.GetResourceName(common.ImporterPodName, pvc.Name)
}

func createImportPodNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	return naming.GetResourceName(common.ImporterPodName, pvc.Name)
}

// createImporterPod creates and returns a pointer to a pod which is created based on the passed-in endpoint, secret
// name, and pvc. A nil secret means the endpoint credentials are not passed to the
// importer pod.
func createImporterPod(log logr.Logger, client client.Client, image, verbose, pullPolicy string, podEnvVar *importPodEnvVar, pvc *corev1.PersistentVolumeClaim, scratchPvcName *string, vddkImageName *string, priorityClassName string) (*corev1.Pod, error) {
	podResourceRequirements, err := GetDefaultPodResourceRequirements(client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := GetWorkloadNodePlacement(client)
	if err != nil {
		return nil, err
	}

	pod := makeImporterPodSpec(pvc.Namespace, image, verbose, pullPolicy, podEnvVar, pvc, scratchPvcName, podResourceRequirements, workloadNodePlacement, vddkImageName, priorityClassName)

	if err := client.Create(context.TODO(), pod); err != nil {
		return nil, err
	}
	log.V(3).Info("importer pod created\n", "pod.Name", pod.Name, "pod.Namespace", pod.Namespace, "image name", image)
	return pod, nil
}

// makeImporterPodSpec creates and return the importer pod spec based on the passed-in endpoint, secret and pvc.
func makeImporterPodSpec(namespace, image, verbose, pullPolicy string, podEnvVar *importPodEnvVar, pvc *corev1.PersistentVolumeClaim, scratchPvcName *string, podResourceRequirements *corev1.ResourceRequirements, workloadNodePlacement *sdkapi.NodePlacement, vddkImageName *string, priorityClassName string) *corev1.Pod {
	// importer pod name contains the pvc name
	podName, _ := pvc.Annotations[AnnImportPod]

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
				common.PrometheusLabel:   "",
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
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			Volumes:           volumes,
			NodeSelector:      workloadNodePlacement.NodeSelector,
			Tolerations:       workloadNodePlacement.Tolerations,
			Affinity:          workloadNodePlacement.Affinity,
			PriorityClassName: priorityClassName,
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

	if vddkImageName != nil {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "vddk-vol-mount",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:  "vddk-side-car",
			Image: *vddkImageName,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vddk-vol-mount",
					MountPath: "/opt",
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "vddk-vol-mount",
			MountPath: "/opt",
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

	if podEnvVar.certConfigMapProxy != "" {
		vm := corev1.VolumeMount{
			Name:      ProxyCertVolName,
			MountPath: common.ImporterProxyCertDir,
		}
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, vm)
		pod.Spec.Volumes = append(pod.Spec.Volumes, createProxyConfigMapVolume(CertVolName, podEnvVar.certConfigMapProxy))
	}

	if podEnvVar.contentType == string(cdiv1.DataVolumeKubeVirt) {
		// Set the fsGroup on the security context to the QemuSubGid
		if pod.Spec.SecurityContext == nil {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}
		fsGroup := common.QemuSubGid
		pod.Spec.SecurityContext.FSGroup = &fsGroup
	}
	SetPodPvcAnnotations(pod, pvc)
	return pod
}

func createProxyConfigMapVolume(certVolName, objRef string) corev1.Volume {
	return corev1.Volume{
		Name: CertVolName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: objRef,
				},
			},
		},
	}
}

// this is being called for pods using PV with filesystem volume mode
func addImportVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      DataVolName,
			MountPath: common.ImporterDataDir,
		},
	}
	return volumeMounts
}

// return the Env portion for the importer container.
func makeImportEnv(podEnvVar *importPodEnvVar, uid types.UID) []corev1.EnvVar {
	env := []corev1.EnvVar{
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
			Name:  common.FilesystemOverheadVar,
			Value: podEnvVar.filesystemOverhead,
		},
		{
			Name:  common.InsecureTLSVar,
			Value: strconv.FormatBool(podEnvVar.insecureTLS),
		},
		{
			Name:  common.ImporterDiskID,
			Value: podEnvVar.diskID,
		},
		{
			Name:  common.ImporterUUID,
			Value: podEnvVar.uuid,
		},
		{
			Name:  common.ImporterBackingFile,
			Value: podEnvVar.backingFile,
		},
		{
			Name:  common.ImporterThumbprint,
			Value: podEnvVar.thumbprint,
		},
		{
			Name:  common.ImportProxyHTTP,
			Value: podEnvVar.httpProxy,
		},
		{
			Name:  common.ImportProxyHTTPS,
			Value: podEnvVar.httpsProxy,
		},
		{
			Name:  common.ImportProxyNoProxy,
			Value: podEnvVar.noProxy,
		},
		{
			Name:  common.ImporterCurrentCheckpoint,
			Value: podEnvVar.currentCheckpoint,
		},
		{
			Name:  common.ImporterPreviousCheckpoint,
			Value: podEnvVar.previousCheckpoint,
		},
		{
			Name:  common.ImporterFinalCheckpoint,
			Value: podEnvVar.finalCheckpoint,
		},
		{
			Name:  common.Preallocation,
			Value: strconv.FormatBool(podEnvVar.preallocation),
		},
	}
	if podEnvVar.secretName != "" {
		env = append(env, corev1.EnvVar{
			Name: common.ImporterAccessKeyID,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeyAccess,
				},
			},
		}, corev1.EnvVar{
			Name: common.ImporterSecretKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeySecret,
				},
			},
		})

	}
	if podEnvVar.certConfigMap != "" {
		env = append(env, corev1.EnvVar{
			Name:  common.ImporterCertDirVar,
			Value: common.ImporterCertDir,
		})
	}
	if podEnvVar.certConfigMapProxy != "" {
		env = append(env, corev1.EnvVar{
			Name:  common.ImporterProxyCertDirVar,
			Value: common.ImporterProxyCertDir,
		})
	}
	return env
}
