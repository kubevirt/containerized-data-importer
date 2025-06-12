package controller

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

const (
	// ErrImportFailedPVC provides a const to indicate an import to the PVC failed
	ErrImportFailedPVC = "ErrImportFailed"
	// ImportSucceededPVC provides a const to indicate an import to the PVC failed
	ImportSucceededPVC = "ImportSucceeded"

	// creatingScratch provides a const to indicate scratch is being created.
	creatingScratch = "CreatingScratchSpace"

	// ImportTargetInUse is reason for event created when an import pvc is in use
	ImportTargetInUse = "ImportTargetInUse"

	// importPodImageStreamFinalizer ensures image stream import pod is deleted when pvc is deleted,
	// as in this case pod has no pvc OwnerReference
	importPodImageStreamFinalizer = "cdi.kubevirt.io/importImageStream"

	// secretExtraHeadersVolumeName is the format string that specifies where extra HTTP header secrets will be mounted
	secretExtraHeadersVolumeName = "cdi-secret-extra-headers-vol-%d"
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
	filesystemOverhead string //nolint:unused // TODO: check if need to remove this field
	cdiNamespace       string
	featureGates       featuregates.FeatureGates
	installerLabels    map[string]string
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
	pullMethod         string
	readyFile          string
	doneFile           string
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
	extraHeaders       []string
	secretExtraHeaders []string
	cacheMode          string
}

type importerPodArgs struct {
	image                   string
	importImage             string
	verbose                 string
	pullPolicy              string
	podEnvVar               *importPodEnvVar
	pvc                     *corev1.PersistentVolumeClaim
	scratchPvcName          *string
	podResourceRequirements *corev1.ResourceRequirements
	imagePullSecrets        []corev1.LocalObjectReference
	workloadNodePlacement   *sdkapi.NodePlacement
	vddkImageName           *string
	vddkExtraArgs           *string
	priorityClassName       string
}

// NewImportController creates a new instance of the import controller.
func NewImportController(mgr manager.Manager, log logr.Logger, importerImage, pullPolicy, verbose string, installerLabels map[string]string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	client := mgr.GetClient()
	reconciler := &ImportReconciler{
		client:          client,
		uncachedClient:  uncachedClient,
		scheme:          mgr.GetScheme(),
		log:             log.WithName("import-controller"),
		image:           importerImage,
		verbose:         verbose,
		pullPolicy:      pullPolicy,
		recorder:        mgr.GetEventRecorderFor("import-controller"),
		cdiNamespace:    util.GetNamespace(),
		featureGates:    featuregates.NewFeatureGates(client),
		installerLabels: installerLabels,
	}
	importController, err := controller.New("import-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
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
	if err := importController.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolumeClaim]{})); err != nil {
		return err
	}
	if err := importController.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, handler.TypedEnqueueRequestForOwner[*corev1.Pod](
		mgr.GetScheme(), mgr.GetClient().RESTMapper(), &corev1.PersistentVolumeClaim{}, handler.OnlyControllerOwner()))); err != nil {
		return err
	}

	return nil
}

