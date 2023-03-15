package framework

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	nodeSelectorTestValue = map[string]string{"kubernetes.io/arch": runtime.GOARCH}
	tolerationsTestValue  = []v1.Toleration{{Key: "test", Value: "123"}}
	affinityTestValue     = &v1.Affinity{}
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

	affinityTestValue = &v1.Affinity{
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
		NodeSelector: nodeSelectorTestValue,
		Affinity:     affinityTestValue,
		Tolerations:  tolerationsTestValue,
	}
}

// PodSpecHasTestNodePlacementValues compares if the pod spec has the set of node placement values defined for testing purposes
func (f *Framework) PodSpecHasTestNodePlacementValues(podSpec v1.PodSpec) bool {
	if !reflect.DeepEqual(podSpec.NodeSelector, nodeSelectorTestValue) {
		fmt.Printf("mismatched nodeSelectors, podSpec:\n%v\nExpected:\n%v\n", podSpec.NodeSelector, nodeSelectorTestValue)
		return false
	}
	if !reflect.DeepEqual(podSpec.Affinity, affinityTestValue) {
		fmt.Printf("mismatched affinity, podSpec:\n%v\nExpected:\n%v\n", *podSpec.Affinity, affinityTestValue)
		return false
	}
	foundMatchingTolerations := false
	for _, toleration := range podSpec.Tolerations {
		if toleration == tolerationsTestValue[0] {
			foundMatchingTolerations = true
		}
	}
	if !foundMatchingTolerations {
		fmt.Printf("no matching tolerations found. podSpec:\n%v\nExpected:\n%v\n", podSpec.Tolerations, tolerationsTestValue)
		return false
	}
	return true
}
