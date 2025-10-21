package namespaced

import (
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	allowIngressToMetrics          = "cdi-allow-ingress-to-metrics"
	allowUploadProxyCommunications = "cdi-allow-uploadproxy-communications"
	allowIngressToCdiAPIWebhook    = "cdi-allow-cdi-api-webhook-server"
	allowEgressToImporterMetrics   = "cdi-allow-cdi-deployment-importer-metrics"
	allowEgressFromPoller          = "cdi-allow-egress-from-poller"
)

func createNetworkPolicies(args *FactoryArgs) []client.Object {
	return []client.Object{
		newIngressToMetricsNP(args.Namespace),
		newUploadProxyCommunicationsNP(args.Namespace),
		newIngressToCdiAPIWebhookNP(args.Namespace),
		newCdiDeploymentToImporterMetricsNP(args.Namespace),
		newEgressFromPollerNP(args.Namespace),
	}
}

func newNetworkPolicy(namespace, name string, spec *networkv1.NetworkPolicySpec) *networkv1.NetworkPolicy {
	return &networkv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{common.CDIComponentLabel: ""},
		},
		Spec: *spec,
	}
}

func newIngressToMetricsNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowIngressToMetrics,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{common.PrometheusLabelKey: common.PrometheusLabelValue},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func newUploadProxyCommunicationsNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowUploadProxyCommunications,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{common.CDIComponentLabel: common.CDIUploadProxyResourceName},
			},
			PolicyTypes: []networkv1.PolicyType{
				networkv1.PolicyTypeIngress,
				networkv1.PolicyTypeEgress,
			},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{common.CDIComponentLabel: common.UploadServerCDILabel},
							},
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func newIngressToCdiAPIWebhookNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowIngressToCdiAPIWebhook,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{common.CDIComponentLabel: common.CDIApiServerResourceName},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeIngress},
			Ingress: []networkv1.NetworkPolicyIngressRule{
				{
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func newCdiDeploymentToImporterMetricsNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowEgressToImporterMetrics,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{common.CDIComponentLabel: common.CDIControllerResourceName},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{common.PrometheusLabelKey: common.PrometheusLabelValue},
							},
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						{
							Port:     ptr.To(intstr.FromInt32(8443)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			},
		},
	)
}

func newEgressFromPollerNP(namespace string) *networkv1.NetworkPolicy {
	return newNetworkPolicy(
		namespace,
		allowEgressFromPoller,
		&networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{common.DataImportCronPollerLabel: ""},
			},
			PolicyTypes: []networkv1.PolicyType{networkv1.PolicyTypeEgress},
			Egress:      []networkv1.NetworkPolicyEgressRule{{}},
		},
	)
}