func (r *ImportReconciler) shouldReconcilePVC(pvc *corev1.PersistentVolumeClaim,
	log logr.Logger) (bool, error) {
	_, pvcUsesExternalPopulator := pvc.Annotations[cc.AnnExternalPopulation]
	if pvcUsesExternalPopulator {
		return false, nil
	}

	waitForFirstConsumerEnabled, err := cc.IsWaitForFirstConsumerEnabled(pvc, r.featureGates)
	if err != nil {
		return false, err
	}

	return (!cc.IsPVCComplete(pvc) || cc.IsMultiStageImportInProgress(pvc)) &&
			(checkPVC(pvc, cc.AnnEndpoint, log) || checkPVC(pvc, cc.AnnSource, log)) &&
			shouldHandlePvc(pvc, waitForFirstConsumerEnabled, log),
		nil
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
		multiStageImport := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCurrentCheckpoint)
		multiStageAlreadyDone := metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnMultiStageImportDone)

		log.V(3).Info("Should not reconcile this PVC",
			"pvc.annotation.phase.complete", cc.IsPVCComplete(pvc),
			"pvc.annotations.endpoint", checkPVC(pvc, cc.AnnEndpoint, log),
			"pvc.annotations.source", checkPVC(pvc, cc.AnnSource, log),
			"isBound", isBound(pvc, log), "isMultistage", multiStageImport, "multiStageDone", multiStageAlreadyDone)
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
	if !metav1.IsControlledBy(pod, pvc) && !cc.IsImageStream(pvc) {
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

	r.updatePVCBoundContion(pvc)

	if pod == nil {
		if cc.IsPVCComplete(pvc) {
			// Don't create the POD if the PVC is completed already
			log.V(1).Info("PVC is already complete")
		} else if pvc.DeletionTimestamp == nil {
			podsUsingPVC, err := cc.GetPodsUsingPVCs(context.TODO(), r.client, pvc.Namespace, sets.New(pvc.Name), false)
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

			if _, ok := pvc.Annotations[cc.AnnImportPod]; ok {
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
			if err := r.cleanup(pvc, pod, log); err != nil {
				return reconcile.Result{}, err
			}
		} else {
			// Copy import proxy ConfigMap (if exists) from cdi namespace to the import namespace
			if err := r.copyImportProxyConfigMap(pvc, pod); err != nil {
				return reconcile.Result{}, err
			}
			// Pod exists, we need to update the PVC status.
			if err := r.updatePvcFromPod(pvc, pod, log); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if !cc.IsPVCComplete(pvc) {
		// We are not done yet, force a re-reconcile in 2 seconds to get an update.
		log.V(1).Info("Force Reconcile pvc import not finished", "pvc.Name", pvc.Name)

		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ImportReconciler) copyImportProxyConfigMap(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod) error {
	cdiConfig := &cdiv1.CDIConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig); err != nil {
		return err
	}
	cmName, err := GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName)
	if err != nil || cmName == "" {
		return nil
	}
	cdiConfigMap := &corev1.ConfigMap{}
	if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: r.cdiNamespace}, cdiConfigMap); err != nil {
		return err
	}
	importConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetImportProxyConfigMapName(pvc.Name),
			Namespace: pvc.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         pod.APIVersion,
				Kind:               pod.Kind,
				Name:               pod.Name,
				UID:                pod.UID,
				BlockOwnerDeletion: ptr.To[bool](true),
				Controller:         ptr.To[bool](true),
			}},
		},
		Data: cdiConfigMap.Data,
	}
	if err := r.client.Create(context.TODO(), importConfigMap); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// GetImportProxyConfigMapName returns the import proxy ConfigMap name
func GetImportProxyConfigMapName(pvcName string) string {
	return naming.GetResourceName("import-proxy-cm", pvcName)
}

func (r *ImportReconciler) initPvcPodName(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Init pod name on PVC")
	anno := pvc.GetAnnotations()

	anno[cc.AnnImportPod] = createImportPodNameFromPvc(pvc)

	requiresScratch := r.requiresScratchSpace(pvc)
	if requiresScratch {
		anno[cc.AnnRequiresScratch] = "true"
	}

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		if err := r.updatePVC(pvc, log); err != nil {
			return err
		}
		log.V(1).Info("Updated PVC", "pvc.anno.AnnImportPod", anno[cc.AnnImportPod])
	}
	return nil
}

