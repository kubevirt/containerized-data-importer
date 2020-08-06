package utils

import (
	"context"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

// cdi-file-host pod/service relative values
const (
	//RegistryHostName provides a deploymnet and service name for registry
	RegistryHostName = "cdi-docker-registry-host"
	// FileHostName provides a deployment and service name for tests
	FileHostName = "cdi-file-host"
	// FileHostS3Bucket provides an S3 bucket name for tests (e.g. http://<serviceIP:port>/FileHostS3Bucket/image)
	FileHostS3Bucket = "images"
	// AccessKeyValue provides a username to use for http and S3 (see hack/build/docker/cdi-func-test-file-host-http/htpasswd)
	AccessKeyValue = "admin"
	// SecretKeyValue provides a password to use for http and S3 (see hack/build/docker/cdi-func-test-file-host-http/htpasswd)
	SecretKeyValue = "password"
	// HttpAuthPort provides a cdi-file-host service auth port for tests
	HTTPAuthPort = 81
	// HttpNoAuthPort provides a cdi-file-host service no-auth port for tests, requires AccessKeyValue and SecretKeyValue
	HTTPNoAuthPort = 80
	// HTTPRateLimitPort provides a cdi-file-host service rate limit port for tests, speed is limited to 25k/s to allow for testing slow connection behavior. No auth.
	HTTPRateLimitPort = 82
	// S3Port provides a cdi-file-host service S3 port, requires AccessKey and SecretKeyValue
	S3Port = 9000
	// HTTPSPort is the https port of cdi-file-host
	HTTPSNoAuthPort = 443
	// RegistryCertConfigMap is the ConfigMap where the cert for the docker registry is stored
	RegistryCertConfigMap = "cdi-docker-registry-host-certs"
	// FileHostCertConfigMap is the ConfigMap where the cert fir the file host is stored
	FileHostCertConfigMap = "cdi-file-host-certs"
	// ImageIOCertConfigMap is the ConfigMap where the cert fir the file host is stored
	ImageIOCertConfigMap = "imageio-certs"
)

var (
	// DefaultStorageClass the defauld storage class used in tests
	DefaultStorageClass *storagev1.StorageClass
	// NfsService is the service in the cdi namespace that will be created if KUBEVIRT_STORAGE=nfs
	NfsService *corev1.Service
	nfsChecked bool
	// DefaultStorageCSI is true if the default storage class is CSI, false other wise.
	DefaultStorageCSI bool
)

func getDefaultStorageClass(client *kubernetes.Clientset) *storagev1.StorageClass {
	storageclasses, err := client.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ginkgo.Fail("Unable to list storage classes")
		return nil
	}
	for _, storageClass := range storageclasses.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			return &storageClass
		}
	}
	ginkgo.Fail("Unable to find default storage classes")
	return nil
}

func isDefaultStorageClassCSI(client *kubernetes.Clientset) bool {
	if DefaultStorageClass != nil {
		_, err := client.StorageV1beta1().CSIDrivers().Get(context.TODO(), DefaultStorageClass.Provisioner, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return true
	}
	return false
}

// IsHostpathProvisioner returns true if hostpath-provisioner is the default storage class
func IsHostpathProvisioner() bool {
	if DefaultStorageClass == nil {
		return false
	}
	return DefaultStorageClass.Provisioner == "kubevirt.io/hostpath-provisioner"
}

// CacheTestsData fetch and cache data required for tests
func CacheTestsData(client *kubernetes.Clientset, cdiNs string) {
	if DefaultStorageClass == nil {
		DefaultStorageClass = getDefaultStorageClass(client)
	}
	DefaultStorageCSI = isDefaultStorageClassCSI(client)
	if !nfsChecked {
		NfsService = getNfsService(client, cdiNs)
		nfsChecked = true
	}
}

func getNfsService(client *kubernetes.Clientset, cdiNs string) *corev1.Service {
	service, err := client.CoreV1().Services(cdiNs).Get(context.TODO(), "nfs-service", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return service
}

// IsNfs returns true if the default storage class is the static nfs storage class with no provisioner
func IsNfs() bool {
	if NfsService == nil {
		return false
	}
	return true
}

// EnableFeatureGate sets specified FeatureGate in the CDIConfig
func EnableFeatureGate(client *cdiclientset.Clientset, feature string) (*bool, error) {
	var previousValue = false
	config, err := client.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if HasFeature(config, feature) {
		previousValue = true
		return &previousValue, nil
	}

	config.Spec.FeatureGates = append(config.Spec.FeatureGates, feature)
	config, err = client.CdiV1beta1().CDIConfigs().Update(context.TODO(), config, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return &previousValue, nil
}

// DisableFeatureGate unsets specified FeatureGate in the CDIConfig
func DisableFeatureGate(client *cdiclientset.Clientset, featureGate string) (*bool, error) {
	var previousValue = false
	config, err := client.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !HasFeature(config, featureGate) {
		return &previousValue, nil
	}

	previousValue = true
	config.Spec.FeatureGates = removeString(config.Spec.FeatureGates, featureGate)
	config, err = client.CdiV1beta1().CDIConfigs().Update(context.TODO(), config, metav1.UpdateOptions{})
	if err != nil {
		return nil, nil
	}

	return &previousValue, nil
}

func removeString(featureGates []string, featureGate string) []string {
	var output []string
	for _, fg := range featureGates {
		if fg != featureGate {
			output = append(output, fg)
		}
	}
	return output
}

// HasFeature - helper to check if specified FeatureGate is in the CDIConfig
func HasFeature(config *cdiv1.CDIConfig, featureGate string) bool {
	for _, fg := range config.Spec.FeatureGates {
		if fg == featureGate {
			return true
		}
	}

	return false
}
