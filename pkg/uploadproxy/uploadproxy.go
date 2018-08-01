package uploadproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"

	apiserver "kubevirt.io/containerized-data-importer/pkg/apiserver"
	//. "kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// selfsigned cert secret name
	apiCertSecretName = "cdi-api-certs"

	apiServiceName = "cdi-api"

	certBytesValue = "cert-bytes"
	keyBytesValue  = "key-bytes"

	uploadPath = "/upload"
)

type UploadApiServer interface {
	Start() error
}

type uploadProxyApp struct {
	bindAddress string
	bindPort    uint

	client *kubernetes.Clientset

	certsDirectory string

	certBytes []byte
	keyBytes  []byte

	// Used to decrypt token.
	uploadProxyPrivateKey *rsa.PrivateKey

	// Used to verify token came from our apiserver.
	apiServerPublicKey *rsa.PublicKey
}

func NewUploadProxy(bindAddress string, bindPort uint, client *kubernetes.Clientset) (UploadApiServer, error) {
	var err error
	app := &uploadProxyApp{
		bindAddress: bindAddress,
		bindPort:    bindPort,
		client:      client,
	}
	app.certsDirectory, err = ioutil.TempDir("", "certsdir")
	if err != nil {
		glog.Fatalf("Unable to create certs temporary directory: %v\n", errors.WithStack(err))
	}

	// generate/retrieve RSA key used to decrypt tokens
	err = app.generateKeys()
	if err != nil {
		glog.Fatal("Unable to generate and retrieve rsa keys: %v", errors.WithStack(err))
	}

	// retrieve RSA key used by apiserver to sign tokens
	err = app.getSigningKey()
	if err != nil {
		glog.Fatal("Unable to retrieve apiserver signing key: %v", errors.WithStack(err))
	}

	// generate self signed cert
	err = app.generateSelfSignedCert()
	if err != nil {
		glog.Fatalf("Unable to create self signed certs for upload proxy: %v\n", errors.WithStack(err))

	}

	http.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
		app.handleUploadRequest(w, r)
	})

	return app, nil
}

func (app *uploadProxyApp) handleUploadRequest(w http.ResponseWriter, r *http.Request) {

	encryptedTokenData := r.Header.Get("UPLOAD_TOKEN")
	if encryptedTokenData == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tokenMessage, err := apiserver.DecryptToken(encryptedTokenData, app.uploadProxyPrivateKey, app.apiServerPublicKey)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if tokenMessage != "" {
		fmt.Printf("DECODED MESSAGE SUCCESSFULLY")
	}
	// TODO add proxy logic here.

}

func (app *uploadProxyApp) generateKeys() error {

	proxyKeyPair, exists, err := apiserver.GetUploadProxyPrivateKey(app.client)
	if err != nil {
		return err
	}

	if !exists {
		proxyKeyPair, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}

		err = apiserver.RecordUploadProxyPrivateKey(app.client, proxyKeyPair)
		if err != nil {
			return err
		}

	}

	err = apiserver.RecordUploadProxyPublicKey(app.client, &proxyKeyPair.PublicKey)
	if err != nil {
		return err
	}

	app.uploadProxyPrivateKey = proxyKeyPair
	return nil
}

func (app *uploadProxyApp) getSigningKey() error {

	publicKey, exists, err := apiserver.GetApiPublicKey(app.client)
	if err != nil {
		return err
	} else if !exists {
		return errors.Errorf("apiserver signing key is not found")
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

	go func() {
		http.ListenAndServeTLS(fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort), certFile, keyFile, nil)
	}()

	// wait for server to exit
	return <-errors
}
