package tests

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cont "kubevirt.io/containerized-data-importer/pkg/controller"
	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	username                 = "foo"
	password                 = "bar"
	proxyHTTPPort            = "8080"
	proxyHTTPPortWithAuth    = "8081"
	proxyTLSHTTPPort         = "443"
	proxyTLSHTTPPortWithAuth = "444"
	proxyServerName          = "cdi-test-proxy"
	fileHostName             = "cdi-file-host"
	tinyCoreQcow2            = "tinyCore.qcow2"
	tinyCoreIso              = "tinyCore.iso"
	tinyCoreIsoGz            = "tinyCore.iso.gz"
	tlsPort                  = ":443"
	tlsAuthPort              = ":444"
	authPort                 = ":81"
	port                     = ":80"
	proxyTestCaConfigMapName = "proxy-test-ca"
	cdiProxyCaConfigMapName  = "cdi-test-proxy-certs"
	nbdKitUserAgent          = "cdi-nbdkit-importer"
	golangUserAgent          = "cdi-golang-importer"
	registryUserAgent        = "Go-http-client/1.1"
)

var _ = Describe("Import Proxy tests", func() {
	var (
		dvName               string
		ocpClient            *configclient.Clientset
		clusterWideProxySpec *ocpconfigv1.ProxySpec
		config               *cdiv1.CDIConfig
		origSpec             *cdiv1.CDIConfigSpec
		cron                 *cdiv1.DataImportCron
		err                  error
	)

	type importProxyTestArguments struct {
		name          string
		size          string
		noProxy       string
		imgName       string
		isHTTPS       bool
		withBasicAuth bool
		expected      types.GomegaMatcher
	}

	f := framework.NewFramework("import-proxy-func-test")

	BeforeEach(func() {
		if utils.IsOpenshift(f.K8sClient) {
			By("Initializing OpenShift client")
			var err error
			ocpClient, err = configclient.NewForConfig(f.RestConfig)
			Expect(err).To(Succeed())

			By("Storing the OpenShift Cluster Wide Proxy original configuration")
			clusterWideProxy, err := ocpClient.ConfigV1().Proxies().Get(context.TODO(), cont.ClusterWideProxyName, metav1.GetOptions{})
			Expect(err).To(Succeed())
			clusterWideProxySpec = clusterWideProxy.Spec.DeepCopy()
		}
		By("Saving original CDIConfig")
		config, err = f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).To(Succeed())
		origSpec = config.Spec.DeepCopy()
	})

	AfterEach(func() {
		if dvName != "" {
			By("Deleting DataVolume")
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dvName)
			Expect(err).To(Succeed())
			Eventually(func() error {
				_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dvName, metav1.GetOptions{})
				return err
			}, timeout, pollingInterval).Should(MatchError(k8serrors.IsNotFound, "PVC not found"))
			dvName = ""
		}

		if cron != nil {
			By("Deleting DataImportCron")
			err = f.CdiClient.CdiV1beta1().DataImportCrons(cron.Namespace).Delete(context.TODO(), cron.Name, metav1.DeleteOptions{})
			Expect(err).To(Succeed())
			cron = nil
		}

		if utils.IsOpenshift(f.K8sClient) {
			By("Reverting the cluster wide proxy spec to the original configuration")
			cleanClusterWideProxy(ocpClient, clusterWideProxySpec)
		}

		By("Restoring CDIConfig to original state")
		err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			origSpec.DeepCopyInto(config)
		})

		Expect(err).To(Succeed())
		Eventually(func() (*cdiv1.CDIConfig, error) {
			return f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		}, 30*time.Second, time.Second).Should(HaveField("Spec", Equal(*origSpec)))
	})

	getPVCNameForConfigMap := func(pvc *corev1.PersistentVolumeClaim) string {
		if pvc.Spec.DataSourceRef != nil {
			return populators.PVCPrimeName(pvc)
		}
		return pvc.Name
	}

	verifyImportProxyConfigMap := func(pvc *corev1.PersistentVolumeClaim) {
		By("Verify import proxy ConfigMap copied to the import namespace")
		trustedCAProxy := cont.GetImportProxyConfigMapName(getPVCNameForConfigMap(pvc))
		Eventually(func() error {
			_, err := f.K8sClient.CoreV1().ConfigMaps(f.Namespace.Name).Get(context.TODO(), trustedCAProxy, metav1.GetOptions{})
			return err
		}, time.Second*60, time.Second).Should(Succeed())
	}

	verifyImportProxyConfigMapIsDeletedOnPodDeletion := func(pvc *corev1.PersistentVolumeClaim) {
		By("Verify import proxy ConfigMap is deleted from import namespace on importer pod deletion")
		pvcName := getPVCNameForConfigMap(pvc)
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), pvcName, metav1.GetOptions{})
		Expect(err).To(Succeed())
		err = utils.DeletePodByName(f.K8sClient, pvc.Annotations[controller.AnnImportPod], f.Namespace.Name, nil)
		Expect(err).To(Succeed())
		trustedCAProxy := cont.GetImportProxyConfigMapName(pvcName)
		Eventually(func() error {
			_, err := f.K8sClient.CoreV1().ConfigMaps(f.Namespace.Name).Get(context.TODO(), trustedCAProxy, metav1.GetOptions{})
			return err
		}, time.Second*60, time.Second).Should(MatchError(k8serrors.IsNotFound, "ConfigMap not found"))
	}

	Context("[Destructive]", Serial, func() {
		DescribeTable("should", func(args importProxyTestArguments) {

			now := time.Now()
			var proxyHTTPURL string
			var proxyHTTPSURL string
			By("Setting proxy to point to test proxy in namespace " + f.CdiInstallNs)
			if args.isHTTPS {
				proxyHTTPSURL = createProxyURL(args.isHTTPS, args.withBasicAuth, f.CdiInstallNs)
			} else {
				proxyHTTPURL = createProxyURL(args.isHTTPS, args.withBasicAuth, f.CdiInstallNs)
			}
			noProxy := args.noProxy
			imgURL := createImgURL(args.isHTTPS, args.withBasicAuth, args.imgName, f.CdiInstallNs)
			dvName = args.name

			updateProxy(f, proxyHTTPURL, proxyHTTPSURL, noProxy, ocpClient)

			By(fmt.Sprintf("Creating new datavolume %s", dvName))
			dv := createHTTPDataVolume(f, dvName, args.size, imgURL, args.isHTTPS, args.withBasicAuth)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).To(Succeed())

			By("Verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dvName)
			Expect(err).To(Succeed())
			f.ForceBindIfWaitForFirstConsumer(pvc)
			verifyImportProxyConfigMap(pvc)
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(cdiv1.Succeeded)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
			Expect(err).To(Succeed())

			By("Checking the importer pod information in the proxy log to verify if the requests were proxied")
			verifyImporterPodInfoInProxyLogs(f, imgURL, now, args.expected)

			verifyImportProxyConfigMapIsDeletedOnPodDeletion(pvc)
		},
			Entry("succeed creating import dv with a proxied server (http)", importProxyTestArguments{
				name:          "dv-import-http-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreQcow2,
				isHTTPS:       false,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso import dv with a proxied server (http)", importProxyTestArguments{
				name:          "dv-import-http-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIso,
				isHTTPS:       false,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso.gz import dv with a proxied server (http)", importProxyTestArguments{
				name:          "dv-import-http-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIsoGz,
				isHTTPS:       false,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating import dv with a proxied server (http) with basic autentication", importProxyTestArguments{
				name:          "dv-import-http-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreQcow2,
				isHTTPS:       false,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso import dv with a proxied server (http) with basic autentication", importProxyTestArguments{
				name:          "dv-import-http-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIso,
				isHTTPS:       false,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso.gz import dv with a proxied server (http) with basic autentication", importProxyTestArguments{
				name:          "dv-import-http-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIsoGz,
				isHTTPS:       false,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating import dv with a proxied server (https) with the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreQcow2,
				isHTTPS:       true,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso import dv with a proxied server (https) with the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIso,
				isHTTPS:       true,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso.gz import dv with a proxied server (https) with the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIsoGz,
				isHTTPS:       true,
				withBasicAuth: false,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating import dv with a proxied server (https) with basic autentication and the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreQcow2,
				isHTTPS:       true,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso import dv with a proxied server (https) with basic autentication and the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIso,
				isHTTPS:       true,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating iso import dv with a proxied server (https) with basic autentication and the target server with tls", importProxyTestArguments{
				name:          "dv-import-https-proxy-auth",
				size:          "1Gi",
				noProxy:       "",
				imgName:       tinyCoreIsoGz,
				isHTTPS:       true,
				withBasicAuth: true,
				expected:      ContainElement(golangUserAgent)}),
			Entry("succeed creating import dv with a proxied server (http) but bypassing the proxy", importProxyTestArguments{
				name:          "dv-import-noproxy",
				size:          "1Gi",
				noProxy:       "*",
				imgName:       tinyCoreQcow2,
				isHTTPS:       false,
				withBasicAuth: false,
				expected:      Not(ContainElement(nbdKitUserAgent))}),
			Entry("succeed creating import dv with a proxied server (http) but bypassing the proxy", importProxyTestArguments{
				name:          "dv-import-noproxy",
				size:          "1Gi",
				noProxy:       "*",
				imgName:       tinyCoreIso,
				isHTTPS:       false,
				withBasicAuth: false,
				expected:      Not(ContainElement(nbdKitUserAgent))}),
		)

		It("should import from https endpoint using global trustedCAProxy without per-DV certConfigMap", func() {
			By("Copying file host CA cert to CDI namespace for use as trustedCAProxy")
			caConfigMapName, err := utils.CopyConfigMap(
				f.K8sClient, f.CdiInstallNs, utils.FileHostCertConfigMap,
				f.CdiInstallNs, "test-trusted-ca", "")
			Expect(err).To(Succeed())

			By("Setting CDIConfig importProxy.trustedCAProxy")
			updateCDIConfigProxy(f, "", "", "", caConfigMapName)

			By("Waiting for CDI to reconcile the trustedCAProxy")
			Eventually(func() (*cdiv1.CDIConfig, error) {
				return f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			}, time.Second*120, time.Second).Should(HaveField("Status.ImportProxy", HaveValue(
				HaveField("TrustedCAProxy", HaveValue(Equal(caConfigMapName))),
			)))

			imgURL := fmt.Sprintf("https://%s.%s:%d/%s",
				fileHostName, f.CdiInstallNs, utils.HTTPSNoAuthPort, tinyCoreIso)
			dvName = "test-global-trusted-ca"

			By("Creating DataVolume without per-DV certConfigMap")
			dv := utils.NewDataVolumeWithHTTPImport(dvName, "400Mi", imgURL)
			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).To(Succeed())
			Expect(dataVolume.Spec.Source.HTTP.CertConfigMap).To(BeEmpty())

			By("Verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dvName)
			Expect(err).To(Succeed())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			By("Verifying proxy ConfigMap was copied to import namespace")
			verifyImportProxyConfigMap(pvc)

			By("Waiting for DataVolume to succeed")
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
			Expect(err).To(Succeed())
		})

		Describe("Global TrustedCA tests", func() {
			AfterEach(func() {
				err := f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Delete(
					context.TODO(), "test-invalid-trusted-ca", metav1.DeleteOptions{})
				Expect(err).To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "ConfigMap not found")))
				err = f.K8sClient.CoreV1().ConfigMaps(f.CdiInstallNs).Delete(
					context.TODO(), "test-global-trusted-ca", metav1.DeleteOptions{})
				Expect(err).To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "ConfigMap not found")))
			})

			It("should succeed after trustedCA is updated from nonexistent to valid ConfigMap", func() {
				// phase 1: set trustedCA to a ConfigMap that doesn't exist
				By("setting CDIConfig.Spec.TrustedCA to a nonexistent ConfigMap")
				nonExistent := "non-existent-cm"
				err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
					config.TrustedCA = &nonExistent
				})
				Expect(err).To(Succeed())

				By("waiting for Status.TrustedCA to remain empty")
				Eventually(func() (*cdiv1.CDIConfig, error) {
					return f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				}, time.Second*120, time.Second).Should(HaveField("Status.TrustedCA", Or(BeNil(), HaveValue(BeEmpty()))))

				By("creating a DataVolume targeting the HTTPS endpoint")
				imgURL := fmt.Sprintf("https://%s.%s:%d/%s",
					fileHostName, f.CdiInstallNs, utils.HTTPSNoAuthPort, tinyCoreIso)
				dvName = "test-global-trusted-ca"
				dv := utils.NewDataVolumeWithHTTPImport(dvName, "400Mi", imgURL)
				dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).To(Succeed())

				By("Verifying pvc was created")
				pvc, err := utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dvName)
				Expect(err).To(Succeed())
				f.ForceBindIfWaitForFirstConsumer(pvc)

				By("verifying the import fails without trustedCA")
				err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.ImportInProgress, dv.Name)
				Expect(err).To(Succeed())

				// phase 2: now create the real CM and update trustedCA
				By("copying the real cert ConfigMap into CDI namespace")
				caConfigMapName, err := utils.CopyConfigMap(f.K8sClient, f.CdiInstallNs, utils.FileHostCertConfigMap,
					f.CdiInstallNs, "test-global-trusted-ca", "ca-bundle.crt")
				Expect(err).To(Succeed())

				By("updating CDIConfig.Spec.TrustedCA to the valid ConfigMap")
				err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
					config.TrustedCA = &caConfigMapName
				})
				Expect(err).To(Succeed())

				By("waiting for Status.TrustedCA to reconcile")
				Eventually(func() (*cdiv1.CDIConfig, error) {
					return f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				}, time.Second*120, time.Second).Should(HaveField("Status.TrustedCA", HaveValue(Equal(caConfigMapName))))

				By("creating a new DataVolume to retry the import using the same image URL")
				dvName = "test-global-trusted-ca-new"
				dv = utils.NewDataVolumeWithHTTPImport(dvName, "400Mi", imgURL)
				dataVolume, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
				Expect(err).To(Succeed())

				By("Verifying pvc was created")
				pvc, err = utils.WaitForPVC(f.K8sClient, dataVolume.Namespace, dvName)
				Expect(err).To(Succeed())
				f.ForceBindIfWaitForFirstConsumer(pvc)

				By("verifying the import succeeds this time")
				err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
				Expect(err).To(Succeed())
			})
		})

		DescribeTable("should proxy registry imports", func(isHTTPS, hasAuth bool) {
			now := time.Now()
			updateProxy(f, "", createProxyURL(isHTTPS, hasAuth, f.CdiInstallNs), "", ocpClient)

			By("Creating new datavolume")
			dv := utils.NewDataVolumeWithRegistryImport("import-dv", "1Gi", fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs))
			dv.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
			cm, err := utils.CopyRegistryCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
			Expect(err).To(Succeed())
			dv.Spec.Source.Registry.CertConfigMap = &cm
			dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
			Expect(err).To(Succeed())
			dvName = dv.Name

			pvc, err := utils.WaitForPVC(f.K8sClient, dv.Namespace, dv.Name)
			Expect(err).To(Succeed())
			f.ForceBindIfWaitForFirstConsumer(pvc)
			verifyImportProxyConfigMap(pvc)
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(cdiv1.Succeeded)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.Succeeded, dv.Name)
			Expect(err).To(Succeed())

			By("Checking the importer pod information in the proxy log to verify if the requests were proxied")
			verifyImporterPodInfoInProxyLogs(f, *dv.Spec.Source.Registry.URL, now, ContainElement(registryUserAgent))

			verifyImportProxyConfigMapIsDeletedOnPodDeletion(pvc)
		},
			Entry("with http proxy, no auth", false, false),
			Entry("with http proxy, auth", false, true),
			Entry("with https proxy, no auth", true, false),
			Entry("with https proxy, auth", true, true),
		)

		DescribeTable("should proxy DataImportCron CronJob poller and registry import", func(isHTTPS, hasAuth bool) {
			const (
				cronName            = "cron-test"
				dataSourceName      = "datasource-test"
				scheduleEveryMinute = "* * * * *"
			)

			now := time.Now()
			ns := f.Namespace.Name
			updateProxy(f, "", createProxyURL(isHTTPS, hasAuth, f.CdiInstallNs), "", ocpClient)

			cm, err := utils.CopyRegistryCertConfigMapDestName(f.K8sClient, ns, f.CdiInstallNs, utils.RegistryCertConfigMap)
			Expect(err).To(Succeed())

			url := fmt.Sprintf(utils.TinyCoreIsoRegistryURL, f.CdiInstallNs)
			reg := cdiv1.DataVolumeSourceRegistry{
				URL:           &url,
				CertConfigMap: &cm,
			}

			By(fmt.Sprintf("Create new DataImportCron %s, url %s", cronName, *reg.URL))
			dic := utils.NewDataImportCron(cronName, "5Gi", scheduleEveryMinute, dataSourceName, 1, reg)
			controller.AddAnnotation(dic, controller.AnnPodRetainAfterCompletion, "true")
			retentionPolicy := cdiv1.DataImportCronRetainNone
			dic.Spec.RetentionPolicy = &retentionPolicy

			cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Create(context.TODO(), dic, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			By("Verify initial job succeeded")
			initialJobName := cont.GetInitialJobName(cron)
			Eventually(func() (*batchv1.Job, error) {
				return f.K8sClient.BatchV1().Jobs(f.CdiInstallNs).Get(context.TODO(), initialJobName, metav1.GetOptions{})
			}, timeout, pollingInterval).Should(HaveField("Status.Succeeded", Equal(int32(1))), "initial job has not succeeded")

			By("Checking the initial job pod information in the proxy log to verify if the requests were proxied")
			verifyPodInfoInProxyLogs(f, f.CdiInstallNs, initialJobName, url, now, ContainElement(registryUserAgent))

			By("Verify cronjob first job succeeded")
			cronJobName := cont.GetCronJobName(cron)
			Eventually(func() (*batchv1.CronJob, error) {
				return f.K8sClient.BatchV1().CronJobs(f.CdiInstallNs).Get(context.TODO(), cronJobName, metav1.GetOptions{})
			}, timeout, pollingInterval).Should(HaveField("Status.LastSuccessfulTime", Not(BeNil())))

			By("Checking the first job pod information in the proxy log to verify if the requests were proxied")
			verifyPodInfoInProxyLogs(f, f.CdiInstallNs, cronJobName, url, now, ContainElement(registryUserAgent))

			var dvName string
			By("Wait for CurrentImports update")
			Eventually(func(g Gomega) {
				var err error
				cron, err = f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
				g.Expect(err).To(Succeed())
				g.Expect(cron.Status.CurrentImports).NotTo(BeEmpty())
				dvName = cron.Status.CurrentImports[0].DataVolumeName
				g.Expect(dvName).NotTo(BeEmpty())
			}, timeout, pollingInterval).Should(Succeed())

			pvc, err := utils.WaitForPVC(f.K8sClient, ns, dvName)
			Expect(err).To(Succeed())
			verifyImportProxyConfigMap(pvc)

			By("Wait for DataImportCron UpToDate")
			Eventually(func() (*cdiv1.DataImportCron, error) {
				return f.CdiClient.CdiV1beta1().DataImportCrons(ns).Get(context.TODO(), cronName, metav1.GetOptions{})
			}, timeout, pollingInterval).Should(HaveField("Status.Conditions", ContainElement(SatisfyAll(
				HaveField("Type", Equal(cdiv1.DataImportCronUpToDate)),
				HaveField("Status", Equal(corev1.ConditionTrue)),
			))), "Timeout waiting for DataImportCron conditions")

			By("Wait for datavolume succeeded")
			err = utils.WaitForDataVolumePhase(f, ns, cdiv1.Succeeded, dvName)
			Expect(err).To(Succeed())

			verifyImportProxyConfigMapIsDeletedOnPodDeletion(pvc)
		},
			Entry("with http proxy, no auth", false, false),
			Entry("with http proxy, auth", false, true),
			Entry("with https proxy, no auth", true, false),
			Entry("with https proxy, auth", true, true),
		)

	})
})

