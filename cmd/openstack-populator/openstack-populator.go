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
	RegionName                  = "regionName"
	AuthType                    = "authType"
	Username                    = "username"
	UserID                      = "userID"
	Password                    = "password"
	ApplicationCredentialID     = "applicationCredentialID"
	ApplicationCredentialName   = "applicationCredentialName"
	ApplicationCredentialSecret = "applicationCredentialSecret"
	Token                       = "token"
	SystemScope                 = "systemScope"
	ProjectName                 = "projectName"
	ProjectID                   = "projectID"
	UserDomainName              = "userDomainName"
	UserDomainID                = "userDomainID"
	ProjectDomainName           = "projectDomainName"
	ProjectDomainID             = "projectDomainID"
	DomainName                  = "domainName"
	DomainID                    = "domainID"
	DefaultDomain               = "defaultDomain"
	InsecureSkipVerify          = "insecureSkipVerify"
	CACert                      = "cacert"
	EndpointAvailability        = "availability"
)

const (
	UnsupportedAuthTypeErrStr = "unsupported authentication type"
	MalformedCAErrStr         = "CA certificate is malformed, failed to configure the CA cert pool"
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
		crNamespace      string
		crName           string
		secretName       string

		volumePath string
	)

	klog.InitFlags(nil)

	// Main arg
	flag.StringVar(&identityEndpoint, "endpoint", "", "endpoint URL (https://openstack.example.com:5000/v2.0)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing OpenStack credentials")

	flag.StringVar(&imageID, "image-id", "", "Openstack image ID")
	flag.StringVar(&volumePath, "volume-path", "", "Path to populate")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")

	flag.Parse()

	populate(volumePath, identityEndpoint, secretName, imageID)
}

func populate(fileName, identityEndpoint, secretName, imageID string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)
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
	if a := getStringFromSecret(EndpointAvailability); a != "" {
		availability = gophercloud.Availability(a)
	}
	endpointOpts := gophercloud.EndpointOpts{
		Region:       getStringFromSecret(RegionName),
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

type CountingReader struct {
	reader io.ReadCloser
	total  *int64
}

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	*cr.total += int64(n)
	return n, err
}

func writeData(reader io.ReadCloser, file *os.File, imageID string, progress *prometheus.GaugeVec) error {
	total := new(int64)
	countingReader := CountingReader{reader, total}

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
	if configuredAuthType := getStringFromSecret(AuthType); configuredAuthType == "" {
		authType = clientconfig.AuthPassword
	} else if supportedAuthType, found := supportedAuthTypes[configuredAuthType]; found {
		authType = supportedAuthType
	} else {
		err = errors.New(UnsupportedAuthTypeErrStr)
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
		ProjectName:       getStringFromSecret(ProjectName),
		ProjectID:         getStringFromSecret(ProjectID),
		UserDomainName:    getStringFromSecret(UserDomainName),
		UserDomainID:      getStringFromSecret(UserDomainID),
		ProjectDomainName: getStringFromSecret(ProjectDomainName),
		ProjectDomainID:   getStringFromSecret(ProjectDomainID),
		DomainName:        getStringFromSecret(DomainName),
		DomainID:          getStringFromSecret(DomainID),
		DefaultDomain:     getStringFromSecret(DefaultDomain),
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
		authInfo.Username = getStringFromSecret(Username)
		authInfo.UserID = getStringFromSecret(UserID)
		authInfo.Password = getStringFromSecret(Password)
	case clientconfig.AuthToken:
		authInfo.Token = getStringFromSecret(Token)
	case clientconfig.AuthV3ApplicationCredential:
		authInfo.Username = getStringFromSecret(Username)
		authInfo.ApplicationCredentialID = getStringFromSecret(ApplicationCredentialID)
		authInfo.ApplicationCredentialName = getStringFromSecret(ApplicationCredentialName)
		authInfo.ApplicationCredentialSecret = getStringFromSecret(ApplicationCredentialSecret)
	}

	identityUrl, err := url.Parse(identityEndpoint)
	if err != nil {
		klog.Fatal(err.Error())
		return
	}

	var TLSClientConfig *tls.Config
	if identityUrl.Scheme == "https" {
		if getBoolFromSecret(InsecureSkipVerify) {
			TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			cacert := []byte(getStringFromSecret(CACert))
			if len(cacert) == 0 {
				klog.Info("CA certificate was not provided,system CA cert pool is used")
			} else {
				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM(cacert)
				if !ok {
					err = errors.New(MalformedCAErrStr)
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