func (r *ImportReconciler) updatePvcFromPod(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, log logr.Logger) error {
	// Keep a copy of the original for comparison later.
	currentPvcCopy := pvc.DeepCopyObject()

	log.V(1).Info("Updating PVC from pod")
	anno := pvc.GetAnnotations()

	termMsg, err := parseTerminationMessage(pod)
	if err != nil {
		log.V(3).Info("Ignoring failure to parse termination message", "error", err.Error())
	}
	setAnnotationsFromPodWithPrefix(anno, pod, termMsg, cc.AnnRunningCondition)

	scratchSpaceRequired := termMsg != nil && termMsg.ScratchSpaceRequired != nil && *termMsg.ScratchSpaceRequired
	if scratchSpaceRequired {
		log.V(1).Info("Pod requires scratch space, terminating pod, and restarting with scratch space", "pod.Name", pod.Name)
	}
	podModificationsNeeded := scratchSpaceRequired

	if statuses := pod.Status.ContainerStatuses; len(statuses) > 0 {
		if isOOMKilled(statuses[0]) {
			log.V(1).Info("Pod died of an OOM, deleting pod, and restarting with qemu cache mode=none if storage supports it", "pod.Name", pod.Name)
			podModificationsNeeded = true
			anno[cc.AnnRequiresDirectIO] = "true"
		}
		if terminated := statuses[0].State.Terminated; terminated != nil && terminated.ExitCode > 0 {
			log.Info("Pod termination code", "pod.Name", pod.Name, "ExitCode", terminated.ExitCode)
			r.recorder.Event(pvc, corev1.EventTypeWarning, ErrImportFailedPVC, terminated.Message)
		}
	}

	if anno[cc.AnnCurrentCheckpoint] != "" {
		anno[cc.AnnCurrentPodID] = string(pod.ObjectMeta.UID)
	}

	anno[cc.AnnImportPod] = pod.Name
	if !podModificationsNeeded {
		// No scratch space required, update the phase based on the pod. If we require scratch space we don't want to update the
		// phase, because the pod might terminate cleanly and mistakenly mark the import complete.
		anno[cc.AnnPodPhase] = string(pod.Status.Phase)
	}

	for _, ev := range pod.Spec.Containers[0].Env {
		if ev.Name == common.CacheMode && ev.Value == common.CacheModeTryNone {
			anno[cc.AnnRequiresDirectIO] = "false"
		}
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
		delete(anno, cc.AnnBoundCondition)
		delete(anno, cc.AnnBoundConditionMessage)
		delete(anno, cc.AnnBoundConditionReason)
	}

	if pvc.GetLabels() == nil {
		pvc.SetLabels(make(map[string]string, 0))
	}
	if !checkIfLabelExists(pvc, common.CDILabelKey, common.CDILabelValue) {
		pvc.GetLabels()[common.CDILabelKey] = common.CDILabelValue
	}
	if cc.IsPVCComplete(pvc) {
		pvc.SetLabels(addLabelsFromTerminationMessage(pvc.GetLabels(), termMsg))
	}

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		if err := r.updatePVC(pvc, log); err != nil {
			return err
		}
		log.V(1).Info("Updated PVC", "pvc.anno.Phase", anno[cc.AnnPodPhase], "pvc.anno.Restarts", anno[cc.AnnPodRestarts])
	}

	if cc.IsPVCComplete(pvc) || podModificationsNeeded {
		if !podModificationsNeeded {
			r.recorder.Event(pvc, corev1.EventTypeNormal, ImportSucceededPVC, "Import Successful")
			log.V(1).Info("Import completed successfully")
		}
		if cc.ShouldDeletePod(pvc) {
			log.V(1).Info("Deleting pod", "pod.Name", pod.Name)
			if err := r.cleanup(pvc, pod, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ImportReconciler) cleanup(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, log logr.Logger) error {
	if err := r.client.Delete(context.TODO(), pod); cc.IgnoreNotFound(err) != nil {
		return err
	}
	if cc.HasFinalizer(pvc, importPodImageStreamFinalizer) {
		cc.RemoveFinalizer(pvc, importPodImageStreamFinalizer)
		if err := r.updatePVC(pvc, log); err != nil {
			return err
		}
	}
	return nil
}

func (r *ImportReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	if err := r.client.Update(context.TODO(), pvc); err != nil {
		return err
	}
	return nil
}

func (r *ImportReconciler) createImporterPod(pvc *corev1.PersistentVolumeClaim) error {
	r.log.V(1).Info("Creating importer POD for PVC", "pvc.Name", pvc.Name)
	var scratchPvcName *string
	var vddkImageName *string
	var vddkExtraArgs *string
	var err error

	requiresScratch := r.requiresScratchSpace(pvc)
	if requiresScratch {
		name := createScratchNameFromPvc(pvc)
		scratchPvcName = &name
	}

	if cc.GetSource(pvc) == cc.SourceVDDK {
		r.log.V(1).Info("Pod requires VDDK sidecar for VMware transfer")
		anno := pvc.GetAnnotations()
		if imageName, ok := anno[cc.AnnVddkInitImageURL]; ok {
			vddkImageName = &imageName
		} else {
			if vddkImageName, err = r.getVddkImageName(); err != nil {
				r.log.V(1).Error(err, "failed to get VDDK image name from configmap")
			}
		}
		if vddkImageName == nil {
			message := fmt.Sprintf("waiting for %s configmap or %s annotation for VDDK image", common.VddkConfigMap, cc.AnnVddkInitImageURL)
			anno[cc.AnnBoundCondition] = "false"
			anno[cc.AnnBoundConditionMessage] = message
			anno[cc.AnnBoundConditionReason] = common.AwaitingVDDK
			if err := r.updatePVC(pvc, r.log); err != nil {
				return err
			}
			return errors.New(message)
		}

		if extraArgs, ok := anno[cc.AnnVddkExtraArgs]; ok && extraArgs != "" {
			r.log.V(1).Info("Mounting extra VDDK args ConfigMap to importer pod", "ConfigMap", extraArgs)
			vddkExtraArgs = &extraArgs
		}
	}

	podEnvVar, err := r.createImportEnvVar(pvc)
	if err != nil {
		return err
	}
	// all checks passed, let's create the importer pod!
	podArgs := &importerPodArgs{
		image:             r.image,
		verbose:           r.verbose,
		pullPolicy:        r.pullPolicy,
		podEnvVar:         podEnvVar,
		pvc:               pvc,
		scratchPvcName:    scratchPvcName,
		vddkImageName:     vddkImageName,
		vddkExtraArgs:     vddkExtraArgs,
		priorityClassName: cc.GetPriorityClass(pvc),
	}

	pod, err := createImporterPod(context.TODO(), r.log, r.client, podArgs, r.installerLabels)
	// Check if pod has failed and, in that case, record an event with the error
	if podErr := cc.HandleFailedPod(err, pvc.Annotations[cc.AnnImportPod], pvc, r.recorder, r.client); podErr != nil {
		return podErr
	}

	r.log.V(1).Info("Created POD", "pod.Name", pod.Name)

	// If importing from image stream, add finalizer. Note we don't watch the importer pod in this case,
	// so to prevent a deadlock we add finalizer only if the pod is not retained after completion.
	if cc.IsImageStream(pvc) && pvc.GetAnnotations()[cc.AnnPodRetainAfterCompletion] != "true" {
		cc.AddFinalizer(pvc, importPodImageStreamFinalizer)
		if err := r.updatePVC(pvc, r.log); err != nil {
			return err
		}
	}

	if requiresScratch {
		r.log.V(1).Info("Pod requires scratch space")
		return r.createScratchPvcForPod(pvc, pod)
	}

	return nil
}

func createScratchNameFromPvc(pvc *v1.PersistentVolumeClaim) string {
	return naming.GetResourceName(pvc.Name, common.ScratchNameSuffix)
}

func (r *ImportReconciler) createImportEnvVar(pvc *corev1.PersistentVolumeClaim) (*importPodEnvVar, error) {
	podEnvVar := &importPodEnvVar{}
	podEnvVar.source = cc.GetSource(pvc)
	podEnvVar.contentType = string(cc.GetPVCContentType(pvc))

	var err error
	if podEnvVar.source != cc.SourceNone {
		podEnvVar.ep, err = cc.GetEndpoint(pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.secretName = r.getSecretName(pvc)
		if podEnvVar.secretName == "" {
			r.log.V(2).Info("no secret will be supplied to endpoint", "endPoint", podEnvVar.ep)
		}
		//get the CDIConfig to extract the proxy configuration to be used to import an image
		cdiConfig := &cdiv1.CDIConfig{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		if err != nil {
			return nil, err
		}
		podEnvVar.certConfigMap, err = r.getCertConfigMap(pvc)
		if err != nil {
			return nil, err
		}
		podEnvVar.insecureTLS, err = r.isInsecureTLS(pvc, cdiConfig)
		if err != nil {
			return nil, err
		}
		podEnvVar.diskID = getValueFromAnnotation(pvc, cc.AnnDiskID)
		podEnvVar.backingFile = getValueFromAnnotation(pvc, cc.AnnBackingFile)
		podEnvVar.uuid = getValueFromAnnotation(pvc, cc.AnnUUID)
		podEnvVar.thumbprint = getValueFromAnnotation(pvc, cc.AnnThumbprint)
		podEnvVar.previousCheckpoint = getValueFromAnnotation(pvc, cc.AnnPreviousCheckpoint)
		podEnvVar.currentCheckpoint = getValueFromAnnotation(pvc, cc.AnnCurrentCheckpoint)
		podEnvVar.finalCheckpoint = getValueFromAnnotation(pvc, cc.AnnFinalCheckpoint)

		for annotation, value := range pvc.Annotations {
			if strings.HasPrefix(annotation, cc.AnnExtraHeaders) {
				podEnvVar.extraHeaders = append(podEnvVar.extraHeaders, value)
			}
			if strings.HasPrefix(annotation, cc.AnnSecretExtraHeaders) {
				podEnvVar.secretExtraHeaders = append(podEnvVar.secretExtraHeaders, value)
			}
		}

		var field string
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTP); err != nil {
			r.log.V(3).Info("no proxy http url will be supplied:", "error", err.Error())
		}
		podEnvVar.httpProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyHTTPS); err != nil {
			r.log.V(3).Info("no proxy https url will be supplied:", "error", err.Error())
		}
		podEnvVar.httpsProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyNoProxy); err != nil {
			r.log.V(3).Info("the noProxy field will not be supplied:", "error", err.Error())
		}
		podEnvVar.noProxy = field
		if field, err = GetImportProxyConfig(cdiConfig, common.ImportProxyConfigMapName); err != nil {
			r.log.V(3).Info("no proxy CA certiticate will be supplied:", "error", err.Error())
		}
		podEnvVar.certConfigMapProxy = field
	}

	fsOverhead, err := GetFilesystemOverhead(context.TODO(), r.client, pvc)
	if err != nil {
		return nil, err
	}
	podEnvVar.filesystemOverhead = string(fsOverhead)

	if preallocation, err := strconv.ParseBool(getValueFromAnnotation(pvc, cc.AnnPreallocationRequested)); err == nil {
		podEnvVar.preallocation = preallocation
	} // else use the default "false"

	//get the requested image size.
	podEnvVar.imageSize, err = cc.GetRequestedImageSize(pvc)
	if err != nil {
		return nil, err
	}

	if v, ok := pvc.Annotations[cc.AnnRequiresDirectIO]; ok && v == "true" {
		podEnvVar.cacheMode = common.CacheModeTryNone
	}

	return podEnvVar, nil
}

func (r *ImportReconciler) isInsecureTLS(pvc *corev1.PersistentVolumeClaim, cdiConfig *cdiv1.CDIConfig) (bool, error) {
	ep, ok := pvc.Annotations[cc.AnnEndpoint]
	if !ok || ep == "" {
		return false, nil
	}
	return IsInsecureTLS(ep, cdiConfig, r.log)
}

// IsInsecureTLS checks if TLS security is disabled for the given endpoint
func IsInsecureTLS(ep string, cdiConfig *cdiv1.CDIConfig, log logr.Logger) (bool, error) {
	url, err := url.Parse(ep)
	if err != nil {
		return false, err
	}

	if url.Scheme != "docker" {
		return false, nil
	}

	for _, value := range cdiConfig.Spec.InsecureRegistries {
		log.V(1).Info("Checking host against value", "host", url.Host, "value", value)
		if value == url.Host {
			return true, nil
		}
	}
	return false, nil
}

func (r *ImportReconciler) getCertConfigMap(pvc *corev1.PersistentVolumeClaim) (string, error) {
	value, ok := pvc.Annotations[cc.AnnCertConfigMap]
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
	name, found := pvc.Annotations[cc.AnnSecret]
	if !found || name == "" {
		msg := "getEndpointSecret: "
		if !found {
			msg += fmt.Sprintf("annotation %q is missing in pvc \"%s/%s\"", cc.AnnSecret, ns, pvc.Name)
		} else {
			msg += fmt.Sprintf("secret name is missing from annotation %q in pvc \"%s/%s\"", cc.AnnSecret, ns, pvc.Name)
		}
		r.log.V(2).Info(msg)
		return "" // importer pod will not contain secret credentials
	}
	return name
}

func (r *ImportReconciler) requiresScratchSpace(pvc *corev1.PersistentVolumeClaim) bool {
	scratchRequired := false
	contentType := cc.GetPVCContentType(pvc)
	// All archive requires scratch space.
	if contentType == cdiv1.DataVolumeArchive {
		scratchRequired = true
	} else {
		switch cc.GetSource(pvc) {
		case cc.SourceGlance:
			scratchRequired = true
		case cc.SourceImageio:
			if val, ok := pvc.Annotations[cc.AnnCurrentCheckpoint]; ok {
				scratchRequired = val != ""
			}
		case cc.SourceRegistry:
			scratchRequired = pvc.Annotations[cc.AnnRegistryImportMethod] != string(cdiv1.RegistryPullNode)
		}
	}
	value, ok := pvc.Annotations[cc.AnnRequiresScratch]
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
	if cc.IgnoreNotFound(err) != nil {
		return err
	}
	if k8serrors.IsNotFound(err) {
		r.log.V(1).Info("Creating scratch space for POD and PVC", "pod.Name", pod.Name, "pvc.Name", pvc.Name)

		storageClassName := GetScratchPvcStorageClass(r.client, pvc)
		// Scratch PVC doesn't exist yet, create it. Determine which storage class to use.
		_, err = createScratchPersistentVolumeClaim(r.client, pvc, pod, scratchPVCName, storageClassName, r.installerLabels, r.recorder)
		if err != nil {
			return err
		}
		anno[cc.AnnBoundCondition] = "false"
		anno[cc.AnnBoundConditionMessage] = "Creating scratch space"
		anno[cc.AnnBoundConditionReason] = creatingScratch
	} else {
		if scratchPvc.DeletionTimestamp != nil {
			// Delete the pod since we are in a deadlock situation now. The scratch PVC from the previous import is not gone
			// yet but terminating, and the new pod is still being created and the scratch PVC now has a finalizer on it.
			// Only way to break it, is to delete the importer pod, and give the pvc a chance to disappear.
			err = r.client.Delete(context.TODO(), pod)
			if err != nil {
				return err
			}
			return fmt.Errorf("terminating scratch space found, deleting pod %s", pod.Name)
		}
		setBoundConditionFromPVC(anno, cc.AnnBoundCondition, scratchPvc)
	}
	anno[cc.AnnRequiresScratch] = "false"
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

	return nil, errors.Errorf("found %s ConfigMap in namespace %s, but it does not contain a '%s' entry", common.VddkConfigMap, namespace, common.VddkConfigDataKey)
}

func (r *ImportReconciler) updatePVCBoundContion(pvc *corev1.PersistentVolumeClaim) {
	// set bound condition by getting the latest event
	events := &corev1.EventList{}

	err := r.client.List(context.TODO(), events,
		client.InNamespace(pvc.GetNamespace()),
		client.MatchingFields{"involvedObject.name": pvc.GetName(),
			"involvedObject.uid": string(pvc.GetUID())},
	)

	if err != nil || len(events.Items) == 0 {
		return
	}

	// Sort event lists by most recent
	sort.Slice(events.Items, func(i, j int) bool {
		return events.Items[i].FirstTimestamp.Time.After(events.Items[j].FirstTimestamp.Time)
	})

	boundMessage := ""

	pvcPrime, exists := pvc.GetAnnotations()[cc.AnnPVCPrimeName]
	if exists {
		// if we are using populators get the latest event from prime pvc
		pvcPrime = fmt.Sprintf("[%s] : ", pvcPrime)
		for _, event := range events.Items {
			if strings.Contains(event.Message, pvcPrime) {
				// split so we can remove prime name prefix from event message
				res := strings.Split(event.Message, pvcPrime)
				boundMessage = res[len(res)-1]
			}
		}
		if boundMessage == "" {
			return
		}
	} else {
		// if not using populators just get the latest event
		boundMessage = events.Items[0].Message
	}

	anno := pvc.GetAnnotations()

	if pvc.Status.Phase == corev1.ClaimBound {
		anno[cc.AnnBoundCondition] = "true"
		anno[cc.AnnBoundConditionReason] = "Bound"
	} else {
		anno[cc.AnnBoundCondition] = "false"
		anno[cc.AnnBoundConditionReason] = "Pending"
	}
	anno[cc.AnnBoundConditionMessage] = boundMessage
}

// returns the import image part of the endpoint string
func getRegistryImportImage(pvc *corev1.PersistentVolumeClaim) (string, error) {
	ep, err := cc.GetEndpoint(pvc)
	if err != nil {
		return "", nil
	}
	if cc.IsImageStream(pvc) {
		return ep, nil
	}
	url, err := url.Parse(ep)
	if err != nil {
		return "", errors.Errorf("illegal registry endpoint %s", ep)
	}
	return url.Host + url.Path, nil
}

// getValueFromAnnotation returns the value of an annotation
func getValueFromAnnotation(pvc *corev1.PersistentVolumeClaim, annotation string) string {
	return pvc.Annotations[annotation]
}

// If this pod is going to transfer one checkpoint in a multi-stage import, attach the checkpoint name to the pod name so
// that each checkpoint gets a unique pod. That way each pod can be inspected using the retainAfterCompletion annotation.
func podNameWithCheckpoint(pvc *corev1.PersistentVolumeClaim) string {
	if checkpoint := pvc.Annotations[cc.AnnCurrentCheckpoint]; checkpoint != "" {
		return pvc.Name + "-checkpoint-" + checkpoint
	}
	return pvc.Name
}

func getImportPodNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	podName, ok := pvc.Annotations[cc.AnnImportPod]
	if ok {
		return podName
	}
	// fallback to legacy naming, in fact the following function is fully compatible with legacy
	// name concatenation "importer-{pvc.Name}" if the name length is under the size limits,
	return naming.GetResourceName(common.ImporterPodName, podNameWithCheckpoint(pvc))
}

