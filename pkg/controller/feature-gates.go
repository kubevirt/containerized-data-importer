package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

const (
	// SkipWaitForFirstConsumerVolumes - if enabled will not schedule worker pods on a storage with WaitForFirstConsumer binding mode
	SkipWaitForFirstConsumerVolumes = "SkipWaitForFirstConsumerVolumes"
)

// FeatureGates is a util for determining whether an optional feature is enabled or not.
type FeatureGates struct {
	client               client.Client
	lock                 *sync.Mutex
	lasValidFeatureGates []string
}

// NewFeatureGates creates a new instance of the feature gates
func NewFeatureGates(c client.Client) (*FeatureGates, error) {
	fg := &FeatureGates{
		client: c,
		lock:   &sync.Mutex{},
	}
	return fg, nil
}

func (f *FeatureGates) isFeatureGateEnabled(featureGate string) bool {
	featureGates := f.getConfig()

	for _, fg := range featureGates {
		if fg == featureGate {
			return true
		}
	}
	return false
}

func (f *FeatureGates) getConfig() []string {
	f.lock.Lock()
	defer f.lock.Unlock()

	config := &cdiv1.CDIConfig{}
	if err := f.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, config); err != nil {
		return f.lasValidFeatureGates
	}
	f.lasValidFeatureGates = config.Spec.FeatureGates
	return f.lasValidFeatureGates
}

// SkipWFFCVolumesEnabled - see SkipWaitForFirstConsumerVolumes const
func (f *FeatureGates) SkipWFFCVolumesEnabled() bool {
	return f.isFeatureGateEnabled(SkipWaitForFirstConsumerVolumes)
}
