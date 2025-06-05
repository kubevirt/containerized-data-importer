package uploadproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/cors"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
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

	cdiConfigTLSWatcher cryptowatch.CdiConfigTLSWatcher

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
	cdiConfigTLSWatcher cryptowatch.CdiConfigTLSWatcher,
	certWatcher CertWatcher,
	clientCertFetcher fetcher.CertFetcher,
	serverCAFetcher fetcher.CertBundleFetcher,
	client kubernetes.Interface) (Server, error) {
	var err error
	app := &uploadProxyApp{
		bindAddress:         bindAddress,
		bindPort:            bindPort,
		cdiConfigTLSWatcher: cdiConfigTLSWatcher,
		certWatcher:         certWatcher,
		clientCreator:       &clientCreator{certFetcher: clientCertFetcher, bundleFetcher: serverCAFetcher},
		client:              client,
		urlResolver:         controller.GetUploadServerURL,
		uploadPossible:      controller.UploadPossibleForPVC,
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
		MinVersion:   tls.VersionTLS12,
	}
	tlsConfig.BuildNameToCertificate() //nolint:staticcheck // todo: BuildNameToCertificate() is deprecated - check this

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
	_, err := io.WriteString(w, "OK")
	if err != nil {
		klog.Errorf("handleHealthzRequest: failed to send response; %v", err)
	}
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

	pvc, err := app.uploadReady(tokenData.Name, tokenData.Namespace)
	if err != nil {
		klog.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		// Return the error to the caller in the body.
		_, err = fmt.Fprint(w, html.EscapeString(err.Error()))
		if err != nil {
			klog.Errorf("handleUploadRequest: failed to send error response: %v", err)
		}
		return
	}

	uploadPath, err := app.resolveUploadPath(pvc, tokenData.Name, r.URL.Path)
	if err != nil {
		klog.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		// Return the error to the caller in the body.
		_, err = fmt.Fprint(w, html.EscapeString(err.Error()))
		if err != nil {
			klog.Errorf("handleUploadRequest: failed to send error response: %v", err)
		}
		return
	}

	app.proxyUploadRequest(uploadPath, w, r)
}

func (app *uploadProxyApp) resolveUploadPath(pvc *v1.PersistentVolumeClaim, pvcName, defaultPath string) (string, error) {
	var path string
	contentType := pvc.Annotations[cc.AnnContentType]
	switch contentType {
	case string(cdiv1.DataVolumeKubeVirt), "":
		path = defaultPath
	case string(cdiv1.DataVolumeArchive):
		if strings.Contains(defaultPath, "alpha") {
			path = common.UploadArchiveAlphaPath
		} else {
			path = common.UploadArchivePath
		}
	default:
		// Caller is escaping user-controlled strings to avoid cross-site scripting (XSS) attacks
		return "", fmt.Errorf("rejecting upload request for PVC %s - upload content-type %s is invalid", pvcName, contentType)
	}

	return app.urlResolver(pvc.Namespace, pvc.Name, path), nil
}

func (app *uploadProxyApp) uploadReady(pvcName, pvcNamespace string) (*v1.PersistentVolumeClaim, error) {
	var pvc *v1.PersistentVolumeClaim
	err := wait.PollUntilContextTimeout(context.TODO(), waitReadyImterval, waitReadyTime, true, func(ctx context.Context) (bool, error) {
		var err error
		pvc, err = app.client.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, fmt.Errorf("rejecting Upload Request for PVC %s that doesn't exist", pvcName)
			}

			return false, err
		}
		// If using upload populator then need to check upload possibility to the PVC'
		if populators.IsPVCDataSourceRefKind(pvc, cdiv1.VolumeUploadSourceRef) {
			pvc, err = app.getPopulationPVC(ctx, pvc, pvcNamespace)
			if pvc == nil || err != nil {
				return false, err
			}
		}

		err = app.uploadPossible(pvc)
		if err != nil {
			return false, err
		}
		phase := v1.PodPhase(pvc.Annotations[cc.AnnPodPhase])
		if phase == v1.PodSucceeded {
			return false, fmt.Errorf("rejecting Upload Request for PVC %s that already finished uploading", pvcName)
		}

		ready, _ := strconv.ParseBool(pvc.Annotations[cc.AnnPodReady])
		return ready, nil
	})

	return pvc, err
}

func (app *uploadProxyApp) proxyUploadRequest(uploadPath string, w http.ResponseWriter, r *http.Request) {
	client, err := app.clientCreator.CreateClient()
	if err != nil {
		klog.Error("Error creating http client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var buff bytes.Buffer
	p := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL, _ = url.Parse(uploadPath)
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		},
		Transport: client.Transport,
		ErrorLog:  log.New(&buff, "", 0),
	}

	p.ServeHTTP(w, r)

	if buff.Len() > 0 {
		msg := buff.String()
		klog.Errorf("Error in reverse proxy: %s", msg)
		fmt.Fprintf(w, "error in upload-proxy: %s", msg)
	}
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

func (app *uploadProxyApp) getTLSConfig() *tls.Config {
	cryptoConfig := app.cdiConfigTLSWatcher.GetCdiTLSConfig()

	//nolint:gosec // False positive (MinVersion unknown at build time)
	tlsConfig := &tls.Config{
		GetCertificate: app.certWatcher.GetCertificate,
		CipherSuites:   cryptoConfig.CipherSuites,
		MinVersion:     cryptoConfig.MinVersion,
	}

	return tlsConfig
}

func (app *uploadProxyApp) startTLS() error {
	var serveFunc func() error
	bindAddr := fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort)

	server := &http.Server{
		Addr:              bindAddr,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if app.certWatcher != nil {
		tlsConfig := app.getTLSConfig()
		tlsConfig.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			klog.V(3).Info("Getting TLS config")
			config := app.getTLSConfig()
			return config, nil
		}
		server.TLSConfig = tlsConfig

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

func (app *uploadProxyApp) getPopulationPVC(ctx context.Context, pvc *v1.PersistentVolumeClaim, pvcNamespace string) (*v1.PersistentVolumeClaim, error) {
	pvcPrimeName, ok := pvc.Annotations[cc.AnnPVCPrimeName]
	if !ok {
		// wait for pvcPrimeName annotation on the pvc
		return nil, nil
	}
	pvcPrime, err := app.client.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcPrimeName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("rejecting Upload Request for PVC %s, PVC' wasn't created yet", pvc.Name)
		}

		return nil, err
	}

	return pvcPrime, nil
}
