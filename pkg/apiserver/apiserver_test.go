package apiserver

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"kubevirt.io/containerized-data-importer/pkg/keys"
)

func signingKeySecretGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		apiSigningKeySecretName)
}

func signingKeySecretCreateAction(privateKey *rsa.PrivateKey) core.Action {
	secret, _ := keys.NewPrivateKeySecret("kube-system", apiSigningKeySecretName, privateKey)
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		secret)
}

func tlsSecretGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		apiCertSecretName)
}

func tlsSecretCreateAction(privateKeyBytes, certBytes, caCertBytes []byte) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"kube-system",
		keys.NewTLSSecretFromBytes("kube-system", apiCertSecretName, privateKeyBytes, certBytes, caCertBytes, nil))
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
	signingKey, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	keyBytes := []byte("madeup")
	certBytes := []byte("madeup")
	signingCertBytes := []byte("madeup")

	signingKeySecret, err := keys.NewPrivateKeySecret("kube-system", apiSigningKeySecretName, signingKey)
	if err != nil {
		t.Errorf("error creating secret: %v", err)
	}

	tlsSecret := keys.NewTLSSecretFromBytes("kube-system", apiCertSecretName, keyBytes, certBytes, signingCertBytes, nil)

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, tlsSecret)
	kubeobjects = append(kubeobjects, signingKeySecret)

	actions := []core.Action{}
	actions = append(actions, tlsSecretGetAction())
	actions = append(actions, signingKeySecretGetAction())

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

	actions := []core.Action{}
	actions = append(actions, tlsSecretGetAction())
	actions = append(actions, tlsSecretCreateAction(app.keyBytes, app.certBytes, app.signingCertBytes))
	actions = append(actions, tlsSecretGetAction())
	actions = append(actions, signingKeySecretGetAction())
	actions = append(actions, signingKeySecretCreateAction(app.privateSigningKey))

	checkActions(actions, client.Actions(), t)
}
