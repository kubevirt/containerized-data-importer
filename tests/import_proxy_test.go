package tests

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

const (
	username         = "foo"
	password         = "bar"
	httpPort         = "8080"
	httpPortWithAuth = "8081"
	proxyServerName  = "cdi-test-proxy"
)

var _ = Describe("Import Proxy tests", func() {

	f := framework.NewFramework("import-proxy-func-test")
	tinyCoreIsoHTTP := func() string { return fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs) }
	tinyCoreIsoHTTPS := func() string { return fmt.Sprintf(utils.HTTPSTinyCoreIsoURL, f.CdiInstallNs) }
	createHTTPDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
		return dataVolume
	}
	createHTTPDataVolumeWithAuth := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
		stringData := map[string]string{
			common.KeyAccess: username,
			common.KeySecret: password,
		}
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		dataVolume.Spec.Source.HTTP.SecretRef = s.Name
		return dataVolume
	}
	createHTTPSDataVolume := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.HTTP.CertConfigMap = cm
		return dataVolume
	}
	createHTTPSDataVolumeWithAuth := func(dataVolumeName, size, url string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(BeNil())
		dataVolume.Spec.Source.HTTP.CertConfigMap = cm
		stringData := map[string]string{
			common.KeyAccess: username,
			common.KeySecret: password,
		}
		s, _ := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		dataVolume.Spec.Source.HTTP.SecretRef = s.Name
		return dataVolume
	}
	createProxyURL := func(isHTTPS bool, withBasicAuth bool) string {
		var auth string
		port := httpPort
		if withBasicAuth {
			port = httpPortWithAuth
			auth = fmt.Sprintf("%s:%s@", username, password)
		}
		return fmt.Sprintf("http://%s%s.%s:%s", auth, proxyServerName, f.CdiInstallNs, port)
	}

	Describe("importing from cdi proxied web server", func() {
		type importProxyTestArguments struct {
			name               string
			size               string
			url                func() string
			proxyURL           func(bool, bool) string
			isHTTPS            bool
			proxyWithBasicAuth bool
			noProxyDomains     string
			dvFunc             func(string, string, string) *cdiv1.DataVolume
			expected           func() types.GomegaMatcher
		}

		DescribeTable("should", func(args importProxyTestArguments) {
			var proxyHTTPURL string
			var proxyHTTPSURL string
			if args.isHTTPS {
				proxyHTTPSURL = args.proxyURL(args.isHTTPS, args.proxyWithBasicAuth)
			} else {
				proxyHTTPURL = args.proxyURL(args.isHTTPS, args.proxyWithBasicAuth)
			}
			noProxy := args.noProxyDomains

			By("updating CDIConfig with ImportProxy configuration")
			updateCDIConfigImporterProxyConfig(f, proxyHTTPURL, proxyHTTPSURL, noProxy)

			By(fmt.Sprintf("creating new datavolume %s", args.name))
			dv := args.dvFunc(args.name, args.size, args.url())
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).ToNot(HaveOccurred())

			By("verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dataVolume.Name)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("verifying a message was printed to indicate a request was done using proxy")
			verifyImporterPodInfoInProxyLogs(f, dataVolume, args.isHTTPS, args.url(), args.expected)

			By("Waiting for import to be completed")
			err = utils.WaitForDataVolumePhaseWithTimeout(f.CdiClient, f.Namespace.Name, cdiv1.Succeeded, dataVolume.Name, 3*90*time.Second)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting DataVolume")
			err = utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, args.name)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

			cleanClusterWideProxy(f)
		},
			Entry("succeed creating import dv with a proxied server (http)", importProxyTestArguments{
				name:               "dv-http-import",
				size:               "1Gi",
				url:                tinyCoreIsoHTTP,
				proxyURL:           createProxyURL,
				isHTTPS:            false,
				proxyWithBasicAuth: false,
				noProxyDomains:     "",
				dvFunc:             createHTTPDataVolume,
				expected:           BeTrue}),
			Entry("succeed creating import dv with a proxied server (http) with basic autentication", importProxyTestArguments{
				name:               "dv-http-import",
				size:               "1Gi",
				url:                tinyCoreIsoHTTP,
				proxyURL:           createProxyURL,
				isHTTPS:            false,
				proxyWithBasicAuth: true,
				noProxyDomains:     "",
				dvFunc:             createHTTPDataVolumeWithAuth,
				expected:           BeTrue}),
			Entry("succeed creating import dv with a proxied server (https) with the target server with tls", importProxyTestArguments{
				name:               "dv-https-import",
				size:               "1Gi",
				url:                tinyCoreIsoHTTPS,
				proxyURL:           createProxyURL,
				isHTTPS:            true,
				proxyWithBasicAuth: false,
				noProxyDomains:     "",
				dvFunc:             createHTTPSDataVolume,
				expected:           BeTrue}),
			Entry("succeed creating import dv with a proxied server (https) with basic autentication and the target server with tls", importProxyTestArguments{
				name:               "dv-https-import",
				size:               "1Gi",
				url:                tinyCoreIsoHTTPS,
				proxyURL:           createProxyURL,
				isHTTPS:            true,
				proxyWithBasicAuth: true,
				noProxyDomains:     "",
				dvFunc:             createHTTPSDataVolumeWithAuth,
				expected:           BeTrue}),
			Entry("succeed creating import dv with a proxied server (https) but bypassing the proxy", importProxyTestArguments{
				name:               "dv-https-import",
				size:               "1Gi",
				url:                tinyCoreIsoHTTPS,
				proxyURL:           createProxyURL,
				isHTTPS:            true,
				proxyWithBasicAuth: false,
				noProxyDomains:     "*",
				dvFunc:             createHTTPSDataVolumeWithAuth,
				expected:           BeFalse}),
		)
	})
})

