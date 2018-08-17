package framework

import (
	"flag"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const (
	NsCreateTime = 30 * time.Second
	NsDeleteTime = 5 * time.Minute
)

// run-time flags
var (
	kubectlPath  *string
	cdiInstallNs *string
	kubeConfig   *string
	master       *string
)

// Framework supports common operations used by functional/e2e tests. It holds the k8s and cdi clients,
// a generated unique namespace, run-time flags, and more fields will be added over time as cdi e2e
// evolves. Global BeforeEach and AfterEach are called in the Framework constructor.
type Framework struct {
	// prefix for generated namespace
	NsPrefix string
	//  k8s client
	K8sClient *kubernetes.Clientset
	// cdi client
	CdiClient *cdiClientset.Clientset
	// generated/unique ns per test
	Namespace *v1.Namespace
	// generated/unique secondary ns for testing across namespaces (eg. clone tests)
	Namespace2 *v1.Namespace // note: not instantiated in NewFramework
	// list of ns to delete beyond the generated ns
	namespacesToDelete []*v1.Namespace
	// test run-time flags
	KubectlPath  string
	CdiInstallNs string
	KubeConfig   string
	Master       string
}

// TODO: look into k8s' SynchronizedBeforeSuite() and SynchronizedAfterSuite() code and their general
//       purpose test/e2e/framework/cleanup.go function support.

// initialize run-time flags
func init() {
fmt.Printf("\n******************* init ***********\n")
	//flag.StringVar(&kubectlPath, "kubectl-path", "", "Set path to kubectl binary")
	//flag.StringVar(&cdiInstallNs, "installed-namespace", "kube-system", "Set the namespace CDI is installed in")
	//flag.StringVar(&kubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	//flag.StringVar(&master, "master", "", "master url")
	kubectlPath = flag.String("kubectl-path", "", "Set path to kubectl binary")
	cdiInstallNs = flag.String("installed-namespace", "kube-system", "Set the namespace CDI is installed in")
	kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master = flag.String("master", "", "master url")
}

// NewFramework makes a new framework and sets up the global BeforeEach/AfterEach's.
// Test run-time flags are parsed and added to the Framework struct.
func NewFramework(prefix string) *Framework {
	f := &Framework{
		NsPrefix: prefix,
	}

	// handle run-time flags
	if !flag.Parsed() {
		flag.Parse()
fmt.Printf("\n********* flags:\nNFlag=%d, NArg=%d, Args=%+v\n", flag.NFlag(), flag.NArg(), flag.Args())
flag.Visit(func(f *flag.Flag) {
	fmt.Printf("\n**** flag=%q, val=%q\n", f.Name, f.Value.String())
})
fmt.Printf("\n****** lookup: kubectl-path=%q\n", flag.Lookup("kubectl-path").Value)
		if *kubectlPath == "" {
			Fail("flag `kubectl-path` was not specified")
		}
		f.KubectlPath = *kubectlPath
		if *cdiInstallNs == "" {
			Fail("flag `installed-namespace` was not specified")
		}
		f.CdiInstallNs = *cdiInstallNs
		if *kubeConfig == "" {
			Fail("flag `kubeconfig` was not specified")
		}
		f.KubeConfig = *kubeConfig
		if *master == "" {
			Fail("flag `master` was not specified")
		}
		f.Master = *master
fmt.Printf("\n******framework:\nf.KubectlPath=%q, f.CdiInstallNs=%q, f.KubeConfig=%q, f.Master=%q",f.KubectlPath, f.CdiInstallNs, f.KubeConfig, f.Master)
	}

	BeforeEach(f.BeforeEach)
	AfterEach(f.AfterEach)

	return f
}

func (f *Framework) BeforeEach() {
	// generate unique primary ns (ns2 not created here)
	By(fmt.Sprintf("Building a %q namespace api object", f.NsPrefix))
	ns, err := f.CreateNamespace(f.NsPrefix, map[string]string{
		"cdi-e2e": f.NsPrefix,
	})
	Expect(err).NotTo(HaveOccurred())
	f.Namespace = ns
	f.AddNamespaceToDelete(ns)

	// clients
	if f.K8sClient == nil {
		By("Creating a kubernetes client")
		kcs, err := f.GetKubeClient()
		Expect(err).NotTo(HaveOccurred())
		f.K8sClient = kcs
	}
	if f.CdiClient == nil {
		By("Creating a CDI client")
		cs, err := f.GetCdiClient()
		Expect(err).NotTo(HaveOccurred())
		f.CdiClient = cs
	}
}

func (f *Framework) AfterEach() {
	// delete the namespace(s) in a defer in case future code added here could generate
	// an exception. For now there is only a defer.
	defer func() {
		for _, ns := range f.namespacesToDelete {
			if len(ns.Name) == 0 {
				continue
			}
			By(fmt.Sprintf("Destroying namespace %q for this suite.", ns.Name))
			err := DeleteNS(f.K8sClient, ns.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	}()
	return
}

// Instantiate a new namespace object with a unique name and the passed-in label(s).
func (f *Framework) CreateNamespace(prefix string, labels map[string]string) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("cdi-e2e-tests-%s-", prefix),
			Namespace:    "",
			Labels:       labels,
		},
		Status: v1.NamespaceStatus{},
	}

fmt.Printf("\n******* ns spec=%+v\n", ns)
	var nsObj *v1.Namespace
	c := f.K8sClient
	err := wait.PollImmediate(2*time.Second, NsCreateTime, func() (bool, error) {
		var err error
		nsObj, err = c.CoreV1().Namespaces().Create(ns)
		if err != nil {
			glog.Errorf("Unexpected error while creating %q namespace: %v", ns.GenerateName, err)
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
fmt.Printf("\n******* nsObj=%+v\n", nsObj)
	return nsObj, nil
}

func (f *Framework) AddNamespaceToDelete(ns *v1.Namespace) {
	f.namespacesToDelete = append(f.namespacesToDelete, ns)
}

func DeleteNS(c *kubernetes.Clientset, ns string) error {
	return wait.PollImmediate(2*time.Second, NsDeleteTime, func() (bool, error) {
		err := c.CoreV1().Namespaces().Delete(ns, nil)
		if err != nil && !apierrs.IsNotFound(err) {
			glog.Errorf("namespace Delete api err: %v", err)
			return false, nil // keep trying
		}
		// see if ns is really deleted
		_, err = c.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil // deleted, done
		}
		if err != nil {
			glog.Errorf("namespace Get api error: %v", err)
		}
		return false, nil // keep trying
	})
}

// Gets an instance of a kubernetes client that includes all the CDI extensions.
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

func (f *Framework) GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromFlags(f.Master, f.KubeConfig)
}

func GetKubeClientFromFlags(master, kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		return nil, err
	}
	return GetKubeClientFromRESTConfig(config)
}

func GetKubeClientFromRESTConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	return kubernetes.NewForConfig(config)
}
