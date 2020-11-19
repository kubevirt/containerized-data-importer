package tests_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/coreos/go-semver/semver"
	route1client "github.com/openshift/client-go/route/clientset/versioned"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	ingressUrl = "www.super-duper-test.ingress.tt.org"
	routeName  = "cdi-uploadproxy"
)

var (
	defaultUrl = ""
)

var _ = Describe("CDI storage class config tests", func() {
	var (
		f                   = framework.NewFramework("cdiconfig-test")
		defaultSc, secondSc *storagev1.StorageClass
	)

	BeforeEach(func() {
		storageClasses, err := f.K8sClient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		for _, sc := range storageClasses.Items {
			if defaultClassValue, ok := sc.Annotations[controller.AnnDefaultStorageClass]; ok {
				if defaultClassValue == "true" {
					defaultSc = &sc
					break
				}
			}
		}
	})

	AfterEach(func() {
		By("Unsetting storage class override if set.")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.ScratchSpaceStorageClass = nil
		})
		Expect(err).ToNot(HaveOccurred())
		if secondSc != nil {
			By("Unmarking default " + secondSc.Name)
			err := SetStorageClassDefault(f, secondSc.Name, false)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() string {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return config.Status.ScratchSpaceStorageClass
			}, time.Second*30, time.Second).Should(Equal(""))
			secondSc = nil
		}
		if defaultSc != nil {
			By("Restoring default to " + defaultSc.Name)
			err := SetStorageClassDefault(f, defaultSc.Name, true)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() string {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return config.Status.ScratchSpaceStorageClass
			}, time.Second*30, time.Second).Should(Equal(defaultSc.Name))
			defaultSc = nil
		}
	})

	It("[test_id:3962]should have the default storage class as its scratchSpaceStorageClass", func() {
		if defaultSc == nil {
			Skip("No default storage class found, skipping test")
		}
		By("Expecting default storage class to be: " + defaultSc.Name)
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(config.Status.ScratchSpaceStorageClass).To(Equal(defaultSc.Name))
	})

	It("[test_id:3964]should set the scratch space to blank if no default exists", func() {
		if defaultSc == nil {
			Skip("No default storage class found, skipping test")
		}
		By("Expecting default storage class to be " + defaultSc.Name)
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(defaultSc.Name).To(Equal(config.Status.ScratchSpaceStorageClass))

		err = SetStorageClassDefault(f, defaultSc.Name, false)
		Expect(err).ToNot(HaveOccurred())
		By("Expecting default storage class to be blank")
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return config.Status.ScratchSpaceStorageClass
		}, time.Second*30, time.Second).Should(Equal(""))
	})

	It("[test_id:3965]should keep the default if you specify an invalid override", func() {
		if defaultSc == nil {
			Skip("No default storage class found, skipping test")
		}
		By("Expecting default storage class to be " + defaultSc.Name)
		invalid := "invalidsc"
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.ScratchSpaceStorageClass = &invalid
		})
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Spec.ScratchSpaceStorageClass == nil {
				return ""
			}
			return *config.Spec.ScratchSpaceStorageClass
		}, time.Second*30, time.Second).Should(Equal(invalid))
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(defaultSc.Name).To(Equal(config.Status.ScratchSpaceStorageClass))
	})

	It("[test_id:3966]Should react to switching the default storage class", func() {
		storageClasses, err := f.K8sClient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		if len(storageClasses.Items) < 2 {
			Skip("Not enough storage classes to switch default")
		}
		By("Expecting default storage class to be " + defaultSc.Name)
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(defaultSc.Name).To(Equal(config.Status.ScratchSpaceStorageClass))
		By("Switching default sc")
		err = SetStorageClassDefault(f, defaultSc.Name, false)
		Expect(err).ToNot(HaveOccurred())
		for _, sc := range storageClasses.Items {
			if sc.Name != defaultSc.Name {
				// Found other class, now set it to default.
				secondSc = &sc
				err = SetStorageClassDefault(f, sc.Name, true)
				Expect(err).ToNot(HaveOccurred())
				break
			}
		}
		By("Expecting default storage class to be " + secondSc.Name)
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return config.Status.ScratchSpaceStorageClass
		}, time.Second*30, time.Second).Should(Equal(secondSc.Name))
	})

	It("[test_id:3967]Should use the override even if a different default is set", func() {
		storageClasses, err := f.K8sClient.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		if len(storageClasses.Items) < 2 {
			Skip("Not enough storage classes to test overrider")
		}
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Make sure default is current value.
		Expect(defaultSc.Name).To(Equal(config.Status.ScratchSpaceStorageClass))
		var override string
		for _, sc := range storageClasses.Items {
			if sc.Name != defaultSc.Name {
				// Found other class, now set it to default.
				override = sc.Name
				break
			}
		}
		err = utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.ScratchSpaceStorageClass = &override
		})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the override " + override + " is now the scratchspace name")
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return config.Status.ScratchSpaceStorageClass
		}, time.Second*30, time.Second).Should(Equal(override))
	})
})

