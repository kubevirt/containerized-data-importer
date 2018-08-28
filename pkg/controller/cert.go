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
	// PrivateKeyKeyName is secret key for private key
	PrivateKeyKeyName = "privarteKey"

	// CertKeyName is the configmap key for the cert
	CertKeyName = "cert"
)

// GetOrCreateCA will get the CA KeyPair, creating it if necessary
func GetOrCreateCA(client kubernetes.Interface, namespace, privateKeySecret, certConfigKey, caName string, owner *metav1.OwnerReference) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, privateKeySecret, certConfigKey)
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

	if err = SaveKeyPair(client, namespace, privateKeySecret, certConfigKey, keyPair, owner); err != nil {
		return nil, errors.Wrap(err, "Error saving CA")
	}

	return keyPair, nil
}

// GetOrCreateServerKeyPair creates a KeyPair for a server
func GetOrCreateServerKeyPair(client kubernetes.Interface,
	namespace, privateKeySecret, certConfigKey string,
	caKeyPair *triple.KeyPair,
	commonName, serviceName string,
	owner *metav1.OwnerReference) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, privateKeySecret, certConfigKey)
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

	if err = SaveKeyPair(client, namespace, privateKeySecret, certConfigKey, keyPair, owner); err != nil {
		return nil, errors.Wrap(err, "Error saving server key pair")
	}

	return keyPair, nil
}

// GetOrCreateClientKeyPair creates a KeyPair for a client
func GetOrCreateClientKeyPair(client kubernetes.Interface,
	namespace, privateKeySecret, certConfigKey string,
	caKeyPair *triple.KeyPair,
	commonName string,
	organizations []string,
	owner *metav1.OwnerReference) (*triple.KeyPair, error) {
	keyPair, err := GetKeyPair(client, namespace, privateKeySecret, certConfigKey)
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

	if err = SaveKeyPair(client, namespace, privateKeySecret, certConfigKey, keyPair, owner); err != nil {
		return nil, errors.Wrap(err, "Error saving client key pair")
	}

	return keyPair, nil
}

// GetKeyPair will return the KeyPair if it exists
func GetKeyPair(client kubernetes.Interface, namespace, privateKeySecret, certConfigKey string) (*triple.KeyPair, error) {
	key, err := GetPrivateKey(client, namespace, privateKeySecret)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting private key secret")
	}

	cert, err := GetCertificate(client, namespace, certConfigKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting cert")
	}

	if key == nil && cert == nil {
		// both nil is okay
		return nil, nil
	}

	if key == nil || cert == nil {
		return nil, errors.Errorf("One of key or cert exists key: %b, cert: %b", (key == nil), (cert == nil))
	}

	return &triple.KeyPair{
		Key:  key,
		Cert: cert,
	}, nil
}

// SaveKeyPair saves a private key and cert to kubernetes
func SaveKeyPair(client kubernetes.Interface, namespace, privateKeySecret, certConfigKey string, keyPair *triple.KeyPair, owner *metav1.OwnerReference) error {
	if err := SavePrivateKey(client, namespace, privateKeySecret, keyPair.Key, owner); err != nil {
		return errors.Wrap(err, "Error saving private key")
	}

	if err := SaveCertificate(client, namespace, certConfigKey, keyPair.Cert, owner); err != nil {
		return errors.Wrap(err, "Error saving certificate")
	}

	return nil
}

// GetPrivateKey returns a private key from kubernetes
func GetPrivateKey(client kubernetes.Interface, namespace, secretName string) (*rsa.PrivateKey, error) {
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

		if key, ok := obj.(*rsa.PrivateKey); ok {
			return key, nil
		}
		return nil, errors.New("Invalid pem format")
	}

	return nil, errors.New("Key in secret not found")
}

// SavePrivateKey saves a private key to kubernetes
func SavePrivateKey(client kubernetes.Interface, namespace, secretName string, privateKey *rsa.PrivateKey, owner *metav1.OwnerReference) error {
	privateKeyBytes := cert.EncodePrivateKeyPEM(privateKey)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			PrivateKeyKeyName: privateKeyBytes,
		},
	}

	if owner != nil {
		secret.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	_, err := client.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
		return errors.Wrap(err, "Error creating cert")
	}

	return nil
}

// GetCertificate gets a certificate in kubernetes
func GetCertificate(client kubernetes.Interface, namespace, configName string) (*x509.Certificate, error) {
	config, err := client.CoreV1().ConfigMaps(namespace).Get(configName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "Error getting configmap")
	}

	if str, ok := config.Data[CertKeyName]; ok {
		certs, err := cert.ParseCertsPEM([]byte(str))
		if err == nil && len(certs) > 0 {
			return certs[0], nil
		}
		return nil, errors.Errorf("Cert parse error %s, %d", err, len(certs))
	}

	return nil, errors.New("Config key missing")
}

// SaveCertificate saves a certificate in kubernetes
func SaveCertificate(client kubernetes.Interface, namespace, configName string, certificate *x509.Certificate, owner *metav1.OwnerReference) error {
	certString := string(cert.EncodeCertPEM(certificate))
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: namespace,
		},
		Data: map[string]string{
			CertKeyName: certString,
		},
	}

	if owner != nil {
		config.OwnerReferences = []metav1.OwnerReference{*owner}
	}

	_, err := client.CoreV1().ConfigMaps(namespace).Create(config)
	if err != nil {
		return errors.Wrap(err, "Error creating configmap")
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
