package apiserver

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	restful "github.com/emicklei/go-restful"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/cert/triple"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	aggregatorapifake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	cdiuploadv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/keys/keystest"
)

var foo aggregatorapifake.Clientset

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
			Namespace: "cdi",
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

func validateAPIServerConfig(t *testing.T, app *cdiAPIApp) {
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
		"cdi",
		"extension-apiserver-authentication")
}

func signingKeySecretGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		apiSigningKeySecretName)
}

func signingKeySecretCreateAction(privateKey *rsa.PrivateKey) core.Action {
	secret, _ := keystest.NewPrivateKeySecret("cdi", apiSigningKeySecretName, privateKey)
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		secret)
}

func tlsSecretGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		apiCertSecretName)
}

func tlsSecretCreateAction(privateKeyBytes, certBytes, caCertBytes []byte) core.Action {
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		keystest.NewTLSSecretFromBytes("cdi", apiCertSecretName, privateKeyBytes, certBytes, caCertBytes, nil))
}

func apiServiceGetAction() core.Action {
	return core.NewRootGetAction(
		schema.GroupVersionResource{
			Resource: "apiservices",
			Version:  "v1",
		},
		"v1alpha1.upload.kubevirt.io")
}

func getExpectedAPIService(certBytes []byte) *apiregistrationv1beta1.APIService {
	return &apiregistrationv1beta1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "v1alpha1.upload.cdi.kubevirt.io",
			Labels: map[string]string{
				"cdi.kubevirt.io": "cdi-api",
			},
		},
		Spec: apiregistrationv1beta1.APIServiceSpec{
			Service: &apiregistrationv1beta1.ServiceReference{
				Namespace: "cdi",
				Name:      "cdi-api",
			},
			Group:                "upload.cdi.kubevirt.io",
			Version:              "v1alpha1",
			CABundle:             certBytes,
			GroupPriorityMinimum: 1000,
			VersionPriority:      15,
		},
	}
}

func apiServiceCreateAction(certBytes []byte) core.Action {
	apiService := getExpectedAPIService(certBytes)
	return core.NewRootCreateAction(
		schema.GroupVersionResource{
			Resource: "apiservices",
			Version:  "v1",
		},
		apiService)
}

func apiServiceUpdateAction(certBytes []byte) core.Action {
	apiService := getExpectedAPIService(certBytes)
	return core.NewRootUpdateAction(
		schema.GroupVersionResource{
			Resource: "apiservices",
			Version:  "v1",
		},
		apiService)
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

	app := &cdiAPIApp{client: client, authorizer: newSuccessfulAuthorizer()}

	actions := []core.Action{}
	actions = append(actions, apiServerConfigMapGetAction())

	err := app.getClientCert()
	if err != nil {
		t.Errorf("getClientCert failed: %+v", err)
	}

	validateAPIServerConfig(t, app)

	checkActions(actions, client.Actions(), t)
}

func TestNewCdiAPIServer(t *testing.T) {
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, getAPIServerConfigMap(t))

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	aggregatorClient := aggregatorapifake.NewSimpleClientset()
	authorizer := &testAuthorizer{}

	server, err := NewCdiAPIServer("0.0.0.0", 0, client, aggregatorClient, authorizer)
	if err != nil {
		t.Errorf("Upload api server creation failed: %+v", err)
	}

	app := server.(*cdiAPIApp)

	req, _ := http.NewRequest("GET", "/apis", nil)
	rr := httptest.NewRecorder()

	app.container.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Unexpected status code %d", rr.Code)
	}
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

	serverKeyPair, err := triple.NewServerKeyPair(caKeyPair, "commonname", "service", "cdi", "cluster.local", []string{}, []string{})
	if err != nil {
		t.Errorf("Error creating server key pair")
	}

	signingKeySecret, err := keystest.NewPrivateKeySecret("cdi", apiSigningKeySecretName, signingKey)
	if err != nil {
		t.Errorf("error creating secret: %v", err)
	}

	tlsSecret := keystest.NewTLSSecret("cdi", apiCertSecretName, serverKeyPair, caKeyPair.Cert, nil)

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, tlsSecret)
	kubeobjects = append(kubeobjects, signingKeySecret)

	actions := []core.Action{}
	actions = append(actions, tlsSecretGetAction())
	actions = append(actions, signingKeySecretGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		client: client,
	}

	err = app.getSelfSignedCert()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestShouldGenerateCertsAndKeyFirstRun(t *testing.T) {
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		client: client,
	}

	err := app.getSelfSignedCert()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	actions := []core.Action{}
	actions = append(actions, tlsSecretGetAction())
	actions = append(actions, tlsSecretCreateAction(app.keyBytes, app.certBytes, app.signingCertBytes))
	actions = append(actions, signingKeySecretGetAction())
	actions = append(actions, signingKeySecretCreateAction(app.privateSigningKey))

	checkActions(actions, client.Actions(), t)
}

func TestCreateAPIService(t *testing.T) {
	kubeobjects := []runtime.Object{}

	aggregatorClient := aggregatorapifake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		aggregatorClient: aggregatorClient,
		signingCertBytes: []byte("data"),
	}

	err := app.createAPIService()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	actions := []core.Action{}
	actions = append(actions, apiServiceGetAction())
	actions = append(actions, apiServiceCreateAction(app.signingCertBytes))

	checkActions(actions, aggregatorClient.Actions(), t)
}

func TestUpdateAPIService(t *testing.T) {
	certBytes := []byte("data")
	service := getExpectedAPIService(certBytes)

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, service)

	aggregatorClient := aggregatorapifake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		aggregatorClient: aggregatorClient,
		signingCertBytes: certBytes,
	}

	err := app.createAPIService()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	actions := []core.Action{}
	actions = append(actions, apiServiceGetAction())
	actions = append(actions, apiServiceUpdateAction(app.signingCertBytes))

	checkActions(actions, aggregatorClient.Actions(), t)
}

