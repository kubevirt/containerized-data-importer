package controller

import (
	"context"
	"crypto/rsa"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/uploadserver"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	cloneControllerAgentName = "clone-controller"

	//AnnCloneRequest sets our expected annotation for a CloneRequest
	AnnCloneRequest = "k8s.io/CloneRequest"
	//AnnCloneOf is used to indicate that cloning was complete
	AnnCloneOf = "k8s.io/CloneOf"
	// AnnCloneToken is the annotation containing the clone token
	AnnCloneToken = "cdi.kubevirt.io/storage.clone.token"

	//CloneUniqueID is used as a special label to be used when we search for the pod
	CloneUniqueID = "cdi.kubevirt.io/storage.clone.cloneUniqeId"
	// AnnCloneSourcePod name of the source clone pod
	AnnCloneSourcePod = "cdi.kubevirt.io/storage.sourceClonePodName"

	// ErrIncompatiblePVC provides a const to indicate a clone is not possible due to an incompatible PVC
	ErrIncompatiblePVC = "ErrIncompatiblePVC"

	// APIServerPublicKeyDir is the path to the apiserver public key dir
	APIServerPublicKeyDir = "/var/run/cdi/apiserver/key"

	// APIServerPublicKeyPath is the path to the apiserver public key
	APIServerPublicKeyPath = APIServerPublicKeyDir + "/id_rsa.pub"

	// CloneSucceededPVC provides a const to indicate a clone to the PVC succeeded
	CloneSucceededPVC = "CloneSucceeded"

	// CloneSourceInUse is reason for event created when clone source pvc is in use
	CloneSourceInUse = "CloneSourceInUse"

	cloneSourcePodFinalizer = "cdi.kubevirt.io/cloneSource"

	cloneTokenLeeway = 10 * time.Second

	uploadClientCertDuration = 365 * 24 * time.Hour

	cloneComplete = "Clone Complete"
)

// CloneReconciler members
type CloneReconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	recorder            record.EventRecorder
	clientCertGenerator generator.CertGenerator
	serverCAFetcher     fetcher.CertBundleFetcher
	log                 logr.Logger
	tokenValidator      token.Validator
	image               string
	verbose             string
	pullPolicy          string
}

// NewCloneController creates a new instance of the config controller.
func NewCloneController(mgr manager.Manager,
	log logr.Logger,
	image, pullPolicy,
	verbose string,
	clientCertGenerator generator.CertGenerator,
	serverCAFetcher fetcher.CertBundleFetcher,
	apiServerKey *rsa.PublicKey) (controller.Controller, error) {
	reconciler := &CloneReconciler{
		client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		log:                 log.WithName("clone-controller"),
		tokenValidator:      newCloneTokenValidator(apiServerKey),
		image:               image,
		verbose:             verbose,
		pullPolicy:          pullPolicy,
		recorder:            mgr.GetEventRecorderFor("clone-controller"),
		clientCertGenerator: clientCertGenerator,
		serverCAFetcher:     serverCAFetcher,
	}
	cloneController, err := controller.New("clone-controller", mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addCloneControllerWatches(mgr, cloneController); err != nil {
		return nil, err
	}
	return cloneController, nil
}

// addConfigControllerWatches sets up the watches used by the config controller.
func addCloneControllerWatches(mgr manager.Manager, cloneController controller.Controller) error {
	// Setup watches
	if err := cloneController.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	if err := cloneController.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		OwnerType:    &corev1.PersistentVolumeClaim{},
		IsController: true,
	}); err != nil {
		return err
	}
	return nil
}

func newCloneTokenValidator(key *rsa.PublicKey) token.Validator {
	return token.NewValidator(common.CloneTokenIssuer, key, cloneTokenLeeway)
}

func (r *CloneReconciler) shouldReconcile(pvc *corev1.PersistentVolumeClaim, log logr.Logger) bool {
	return checkPVC(pvc, AnnCloneRequest, log) &&
		!metav1.HasAnnotation(pvc.ObjectMeta, AnnCloneOf) &&
		isBound(pvc, log)
}