func updateCDIConfigImporterProxyConfig(f *framework.Framework, proxyHTTPURL string, proxyHTTPSURL string, noProxy string) {
	if !utils.IsOpenshift(f.K8sClient) {
		updateCDIConfigProxy(f, proxyHTTPURL, proxyHTTPSURL, noProxy)
	} else {
		updateCDIConfigByUpdatingTheClusterWideProxy(f, proxyHTTPURL, proxyHTTPSURL, noProxy)
	}
}

func updateCDIConfigProxy(f *framework.Framework, proxyHTTPURL string, proxyHTTPSURL string, noProxy string) {
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.ImportProxy = &cdiv1.ImportProxy{
			HTTPProxy:  &proxyHTTPURL,
			HTTPSProxy: &proxyHTTPSURL,
			NoProxy:    &noProxy,
		}
	})
	Expect(err).ToNot(HaveOccurred())
}

func updateCDIConfigByUpdatingTheClusterWideProxy(f *framework.Framework, proxyHTTPURL string, proxyHTTPSURL string, noProxy string) {
	By("verifying if OpenShift Cluster Wide Proxy exist")
	ocpClient, err := configclient.NewForConfig(f.RestConfig)
	Expect(err).ToNot(HaveOccurred())
	proxy, err := ocpClient.ConfigV1().Proxies().Get(context.TODO(), controller.ClusterWideProxyName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		Skip("This OpenShift cluster version does not have a Cluster Wide Proxy object")
	}

	By("updating OpenShift Cluster Wide Proxy with ImportProxy urls")
	proxy.Spec.HTTPProxy = proxyHTTPURL
	proxy.Spec.HTTPSProxy = proxyHTTPSURL
	// we do not want in this test to have OpenShift behind a proxy, but to test if the importer pod uses the configured proxy, then, we disable proxy for the OpenShift operations
	proxy.Spec.NoProxy = "*"
	_, err = ocpClient.ConfigV1().Proxies().Update(context.TODO(), proxy, metav1.UpdateOptions{})
	Expect(err).ToNot(HaveOccurred())

	// the default OpenShift no_proxy configuration only appears in the proxy object after and http(s) url is updated
	By("wating OpenShift Cluster Wide Proxy reconcile")
	Eventually(func() bool {
		cwproxy, err := ocpClient.ConfigV1().Proxies().Get(context.TODO(), controller.ClusterWideProxyName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		if cwproxy.Status.HTTPProxy == proxyHTTPURL && cwproxy.Status.HTTPSProxy == proxyHTTPSURL {
			return true
		}
		return false
	}, time.Second*60, time.Second).Should(BeTrue())

	proxy, err = ocpClient.ConfigV1().Proxies().Get(context.TODO(), controller.ClusterWideProxyName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	By("wating CDIConfig reconcile")
	Eventually(func() bool {
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		proxyHTTPURL = controller.GetImportProxyConfig(config, common.ImportProxyHTTP)
		proxyHTTPSURL = controller.GetImportProxyConfig(config, common.ImportProxyHTTPS)
		if proxyHTTPURL == proxy.Status.HTTPProxy && proxyHTTPSURL == proxy.Status.HTTPSProxy {
			// since we disabled the proxy in the cluster wide proxy obj with no_domain="*", we need to reconfigure it in the cdiConfig to test the importe pod using the proxy
			if noProxy != "*" {
				err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
					config.ImportProxy = &cdiv1.ImportProxy{
						HTTPProxy:  &proxyHTTPURL,
						HTTPSProxy: &proxyHTTPSURL,
						NoProxy:    &noProxy,
					}
				})
				Expect(err).ToNot(HaveOccurred())
			}
			return true
		}
		return false
	}, time.Second*30, time.Second).Should(BeTrue())
}