func createImportPodNameFromPvc(pvc *corev1.PersistentVolumeClaim) string {
	return naming.GetResourceName(common.ImporterPodName, podNameWithCheckpoint(pvc))
}

// createImporterPod creates and returns a pointer to a pod which is created based on the passed-in endpoint, secret
// name, and pvc. A nil secret means the endpoint credentials are not passed to the
// importer pod.
func createImporterPod(ctx context.Context, log logr.Logger, client client.Client, args *importerPodArgs, installerLabels map[string]string) (*corev1.Pod, error) {
	var err error
	args.podResourceRequirements, err = cc.GetDefaultPodResourceRequirements(client)
	if err != nil {
		return nil, err
	}

	args.imagePullSecrets, err = cc.GetImagePullSecrets(client)
	if err != nil {
		return nil, err
	}

	args.workloadNodePlacement, err = cc.GetWorkloadNodePlacement(ctx, client)
	if err != nil {
		return nil, err
	}

	if isRegistryNodeImport(args) {
		args.importImage, err = getRegistryImportImage(args.pvc)
		if err != nil {
			return nil, err
		}
		setRegistryNodeImportEnvVars(args)
	}

	pod := makeImporterPodSpec(args)

	util.SetRecommendedLabels(pod, installerLabels, "cdi-controller")

	// add any labels from pvc to the importer pod
	util.MergeLabels(args.pvc.Labels, pod.Labels)

	if err = client.Create(context.TODO(), pod); err != nil {
		return nil, err
	}

	log.V(3).Info("importer pod created\n", "pod.Name", pod.Name, "pod.Namespace", pod.Namespace, "image name", args.image)
	return pod, nil
}

