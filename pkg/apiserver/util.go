package apiserver

import (
	"crypto/rsa"
	"encoding/json"
	"time"

	"gopkg.in/square/go-jose.v2"
	"k8s.io/client-go/util/cert"

	"github.com/pkg/errors"
)

const (
	// APIPublicKeyConfigMap is the uploadProxy Public key
	APIPublicKeyConfigMap = "cdi-api-public"

	// timeout seconds for each token. 5 minutes
	tokenTimeout = 300
)

// TokenData defines the data in the upload token
type TokenData struct {
	PvcName           string    `json:"pvcName"`
	Namespace         string    `json:"namespace"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
}

// VerifyToken checks the token signature and returns the contents
func VerifyToken(token string, publicKey *rsa.PublicKey) (*TokenData, error) {
	object, err := jose.ParseSigned(token)
	if err != nil {
		return nil, err
	}

	message, err := object.Verify(publicKey)
	if err != nil {
		return nil, err
	}

	tokenData := &TokenData{}
	err = json.Unmarshal(message, tokenData)
	if err != nil {
		return nil, err
	}

	// don't allow expired tokens to be viewed
	start := tokenData.CreationTimestamp.Unix()
	now := time.Now().UTC().Unix()
	if (now - start) > tokenTimeout {
		return nil, errors.Errorf("Token expired")
	}

	// If we get here, the message is good
	return tokenData, nil
}

// GenerateToken generates a token from the given parameters
func GenerateToken(pvcName string, namespace string, signingKey *rsa.PrivateKey) (string, error) {
	tokenData := &TokenData{
		Namespace:         namespace,
		PvcName:           pvcName,
		CreationTimestamp: time.Now().UTC(),
	}

	message, err := json.Marshal(tokenData)
	if err != nil {
		return "", err
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.PS512, Key: signingKey}, nil)
	if err != nil {
		return "", err
	}

	object, err := signer.Sign(message)
	if err != nil {
		return "", nil
	}

	serialized, err := object.CompactSerialize()
	if err != nil {
		return "", err
	}

	return serialized, nil
}

// DecodePublicKey decodes a PEM encoded public key
func DecodePublicKey(encodedKey string) (*rsa.PublicKey, error) {
	keys, err := cert.ParsePublicKeysPEM([]byte(string(encodedKey)))
	if err != nil {
		return nil, err
	}

	if len(keys) != 1 {
		return nil, errors.New("Unexected number of pulic keys")
	}

	key, ok := keys[0].(*rsa.PublicKey)
	if !ok {
		return nil, err
	}

	return key, nil
}
