package tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/framework"
)

const (
	defaultTimeout      = 90 * time.Second
	testNamespacePrefix = "cdi-test-"
)

var (
	versionRegex           = regexp.MustCompile(`ubernetes .*v(\d+\.\d+\.\d+)`)
	versionRegexServer     = regexp.MustCompile(`Server Version: .*({.*})`)
	versionRegexGitVersion = regexp.MustCompile(`GitVersion:"v(\d+\.\d+\.\d+)"`)
)

// CDIFailHandler call ginkgo.Fail with printing the additional information
func CDIFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	ginkgo.Fail(message, callerSkip...)
}

//RunKubectlCommand ...
func RunKubectlCommand(f *framework.Framework, args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := CreateKubectlCommand(f, args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
	}
	return string(stdOutBytes), nil
}

// CreateKubectlCommand returns the Cmd to execute kubectl
func CreateKubectlCommand(f *framework.Framework, args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.KubectlPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}

//PrintControllerLog ...
func PrintControllerLog(f *framework.Framework) {
	PrintPodLog(f, f.ControllerPod.Name, f.CdiInstallNs)
}

//PrintPodLog ...
func PrintPodLog(f *framework.Framework, podName, namespace string) {
	log, err := RunKubectlCommand(f, "logs", podName, "-n", namespace)
	if err == nil {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Pod log\n%s\n", log)
	} else {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Unable to get pod log, %s\n", err.Error())
	}
}

//PanicOnError ...
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: maybe move this to framework and add it to an AfterEach. Current framework will delete
//       all namespaces that it creates.

//DestroyAllTestNamespaces ...
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
		ginkgo.Fail("Unable to list namespaces")
	}

	for _, namespace := range namespaces.Items {
		if strings.HasPrefix(namespace.GetName(), testNamespacePrefix) {
			framework.DeleteNS(client, namespace.Name)
		}
	}
}

// GetKubeVersion returns the version returned by the kubectl version command as a semver compatible string
func GetKubeVersion(f *framework.Framework) string {
	// Check non json version output.
	out, err := RunKubectlCommand(f, "version")
	if err != nil {
		return ""
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
	matches := versionRegex.FindStringSubmatch(out)
	if len(matches) > 1 {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: kubectl version: %s\n", matches[1])
		return matches[1]
	}
	// Didn't match, maybe its the newer version
	matches = versionRegexServer.FindStringSubmatch(out)
	if len(matches) > 1 {
		fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: kubectl version output: %s\n", matches[1])
		// Would love to use json.Unmarshal, but keys aren't quoted
		gitVersion := versionRegexGitVersion.FindStringSubmatch(matches[1])
		if len(gitVersion) > 1 {
			return gitVersion[1]
		}
		return ""
	}
	return ""
}