func TestGetAPIResouceList(t *testing.T) {
	rr := doGetRequest(t, "/apis/upload.cdi.kubevirt.io/v1alpha1")

	resourceList := metav1.APIResourceList{}
	err := json.Unmarshal(rr.Body.Bytes(), &resourceList)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedResourceList := metav1.APIResourceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIResourceList",
			APIVersion: "v1",
		},
		GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
		APIResources: []metav1.APIResource{
			{
				Name:         "uploadtokenrequests",
				SingularName: "uploadtokenrequest",
				Namespaced:   true,
				Group:        "upload.cdi.kubevirt.io",
				Version:      "v1alpha1",
				Kind:         "UploadTokenRequest",
				Verbs:        []string{"create"},
				ShortNames:   []string{"utr", "utrs"},
			},
		},
	}

	checkEqual(t, expectedResourceList, resourceList)
}

func TestGetAPIGroup(t *testing.T) {
	rr := doGetRequest(t, "/apis/upload.cdi.kubevirt.io")

	apiGroup := metav1.APIGroup{}
	err := json.Unmarshal(rr.Body.Bytes(), &apiGroup)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedAPIGroup := getExpectedAPIGroup()

	checkEqual(t, expectedAPIGroup, apiGroup)
}
func TestGetAPIGroupList(t *testing.T) {
	rr := doGetRequest(t, "/apis")

	apiGroupList := metav1.APIGroupList{}
	err := json.Unmarshal(rr.Body.Bytes(), &apiGroupList)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedAPIGroupList := metav1.APIGroupList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIGroupList",
			APIVersion: "v1",
		},
		Groups: []metav1.APIGroup{
			getExpectedAPIGroup(),
		},
	}

	checkEqual(t, expectedAPIGroupList, apiGroupList)
}

func TestGetToken(t *testing.T) {
	type args struct {
		authorizer     CdiAPIAuthorizer
		pvc            *v1.PersistentVolumeClaim
		uploadPossible uploadPossibleFunc
	}

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	request := &cdiuploadv1alpha1.UploadTokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Spec: cdiuploadv1alpha1.UploadTokenRequestSpec{
			PvcName: "test-pvc",
		},
	}

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
		},
	}

	serializedRequest, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	authorizeSuccess := &testAuthorizer{allowed: true}

	tests := []struct {
		name           string
		args           args
		expectedStatus int
		checkToken     bool
	}{
		{
			"authoriser error",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "", err: fmt.Errorf("Error")},
			},
			http.StatusInternalServerError,
			false,
		},
		{
			"authoriser not allowed",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "bad person", err: nil},
			},
			http.StatusUnauthorized,
			false,
		},
		{
			"pvc does not exist",
			args{
				authorizer: authorizeSuccess,
			},
			http.StatusBadRequest,
			false,
		},
		{
			"upload not possible",
			args{
				authorizer:     authorizeSuccess,
				pvc:            pvc,
				uploadPossible: func(*v1.PersistentVolumeClaim) error { return fmt.Errorf("NOPE") },
			},
			http.StatusServiceUnavailable,
			false,
		},
		{
			"upload possible",
			args{
				authorizer:     authorizeSuccess,
				pvc:            pvc,
				uploadPossible: func(*v1.PersistentVolumeClaim) error { return nil },
			},
			http.StatusOK,
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			kubeobjects := []runtime.Object{}
			if test.args.pvc != nil {
				kubeobjects = append(kubeobjects, test.args.pvc)
			}
			client := k8sfake.NewSimpleClientset(kubeobjects...)

			app := &cdiAPIApp{client: client,
				privateSigningKey: signingKey,
				authorizer:        test.args.authorizer,
				uploadPossible:    test.args.uploadPossible}
			app.composeUploadTokenAPI()

			req, _ := http.NewRequest("POST",
				"/apis/upload.cdi.kubevirt.io/v1alpha1/namespaces/default/uploadtokenrequests",
				bytes.NewReader(serializedRequest))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app.container.ServeHTTP(rr, req)

			if rr.Code != test.expectedStatus {
				tt.Fatalf("Wrong status code, expected %d, got %d", test.expectedStatus, rr.Code)
			}

			if test.checkToken {
				uploadTokenRequest := &cdiuploadv1alpha1.UploadTokenRequest{}
				err := json.Unmarshal(rr.Body.Bytes(), &uploadTokenRequest)
				if err != nil {
					tt.Fatalf("Deserializing UploadTokenRequest failed: %+v", err)
				}

				if uploadTokenRequest.Status.Token == "" {
					tt.Fatal("UploadTokenRequest response does not contain a token")
				}
			}
		})
	}
}

func doGetRequest(t *testing.T, url string) *httptest.ResponseRecorder {
	app := &cdiAPIApp{}
	app.composeUploadTokenAPI()

	req, _ := http.NewRequest("GET", url, nil)
	rr := httptest.NewRecorder()

	app.container.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Wrong status code, expected %d, got %d", http.StatusOK, rr.Code)
	}

	return rr
}

func getExpectedAPIGroup() metav1.APIGroup {
	return metav1.APIGroup{
		Name: "upload.cdi.kubevirt.io",
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIGroup",
			APIVersion: "v1",
		},
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
			Version:      "v1alpha1",
		},
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
				Version:      "v1alpha1",
			},
		},
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: "",
			},
		},
	}
}

func checkEqual(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Objects are not equal\nDiff:\n %s", diff.ObjectGoPrintDiff(expected, actual))
	}
}
