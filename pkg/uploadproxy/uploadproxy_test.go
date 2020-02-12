package uploadproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

type httpClientConfig struct {
	key    []byte
	cert   []byte
	caCert []byte
}

type validateSuccess struct{}

type validateFailure struct{}

func (*validateSuccess) Validate(string) (*token.Payload, error) {
	return &token.Payload{
		Operation: token.OperationUpload,
		Name:      "testpvc",
		Namespace: "default",
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}, nil
}

func (*validateFailure) Validate(string) (*token.Payload, error) {
	return nil, fmt.Errorf("Bad token")
}

func getPublicKeyEncoded(t *testing.T) string {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	publicKeyPem, err := cert.EncodePublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(publicKeyPem)
}

func getHTTPClientConfig(t *testing.T) *httpClientConfig {
	caKeyPair, err := triple.NewCA("myca")
	if err != nil {
		panic("Error creating CA cert")
	}

	clientKeyPair, err := triple.NewClientKeyPair(caKeyPair, "testclient", []string{})
	if err != nil {
		panic("Error creating client cert")
	}
	return &httpClientConfig{
		key:    cert.EncodePrivateKeyPEM(clientKeyPair.Key),
		cert:   cert.EncodeCertPEM(clientKeyPair.Cert),
		caCert: cert.EncodeCertPEM(caKeyPair.Cert),
	}
}

func newProxyRequest(t *testing.T, authHeaderValue string) *http.Request {
	req, err := http.NewRequest("POST", common.UploadPathSync, strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	if authHeaderValue != "" {
		req.Header.Set("Authorization", authHeaderValue)
	}
	return req
}

func newProxyHeadRequest(t *testing.T, authHeaderValue string) *http.Request {
	req, err := http.NewRequest("HEAD", common.UploadPathSync, nil)
	if err != nil {
		t.Fatal(err)
	}
	if authHeaderValue != "" {
		req.Header.Set("Authorization", authHeaderValue)
	}
	return req
}

func submitRequestAndCheckStatus(t *testing.T, request *http.Request, expectedCode int, app *uploadProxyApp) {
	rr := httptest.NewRecorder()
	if app == nil {
		app = createApp()
	}

	app.ServeHTTP(rr, request)

	if rr.Code != expectedCode {
		t.Errorf("handler returned wrong status code: got %v want %v",
			rr.Code, expectedCode)
	}
}

func createApp() *uploadProxyApp {
	app := &uploadProxyApp{}
	app.initHandlers()
	return app
}

func TestGetSigningKey(t *testing.T) {
	publicKeyPEM := getPublicKeyEncoded(t)
	app := createApp()

	err := app.getSigningKey(publicKeyPEM)
	if err != nil {
		t.Errorf("Failed to parse public key pem")
	}

	if app.tokenValidator == nil {
		t.Errorf("Failed to create token validator")
	}
}

func TestGetUploadServerClient(t *testing.T) {
	certs := getHTTPClientConfig(t)
	certFetcher := &fetcher.MemCertFetcher{Cert: certs.cert, Key: certs.key}
	bundleFetcher := &fetcher.MemCertBundleFetcher{Bundle: certs.caCert}

	cc := &clientCreator{certFetcher: certFetcher, bundleFetcher: bundleFetcher}
	_, err := cc.CreateClient()
	if err != nil {
		t.Errorf("Failed to create http client")
	}
}

func TestMalformedAuthHeader(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
	}{
		{
			"invalid prefix",
			"Beereer valid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := newProxyRequest(t, test.headerValue)
			submitRequestAndCheckStatus(t, req, http.StatusBadRequest, nil)
		})
	}
}

type fakeClientCreator struct {
	client *http.Client
}

func (fcc *fakeClientCreator) CreateClient() (*http.Client, error) {
	return fcc.client, nil
}

func setupProxyTests(handler http.HandlerFunc) *uploadProxyApp {
	server := httptest.NewServer(handler)

	urlResolver := func(string, string, string) string {
		return server.URL
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpvc",
			Namespace: "default",
			Annotations: map[string]string{
				"cdi.kubevirt.io/storage.pod.phase": "Running",
				"cdi.kubevirt.io/storage.pod.ready": "true",
			},
		},
	}

	objects := []runtime.Object{}
	objects = append(objects, pvc)
	app := createApp()
	app.client = k8sfake.NewSimpleClientset(objects...)
	app.tokenValidator = &validateSuccess{}
	app.urlResolver = urlResolver
	app.clientCreator = &fakeClientCreator{client: server.Client()}

	return app
}

func TestProxy(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{
			"Test OK",
			http.StatusOK,
		},
		{
			"Test Error",
			http.StatusInternalServerError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := setupProxyTests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
			}))

			req := newProxyRequest(t, "Bearer valid")
			submitRequestAndCheckStatus(t, req, test.statusCode, app)
		})
	}
}

func TestHeadProxy(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{
			"Test OK",
			http.StatusOK,
		},
		{
			"Test Error",
			http.StatusInternalServerError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := setupProxyTests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
			}))

			req := newProxyHeadRequest(t, "Bearer valid")
			submitRequestAndCheckStatus(t, req, test.statusCode, app)
		})
	}
}
func TestTokenInvalid(t *testing.T) {
	app := createApp()
	app.tokenValidator = &validateFailure{}

	req := newProxyRequest(t, "Bearer valid")

	submitRequestAndCheckStatus(t, req, http.StatusUnauthorized, app)
}

func TestNoAuthHeader(t *testing.T) {
	req := newProxyRequest(t, "")
	submitRequestAndCheckStatus(t, req, http.StatusBadRequest, nil)
}

func TestHealthz(t *testing.T) {
	req, err := http.NewRequest("GET", healthzPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	submitRequestAndCheckStatus(t, req, http.StatusOK, nil)
}
