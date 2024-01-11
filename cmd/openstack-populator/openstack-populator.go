package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog/v2"
)

const (
	regionName                  = "regionName"
	authTypeString              = "authType"
	username                    = "username"
	userID                      = "userID"
	password                    = "password"
	applicationCredentialID     = "applicationCredentialID"
	applicationCredentialName   = "applicationCredentialName"
	applicationCredentialSecret = "applicationCredentialSecret"
	token                       = "token"
	systemScope                 = "systemScope"
	projectName                 = "projectName"
	projectID                   = "projectID"
	userDomainName              = "userDomainName"
	userDomainID                = "userDomainID"
	projectDomainName           = "projectDomainName"
	projectDomainID             = "projectDomainID"
	domainName                  = "domainName"
	domainID                    = "domainID"
	defaultDomain               = "defaultDomain"
	insecureSkipVerify          = "insecureSkipVerify"
	caCert                      = "cacert"
	endpointAvailability        = "availability"
)

const (
	unsupportedAuthTypeErrStr = "unsupported authentication type"
	malformedCAErrStr         = "CA certificate is malformed, failed to configure the CA cert pool"
)

var supportedAuthTypes = map[string]clientconfig.AuthType{
	"password":              clientconfig.AuthPassword,
	"token":                 clientconfig.AuthToken,
	"applicationcredential": clientconfig.AuthV3ApplicationCredential,
}

func main() {
	var (
		identityEndpoint string
		imageID          string
		secretName       string

		volumePath string
	)

	klog.InitFlags(nil)

	// Main arg
	flag.StringVar(&identityEndpoint, "endpoint", "", "endpoint URL (https://openstack.example.com:5000/v2.0)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing OpenStack credentials")
	flag.StringVar(&imageID, "image-id", "", "Openstack image ID")
	flag.StringVar(&volumePath, "volume-path", "", "Path to populate")

	flag.Parse()

	populate(volumePath, identityEndpoint, secretName, imageID)
}

func populate(fileName, identityEndpoint, secretName, imageID string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":2112", nil); err != nil {
			klog.Error(err)
		}
	}()
	progressGague := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "volume_populators",
			Name:      "openstack_volume_populator",
			Help:      "Amount of data transferred",
		},
		[]string{"image_id"},
	)

	if err := prometheus.Register(progressGague); err != nil {
		klog.Error("Prometheus progress counter not registered:", err)
	} else {
		klog.Info("Prometheus progress counter registered.")
	}

	availability := gophercloud.AvailabilityPublic
	if a := getStringFromSecret(endpointAvailability); a != "" {
		availability = gophercloud.Availability(a)
	}
	endpointOpts := gophercloud.EndpointOpts{
		Region:       getStringFromSecret(regionName),
		Availability: availability,
	}
	provider, err := getProviderClient(identityEndpoint)
	if err != nil {
		klog.Fatal(err)
	}
	imageService, err := openstack.NewImageServiceV2(provider, endpointOpts)
	if err != nil {
		klog.Fatal(err)
	}
	image, err := imagedata.Download(imageService, imageID).Extract()
	if err != nil {
		klog.Fatal(err)
	}
	defer image.Close()

	if err != nil {
		klog.Fatal(err)
	}
	flags := os.O_RDWR
	if strings.HasSuffix(fileName, "disk.img") {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(fileName, flags, 0650)
	if err != nil {
		klog.Fatal(err)
	}
	defer f.Close()

	err = writeData(image, f, imageID, progressGague)
	if err != nil {
		klog.Fatal(err)
	}
}

type countingReader struct {
	reader io.ReadCloser
	total  *int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	*cr.total += int64(n)
	return n, err
}

