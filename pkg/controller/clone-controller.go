package controller

import (
	"context"
	"crypto/rsa"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

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
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	// AnnCloneSourcePod name of the source clone pod
	AnnCloneSourcePod = "cdi.kubevirt.io/storage.sourceClonePodName"

	// TokenKeyDir is the path to the apiserver public key dir
	TokenKeyDir = "/var/run/cdi/token/keys"

	// TokenPublicKeyPath is the path to the apiserver public key
	TokenPublicKeyPath = TokenKeyDir + "/id_rsa.pub"

	// TokenPrivateKeyPath is the path to the apiserver private key
	TokenPrivateKeyPath = TokenKeyDir + "/id_rsa"

	// CloneSucceededPVC provides a const to indicate a clone to the PVC succeeded
	CloneSucceededPVC = "CloneSucceeded"

	cloneSourcePodFinalizer = "cdi.kubevirt.io/cloneSource"

	uploadClientCertDuration = 365 * 24 * time.Hour
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
		shortTokenValidator: cc.NewCloneTokenValidator(common.CloneTokenIssuer, apiServerKey),
		longTokenValidator:  cc.NewCloneTokenValidator(common.ExtendedCloneTokenIssuer, apiServerKey),
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

// addCloneControllerWatches sets up the watches used by the clone controller.
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

func (r *CloneReconciler) shouldReconcile(pvc *corev1.PersistentVolumeClaim, log logr.Logger) bool {
	return checkPVC(pvc, cc.AnnCloneRequest, log) &&
		!metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCloneOf) &&
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
			"checkPVC(AnnCloneRequest)", checkPVC(pvc, cc.AnnCloneRequest, log),
			"NOT has annotation(AnnCloneOf)", !metav1.HasAnnotation(pvc.ObjectMeta, cc.AnnCloneOf),
			"isBound", isBound(pvc, log),
			"has finalizer?", cc.HasFinalizer(pvc, cloneSourcePodFinalizer))
		if cc.HasFinalizer(pvc, cloneSourcePodFinalizer) || pvc.DeletionTimestamp != nil {
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
		pvc.Annotations[AnnCloneSourcePod] = cc.CreateCloneSourcePodName(pvc)

		// add finalizer before creating clone source pod
		cc.AddFinalizer(pvc, cloneSourcePodFinalizer)

		if err := r.updatePVC(pvc); err != nil {
			return reconcile.Result{}, err
		}

		// will reconcile again after PVC update notification
		return reconcile.Result{}, nil
	}

	if requeueAfter, err := r.reconcileSourcePod(sourcePod, pvc, log); requeueAfter != 0 || err != nil {
		return reconcile.Result{RequeueAfter: requeueAfter}, err
	}

	if err := r.ensureCertSecret(sourcePod, pvc, log); err != nil {
		return reconcile.Result{}, err
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

		sourcePopulated, err := cc.IsPopulated(sourcePvc, r.client)
		if err != nil {
			return 0, err
		}
		if !sourcePopulated {
			return 2 * time.Second, nil
		}

		if err := r.validateSourceAndTarget(sourcePvc, targetPvc); err != nil {
			return 0, err
		}

		pods, err := cc.GetPodsUsingPVCs(r.client, sourcePvc.Namespace, sets.NewString(sourcePvc.Name), true)
		if err != nil {
			return 0, err
		}

		if len(pods) > 0 {
			for _, pod := range pods {
				r.log.V(1).Info("can't create clone source pod, pvc in use by other pod",
					"namespace", sourcePvc.Namespace, "name", sourcePvc.Name, "pod", pod.Name)
				r.recorder.Eventf(targetPvc, corev1.EventTypeWarning, cc.CloneSourceInUse,
					"pod %s/%s using PersistentVolumeClaim %s", pod.Namespace, pod.Name, sourcePvc.Name)
			}
			return 2 * time.Second, nil
		}

		sourcePod, err := r.CreateCloneSourcePod(r.image, r.pullPolicy, targetPvc, log)
		// Check if pod has failed and, in that case, record an event with the error
		if podErr := cc.HandleFailedPod(err, cc.CreateCloneSourcePodName(targetPvc), targetPvc, r.recorder, r.client); podErr != nil {
			return 0, podErr
		}

		log.V(3).Info("Created source pod ", "sourcePod.Namespace", sourcePod.Namespace, "sourcePod.Name", sourcePod.Name)
	}
	return 0, nil
}

func (r *CloneReconciler) ensureCertSecret(sourcePod *corev1.Pod, targetPvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	if sourcePod == nil {
		return nil
	}

	if sourcePod.Status.Phase == corev1.PodRunning {
		return nil
	}

	clientName, ok := targetPvc.Annotations[AnnUploadClientName]
	if !ok {
		return errors.Errorf("PVC %s/%s missing required %s annotation", targetPvc.Namespace, targetPvc.Name, AnnUploadClientName)
	}

	cert, key, err := r.clientCertGenerator.MakeClientCert(clientName, nil, uploadClientCertDuration)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourcePod.Name,
			Namespace: sourcePod.Namespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue, //filtered by the podInformer
				common.CDIComponentLabel: common.ClonerSourcePodName,
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePodOwnerReference(sourcePod),
			},
		},
		Data: map[string][]byte{
			"tls.key": key,
			"tls.crt": cert,
		},
	}

	util.SetRecommendedLabels(secret, r.installerLabels, "cdi-controller")

	err = r.client.Create(context.TODO(), secret)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "error creating cert secret")
	}

	return nil
}

