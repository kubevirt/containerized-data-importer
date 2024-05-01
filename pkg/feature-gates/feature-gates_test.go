/*
Copyright 2020 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package featuregates

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

var _ = Describe("Feature Gates", func() {
	It("Should be false if not set", func() {
		featureGates, _ := createFeatureGatesAndClient()
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeFalse())
	})

	It("Should reflect HonorWaitForFirstConsumer config changes", func() {
		featureGates, client := createFeatureGatesAndClient()
		cdiConfig := &cdiv1.CDIConfig{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		// update the config on the status not the spec
		cdiConfig.Spec.FeatureGates = []string{HonorWaitForFirstConsumer}
		err = client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeTrue())

		cdiConfig.Spec.FeatureGates = nil
		err = client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeFalse())
	})

	It("Should reflect DataVolumeClaimAdoption config changes", func() {
		featureGates, client := createFeatureGatesAndClient()
		cdiConfig := &cdiv1.CDIConfig{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		// update the config on the status not the spec
		cdiConfig.Spec.FeatureGates = []string{DataVolumeClaimAdoption}
		err = client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.ClaimAdoptionEnabled()).To(BeTrue())

		cdiConfig.Spec.FeatureGates = nil
		err = client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.ClaimAdoptionEnabled()).To(BeFalse())
	})
})

func createFeatureGatesAndClient(objects ...runtime.Object) (FeatureGates, client.Client) {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := &cdiv1.CDIConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDIConfig",
			APIVersion: "cdi.kubevirt.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ConfigName,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
	}
	objs = append(objs, cdiConfig)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	if err := cdiv1.AddToScheme(s); err != nil {
		panic(err)
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

	// Create a NewFeatureGates with fake client.
	return NewFeatureGates(cl), cl
}
