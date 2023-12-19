package framework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	nsCreateTime = 60 * time.Second
	nsDeleteTime = 5 * time.Minute
	//NsPrefixLabel provides a cdi prefix label to identify the test namespace
	NsPrefixLabel = "cdi-e2e"
	cdiPodPrefix  = "cdi-deployment"
)

// run-time flags
var (
	ClientsInstance = &Clients{}
)

// Config provides some basic test config options
type Config struct {
	// SkipNamespaceCreation sets whether to skip creating a namespace. Use this ONLY for tests that do not require
	// a namespace at all, like basic sanity or other global tests.
	SkipNamespaceCreation bool

	// FeatureGates may be overridden for a framework
	FeatureGates []string
}

// Clients is the struct containing the client-go kubernetes clients
type Clients struct {
	KubectlPath    string
	OcPath         string
	CdiInstallNs   string
	KubeConfig     string
	KubeURL        string
	GoCLIPath      string
	SnapshotSCName string
	BlockSCName    string
	CsiCloneSCName string
	DockerPrefix   string
	DockerTag      string

	//  k8sClient provides our k8s client pointer
	K8sClient *kubernetes.Clientset
	// CdiClient provides our CDI client pointer
	CdiClient *cdiClientset.Clientset
	// ExtClient provides our CSI client pointer
	ExtClient *extclientset.Clientset
	// CrClient is a controller runtime client
	CrClient crclient.Client
	// RestConfig provides a pointer to our REST client config.
	RestConfig *rest.Config
	// DynamicClient performs generic operations on arbitrary k8s API objects.
	DynamicClient dynamic.Interface
}

// K8s returns Kubernetes Clientset
func (c *Clients) K8s() *kubernetes.Clientset {
	return c.K8sClient
}

// Cdi returns CDI Clientset
func (c *Clients) Cdi() *cdiClientset.Clientset {
	return c.CdiClient
}

// Framework supports common operations used by functional/e2e tests. It holds the k8s and cdi clients,
// a generated unique namespace, run-time flags, and more fields will be added over time as cdi e2e
// evolves. Global BeforeEach and AfterEach are called in the Framework constructor.
type Framework struct {
	Config
	// NsPrefix is a prefix for generated namespace
	NsPrefix string
	// Namespace provides a namespace for each test generated/unique ns per test
	Namespace *v1.Namespace
	// Namespace2 provides an additional generated/unique secondary ns for testing across namespaces (eg. clone tests)
	Namespace2         *v1.Namespace // note: not instantiated in NewFramework
	namespacesToDelete []*v1.Namespace

	// ControllerPod provides a pointer to our test controller pod
	ControllerPod *v1.Pod

	*Clients
}

// NewFramework calls NewFramework and handles errors by calling Fail. Config is optional, but
// if passed there can only be one.
// To understand the order in which things are run, read http://onsi.github.io/ginkgo/#understanding-ginkgos-lifecycle
// flag parsing happens AFTER ginkgo has constructed the entire testing tree. So anything that uses information from flags
// cannot work when called during test tree construction.
func NewFramework(prefix string, config ...Config) *Framework {
	cfg := Config{
		FeatureGates: []string{featuregates.HonorWaitForFirstConsumer},
	}
	if len(config) > 0 {
		cfg = config[0]
	}
	f := &Framework{
		Config:   cfg,
		NsPrefix: prefix,
		Clients:  ClientsInstance,
	}

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)
	return f
}

