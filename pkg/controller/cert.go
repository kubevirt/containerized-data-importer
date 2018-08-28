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
	// PrivateKeyKeyName is the key for the private key
	PrivateKeyKeyName = "tls.key"

	// CertKeyName is the key for the cert
	CertKeyName = "tls.cert"

	// CACertKeyName is the key for the ca cert
	CACertKeyName = "ca.cert"
)

// GetOrCreateCA will get the CA KeyPair, creating it if necessary
func GetOrCreateCA(client kubernetes.Interface, namespace, secretName, caName string, owner *metav1.OwnerReference) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, secretName)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting CA")
	}

	if keyPair != nil {
		glog.Infof("Retrieved CA key/cert %s from kubernetes", caName)
		return keyPair, nil
	}

	glog.Infof("Recreating CA %s", caName)

	keyPair, err = triple.NewCA(caName)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating CA")
	}

	if err = SaveKeyPair(client, namespace, secretName, keyPair, owner, nil); err != nil {
		return nil, errors.Wrap(err, "Error saving CA")
	}

	return keyPair, nil
}

// GetOrCreateServerKeyPair creates a KeyPair for a upload server
func GetOrCreateServerKeyPair(client kubernetes.Interface,
	namespace, secretName string,
	caKeyPair *triple.KeyPair,
	commonName, serviceName string,
	owner *metav1.OwnerReference,
	clientCACert *x509.Certificate) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, secretName)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting server key pair")
	}

	if keyPair != nil {
		glog.Infof("Retrieved server key/cert %s %s from kubernetes", commonName, serviceName)
		return keyPair, nil
	}

	keyPair, err = triple.NewServerKeyPair(caKeyPair, commonName, serviceName, namespace, "cluster.local", []string{}, []string{})
	if err != nil {
		return nil, errors.Wrap(err, "Error creating server key pair")
	}

	if err = SaveKeyPair(client, namespace, secretName, keyPair, owner, clientCACert); err != nil {
		return nil, errors.Wrap(err, "Error saving server key pair")
	}

	return keyPair, nil
}

// GetOrCreateClientKeyPair creates a KeyPair for a client
func GetOrCreateClientKeyPair(client kubernetes.Interface,
	namespace, secretName string,
	caKeyPair *triple.KeyPair,
	commonName string,
	organizations []string,
	owner *metav1.OwnerReference,
	serverCACert *x509.Certificate) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, secretName)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting server key pair")
	}

	if keyPair != nil {
		glog.Infof("Retrieved client key/cert %s from kubernetes", organizations)
		return keyPair, nil
	}

	keyPair, err = triple.NewClientKeyPair(caKeyPair, commonName, organizations)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating client key pair")
	}

	if err = SaveKeyPair(client, namespace, secretName, keyPair, owner, serverCACert); err != nil {
		return nil, errors.Wrap(err, "Error saving client key pair")
	}

	return keyPair, nil
}

// GetKeyPair will return the KeyPair if it exists
func GetKeyPair(client kubernetes.Interface, namespace, secretName string) (*triple.KeyPair, error) {
	var keyPair triple.KeyPair

	secret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Error getting secret")
	}

	if bytes, ok := secret.Data[PrivateKeyKeyName]; ok {
		obj, err := cert.ParsePrivateKeyPEM(bytes)
		if err != nil {
			return nil, errors.Wrap(err, "Error parsing secret")
		}

		key, ok := obj.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("Invalid pem format")
		}
		keyPair.Key = key
	}

	if bytes, ok := secret.Data[CertKeyName]; ok {
		certs, err := cert.ParseCertsPEM(bytes)
		if err != nil || len(certs) != 1 {
			return nil, errors.Errorf("Cert parse error %s, %d", err, len(certs))
		}
		keyPair.Cert = certs[0]
	}

	return &keyPair, nil
}

// SaveKeyPair saves a private key and cert to kubernetes
func SaveKeyPair(client kubernetes.Interface, namespace, secretName string, keyPair *triple.KeyPair, owner *metav1.OwnerReference, caCert *x509.Certificate) error {
	privateKeyBytes := cert.EncodePrivateKeyPEM(keyPair.Key)
	certBytes := cert.EncodeCertPEM(keyPair.Cert)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			PrivateKeyKeyName: privateKeyBytes,
			CertKeyName:       certBytes,
		},
	}

	if owner != nil {
		secret.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	if caCert != nil {
		caCertBytes := cert.EncodeCertPEM(caCert)
		secret.Data[CACertKeyName] = caCertBytes
	}

	_, err := client.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
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