var _ = Describe("CDI ingress config tests, using manifests", func() {
	var (
		f            = framework.NewFramework("cdiconfig-test")
		routeStart   = func() string { return fmt.Sprintf("%s-%s.", routeName, f.CdiInstallNs) }
		manifestFile string
	)

	BeforeEach(func() {
		version := *semver.New(tests.GetKubeVersion(f))
		minVersion := *semver.New("1.14.0")
		if version.LessThan(minVersion) {
			Skip(fmt.Sprintf("kubernetes version %s, doesn't support network ingress", version.String()))
		}
		cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
		Expect(err).ToNot(HaveOccurred())
		By("Checking if a route exists, we set that as default")
		openshiftClient, err := route1client.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())
		_, err = openshiftClient.RouteV1().Routes(f.CdiInstallNs).Get(context.TODO(), "cdi-uploadproxy", metav1.GetOptions{})
		if err == nil {
			By("setting defaultURL to route")
			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if config.Status.UploadProxyURL == nil {
					return false
				}
				return strings.HasPrefix(*config.Status.UploadProxyURL, routeStart())
			}, time.Second*30, time.Second).Should(BeTrue())
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			defaultUrl = *config.Status.UploadProxyURL
		}
		By("Making sure no url is set")
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(defaultUrl))
	})

	AfterEach(func() {
		tests.RunKubectlCommand(f, "delete", "-f", manifestFile, "-n", f.CdiInstallNs)
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(defaultUrl))
	})

	It("[test_id:4949]Should properly react to network ingress", func() {
		manifestFile = "manifests/ingressNetworkApigroup.yaml"
		out, err := tests.RunKubectlCommand(f, "create", "-f", manifestFile, "-n", f.CdiInstallNs)
		fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL != nil {
				return *config.Status.UploadProxyURL
			}
			return ""
		}, time.Second*30, time.Second).Should(Equal("test.manifest.example.com"))
	})

	It("[test_id:4950]Keep current value on no http rule ingress", func() {
		manifestFile = "manifests/ingressNoHttp.yaml"
		controllerPod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiDeploymentPodPrefix, "app=containerized-data-importer")
		Expect(err).ToNot(HaveOccurred())
		currentRestarts := controllerPod.Status.ContainerStatuses[0].RestartCount
		fmt.Fprintf(GinkgoWriter, "INFO: Current number of restarts: %d\n", currentRestarts)
		out, err := tests.RunKubectlCommand(f, "create", "-f", manifestFile, "-n", f.CdiInstallNs)
		fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL != nil {
				return *config.Status.UploadProxyURL
			}
			return ""
		}, time.Second*30, time.Second).Should(Equal(defaultUrl))
		for i := 0; i < 10; i++ {
			// Check for 20 seconds if the deployment pod crashed.
			time.Sleep(2 * time.Second)
			controllerPod, err = utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, cdiDeploymentPodPrefix, "app=containerized-data-importer")
			Expect(err).ToNot(HaveOccurred())
			// Restarts will increase if we crashed due to null pointer.
			Expect(currentRestarts).To(Equal(controllerPod.Status.ContainerStatuses[0].RestartCount))
		}
	})
})

