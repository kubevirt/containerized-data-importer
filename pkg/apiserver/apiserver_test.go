package apiserver

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/util/cert"

	. "kubevirt.io/containerized-data-importer/pkg/common"
)

func apiPublicKeyCreateAction(key *rsa.PublicKey) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		newConfigMap(APIPublicKeyConfigMap, "", key))
}

func apiPublicKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		APIPublicKeyConfigMap)
}

func apiPrivateKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		apiCertSecretName)
}

func apiPrivateKeyCreateAction(certBytes, keyBytes, signingCertBytes []byte) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		newAPISecret(apiCertSecretName, "kube-system", certBytes, keyBytes, signingCertBytes))
}

func newAPISecret(name string, namespace string, certBytes, keyBytes, signingCertBytes []byte) *v1.Secret {

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				CDI_COMPONENT_LABEL: "cdi-api-aggregator",
			},
		},
		Type: "Opaque",
		Data: map[string][]byte{
			certBytesValue:        certBytes,
			keyBytesValue:         keyBytes,
			signingCertBytesValue: signingCertBytes,
		},
	}
	return secret
}

func newConfigMap(name string, namespace string, key *rsa.PublicKey) *v1.ConfigMap {
	encodedKey, err := EncodePublicKey(key)
	if err != nil {
		panic("encode public key failed")
	}

	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				CDI_COMPONENT_LABEL: name,
			},
		},
		Data: map[string]string{
			"publicKey": encodedKey,
		},
	}
	if namespace != "" {
		config.Namespace = namespace
	}
	return config
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

func generateTestKey() (*rsa.PrivateKey, error) {
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return apiKeyPair, nil
}

func Test_tokenEncrption(t *testing.T) {
	apiServerKeys, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	encryptedToken, err := GenerateToken("fakepvc", "fakenamespace", apiServerKeys)

	if err != nil {
		t.Errorf("unable to generate token: %v", err)
	}

	tokenData, err := VerifyToken(encryptedToken, &apiServerKeys.PublicKey)

	if err != nil {
		t.Errorf("unable to verify token: %v", err)
	}

	if tokenData.PvcName != "fakepvc" && tokenData.Namespace != "fakenamespace" {
		t.Errorf("unexpected token generated")
	}
}

func TestKeyRetrieval(t *testing.T) {
	apiKeyPair, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	keyBytes := cert.EncodePrivateKeyPEM(apiKeyPair)
	certBytes := []byte("madeup")
	signingCertBytes := []byte("madeup")

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(APIPublicKeyConfigMap, "kube-system", &apiKeyPair.PublicKey))
	kubeobjects = append(kubeobjects, newAPISecret(apiCertSecretName, "kube-system", certBytes, keyBytes, signingCertBytes))

	actions := []core.Action{}
	actions = append(actions, apiPrivateKeyGetAction())
	actions = append(actions, apiPublicKeyGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadAPIApp{
		client: client,
	}

	err = app.getSelfSignedCert()
	if err != nil {
		t.Errorf("error creating upload proxy app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestShouldGenerateCertsAndKeyFirstRun(t *testing.T) {
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadAPIApp{
		client: client,
	}

	err := app.getSelfSignedCert()
	if err != nil {
		t.Errorf("error creating upload proxy app: %v", err)
	}

	obj, err := cert.ParsePrivateKeyPEM(app.keyBytes)
	privateKey, ok := obj.(*rsa.PrivateKey)
	if err != nil {
		t.Errorf("Error parsing private key: %v", err)
	}
	if !ok {
		t.Errorf("Unable to parse private key")
	}

	actions := []core.Action{}
	actions = append(actions, apiPrivateKeyGetAction())
	actions = append(actions, apiPrivateKeyCreateAction(app.certBytes, app.keyBytes, app.signingCertBytes))
	actions = append(actions, apiPublicKeyGetAction())
	actions = append(actions, apiPublicKeyCreateAction(&privateKey.PublicKey))

	checkActions(actions, client.Actions(), t)
}