// verifyImporterPodInfoInProxyLogs verify if the importer pod IP is in the proxy log
func verifyImporterPodInfoInProxyLogs(f *framework.Framework, dataVolume *cdiv1.DataVolume, isHTTPS bool, uri string, expected func() types.GomegaMatcher) {
	Eventually(func() bool {
		importerPod, err := utils.FindPodByPrefix(f.K8sClient, dataVolume.Namespace, common.ImporterPodName, common.CDILabelSelector)
		if err == nil {
			if len(importerPod.Status.ContainerStatuses) == 1 && importerPod.Status.ContainerStatuses[0].State.Waiting != nil {
				Expect(importerPod.Status.ContainerStatuses[0].State.Waiting.Reason).To(Equal("ContainerCreating"))
			}
			fmt.Fprintf(GinkgoWriter, "INFO: analyzing importer pod %s\n", importerPod.Name)
			//To verifiy if the importer pod request was proxied, we the proxy's logs the method, the url and the importer pod IP.
			method := "METHOD:GET"
			if isHTTPS {
				//Since the proxy hijacks the HTTPS conections (i.e., tunneling), the first request uses the method "CONNECT" instead of "GET"
				method = "METHOD:CONNECT"
			}
			proxyPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, proxyServerName, fmt.Sprintf("name=%s", proxyServerName))
			Expect(err).ToNot(HaveOccurred())

			log, _ := RunKubectlCommand(f, "logs", proxyPod.Name, "-n", f.CdiInstallNs)
			u, _ := url.Parse(uri)
			for _, line := range strings.Split(strings.TrimSuffix(log, "\n"), "\n") {
				if strings.Contains(line, method) && strings.Contains(line, u.Host) && strings.Contains(line, importerPod.Status.PodIP) {
					fmt.Fprintf(GinkgoWriter, "INFO: Checking proxy POD %s: request was proxied\n", importerPod.Name)
					return true
				}
			}
			return false
		}
		fmt.Fprintf(GinkgoWriter, "INFO: importer pod %s err %v\n", common.ImporterPodName, err)
		return false
	}, time.Second*60, time.Second).Should(expected())
}

func cleanClusterWideProxy(f *framework.Framework) {
	if utils.IsOpenshift(f.K8sClient) {
		By("restoring cluster-wide proxy object to original state")
		ocpClient, err := configclient.NewForConfig(f.RestConfig)
		Expect(err).ToNot(HaveOccurred())
		proxy, err := ocpClient.ConfigV1().Proxies().Get(context.TODO(), controller.ClusterWideProxyName, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			Skip("This OpenShift cluster version does not have a Cluster Wide Proxy object")
		}
		proxy.Spec.HTTPProxy = ""
		proxy.Spec.HTTPSProxy = ""
		proxy.Spec.NoProxy = ""
		_, err = ocpClient.ConfigV1().Proxies().Update(context.TODO(), proxy, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("wating OpenShift Cluster Wide Proxy reconcile after the cleanup")
		Eventually(func() bool {
			proxy, err = ocpClient.ConfigV1().Proxies().Get(context.TODO(), controller.ClusterWideProxyName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if proxy.Status.HTTPProxy == "" && proxy.Status.HTTPSProxy == "" && proxy.Status.NoProxy == "" {
				return true
			}
			return false
		}, time.Second*60, time.Second).Should(BeTrue())
	}
}
