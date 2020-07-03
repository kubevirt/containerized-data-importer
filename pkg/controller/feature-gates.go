package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SkipWaitForFirstConsumerVolumes - if enabled will not schedule worker pods on a storage with WaitForFirstConsumer binding mode
	SkipWaitForFirstConsumerVolumes = "SkipWaitForFirstConsumerVolumes"
)

// FeatureGates is a util for determining whether an optional feature is enabled or not.
type FeatureGates struct {
	client client.Client
}

// NewFeatureGates creates a new instance of the feature gates
func NewFeatureGates(c client.Client) (*FeatureGates, error) {
	fg := &FeatureGates{client: c}
	return fg, nil
}

func (f *FeatureGates) isFeatureGateEnabled(featureGate string) bool {
	cfg, err := getConfig(f.client)
	if err != nil {
		// TODO: what to do here? kubevirt always has config in ClusterConfig object available
		return false
	}

	for _, fg := range cfg.Status.FeatureGates {
		if fg == featureGate {
			return true
		}
	}
	return false
}

func getConfig(client client.Client) (*cdiv1.CDIConfig, error) {
	config := &cdiv1.CDIConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return nil, err
	}
	return config, nil
}

// SkipWFFCVolumesEnabled - see SkipWaitForFirstConsumerVolumes const
func (f *FeatureGates) SkipWFFCVolumesEnabled() bool {
	return f.isFeatureGateEnabled(SkipWaitForFirstConsumerVolumes)
}
