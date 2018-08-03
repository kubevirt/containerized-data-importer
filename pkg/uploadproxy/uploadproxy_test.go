package uploadproxy

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

	apiserver "kubevirt.io/containerized-data-importer/pkg/apiserver"
	. "kubevirt.io/containerized-data-importer/pkg/common"
)

func proxyPublicKeyCreateAction(key *rsa.PublicKey) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		newConfigMap(apiserver.UploadProxyPublicKeyConfigMap, "", key))
}

func apiPublicKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		apiserver.ApiPublicKeyConfigMap)
}

func proxyPublicKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		apiserver.UploadProxyPublicKeyConfigMap)
}
func proxyPrivateKeyGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		apiserver.UploadProxySecretName)
}

func proxyPrivateKeyCreateAction(key *rsa.PrivateKey) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		newSecret(apiserver.UploadProxySecretName, "", key))
}

func newSecret(name string, namespace string, key *rsa.PrivateKey) *v1.Secret {

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				CDI_COMPONENT_LABEL: name,
			},
		},
		Data: map[string][]byte{
			"privateKey": []byte(apiserver.EncodePrivateKey(key)),
		},
	}
	if namespace != "" {
		secret.Namespace = namespace
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
			"publicKey": apiserver.EncodePublicKey(key),
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

func TestKeyRetrieval(t *testing.T) {
	proxyKeyPair, apiKeyPair, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(apiserver.ApiPublicKeyConfigMap, "kube-system", &apiKeyPair.PublicKey))
	kubeobjects = append(kubeobjects, newSecret(apiserver.UploadProxySecretName, "kube-system", proxyKeyPair))
	kubeobjects = append(kubeobjects, newConfigMap(apiserver.UploadProxyPublicKeyConfigMap, "kube-system", &proxyKeyPair.PublicKey))

	actions := []core.Action{}
	actions = append(actions, proxyPrivateKeyGetAction())
	actions = append(actions, proxyPublicKeyGetAction())
	actions = append(actions, apiPublicKeyGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	_, err = NewUploadProxy("0.0.0.0", 8443, client)

	if err != nil {
		t.Errorf("error creating upload proxy app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestPublicKeyCreation(t *testing.T) {
	proxyKeyPair, apiKeyPair, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newConfigMap(apiserver.ApiPublicKeyConfigMap, "kube-system", &apiKeyPair.PublicKey))
	kubeobjects = append(kubeobjects, newSecret(apiserver.UploadProxySecretName, "kube-system", proxyKeyPair))

	actions := []core.Action{}
	actions = append(actions, proxyPrivateKeyGetAction())
	actions = append(actions, proxyPublicKeyGetAction())
	actions = append(actions, proxyPublicKeyCreateAction(&proxyKeyPair.PublicKey))

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	app := &uploadProxyApp{
		client: client,
	}

	err = app.generateKeys()
	if err != nil {
		t.Errorf("error creating upload proxy app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestExpectErrorIfAPIKeyNotFound(t *testing.T) {
	proxyKeyPair, _, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, newSecret(apiserver.UploadProxySecretName, "kube-system", proxyKeyPair))

	actions := []core.Action{}
	actions = append(actions, proxyPrivateKeyGetAction())
	actions = append(actions, proxyPublicKeyGetAction())
	actions = append(actions, proxyPublicKeyCreateAction(&proxyKeyPair.PublicKey))
	actions = append(actions, apiPublicKeyGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	_, err = NewUploadProxy("0.0.0.0", 8443, client)

	if err == nil {
		t.Errorf("Expected err to have occurred because API public key is not present in cluster")
	}

	checkActions(actions, client.Actions(), t)
}

func TestGenerateSelfSignedCert(t *testing.T) {

	app := uploadProxyApp{}
	err := app.generateSelfSignedCert()
	if err != nil {

		t.Errorf("failed to generate self signed cert: %v", err)
	}

	if len(app.keyBytes) == 0 || len(app.certBytes) == 0 {
		t.Errorf("failed to generate self signed cert, bytes are empty")
	}

}