func updateProxy(f *framework.Framework, proxyHTTPURL, proxyHTTPSURL, noProxy string, ocpClient *configclient.Clientset) {
	By("Updating CDIConfig with ImportProxy configuration")
	if !utils.IsOpenshift(f.K8sClient) {
		updateCDIConfigProxy(f, proxyHTTPURL, proxyHTTPSURL, noProxy, cdiProxyCaConfigMapName)
	} else {
		clusterWideProxyCAConfigMapName, err := utils.CopyConfigMap(f.K8sClient, f.CdiInstallNs, cdiProxyCaConfigMapName, "openshift-config", proxyTestCaConfigMapName, "ca-bundle.crt")
		Expect(err).To(Succeed())
		updateCDIConfigByUpdatingTheClusterWideProxy(f, ocpClient, proxyHTTPURL, proxyHTTPSURL, noProxy, clusterWideProxyCAConfigMapName)
	}
}

func createProxyURL(isHTTPS, withBasicAuth bool, namespace string) string {
	var auth string
	protocol := "http"
	port := proxyHTTPPort
	if isHTTPS {
		protocol = "https"
		port = proxyTLSHTTPPort
	}
	if withBasicAuth {
		port = proxyHTTPPortWithAuth
		auth = fmt.Sprintf("%s:%s@", username, password)
		if isHTTPS {
			port = proxyTLSHTTPPortWithAuth
		}
	}
	return fmt.Sprintf("%s://%s%s.%s:%s", protocol, auth, proxyServerName, namespace, port)
}