// BeforeEach provides a set of operations to run before each test
func (f *Framework) BeforeEach() {
	if !f.SkipNamespaceCreation {
		// generate unique primary ns (ns2 not created here)
		ginkgo.By(fmt.Sprintf("Building a %q namespace api object", f.NsPrefix))
		ns, err := f.CreateNamespace(f.NsPrefix, map[string]string{
			NsPrefixLabel: f.NsPrefix,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		f.Namespace = ns
		f.AddNamespaceToDelete(ns)
	}

	if f.ControllerPod == nil {
		pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiPodPrefix, common.CDILabelSelector)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Located cdi-controller-pod: %q\n", pod.Name)
		f.ControllerPod = pod
	}

	if utils.IsStaticNfsWithInternalClusterServer() {
		ginkgo.By("Creating NFS PVs before the test")
		gomega.Expect(createNFSPVs(f.K8sClient, f.CdiInstallNs)).To(gomega.Succeed())
	}

	f.updateCDIConfig()
}

// AfterEach provides a set of operations to run after each test
func (f *Framework) AfterEach() {
	// delete the namespace(s) in a defer in case future code added here could generate
	// an exception. For now there is only a defer.
	defer func() {
		for _, ns := range f.namespacesToDelete {
			defer func() { f.namespacesToDelete = nil }()
			if ns == nil || len(ns.Name) == 0 {
				continue
			}
			ginkgo.By(fmt.Sprintf("Destroying namespace %q for this suite.", ns.Name))
			err := DeleteNS(f.K8sClient, ns.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		if utils.IsStaticNfsWithInternalClusterServer() {
			ginkgo.By("Deleting NFS PVs after the test")
			gomega.Expect(deleteNFSPVs(f.K8sClient, f.CdiInstallNs)).To(gomega.Succeed())
			// manually clear out the NFS directories as we use delete retain policy.
			nfsServerPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "nfs-server", "app=nfs-server")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for i := 1; i <= pvCount; i++ {
				stdout, stderr, err := f.ExecShellInPod(nfsServerPod.Name, f.CdiInstallNs, fmt.Sprintf("/bin/rm -rf /data/nfs/disk%d/*", i))
				if err != nil {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: cleaning up nfs disk%d failed: %s, %s\n", i, stdout, stderr)
				}
			}
		}
	}()
}

// CreateNamespace instantiates a new namespace object with a unique name and the passed-in label(s).
func (f *Framework) CreateNamespace(prefix string, labels map[string]string) (*v1.Namespace, error) {
	if labels == nil {
		labels = make(map[string]string)
	}
	// pod-security.kubernetes.io/<MODE>: <LEVEL>
	labels["pod-security.kubernetes.io/enforce"] = "restricted"
	if utils.IsOpenshift(f.K8sClient) {
		labels["security.openshift.io/scc.podSecurityLabelSync"] = "false"
	}

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("cdi-e2e-tests-%s-", prefix),
			Namespace:    "",
			Labels:       labels,
		},
		Status: v1.NamespaceStatus{},
	}

	var nsObj *v1.Namespace
	c := f.K8sClient
	err := wait.PollImmediate(2*time.Second, nsCreateTime, func() (bool, error) {
		var err error
		nsObj, err = c.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil // done
		}
		klog.Warningf("Unexpected error while creating %q namespace: %v", ns.GenerateName, err)
		return false, err // keep trying
	})
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Created new namespace %q\n", nsObj.Name)
	return nsObj, nil
}

// AddNamespaceToDelete provides a wrapper around the go append function
func (f *Framework) AddNamespaceToDelete(ns *v1.Namespace) {
	f.namespacesToDelete = append(f.namespacesToDelete, ns)
}

// DeleteNS provides a function to delete the specified namespace from the test cluster
func DeleteNS(c *kubernetes.Clientset, ns string) error {
	// return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
	err := c.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return err
	}
	return nil
}

// GetDynamicClient gets an instance of a dynamic client that performs generic operations on arbitrary k8s API objects.
func (c *Clients) GetDynamicClient() (dynamic.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(c.KubeURL, c.KubeConfig)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return dyn, nil
}

// GetCdiClient gets an instance of a kubernetes client that includes all the CDI extensions.
func (c *Clients) GetCdiClient() (*cdiClientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(c.KubeURL, c.KubeConfig)
	if err != nil {
		return nil, err
	}
	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cdiClient, nil
}

// GetExtClient gets an instance of a kubernetes client that includes all the api extensions.
func (c *Clients) GetExtClient() (*extclientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(c.KubeURL, c.KubeConfig)
	if err != nil {
		return nil, err
	}
	extClient, err := extclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return extClient, nil
}

