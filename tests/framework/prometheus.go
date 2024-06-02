package framework

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/gomega"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/tests/utils"
)

// runGoCLICommand runs a gocli Cmd and returns output and err
func (f *Framework) runGoCLICommand(args ...string) (string, error) {
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

// createOcCommand returns the Cmd to execute oc
func (f *Framework) createOcCommand(args ...string) *exec.Cmd {
	kubeconfig := f.KubeConfig
	path := f.OcPath

	cmd := exec.Command(path, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	cmd.Env = append(os.Environ(), kubeconfEnv)

	return cmd
}

// runOcCommand runs an oc Cmd and returns output and err
func (f *Framework) runOcCommand(args ...string) (string, error) {
	var errb bytes.Buffer
	cmd := f.createOcCommand(args...)

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

// IsPrometheusAvailable decides whether or not we will run prometheus alert/metric tests
func (f *Framework) IsPrometheusAvailable() bool {
	_, err := f.ExtClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), "prometheuses.monitoring.coreos.com", metav1.GetOptions{})
	return !k8serrors.IsNotFound(err)
}

// MakePrometheusHTTPRequest makes a request to the prometheus api and returns the response
func (f *Framework) MakePrometheusHTTPRequest(endpoint string) *http.Response {
	var token, url, monitoringNs string
	var resp *http.Response

	url = f.getPrometheusURL()
	monitoringNs = f.getMonitoringNs()
	token, err := f.GetTokenForServiceAccount(monitoringNs, "prometheus-k8s")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	gomega.Eventually(func() bool {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/%s", url, endpoint), nil)
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

func (f *Framework) getPrometheusURL() string {
	var host, port, url string
	var err error

	if utils.IsOpenshift(f.K8sClient) {
		gomega.Eventually(func() bool {
			host, err = f.runOcCommand("-n", "openshift-monitoring", "get", "route", "prometheus-k8s", "--template", "{{.spec.host}}")
			return err == nil
		}, 10*time.Second, time.Second).Should(gomega.BeTrue())
		url = fmt.Sprintf("https://%s", host)
	} else {
		gomega.Eventually(func() bool {
			port, err = f.runGoCLICommand("ports", "prometheus")
			return err == nil
		}, 10*time.Second, time.Second).Should(gomega.BeTrue())
		port = strings.TrimSpace(port)
		gomega.Expect(port).ToNot(gomega.BeEmpty())
		url = fmt.Sprintf("http://127.0.0.1:%s", port)
	}

	return url
}

func (f *Framework) getMonitoringNs() string {
	if utils.IsOpenshift(f.K8sClient) {
		return "openshift-monitoring"
	}

	return "monitoring"
}
