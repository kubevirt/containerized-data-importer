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

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	//AnnCloneRequest sets our expected annotation for a CloneRequest
	AnnCloneRequest = "k8s.io/CloneRequest"
	//AnnCloneOf is used to indicate that cloning was complete
	AnnCloneOf = "k8s.io/CloneOf"
	// AnnCloneToken is the annotation containing the clone token
	AnnCloneToken = "cdi.kubevirt.io/storage.clone.token"
	// AnnExtendedCloneToken is the annotation containing the long term clone token
	AnnExtendedCloneToken = "cdi.kubevirt.io/storage.extended.clone.token"

	//CloneUniqueID is used as a special label to be used when we search for the pod
	CloneUniqueID = "cdi.kubevirt.io/storage.clone.cloneUniqeId"
	// AnnCloneSourcePod name of the source clone pod
	AnnCloneSourcePod = "cdi.kubevirt.io/storage.sourceClonePodName"

	// ErrIncompatiblePVC provides a const to indicate a clone is not possible due to an incompatible PVC
	ErrIncompatiblePVC = "ErrIncompatiblePVC"

	// TokenKeyDir is the path to the apiserver public key dir
	TokenKeyDir = "/var/run/cdi/token/keys"

	// TokenPublicKeyPath is the path to the apiserver public key
	TokenPublicKeyPath = TokenKeyDir + "/id_rsa.pub"

	// TokenPrivateKeyPath is the path to the apiserver private key
	TokenPrivateKeyPath = TokenKeyDir + "/id_rsa"

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
	longTokenValidator  token.Validator
	shortTokenValidator token.Validator
	image               string
	verbose             string
	pullPolicy          string
	installerLabels     map[string]string
}

// NewCloneController creates a new instance of the config controller.
func NewCloneController(mgr manager.Manager,
	log logr.Logger,
	image, pullPolicy,
	verbose string,
	clientCertGenerator generator.CertGenerator,
	serverCAFetcher fetcher.CertBundleFetcher,
	apiServerKey *rsa.PublicKey,
	installerLabels map[string]string) (controller.Controller, error) {
	reconciler := &CloneReconciler{
		client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		log:                 log.WithName("clone-controller"),
		shortTokenValidator: newCloneTokenValidator(common.CloneTokenIssuer, apiServerKey),
		longTokenValidator:  newCloneTokenValidator(common.ExtendedCloneTokenIssuer, apiServerKey),
		image:               image,
		verbose:             verbose,
		pullPolicy:          pullPolicy,
		recorder:            mgr.GetEventRecorderFor("clone-controller"),
		clientCertGenerator: clientCertGenerator,
		serverCAFetcher:     serverCAFetcher,
		installerLabels:     installerLabels,
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
	if err := cloneController.Watch(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			target, ok := obj.GetAnnotations()[AnnOwnerRef]
			if !ok {
				return nil
			}
			namespace, name, err := cache.SplitMetaNamespaceKey(target)
			if err != nil {
				return nil
			}
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: namespace,
						Name:      name,
					},
				},
			}
		},
	)); err != nil {
		return err
	}
	return nil
}

func newCloneTokenValidator(issuer string, key *rsa.PublicKey) token.Validator {
	return token.NewValidator(issuer, key, cloneTokenLeeway)
}

func (r *CloneReconciler) shouldReconcile(pvc *corev1.PersistentVolumeClaim, log logr.Logger) bool {
	return checkPVC(pvc, AnnCloneRequest, log) &&
		!metav1.HasAnnotation(pvc.ObjectMeta, AnnCloneOf) &&
		isBound(pvc, log)
}

