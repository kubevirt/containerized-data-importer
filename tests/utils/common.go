package utils

import (
	"fmt"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
)

var (
	// DefaultNodeName the default node to use in tests
	DefaultNodeName string
	// DefaultStorageClass the defauld storage class used in tests
	DefaultStorageClass *storagev1.StorageClass
	// NfsService is the service in the cdi namespace that will be created if KUBEVIRT_STORAGE=nfs
	NfsService *corev1.Service
	nfsChecked bool
)

func getDefaultNodeName(client *kubernetes.Clientset) string {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		ginkgo.Fail("Unable to list nodes")
	}
	for _, node := range nodes.Items {
		scheduleable := true
		if node.Spec.Taints != nil {
			for _, taint := range node.Spec.Taints {
				if taint.Effect == corev1.TaintEffectNoSchedule {
					scheduleable = false
				}
			}
		}
		if node.GetLabels() != nil {
			if val, ok := node.GetLabels()["node-role.kubernetes.io/master"]; ok && val == "true" {
				scheduleable = false
			}
		}
		if scheduleable {
			ginkgo.GinkgoWriter.Write([]byte(fmt.Sprintf("Using node %s as target node for hostpath provisioner if set.\n", node.Name)))
			return node.Name
		}
	}
	return ""
}

func getDefaultStorageClass(client *kubernetes.Clientset) *storagev1.StorageClass {
	storageclasses, err := client.StorageV1().StorageClasses().List(metav1.ListOptions{})
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

// IsHostpathProvisioner returns true if hostpath-provisioner is the default storage class
func IsHostpathProvisioner() bool {
	if DefaultStorageClass == nil {
		return false
	}
	return DefaultStorageClass.Provisioner == "kubevirt.io/hostpath-provisioner"
}

// AddProvisionOnNodeToAnn adds 'kubevirt.io/provisionOnNode' annotaion
func AddProvisionOnNodeToAnn(pvcAnn map[string]string) {
	if DefaultNodeName == "" {
		ginkgo.Skip("No scheduleable nodes available")
	}
	pvcAnn["kubevirt.io/provisionOnNode"] = DefaultNodeName
}

// CacheTestsData fetch and cache data required for tests
func CacheTestsData(client *kubernetes.Clientset, cdiNs string) {
	if DefaultNodeName == "" {
		DefaultNodeName = getDefaultNodeName(client)
	}
	if DefaultStorageClass == nil {
		DefaultStorageClass = getDefaultStorageClass(client)
	}
	if !nfsChecked {
		NfsService = getNfsService(client, cdiNs)
		nfsChecked = true
	}
}

func getNfsService(client *kubernetes.Clientset, cdiNs string) *corev1.Service {
	service, err := client.CoreV1().Services(cdiNs).Get("nfs-service", metav1.GetOptions{})
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