func createImgURL(withHTTPS, withAuth bool, imgName, namespace string) string {
	protocol := "http"
	imgPort := port
	if withAuth {
		imgPort = authPort
	}
	if withHTTPS {
		protocol = "https"
		imgPort = tlsPort
		if withAuth {
			imgPort = tlsAuthPort
		}
	}
	return fmt.Sprintf("%s://%s.%s%s/%s", protocol, fileHostName, namespace, imgPort, imgName)
}

func createHTTPDataVolume(f *framework.Framework, dataVolumeName, size, url string, isHTTPS, withBasicAuth bool) *cdiv1.DataVolume {
	dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, size, url)
	dataVolume.Annotations[controller.AnnPodRetainAfterCompletion] = "true"
	if isHTTPS {
		cm, err := utils.CopyFileHostCertConfigMap(f.K8sClient, f.Namespace.Name, f.CdiInstallNs)
		Expect(err).To(Succeed())
		dataVolume.Spec.Source.HTTP.CertConfigMap = cm
	}
	if withBasicAuth {
		stringData := map[string]string{
			common.KeyAccess: utils.AccessKeyValue,
			common.KeySecret: utils.SecretKeyValue,
		}
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, stringData, nil, f.Namespace.Name, "mysecret"))
		Expect(err).To(Succeed())
		dataVolume.Spec.Source.HTTP.SecretRef = s.Name
	}
	return dataVolume
}

