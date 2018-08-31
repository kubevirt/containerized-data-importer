package uploadproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"
)

func generateTestKeys() (string, error) {

	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	publicKeyPem, err := cert.EncodePublicKeyPEM(&apiKeyPair.PublicKey)
	if err != nil {
		return "", err
	}

	caKeyPair, err := triple.NewCA("uploadproxytest")
	if err != nil {
		return "", err
	}

	err = os.MkdirAll("/etc/tls/uploadproxy", 0664)
	if err != nil {
		return "", err
	}

	// in reality this is the CA for the upload servers not the clients but whatever shouldn't matter here
	err = ioutil.WriteFile("/etc/tls/uploadproxy/ca.cert", cert.EncodeCertPEM(caKeyPair.Cert), 0600)
	if err != nil {
		return "", err
	}

	clientKeyPair, err := triple.NewClientKeyPair(caKeyPair, "uploadproxy", []string{})
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile("/etc/tls/uploadproxy/tls.cert", cert.EncodeCertPEM(clientKeyPair.Cert), 0600)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile("/etc/tls/uploadproxy/tls.key", cert.EncodePrivateKeyPEM(clientKeyPair.Key), 0600)
	if err != nil {
		return "", err
	}

	return string(publicKeyPem), nil
}

func TestKeyRetrieval(t *testing.T) {
	publicKeyPEM, err := generateTestKeys()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	app := uploadProxyApp{}
	err = app.getSigningKey(publicKeyPEM)
	if err != nil {
		t.Errorf("Failed to parse public key pem")
	}

	if app.apiServerPublicKey == nil {
		t.Errorf("Failed to create public key")
	}
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