// makeImporterPodSpec creates and return the importer pod spec based on the passed-in endpoint, secret and pvc.
func makeImporterPodSpec(args *importerPodArgs) *corev1.Pod {
	// importer pod name contains the pvc name
	podName := args.pvc.Annotations[cc.AnnImportPod]

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: args.pvc.Namespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:        common.CDILabelValue,
				common.CDIComponentLabel:  common.ImporterPodName,
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               args.pvc.Name,
					UID:                args.pvc.GetUID(),
					BlockOwnerDeletion: ptr.To[bool](true),
					Controller:         ptr.To[bool](true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers:        makeImporterContainerSpec(args),
			InitContainers:    makeImporterInitContainersSpec(args),
			Volumes:           makeImporterVolumeSpec(args),
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			NodeSelector:      args.workloadNodePlacement.NodeSelector,
			Tolerations:       args.workloadNodePlacement.Tolerations,
			Affinity:          args.workloadNodePlacement.Affinity,
			PriorityClassName: args.priorityClassName,
			ImagePullSecrets:  args.imagePullSecrets,
		},
	}

	/**
	FIXME: When registry source is ImageStream, if we set importer pod OwnerReference (to its pvc, like all other cases),
	for some reason (OCP issue?) we get the following error:
		Failed to pull image "imagestream-name": rpc error: code = Unknown
		desc = Error reading manifest latest in docker.io/library/imagestream-name: errors:
		denied: requested access to the resource is denied
		unauthorized: authentication required
	When we don't set pod OwnerReferences, all works well.
	*/
	if isRegistryNodeImport(args) && cc.IsImageStream(args.pvc) {
		pod.OwnerReferences = nil
		pod.Annotations[cc.AnnOpenShiftImageLookup] = "*"
	}

	cc.CopyAllowedAnnotations(args.pvc, pod)
	cc.SetRestrictedSecurityContext(&pod.Spec)
	// We explicitly define a NodeName for dynamically provisioned PVCs
	// when the PVC is being handled by a populator (PVC')
	cc.SetNodeNameIfPopulator(args.pvc, &pod.Spec)

	return pod
}