var _ = Describe("CDI ingress config tests", func() {
	var (
		f          = framework.NewFramework("cdiconfig-test")
		routeStart = func() string { return fmt.Sprintf("%s-%s.", routeName, f.CdiInstallNs) }
		ingress    *extensionsv1beta1.Ingress
	)

	BeforeEach(func() {
		cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
		Expect(err).ToNot(HaveOccurred())
		By("Checking if a route exists, we set that as default")
		openshiftClient, err := route1client.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())
		_, err = openshiftClient.RouteV1().Routes(f.CdiInstallNs).Get(context.TODO(), "cdi-uploadproxy", metav1.GetOptions{})
		if err == nil {
			By("setting defaultURL to route")
			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if config.Status.UploadProxyURL == nil {
					return false
				}
				return strings.HasPrefix(*config.Status.UploadProxyURL, routeStart())
			}, time.Second*30, time.Second).Should(BeTrue())
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			defaultUrl = *config.Status.UploadProxyURL
		}
		By("Making sure no url is set")
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(defaultUrl))
	})

	AfterEach(func() {
		By("Unsetting override")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.UploadProxyURLOverride = nil
		})
		Expect(err).ToNot(HaveOccurred())
		if ingress != nil {
			By("Cleaning up ingress")
			err := f.K8sClient.ExtensionsV1beta1().Ingresses(ingress.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() string {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				if config.Status.UploadProxyURL == nil {
					return ""
				}
				return *config.Status.UploadProxyURL
			}, time.Second*30, time.Second).Should(Equal(defaultUrl))
		}
	})

	It("[test_id:3960]Should set uploadProxyURL if override is not defined", func() {
		// TODO, don't hard code "cdi-uploadproxy", read it from container env of cdi-deployment deployment.
		ingress = createIngress("test-ingress", f.CdiInstallNs, "cdi-uploadproxy", ingressUrl)
		_, err := f.K8sClient.ExtensionsV1beta1().Ingresses(f.CdiInstallNs).Create(context.TODO(), ingress, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		By("Expecting uploadproxy url to be " + ingressUrl)
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(ingressUrl))
	})

	It("[test_id:3961]Should keep override uploadProxyURL if override is defined", func() {
		override := "www.override.tt.org"
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.UploadProxyURLOverride = &override
		})
		Expect(err).ToNot(HaveOccurred())
		// TODO, don't hard code "cdi-uploadproxy", read it from container env of cdi-deployment deployment.
		ingress = createIngress("test-ingress", f.CdiInstallNs, "cdi-uploadproxy", ingressUrl)
		_, err = f.K8sClient.ExtensionsV1beta1().Ingresses(f.CdiInstallNs).Create(context.TODO(), ingress, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		By("Expecting uploadproxy url to be " + override)
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(override))
	})
})

var _ = Describe("CDI route config tests", func() {
	var (
		f               = framework.NewFramework("cdiconfig-test")
		routeStart      = func() string { return fmt.Sprintf("%s-%s.", routeName, f.CdiInstallNs) }
		openshiftClient *route1client.Clientset
	)

	BeforeEach(func() {
		cfg, err := clientcmd.BuildConfigFromFlags(f.Master, f.KubeConfig)
		Expect(err).ToNot(HaveOccurred())
		openshiftClient, err = route1client.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())
		_, err = openshiftClient.RouteV1().Routes(f.CdiInstallNs).Get(context.TODO(), "cdi-uploadproxy", metav1.GetOptions{})
		if err != nil {
			Skip("Unable to list routes, skipping")
		}
		By("Making sure no url is set to default route")
		Eventually(func() bool {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return false
			}
			return strings.HasPrefix(*config.Status.UploadProxyURL, routeStart())
		}, time.Second*30, time.Second).Should(BeTrue())
	})

	AfterEach(func() {
		By("Unsetting override")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.UploadProxyURLOverride = nil
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test_id:4951]Should override uploadProxyURL if override is set", func() {
		if openshiftClient == nil {
			Skip("Routes not available")
		}
		override := "www.override.tt.org"
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			config.UploadProxyURLOverride = &override
		})
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() string {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			if config.Status.UploadProxyURL == nil {
				return ""
			}
			return *config.Status.UploadProxyURL
		}, time.Second*30, time.Second).Should(Equal(override))
	})
})

var _ = Describe("CDIConfig instance management", func() {
	f := framework.NewFramework("cdiconfig-test")

	It("[test_id:4952]Should re-create the object if deleted", func() {
		By("Verifying the object exists")
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Save the UID, so we can check it against a new one.
		orgUID := config.GetUID()
		GinkgoWriter.Write([]byte(fmt.Sprintf("Original CDIConfig UID: %s\n", orgUID)))
		By("Deleting the object")
		err = f.CdiClient.CdiV1beta1().CDIConfigs().Delete(context.TODO(), config.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			newConfig, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			GinkgoWriter.Write([]byte(fmt.Sprintf("New CDIConfig UID: %s\n", newConfig.GetUID())))
			return orgUID != newConfig.GetUID()
		}, time.Second*30, time.Second).Should(BeTrue())
	})
})

