package populators

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
)

const (
	forkliftPopulatorName  = "forklift-populator-controller"
	populatorContainerName = "populate"
	populatorPodPrefix     = "populate"
	populatorPodVolumeName = "target"
	mountPath              = "/mnt/"
	devicePath             = "/dev/block"
)

const apiGroup = "forklift.konveyor.io"

var (
	supportedPopulators = map[string]client.Object{
		"OvirtVolumePopulator":     &v1beta1.OvirtVolumePopulator{},
		"OpenstackVolumePopulator": &v1beta1.OpenstackVolumePopulator{},
	}
)

var errCrNotFound = errors.New("populator CR not found")

// ForkliftPopulatorReconciler members
type ForkliftPopulatorReconciler struct {
	ReconcilerBase
	importerImage       string
	ovirtPopulatorImage string
}

// NewForkliftPopulator creates a new instance of the forklift controller.
func NewForkliftPopulator(
	ctx context.Context,
	mgr manager.Manager,
	log logr.Logger,
	importerImage string,
	ovirtPopulatorImage string,
	installerLabels map[string]string,
) (controller.Controller, error) {
	client := mgr.GetClient()
	reconciler := &ForkliftPopulatorReconciler{
		ReconcilerBase: ReconcilerBase{
			client:          client,
			scheme:          mgr.GetScheme(),
			log:             log.WithName(forkliftPopulatorName),
			recorder:        mgr.GetEventRecorderFor(forkliftPopulatorName),
			featureGates:    featuregates.NewFeatureGates(client),
			installerLabels: installerLabels,
		},
		importerImage:       importerImage,
		ovirtPopulatorImage: ovirtPopulatorImage,
	}

	forkliftPopulatorController, err := controller.New(forkliftPopulatorName, mgr, controller.Options{
		Reconciler: reconciler,
	})
	if err != nil {
		return nil, err
	}

	for kind, sourceType := range supportedPopulators {
		if err := addWatchers(mgr, forkliftPopulatorController, log, kind, sourceType); err != nil {
			return nil, err
		}
	}

	return forkliftPopulatorController, nil
}

func addWatchers(mgr manager.Manager, c controller.Controller, log logr.Logger, sourceKind string, sourceType client.Object) error {
	// Setup watches
	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}), handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			if isPVCForkliftKind(pvc) {
				pvcKey := types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}
				return []reconcile.Request{{NamespacedName: pvcKey}}
			}

			if isPVCPrimeForkliftKind(pvc) {
				owner := metav1.GetControllerOf(pvc)
				pvcKey := types.NamespacedName{Namespace: pvc.Namespace, Name: owner.Name}
				return []reconcile.Request{{NamespacedName: pvcKey}}
			}
			return nil
		}),
	); err != nil {
		return err
	}

	// Watch the populator Pod
	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}), handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			pod := obj.(*corev1.Pod)
			if pod.GetAnnotations()[cc.AnnPopulatorKind] != "forklift" {
				return nil
			}

			// Get pod owner reference
			owner := metav1.GetControllerOf(pod)
			if owner == nil {
				return nil
			}

			pvcPrime := &corev1.PersistentVolumeClaim{}
			err := mgr.GetClient().Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, pvcPrime)
			if err != nil {
				return nil
			}

			// Check if the owner is a PVC prime
			if !isPVCPrimeForkliftKind(pvcPrime) {
				return nil
			}

			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}}}
		}),
	); err != nil {
		return err
	}

	mapDataSourceRefToPVC := func(ctx context.Context, obj client.Object) (reqs []reconcile.Request) {
		var pvcs corev1.PersistentVolumeClaimList
		matchingFields := client.MatchingFields{
			dataSourceRefField: getPopulatorIndexKey(apiGroup, sourceKind, obj.GetNamespace(), obj.GetName()),
		}
		if err := mgr.GetClient().List(ctx, &pvcs, matchingFields); err != nil {
			log.Error(err, "Unable to list PVCs", "matchingFields", matchingFields)
			return reqs
		}
		for _, pvc := range pvcs.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}})
		}
		return reqs
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), sourceType),
		handler.EnqueueRequestsFromMapFunc(mapDataSourceRefToPVC),
	); err != nil {
		return err
	}

	return nil
}