// GetCrClient returns a controller runtime client
func (c *Clients) GetCrClient() (crclient.Client, error) {
	if err := snapshotv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	if err := cdiv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	if err := promv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	client, err := crclient.New(c.RestConfig, crclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetRESTConfigForServiceAccount returns a RESTConfig for SA
func (f *Framework) GetRESTConfigForServiceAccount(namespace, name string) (*rest.Config, error) {
	token, err := f.GetTokenForServiceAccount(namespace, name)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:        f.RestConfig.Host,
		APIPath:     f.RestConfig.APIPath,
		BearerToken: string(token),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, nil
}

// GetTokenForServiceAccount returns a token for a given SA
func (f *Framework) GetTokenForServiceAccount(namespace, name string) (string, error) {
	token, err := f.K8sClient.CoreV1().ServiceAccounts(namespace).
		CreateToken(
			context.TODO(),
			name,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{},
			},
			metav1.CreateOptions{},
		)
	if err != nil {
		return "", err
	}

	return token.Status.Token, nil
}

// GetCdiClientForServiceAccount returns a cdi client for a service account
func (f *Framework) GetCdiClientForServiceAccount(namespace, name string) (*cdiClientset.Clientset, error) {
	cfg, err := f.GetRESTConfigForServiceAccount(namespace, name)
	if err != nil {
		return nil, err
	}

	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return cdiClient, nil
}

// GetKubeClient returns a Kubernetes rest client
func (c *Clients) GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromRESTConfig(c.RestConfig)
}

// LoadConfig loads our specified kubeconfig
func (c *Clients) LoadConfig() (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags(c.KubeURL, c.KubeConfig)
}

// GetKubeClientFromRESTConfig provides a function to get a K8s client using hte REST config
func GetKubeClientFromRESTConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	return kubernetes.NewForConfig(config)
}

// CreatePrometheusServiceInNs creates a service for prometheus in the specified namespace. This
// allows us to test for prometheus end points using the service to connect to the endpoints.
func (f *Framework) CreatePrometheusServiceInNs(namespace string) (*v1.Service, error) {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-prometheus-metrics",
			Namespace: namespace,
			Labels: map[string]string{
				common.PrometheusLabelKey: common.PrometheusLabelValue,
				"kubevirt.io":             "",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "metrics",
					Port: 8443,
					TargetPort: intstr.IntOrString{
						StrVal: "metrics",
					},
					Protocol: v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				common.PrometheusLabelKey: common.PrometheusLabelValue,
			},
		},
	}
	return f.K8sClient.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
}

// CreateQuotaInNs creates a quota and sets it on the current test namespace.
func (f *Framework) CreateQuotaInNs(requestCPU, requestMemory, limitsCPU, limitsMemory int64) error {
	return f.CreateQuotaInSpecifiedNs(f.Namespace.GetName(), requestCPU, requestMemory, limitsCPU, limitsMemory)
}

// CreateQuotaInSpecifiedNs creates a quota and sets it on the specified test namespace.
func (f *Framework) CreateQuotaInSpecifiedNs(ns string, requestCPU, requestMemory, limitsCPU, limitsMemory int64) error {
	resourceQuota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-quota",
			Namespace: ns,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceRequestsCPU:    *resource.NewQuantity(requestCPU, resource.DecimalSI),
				v1.ResourceRequestsMemory: *resource.NewQuantity(requestMemory, resource.DecimalSI),
				v1.ResourceLimitsCPU:      *resource.NewQuantity(limitsCPU, resource.DecimalSI),
				v1.ResourceLimitsMemory:   *resource.NewQuantity(limitsMemory, resource.DecimalSI),
			},
		},
	}
	_, err := f.K8sClient.CoreV1().ResourceQuotas(ns).Create(context.TODO(), resourceQuota, metav1.CreateOptions{})
	if err != nil {
		ginkgo.Fail("Unable to set resource quota " + err.Error())
	}
	return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
		quota, err := f.K8sClient.CoreV1().ResourceQuotas(ns).Get(context.TODO(), "test-quota", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return len(quota.Status.Used) == 4, nil
	})
}