// Reconcile the reconcile loop for host assisted clone pvc.
func (r *CloneReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	// Get the PVC.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Clone PVCs")
	if pvc.DeletionTimestamp != nil || !r.shouldReconcile(pvc, log) {
		log.V(1).Info("Should not reconcile this PVC",
			"checkPVC(AnnCloneRequest)", checkPVC(pvc, AnnCloneRequest, log),
			"NOT has annotation(AnnCloneOf)", !metav1.HasAnnotation(pvc.ObjectMeta, AnnCloneOf),
			"isBound", isBound(pvc, log),
			"has finalizer?", r.hasFinalizer(pvc, cloneSourcePodFinalizer))
		if r.hasFinalizer(pvc, cloneSourcePodFinalizer) {
			// Clone completed, remove source pod and finalizer.
			if err := r.cleanup(pvc, log); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	ready, err := r.waitTargetPodRunningOrSucceeded(pvc, log)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "error ensuring target upload pod running")
	}

	if !ready {
		log.V(3).Info("Upload pod not ready yet for PVC")
		return reconcile.Result{}, nil
	}

	sourcePod, err := r.findCloneSourcePod(pvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	_, nameExists := pvc.Annotations[AnnCloneSourcePod]
	if !nameExists && sourcePod == nil {
		pvc.Annotations[AnnCloneSourcePod] = createCloneSourcePodName(pvc)
		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	if requeue, err := r.reconcileSourcePod(sourcePod, pvc, log); requeue || err != nil {
		return reconcile.Result{Requeue: requeue}, err
	}

	if err := r.updatePvcFromPod(sourcePod, pvc, log); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *CloneReconciler) reconcileSourcePod(sourcePod *corev1.Pod, targetPvc *corev1.PersistentVolumeClaim, log logr.Logger) (bool, error) {
	if sourcePod == nil {
		sourcePvc, err := r.getCloneRequestSourcePVC(targetPvc)
		if err != nil {
			return false, err
		}

		sourcePopulated, err := IsPopulated(sourcePvc, r.client)
		if err != nil {
			return false, err
		}
		if !sourcePopulated {
			return true, nil
		}

		if err := r.validateSourceAndTarget(sourcePvc, targetPvc); err != nil {
			return false, err
		}

		clientName, ok := targetPvc.Annotations[AnnUploadClientName]
		if !ok {
			return false, errors.Errorf("PVC %s/%s missing required %s annotation", targetPvc.Namespace, targetPvc.Name, AnnUploadClientName)
		}

		pods, err := getPodsUsingPVCs(r.client, sourcePvc.Namespace, sets.NewString(sourcePvc.Name), true)
		if err != nil {
			return false, err
		}

		filtered := filterCloneSourcePods(pods)

		if len(filtered) > 0 {
			for _, pod := range filtered {
				r.log.V(1).Info("can't create clone source pod, pvc in use by other pod",
					"namespace", sourcePvc.Namespace, "name", sourcePvc.Name, "pod", pod.Name)
				r.recorder.Eventf(targetPvc, corev1.EventTypeWarning, CloneSourceInUse,
					"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, sourcePvc.Name)
			}
			return true, nil
		}

		sourcePod, err := r.CreateCloneSourcePod(r.image, r.pullPolicy, clientName, targetPvc, log)
		if err != nil {
			return false, err
		}
		log.V(3).Info("Created source pod ", "sourcePod.Namespace", sourcePod.Namespace, "sourcePod.Name", sourcePod.Name)
	}
	return false, nil
}

func (r *CloneReconciler) updatePvcFromPod(sourcePod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()
	log.V(1).Info("Updating PVC from pod")

	pvc = r.addFinalizer(pvc, cloneSourcePodFinalizer)

	log.V(3).Info("Pod phase for PVC", "PVC phase", pvc.Annotations[AnnPodPhase])

	if podSucceededFromPVC(pvc) && pvc.Annotations[AnnCloneOf] != "true" {
		log.V(1).Info("Adding CloneOf annotation to PVC")
		pvc.Annotations[AnnCloneOf] = "true"
		r.recorder.Event(pvc, corev1.EventTypeNormal, CloneSucceededPVC, cloneComplete)
	}
	if sourcePod != nil && sourcePod.Status.ContainerStatuses != nil {
		// update pvc annotation tracking pod restarts only if the source pod restart count is greater
		// see the same in upload-controller
		annPodRestarts, _ := strconv.Atoi(pvc.Annotations[AnnPodRestarts])
		podRestarts := int(sourcePod.Status.ContainerStatuses[0].RestartCount)
		if podRestarts > annPodRestarts {
			pvc.Annotations[AnnPodRestarts] = strconv.Itoa(podRestarts)
		}
		setConditionFromPodWithPrefix(pvc.Annotations, AnnSourceRunningCondition, sourcePod)
	}

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		return r.updatePVC(pvc)
	}
	return nil
}

func (r *CloneReconciler) updatePVC(pvc *corev1.PersistentVolumeClaim) error {
	if err := r.client.Update(context.TODO(), pvc); err != nil {
		return err
	}
	return nil
}

func (r *CloneReconciler) waitTargetPodRunningOrSucceeded(pvc *corev1.PersistentVolumeClaim, log logr.Logger) (bool, error) {
	rs, ok := pvc.Annotations[AnnPodReady]
	if !ok {
		log.V(3).Info("clone target pod not ready")
		return false, nil
	}

	ready, err := strconv.ParseBool(rs)
	if err != nil {
		return false, errors.Wrapf(err, "error parsing %s annotation", AnnPodReady)
	}

	if !ready {
		log.V(3).Info("clone target pod not ready")
		return podSucceededFromPVC(pvc), nil
	}

	return true, nil
}

func (r *CloneReconciler) findCloneSourcePod(pvc *corev1.PersistentVolumeClaim) (*corev1.Pod, error) {
	isCloneRequest, sourceNamespace, _ := ParseCloneRequestAnnotation(pvc)
	if !isCloneRequest {
		return nil, nil
	}
	cloneSourcePodName, exists := pvc.Annotations[AnnCloneSourcePod]
	if !exists {
		// fallback to legacy name, to find any pod that still might be running after upgrade
		cloneSourcePodName = createCloneSourcePodName(pvc)
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			CloneUniqueID: cloneSourcePodName,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating label selector")
	}

	podList := &corev1.PodList{}
	if err := r.client.List(context.TODO(), podList, &client.ListOptions{Namespace: sourceNamespace, LabelSelector: selector}); err != nil {
		return nil, errors.Wrap(err, "error listing pods")
	}

	if len(podList.Items) > 1 {
		return nil, errors.Errorf("multiple source pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
	}

	if len(podList.Items) == 0 {
		return nil, nil
	}

	return &podList.Items[0], nil
}

func (r *CloneReconciler) validateSourceAndTarget(sourcePvc, targetPvc *corev1.PersistentVolumeClaim) error {
	if err := validateCloneToken(r.tokenValidator, sourcePvc, targetPvc); err != nil {
		return err
	}

	err := ValidateCanCloneSourceAndTargetSpec(&sourcePvc.Spec, &targetPvc.Spec)
	if err == nil {
		// Validation complete, put source PVC bound status in annotation
		setBoundConditionFromPVC(targetPvc.GetAnnotations(), AnnBoundCondition, sourcePvc)
		return nil
	}
	return err
}

func (r *CloneReconciler) addFinalizer(pvc *corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	if r.hasFinalizer(pvc, name) {
		return pvc
	}

	pvc.Finalizers = append(pvc.Finalizers, name)
	return pvc
}

func (r *CloneReconciler) removeFinalizer(pvc *corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	if !r.hasFinalizer(pvc, name) {
		return pvc
	}

	var finalizers []string
	for _, f := range pvc.Finalizers {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}

	pvc.Finalizers = finalizers
	return pvc
}

func (r *CloneReconciler) hasFinalizer(object metav1.Object, value string) bool {
	for _, f := range object.GetFinalizers() {
		if f == value {
			return true
		}
	}
	return false
}

// returns the CloneRequest string which contains the pvc name (and namespace) from which we want to clone the image.
func (r *CloneReconciler) getCloneRequestSourcePVC(pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	exists, namespace, name := ParseCloneRequestAnnotation(pvc)
	if !exists {
		return nil, errors.New("error parsing clone request annotation")
	}
	pvc = &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, pvc); err != nil {
		return nil, errors.Wrap(err, "error getting clone source PVC")
	}
	return pvc, nil
}

func (r *CloneReconciler) cleanup(pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	log.V(3).Info("Cleaning up for PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)

	pod, err := r.findCloneSourcePod(pvc)
	if err != nil {
		return err
	}

	if pod != nil && pod.DeletionTimestamp == nil {
		if podSucceededFromPVC(pvc) && pod.Status.Phase == corev1.PodRunning {
			log.V(3).Info("Clone succeeded, waiting for source pod to stop running", "pod.Namespace", pod.Namespace, "pod.Name", pod.Name)
			return nil
		}

		if err = r.client.Delete(context.TODO(), pod); err != nil {
			if !k8serrors.IsNotFound(err) {
				return errors.Wrap(err, "error deleting clone source pod")
			}
		}
	}

	return r.updatePVC(r.removeFinalizer(pvc, cloneSourcePodFinalizer))
}

// CreateCloneSourcePod creates our cloning src pod which will be used for out of band cloning to read the contents of the src PVC
func (r *CloneReconciler) CreateCloneSourcePod(image, pullPolicy, clientName string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*corev1.Pod, error) {
	exists, sourcePvcNamespace, sourcePvcName := ParseCloneRequestAnnotation(pvc)
	if !exists {
		return nil, errors.Errorf("bad CloneRequest Annotation")
	}

	ownerKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cache key")
	}

	clientCert, clientKey, err := r.clientCertGenerator.MakeClientCert(clientName, nil, uploadClientCertDuration)
	if err != nil {
		return nil, err
	}

	serverCABundle, err := r.serverCAFetcher.BundleBytes()
	if err != nil {
		return nil, err
	}

	podResourceRequirements, err := GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := GetWorkloadNodePlacement(r.client)
	if err != nil {
		return nil, err
	}

	pod := MakeCloneSourcePodSpec(image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerKey, clientKey, clientCert, serverCABundle, pvc, podResourceRequirements, workloadNodePlacement)

	if err := r.client.Create(context.TODO(), pod); err != nil {
		return nil, errors.Wrap(err, "source pod API create errored")
	}

	log.V(1).Info("cloning source pod (image) created\n", "pod.Namespace", pod.Namespace, "pod.Name", pod.Name, "image", image)

	return pod, nil
}

func createCloneSourcePodName(targetPvc *corev1.PersistentVolumeClaim) string {
	return string(targetPvc.GetUID()) + common.ClonerSourcePodNameSuffix
}

// MakeCloneSourcePodSpec creates and returns the clone source pod spec based on the target pvc.
func MakeCloneSourcePodSpec(image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerRefAnno string,
	clientKey, clientCert, serverCACert []byte, targetPvc *corev1.PersistentVolumeClaim, resourceRequirements *corev1.ResourceRequirements,
	workloadNodePlacement *sdkapi.NodePlacement) *corev1.Pod {

	var ownerID string
	cloneSourcePodName, _ := targetPvc.Annotations[AnnCloneSourcePod]
	url := GetUploadServerURL(targetPvc.Namespace, targetPvc.Name, uploadserver.UploadPathSync)
	pvcOwner := metav1.GetControllerOf(targetPvc)
	if pvcOwner != nil && pvcOwner.Kind == "DataVolume" {
		ownerID = string(pvcOwner.UID)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloneSourcePodName,
			Namespace: sourcePvcNamespace,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
				AnnOwnerRef:  ownerRefAnno,
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue, //filtered by the podInformer
				common.CDIComponentLabel: common.ClonerSourcePodName,
				// this label is used when searching for a pvc's cloner source pod.
				CloneUniqueID:          cloneSourcePodName,
				common.PrometheusLabel: "",
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []corev1.Container{
				{
					Name:            common.ClonerSourcePodName,
					Image:           image,
					ImagePullPolicy: corev1.PullPolicy(pullPolicy),
					Env: []corev1.EnvVar{
						/*
						 Easier to just stick key/certs in env vars directly no.
						 Maybe revisit when we fix the "naming things" problem.
						*/
						{
							Name:  "CLIENT_KEY",
							Value: string(clientKey),
						},
						{
							Name:  "CLIENT_CERT",
							Value: string(clientCert),
						},
						{
							Name:  "SERVER_CA_CERT",
							Value: string(serverCACert),
						},
						{
							Name:  "UPLOAD_URL",
							Value: url,
						},
						{
							Name:  common.OwnerUID,
							Value: ownerID,
						},
					},
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
			Volumes: []corev1.Volume{
				{
					Name: DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
							// Seems to be problematic with k8s-1.17 provider
							// with SELinux enabled.  Why?  I do not know right now.
							//ReadOnly:  true,
						},
					},
				},
			},
			NodeSelector: workloadNodePlacement.NodeSelector,
			Tolerations:  workloadNodePlacement.Tolerations,
			Affinity:     workloadNodePlacement.Affinity,
		},
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	var volumeMode corev1.PersistentVolumeMode
	var addVars []corev1.EnvVar

	if targetPvc.Spec.VolumeMode != nil {
		volumeMode = *targetPvc.Spec.VolumeMode
	} else {
		volumeMode = corev1.PersistentVolumeFilesystem
	}

	if volumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		addVars = []corev1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "block",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.WriteBlockPath,
			},
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
		addVars = []corev1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "filesystem",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.ClonerMountPath,
			},
		}
	}

	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, addVars...)

	return pod
}