func makeImporterContainerSpec(args *importerPodArgs) []corev1.Container {
	containers := []corev1.Container{
		{
			Name:            common.ImporterPodName,
			Image:           args.image,
			ImagePullPolicy: corev1.PullPolicy(args.pullPolicy),
			Args:            []string{"-v=" + args.verbose},
			Env:             makeImportEnv(args.podEnvVar, getOwnerUID(args)),
			Ports: []corev1.ContainerPort{
				{
					Name:          "metrics",
					ContainerPort: 8443,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		},
	}
	if cc.GetVolumeMode(args.pvc) == corev1.PersistentVolumeBlock {
		containers[0].VolumeDevices = cc.AddVolumeDevices()
	} else {
		containers[0].VolumeMounts = cc.AddImportVolumeMounts()
	}
	if isRegistryNodeImport(args) {
		containers = append(containers, corev1.Container{
			Name:            "server",
			Image:           args.importImage,
			ImagePullPolicy: corev1.PullPolicy(args.pullPolicy),
			Command:         []string{"/shared/server", "-p", "8100", "-image-dir", "/disk", "-ready-file", "/shared/ready", "-done-file", "/shared/done"},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/shared",
					Name:      "shared-volume",
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		})
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			MountPath: "/shared",
			Name:      "shared-volume",
		})
	}
	if args.scratchPvcName != nil {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      cc.ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}
	if args.vddkImageName != nil {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "vddk-vol-mount",
			MountPath: "/opt",
		})
	}
	if args.vddkExtraArgs != nil {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      common.VddkArgsVolName,
			MountPath: common.VddkArgsDir,
		})
	}
	if args.podEnvVar.certConfigMap != "" {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      CertVolName,
			MountPath: common.ImporterCertDir,
		})
	}
	if args.podEnvVar.certConfigMapProxy != "" {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      ProxyCertVolName,
			MountPath: common.ImporterProxyCertDir,
		})
	}
	if args.podEnvVar.source == cc.SourceGCS && args.podEnvVar.secretName != "" {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      SecretVolName,
			MountPath: common.ImporterGoogleCredentialDir,
		})
	}
	for index := range args.podEnvVar.secretExtraHeaders {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf(secretExtraHeadersVolumeName, index),
			MountPath: path.Join(common.ImporterSecretExtraHeadersDir, fmt.Sprint(index)),
		})
	}
	if args.podResourceRequirements != nil {
		for i := range containers {
			containers[i].Resources = *args.podResourceRequirements
		}
	}
	return containers
}