// UpdateQuotaInNs updates an existing quota in the current test namespace.
func (f *Framework) UpdateQuotaInNs(requestCPU, requestMemory, limitsCPU, limitsMemory int64) error {
	resourceQuota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-quota",
			Namespace: f.Namespace.GetName(),
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceRequestsCPU:    *resource.NewQuantity(requestCPU, resource.DecimalSI),
				v1.ResourceRequestsMemory: *resource.NewQuantity(requestMemory, resource.DecimalSI),
				v1.ResourceLimitsCPU:      *resource.NewQuantity(limitsCPU, resource.DecimalSI),
				v1.ResourceLimitsMemory:   *resource.NewQuantity(limitsMemory, resource.DecimalSI),
			},
		},
	}
	_, err := f.K8sClient.CoreV1().ResourceQuotas(f.Namespace.GetName()).Update(context.TODO(), resourceQuota, metav1.UpdateOptions{})
	if err != nil {
		ginkgo.Fail("Unable to set resource quota " + err.Error())
	}
	return wait.PollImmediate(5*time.Second, nsDeleteTime, func() (bool, error) {
		quota, err := f.K8sClient.CoreV1().ResourceQuotas(f.Namespace.GetName()).Get(context.TODO(), "test-quota", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: GET ResourceQuota failed once, retrying: %v\n", err.Error())
			return false, nil
		}
		return len(quota.Status.Used) == 4, nil
	})
}

// CreateStorageQuota creates a quota to limit pvc count and cumulative storage capacity
func (f *Framework) CreateStorageQuota(numPVCs, requestStorage int64) error {
	ns := f.Namespace.GetName()
	resourceQuota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-storage-quota",
			Namespace: ns,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourcePersistentVolumeClaims: *resource.NewQuantity(numPVCs, resource.DecimalSI),
				v1.ResourceRequestsStorage:        *resource.NewQuantity(requestStorage, resource.DecimalSI),
			},
		},
	}
	_, err := f.K8sClient.CoreV1().ResourceQuotas(ns).Create(context.TODO(), resourceQuota, metav1.CreateOptions{})
	if err != nil {
		ginkgo.Fail("Unable to set resource quota " + err.Error())
	}
	return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
		quota, err := f.K8sClient.CoreV1().ResourceQuotas(ns).Get(context.TODO(), "test-storage-quota", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: GET ResourceQuota failed once, retrying: %v\n", err.Error())
			return false, nil
		}
		return len(quota.Status.Used) == 2, nil
	})
}

// UpdateStorageQuota updates an existing storage quota in the current test namespace.
func (f *Framework) UpdateStorageQuota(numPVCs, requestStorage int64) error {
	resourceQuota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-storage-quota",
			Namespace: f.Namespace.GetName(),
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourcePersistentVolumeClaims: *resource.NewQuantity(numPVCs, resource.DecimalSI),
				v1.ResourceRequestsStorage:        *resource.NewQuantity(requestStorage, resource.DecimalSI),
			},
		},
	}
	_, err := f.K8sClient.CoreV1().ResourceQuotas(f.Namespace.GetName()).Update(context.TODO(), resourceQuota, metav1.UpdateOptions{})
	if err != nil {
		ginkgo.Fail("Unable to set resource quota " + err.Error())
	}
	return wait.PollImmediate(5*time.Second, nsDeleteTime, func() (bool, error) {
		quota, err := f.K8sClient.CoreV1().ResourceQuotas(f.Namespace.GetName()).Get(context.TODO(), "test-storage-quota", metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: GET ResourceQuota failed once, retrying: %v\n", err.Error())
			return false, nil
		}
		requestStorageUpdated := resource.NewQuantity(requestStorage, resource.DecimalSI).Cmp(quota.Status.Hard[v1.ResourceRequestsStorage])
		numPVCsUpdated := resource.NewQuantity(numPVCs, resource.DecimalSI).Cmp(quota.Status.Hard[v1.ResourcePersistentVolumeClaims])
		return len(quota.Status.Used) == 2 && requestStorageUpdated+numPVCsUpdated == 0, nil
	})
}

