package resources

import (
	"fmt"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ResourceBuilder helps in building k8s resources
type ResourceBuilder struct {
	commonLabels   map[string]string
	operatorLabels map[string]string
}

// NewResourceBuilder creates new ResourceBuilder
func NewResourceBuilder(commonLabels map[string]string, operatorLabels map[string]string) ResourceBuilder {
	return ResourceBuilder{
		commonLabels:   commonLabels,
		operatorLabels: operatorLabels,
	}
}

// WithCommonLabels aggregates common labels
func (b *ResourceBuilder) WithCommonLabels(labels map[string]string) map[string]string {
	return WithLabels(labels, b.commonLabels)
}

// WithOperatorLabels aggregates operator labels
func (b *ResourceBuilder) WithOperatorLabels(labels map[string]string) map[string]string {
	return WithLabels(labels, b.operatorLabels)
}

// CreateOperatorDeployment creates deployment
func (b *ResourceBuilder) CreateOperatorDeployment(name, namespace, matchKey, matchValue, serviceAccount string, numReplicas int32, podSpec corev1.PodSpec) *appsv1.Deployment {
	labels := WithLabels(map[string]string{matchKey: matchValue}, b.operatorLabels)
	return CreateDeployment(name, namespace, labels, labels, numReplicas, podSpec, serviceAccount, nil)
}

// CreateDeployment creates deployment
func (b *ResourceBuilder) CreateDeployment(name, namespace, matchKey, matchValue, serviceAccount string, numReplicas int32, podSpec corev1.PodSpec, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	selectorMatchMap := map[string]string{matchKey: matchValue}
	finalLabels := WithLabels(map[string]string{matchKey: matchValue}, b.commonLabels)
	return CreateDeployment(name, namespace, selectorMatchMap, finalLabels, numReplicas, podSpec, serviceAccount, infraNodePlacement)
}

// CreateContainer creates container
func (b *ResourceBuilder) CreateContainer(name, image, pullPolicy string) *corev1.Container {
	return b.CreatePortsContainer(name, image, pullPolicy, nil)
}

// CreatePortsContainer creates container with ports
func (b *ResourceBuilder) CreatePortsContainer(name, image, pullPolicy string, ports []corev1.ContainerPort) *corev1.Container {
	container := corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: corev1.PullPolicy(pullPolicy),
		Ports:           ports,
	}
	return &container
}

// CreateService creates service
func (b *ResourceBuilder) CreateService(name, matchKey, matchValue string, additionalLabels map[string]string) *corev1.Service {
	return CreateService(name, matchKey, matchValue, b.WithCommonLabels(additionalLabels))
}

// CreateSecret creates secret
func (b *ResourceBuilder) CreateSecret(name string) *corev1.Secret {
	return CreateSecret(name, b.WithCommonLabels(nil))
}

// CreateConfigMap creates config map
func (b *ResourceBuilder) CreateConfigMap(name string) *corev1.ConfigMap {
	return CreateConfigMap(name, b.WithCommonLabels(nil))
}

// CreateSecret creates secret
func CreateSecret(name string, labels map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

// CreateConfigMap creates config map
func CreateConfigMap(name string, labels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

// CreateDeployment creates deployment
func CreateDeployment(name string, namespace string, selectorMatchMap map[string]string, labels map[string]string, numReplicas int32, podSpec corev1.PodSpec, serviceAccount string, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &numReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorMatchMap,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}
	if infraNodePlacement != nil {
		deployment.Spec.Template.Spec.NodeSelector = infraNodePlacement.NodeSelector
		deployment.Spec.Template.Spec.Tolerations = infraNodePlacement.Tolerations
		deployment.Spec.Template.Spec.Affinity = infraNodePlacement.Affinity
	}
	if serviceAccount != "" {
		deployment.Spec.Template.Spec.ServiceAccountName = serviceAccount
	}
	return deployment
}

// CreateService creates service
func CreateService(name, matchKey, matchValue string, labels map[string]string) *corev1.Service {
	matchMap := map[string]string{matchKey: matchValue}
	finalLabels := WithLabels(matchMap, labels)
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: finalLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{matchKey: matchValue},
		},
	}
}

// ValidateGVKs makes sure all resources have initialized GVKs
func ValidateGVKs(objects []runtime.Object) {
	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Version == "" || gvk.Kind == "" {
			panic(fmt.Sprintf("Uninitialized GVK for %+v", obj))
		}
	}
}

// WithLabels aggregates existing labels
func WithLabels(labels map[string]string, existing map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	for k, v := range existing {
		_, ok := labels[k]
		if !ok {
			labels[k] = v
		}
	}

	return labels
}
