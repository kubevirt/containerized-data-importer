package featuregates

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// HonorWaitForFirstConsumer - if enabled will not schedule worker pods on a storage with WaitForFirstConsumer binding mode
	HonorWaitForFirstConsumer = "HonorWaitForFirstConsumer"
)

// FeatureGates is a util for determining whether an optional feature is enabled or not.
type FeatureGates interface {
	// HonorWaitForFirstConsumerEnabled - see the HonorWaitForFirstConsumer const
	HonorWaitForFirstConsumerEnabled() (bool, error)
}

// CDIConfigFeatureGates is a util for determining whether an optional feature is enabled or not.
type CDIConfigFeatureGates struct {
	client client.Client
}

// NewFeatureGates creates a new instance of the feature gates
func NewFeatureGates(c client.Client) *CDIConfigFeatureGates {
	return &CDIConfigFeatureGates{client: c}
}

func (f *CDIConfigFeatureGates) isFeatureGateEnabled(featureGate string) (bool, error) {
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

func (f *CDIConfigFeatureGates) getConfig() ([]string, error) {
	config := &cdiv1.CDIConfig{}
	if err := f.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return nil, err
	}

	return config.Spec.FeatureGates, nil
}

// HonorWaitForFirstConsumerEnabled - see the HonorWaitForFirstConsumer const
func (f *CDIConfigFeatureGates) HonorWaitForFirstConsumerEnabled() (bool, error) {
	return f.isFeatureGateEnabled(HonorWaitForFirstConsumer)
}
