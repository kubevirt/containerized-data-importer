package tests

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
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

//RunKubectlCommand runs a kubectl Cmd and returns output and err
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

//RunGoCLICommand runs a gocli Cmd and returns output and err
func RunGoCLICommand(f *framework.Framework, args ...string) (string, error) {
	var errb bytes.Buffer
	path := f.GoCLIPath
	cmd := exec.Command(path, args...)

	cmd.Stderr = &errb
	stdOutBytes, err := cmd.Output()
	if err != nil {
		if len(errb.String()) > 0 {
			return errb.String(), err
		}
	}
	return string(stdOutBytes), nil
}

//RunOcCommand runs an oc Cmd and returns output and err
func RunOcCommand(f *framework.Framework, args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := CreateOcCommand(f, args...)

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

// CreateOcCommand returns the Cmd to execute oc
func CreateOcCommand(f *framework.Framework, args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.OcPath

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
	if !foundMatchingTolerations {
		fmt.Printf("no matching tolerations found. podSpec:\n%v\nExpected:\n%v\n", podSpec.Tolerations, tolerationsTestValue)
		return false
	}
	return true
}

// WaitForConditions waits until the data volume conditions match the expected conditions
func WaitForConditions(f *framework.Framework, dataVolumeName string, timeout, pollingInterval time.Duration, expectedConditions ...*cdiv1.DataVolumeCondition) {
	gomega.Eventually(func() bool {
		resultDv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		gomega.Expect(dverr).ToNot(gomega.HaveOccurred())
		return VerifyConditions(resultDv.Status.Conditions, expectedConditions)
	}, timeout, pollingInterval).Should(gomega.BeTrue())
}

// VerifyConditions checks if the conditions match testConditions
func VerifyConditions(actualConditions []cdiv1.DataVolumeCondition, testConditions []*cdiv1.DataVolumeCondition) bool {
	for _, condition := range testConditions {
		if condition != nil {
			actualCondition := findConditionByType(condition.Type, actualConditions)
			if actualCondition != nil {
				if actualCondition.Status != condition.Status {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Status does not match for type: %s, status expected: [%s], status found: [%s]\n", condition.Type, condition.Status, actualCondition.Status)
					return false
				}
				if strings.Compare(actualCondition.Reason, condition.Reason) != 0 {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Reason does not match for type: %s, reason expected [%s], reason found: [%s]\n", condition.Type, condition.Reason, actualCondition.Reason)
					return false
				}
				if !strings.Contains(actualCondition.Message, condition.Message) {
					fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Message does not match for type: %s, message expected: [%s],  message found: [%s]\n", condition.Type, condition.Message, actualCondition.Message)
					return false
				}
			}
		}
	}
	return true
}

func findConditionByType(conditionType cdiv1.DataVolumeConditionType, conditions []cdiv1.DataVolumeCondition) *cdiv1.DataVolumeCondition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// IsPrometheusAvailable decides whether or not we will run prometheus alert/metric tests
func IsPrometheusAvailable(client *extclientset.Clientset) bool {
	_, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), "prometheuses.monitoring.coreos.com", metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return false
	}
	return true
}

// MakePrometheusHTTPRequest makes a request to the prometheus api and returns the response
func MakePrometheusHTTPRequest(f *framework.Framework, endpoint string) *http.Response {
	var token, url, monitoringNs string
	var resp *http.Response

	url = getPrometheusURL(f)
	monitoringNs = getMonitoringNs(f)
	gomega.Eventually(func() bool {
		var secretName string
		sa, err := f.K8sClient.CoreV1().ServiceAccounts(monitoringNs).Get(context.TODO(), "prometheus-k8s", metav1.GetOptions{})
		if err != nil {
			return false
		}
		for _, secret := range sa.Secrets {
			if strings.HasPrefix(secret.Name, "prometheus-k8s-token") {
				secretName = secret.Name
			}
		}
		secret, err := f.K8sClient.CoreV1().Secrets(monitoringNs).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		if _, ok := secret.Data["token"]; !ok {
			return false
		}
		token = string(secret.Data["token"])
		return true
	}, 10*time.Second, time.Second).Should(gomega.BeTrue())

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	gomega.Eventually(func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/%s", url, endpoint), nil)
		if err != nil {
			return false
		}
		req.Header.Add("Authorization", "Bearer "+token)
		resp, err = client.Do(req)
		if err != nil {
			return false
		}
		if resp.StatusCode != http.StatusOK {
			return false
		}
		return true
	}, 10*time.Second, 1*time.Second).Should(gomega.BeTrue())

	return resp
}

func getPrometheusURL(f *framework.Framework) string {
	var host, port, url string
	var err error

	if utils.IsOpenshift(f.K8sClient) {
		gomega.Eventually(func() bool {
			host, err = RunOcCommand(f, "-n", "openshift-monitoring", "get", "route", "prometheus-k8s", "--template", "{{.spec.host}}")
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, time.Second).Should(gomega.BeTrue())
		url = fmt.Sprintf("https://%s", host)
	} else {
		gomega.Eventually(func() bool {
			port, err = RunGoCLICommand(f, "ports", "prometheus")
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, time.Second).Should(gomega.BeTrue())
		port = strings.TrimSpace(port)
		gomega.Expect(port).ToNot(gomega.BeEmpty())
		url = fmt.Sprintf("http://localhost:%s", port)
	}

	return url
}

func getMonitoringNs(f *framework.Framework) string {
	if utils.IsOpenshift(f.K8sClient) {
		return "openshift-monitoring"
	}

	return "monitoring"
}
