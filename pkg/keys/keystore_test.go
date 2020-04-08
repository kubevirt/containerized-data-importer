/*
Copyright 2018 The CDI Authors.

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

package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"kubevirt.io/containerized-data-importer/pkg/keys/keystest"
)

func privateKeySecretCreateAction(namespace, secretName string, privateKey *rsa.PrivateKey) core.Action {
	secret, _ := keystest.NewPrivateKeySecret(namespace, secretName, privateKey)
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		namespace,
		secret)
}

func secretGetAction(namespace, secretName string) core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		namespace,
		secretName)
}

func cdiConfigGetAction(namespace string) core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		namespace,
		"cdi-config")
}

func checkActions(expected []core.Action, actual []core.Action) {
	for i, action := range actual {
		if len(expected) < i+1 {
			Fail(fmt.Sprintf("%d unexpected actions: %+v", len(actual)-len(expected), actual[i:]))
			break
		}

		expectedAction := expected[i]
		checkAction(expectedAction, action)
	}

	if len(expected) != len(actual) {
		Fail(fmt.Sprintf("Expected %d actions, got %d", len(expected), len(actual)))
	}
}

func checkAction(expected, actual core.Action) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		Fail(fmt.Sprintf("Expected\n\t%#v\ngot\n\t%#v", expected, actual))
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		Fail(fmt.Sprintf("Action has wrong type. Expected: %t. Got: %t", expected, actual))
		return
	}

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			Fail(fmt.Sprintf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object)))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			Fail(fmt.Sprintf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object)))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, expPatch) {
			Fail(fmt.Sprintf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch)))
		}
	}
}

var _ = Describe("Create Private Key", func() {
	namespace := "default"
	secret := "mysecret"

	It("Should create a Private Key", func() {
		client := k8sfake.NewSimpleClientset()

		privateKey, err := GetOrCreatePrivateKey(client, namespace, secret)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))
		actions = append(actions, cdiConfigGetAction(namespace))
		actions = append(actions, privateKeySecretCreateAction(namespace, secret, privateKey))

		checkActions(actions, client.Actions())
	})

	It("Should get an existing Private Key", func() {
		kubeobjects := []runtime.Object{}

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())

		privateKeySecret, err := keystest.NewPrivateKeySecret(namespace, secret, privateKey)
		Expect(err).NotTo(HaveOccurred())

		kubeobjects = append(kubeobjects, privateKeySecret)

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		returnedPrivateKey, err := GetOrCreatePrivateKey(client, namespace, secret)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))

		checkActions(actions, client.Actions())

		if !reflect.DeepEqual(privateKey, returnedPrivateKey) {
			Fail("Keys do not match")
		}
	})
})
