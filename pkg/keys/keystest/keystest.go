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

package keystest

import (
	"crypto/rsa"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

// NewPrivateKeySecret returns a new private key secret
func NewPrivateKeySecret(namespace, secretName string, privateKey *rsa.PrivateKey) (*v1.Secret, error) {
	privateKeyBytes := cert.EncodePrivateKeyPEM(privateKey)
	publicKeyBytes, err := cert.EncodePublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error encoding public key")
	}

	data := map[string][]byte{
		"id_rsa":     privateKeyBytes,
		"id_rsa.pub": publicKeyBytes,
	}

	return newSecret(namespace, secretName, data, nil), nil
}

func newSecret(namespace, secretName string, data map[string][]byte, owner *metav1.OwnerReference) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				common.CDIComponentLabel:           "keystore",
				common.AppKubernetesComponentLabel: "storage",
				common.AppKubernetesManagedByLabel: "cdi-apiserver",
			},
		},
		Type: "Opaque",
		Data: data,
	}

	if owner != nil {
		secret.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	return secret
}
