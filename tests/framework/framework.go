package framework

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	csiClientset "kubevirt.io/containerized-data-importer/pkg/snapshot-client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/tests/utils"
	ginkgo_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"
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
	kubectlPath    *string
	ocPath         *string
	cdiInstallNs   *string
	kubeConfig     *string
	master         *string
	goCLIPath      *string
	snapshotSCName *string
	blockSCName    *string
)

// Config provides some basic test config options
type Config struct {
	// SkipNamespaceCreation sets whether to skip creating a namespace. Use this ONLY for tests that do not require
	// a namespace at all, like basic sanity or other global tests.
	SkipNamespaceCreation bool
	// SkipControllerPodLookup sets whether to skip looking up the name of the cdi controller pod.
	SkipControllerPodLookup bool
}

// Framework supports common operations used by functional/e2e tests. It holds the k8s and cdi clients,
// a generated unique namespace, run-time flags, and more fields will be added over time as cdi e2e
// evolves. Global BeforeEach and AfterEach are called in the Framework constructor.
type Framework struct {
	Config
	// NsPrefix is a prefix for generated namespace
	NsPrefix string
	//  k8sClient provides our k8s client pointer
	K8sClient *kubernetes.Clientset
	// CdiClient provides our CDI client pointer
	CdiClient *cdiClientset.Clientset
	// CsiClient provides our CSI client pointer
	CsiClient *csiClientset.Clientset
	// RestConfig provides a pointer to our REST client config.
	RestConfig *rest.Config
	// Namespace provides a namespace for each test generated/unique ns per test
	Namespace *v1.Namespace
	// Namespace2 provides an additional generated/unique secondary ns for testing across namespaces (eg. clone tests)
	Namespace2         *v1.Namespace // note: not instantiated in NewFramework
	namespacesToDelete []*v1.Namespace

	// ControllerPod provides a pointer to our test controller pod
	ControllerPod *v1.Pod

	// KubectlPath is a test run-time flag so we can find kubectl
	KubectlPath string
	// OcPath is a test run-time flag so we can find OpenShift Client
	OcPath string
	// CdiInstallNs is a test run-time flag to store the Namespace we installed CDI in
	CdiInstallNs string
	// KubeConfig is a test run-time flag to store the location of our test setup kubeconfig
	KubeConfig string
	// Master is a test run-time flag to store the id of our master node
	Master string
	// GoCliPath is a test run-time flag to store the location of gocli
	GoCLIPath string
	// SnapshotSCName is the Storage Class name that supports Snapshots
	SnapshotSCName string
	// BlockSCName is the Storage Class name that supports block mode
	BlockSCName string

	reporter *KubernetesReporter

	mux sync.Mutex
}

// TODO: look into k8s' SynchronizedBeforeSuite() and SynchronizedAfterSuite() code and their general
//       purpose test/e2e/framework/cleanup.go function support.

// initialize run-time flags
func init() {
	// By accessing something in the ginkgo_reporters package, we are ensuring that the init() is called
	// That init calls flag.StringVar, and makes sure the --junit-output flag is added before we call
	// flag.Parse in NewFramework. Without this, the flag is NOT added.
	fmt.Fprintf(ginkgo.GinkgoWriter, "Making sure junit flag is available %v\n", ginkgo_reporters.JunitOutput)
	kubectlPath = flag.String("kubectl-path", "kubectl", "The path to the kubectl binary")
	ocPath = flag.String("oc-path", "oc", "The path to the oc binary")
	cdiInstallNs = flag.String("cdi-namespace", "cdi", "The namespace of the CDI controller")
	kubeConfig = flag.String("kubeconfig", "/var/run/kubernetes/admin.kubeconfig", "The absolute path to the kubeconfig file")
	master = flag.String("master", "", "master url:port")
	goCLIPath = flag.String("gocli-path", "cli.sh", "The path to cli script")
	snapshotSCName = flag.String("snapshot-sc", "", "The Storage Class supporting snapshots")
	blockSCName = flag.String("block-sc", "", "The Storage Class supporting block mode volumes")
}

// NewFrameworkOrDie calls NewFramework and handles errors by calling Fail. Config is optional, but
// if passed there can only be one.
func NewFrameworkOrDie(prefix string, config ...Config) *Framework {
	cfg := Config{}
	if len(config) > 0 {
		cfg = config[0]
	}
	f, err := NewFramework(prefix, cfg)
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("failed to create test framework with config %+v: %v", cfg, err))
	}
	return f
}

// NewFramework makes a new framework and sets up the global BeforeEach/AfterEach's.
// Test run-time flags are parsed and added to the Framework struct.
func NewFramework(prefix string, config Config) (*Framework, error) {
	f := &Framework{
		Config:   config,
		NsPrefix: prefix,
	}

	// handle run-time flags
	if !flag.Parsed() {
		flag.Parse()
		klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
		klog.InitFlags(klogFlags)
		flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
			f2 := klogFlags.Lookup(f1.Name)
			if f2 != nil {
				value := f1.Value.String()
				f2.Value.Set(value)
			}
		})

		fmt.Fprintf(ginkgo.GinkgoWriter, "** Test flags:\n")
		flag.Visit(func(f *flag.Flag) {
			fmt.Fprintf(ginkgo.GinkgoWriter, "   %s = %q\n", f.Name, f.Value.String())
		})
		fmt.Fprintf(ginkgo.GinkgoWriter, "**\n")
	}

	f.KubectlPath = *kubectlPath
	f.OcPath = *ocPath
	f.CdiInstallNs = *cdiInstallNs
	f.KubeConfig = *kubeConfig
	f.Master = *master
	f.GoCLIPath = *goCLIPath
	f.SnapshotSCName = *snapshotSCName
	f.BlockSCName = *blockSCName

	restConfig, err := f.LoadConfig()
	if err != nil {
		// Can't use Expect here due this being called outside of an It block, and Expect
		// requires any calls to it to be inside an It block.
		return nil, errors.Wrap(err, "ERROR, unable to load RestConfig")
	}
	f.RestConfig = restConfig
	// clients
	kcs, err := f.GetKubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create K8SClient")
	}
	f.K8sClient = kcs

	cs, err := f.GetCdiClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create CdiClient")
	}
	f.CdiClient = cs

	csics, err := f.GetCsiClient()
	if err != nil {
		return nil, errors.Wrap(err, "ERROR, unable to create CsiClient")
	}
	f.CsiClient = csics

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)
	f.reporter = NewKubernetesReporter()

	f.mux.Lock()
	utils.CacheTestsData(f.K8sClient)
	f.mux.Unlock()

	return f, err
}

