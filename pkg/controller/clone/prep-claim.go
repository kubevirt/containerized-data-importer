package clone

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// PrepClaimPhaseName is the name of the prep claim phase
const PrepClaimPhaseName = "PrepClaim"

// PrepClaimPhase is responsible for prepping a PVC for rebind
type PrepClaimPhase struct {
	Owner           client.Object
	DesiredClaim    *corev1.PersistentVolumeClaim
	Image           string
	PullPolicy      corev1.PullPolicy
	InstallerLabels map[string]string
	OwnershipLabel  string
	Client          client.Client
	Log             logr.Logger
	Recorder        record.EventRecorder
}

var _ Phase = &PrepClaimPhase{}

// Name returns the name of the phase
func (p *PrepClaimPhase) Name() string {
	return PrepClaimPhaseName
}

// Reconcile ensures that a pvc is bound and resized if necessary
func (p *PrepClaimPhase) Reconcile(ctx context.Context) (*reconcile.Result, error) {
	actualClaim := &corev1.PersistentVolumeClaim{}
	pvcExists, err := getResource(ctx, p.Client, p.DesiredClaim.Namespace, p.DesiredClaim.Name, actualClaim)
	if err != nil {
		return nil, err
	}

	if !pvcExists {
		return nil, fmt.Errorf("claim %s/%s does not exist", p.DesiredClaim.Namespace, p.DesiredClaim.Name)
	}

	podName := fmt.Sprintf("prep-%s", string(p.Owner.GetUID()))
	pod := &corev1.Pod{}
	podExists, err := getResource(ctx, p.Client, p.DesiredClaim.Namespace, podName, pod)
	if err != nil {
		return nil, err
	}

	podRequired := false
	requestedSize, hasRequested := p.DesiredClaim.Spec.Resources.Requests[corev1.ResourceStorage]
	currentSize, hasCurrent := actualClaim.Spec.Resources.Requests[corev1.ResourceStorage]
	actualSize, hasActual := actualClaim.Status.Capacity[corev1.ResourceStorage]
	if !hasRequested || !hasCurrent {
		return nil, fmt.Errorf("requested PVC sizes missing")
	}

	p.Log.V(3).Info("Expand sizes", "req", requestedSize, "cur", currentSize, "act", actualSize)

	if !hasActual {
		if cc.IsBound(actualClaim) {
			// PVC is bound but its status hasn't been updated yet.
			// We'll reconcile again once the status is updated.
			if actualClaim.Status.Phase == corev1.ClaimPending {
				return &reconcile.Result{}, nil
			}
			return nil, fmt.Errorf("actual PVC size missing")
		}

		p.Log.V(3).Info("prep pod required to force bind")
		podRequired = true
	} else {
		if currentSize.Cmp(requestedSize) < 0 {
			p.Log.V(3).Info("Updating resource requests to", "size", requestedSize)

			actualClaim.Spec.Resources.Requests[corev1.ResourceStorage] = requestedSize
			if err := p.Client.Update(ctx, actualClaim); err != nil {
				return nil, err
			}

			// come back once pvc is updated
			return &reconcile.Result{}, nil
		}

		if actualSize.Cmp(requestedSize) < 0 {
			p.Log.V(3).Info("prep pod required to do resize")
			podRequired = true
		}
	}

	p.Log.V(3).Info("Prep status", "podRequired", podRequired, "podExists", podExists)

	if !podRequired && !podExists {
		// all done finally
		return nil, nil
	}

	if podExists && pod.Status.Phase == corev1.PodSucceeded {
		p.Log.V(3).Info("Prep pod succeeded, deleting")

		if err := p.Client.Delete(ctx, pod); err != nil {
			return nil, err
		}
	}

	if podRequired && !podExists {
		p.Log.V(3).Info("creating prep pod")

		if err := p.createPod(ctx, podName, actualClaim); err != nil {
			return nil, err
		}
	}

	// pod is running
	return &reconcile.Result{}, nil
}

func (p *PrepClaimPhase) createPod(ctx context.Context, name string, pvc *corev1.PersistentVolumeClaim) error {
	resourceRequirements, err := cc.GetDefaultPodResourceRequirements(p.Client)
	if err != nil {
		return err
	}

	imagePullSecrets, err := cc.GetImagePullSecrets(p.Client)
	if err != nil {
		return err
	}

	workloadNodePlacement, err := cc.GetWorkloadNodePlacement(ctx, p.Client)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				cc.AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "cdi-populator-prep",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "dummy",
					Image:           p.Image,
					ImagePullPolicy: corev1.PullPolicy(p.PullPolicy),
					Command:         []string{"/bin/bash"},
					Args:            []string{"-c", "echo", "'hello cdi'"},
				},
			},
			ImagePullSecrets: imagePullSecrets,
			RestartPolicy:    corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: cc.DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
			NodeSelector: workloadNodePlacement.NodeSelector,
			Tolerations:  workloadNodePlacement.Tolerations,
			Affinity:     workloadNodePlacement.Affinity,
		},
	}
	util.SetRecommendedLabels(pod, p.InstallerLabels, "cdi-controller")

	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = cc.AddVolumeDevices()
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      cc.DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
	}

	if resourceRequirements != nil {
		pod.Spec.Containers[0].Resources = *resourceRequirements
	}

	if pvc.Annotations[cc.AnnSelectedNode] != "" {
		pod.Spec.NodeName = pvc.Annotations[cc.AnnSelectedNode]
	}

	if p.OwnershipLabel != "" {
		AddOwnershipLabel(p.OwnershipLabel, pod, p.Owner)
	}

	cc.SetAllowedAnnotations(pod, pvc.ObjectMeta)
	cc.SetRestrictedSecurityContext(&pod.Spec)

	if err := p.Client.Create(ctx, pod); err != nil {
		return err
	}

	return nil
}
