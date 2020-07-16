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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	log = logf.Log.WithName("feature-gates-test")
)
var _ = Describe("Feature Gates", func() {
	It("Should be false if not set", func() {
		featureGates := createFeatureGates()
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeFalse())
	})

	It("Should reflect config changes", func() {
		featureGates := createFeatureGates()
		cdiConfig := &cdiv1.CDIConfig{}
		err := featureGates.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		// update the config on the status not the spec
		cdiConfig.Spec.FeatureGates = []string{HonorWaitForFirstConsumer}
		err = featureGates.client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeTrue())

		cdiConfig.Spec.FeatureGates = nil
		err = featureGates.client.Update(context.TODO(), cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(featureGates.HonorWaitForFirstConsumerEnabled()).To(BeFalse())
	})
})

func createFeatureGates(objects ...runtime.Object) *FeatureGates {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := controller.MakeEmptyCDIConfigSpec(common.ConfigName)
	objs = append(objs, cdiConfig)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a NewFeatureGates with fake client.
	f, _ := NewFeatureGates(cl)

	return f
}
