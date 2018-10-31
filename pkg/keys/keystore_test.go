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
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/util/cert/triple"
	"kubevirt.io/containerized-data-importer/pkg/keys/keystest"
)

func tlsSecretCreateAction(namespace, secretName string, keyPair *triple.KeyPair, caCert *x509.Certificate) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		namespace,
		keystest.NewTLSSecret(namespace, secretName, keyPair, caCert, nil))
}

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

var _ = Describe("Create CA", func() {
	namespace := "default"
	secret := "mysecret"

	It("Should create a CA", func() {
		kubeobjects := []runtime.Object{}

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		By("Creating or getting a new CA")
		keyPair, err := GetOrCreateCA(client, namespace, secret, "myca")
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))
		actions = append(actions, tlsSecretCreateAction(namespace, secret, keyPair, nil))

		checkActions(actions, client.Actions())
	})

	It("Should get an existing CA", func() {
		kubeobjects := []runtime.Object{}

		caKeyPair, err := triple.NewCA("myca")
		Expect(err).NotTo(HaveOccurred())

		tlsSecret := keystest.NewTLSSecret(namespace, secret, caKeyPair, nil, nil)
		kubeobjects = append(kubeobjects, tlsSecret)

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		keyPair, err := GetOrCreateCA(client, namespace, secret, "myca")
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))

		checkActions(actions, client.Actions())

		if !reflect.DeepEqual(caKeyPair, keyPair) {
			Fail("Keys do not match")
		}
	})
})

var _ = Describe("Create Server Cert", func() {
	namespace := "default"
	secret := "mysecret"

	It("Should create a Server Cert", func() {
		kubeobjects := []runtime.Object{}

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		caKeyPair, err := triple.NewCA("myca")
		Expect(err).NotTo(HaveOccurred())

		caKeyPair2, err := triple.NewCA("myca2")
		Expect(err).NotTo(HaveOccurred())

		serverKeyPair, err := GetOrCreateServerKeyPairAndCert(client,
			namespace,
			secret,
			caKeyPair,
			caKeyPair2.Cert,
			"commonname",
			"service",
			nil,
		)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))
		actions = append(actions, tlsSecretCreateAction(namespace, secret, &serverKeyPair.KeyPair, caKeyPair2.Cert))

		checkActions(actions, client.Actions())
	})

	It("Should get an existing Server Cert", func() {
		kubeobjects := []runtime.Object{}

		caKeyPair, err := triple.NewCA("myca")
		Expect(err).NotTo(HaveOccurred())

		caKeyPair2, err := triple.NewCA("myca2")
		Expect(err).NotTo(HaveOccurred())

		keyPair, err := triple.NewServerKeyPair(caKeyPair, "commonname", "service", namespace, "cluster.local", []string{}, []string{})
		Expect(err).NotTo(HaveOccurred())

		tlsSecret := keystest.NewTLSSecret(namespace, secret, keyPair, caKeyPair2.Cert, nil)

		kubeobjects = append(kubeobjects, tlsSecret)

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		serverKeyPairAndCert, err := GetOrCreateServerKeyPairAndCert(client,
			namespace,
			secret,
			caKeyPair,
			caKeyPair2.Cert,
			"commonname",
			"service",
			nil,
		)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))

		checkActions(actions, client.Actions())

		keyPairAndCert := &KeyPairAndCert{KeyPair: *keyPair, CACert: caKeyPair2.Cert}
		if !reflect.DeepEqual(keyPairAndCert, serverKeyPairAndCert) {
			Fail("Keys do not match")
		}
	})
})

var _ = Describe("Create Client Cert", func() {
	namespace := "default"
	secret := "mysecret"

	It("Should create a Client Cert", func() {
		kubeobjects := []runtime.Object{}

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		caKeyPair, err := triple.NewCA("myca")
		Expect(err).NotTo(HaveOccurred())

		caKeyPair2, err := triple.NewCA("myca2")
		Expect(err).NotTo(HaveOccurred())

		clientKeyPair, err := GetOrCreateClientKeyPairAndCert(client,
			namespace,
			secret,
			caKeyPair,
			caKeyPair2.Cert,
			"commonname",
			[]string{"myorg"},
			nil,
		)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))
		actions = append(actions, tlsSecretCreateAction(namespace, secret, &clientKeyPair.KeyPair, caKeyPair2.Cert))

		checkActions(actions, client.Actions())
	})

	It("Should get an existing Client Cert", func() {
		kubeobjects := []runtime.Object{}

		caKeyPair, err := triple.NewCA("myca")
		Expect(err).NotTo(HaveOccurred())

		caKeyPair2, err := triple.NewCA("myca2")
		Expect(err).NotTo(HaveOccurred())

		keyPair, err := triple.NewClientKeyPair(caKeyPair, "commonname", []string{"myorg"})
		Expect(err).NotTo(HaveOccurred())

		tlsSecret := keystest.NewTLSSecret(namespace, secret, keyPair, caKeyPair2.Cert, nil)

		kubeobjects = append(kubeobjects, tlsSecret)

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		serverKeyPairAndCert, err := GetOrCreateClientKeyPairAndCert(client,
			namespace,
			secret,
			caKeyPair,
			caKeyPair2.Cert,
			"commonname",
			[]string{"myorg"},
			nil,
		)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))

		checkActions(actions, client.Actions())

		keyPairAndCert := &KeyPairAndCert{KeyPair: *keyPair, CACert: caKeyPair2.Cert}
		if !reflect.DeepEqual(keyPairAndCert, serverKeyPairAndCert) {
			Fail("Keys do not match")
		}
	})
})

var _ = Describe("Create Private Key", func() {
	namespace := "default"
	secret := "mysecret"

	It("Should create a Private Key", func() {
		kubeobjects := []runtime.Object{}

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		privateKey, err := GetOrCreatePrivateKey(client, namespace, secret)
		Expect(err).NotTo(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, secretGetAction(namespace, secret))
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

var _ = Describe("Self Signed Cert", func() {
	It("Should create a self signed cert, with a proper target directort", func() {
		tempDir, err := ioutil.TempDir("", "certs_test")
		defer os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())

		keyFile, certFile, err := GenerateSelfSignedCert(tempDir, "name", "namespace")
		Expect(err).NotTo(HaveOccurred())
		_, err = ioutil.ReadFile(keyFile)
		Expect(err).NotTo(HaveOccurred())
		_, err = ioutil.ReadFile(certFile)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should NOT create a self signed cert, with an invalid directort", func() {
		_, _, err := GenerateSelfSignedCert("/idontexistinthepath", "name", "namespace")
		Expect(err).To(HaveOccurred())
	})
})