// BeforeEach provides a set of operations to run before each test
func (f *Framework) BeforeEach() {
	if !f.SkipControllerPodLookup {
		if f.ControllerPod == nil {
			pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiPodPrefix, common.CDILabelSelector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Located cdi-controller-pod: %q\n", pod.Name)
			f.ControllerPod = pod
		}
	}

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
		nsObj, err = c.CoreV1().Namespaces().Create(ns)
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
	return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
		err := c.CoreV1().Namespaces().Delete(ns, nil)
		if err != nil && !apierrs.IsNotFound(err) {
			return false, nil // keep trying
		}
		// see if ns is really deleted
		_, err = c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil // deleted, done
		}
		if err != nil {
			klog.Warningf("namespace %q Get api error: %v", ns, err)
		}
		return false, nil // keep trying
	})
}

// GetCdiClient gets an instance of a kubernetes client that includes all the CDI extensions.
func (f *Framework) GetCdiClient() (*cdiClientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
	if err != nil {
		return nil, err
	}
	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cdiClient, nil
}

// GetCsiClient gets an instance of a kubernetes client that includes all the CSI extensions.
func (f *Framework) GetCsiClient() (*csiClientset.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
	if err != nil {
		return nil, err
	}
	csiClient, err := csiClientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return csiClient, nil
}

// GetCdiClientForServiceAccount returns a cdi client for a service account
func (f *Framework) GetCdiClientForServiceAccount(namespace, name string) (*cdiClientset.Clientset, error) {
	var secretName string

	sl, err := f.K8sClient.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
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

	secret, err := f.K8sClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	token, ok := secret.Data["token"]
	if !ok {
		return nil, fmt.Errorf("no token key")
	}

	cfg := &rest.Config{
		Host:        f.RestConfig.Host,
		APIPath:     f.RestConfig.APIPath,
		BearerToken: string(token),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return cdiClient, nil
}

// GetKubeClient returns a Kubernetes rest client
func (f *Framework) GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromRESTConfig(f.RestConfig)
}

// LoadConfig loads our specified kubeconfig
func (f *Framework) LoadConfig() (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
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
	return f.K8sClient.CoreV1().Services(namespace).Create(service)
}

// IsSnapshotStorageClassAvailable checks if the snapshot storage class exists.
func (f *Framework) IsSnapshotStorageClassAvailable() bool {
	// Fetch the storage class
	storageclass, err := f.K8sClient.StorageV1().StorageClasses().Get(f.SnapshotSCName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	// List the snapshot classes
	scs, err := f.CsiClient.SnapshotV1alpha1().VolumeSnapshotClasses().List(metav1.ListOptions{})
	if err != nil {
		klog.V(3).Infof("Cannot list snapshot classes")
		return false
	}

	for _, snapshotClass := range scs.Items {
		// Validate association between snapshot class and storage class
		if snapshotClass.Snapshotter == storageclass.Provisioner {
			return true
		}
	}

	return false
}

// IsBlockVolumeStorageClassAvailable checks if the block volume storage class exists.
func (f *Framework) IsBlockVolumeStorageClassAvailable() bool {
	sc, err := f.K8sClient.StorageV1().StorageClasses().Get(f.BlockSCName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sc.Name == f.BlockSCName
}

func getMaxFailsFromEnv() int {
	maxFailsEnv := os.Getenv("REPORTER_MAX_FAILS")
	if maxFailsEnv == "" {
		return 10
	}

	maxFails, err := strconv.Atoi(maxFailsEnv)
	if err != nil { // if the variable is set with a non int value
		fmt.Println("Invalid REPORTER_MAX_FAILS variable, defaulting to 10")
		return 10
	}

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

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(metav1.ListOptions{})
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

func (r *KubernetesReporter) logNodes(kubeCli *kubernetes.Clientset) {

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_nodes.log", r.FailureCount)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	nodes, err := kubeCli.CoreV1().Nodes().List(metav1.ListOptions{})
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

	pvs, err := kubeCli.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
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

	pvcs, err := kubeCli.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(metav1.ListOptions{})
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

	dvs, err := cdiClientset.CdiV1alpha1().DataVolumes(v1.NamespaceAll).List(metav1.ListOptions{})
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

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(metav1.ListOptions{})
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
			logs, err := kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name}).DoRaw()
			if err == nil {
				fmt.Fprintln(current, string(logs))
			}

			logs, err = kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name, Previous: true}).DoRaw()
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

	events, err := kubeCli.CoreV1().Events(v1.NamespaceAll).List(metav1.ListOptions{})
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
