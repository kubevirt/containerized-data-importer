package uploadproxy

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"

	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"

	apiserver "kubevirt.io/containerized-data-importer/pkg/apiserver"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	uploadserver "kubevirt.io/containerized-data-importer/pkg/uploadserver"
)

const (
	// selfsigned cert secret name
	apiCertSecretName = "cdi-api-certs"

	apiServiceName = "cdi-api"

	certBytesValue = "cert-bytes"
	keyBytesValue  = "key-bytes"

	uploadPath = "/v1alpha1/upload"
)

// Server is the public interface to the upload proxy
type Server interface {
	Start() error
}

type uploadProxyApp struct {
	bindAddress string
	bindPort    uint

	client kubernetes.Interface

	certsDirectory string

	certBytes []byte
	keyBytes  []byte

	// Used to verify token came from our apiserver.
	apiServerPublicKey *rsa.PublicKey

	uploadServerClient *http.Client
}

var authHeaderMatcher *regexp.Regexp

func init() {
	authHeaderMatcher = regexp.MustCompile(`(?i)^Bearer\s+([A-Za-z0-9\-\._~\+\/]+)$`)
}

// NewUploadProxy returns an initialized uploadProxyApp
func NewUploadProxy(bindAddress string,
	bindPort uint,
	apiServerPublicKey string,
	tlsClientKey string,
	tlsClientCert string,
	tlsServerCert string,
	client kubernetes.Interface) (Server, error) {
	var err error
	app := &uploadProxyApp{
		bindAddress: bindAddress,
		bindPort:    bindPort,
		client:      client,
	}
	app.certsDirectory, err = ioutil.TempDir("", "certsdir")
	if err != nil {
		return nil, errors.Errorf("Unable to create certs temporary directory: %v\n", errors.WithStack(err))
	}

	// retrieve RSA key used by apiserver to sign tokens
	err = app.getSigningKey(apiServerPublicKey)
	if err != nil {
		return nil, errors.Errorf("Unable to retrieve apiserver signing key: %v", errors.WithStack(err))
	}

	// generate self signed cert
	err = app.generateSelfSignedCert()
	if err != nil {
		return nil, errors.Errorf("Unable to create self signed certs for upload proxy: %v\n", errors.WithStack(err))

	}

	// get upload server http client
	err = app.getUploadServerClient(tlsClientKey, tlsClientCert, tlsServerCert)
	if err != nil {
		return nil, errors.Errorf("Unable to create upload server client: %v\n", errors.WithStack(err))
	}

	return app, nil
}

func (app *uploadProxyApp) getUploadServerClient(tlsClientKey, tlsClientCert, tlsServerCert string) error {
	clientCert, err := tls.X509KeyPair([]byte(tlsClientCert), []byte(tlsClientKey))
	if err != nil {
		return err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(tlsServerCert))

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	app.uploadServerClient = client

	return nil
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

	tokenData, err := apiserver.VerifyToken(match[1], app.apiServerPublicKey)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	glog.V(Vuser).Infof("Received valid token: pvc: %s, namespace: %s", tokenData.PvcName, tokenData.Namespace)

	app.proxyUploadRequest(tokenData.Namespace, tokenData.PvcName, w, r)
}

func (app *uploadProxyApp) proxyUploadRequest(namespace, pvc string, w http.ResponseWriter, r *http.Request) {
	url := uploadserver.GetUploadServerURL(namespace, pvc)

	req, err := http.NewRequest("POST", url, r.Body)
	req.ContentLength = r.ContentLength

	glog.V(Vdebug).Infof("Posting to: %s", url)

	response, err := app.uploadServerClient.Do(req)
	if err != nil {
		glog.Errorf("Error proxying %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	glog.V(Vdebug).Infof("Response status for url %s: %d", url, response.StatusCode)

	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	if err != nil {
		glog.Warningf("Error proxying response from url %s", url)
	}
}

func (app *uploadProxyApp) getSigningKey(publicKeyPEM string) error {
	publicKey, err := apiserver.DecodePublicKey(publicKeyPEM)
	if err != nil {
		return err
	}

	app.apiServerPublicKey = publicKey
	return nil
}

func (app *uploadProxyApp) Start() error {
	return app.startTLS()
}

func (app *uploadProxyApp) generateSelfSignedCert() error {
	namespace := apiserver.GetNamespace()

	caKeyPair, _ := triple.NewCA("kubecdi.io")
	keyPair, _ := triple.NewServerKeyPair(
		caKeyPair,
		"cdi-api."+namespace+".pod.cluster.local",
		"cdi-api",
		namespace,
		"cluster.local",
		nil,
		nil,
	)

	app.keyBytes = cert.EncodePrivateKeyPEM(keyPair.Key)
	app.certBytes = cert.EncodeCertPEM(keyPair.Cert)

	return nil
}

func (app *uploadProxyApp) startTLS() error {

	errors := make(chan error)

	keyFile := filepath.Join(app.certsDirectory, "/key.pem")
	certFile := filepath.Join(app.certsDirectory, "/cert.pem")

	err := ioutil.WriteFile(keyFile, app.keyBytes, 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(certFile, app.certBytes, 0600)
	if err != nil {
		return err
	}

	http.HandleFunc(uploadPath, app.handleUploadRequest)

	go func() {
		errors <- http.ListenAndServeTLS(fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort), certFile, keyFile, nil)
	}()

	// wait for server to exit
	return <-errors
}
