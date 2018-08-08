package tests

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var KubectlPath = ""
var OcPath = ""
var CDIInstallNamespace = "kube-system"

var (
	kubeconfig string
	master     string
)

func init() {
	flag.StringVar(&KubectlPath, "kubectl-path", "", "Set path to kubectl binary")
	flag.StringVar(&OcPath, "oc-path", "", "Set path to oc binary")
	flag.StringVar(&CDIInstallNamespace, "installed-namespace", "kube-system", "Set the namespace CDI is installed in")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
}

// CDIFailHandler call ginkgo.Fail with printing the additional information
func CDIFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	Fail(message, callerSkip...)
}

func RunKubectlCommand(args ...string) (string, error) {
	kubeconfig := flag.Lookup("kubeconfig").Value
	if kubeconfig == nil || kubeconfig.String() == "" {
		return "", fmt.Errorf("can not find kubeconfig")
	}

	master := flag.Lookup("master").Value
	if master != nil && master.String() != "" {
		args = append(args, "--server", master.String())
	}

	cmd := exec.Command(KubectlPath, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig.String())
	cmd.Env = append(os.Environ(), kubeconfEnv)

	stdOutBytes, err := cmd.Output()
	if err != nil {
		return string(stdOutBytes), err
	}
	return string(stdOutBytes), nil
}

func SkipIfNoKubectl() {
	if KubectlPath == "" {
		Skip("Skip test that requires kubectl binary")
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromFlags(master, kubeconfig)
}

func GetKubeClientFromFlags(master string, kubeconfig string) (*kubernetes.Clientset, error) {
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

	coreClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return coreClient, nil
}
