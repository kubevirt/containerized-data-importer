package tests

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/framework"
)

const (
	defaultTimeout      = 30 * time.Second
	testNamespacePrefix = "cdi-test-"
)

// CDIFailHandler call ginkgo.Fail with printing the additional information
func CDIFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	Fail(message, callerSkip...)
}

func RunKubectlCommand(f *framework.Framework, args ...string) (string, error) {
	kubeconfig := f.KubeConfig
	kPath := f.KubectlPath

	cmd := exec.Command(kPath, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	stdOutBytes, err := cmd.Output()
	if err != nil {
		return string(stdOutBytes), err
	}
	return string(stdOutBytes), nil
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: maybe move this to framework and add it to an AfterEach. Current framework will delete
//       all namespaces that it creates.
func DestroyAllTestNamespaces(client *kubernetes.Clientset) {
	var namespaces *k8sv1.NamespaceList
	var err error
	if wait.PollImmediate(2*time.Second, defaultTimeout, func() (bool, error) {
		namespaces, err = client.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	}) != nil {
		Fail("Unable to list namespaces")
	}

	for _, namespace := range namespaces.Items {
		if strings.HasPrefix(namespace.GetName(), testNamespacePrefix) {
			framework.DeleteNS(client, namespace.Name)
		}
	}
}