// DeleteStorageQuota an existing storage quota in the current test namespace.
func (f *Framework) DeleteStorageQuota() error {
	return wait.PollImmediate(3*time.Second, time.Minute, func() (bool, error) {
		err := f.K8sClient.CoreV1().ResourceQuotas(f.Namespace.GetName()).Delete(context.TODO(), "test-storage-quota", metav1.DeleteOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: DELETE ResourceQuota failed once, retrying: %v\n", err.Error())
			return false, nil
		}
		return false, nil
	})
}

// CreateWFFCVariationOfStorageClass creates a WFFC variation of a storage class
func (f *Framework) CreateWFFCVariationOfStorageClass(sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	wffc := storagev1.VolumeBindingWaitForFirstConsumer
	setWaitForFirstConsumer := func(sc *storagev1.StorageClass) {
		sc.VolumeBindingMode = &wffc
	}

	return f.CreateNonDefaultVariationOfStorageClass(sc, setWaitForFirstConsumer)
}

// CreateNonDefaultVariationOfStorageClass creates a variation of a storage class following mutationFunc's changes
func (f *Framework) CreateNonDefaultVariationOfStorageClass(sc *storagev1.StorageClass, mutationFunc func(*storagev1.StorageClass)) (*storagev1.StorageClass, error) {
	scCopy := sc.DeepCopy()
	mutationFunc(scCopy)
	if reflect.DeepEqual(sc, scCopy) {
		return sc, nil
	}
	if val, ok := scCopy.Annotations[cc.AnnDefaultStorageClass]; ok && val == "true" {
		scCopy.Annotations[cc.AnnDefaultStorageClass] = "false"
	}
	scCopy.ObjectMeta = metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("%s-temp-variation", scCopy.Name),
		Labels: map[string]string{
			"cdi.kubevirt.io/testing": "",
		},
		Annotations: scCopy.Annotations,
	}

	return f.K8sClient.StorageV1().StorageClasses().Create(context.TODO(), scCopy, metav1.CreateOptions{})
}

// UpdateCdiConfigResourceLimits sets the limits in the CDIConfig object
func (f *Framework) UpdateCdiConfigResourceLimits(resourceCPU, resourceMemory, limitsCPU, limitsMemory int64) error {
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.PodResourceRequirements = &v1.ResourceRequirements{
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    *resource.NewQuantity(resourceCPU, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(resourceMemory, resource.DecimalSI)},
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    *resource.NewQuantity(limitsCPU, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(limitsMemory, resource.DecimalSI)},
		}
	})
	if err != nil {
		return err
	}

	// see if config got updated
	return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
		res, err := f.runKubectlCommand("get", "CDIConfig", "config", "-o=jsonpath={.status.defaultPodResourceRequirements..['cpu', 'memory']}")
		if err != nil {
			return false, err
		}
		values := strings.Fields(res)
		if len(values) != 4 {
			return false, errors.New(fmt.Sprintf("length is not 4: %d", len(values)))
		}
		reqCPU, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return false, err
		}
		reqMem, err := strconv.ParseInt(values[2], 10, 64)
		if err != nil {
			return false, err
		}
		limCPU, err := strconv.ParseInt(values[1], 10, 64)
		if err != nil {
			return false, err
		}
		limMem, err := strconv.ParseInt(values[3], 10, 64)
		if err != nil {
			return false, err
		}
		return resourceCPU == reqCPU && resourceMemory == reqMem && limitsCPU == limCPU && limitsMemory == limMem, nil
	})
}

// ExpectEvent polls and fetches events during a defined period of time
func (f *Framework) ExpectEvent(dataVolumeNamespace string) gomega.AsyncAssertion {
	return gomega.Eventually(func() string {
		events, err := f.runKubectlCommand("get", "events", "-n", dataVolumeNamespace)
		if err == nil {
			fmt.Fprintf(ginkgo.GinkgoWriter, "%s", events)
			return events
		}
		fmt.Fprintf(ginkgo.GinkgoWriter, "ERROR: %s\n", err.Error())
		return ""
	}, timeout, pollingInterval)
}