func updateCDIConfigProxy(f *framework.Framework, proxyHTTPURL, proxyHTTPSURL, noProxy, trustedCa string) {
	err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
		config.ImportProxy = &cdiv1.ImportProxy{
			HTTPProxy:      &proxyHTTPURL,
			HTTPSProxy:     &proxyHTTPSURL,
			NoProxy:        &noProxy,
			TrustedCAProxy: &trustedCa,
		}
	})
	Expect(err).To(Succeed())
}

// updateCDIConfigByUpdatingTheClusterWideProxy changes the OpenShift cluster-wide proxy configuration, but we do not want in this test to have the OpenShift API behind the proxy since it might break OpenShift because of proxy hijacking.
// Then, for testing the importer pod using the proxy configuration from the cluster-wide proxy, we disable the proxy in the cluster-wide proxy obj with noProxy="*", and enable the proxy in the CDIConfig to test the importer pod with proxy configurations.
func updateCDIConfigByUpdatingTheClusterWideProxy(f *framework.Framework, ocpClient *configclient.Clientset, proxyHTTPURL, proxyHTTPSURL, noProxy, trustedCa string) {
	By("Updating OpenShift Cluster Wide Proxy with ImportProxy urls")
	updateClusterWideProxyObj(ocpClient, proxyHTTPURL, proxyHTTPSURL, noProxy, trustedCa)

	By("Waiting OpenShift Cluster Wide Proxy reconcile")
	// the default OpenShift no_proxy configuration only appears in the proxy object after and http(s) url is updated
	Eventually(func() (*ocpconfigv1.Proxy, error) {
		return ocpClient.ConfigV1().Proxies().Get(context.TODO(), cont.ClusterWideProxyName, metav1.GetOptions{})
	}, time.Second*60, time.Second).Should(SatisfyAll(
		HaveField("Status.HTTPProxy", Equal(proxyHTTPURL)),
		HaveField("Status.HTTPSProxy", Equal(proxyHTTPSURL)),
	))

	By("Waiting CDIConfig reconcile")
	proxyValue := func(value string) types.GomegaMatcher {
		if value == "" {
			return Or(BeNil(), HaveValue(BeEmpty()))
		}
		return HaveValue(Equal(value))
	}
	Eventually(func() (*cdiv1.CDIConfig, error) {
		return f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
	}, time.Second*120, time.Second).Should(HaveField("Status.ImportProxy", HaveValue(SatisfyAll(
		HaveField("HTTPProxy", proxyValue(proxyHTTPURL)),
		HaveField("HTTPSProxy", proxyValue(proxyHTTPSURL)),
	))))
}

