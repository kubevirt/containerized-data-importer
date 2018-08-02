package apiserver

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	. "kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// upload proxy generated
	UploadProxySecretName = "cdi-proxy-private"

	// uploadProxy Public key
	ApiPublicKeyConfigMap = "cdi-api-public"

	// uploadProxy Public key
	UploadProxyPublicKeyConfigMap = "cdi-proxy-public"

	// timeout seconds for each token. 5 minutes
	tokenTimeout = 300
)

type TokenData struct {
	RandomPadding     []byte    `json:"randomPadding"`
	PvcName           string    `json:"pvcName"`
	Namespace         string    `json:"Namespace"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
}

type tokenPayload struct {
	EncryptedData []byte `json:"encryptedData"`
	Signature     []byte `json:"signature"`
}

func DecryptToken(encryptedToken string,
	privateDecryptionKey *rsa.PrivateKey,
	publicSigningKey *rsa.PublicKey) (*TokenData, error) {

	label := []byte("")
	hash := sha256.New()

	tokenPayload, err := decodeTokenData(encryptedToken)
	if err != nil {
		return nil, err
	}

	message, err := rsa.DecryptOAEP(hash, rand.Reader, privateDecryptionKey, tokenPayload.EncryptedData, label)
	if err != nil {
		return nil, err
	}

	var opts rsa.PSSOptions
	opts.SaltLength = rsa.PSSSaltLengthAuto
	newhash := crypto.SHA256
	pssh := newhash.New()
	pssh.Write(message)
	hashed := pssh.Sum(nil)

	//Verify Signature
	err = rsa.VerifyPSS(publicSigningKey, newhash, hashed, tokenPayload.Signature, &opts)
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

	// If we get here, the message is decrypted and the signature passed
	return tokenData, nil
}

func GenerateEncryptedToken(pvcName string,
	namespace string,
	publicEncryptionKey *rsa.PublicKey,
	privateSigningKey *rsa.PrivateKey) (string, error) {

	tokenData := &TokenData{
		Namespace:         namespace,
		PvcName:           pvcName,
		CreationTimestamp: time.Now().UTC(),
	}

	randPad := make([]byte, 16)
	rand.Read(randPad)
	tokenData.RandomPadding = randPad

	message, err := json.Marshal(tokenData)
	if err != nil {
		return "", err
	}

	label := []byte("")
	hash := sha256.New()

	encryptedMessage, err := rsa.EncryptOAEP(hash, rand.Reader, publicEncryptionKey, message, label)
	if err != nil {
		return "", err
	}

	var opts rsa.PSSOptions
	opts.SaltLength = rsa.PSSSaltLengthAuto
	newhash := crypto.SHA256
	pssh := newhash.New()
	pssh.Write(message)
	hashed := pssh.Sum(nil)

	signature, err := rsa.SignPSS(rand.Reader, privateSigningKey, newhash, hashed, &opts)

	if err != nil {
		return "", err
	}

	tokenPayload := &tokenPayload{
		EncryptedData: encryptedMessage,
		Signature:     signature,
	}

	return encodeTokenData(tokenPayload)
}

func encodeTokenData(tokenPayload *tokenPayload) (string, error) {

	bytes, err := json.Marshal(tokenPayload)
	if err != nil {
		return "", err
	}

	str := base64.StdEncoding.EncodeToString(bytes)
	return str, nil
}

func decodeTokenData(encodedtokenPayload string) (*tokenPayload, error) {
	bytes, err := base64.StdEncoding.DecodeString(encodedtokenPayload)
	if err != nil {
		return nil, err
	}

	tokenPayload := &tokenPayload{}
	err = json.Unmarshal(bytes, tokenPayload)
	if err != nil {
		return nil, err
	}

	return tokenPayload, nil
}

func RecordApiPublicKey(client kubernetes.Interface, publicKey *rsa.PublicKey) error {
	return setPublicKeyConfigMap(client, publicKey, ApiPublicKeyConfigMap)
}

func RecordUploadProxyPublicKey(client kubernetes.Interface, publicKey *rsa.PublicKey) error {
	return setPublicKeyConfigMap(client, publicKey, UploadProxyPublicKeyConfigMap)
}

func RecordUploadProxyPrivateKey(client kubernetes.Interface, privateKey *rsa.PrivateKey) error {
	return setPrivateKeySecret(client, privateKey, UploadProxySecretName)
}

func GetApiPublicKey(client kubernetes.Interface) (*rsa.PublicKey, bool, error) {
	return getPublicKey(client, ApiPublicKeyConfigMap)
}

func GetUploadProxyPublicKey(client kubernetes.Interface) (*rsa.PublicKey, bool, error) {
	return getPublicKey(client, UploadProxyPublicKeyConfigMap)
}

func GetUploadProxyPrivateKey(client kubernetes.Interface) (*rsa.PrivateKey, bool, error) {
	return getPrivateSecret(client, UploadProxySecretName)
}

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
		} else {
			return nil, false, err
		}
	}

	return config, true, nil
}

func EncodePublicKey(key *rsa.PublicKey) string {
	bytes := x509.MarshalPKCS1PublicKey(key)
	return base64.StdEncoding.EncodeToString(bytes)
}

func DecodePublicKey(encodedKey string) (*rsa.PublicKey, error) {
	bytes, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKCS1PublicKey(bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func getPublicKey(client kubernetes.Interface, configMap string) (*rsa.PublicKey, bool, error) {
	config, exists, err := getConfigMap(client, configMap)
	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	publicKeyEncoded, ok := config.Data["publicKey"]
	if !ok {
		return nil, false, nil
	}

	key, err := DecodePublicKey(publicKeyEncoded)

	return key, true, err
}

func setPublicKeyConfigMap(client kubernetes.Interface, publicKey *rsa.PublicKey, configMap string) error {
	publicKeyEncoded := EncodePublicKey(publicKey)
	namespace := GetNamespace()

	config, exists, err := getConfigMap(client, configMap)
	if err != nil {
		return err
	}

	if exists {
		curKeyEncoded, ok := config.Data["publicKey"]
		if !ok || curKeyEncoded != publicKeyEncoded {
			// Update if the key doesn't exist or doesn't match
			config.Data["publicKey"] = publicKeyEncoded
			_, err := client.CoreV1().ConfigMaps(namespace).Update(config)
			if err != nil {
				return err
			}
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

func EncodePrivateKey(key *rsa.PrivateKey) string {
	bytes := x509.MarshalPKCS1PrivateKey(key)
	return base64.StdEncoding.EncodeToString(bytes)
}

func DecodePrivateKey(encodedKey string) (*rsa.PrivateKey, error) {
	bytes, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKCS1PrivateKey(bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func getSecret(client kubernetes.Interface, secretName string) (*v1.Secret, bool, error) {
	namespace := GetNamespace()
	secret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	return secret, true, nil
}

func getPrivateSecret(client kubernetes.Interface, secretName string) (*rsa.PrivateKey, bool, error) {
	secret, exists, err := getSecret(client, secretName)
	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	privateKeyEncoded, ok := secret.Data["privateKey"]
	if !ok {
		return nil, false, nil
	}

	key, err := DecodePrivateKey(string(privateKeyEncoded))

	return key, true, err
}

func setPrivateKeySecret(client kubernetes.Interface, privateKey *rsa.PrivateKey, secretName string) error {
	privateKeyEncoded := EncodePrivateKey(privateKey)
	namespace := GetNamespace()

	secret, exists, err := getSecret(client, secretName)
	if err != nil {
		return err
	}

	if exists {
		// Update
		secret.Data["privateKey"] = []byte(privateKeyEncoded)
		_, err := client.CoreV1().Secrets(namespace).Update(secret)
		if err != nil {
			return err
		}
	} else {
		// Create
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Labels: map[string]string{
					CDI_COMPONENT_LABEL: secretName,
				},
			},
			Data: map[string][]byte{
				"privateKey": []byte(privateKeyEncoded),
			},
		}
		_, err := client.CoreV1().Secrets(namespace).Create(secret)
		if err != nil {
			return err
		}
	}
	return nil
}
