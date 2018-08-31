package apiserver

import (
	"crypto/rsa"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"k8s.io/client-go/util/cert"

	"gopkg.in/square/go-jose.v2"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	. "kubevirt.io/containerized-data-importer/pkg/common"
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

// RecordAPIPublicKey stores the api key in a config map
func RecordAPIPublicKey(client kubernetes.Interface, publicKey *rsa.PublicKey) error {
	return setPublicKeyConfigMap(client, publicKey, APIPublicKeyConfigMap)
}

// GetNamespace returns the nakespace the pod is executing in
func GetNamespace() string {
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}
	return metav1.NamespaceSystem
}

func getConfigMap(client kubernetes.Interface, configMap string) (*v1.ConfigMap, bool, error) {
	namespace := GetNamespace()

	config, err := client.CoreV1().ConfigMaps(namespace).Get(configMap, metav1.GetOptions{})

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return config, true, nil
}

// EncodePublicKey PEM encodes a public key
func EncodePublicKey(key *rsa.PublicKey) (string, error) {
	bytes, err := cert.EncodePublicKeyPEM(key)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
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

func setPublicKeyConfigMap(client kubernetes.Interface, publicKey *rsa.PublicKey, configMap string) error {
	namespace := GetNamespace()
	publicKeyEncoded, err := EncodePublicKey(publicKey)
	if err != nil {
		return err
	}

	config, exists, err := getConfigMap(client, configMap)
	if err != nil {
		return err
	}

	if exists {
		curKeyEncoded, ok := config.Data["publicKey"]
		if !ok || curKeyEncoded != publicKeyEncoded {
			// returning error rather than updating
			// this will make if obvious if the keys somehow become out of sync
			// also fewer permissions for service account
			return errors.Errorf("Problem with public key, exists %b, not equat %b", ok, curKeyEncoded == publicKeyEncoded)
		}
	} else {
		// Create
		config := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMap,
				Labels: map[string]string{
					CDI_COMPONENT_LABEL: configMap,
				},
			},
			Data: map[string]string{
				"publicKey": publicKeyEncoded,
			},
		}
		_, err := client.CoreV1().ConfigMaps(namespace).Create(config)
		if err != nil {
			return err
		}
	}
	return nil
}