// Reconcile the reconcile loop for the PVC with DataSourceRef of OvirtVolumePopulator or OpenstackVolumePopulator kind
func (r *ForkliftPopulatorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("PVC", req.NamespacedName)
	log.V(1).Info("reconciling Forklift PVCs")
	return r.reconcile(ctx, req, r, log)
}

func (r *ForkliftPopulatorReconciler) reconcile(ctx context.Context, req reconcile.Request, populator populatorController, log logr.Logger) (reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.client.Get(ctx, req.NamespacedName, pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if pvc.Status.Phase == corev1.ClaimBound {
		r.log.Info("PVC is bound, skipping...", "pvc", pvc.Name)
		return reconcile.Result{}, nil
	}

	// We first perform the common reconcile steps.
	// We should only continue if we get a valid PVC'
	pvcPrime, err := r.reconcileCommon(pvc, populator, log)
	if err != nil || pvcPrime == nil {
		return reconcile.Result{}, err
	}

	r.log.V(1).Info("reconciling PVC prime", "pvc", pvcPrime.Name)

	// Each populator reconciles the target PVC in a different way
	if cc.IsUnbound(pvc) || !cc.IsPVCComplete(pvc) {
		return populator.reconcileTargetPVC(pvc, pvcPrime)
	}

	return r.reconcileCleanup(pvcPrime)
}

// TODO(benny) rename
func (r *ForkliftPopulatorReconciler) reconcileCommon(pvc *corev1.PersistentVolumeClaim, populator populatorController, log logr.Logger) (*corev1.PersistentVolumeClaim, error) {
	if pvc.DeletionTimestamp != nil {
		log.V(1).Info("PVC being terminated, ignoring")
		return nil, nil
	}

	pvcPrime, err := r.getPVCPrime(pvc)
	if err != nil {
		return nil, err
	}
	if pvcPrime != nil {
		return pvcPrime, nil
	}

	dataSourceRef := pvc.Spec.DataSourceRef

	if !isPVCForkliftKind(pvc) {
		log.V(1).Info("reconciled unexpected PVC, ignoring")
		return nil, nil
	}

	// TODO: Remove this check once we support cross-namespace dataSourceRef
	if dataSourceRef.Namespace != nil && *dataSourceRef.Namespace != pvc.Namespace {
		log.V(1).Info("cross-namespace dataSourceRef not supported yet, ignoring")
		return nil, nil
	}

	populationSource, err := populator.getPopulationSource(pvc)
	if populationSource == nil {
		return nil, err
	}

	ready, nodeName, err := claimReadyForPopulation(context.TODO(), r.client, pvc)
	if !ready || err != nil {
		return nil, err
	}

	if cc.IsUnbound(pvc) {
		_, err := r.createPVCPrime(pvc, populationSource, nodeName != "", populator.updatePVCForPopulation)
		if err != nil {
			r.recorder.Eventf(pvc, corev1.EventTypeWarning, errCreatingPVCPrime, err.Error())
			return nil, err
		}
	}

	return nil, nil
}

func (r *ForkliftPopulatorReconciler) getPopulationSource(pvc *corev1.PersistentVolumeClaim) (client.Object, error) {
	switch pvc.Spec.DataSourceRef.Kind {
	case "OvirtVolumePopulator":
		return &v1beta1.OvirtVolumePopulator{}, nil
	case "OpenstackVolumePopulator":
		return &v1beta1.OpenstackVolumePopulator{}, nil
	default:
		return nil, fmt.Errorf("unknown populator type %T", pvc.Spec.DataSourceRef.Kind)
	}
}

func isPVCForkliftKind(pvc *corev1.PersistentVolumeClaim) bool {
	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef == nil {
		return false
	}

	if (dataSourceRef.APIGroup != nil && *dataSourceRef.APIGroup != apiGroup) ||
		dataSourceRef.Name == "" || dataSourceRef.Kind == "" {
		return false
	}

	_, ok := supportedPopulators[dataSourceRef.Kind]
	return ok
}

func isPVCPrimeForkliftKind(pvc *corev1.PersistentVolumeClaim) bool {
	owner := metav1.GetControllerOf(pvc)
	if owner == nil || owner.Kind != "PersistentVolumeClaim" {
		return false
	}

	populatorKind := pvc.Annotations[cc.AnnPopulatorKind]
	return populatorKind == "forklift"
}

func (r *ForkliftPopulatorReconciler) reconcileTargetPVC(pvc, pvcPrime *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	pvcPrimeCopy := pvcPrime.DeepCopy()

	// Look for the populator pod
	podName := fmt.Sprintf("%s-%s", populatorPodPrefix, pvc.UID)
	pod, err := r.getImportPod(pvcPrime, podName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if pod == nil {
		err = r.createPopulatorPod(pvcPrime, pvc)
		if err != nil {
			if errors.Is(err, errCrNotFound) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	anno := pvcPrimeCopy.Annotations
	anno[cc.AnnPodPhase] = string(pod.Status.Phase)
	anno[cc.AnnImportPod] = pod.Name

	phase := pvcPrimeCopy.Annotations[cc.AnnPodPhase]
	switch phase {
	case string(corev1.PodRunning):
		if err := r.updatePVCPrime(pvc, pvcPrimeCopy); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	case string(corev1.PodFailed):
		r.recorder.Eventf(pvc, corev1.EventTypeWarning, importFailed, messageImportFailed, pvc.Name)
	case string(corev1.PodPending):
		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	case string(corev1.PodSucceeded):
		if err := cc.Rebind(context.TODO(), r.client, pvcPrime, pvc); err != nil {
			return reconcile.Result{}, err
		}
	default:
		//Should never happen
		return reconcile.Result{}, fmt.Errorf("unknown pod phase %s", phase)
	}

	if err := r.updatePVCPrime(pvc, pvcPrimeCopy); err != nil {
		return reconcile.Result{}, err
	}

	if cc.IsPVCComplete(pvcPrime) {
		// TODO(benny) use a different const?
		r.recorder.Eventf(pvc, corev1.EventTypeNormal, importSucceeded, messageImportSucceeded, pvc.Name)
	}

	if cc.ShouldDeletePod(pvcPrime) {
		if err := r.client.Delete(context.TODO(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: pvc.GetNamespace(),
			},
		}); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ForkliftPopulatorReconciler) updatePVCPrime(pvc, pvcPrime *corev1.PersistentVolumeClaim) error {
	_, err := r.updatePVCWithPVCPrimeAnnotations(pvc, pvcPrime, r.updateAnnotations)
	if err != nil {
		return err
	}

	return nil
}

func (r *ForkliftPopulatorReconciler) updateAnnotations(pvc, pvcPrime *corev1.PersistentVolumeClaim) {
	phase := pvcPrime.Annotations[cc.AnnPodPhase]
	if err := r.updateImportProgress(phase, pvc, pvcPrime); err != nil {
		r.log.Error(err, "Failed to update import progress for pvc %s/%s", pvc.Namespace, pvc.Name)
	}
}

func (r *ForkliftPopulatorReconciler) updatePVCForPopulation(pvc *corev1.PersistentVolumeClaim, source client.Object) {
	annotations := pvc.Annotations
	annotations[cc.AnnUsePopulator] = "true"
	cc.AddAnnotation(pvc, cc.AnnPopulatorKind, "forklift")
	cc.AddAnnotation(pvc, cc.AnnUsePopulator, "true")
}

func (r *ForkliftPopulatorReconciler) updateImportProgress(podPhase string, pvc, pvcPrime *corev1.PersistentVolumeClaim) error {
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
	importRegExp := regexp.MustCompile("progress\\{ownerUID\\=\"" + string(pvc.UID) + "\"\\} (\\d{1,3}\\.?\\d*)")
	httpClient = cc.BuildHTTPClient(httpClient)
	progressReport, err := cc.GetProgressReportFromURL(url, importRegExp, httpClient)
	if err != nil {
		return err
	}

	if progressReport != "" {
		if f, err := strconv.ParseFloat(progressReport, 64); err == nil {
			cc.AddAnnotation(pvc, cc.AnnPopulatorProgress, fmt.Sprintf("%.2f%%", f))
		}
	}

	return nil
}

func (r *ForkliftPopulatorReconciler) getImportPod(pvc *corev1.PersistentVolumeClaim, importPodName string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: importPodName, Namespace: pvc.GetNamespace()}, pod); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}

	if !metav1.IsControlledBy(pod, pvc) {
		return nil, errors.Errorf("Pod is not owned by PVC")
	}
	return pod, nil
}

func (r *ForkliftPopulatorReconciler) createPopulatorPod(pvcPrime, pvc *corev1.PersistentVolumeClaim) error {
	var rawBlock bool
	if pvc.Spec.VolumeMode != nil && corev1.PersistentVolumeBlock == *pvc.Spec.VolumeMode {
		rawBlock = true
	}

	crKind := pvc.Spec.DataSourceRef.Kind
	crName := pvc.Spec.DataSourceRef.Name
	var executable, secretName, containerImage, transferNetwork string
	var args []string

	switch crKind {
	case "OvirtVolumePopulator":
		crInstance := &v1beta1.OvirtVolumePopulator{}
		found, err := cc.GetResource(context.TODO(), r.client, pvc.Namespace, crName, crInstance)
		if err != nil {
			return err
		}
		if !found {
			return errCrNotFound
		}
		executable = "ovirt-populator"
		args = getOvirtPopulatorPodArgs(rawBlock, crInstance)
		secretName = crInstance.Spec.SecretRef
		containerImage = r.ovirtPopulatorImage
		transferNetwork = crInstance.Spec.TransferNetwork
	case "OpenstackVolumePopulator":
		crInstance := &v1beta1.OpenstackVolumePopulator{}
		found, err := cc.GetResource(context.TODO(), r.client, pvc.Namespace, crName, crInstance)
		if err != nil {
			return err
		}
		if !found {
			return errCrNotFound
		}
		executable = "openstack-populator"
		args = getOpenstackPopulatorPodArgs(rawBlock, crInstance)
		secretName = crInstance.Spec.SecretRef
		containerImage = r.importerImage
		transferNetwork = crInstance.Spec.TransferNetwork
	default:
		return fmt.Errorf("unknown populator type %T", crKind)
	}

	args = append(args, fmt.Sprintf("--owner-uid=%s", string(pvc.UID)))
	args = append(args, fmt.Sprintf("--pvc-size=%d", pvc.Spec.Resources.Requests.Storage().Value()))

	annotations := map[string]string{
		cc.AnnPopulatorKind: "forklift",
	}

	if transferNetwork != "" {
		annotations[cc.AnnPodMultusDefaultNetwork] = transferNetwork
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", populatorPodPrefix, pvc.UID),
			Namespace: pvc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvcPrime.Name,
					UID:                pvcPrime.GetUID(),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Annotations: annotations,
		},

		Spec: makePopulatePodSpec(pvcPrime.Name, secretName),
	}

	cc.SetNodeNameIfPopulator(pvc, &pod.Spec)

	con := &pod.Spec.Containers[0]
	con.Image = containerImage
	con.Command = []string{executable}
	con.Args = args
	if rawBlock {
		con.VolumeDevices = []corev1.VolumeDevice{
			{
				Name:       populatorPodVolumeName,
				DevicePath: devicePath,
			},
		}
	} else {
		con.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      populatorPodVolumeName,
				MountPath: mountPath,
			},
		}
	}

	if err := r.client.Create(context.TODO(), &pod); err != nil {
		return err
	}

	return nil
}

