package framework

import (
	"context"
	"runtime"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

var (
	//NodeSelectorTestValue is nodeSelector value for test
	NodeSelectorTestValue = map[string]string{"kubernetes.io/arch": runtime.GOARCH}
	//TolerationsTestValue is tolerations value for test
	TolerationsTestValue = []v1.Toleration{{Key: "test", Value: "123"}}
	//AffinityTestValue is affinity value for test
	AffinityTestValue = &v1.Affinity{}
)

// TestNodePlacementValues returns a pre-defined set of node placement values for testing purposes.
// The values chosen are valid, but the pod will likely not be schedulable.
func (f *Framework) TestNodePlacementValues() sdkapi.NodePlacement {
	nodes, _ := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

	nodeName := nodes.Items[0].Name
	for _, node := range nodes.Items {
		if _, hasLabel := node.Labels["node-role.kubernetes.io/worker"]; hasLabel {
			nodeName = node.Name
			break
		}
	}

	AffinityTestValue = &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{Key: "kubernetes.io/hostname", Operator: v1.NodeSelectorOpIn, Values: []string{nodeName}},
						},
					},
				},
			},
		},
	}

	return sdkapi.NodePlacement{
		NodeSelector: NodeSelectorTestValue,
		Affinity:     AffinityTestValue,
		Tolerations:  TolerationsTestValue,
	}
}