func writeData(reader io.ReadCloser, file *os.File, imageID string, progress *prometheus.GaugeVec) error {
	total := new(int64)
	countingReader := countingReader{reader, total}

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				progress.WithLabelValues(imageID).Set(float64(*total))
				klog.Info("Transferred: ", *total)
				time.Sleep(3 * time.Second)
			}
		}
	}()

	if _, err := io.Copy(file, &countingReader); err != nil {
		klog.Fatal(err)
	}
	done <- true
	progress.WithLabelValues(imageID).Set(float64(*total))

	return nil
}

func getAuthType() (authType clientconfig.AuthType, err error) {
	if configuredAuthType := getStringFromSecret(authTypeString); configuredAuthType == "" {
		authType = clientconfig.AuthPassword
	} else if supportedAuthType, found := supportedAuthTypes[configuredAuthType]; found {
		authType = supportedAuthType
	} else {
		err = errors.New(unsupportedAuthTypeErrStr)
		klog.Fatal(err.Error(), "authType", configuredAuthType)
	}
	return
}

func getStringFromSecret(key string) string {
	value, err := os.ReadFile(fmt.Sprintf("/etc/secret-volume/%s", key))
	if err != nil {
		klog.Info(err.Error())
		return ""
	}
	return string(value)
}

func getBoolFromSecret(key string) bool {
	if keyStr := getStringFromSecret(key); keyStr != "" {
		value, err := strconv.ParseBool(keyStr)
		if err != nil {
			return false
		}
		return value
	}
	return false
}

func getProviderClient(identityEndpoint string) (provider *gophercloud.ProviderClient, err error) {

	authInfo := &clientconfig.AuthInfo{
		AuthURL:           identityEndpoint,
		ProjectName:       getStringFromSecret(projectName),
		ProjectID:         getStringFromSecret(projectID),
		UserDomainName:    getStringFromSecret(userDomainName),
		UserDomainID:      getStringFromSecret(userDomainID),
		ProjectDomainName: getStringFromSecret(projectDomainName),
		ProjectDomainID:   getStringFromSecret(projectDomainID),
		DomainName:        getStringFromSecret(domainName),
		DomainID:          getStringFromSecret(domainID),
		DefaultDomain:     getStringFromSecret(defaultDomain),
		AllowReauth:       true,
	}

	var authType clientconfig.AuthType
	authType, err = getAuthType()
	if err != nil {
		klog.Fatal(err.Error())
		return
	}

	switch authType {
	case clientconfig.AuthPassword:
		authInfo.Username = getStringFromSecret(username)
		authInfo.UserID = getStringFromSecret(userID)
		authInfo.Password = getStringFromSecret(password)
	case clientconfig.AuthToken:
		authInfo.Token = getStringFromSecret(token)
	case clientconfig.AuthV3ApplicationCredential:
		authInfo.Username = getStringFromSecret(username)
		authInfo.ApplicationCredentialID = getStringFromSecret(applicationCredentialID)
		authInfo.ApplicationCredentialName = getStringFromSecret(applicationCredentialName)
		authInfo.ApplicationCredentialSecret = getStringFromSecret(applicationCredentialSecret)
	}

	identityURL, err := url.Parse(identityEndpoint)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}

	var TLSClientConfig *tls.Config
	if identityURL.Scheme == "https" {
		if getBoolFromSecret(insecureSkipVerify) {
			TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			cacert := []byte(getStringFromSecret(caCert))
			if len(cacert) == 0 {
				klog.Info("CA certificate was not provided,system CA cert pool is used")
			} else {
				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM(cacert)
				if !ok {
					err = errors.New(malformedCAErrStr)
					klog.Fatal(err.Error())
					return
				}
				TLSClientConfig = &tls.Config{RootCAs: roots}
			}

		}
	}

	provider, err = openstack.NewClient(identityEndpoint)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}

	provider.HTTPClient.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       TLSClientConfig,
	}

	clientOpts := &clientconfig.ClientOpts{
		AuthType: authType,
		AuthInfo: authInfo,
	}

	opts, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}

	err = openstack.Authenticate(provider, *opts)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}
	return
}
