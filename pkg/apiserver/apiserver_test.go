package apiserver

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	restful "github.com/emicklei/go-restful"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/util/cert/triple"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"kubevirt.io/containerized-data-importer/pkg/keys/keystest"
)

type testAuthorizer struct {
	allowed            bool
	reason             string
	err                error
	userHeaders        []string
	groupHeaders       []string
	extraPrefixHeaders []string
}

func (a *testAuthorizer) Authorize(req *restful.Request) (bool, string, error) {
	return a.allowed, a.reason, a.err
}

func (a *testAuthorizer) AddUserHeaders(header []string) {
	a.userHeaders = append(a.userHeaders, header...)
}

func (a *testAuthorizer) GetUserHeaders() []string {
	return a.userHeaders
}

func (a *testAuthorizer) AddGroupHeaders(header []string) {
	a.groupHeaders = append(a.groupHeaders, header...)
}

func (a *testAuthorizer) GetGroupHeaders() []string {
	return a.groupHeaders
}

func (a *testAuthorizer) AddExtraPrefixHeaders(header []string) {
	a.extraPrefixHeaders = append(a.extraPrefixHeaders, header...)
}

func (a *testAuthorizer) GetExtraPrefixHeaders() []string {
	return a.extraPrefixHeaders
}

func newSuccessfulAuthorizer() CdiAPIAuthorizer {
	return &testAuthorizer{true, "", nil, []string{}, []string{}, []string{}}
}

func newFailureAuthorizer() CdiAPIAuthorizer {
	return &testAuthorizer{false, "You are a bad person", fmt.Errorf("Not authorized"), []string{}, []string{}, []string{}}
}

func getAPIServerConfigMap(t *testing.T) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "extension-apiserver-authentication",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"client-ca-file":                     "bunchofbytes",
			"requestheader-allowed-names":        "[\"front-proxy-client\"]",
			"requestheader-client-ca-file":       "morebytes",
			"requestheader-extra-headers-prefix": "[\"X-Remote-Extra-\"]",
			"requestheader-group-headers":        "[\"X-Remote-Group\"]",
			"requestheader-username-headers":     "[\"X-Remote-User\"]",
		},
	}
}

func validateAPIServerConfig(t *testing.T, app *uploadAPIApp) {
	if string(app.clientCABytes) != "bunchofbytes" {
		t.Errorf("no match on client-ca-file")
	}

	if string(app.requestHeaderClientCABytes) != "morebytes" {
		t.Errorf("no match on requestheader-client-ca-file")
	}

	if !reflect.DeepEqual(app.authorizer.GetExtraPrefixHeaders(), []string{"X-Remote-Extra-"}) {
		t.Errorf("no match on requestheader-extra-headers-prefix")
	}

	if !reflect.DeepEqual(app.authorizer.GetGroupHeaders(), []string{"X-Remote-Group"}) {
		t.Logf("%+v", app.authorizer.GetGroupHeaders())
		t.Errorf("requestheader-group-headers")
	}

	if !reflect.DeepEqual(app.authorizer.GetUserHeaders(), []string{"X-Remote-User"}) {
		t.Logf("%+v", app.authorizer.GetUserHeaders())
		t.Errorf("requestheader-username-headers")
	}
}

func apiServerConfigMapGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"kube-system",
		"extension-apiserver-authentication")
}

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
	secret, _ := keystest.NewPrivateKeySecret("kube-system", apiSigningKeySecretName, privateKey)
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
		keystest.NewTLSSecretFromBytes("kube-system", apiCertSecretName, privateKeyBytes, certBytes, caCertBytes, nil))
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

func TestGetClientCert(t *testing.T) {
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, getAPIServerConfigMap(t))

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &uploadAPIApp{client: client, authorizer: newSuccessfulAuthorizer()}

	actions := []core.Action{}
	actions = append(actions, apiServerConfigMapGetAction())

	err := app.getClientCert()
	if err != nil {
		t.Errorf("getClientCert failed: %+v", err)
	}

	validateAPIServerConfig(t, app)

	checkActions(actions, client.Actions(), t)
}

func TestGetSelfSignedCert(t *testing.T) {
	signingKey, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		t.Errorf("Error creating CA key pair")
	}

	serverKeyPair, err := triple.NewServerKeyPair(caKeyPair, "commonname", "service", "kube-system", "cluster.local", []string{}, []string{})
	if err != nil {
		t.Errorf("Error creating server key pair")
	}

	signingKeySecret, err := keystest.NewPrivateKeySecret("kube-system", apiSigningKeySecretName, signingKey)
	if err != nil {
		t.Errorf("error creating secret: %v", err)
	}

	tlsSecret := keystest.NewTLSSecret("kube-system", apiCertSecretName, serverKeyPair, caKeyPair.Cert, nil)

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
	actions = append(actions, signingKeySecretGetAction())
	actions = append(actions, signingKeySecretCreateAction(app.privateSigningKey))

	checkActions(actions, client.Actions(), t)
}

func TestGetRequest(t *testing.T) {
	rr := doGetRequest(t, "/apis/upload.cdi.kubevirt.io/v1alpha1")

	resourceList := metav1.APIResourceList{}
	err := json.Unmarshal(rr.Body.Bytes(), &resourceList)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}
}

func doGetRequest(t *testing.T, url string) *httptest.ResponseRecorder {
	app := &uploadAPIApp{}
	app.composeUploadTokenAPI()

	req, _ := http.NewRequest("GET", url, nil)
	rr := httptest.NewRecorder()

	restful.DefaultContainer.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Wrong status code, expected %d, got %d", http.StatusOK, rr.Code)
	}

	return rr
}
