package generator

import (
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
)

// CertGenerator is an interface for creating certs
type CertGenerator interface {
	MakeClientCert(name string, groups []string, duration time.Duration) ([]byte, []byte, error)
	MakeServerCert(namespace, service string, duration time.Duration) ([]byte, []byte, error)
}

// FetchCertGenerator fetches and generates certs
type FetchCertGenerator struct {
	Fetcher fetcher.CertFetcher
}

// MakeClientCert generates a client cert
func (cg *FetchCertGenerator) MakeClientCert(name string, groups []string, duration time.Duration) ([]byte, []byte, error) {
	ca, err := cg.getCA()
	if err != nil {
		return nil, nil, err
	}

	userInfo := &user.DefaultInfo{Name: name, Groups: groups}
	certKeyPair, err := ca.MakeClientCertificateForDuration(userInfo, duration)
	if err != nil {
		return nil, nil, err
	}

	return certKeyPair.GetPEMBytes()
}

// MakeServerCert generates a server cert
func (cg *FetchCertGenerator) MakeServerCert(namespace, service string, duration time.Duration) ([]byte, []byte, error) {
	ca, err := cg.getCA()
	if err != nil {
		return nil, nil, err
	}

	hostnames := sets.NewString(serviceToHostnames(namespace, service)...)
	certKeyPair, err := ca.MakeServerCertForDuration(hostnames, duration)
	if err != nil {
		return nil, nil, err
	}
	return certKeyPair.GetPEMBytes()
}

func (cg *FetchCertGenerator) getCA() (*crypto.CA, error) {
	cert, err := cg.Fetcher.CertBytes()
	if err != nil {
		return nil, err
	}

	key, err := cg.Fetcher.KeyBytes()
	if err != nil {
		return nil, err
	}

	ca, err := crypto.GetCAFromBytes(cert, key)
	if err != nil {
		return nil, err
	}

	return ca, nil
}

func serviceToHostnames(namespace, service string) []string {
	return []string{
		service,
		fmt.Sprintf("%s.%s", service, namespace),
		fmt.Sprintf("%s.%s.svc", service, namespace),
	}
}
