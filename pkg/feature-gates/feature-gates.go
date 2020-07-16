package featuregates

import (
	"context"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// HonorWaitForFirstConsumer - if enabled will not schedule worker pods on a storage with WaitForFirstConsumer binding mode
	HonorWaitForFirstConsumer = "HonorWaitForFirstConsumer"
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

func (f *FeatureGates) isFeatureGateEnabled(featureGate string) (bool, error) {
	featureGates, err := f.getConfig()
	if err != nil {
		return false, errors.Wrap(err, "error getting CDIConfig")
	}

	for _, fg := range featureGates {
		if fg == featureGate {
			return true, nil
		}
	}
	return false, nil
}

func (f *FeatureGates) getConfig() ([]string, error) {
	config := &cdiv1.CDIConfig{}
	if err := f.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return nil, err
	}

	return config.Spec.FeatureGates, nil
}

// HonorWaitForFirstConsumerEnabled - see HonorWaitForFirstConsumer const
func (f *FeatureGates) HonorWaitForFirstConsumerEnabled() (bool, error) {
	return f.isFeatureGateEnabled(HonorWaitForFirstConsumer)
}
