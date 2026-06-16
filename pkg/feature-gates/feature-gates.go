package featuregates

import (
	"context"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// HonorWaitForFirstConsumer - if enabled will not schedule worker pods on a storage with WaitForFirstConsumer binding mode
	HonorWaitForFirstConsumer = "HonorWaitForFirstConsumer"

	// DataVolumeClaimAdoption - if enabled will allow PVC to be adopted by a DataVolume
	// it is not an error if PVC of sam name exists before DataVolume is created
	DataVolumeClaimAdoption = "DataVolumeClaimAdoption"

	// WebhookPvcRendering is the legacy feature gate string for PVC webhook rendering
	// Deprecated: This feature is now always enabled
	// The string is kept for backward compatibility with existing CDI CRs that list it in featureGates
	// Use CDIConfigSpec.DisableWebhookPvcRendering to disable if needed
	WebhookPvcRendering = "WebhookPvcRendering"
)

// FeatureGates is a util for determining whether an optional feature is enabled or not.
type FeatureGates interface {
	// HonorWaitForFirstConsumerEnabled - see the HonorWaitForFirstConsumer const
	HonorWaitForFirstConsumerEnabled() (bool, error)

	// ClaimAdoptionEnabled - see the DataVolumeClaimAdoption const
	ClaimAdoptionEnabled() (bool, error)

	// WebhookPvcRenderingEnabled - see the WebhookPvcRendering const
	WebhookPvcRenderingEnabled() (bool, error)
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
	featureGates, err := f.getFeatureGates()
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

// HonorWaitForFirstConsumerEnabled - see the HonorWaitForFirstConsumer const
func (f *CDIConfigFeatureGates) HonorWaitForFirstConsumerEnabled() (bool, error) {
	return f.isFeatureGateEnabled(HonorWaitForFirstConsumer)
}

// ClaimAdoptionEnabled - see the DataVolumeClaimAdoption const
func (f *CDIConfigFeatureGates) ClaimAdoptionEnabled() (bool, error) {
	return f.isFeatureGateEnabled(DataVolumeClaimAdoption)
}

func (f *CDIConfigFeatureGates) getCDIConfig() (*cdiv1.CDIConfig, error) {
	config := &cdiv1.CDIConfig{}
	if err := f.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (f *CDIConfigFeatureGates) getFeatureGates() ([]string, error) {
	config, err := f.getCDIConfig()
	if err != nil {
		return nil, err
	}
	return config.Spec.FeatureGates, nil
}

// WebhookPvcRenderingEnabled tells if webhook PVC rendering is enabled
// The webhook is enabled by default, it is only disabled when
// CDIConfigSpec.DisableWebhookPvcRendering is set to true
func (f *CDIConfigFeatureGates) WebhookPvcRenderingEnabled() (bool, error) {
	config, err := f.getCDIConfig()
	if err != nil {
		return false, errors.Wrap(err, "error getting CDIConfig")
	}
	if config.Spec.DisableWebhookPvcRendering != nil && *config.Spec.DisableWebhookPvcRendering {
		return false, nil
	}
	return true, nil
}

// IsWebhookPvcRenderingEnabled tells if webhook PVC rendering is enabled
func IsWebhookPvcRenderingEnabled(c client.Client) (bool, error) {
	gates := NewFeatureGates(c)
	return gates.WebhookPvcRenderingEnabled()
}