func updateClusterWideProxyObj(ocpClient *configclient.Clientset, HTTPProxy, HTTPSProxy, NoProxy, trustedCa string) {
	proxy, err := ocpClient.ConfigV1().Proxies().Get(context.TODO(), cont.ClusterWideProxyName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		Skip("This OpenShift cluster version does not have a Cluster Wide Proxy object")
	}
	Expect(err).To(Succeed())
	proxy.Spec.HTTPProxy = HTTPProxy
	proxy.Spec.HTTPSProxy = HTTPSProxy
	proxy.Spec.NoProxy = NoProxy
	proxy.Spec.TrustedCA.Name = trustedCa
	_, err = ocpClient.ConfigV1().Proxies().Update(context.TODO(), proxy, metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

// verifyImporterPodInfoInProxyLogs verifiy if the importer pod request (method, url and impoter pod IP) appears in the proxy log
func verifyImporterPodInfoInProxyLogs(f *framework.Framework, imgURL string, since time.Time, expected types.GomegaMatcher) {
	verifyPodInfoInProxyLogs(f, f.Namespace.Name, common.ImporterPodName, imgURL, since, expected)
}

func verifyPodInfoInProxyLogs(f *framework.Framework, podNamespace, podPrefix, imgURL string, since time.Time, expected types.GomegaMatcher) {
	podIP := getPodIP(f, podNamespace, podPrefix)
	Eventually(func() ([]string, error) {
		return proxyUserAgents(imgURL, podIP, getProxyLog(f, since))
	}, time.Second*60, time.Second).Should(expected)
}

func getPodIP(f *framework.Framework, podNamespace, podPrefix string) string {
	var pod *corev1.Pod
	Eventually(func() (*corev1.Pod, error) {
		var err error
		pod, err = utils.FindPodByPrefix(f.K8sClient, podNamespace, podPrefix, "")
		return pod, err
	}, time.Second*60, time.Second).Should(SatisfyAll(
		HaveField("Status.Phase", Not(Equal(corev1.PodPending))),
		HaveField("Status.PodIP", Not(BeEmpty())),
	))
	fmt.Fprintf(GinkgoWriter, "INFO: Checking POD %s IP: %s\n", pod.Name, pod.Status.PodIP)
	return pod.Status.PodIP
}

func getProxyLog(f *framework.Framework, since time.Time) string {
	proxyPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, proxyServerName, fmt.Sprintf("name=%s", proxyServerName))
	Expect(err).To(Succeed())
	fmt.Fprintf(GinkgoWriter, "INFO: Analyzing the proxy pod %s logs\n", proxyPod.Name)
	log, err := f.K8sClient.CoreV1().Pods(proxyPod.Namespace).GetLogs(proxyPod.Name, &corev1.PodLogOptions{
		SinceTime: &metav1.Time{Time: since},
	}).DoRaw(context.Background())
	Expect(err).To(Succeed())
	return string(log)
}

// proxyUserAgents extracts user agents for requests to the image host from this pod.
func proxyUserAgents(imgURL, podIP, proxyLog string) ([]string, error) {
	lineMatcher := regexp.MustCompile(`METHOD:(GET|CONNECT) URL:(\S+) SRC IP:(\S+) .* USER AGENT:(\S*)`)
	target, err := url.Parse(imgURL)
	if err != nil {
		return nil, err
	}
	var agents []string
	for _, matched := range lineMatcher.FindAllStringSubmatch(proxyLog, -1) {
		requestURL, err := url.Parse(matched[2])
		if err != nil {
			continue
		}
		if requestURL.Hostname() == target.Hostname() && strings.HasPrefix(matched[3], podIP+":") {
			agents = append(agents, matched[4])
		}
	}
	return agents, nil
}

func cleanClusterWideProxy(ocpClient *configclient.Clientset, clusterWideProxySpec *ocpconfigv1.ProxySpec) {
	updateClusterWideProxyObj(ocpClient, clusterWideProxySpec.HTTPProxy, clusterWideProxySpec.HTTPSProxy, clusterWideProxySpec.NoProxy, clusterWideProxySpec.TrustedCA.Name)
	By("Waiting OpenShift Cluster Wide Proxy to be reset to original configuration")
	Eventually(func() (*ocpconfigv1.Proxy, error) {
		return ocpClient.ConfigV1().Proxies().Get(context.TODO(), cont.ClusterWideProxyName, metav1.GetOptions{})
	}, timeout, pollingInterval).Should(SatisfyAll(
		HaveField("Status.HTTPProxy", Equal(clusterWideProxySpec.HTTPProxy)),
		HaveField("Status.HTTPSProxy", Equal(clusterWideProxySpec.HTTPSProxy)),
	))
}

func TestProxyUserAgents(t *testing.T) {
	const target = "https://image.example/disk.img"
	const podIP = "10.0.0.1"
	cases := []struct {
		name     string
		log      string
		expected types.GomegaMatcher
	}{
		{
			name:     "matches a GET request from the pod to the image host",
			log:      "METHOD:GET URL:https://image.example:443/disk.img SRC IP:10.0.0.1:1234 STATUS:200 USER AGENT:cdi-golang-importer",
			expected: ContainElement(golangUserAgent),
		},
		{
			name:     "matches CONNECT entries with a URL",
			log:      "METHOD:CONNECT URL:https://image.example:443 SRC IP:10.0.0.1:1234 STATUS:200 USER AGENT:cdi-nbdkit-importer",
			expected: ContainElement(nbdKitUserAgent),
		},
		{
			name:     "ignores another image host",
			log:      "METHOD:GET URL:https://other.example/disk.img SRC IP:10.0.0.1:1234 STATUS:200 USER AGENT:cdi-golang-importer",
			expected: BeEmpty(),
		},
		{
			name:     "ignores another pod sharing an IP prefix",
			log:      "METHOD:GET URL:https://image.example/disk.img SRC IP:10.0.0.10:1234 STATUS:200 USER AGENT:cdi-golang-importer",
			expected: BeEmpty(),
		},
		{
			name: "ignores unrelated lines and preserves distinct user agents",
			log: "unrelated log line\n" +
				"METHOD:GET URL:https://image.example/disk.img SRC IP:10.0.0.1:1234 STATUS:200 USER AGENT:other-client\n" +
				"METHOD:GET URL:https://image.example/disk.img SRC IP:10.0.0.1:5678 STATUS:200 USER AGENT:cdi-golang-importer\n",
			expected: ConsistOf("other-client", golangUserAgent),
		},
		{
			name:     "does not mistake another user agent for the expected importer",
			log:      "METHOD:GET URL:https://image.example/disk.img SRC IP:10.0.0.1:1234 STATUS:200 USER AGENT:other-client",
			expected: Not(ContainElement(golangUserAgent)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			agents, err := proxyUserAgents(target, podIP, tc.log)
			g.Expect(err).To(Succeed())
			g.Expect(agents).To(tc.expected)
		})
	}
}
