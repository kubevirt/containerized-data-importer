package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
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
	reporter        = NewKubernetesReporter()
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
	Master         string
	GoCLIPath      string
	SnapshotSCName string
	BlockSCName    string

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
	reporter *KubernetesReporter
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
		reporter: reporter,
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

	if utils.IsNfs() {
		ginkgo.By("Creating NFS PVs before the test")
		createNFSPVs(f.K8sClient, f.CdiInstallNs)
	}

	ginkgo.By(fmt.Sprintf("Configuring default FeatureGates %q", f.FeatureGates))
	f.setFeatureGates(f.FeatureGates)
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

		if utils.IsNfs() {
			ginkgo.By("Deleting NFS PVs after the test")
			deleteNFSPVs(f.K8sClient, f.CdiInstallNs)
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

	if ginkgo.CurrentGinkgoTestDescription().Failed {
		f.reporter.FailureCount++
		f.reporter.Dump(f.K8sClient, f.CdiClient, ginkgo.CurrentGinkgoTestDescription().Duration)
	}

	return
}

// CreateNamespace instantiates a new namespace object with a unique name and the passed-in label(s).
func (f *Framework) CreateNamespace(prefix string, labels map[string]string) (*v1.Namespace, error) {
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

// GetCdiClient gets an instance of a kubernetes client that includes all the CDI extensions.
func (c *Clients) GetCdiClient() (*cdiClientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(c.Master, c.KubeConfig)
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
	cfg, err := clientcmd.BuildConfigFromFlags(c.Master, c.KubeConfig)
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

	client, err := crclient.New(c.RestConfig, crclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetRESTConfigForServiceAccount returns a RESTConfig for SA
func (f *Framework) GetRESTConfigForServiceAccount(namespace, name string) (*rest.Config, error) {
	var secretName string

	sl, err := f.K8sClient.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, s := range sl.Items {
		if s.Type == v1.SecretTypeServiceAccountToken {
			n := s.Name
			if len(n) > 12 && n[0:len(n)-12] == name {
				secretName = s.Name
				break
			}
		}
	}

	if len(secretName) == 0 {
		return nil, fmt.Errorf("couldn't find service account secret")
	}

	secret, err := f.K8sClient.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	token, ok := secret.Data["token"]
	if !ok {
		return nil, fmt.Errorf("no token key")
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
	return clientcmd.BuildConfigFromFlags(c.Master, c.KubeConfig)
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
				common.PrometheusLabel: "",
				"kubevirt.io":          "",
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
				common.PrometheusLabel: "",
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
		return len(quota.Status.Hard) == 4, nil
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
	return err
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
			return false, errors.New("length is not 4: " + string(len(values)))
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

//runKubectlCommand ...
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

func (f *Framework) setFeatureGates(defaultFeatureGates []string) {
	gomega.Eventually(func() bool {
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.FeatureGates = defaultFeatureGates
		})
		return err == nil
	}, timeout, pollingInterval).Should(gomega.BeTrue())
}

func getMaxFailsFromEnv() int {
	maxFailsEnv := os.Getenv("REPORTER_MAX_FAILS")
	if maxFailsEnv == "" {
		fmt.Fprintf(os.Stderr, "defaulting to 10 reported failures\n")
		return 10
	}

	maxFails, err := strconv.Atoi(maxFailsEnv)
	if err != nil { // if the variable is set with a non int value
		fmt.Println("Invalid REPORTER_MAX_FAILS variable, defaulting to 10")
		return 10
	}

	fmt.Fprintf(os.Stderr, "Number of reported failures[%d]\n", maxFails)
	return maxFails
}

// KubernetesReporter is the struct that holds the report info.
type KubernetesReporter struct {
	FailureCount int
	artifactsDir string
	maxFails     int
}

// NewKubernetesReporter creates a new instance of the reporter.
func NewKubernetesReporter() *KubernetesReporter {
	return &KubernetesReporter{
		FailureCount: 0,
		artifactsDir: os.Getenv("ARTIFACTS"),
		maxFails:     getMaxFailsFromEnv(),
	}
}

// Dump dumps the current state of the cluster. The relevant logs are collected starting
// from the since parameter.
func (r *KubernetesReporter) Dump(kubeCli *kubernetes.Clientset, cdiClient *cdiClientset.Clientset, since time.Duration) {
	// If we got not directory, print to stderr
	if r.artifactsDir == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Current failure count[%d]\n", r.FailureCount)
	if r.FailureCount > r.maxFails {
		return
	}

	// Can call this as many times as needed, if the directory exists, nothing happens.
	if err := os.MkdirAll(r.artifactsDir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}

	r.logDVs(cdiClient)
	r.logEvents(kubeCli, since)
	r.logNodes(kubeCli)
	r.logPVCs(kubeCli)
	r.logPVs(kubeCli)
	r.logPods(kubeCli)
	r.logServices(kubeCli)
	r.logEndpoints(kubeCli)
	r.logLogs(kubeCli, since)
}

// Cleanup cleans up the current content of the artifactsDir
func (r *KubernetesReporter) Cleanup() {
	// clean up artifacts from previous run
	if r.artifactsDir != "" {
		os.RemoveAll(r.artifactsDir)
	}
}

func (r *KubernetesReporter) logPods(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_pods.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v", err)
		return
	}
	defer f.Close()

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(pods, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logServices(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_services.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v", err)
		return
	}
	defer f.Close()

	services, err := kubeCli.CoreV1().Services(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch services: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(services, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logEndpoints(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_endpoints.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v", err)
		return
	}
	defer f.Close()

	endpoints, err := kubeCli.CoreV1().Endpoints(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch endpointss: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(endpoints, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logNodes(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_nodes.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	nodes, err := kubeCli.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch nodes: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(nodes, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logPVs(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_pvs.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	pvs, err := kubeCli.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvs: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(pvs, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logPVCs(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_pvcs.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	pvcs, err := kubeCli.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvcs: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(pvcs, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logDVs(cdiClientset *cdiClientset.Clientset) {
	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_dvs.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	dvs, err := cdiClientset.CdiV1beta1().DataVolumes(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch datavolumes: %v\n", err)
		return
	}

	j, err := json.MarshalIndent(dvs, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logLogs(kubeCli *kubernetes.Clientset, since time.Duration) {

	logsdir := filepath.Join(r.artifactsDir, "pods")

	if err := os.MkdirAll(logsdir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}

	startTime := time.Now().Add(-since).Add(-5 * time.Second)

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			current, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%d_%s_%s-%s.log", r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer current.Close()

			previous, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%d_%s_%s-%s_previous.log", r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer previous.Close()

			logStart := metav1.NewTime(startTime)
			logs, err := kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(current, string(logs))
			}

			logs, err = kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name, Previous: true}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(previous, string(logs))
			}
		}
	}
}

func (r *KubernetesReporter) logEvents(kubeCli *kubernetes.Clientset, since time.Duration) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_events.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	startTime := time.Now().Add(-since).Add(-5 * time.Second)

	events, err := kubeCli.CoreV1().Events(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}

	e := events.Items
	sort.Slice(e, func(i, j int) bool {
		return e[i].LastTimestamp.After(e[j].LastTimestamp.Time)
	})

	eventsToPrint := v1.EventList{}
	for _, event := range e {
		if event.LastTimestamp.Time.After(startTime) {
			eventsToPrint.Items = append(eventsToPrint.Items, event)
		}
	}

	j, err := json.MarshalIndent(eventsToPrint, "", "    ")
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(j))
}