func makePopulatePodSpec(pvcPrimeName, secretName string) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  populatorContainerName,
				Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 8443}},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					RunAsUser:                ptr.To[int64](107),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				EnvFrom: []corev1.EnvFromSource{
					{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
						},
					},
				},
			},
		},
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup: ptr.To[int64](107),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
		Volumes: []corev1.Volume{
			{
				Name: populatorPodVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcPrimeName,
					},
				},
			},
		},
	}
}

func getOvirtPopulatorPodArgs(rawBlock bool, ovirtCR *v1beta1.OvirtVolumePopulator) []string {
	var args []string
	if rawBlock {
		args = append(args, "--volume-path="+devicePath)
	} else {
		args = append(args, "--volume-path="+mountPath+"disk.img")
	}

	args = append(args, "--secret-name="+ovirtCR.Spec.SecretRef)
	args = append(args, "--disk-id="+ovirtCR.Spec.DiskID)
	args = append(args, "--engine-url="+ovirtCR.Spec.EngineURL)

	return args
}

func getOpenstackPopulatorPodArgs(rawBlock bool, openstackCR *v1beta1.OpenstackVolumePopulator) []string {
	args := []string{}
	if rawBlock {
		args = append(args, "--volume-path="+devicePath)
	} else {
		args = append(args, "--volume-path="+mountPath+"disk.img")
	}

	args = append(args, "--endpoint="+openstackCR.Spec.IdentityURL)
	args = append(args, "--secret-name="+openstackCR.Spec.SecretRef)
	args = append(args, "--image-id="+openstackCR.Spec.ImageID)

	return args
}
