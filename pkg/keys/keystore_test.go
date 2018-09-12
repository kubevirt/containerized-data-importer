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
	"reflect"
	"testing"

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

func checkActions(expected []core.Action, actual []core.Action, t *testing.T) {
	for i, action := range actual {
		if len(expected) < i+1 {
			t.Errorf("%d unexpected actions: %+v", len(actual)-len(expected), actual[i:])
			break
		}

		expectedAction := expected[i]
		checkAction(expectedAction, action, t)
	}

	if len(expected) != len(actual) {
		t.Errorf("Expected %d actions, got %d", len(expected), len(actual))
	}
}

func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expObject, object))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, expPatch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintDiff(expPatch, patch))
		}
	}
}

func TestCreateCA(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	keyPair, err := GetOrCreateCA(client, namespace, secret, "myca")
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))
	actions = append(actions, tlsSecretCreateAction(namespace, secret, keyPair, nil))

	checkActions(actions, client.Actions(), t)
}

func TestGetCA(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	tlsSecret := keystest.NewTLSSecret(namespace, secret, caKeyPair, nil, nil)
	kubeobjects = append(kubeobjects, tlsSecret)

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	keyPair, err := GetOrCreateCA(client, namespace, secret, "myca")
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))

	checkActions(actions, client.Actions(), t)

	if !reflect.DeepEqual(caKeyPair, keyPair) {
		t.Errorf("Keys do not match")
	}
}

func TestCreateServerCert(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	caKeyPair2, err := triple.NewCA("myca2")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	serverKeyPair, err := GetOrCreateServerKeyPairAndCert(client,
		namespace,
		secret,
		caKeyPair,
		caKeyPair2.Cert,
		"commonname",
		"service",
		nil,
	)
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))
	actions = append(actions, tlsSecretCreateAction(namespace, secret, &serverKeyPair.KeyPair, caKeyPair2.Cert))

	checkActions(actions, client.Actions(), t)
}

func TestGetServerCert(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	caKeyPair2, err := triple.NewCA("myca2")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	keyPair, err := triple.NewServerKeyPair(caKeyPair, "commonname", "service", namespace, "cluster.local", []string{}, []string{})
	if err != nil {
		t.Errorf("Error creating server key pair")
	}

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
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))

	checkActions(actions, client.Actions(), t)

	keyPairAndCert := &KeyPairAndCert{KeyPair: *keyPair, CACert: caKeyPair2.Cert}
	if !reflect.DeepEqual(keyPairAndCert, serverKeyPairAndCert) {
		t.Errorf("Keys do not match")
	}
}

func TestCreateClientCert(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	caKeyPair2, err := triple.NewCA("myca2")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	clientKeyPair, err := GetOrCreateClientKeyPairAndCert(client,
		namespace,
		secret,
		caKeyPair,
		caKeyPair2.Cert,
		"commonname",
		[]string{"myorg"},
		nil,
	)
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))
	actions = append(actions, tlsSecretCreateAction(namespace, secret, &clientKeyPair.KeyPair, caKeyPair2.Cert))

	checkActions(actions, client.Actions(), t)
}

func TestGetClientCert(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	caKeyPair2, err := triple.NewCA("myca2")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	keyPair, err := triple.NewClientKeyPair(caKeyPair, "commonname", []string{"myorg"})
	if err != nil {
		t.Errorf("Error creating server key pair")
	}

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
	if err != nil {
		t.Errorf("Error getting/creating CA %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))

	checkActions(actions, client.Actions(), t)

	keyPairAndCert := &KeyPairAndCert{KeyPair: *keyPair, CACert: caKeyPair2.Cert}
	if !reflect.DeepEqual(keyPairAndCert, serverKeyPairAndCert) {
		t.Errorf("Keys do not match")
	}
}

func TestCreatePrivateKey(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	privateKey, err := GetOrCreatePrivateKey(client, namespace, secret)
	if err != nil {
		t.Errorf("Error getting/creating private key %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))
	actions = append(actions, privateKeySecretCreateAction(namespace, secret, privateKey))

	checkActions(actions, client.Actions(), t)
}

func TestGetPrivateKey(t *testing.T) {
	namespace := "default"
	secret := "mysecret"
	kubeobjects := []runtime.Object{}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Errorf("Error creating private key")
	}

	privateKeySecret, err := keystest.NewPrivateKeySecret(namespace, secret, privateKey)
	if err != nil {
		t.Errorf("Error creating private key secret")
	}

	kubeobjects = append(kubeobjects, privateKeySecret)

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	returnedPrivateKey, err := GetOrCreatePrivateKey(client, namespace, secret)
	if err != nil {
		t.Errorf("Error getting/creating private key %+v", err)
	}

	actions := []core.Action{}
	actions = append(actions, secretGetAction(namespace, secret))

	checkActions(actions, client.Actions(), t)

	if !reflect.DeepEqual(privateKey, returnedPrivateKey) {
		t.Errorf("Keys do not match")
	}
}
