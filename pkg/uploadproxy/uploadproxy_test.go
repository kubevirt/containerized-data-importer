package uploadproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"k8s.io/client-go/util/cert"
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