func makeImporterVolumeSpec(args *importerPodArgs) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: cc.DataVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: args.pvc.Name,
					ReadOnly:  false,
				},
			},
		},
	}
	if isRegistryNodeImport(args) {
		volumes = append(volumes, corev1.Volume{
			Name: "shared-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	if args.scratchPvcName != nil {
		volumes = append(volumes, corev1.Volume{
			Name: cc.ScratchVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: *args.scratchPvcName,
					ReadOnly:  false,
				},
			},
		})
	}
	if args.vddkImageName != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "vddk-vol-mount",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	if args.vddkExtraArgs != nil {
		volumes = append(volumes, corev1.Volume{
			Name: common.VddkArgsVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: *args.vddkExtraArgs,
					},
				},
			},
		})
	}
	if args.podEnvVar.certConfigMap != "" {
		volumes = append(volumes, createConfigMapVolume(CertVolName, args.podEnvVar.certConfigMap))
	}
	if args.podEnvVar.certConfigMapProxy != "" {
		volumes = append(volumes, createConfigMapVolume(ProxyCertVolName, GetImportProxyConfigMapName(args.pvc.Name)))
	}
	if args.podEnvVar.source == cc.SourceGCS && args.podEnvVar.secretName != "" {
		volumes = append(volumes, createSecretVolume(SecretVolName, args.podEnvVar.secretName))
	}
	for index, header := range args.podEnvVar.secretExtraHeaders {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf(secretExtraHeadersVolumeName, index),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: header,
				},
			},
		})
	}
	return volumes
}

