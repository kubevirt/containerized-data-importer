package tests

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/onsi/ginkgo"

	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/framework"
)

const (
	defaultTimeout      = 270 * time.Second
	testNamespacePrefix = "cdi-test-"
)

var (
	versionRegex           = regexp.MustCompile(`ubernetes .*v(\d+\.\d+\.\d+)`)
	versionRegexServer     = regexp.MustCompile(`Server Version: .*({.*})`)
	versionRegexGitVersion = regexp.MustCompile(`GitVersion:"v(\d+\.\d+\.\d+)\+?\S*"`)
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

//IsOpenshift checks if we are on OpenShift platform
func IsOpenshift(client kubernetes.Interface) bool {
	//OpenShift 3.X check
	result := client.Discovery().RESTClient().Get().AbsPath("/oapi/v1").Do()
	var statusCode int
	result.StatusCode(&statusCode)

	if result.Error() == nil {
		// It is OpenShift
		if statusCode == http.StatusOK {
			return true
		}
	} else {
		// Got 404 so this is not Openshift 3.X, let's check OpenShift 4
		result = client.Discovery().RESTClient().Get().AbsPath("/apis/route.openshift.io").Do()
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
