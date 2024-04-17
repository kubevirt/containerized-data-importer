package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
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
	dto "github.com/prometheus/client_model/go"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"

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

type appConfig struct {
	identityEndpoint string
	imageID          string
	secretName       string
	ownerUID         string
	pvcSize          int64
	volumePath       string
}

type countingReader struct {
	reader io.ReadCloser
	total  int64
	read   *int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	cr.total += int64(n)
	return n, err
}

func main() {
	klog.InitFlags(nil)

	config := &appConfig{}
	flag.StringVar(&config.identityEndpoint, "endpoint", "", "endpoint URL (https://openstack.example.com:5000/v2.0)")
	flag.StringVar(&config.secretName, "secret-name", "", "secret containing OpenStack credentials")
	flag.StringVar(&config.imageID, "image-id", "", "Openstack image ID")
	flag.StringVar(&config.volumePath, "volume-path", "", "Path to populate")
	flag.StringVar(&config.ownerUID, "owner-uid", "", "Owner UID (usually PVC UID)")
	flag.Int64Var(&config.pvcSize, "pvc-size", 0, "Size of pvc (in bytes)")
	flag.Parse()

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(certsDirectory)

	prometheusutil.StartPrometheusEndpoint("/certsdir")

	populate(config)
}

func populate(config *appConfig) {
	provider, err := getProviderClient(config.identityEndpoint)
	if err != nil {
		klog.Fatal(err)
	}

	imageReader, err := setupImageService(provider, config)
	if err != nil {
		klog.Fatal(err)
	}
	defer imageReader.Close()

	downloadAndSaveImage(config, imageReader)
}

func downloadAndSaveImage(config *appConfig, imageReader io.ReadCloser) {
	klog.Info("Downloading the image: ", config.imageID)
	file := openFile(config.volumePath)
	defer file.Close()

	progress := createProgressCounter()
	writeData(imageReader, file, config, progress)
}

func setupImageService(provider *gophercloud.ProviderClient, config *appConfig) (io.ReadCloser, error) {
	imageService, err := openstack.NewImageServiceV2(provider, getEndpointOpts())
	if err != nil {
		return nil, err
	}

	imageReader, err := imagedata.Download(imageService, config.imageID).Extract()
	if err != nil {
		return nil, err
	}

	return imageReader, nil
}

func getEndpointOpts() gophercloud.EndpointOpts {
	availability := gophercloud.AvailabilityPublic
	if a := getStringFromSecret(endpointAvailability); a != "" {
		availability = gophercloud.Availability(a)
	}

	return gophercloud.EndpointOpts{
		Region:       getStringFromSecret(regionName),
		Availability: availability,
	}
}

func writeData(reader io.ReadCloser, file *os.File, config *appConfig, progress *prometheus.CounterVec) {
	countingReader := &countingReader{reader: reader, total: config.pvcSize, read: new(int64)}
	done := make(chan bool)

	go reportProgress(done, countingReader, progress, config)

	if _, err := io.Copy(file, countingReader); err != nil {
		klog.Fatal(err)
	}
	done <- true
}

func reportProgress(done chan bool, countingReader *countingReader, progress *prometheus.CounterVec, config *appConfig) {
	for {
		select {
		case <-done:
			finalizeProgress(progress, config.ownerUID)
			return
		default:
			updateProgress(countingReader, progress, config.ownerUID)
			time.Sleep(1 * time.Second)
		}
	}
}

func createProgressCounter() *prometheus.CounterVec {
	progressVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "openstack_populator_progress",
			Help: "Progress of volume population",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progressVec); err != nil {
		klog.Error("Prometheus progress counter not registered:", err)
	} else {
		klog.Info("Prometheus progress counter registered.")
	}
	return progressVec
}

func finalizeProgress(progress *prometheus.CounterVec, ownerUID string) {
	currentVal := progress.WithLabelValues(ownerUID)

	var metric dto.Metric
	if err := currentVal.Write(&metric); err != nil {
		klog.Error("Error reading current progress:", err)
		return
	}

	if metric.Counter != nil {
		remainingProgress := 100 - *metric.Counter.Value
		if remainingProgress > 0 {
			currentVal.Add(remainingProgress)
		}
	}

	klog.Info("Finished populating the volume. Progress: 100%")
}

func updateProgress(countingReader *countingReader, progress *prometheus.CounterVec, ownerUID string) {
	if countingReader.total <= 0 {
		return
	}

	metric := &dto.Metric{}
	if err := progress.WithLabelValues(ownerUID).Write(metric); err != nil {
		klog.Errorf("updateProgress: failed to write metric; %v", err)
	}

	currentProgress := (float64(*countingReader.read) / float64(countingReader.total)) * 100

	if currentProgress > *metric.Counter.Value {
		progress.WithLabelValues(ownerUID).Add(currentProgress - *metric.Counter.Value)
	}

	klog.Info("Progress: ", int64(currentProgress), "%")
}

func openFile(volumePath string) *os.File {
	flags := os.O_RDWR
	if strings.HasSuffix(volumePath, "disk.img") {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(volumePath, flags, 0650)
	if err != nil {
		klog.Fatal(err)
	}
	return file
}

func getAuthType() (clientconfig.AuthType, error) {
	configuredAuthType := getStringFromSecret(authTypeString)
	if configuredAuthType == "" {
		return clientconfig.AuthPassword, nil
	}

	if supportedAuthType, found := supportedAuthTypes[configuredAuthType]; found {
		return supportedAuthType, nil
	}

	err := errors.New(unsupportedAuthTypeErrStr)
	klog.Fatal(err.Error(), "authType", configuredAuthType)
	return clientconfig.AuthType(""), err
}

func getStringFromSecret(key string) string {
	value := os.Getenv(key)
	return value
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

func getProviderClient(identityEndpoint string) (*gophercloud.ProviderClient, error) {
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
	authType, err := getAuthType()
	if err != nil {
		klog.Fatal(err.Error())
		return nil, err
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
		return nil, err
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
					return nil, err
				}
				TLSClientConfig = &tls.Config{RootCAs: roots}
			}
		}
	}

	provider, err := openstack.NewClient(identityEndpoint)
	if err != nil {
		klog.Fatal(err.Error())
		return nil, err
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
		return nil, err
	}

	err = openstack.Authenticate(provider, *opts)
	if err != nil {
		klog.Fatal(err.Error())
		return nil, err
	}
	return provider, nil
}
