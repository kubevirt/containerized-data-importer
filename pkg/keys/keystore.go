/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

const (
	// KeyStorePrivateKeyFile is the key in a secret containing an RSA private key
	KeyStorePrivateKeyFile = "id_rsa"

	// KeyStorePublicKeyFile is the key in a secret containing an RSA publis key
	KeyStorePublicKeyFile = "id_rsa.pub"
)

// GetOrCreatePrivateKey gets or creates a private key secret
func GetOrCreatePrivateKey(client kubernetes.Interface, namespace, secretName string) (*rsa.PrivateKey, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "Error getting secret")
		}

		// let's create the secret
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, errors.Wrap(err, "Error generating key")
		}

		secret, err = newPrivateKeySecret(client, namespace, secretName, privateKey)
		if err != nil {
			return nil, errors.Wrap(err, "Error creating prvate key secret")
		}

		secret, err = client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				return nil, errors.Wrap(err, "Error creating secret")
			}

			secret, err = client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "Error getting secret, second time")
			}
		}
	}

	bytes, ok := secret.Data[KeyStorePrivateKeyFile]
	if !ok {
		return nil, errors.Wrap(err, "Secret missing private key")
	}

	return parsePrivateKey(bytes)
}

// newPrivateKeySecret returns a new private key secret
func newPrivateKeySecret(client kubernetes.Interface, namespace, secretName string, privateKey *rsa.PrivateKey) (*v1.Secret, error) {
	privateKeyBytes := cert.EncodePrivateKeyPEM(privateKey)
	publicKeyBytes, err := cert.EncodePublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error encoding public key")
	}

	data := map[string][]byte{
		KeyStorePrivateKeyFile: privateKeyBytes,
		KeyStorePublicKeyFile:  publicKeyBytes,
	}

	secret, err := newSecret(client, namespace, secretName, data, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create PrivateKeySecret")
	}

	return secret, nil
}

func newSecret(client kubernetes.Interface, namespace, secretName string, data map[string][]byte, owner *metav1.OwnerReference) (*v1.Secret, error) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel: "keystore",
			},
		},
		Type: "Opaque",
		Data: data,
	}

	if owner == nil {
		err := operator.SetOwner(client, secret)
		if err != nil {
			return nil, errors.Wrap(err, "Error setting secret owner ref")
		}

	} else {
		secret.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	return secret, nil
}

func parsePrivateKey(bytes []byte) (*rsa.PrivateKey, error) {
	obj, err := cert.ParsePrivateKeyPEM(bytes)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing secret")
	}

	key, ok := obj.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("Invalid pem format")
	}

	return key, nil
}