var _ = Describe("Modifying CDIConfig spec tests", func() {
	var origSpec *cdiv1.CDIConfigSpec
	f := framework.NewFramework("cdiconfig-test")
	BeforeEach(func() {
		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		origSpec = config.Spec.DeepCopy()
	})
	AfterEach(func() {
		By("Restoring CDIConfig to original state")
		err := utils.UpdateCDIConfig(f.CrClient, func(config *cdiv1.CDIConfigSpec) {
			origSpec.DeepCopyInto(config)
		})
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return apiequality.Semantic.DeepEqual(config.Spec, *origSpec)
		}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not properly restored to original value")

		Eventually(func() bool {
			config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return !apiequality.Semantic.DeepEqual(config.Status, cdiv1.CDIConfigStatus{})
		}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig status not restored by config controller")
	})

	DescribeTable("Should disallow invalid global filesystem overhead values", func(overhead string, success bool) {
		err := utils.UpdateCDIConfigWithOptions(f.CrClient, metav1.UpdateOptions{DryRun: []string{"All"}}, func(config *cdiv1.CDIConfigSpec) {
			config.FilesystemOverhead = &cdiv1.FilesystemOverhead{
				Global: cdiv1.Percent(overhead),
			}
		})
		if success {
			Expect(err).ToNot(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		Entry("[test_id:4714] Not a number1", "abc", false),
		Entry("[test_id:4674] Not a number2", "1.abc", false),
		Entry("[test_id:5011] Too big1", "1.0001", false),
		Entry("[test_id:5012] Too big2", "inf", false),
		Entry("[test_id:5013] Negative", "-0.1", false),
		Entry("one", "1", true),
		Entry("zero", "0", true),
		Entry("zero2", "0.0", true),
	)

	DescribeTable("Per-storageClass filesystem overhead value", func(overhead string, success bool) {
		defaultSCName := utils.DefaultStorageClass.GetName()
		err := utils.UpdateCDIConfigWithOptions(f.CrClient, metav1.UpdateOptions{}, func(config *cdiv1.CDIConfigSpec) {
			config.FilesystemOverhead = &cdiv1.FilesystemOverhead{
				Global:       "0.99", // Used to easily test that the update happened
				StorageClass: map[string]cdiv1.Percent{defaultSCName: cdiv1.Percent(overhead)},
			}
		})
		if success {
			Expect(err).ToNot(HaveOccurred())
			By("Waiting for the CDIConfig status to be updated by the controller")
			Eventually(func() bool {
				config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return config.Status.FilesystemOverhead.Global == cdiv1.Percent("0.99")
			}, timeout, pollingInterval).Should(BeTrue(), "CDIConfig not set")
		} else {
			Expect(err).To(HaveOccurred())
		}

		config, err := f.CdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		if success {
			By(fmt.Sprintf("CDI Config spec %v", config.Spec.FilesystemOverhead.StorageClass))
			By(fmt.Sprintf("CDI Config status %v", config.Status.FilesystemOverhead.StorageClass))
			Expect(config.Status.FilesystemOverhead.StorageClass[defaultSCName]).To(Equal(cdiv1.Percent(overhead)))
		} else {
			Expect(config.Status.FilesystemOverhead.StorageClass[defaultSCName]).ToNot(Equal(cdiv1.Percent(overhead)))
		}
	},
		Entry("[test_id:5014]should not update if not a number1", "abc", false),
		Entry("[test_id:5015]should not update if not a number2", "1.abc", false),
		Entry("[test_id:5016]should not update if too big1", "1.01", false),
		Entry("[test_id:5017]should not update if too big2", "inf", false),
		Entry("[test_id:5918]should not update if value is negative", "-0.1", false),
		Entry("should update if value is one", "1", true),
		Entry("should update if value is zero", "0", true),
		Entry("should update if value is zero2", "0.0", true),
	)

})

func createIngress(name, ns, service, hostUrl string) *extensionsv1beta1.Ingress {
	return &extensionsv1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: extensionsv1beta1.IngressSpec{
			Backend: &extensionsv1beta1.IngressBackend{
				ServiceName: service,
				ServicePort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 443,
				},
			},
			Rules: []extensionsv1beta1.IngressRule{
				{Host: hostUrl},
			},
		},
	}
}

func SetStorageClassDefault(f *framework.Framework, scName string, isDefault bool) error {
	sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), scName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	ann := sc.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ann[controller.AnnDefaultStorageClass] = strconv.FormatBool(isDefault)
	sc.SetAnnotations(ann)
	_, err = f.K8sClient.StorageV1().StorageClasses().Update(context.TODO(), sc, metav1.UpdateOptions{})
	Eventually(func() string {
		sc, err := f.K8sClient.StorageV1().StorageClasses().Get(context.TODO(), scName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return sc.GetAnnotations()[controller.AnnDefaultStorageClass]
	}, time.Second*30, time.Second).Should(Equal(strconv.FormatBool(isDefault)))
	return nil
}
