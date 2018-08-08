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
		newConfigMap(ApiPublicKeyConfigMap, "", key))
}

func apiPublicKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		ApiPublicKeyConfigMap)
}

func proxyPublicKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		UploadProxyPublicKeyConfigMap)
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
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				CDI_COMPONENT_LABEL: name,
			},
		},
		Data: map[string]string{
			"publicKey": EncodePublicKey(key),
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

func generateTestKeys() (*rsa.PrivateKey, *rsa.PrivateKey, error) {

	proxyKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	return proxyKeyPair, apiKeyPair, nil
}

func Test_tokenEncrption(t *testing.T) {
	proxyKeys, apiServerKeys, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	encryptedToken, err := GenerateEncryptedToken("fakepvc", "fakenamespace", &proxyKeys.PublicKey, apiServerKeys)

	if err != nil {
		t.Errorf("unable to encrypt token: %v", err)
	}

	tokenData, err := DecryptToken(encryptedToken, proxyKeys, &apiServerKeys.PublicKey)

	if err != nil {
		t.Errorf("unable to decrypt token: %v", err)
	}

	if tokenData.PvcName != "fakepvc" && tokenData.Namespace != "fakenamespace" {
		t.Errorf("unexpected token generated")
	}
}

func TestKeyRetrieval(t *testing.T) {
	proxyKeyPair, apiKeyPair, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	keyBytes := cert.EncodePrivateKeyPEM(apiKeyPair)
	certBytes := []byte("madeup")
	signingCertBytes := []byte("madeup")

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(ApiPublicKeyConfigMap, "kube-system", &apiKeyPair.PublicKey))
	kubeobjects = append(kubeobjects, newAPISecret(apiCertSecretName, "kube-system", certBytes, keyBytes, signingCertBytes))
	kubeobjects = append(kubeobjects, newConfigMap(UploadProxyPublicKeyConfigMap, "kube-system", &proxyKeyPair.PublicKey))

	actions := []core.Action{}
	actions = append(actions, apiPrivateKeyGetAction())
	actions = append(actions, apiPublicKeyGetAction())
	actions = append(actions, proxyPublicKeyGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadApiApp{
		client: client,
	}

	err = app.getSelfSignedCert()
	if err != nil {
		t.Errorf("error creating upload proxy app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestExpectErrIfProxyKeyNotFound(t *testing.T) {
	_, apiKeyPair, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	keyBytes := cert.EncodePrivateKeyPEM(apiKeyPair)
	certBytes := []byte("madeup")
	signingCertBytes := []byte("madeup")

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(ApiPublicKeyConfigMap, "kube-system", &apiKeyPair.PublicKey))
	kubeobjects = append(kubeobjects, newAPISecret(apiCertSecretName, "kube-system", certBytes, keyBytes, signingCertBytes))

	actions := []core.Action{}
	actions = append(actions, apiPrivateKeyGetAction())
	actions = append(actions, apiPublicKeyGetAction())
	actions = append(actions, proxyPublicKeyGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadApiApp{
		client: client,
	}

	err = app.getSelfSignedCert()
	if err == nil {
		t.Errorf("Expected err to have occurred because proxy public key is not present in cluster")
	}

	checkActions(actions, client.Actions(), t)
}

func TestShouldGenerateCertsAndKeyFirstRun(t *testing.T) {
	proxyKeyPair, _, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(UploadProxyPublicKeyConfigMap, "kube-system", &proxyKeyPair.PublicKey))

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadApiApp{
		client: client,
	}

	err = app.getSelfSignedCert()
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
	actions = append(actions, proxyPublicKeyGetAction())

	checkActions(actions, client.Actions(), t)
}