// Reconcile the reconcile loop for host assisted clone pvc.
func (r *CloneReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
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
			"has finalizer?", HasFinalizer(pvc, cloneSourcePodFinalizer))
		if HasFinalizer(pvc, cloneSourcePodFinalizer) || pvc.DeletionTimestamp != nil {
			// Clone completed, remove source pod and/or finalizer
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

		// add finalizer before creating clone source pod
		AddFinalizer(pvc, cloneSourcePodFinalizer)

		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}

		// will reconcile again after PVC update notification
		return reconcile.Result{}, nil
	}

	if requeueAfter, err := r.reconcileSourcePod(sourcePod, pvc, log); requeueAfter != 0 || err != nil {
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	if err := r.updatePvcFromPod(sourcePod, pvc, log); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *CloneReconciler) reconcileSourcePod(sourcePod *corev1.Pod, targetPvc *corev1.PersistentVolumeClaim, log logr.Logger) (time.Duration, error) {
	if sourcePod == nil {
		sourcePvc, err := r.getCloneRequestSourcePVC(targetPvc)
		if err != nil {
			return 0, err
		}

		sourcePopulated, err := IsPopulated(sourcePvc, r.client)
		if err != nil {
			return 0, err
		}
		if !sourcePopulated {
			return 2 * time.Second, nil
		}

		if err := r.validateSourceAndTarget(sourcePvc, targetPvc); err != nil {
			return 0, err
		}

		clientName, ok := targetPvc.Annotations[AnnUploadClientName]
		if !ok {
			return 0, errors.Errorf("PVC %s/%s missing required %s annotation", targetPvc.Namespace, targetPvc.Name, AnnUploadClientName)
		}

		pods, err := GetPodsUsingPVCs(r.client, sourcePvc.Namespace, sets.NewString(sourcePvc.Name), true)
		if err != nil {
			return 0, err
		}

		if len(pods) > 0 {
			for _, pod := range pods {
				r.log.V(1).Info("can't create clone source pod, pvc in use by other pod",
					"namespace", sourcePvc.Namespace, "name", sourcePvc.Name, "pod", pod.Name)
				r.recorder.Eventf(targetPvc, corev1.EventTypeWarning, CloneSourceInUse,
					"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, sourcePvc.Name)
			}
			return 2 * time.Second, nil
		}

		sourcePod, err := r.CreateCloneSourcePod(r.image, r.pullPolicy, clientName, targetPvc, log)
		if err != nil {
			return 0, err
		}
		log.V(3).Info("Created source pod ", "sourcePod.Namespace", sourcePod.Namespace, "sourcePod.Name", sourcePod.Name)
	}
	return 0, nil
}

func (r *CloneReconciler) updatePvcFromPod(sourcePod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()
	log.V(1).Info("Updating PVC from pod")

	log.V(3).Info("Pod phase for PVC", "PVC phase", pvc.Annotations[AnnPodPhase])

	if podSucceededFromPVC(pvc) && pvc.Annotations[AnnCloneOf] != "true" && sourcePodFinished(sourcePod) {
		log.V(1).Info("Adding CloneOf annotation to PVC")
		pvc.Annotations[AnnCloneOf] = "true"
		r.recorder.Event(pvc, corev1.EventTypeNormal, CloneSucceededPVC, cloneComplete)
	}

	setAnnotationsFromPodWithPrefix(pvc.Annotations, sourcePod, AnnSourceRunningCondition)

	if !reflect.DeepEqual(currentPvcCopy, pvc) {
		return r.updatePVC(pvc)
	}
	return nil
}

func sourcePodFinished(sourcePod *corev1.Pod) bool {
	if sourcePod == nil {
		return true
	}

	return sourcePod.Status.Phase == corev1.PodSucceeded || sourcePod.Status.Phase == corev1.PodFailed
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

	return ready || podSucceededFromPVC(pvc), nil
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
	// first check for extended token
	v := r.longTokenValidator
	tok, ok := targetPvc.Annotations[AnnExtendedCloneToken]
	if !ok {
		tok, ok = targetPvc.Annotations[AnnCloneToken]
		if !ok {
			return errors.New("clone token missing")
		}
		v = r.shortTokenValidator
	}

	if err := validateCloneTokenPVC(tok, v, sourcePvc, targetPvc); err != nil {
		return err
	}
	contentType, err := ValidateCanCloneSourceAndTargetContentType(sourcePvc, targetPvc)
	if err != nil {
		return err
	}
	err = ValidateCanCloneSourceAndTargetSpec(&sourcePvc.Spec, &targetPvc.Spec, contentType)
	if err == nil {
		// Validation complete, put source PVC bound status in annotation
		setBoundConditionFromPVC(targetPvc.GetAnnotations(), AnnBoundCondition, sourcePvc)
		return nil
	}
	return err
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
		if shouldDeletePod(pvc) {
			log.V(3).Info("Deleting pod", "pod.Name", pod.Name)
			if err = r.client.Delete(context.TODO(), pod); err != nil {
				if !k8serrors.IsNotFound(err) {
					return errors.Wrap(err, "error deleting clone source pod")
				}
			}
		}
	}

	RemoveFinalizer(pvc, cloneSourcePodFinalizer)

	return r.updatePVC(pvc)
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

	sourcePvc, err := r.getCloneRequestSourcePVC(pvc)
	if err != nil {
		return nil, err
	}

	var sourceVolumeMode corev1.PersistentVolumeMode
	if sourcePvc.Spec.VolumeMode != nil {
		sourceVolumeMode = *sourcePvc.Spec.VolumeMode
	} else {
		sourceVolumeMode = corev1.PersistentVolumeFilesystem
	}

	pod := MakeCloneSourcePodSpec(sourceVolumeMode, image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerKey, clientKey, clientCert, serverCABundle, pvc, podResourceRequirements, workloadNodePlacement)
	util.SetRecommendedLabels(pod, r.installerLabels, "cdi-controller")

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
func MakeCloneSourcePodSpec(sourceVolumeMode corev1.PersistentVolumeMode, image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerRefAnno string,
	clientKey, clientCert, serverCACert []byte, targetPvc *corev1.PersistentVolumeClaim, resourceRequirements *corev1.ResourceRequirements,
	workloadNodePlacement *sdkapi.NodePlacement) *corev1.Pod {

	var ownerID string
	cloneSourcePodName := targetPvc.Annotations[AnnCloneSourcePod]
	url := GetUploadServerURL(targetPvc.Namespace, targetPvc.Name, common.UploadPathSync)
	pvcOwner := metav1.GetControllerOf(targetPvc)
	if pvcOwner != nil && pvcOwner.Kind == "DataVolume" {
		ownerID = string(pvcOwner.UID)
	} else {
		ouid, ok := targetPvc.Annotations[annOwnerUID]
		if ok {
			ownerID = ouid
		}
	}

	preallocationRequested, _ := targetPvc.Annotations[AnnPreallocationRequested]

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
				CloneUniqueID:             cloneSourcePodName,
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
				SELinuxOptions: &corev1.SELinuxOptions{
					User:  "system_u",
					Role:  "system_r",
					Type:  "spc_t",
					Level: "s0",
				},
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
						{
							Name:  common.Preallocation,
							Value: preallocationRequested,
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
							ReadOnly:  true,
						},
					},
				},
			},
			NodeSelector:      workloadNodePlacement.NodeSelector,
			Tolerations:       workloadNodePlacement.Tolerations,
			Affinity:          workloadNodePlacement.Affinity,
			PriorityClassName: getPriorityClass(targetPvc),
		},
	}

	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = &corev1.Affinity{}
	}

	if pod.Spec.Affinity.PodAffinity == nil {
		pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
	}

	pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
		pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
		corev1.WeightedPodAffinityTerm{
			Weight: 100,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      common.UploadTargetLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{string(targetPvc.UID)},
						},
					},
				},
				Namespaces:  []string{targetPvc.Namespace},
				TopologyKey: corev1.LabelHostname,
			},
		},
	)

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	var addVars []corev1.EnvVar

	if sourceVolumeMode == corev1.PersistentVolumeBlock {
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
	SetPodPvcAnnotations(pod, targetPvc)
	return pod
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

// ValidateCanCloneSourceAndTargetContentType validates the pvcs passed has the same content type.
func ValidateCanCloneSourceAndTargetContentType(sourcePvc, targetPvc *corev1.PersistentVolumeClaim) (cdiv1.DataVolumeContentType, error) {
	sourceContentType := GetContentType(sourcePvc)
	targetContentType := GetContentType(targetPvc)
	if sourceContentType != targetContentType {
		return "", fmt.Errorf("source contentType (%s) and target contentType (%s) do not match", sourceContentType, targetContentType)
	}
	return cdiv1.DataVolumeContentType(sourceContentType), nil
}

// ValidateCanCloneSourceAndTargetSpec validates the specs passed in are compatible for cloning.
func ValidateCanCloneSourceAndTargetSpec(sourceSpec, targetSpec *corev1.PersistentVolumeClaimSpec, contentType cdiv1.DataVolumeContentType) error {
	err := ValidateCloneSize(sourceSpec.Resources, targetSpec.Resources)
	if err != nil {
		return err
	}
	// Allow different source and target volume modes only on KubeVirt content type
	sourceVolumeMode := resolveVolumeMode(sourceSpec.VolumeMode)
	targetVolumeMode := resolveVolumeMode(targetSpec.VolumeMode)
	if sourceVolumeMode != targetVolumeMode && contentType != cdiv1.DataVolumeKubeVirt {
		return fmt.Errorf("source volumeMode (%s) and target volumeMode (%s) do not match, contentType (%s)",
			sourceVolumeMode, targetVolumeMode, contentType)
	}
	// Can clone.
	return nil
}

// ValidateCloneSize validates the clone size requirements
func ValidateCloneSize(sourceResources corev1.ResourceRequirements, targetResources corev1.ResourceRequirements) error {
	sourceRequest := sourceResources.Requests[corev1.ResourceStorage]
	targetRequest := targetResources.Requests[corev1.ResourceStorage]
	// Verify that the target PVC size is equal or larger than the source.
	if sourceRequest.Value() > targetRequest.Value() {
		return errors.New("target resources requests storage size is smaller than the source")
	}
	return nil
}