func makeImporterInitContainersSpec(args *importerPodArgs) []corev1.Container {
	var initContainers []corev1.Container
	if isRegistryNodeImport(args) {
		initContainers = append(initContainers, corev1.Container{
			Name:            "init",
			Image:           args.image,
			ImagePullPolicy: corev1.PullPolicy(args.pullPolicy),
			Command:         []string{"sh", "-c", "cp /usr/bin/cdi-containerimage-server /shared/server"},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/shared",
					Name:      "shared-volume",
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		})
	}
	if args.vddkImageName != nil {
		initContainers = append(initContainers, corev1.Container{
			Name:  "vddk-side-car",
			Image: *args.vddkImageName,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vddk-vol-mount",
					MountPath: "/opt",
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		})
	}
	if args.podResourceRequirements != nil {
		for i := range initContainers {
			initContainers[i].Resources = *args.podResourceRequirements
		}
	}
	return initContainers
}

func isRegistryNodeImport(args *importerPodArgs) bool {
	return cc.GetSource(args.pvc) == cc.SourceRegistry &&
		args.pvc.Annotations[cc.AnnRegistryImportMethod] == string(cdiv1.RegistryPullNode)
}

func getOwnerUID(args *importerPodArgs) types.UID {
	if len(args.pvc.OwnerReferences) == 1 {
		return args.pvc.OwnerReferences[0].UID
	}
	return args.pvc.UID
}

func setRegistryNodeImportEnvVars(args *importerPodArgs) {
	args.podEnvVar.source = cc.SourceHTTP
	args.podEnvVar.ep = "http://localhost:8100/disk.img"
	args.podEnvVar.pullMethod = string(cdiv1.RegistryPullNode)
	args.podEnvVar.readyFile = "/shared/ready"
	args.podEnvVar.doneFile = "/shared/done"
}

func createConfigMapVolume(certVolName, objRef string) corev1.Volume {
	return corev1.Volume{
		Name: certVolName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: objRef,
				},
			},
		},
	}
}

func createSecretVolume(thisVolName, objRef string) corev1.Volume {
	return corev1.Volume{
		Name: thisVolName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: objRef,
			},
		},
	}
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
			Name:  common.ImporterPullMethod,
			Value: podEnvVar.pullMethod,
		},
		{
			Name:  common.ImporterReadyFile,
			Value: podEnvVar.readyFile,
		},
		{
			Name:  common.ImporterDoneFile,
			Value: podEnvVar.doneFile,
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
		{
			Name:  common.CacheMode,
			Value: podEnvVar.cacheMode,
		},
	}
	if podEnvVar.secretName != "" && podEnvVar.source != cc.SourceGCS {
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
	if podEnvVar.secretName != "" && podEnvVar.source == cc.SourceGCS {
		env = append(env, corev1.EnvVar{
			Name:  common.ImporterGoogleCredentialFileVar,
			Value: common.ImporterGoogleCredentialFile,
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
	for index, header := range podEnvVar.extraHeaders {
		env = append(env, corev1.EnvVar{
			Name:  fmt.Sprintf("%s%d", common.ImporterExtraHeader, index),
			Value: header,
		})
	}
	return env
}

func isOOMKilled(status v1.ContainerStatus) bool {
	if terminated := status.State.Terminated; terminated != nil {
		if terminated.Reason == cc.OOMKilledReason {
			return true
		}
	}
	if terminated := status.LastTerminationState.Terminated; terminated != nil {
		if terminated.Reason == cc.OOMKilledReason {
			return true
		}
	}

	return false
}