func validateCloneToken(validator token.Validator, source, target *corev1.PersistentVolumeClaim) error {
	tok, ok := target.Annotations[AnnCloneToken]
	if !ok {
		return errors.New("clone token missing")
	}

	tokenData, err := validator.Validate(tok)
	if err != nil {
		return errors.Wrap(err, "error verifying token")
	}

	if tokenData.Operation != token.OperationClone ||
		tokenData.Name != source.Name ||
		tokenData.Namespace != source.Namespace ||
		tokenData.Resource.Resource != "persistentvolumeclaims" ||
		tokenData.Params["targetNamespace"] != target.Namespace ||
		tokenData.Params["targetName"] != target.Name {
		return errors.New("invalid token")
	}

	return nil
}

// ParseCloneRequestAnnotation parses the clone request annotation
func ParseCloneRequestAnnotation(pvc *corev1.PersistentVolumeClaim) (exists bool, namespace, name string) {
	var ann string
	ann, exists = pvc.Annotations[AnnCloneRequest]
	if !exists {
		return
	}

	sp := strings.Split(ann, "/")
	if len(sp) != 2 {
		exists = false
		return
	}

	namespace, name = sp[0], sp[1]
	return
}

// ValidateCanCloneSourceAndTargetSpec validates the specs passed in are compatible for cloning.
func ValidateCanCloneSourceAndTargetSpec(sourceSpec, targetSpec *corev1.PersistentVolumeClaimSpec) error {
	sourceRequest := sourceSpec.Resources.Requests[corev1.ResourceStorage]
	targetRequest := targetSpec.Resources.Requests[corev1.ResourceStorage]
	// Verify that the target PVC size is equal or larger than the source.
	if sourceRequest.Value() > targetRequest.Value() {
		return errors.New("target resources requests storage size is smaller than the source")
	}
	// Verify that the source and target volume modes are the same.
	sourceVolumeMode := corev1.PersistentVolumeFilesystem
	if sourceSpec.VolumeMode != nil && *sourceSpec.VolumeMode == corev1.PersistentVolumeBlock {
		sourceVolumeMode = corev1.PersistentVolumeBlock
	}
	targetVolumeMode := corev1.PersistentVolumeFilesystem
	if targetSpec.VolumeMode != nil && *targetSpec.VolumeMode == corev1.PersistentVolumeBlock {
		targetVolumeMode = corev1.PersistentVolumeBlock
	}
	if sourceVolumeMode != targetVolumeMode {
		return fmt.Errorf("source volumeMode (%s) and target volumeMode (%s) do not match",
			sourceVolumeMode, targetVolumeMode)
	}
	// Can clone.
	return nil
}
