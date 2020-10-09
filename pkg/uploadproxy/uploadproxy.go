package uploadproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/cors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
)

const (
	healthzPath = "/healthz"

	waitReadyTime     = 10 * time.Second
	waitReadyImterval = time.Second

	proxyRequestTimeout = 24 * time.Hour

	uploadTokenLeeway = 10 * time.Second
)

// Server is the public interface to the upload proxy
type Server interface {
	Start() error
}

// CertWatcher is the interface for resources that watch certs
type CertWatcher interface {
	GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error)
}

// ClientCreator crates *http.Clients
type ClientCreator interface {
	CreateClient() (*http.Client, error)
}

type urlLookupFunc func(string, string, string) string
type uploadPossibleFunc func(*v1.PersistentVolumeClaim) error

type uploadProxyApp struct {
	bindAddress string
	bindPort    uint

	client kubernetes.Interface

	certWatcher CertWatcher

	clientCreator ClientCreator

	tokenValidator token.Validator

	handler http.Handler

	// test hooks
	urlResolver    urlLookupFunc
	uploadPossible uploadPossibleFunc
}

type clientCreator struct {
	certFetcher   fetcher.CertFetcher
	bundleFetcher fetcher.CertBundleFetcher
}

var authHeaderMatcher = regexp.MustCompile(`(?i)^Bearer\s+([A-Za-z0-9\-\._~\+\/]+)$`)

// NewUploadProxy returns an initialized uploadProxyApp
func NewUploadProxy(bindAddress string,
	bindPort uint,
	apiServerPublicKey string,
	certWatcher CertWatcher,
	clientCertFetcher fetcher.CertFetcher,
	serverCAFetcher fetcher.CertBundleFetcher,
	client kubernetes.Interface) (Server, error) {
	var err error
	app := &uploadProxyApp{
		bindAddress:    bindAddress,
		bindPort:       bindPort,
		certWatcher:    certWatcher,
		clientCreator:  &clientCreator{certFetcher: clientCertFetcher, bundleFetcher: serverCAFetcher},
		client:         client,
		urlResolver:    controller.GetUploadServerURL,
		uploadPossible: controller.UploadPossibleForPVC,
	}
	// retrieve RSA key used by apiserver to sign tokens
	err = app.getSigningKey(apiServerPublicKey)
	if err != nil {
		return nil, errors.Errorf("unable to retrieve apiserver signing key: %v", errors.WithStack(err))
	}

	app.initHandler()

	return app, nil
}

func (c *clientCreator) CreateClient() (*http.Client, error) {
	clientCertBytes, err := c.certFetcher.CertBytes()
	if err != nil {
		return nil, err
	}

	clientKeyBytes, err := c.certFetcher.KeyBytes()
	if err != nil {
		return nil, err
	}

	serverBundleBytes, err := c.bundleFetcher.BundleBytes()
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(serverBundleBytes) {
		klog.Error("Error parsing uploadserver CA bundle")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport, Timeout: proxyRequestTimeout}, nil
}

func (app *uploadProxyApp) initHandler() {
	mux := http.NewServeMux()
	mux.HandleFunc(healthzPath, app.handleHealthzRequest)
	for _, path := range common.ProxyPaths {
		mux.HandleFunc(path, app.handleUploadRequest)
	}
	app.handler = cors.AllowAll().Handler(mux)
}

func (app *uploadProxyApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.handler.ServeHTTP(w, r)
}

func (app *uploadProxyApp) handleHealthzRequest(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK")
}

func (app *uploadProxyApp) handleUploadRequest(w http.ResponseWriter, r *http.Request) {
	tokenHeader := r.Header.Get("Authorization")
	if tokenHeader == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	match := authHeaderMatcher.FindStringSubmatch(tokenHeader)
	if len(match) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tokenData, err := app.tokenValidator.Validate(match[1])
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if tokenData.Operation != token.OperationUpload ||
		tokenData.Name == "" ||
		tokenData.Namespace == "" ||
		tokenData.Resource.Resource != "persistentvolumeclaims" {
		klog.Errorf("Bad token %+v", tokenData)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(1).Infof("Received valid token: pvc: %s, namespace: %s", tokenData.Name, tokenData.Namespace)

	err = app.uploadReady(tokenData.Name, tokenData.Namespace)
	if err != nil {
		klog.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		// Return the error to the caller in the body.
		w.Write([]byte(err.Error()))
		return
	}

	app.proxyUploadRequest(tokenData.Namespace, tokenData.Name, w, r)
}

func (app *uploadProxyApp) uploadReady(pvcName, pvcNamespace string) error {
	return wait.PollImmediate(waitReadyImterval, waitReadyTime, func() (bool, error) {
		pvc, err := app.client.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, fmt.Errorf("rejecting Upload Request for PVC %s that doesn't exist", pvcName)
			}

			return false, err
		}

		err = app.uploadPossible(pvc)
		if err != nil {
			return false, err
		}
		phase := v1.PodPhase(pvc.Annotations[controller.AnnPodPhase])
		if phase == v1.PodSucceeded {
			return false, fmt.Errorf("rejecting Upload Request for PVC %s that already finished uploading", pvcName)
		}

		ready, _ := strconv.ParseBool(pvc.Annotations[controller.AnnPodReady])
		return ready, nil
	})
}

func (app *uploadProxyApp) proxyUploadRequest(namespace, pvc string, w http.ResponseWriter, r *http.Request) {
	client, err := app.clientCreator.CreateClient()
	if err != nil {
		klog.Error("Error creating http client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL, _ = url.Parse(app.urlResolver(namespace, pvc, r.URL.Path))
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		},
		Transport: client.Transport,
	}

	p.ServeHTTP(w, r)
}

func (app *uploadProxyApp) getSigningKey(publicKeyPEM string) error {
	publicKey, err := controller.DecodePublicKey([]byte(publicKeyPEM))
	if err != nil {
		return err
	}

	app.tokenValidator = token.NewValidator(common.UploadTokenIssuer, publicKey, uploadTokenLeeway)
	return nil
}

func (app *uploadProxyApp) Start() error {
	return app.startTLS()
}

func (app *uploadProxyApp) startTLS() error {
	var serveFunc func() error
	bindAddr := fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort)

	server := &http.Server{
		Addr:    bindAddr,
		Handler: app,
	}

	if app.certWatcher != nil {
		server.TLSConfig = &tls.Config{
			GetCertificate: app.certWatcher.GetCertificate,
		}

		serveFunc = func() error {
			return server.ListenAndServeTLS("", "")
		}
	} else {
		serveFunc = func() error {
			return server.ListenAndServe()
		}
	}

	errChan := make(chan error)

	go func() {
		errChan <- serveFunc()
	}()

	// wait for server to exit
	return <-errChan
}
