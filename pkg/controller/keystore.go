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

package controller

import (
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"
)

const (
	keyStoreTLSKeyFile  = "tls.key"
	keyStoreTLSCertFile = "tls.cert"
	keyStoreTLSCAFile   = "ca.cert"
)

// KeyPairAndCert holds a KeyPair and optional CA
// In the case of a server key pair, the CA is the CA that signed client certs
// In the case of a client key pair, the CA is the CA that signed the server cert
type KeyPairAndCert struct {
	KeyPair triple.KeyPair
	CACert  *x509.Certificate
}

// GetOrCreateCA will get the CA KeyPair, creating it if necessary
func GetOrCreateCA(client kubernetes.Interface, namespace, secretName, caName string) (*triple.KeyPair, error) {
	keyPairAndCert, err := GetKeyPairAndCert(client, namespace, secretName)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting CA")
	}

	if keyPairAndCert != nil {
		glog.Infof("Retrieved CA key/cert %s from kubernetes", caName)
		return &keyPairAndCert.KeyPair, nil
	}

	glog.Infof("Recreating CA %s", caName)

	keyPair, err := triple.NewCA(caName)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating CA")
	}

	if err = SaveKeyPairAndCert(client, namespace, secretName, KeyPairAndCert{*keyPair, nil}, false, nil); err != nil {
		return nil, errors.Wrap(err, "Error saving CA")
	}

	return keyPair, nil
}

// CreateServerKeyPairAndCert creates secret for an upload server
func CreateServerKeyPairAndCert(client kubernetes.Interface,
	namespace,
	secretName string,
	caKeyPair *triple.KeyPair,
	clientCACert *x509.Certificate,
	commonName string,
	serviceName string,
	failIfExists bool,
	owner *metav1.OwnerReference) error {
	keyPair, err := triple.NewServerKeyPair(caKeyPair, commonName, serviceName, namespace, "cluster.local", []string{}, []string{})
	if err != nil {
		return errors.Wrap(err, "Error creating server key pair")
	}

	if err = SaveKeyPairAndCert(client, namespace, secretName, KeyPairAndCert{*keyPair, clientCACert}, failIfExists, owner); err != nil {
		return errors.Wrap(err, "Error saving server key pair")
	}

	return nil
}

// CreateClientKeyPairAndCert creates a secret for upload proxy
func CreateClientKeyPairAndCert(client kubernetes.Interface,
	namespace, secretName string,
	caKeyPair *triple.KeyPair,
	serverCACert *x509.Certificate,
	commonName string,
	organizations []string,
	failIfExists bool,
	owner *metav1.OwnerReference) error {
	keyPair, err := triple.NewClientKeyPair(caKeyPair, commonName, organizations)
	if err != nil {
		return errors.Wrap(err, "Error creating client key pair")
	}

	if err = SaveKeyPairAndCert(client, namespace, secretName, KeyPairAndCert{*keyPair, serverCACert}, failIfExists, owner); err != nil {
		return errors.Wrap(err, "Error saving client key pair")
	}

	return nil
}

// GetKeyPairAndCert will return the secret data if it exists
func GetKeyPairAndCert(client kubernetes.Interface, namespace, secretName string) (*KeyPairAndCert, error) {
	var keyPairAndCert KeyPairAndCert

	secret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Error getting secret")
	}

	if bytes, ok := secret.Data[keyStoreTLSKeyFile]; ok {
		obj, err := cert.ParsePrivateKeyPEM(bytes)
		if err != nil {
			return nil, errors.Wrap(err, "Error parsing secret")
		}

		key, ok := obj.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("Invalid pem format")
		}

		keyPairAndCert.KeyPair.Key = key
	} else {
		return nil, errors.Errorf("Private key missing from secret")
	}

	if bytes, ok := secret.Data[keyStoreTLSCertFile]; ok {
		certs, err := cert.ParseCertsPEM(bytes)
		if err != nil || len(certs) != 1 {
			return nil, errors.Errorf("Cert parse error %s, %d", err, len(certs))
		}
		keyPairAndCert.KeyPair.Cert = certs[0]
	} else {
		return nil, errors.Errorf("Cert missing from secret")
	}

	// okay if this doesn't exist
	if bytes, ok := secret.Data[keyStoreTLSCAFile]; ok {
		certs, err := cert.ParseCertsPEM(bytes)
		if err != nil || len(certs) != 1 {
			return nil, errors.Errorf("CA cert parse error %s, %d", err, len(certs))
		}
		keyPairAndCert.CACert = certs[0]
	}

	return &keyPairAndCert, nil
}

// SaveKeyPairAndCert saves a private key, cert, and maybe a ca cert to kubernetes
func SaveKeyPairAndCert(client kubernetes.Interface, namespace, secretName string, keyPairAntCA KeyPairAndCert, failIfExists bool, owner *metav1.OwnerReference) error {
	privateKeyBytes := cert.EncodePrivateKeyPEM(keyPairAntCA.KeyPair.Key)
	certBytes := cert.EncodeCertPEM(keyPairAntCA.KeyPair.Cert)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			keyStoreTLSKeyFile:  privateKeyBytes,
			keyStoreTLSCertFile: certBytes,
		},
	}

	if owner != nil {
		secret.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	if keyPairAntCA.CACert != nil {
		caCertBytes := cert.EncodeCertPEM(keyPairAntCA.CACert)
		secret.Data[keyStoreTLSCAFile] = caCertBytes
	}

	_, err := client.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
		if !failIfExists && k8serrors.IsAlreadyExists(err) {
			return nil
		}
		return errors.Wrap(err, "Error creating cert")
	}

	return nil
}

// GetNamespace returns the namespace the pod is executing in
func GetNamespace() string {
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}
	return metav1.NamespaceSystem
}