func (r *CloneReconciler) updatePvcFromPod(sourcePod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	currentPvcCopy := pvc.DeepCopyObject()
	log.V(1).Info("Updating PVC from pod")

	log.V(3).Info("Pod phase for PVC", "PVC phase", pvc.Annotations[cc.AnnPodPhase])

	if podSucceededFromPVC(pvc) && pvc.Annotations[cc.AnnCloneOf] != "true" && sourcePodFinished(sourcePod) {
		log.V(1).Info("Adding CloneOf annotation to PVC")
		pvc.Annotations[cc.AnnCloneOf] = "true"
		r.recorder.Event(pvc, corev1.EventTypeNormal, CloneSucceededPVC, cc.CloneComplete)
	}

	setAnnotationsFromPodWithPrefix(pvc.Annotations, sourcePod, cc.AnnSourceRunningCondition)

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
	rs, ok := pvc.Annotations[cc.AnnPodReady]
	if !ok {
		log.V(3).Info("clone target pod not ready")
		return false, nil
	}

	ready, err := strconv.ParseBool(rs)
	if err != nil {
		return false, errors.Wrapf(err, "error parsing %s annotation", cc.AnnPodReady)
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
		cloneSourcePodName = cc.CreateCloneSourcePodName(pvc)
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			cc.CloneUniqueID: cloneSourcePodName,
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
	tok, ok := targetPvc.Annotations[cc.AnnExtendedCloneToken]
	if !ok {
		tok, ok = targetPvc.Annotations[cc.AnnCloneToken]
		if !ok {
			return errors.New("clone token missing")
		}
		v = r.shortTokenValidator
	}

	if err := cc.ValidateCloneTokenPVC(tok, v, sourcePvc, targetPvc); err != nil {
		return err
	}
	contentType, err := ValidateCanCloneSourceAndTargetContentType(sourcePvc, targetPvc)
	if err != nil {
		return err
	}
	err = ValidateCanCloneSourceAndTargetSpec(r.client, sourcePvc, targetPvc, contentType)
	if err == nil {
		// Validation complete, put source PVC bound status in annotation
		setBoundConditionFromPVC(targetPvc.GetAnnotations(), cc.AnnBoundCondition, sourcePvc)
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
		if cc.ShouldDeletePod(pvc) {
			log.V(3).Info("Deleting pod", "pod.Name", pod.Name)
			if err = r.client.Delete(context.TODO(), pod); err != nil {
				if !k8serrors.IsNotFound(err) {
					return errors.Wrap(err, "error deleting clone source pod")
				}
			}
		}
	}

	cc.RemoveFinalizer(pvc, cloneSourcePodFinalizer)

	return r.updatePVC(pvc)
}

// CreateCloneSourcePod creates our cloning src pod which will be used for out of band cloning to read the contents of the src PVC
func (r *CloneReconciler) CreateCloneSourcePod(image, pullPolicy string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*corev1.Pod, error) {
	exists, sourcePvcNamespace, sourcePvcName := ParseCloneRequestAnnotation(pvc)
	if !exists {
		return nil, errors.Errorf("bad CloneRequest Annotation")
	}

	ownerKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return nil, errors.Wrap(err, "error getting cache key")
	}

	serverCABundle, err := r.serverCAFetcher.BundleBytes()
	if err != nil {
		return nil, err
	}

	podResourceRequirements, err := cc.GetDefaultPodResourceRequirements(r.client)
	if err != nil {
		return nil, err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(r.client)
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

	pod := MakeCloneSourcePodSpec(sourceVolumeMode, image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerKey, serverCABundle, pvc, podResourceRequirements, workloadNodePlacement)
	util.SetRecommendedLabels(pod, r.installerLabels, "cdi-controller")

	if err := r.client.Create(context.TODO(), pod); err != nil {
		return nil, errors.Wrap(err, "source pod API create errored")
	}

	log.V(1).Info("cloning source pod (image) created\n", "pod.Namespace", pod.Namespace, "pod.Name", pod.Name, "image", image)

	return pod, nil
}

// MakeCloneSourcePodSpec creates and returns the clone source pod spec based on the target pvc.
func MakeCloneSourcePodSpec(sourceVolumeMode corev1.PersistentVolumeMode, image, pullPolicy, sourcePvcName, sourcePvcNamespace, ownerRefAnno string,
	serverCACert []byte, targetPvc *corev1.PersistentVolumeClaim, resourceRequirements *corev1.ResourceRequirements,
	workloadNodePlacement *sdkapi.NodePlacement) *corev1.Pod {

	var ownerID string
	cloneSourcePodName := targetPvc.Annotations[AnnCloneSourcePod]
	url := GetUploadServerURL(targetPvc.Namespace, targetPvc.Name, common.UploadPathSync)
	pvcOwner := metav1.GetControllerOf(targetPvc)
	if pvcOwner != nil && pvcOwner.Kind == "DataVolume" {
		ownerID = string(pvcOwner.UID)
	} else {
		ouid, ok := targetPvc.Annotations[cc.AnnOwnerUID]
		if ok {
			ownerID = ouid
		}
	}

	preallocationRequested := targetPvc.Annotations[cc.AnnPreallocationRequested]

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloneSourcePodName,
			Namespace: sourcePvcNamespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
				AnnOwnerRef:     ownerRefAnno,
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue, //filtered by the podInformer
				common.CDIComponentLabel: common.ClonerSourcePodName,
				// this label is used when searching for a pvc's cloner source pod.
				cc.CloneUniqueID:          cloneSourcePodName,
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            common.ClonerSourcePodName,
					Image:           image,
					ImagePullPolicy: corev1.PullPolicy(pullPolicy),
					Env: []corev1.EnvVar{
						{
							Name: "CLIENT_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cloneSourcePodName,
									},
									Key: "tls.key",
								},
							},
						},
						{
							Name: "CLIENT_CERT",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cloneSourcePodName,
									},
									Key: "tls.crt",
								},
							},
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
					Name: cc.DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
						},
					},
				},
			},
			NodeSelector:      workloadNodePlacement.NodeSelector,
			Tolerations:       workloadNodePlacement.Tolerations,
			Affinity:          workloadNodePlacement.Affinity,
			PriorityClassName: cc.GetPriorityClass(targetPvc),
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
		pod.Spec.Containers[0].VolumeDevices = cc.AddVolumeDevices()
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
				Name:      cc.DataVolName,
				MountPath: common.ClonerMountPath,
				ReadOnly:  true,
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
	setPodPvcAnnotations(pod, targetPvc)
	cc.SetRestrictedSecurityContext(&pod.Spec)
	return pod
}

