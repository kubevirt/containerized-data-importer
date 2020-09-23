package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"time"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	"github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	nodeSelectorTestValue  = map[string]string{"kubernetes.io/arch": runtime.GOARCH}
	tolerationsTestValue   = []v1.Toleration{{Key: "test", Value: "123"}}
	affinityTestValue      = &v1.Affinity{}
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
		// err will not always be nil calling kubectl, this is expected on no results for instance.
		// still return the value and let the called decide what to do.
		return string(stdOutBytes), err
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

// TestNodePlacementValues returns a pre-defined set of node placement values for testing purposes.
// The values chosen are valid, but the pod will likely not be schedulable.
func TestNodePlacementValues(f *framework.Framework) sdkapi.NodePlacement {
	nodes, _ := f.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	affinityTestValue = &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{Key: "kubernetes.io/hostname", Operator: v1.NodeSelectorOpIn, Values: []string{nodes.Items[0].Name}},
						},
					},
				},
			},
		},
	}

	return sdkapi.NodePlacement{
		NodeSelector: nodeSelectorTestValue,
		Affinity:     affinityTestValue,
		Tolerations:  tolerationsTestValue,
	}
}

// PodSpecHasTestNodePlacementValues compares if the pod spec has the set of node placement values defined for testing purposes
func PodSpecHasTestNodePlacementValues(f *framework.Framework, podSpec v1.PodSpec) bool {
	if !reflect.DeepEqual(podSpec.NodeSelector, nodeSelectorTestValue) {
		fmt.Printf("mismatched nodeSelectors, podSpec:\n%v\nExpected:\n%v\n", podSpec.NodeSelector, nodeSelectorTestValue)
		return false
	}
	if !reflect.DeepEqual(podSpec.Affinity, affinityTestValue) {
		fmt.Printf("mismatched affinity, podSpec:\n%v\nExpected:\n%v\n", *podSpec.Affinity, affinityTestValue)
		return false
	}
	foundMatchingTolerations := false
	for _, toleration := range podSpec.Tolerations {
		if toleration == tolerationsTestValue[0] {
			foundMatchingTolerations = true
		}
	}
	if foundMatchingTolerations != true {
		fmt.Printf("no matching tolerations found. podSpec:\n%v\nExpected:\n%v\n", podSpec.Tolerations, tolerationsTestValue)
		return false
	}
	return true
}