// ExpectCloneFallback checks PVC annotations and event of clone fallback to host-assisted
func (f *Framework) ExpectCloneFallback(pvc *v1.PersistentVolumeClaim, reason, message string) {
	ginkgo.By("Check PVC annotations and event of clone fallback to host-assisted")
	gomega.Expect(pvc.Annotations[cc.AnnCloneType]).To(gomega.Equal("copy"))
	gomega.Expect(pvc.Annotations[populators.AnnCloneFallbackReason]).To(gomega.Equal(message))
	f.ExpectEvent(pvc.Namespace).Should(gomega.And(
		gomega.ContainSubstring(pvc.Name),
		gomega.ContainSubstring(reason),
		gomega.ContainSubstring(message)))
}

// runKubectlCommand ...
func (f *Framework) runKubectlCommand(args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := f.createKubectlCommand(args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
	}
	return string(stdOutBytes), nil
}

// createKubectlCommand returns the Cmd to execute kubectl
func (f *Framework) createKubectlCommand(args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}

// IsCSIVolumeCloneStorageClassAvailable checks if the storage class capable of CSI Volume Cloning exists.
func (f *Framework) IsCSIVolumeCloneStorageClassAvailable() bool {
	sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.CsiCloneSCName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sc.Name == f.CsiCloneSCName
}

// IsSnapshotStorageClassAvailable checks if the snapshot storage class exists.
func (f *Framework) IsSnapshotStorageClassAvailable() bool {
	// Fetch the storage class
	storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	scs := &snapshotv1.VolumeSnapshotClassList{}
	if err = f.CrClient.List(context.TODO(), scs); err != nil {
		return false
	}

	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Driver == storageclass.Provisioner {
			return true
		}
	}

	return false
}

// GetNoSnapshotStorageClass gets storage class without snapshot support
func (f *Framework) GetNoSnapshotStorageClass() *string {
	scs, err := f.K8sClient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil
	}
	vscs := &snapshotv1.VolumeSnapshotClassList{}
	if err = f.CrClient.List(context.TODO(), vscs); err != nil {
		return nil
	}
	for _, sc := range scs.Items {
		if sc.Name == "local" || strings.Contains(sc.Provisioner, "no-provisioner") {
			continue
		}
		for _, vsc := range vscs.Items {
			if vsc.Driver == sc.Provisioner {
				continue
			}
		}
		return &sc.Name
	}

	return nil
}

// GetSnapshotClass returns the volume snapshot class.
func (f *Framework) GetSnapshotClass() *snapshotv1.VolumeSnapshotClass {
	// Fetch the storage class
	storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.SnapshotSCName, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	scs := &snapshotv1.VolumeSnapshotClassList{}
	if err = f.CrClient.List(context.TODO(), scs); err != nil {
		return nil
	}

	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Driver == storageclass.Provisioner {
			return &snapshotClass
		}
	}

	return nil
}

// IsBlockVolumeStorageClassAvailable checks if the block volume storage class exists.
func (f *Framework) IsBlockVolumeStorageClassAvailable() bool {
	sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), f.BlockSCName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sc.Name == f.BlockSCName
}

// IsBindingModeWaitForFirstConsumer checks if the storage class with specified name has the VolumeBindingMode set to WaitForFirstConsumer
func (f *Framework) IsBindingModeWaitForFirstConsumer(storageClassName *string) bool {
	if storageClassName == nil {
		return false
	}
	storageClass, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), *storageClassName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return storageClass.VolumeBindingMode != nil &&
		*storageClass.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer
}

func (f *Framework) updateCDIConfig() {
	ginkgo.By(fmt.Sprintf("Configuring default FeatureGates %q", f.FeatureGates))
	gomega.Eventually(func() error {
		return utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.FeatureGates = f.FeatureGates
		})
	}, timeout, pollingInterval).Should(gomega.BeNil())
}