// ParseCloneRequestAnnotation parses the clone request annotation
func ParseCloneRequestAnnotation(pvc *corev1.PersistentVolumeClaim) (exists bool, namespace, name string) {
	var ann string
	ann, exists = pvc.Annotations[cc.AnnCloneRequest]
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
	sourceContentType := cc.GetContentType(sourcePvc)
	targetContentType := cc.GetContentType(targetPvc)
	if sourceContentType != targetContentType {
		return "", fmt.Errorf("source contentType (%s) and target contentType (%s) do not match", sourceContentType, targetContentType)
	}
	return cdiv1.DataVolumeContentType(sourceContentType), nil
}

// ValidateCanCloneSourceAndTargetSpec validates the specs passed in are compatible for cloning.
func ValidateCanCloneSourceAndTargetSpec(c client.Client, sourcePvc, targetPvc *corev1.PersistentVolumeClaim, contentType cdiv1.DataVolumeContentType) error {
	// This annotation is only needed for some specific cases, when the target size is actually calculated by
	// the size detection pod, and there are some differences in fs overhead between src and target volumes
	_, permissive := targetPvc.Annotations[cc.AnnPermissiveClone]

	// Allow different source and target volume modes only on KubeVirt content type
	sourceVolumeMode := util.ResolveVolumeMode(sourcePvc.Spec.VolumeMode)
	targetVolumeMode := util.ResolveVolumeMode(targetPvc.Spec.VolumeMode)
	if sourceVolumeMode != targetVolumeMode && contentType != cdiv1.DataVolumeKubeVirt {
		return fmt.Errorf("source volumeMode (%s) and target volumeMode (%s) do not match, contentType (%s)",
			sourceVolumeMode, targetVolumeMode, contentType)
	}

	// TODO: use detection pod here, then permissive should not be needed
	sourceUsableSpace, err := getUsableSpace(c, sourcePvc)
	if err != nil {
		return err
	}
	targetUsableSpace, err := getUsableSpace(c, targetPvc)
	if err != nil {
		return err
	}

	if !permissive && sourceUsableSpace.Cmp(targetUsableSpace) > 0 {
		return errors.New("target resources requests storage size is smaller than the source")
	}

	// Can clone.
	return nil
}

func getUsableSpace(c client.Client, pvc *corev1.PersistentVolumeClaim) (resource.Quantity, error) {
	sizeRequest := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	volumeMode := util.ResolveVolumeMode(pvc.Spec.VolumeMode)

	if volumeMode == corev1.PersistentVolumeFilesystem {
		fsOverhead, err := cc.GetFilesystemOverheadForStorageClass(c, pvc.Spec.StorageClassName)
		if err != nil {
			return resource.Quantity{}, err
		}
		fsOverheadFloat, _ := strconv.ParseFloat(string(fsOverhead), 64)
		usableSpaceRaw := util.GetUsableSpace(fsOverheadFloat, sizeRequest.Value())

		return *resource.NewScaledQuantity(usableSpaceRaw, 0), nil
	}

	return sizeRequest, nil
}
