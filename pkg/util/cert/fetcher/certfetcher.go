package fetcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	baseCertPath = "/var/run/certs"
)

// CertFetcher is an interface for retreiving certs
type CertFetcher interface {
	KeyBytes() ([]byte, error)
	CertBytes() ([]byte, error)
}

// FileCertFetcher reads certs from files
type FileCertFetcher struct {
	Name string
}

// KeyBytes returns key bytes
func (f *FileCertFetcher) KeyBytes() ([]byte, error) {
	return ioutil.ReadFile(path.Join(baseCertPath, f.Name, "tls.key"))
}

// CertBytes returns cert bytes
func (f *FileCertFetcher) CertBytes() ([]byte, error) {
	return ioutil.ReadFile(path.Join(baseCertPath, f.Name, "tls.crt"))
}

// MemCertFetcher reads certs from files
type MemCertFetcher struct {
	Cert, Key []byte
}

// KeyBytes returns key bytes
func (f *MemCertFetcher) KeyBytes() ([]byte, error) {
	return f.Key, nil
}

// CertBytes returns cert bytes
func (f *MemCertFetcher) CertBytes() ([]byte, error) {
	return f.Cert, nil
}

// CertBundleFetcher is an interface for retreiving CA Certbundles
type CertBundleFetcher interface {
	BundleBytes() ([]byte, error)
}

// ConfigMapCertBundleFetcher reads bundles from ConfigMaps
type ConfigMapCertBundleFetcher struct {
	Name   string
	Client corev1.ConfigMapInterface
}

// BundleBytes returns bundle bytes
func (f *ConfigMapCertBundleFetcher) BundleBytes() ([]byte, error) {
	cm, err := f.Client.Get(context.TODO(), f.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	bundle, ok := cm.Data["ca-bundle.crt"]
	if !ok {
		return nil, fmt.Errorf("CA bundle missing")
	}

	return []byte(bundle), err
}

// MemCertBundleFetcher reads bundles
type MemCertBundleFetcher struct {
	Bundle []byte
}

// BundleBytes returns bundle bytes
func (f *MemCertBundleFetcher) BundleBytes() ([]byte, error) {
	return f.Bundle, nil
}
