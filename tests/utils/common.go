package utils

import (
	"context"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
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

// GetTestNamespaceList returns a list of namespaces that have been created by the functional tests.
func GetTestNamespaceList(client *kubernetes.Clientset, nsPrefix string) (*corev1.NamespaceList, error) {
	//Ensure that no namespaces with the prefix label exist
	return client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: nsPrefix,
	})
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

// UpdateCDIConfigWithOptions updates CDIConfig with specific UpdateOptions
func UpdateCDIConfigWithOptions(c client.Client, opts metav1.UpdateOptions, updateFunc func(*cdiv1.CDIConfigSpec)) error {
	cdi, err := controller.GetActiveCDI(c)
	if err != nil {
		return err
	}

	if cdi.Spec.Config == nil {
		cdi.Spec.Config = &cdiv1.CDIConfigSpec{}
	}

	updateFunc(cdi.Spec.Config)

	if err = c.Update(context.TODO(), cdi, &client.UpdateOptions{Raw: &opts}); err != nil {
		return err
	}

	if err = wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
		cfg := &cdiv1.CDIConfig{}
		err := c.Get(context.TODO(), types.NamespacedName{Name: "config"}, cfg)
		return apiequality.Semantic.DeepEqual(&cfg.Spec, cdi.Spec.Config), err
	}); err != nil {
		return err
	}

	return nil
}

// UpdateCDIConfig updates CDIConfig
func UpdateCDIConfig(c client.Client, updateFunc func(*cdiv1.CDIConfigSpec)) error {
	return UpdateCDIConfigWithOptions(c, metav1.UpdateOptions{}, updateFunc)
}

// EnableFeatureGate sets specified FeatureGate in the CDIConfig
func EnableFeatureGate(c client.Client, feature string) (*bool, error) {
	var previousValue = false

	if err := UpdateCDIConfig(c, func(config *cdiv1.CDIConfigSpec) {
		if HasFeature(config, feature) {
			previousValue = true
			return
		}

		config.FeatureGates = append(config.FeatureGates, feature)
	}); err != nil {
		return nil, err
	}

	return &previousValue, nil
}

// DisableFeatureGate unsets specified FeatureGate in the CDIConfig
func DisableFeatureGate(c client.Client, featureGate string) (*bool, error) {
	var previousValue = false

	if err := UpdateCDIConfig(c, func(config *cdiv1.CDIConfigSpec) {
		if !HasFeature(config, featureGate) {
			return
		}

		previousValue = true
		config.FeatureGates = removeString(config.FeatureGates, featureGate)
	}); err != nil {
		return nil, err
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
func HasFeature(config *cdiv1.CDIConfigSpec, featureGate string) bool {
	for _, fg := range config.FeatureGates {
		if fg == featureGate {
			return true
		}
	}

	return false
}

//IsOpenshift checks if we are on OpenShift platform
func IsOpenshift(client kubernetes.Interface) bool {
	//OpenShift 3.X check
	result := client.Discovery().RESTClient().Get().AbsPath("/oapi/v1").Do(context.TODO())
	var statusCode int
	result.StatusCode(&statusCode)

	if result.Error() == nil {
		// It is OpenShift
		if statusCode == http.StatusOK {
			return true
		}
	} else {
		// Got 404 so this is not Openshift 3.X, let's check OpenShift 4
		result = client.Discovery().RESTClient().Get().AbsPath("/apis/route.openshift.io").Do(context.TODO())
		var statusCode int
		result.StatusCode(&statusCode)

		if result.Error() == nil {
			// It is OpenShift
			if statusCode == http.StatusOK {
				return true
			}
		}
	}

	return false
}
