package tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	dataVolumeName     = "test-dv"
	pvcName            = "test-pvc"
	validURL           = "http://www.example.com/example.img"
	invalidURLFormat   = "invalidURL"
	datavolumeTestFile = "manifests/datavolume.yaml"
	destinationFile    = "/var/tmp/datavolume_test.yaml"
)

var _ = Describe("[rfe_id:1130][crit:medium][posneg:negative][vendor:cnv-qe@redhat.com][level:component]Validation tests", func() {
	f := framework.NewFrameworkOrDie("api-validation-func-test")

	Describe("Verify DataVolume validation", func() {
		Context("when creating Datavolume", func() {
			dv := map[string]interface{}{}

			AfterEach(func() {
				err := os.Remove(destinationFile)
				Expect(err).ToNot(HaveOccurred())
			})

			table.DescribeTable("with Datavolume source validation should", func(sourceType string, args ...string) {

				By("Reading yaml file from: " + datavolumeTestFile)
				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				switch sourceType {
				case "http":
					url := args[0]
					dv["spec"].(map[string]interface{})["source"] = map[string]interface{}{"http": map[string]interface{}{"url": url}}

				case "s3":
					url := args[0]
					dv["spec"].(map[string]interface{})["source"] = map[string]interface{}{"s3": map[string]interface{}{"url": url}}
				case "pvc":
					namespace := args[0]
					name := args[1]
					dv["spec"].(map[string]interface{})["source"] = map[string]interface{}{
						"pvc": map[string]interface{}{
							"namespace": namespace,
							"name":      name}}
				}

				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					_, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
					if err != nil {
						return true
					}
					return false
				}, timeout, pollingInterval).Should(BeTrue())

			},
				table.Entry("[test_id:1321]fail with http source with invalid url format", "http", invalidURLFormat),
				table.Entry("[test_id:1322]fail with http source with empty url", "http", ""),
				table.Entry("[test_id:1323][crit:low]fail with s3 source with invalid url format", "s3", invalidURLFormat),
				table.Entry("[test_id:1324][crit:low]fail with s3 source with empty url", "s3", ""),
				table.Entry("[test_id:1325]fail with empty PVC source namespace", "pvc", "", "test-pvc"),
				table.Entry("[test_id:1326]fail with empty PVC source name", "pvc", "test", ""),
				table.Entry("fail with source PVC doesn't exist", "pvc", "test", "test-pvc"),
			)

			table.DescribeTable("with Datavolume PVC size should", func(size string) {

				By("Reading yaml file from: " + datavolumeTestFile)
				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				dv["spec"].(map[string]interface{})["pvc"].(map[string]interface{})["resources"].(map[string]interface{})["requests"].(map[string]interface{})["storage"] = size
				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl apply")
				Eventually(func() bool {
					_, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
					if err != nil {
						return true
					}
					return false
				}, timeout, pollingInterval).Should(BeTrue())
			},
				table.Entry("[test_id:1033]fail with zero PVC size", "0"),
				table.Entry("[test_id:1327]fail with negative PVC size", "-500m"),
				table.Entry("[test_id:1328]fail with invalid PVC size", "invalid_size"),
			)

		})
	})

	Context("DataVolume Already Exists", func() {
		BeforeEach(func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "500Mi", validURL)

			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})
		It("[test_id:1030]should fail creating an already existing DataVolume", func() {
			By("Verifying kubectl create")
			Eventually(func() bool {

				_, err := RunKubectlCommand(f, "create", "-f", datavolumeTestFile, "-n", f.Namespace.Name)
				if err != nil {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("DataVolume destination PVC", func() {
		BeforeEach(func() {
			pvc := utils.NewPVCDefinition(dataVolumeName, "50Mi", nil, nil)

			_, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, pvc)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(dataVolumeName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc)
			Expect(err).ToNot(HaveOccurred())
		})
		It("[test_id:1759]should fail creating a DataVolume with already existing destination pvc", func() {
			By("Verifying kubectl create")
			Eventually(func() bool {

				_, err := RunKubectlCommand(f, "create", "-f", datavolumeTestFile, "-n", f.Namespace.Name)
				if err != nil {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("when creating data volumes from manual manifests", func() {
		table.DescribeTable("with manifests Datavolume should", func(destinationFile string, expectError bool, errorContains string) {
			By("Verifying kubectl apply")
			out, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			if expectError {
				Expect(err).To(HaveOccurred())
				By("Verifying stderr contains: " + errorContains)
				Expect(strings.Contains(out, errorContains)).To(BeTrue())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
			table.Entry("[test_id:1760]fail with blank image source and contentType archive", "manifests/dvBlankArchive.yaml", true, "SourceType cannot be blank and the contentType be archive"),
			table.Entry("[test_id:1761]fail with invalid contentType", "manifests/dvInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive"),
			table.Entry("[test_id:1762]fail with missing source", "manifests/dvMissingSource.yaml", true, "Missing Data volume source"),
			table.Entry("[test_id:1763]fail with multiple sources", "manifests/dvMultiSource.yaml", true, "Multiple Data volume sources"),
			table.Entry("[test_id:1764]fail with invalid URL for http source", "manifests/dvInvalidURL.yaml", true, "spec.source Invalid source URL"),
			table.Entry("[test_id:1765]fail with invalid source PVC", "manifests/dvInvalidSourcePVC.yaml", true, "spec.source.pvc.name in body is required"),
			table.Entry("[test_id:1766][posneg:positive]succeed with valid source http", "manifests/datavolume.yaml", false, ""),
			table.Entry("[test_id:1767]fail with missing PVC spec", "manifests/dvMissingPVCSpec.yaml", true, "Missing Data volume PVC"),
			table.Entry("fail with missing PVC accessModes", "manifests/dvMissingPVCAccessModes.yaml", true, "spec.pvc.accessModes in body is required"),
			table.Entry("[test_id:1768]fail with missing resources spec", "manifests/dvMissingResourceSpec.yaml", true, "spec.pvc.resources in body is required"),
			table.Entry("fail with missing PVC size", "manifests/dvMissingPVCSize.yaml", true, "PVC size is missing"),
			table.Entry("[test_id:1769]fail with 0 size PVC", "manifests/dv0SizePVC.yaml", true, "PVC size can't be equal or less than zero"),
			table.Entry("[test_id:1937]fail with invalid content type on blank image", "manifests/dvBlankInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive"),
			table.Entry("[test_id:1931][posneg:positive]succeed with leading zero in requests storage size", "manifests/dvLeadingZero.yaml", false, ""),
			table.Entry("[test_id:1925]fail with invalid request storage size", "manifests/dvInvalidStorageSizeQuantity.yaml", true, "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"),
			table.Entry("[test_id:1923]fail with missing storage size", "manifests/dvMissingRequestSpec.yaml", true, "PVC size is missing"),
			table.Entry("[test_id:1915]fail with invalid access modes", "manifests/dvInvalidAccessModes.yaml", true, "supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\""),
			table.Entry("fail with multiple access modes", "manifests/dvMultipleAccessModes.yaml", true, "PVC multiple accessModes"),
			table.Entry("[test_id:1861]fail with missing source (but source key)", "manifests/dvMissingSource2.yaml", true, "Missing Data volume source"),
			table.Entry("[test_id:1860]fail with missing http url key", "manifests/dvMissingSourceHttp.yaml", true, "Missing Data volume source"),
			table.Entry("[test_id:1858]fail with missing datavolume spec", "manifests/dvMissingCompleteSpec.yaml", true, "Missing Data volume source"),
			table.Entry("[test_id:1857]fail without datavolume name", "manifests/dvNoName.yaml", true, "Required value: name or generateName is required"),
			table.Entry("[test_id:1856]fail without meta data", "manifests/dvNoMetaData.yaml", true, "Required value: name or generateName is required"),
		)

	})

	Describe("Verify auth portions of webhooks", func() {
		const sourcePVCName = "source-pvc"

		Context("Authorization checks", func() {
			var err error
			var targetNamespace *corev1.Namespace
			var targetServiceAccount string

			BeforeEach(func() {
				targetNamespace, err = f.CreateNamespace("cdi-webhook-auth-test", nil)
				Expect(err).ToNot(HaveOccurred())

				targetServiceAccount = createAuthServiceAccount(f.K8sClient, targetNamespace.Name)
			})

			AfterEach(func() {
				if targetNamespace != nil {
					err = f.K8sClient.CoreV1().Namespaces().Delete(targetNamespace.Name, &metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should deny unauthorized user when creating datavolume", func() {
				srcPVCDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
				f.CreateAndPopulateSourcePVC(srcPVCDef, "fill-source", fmt.Sprintf("echo \"hello world\" > %s/data.txt", utils.DefaultPvcMountPath))

				targetDV := utils.NewCloningDataVolume("target-dv", "1G", srcPVCDef)

				config := getRESTConfigForServiceAccount(f.RestConfig, f.K8sClient, targetNamespace.Name, targetServiceAccount)
				client, err := cdiclient.NewForConfig(config)
				Expect(err).ToNot(HaveOccurred())

				// can't list dvs in source
				_, err = client.CdiV1alpha1().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
				Expect(err).To(HaveOccurred())

				// can list dvs in dest
				_, err = client.CdiV1alpha1().DataVolumes(targetNamespace.Name).List(metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				// can't create clone of dv in source
				_, err = client.CdiV1alpha1().DataVolumes(targetNamespace.Name).Create(targetDV)
				Expect(err).To(HaveOccurred())
			})

			It("should deny unauthorized user when creating PVC directly", func() {
				srcPVCDef := utils.NewPVCDefinition(sourcePVCName, "1G", nil, nil)
				f.CreateAndPopulateSourcePVC(srcPVCDef, "fill-source", fmt.Sprintf("echo \"hello world\" > %s/data.txt", utils.DefaultPvcMountPath))
				targetPVCDef := utils.NewPVCDefinition(
					"target-pvc",
					"1G",
					map[string]string{controller.AnnCloneRequest: f.Namespace.Name + "/" + sourcePVCName},
					nil)

				config := getRESTConfigForServiceAccount(f.RestConfig, f.K8sClient, targetNamespace.Name, targetServiceAccount)
				client, err := kubernetes.NewForConfig(config)
				Expect(err).ToNot(HaveOccurred())

				// can't list pvcs in source
				_, err = client.CoreV1().PersistentVolumeClaims(f.Namespace.Name).List(metav1.ListOptions{})
				Expect(err).To(HaveOccurred())

				// can list pvcs in dest
				_, err = client.CoreV1().PersistentVolumeClaims(targetNamespace.Name).List(metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				// can't create clone of pvc in source
				_, err = client.CoreV1().PersistentVolumeClaims(targetNamespace.Name).Create(targetPVCDef)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

const authServiceAccountName = "cdi-auth-test"

func getRESTConfigForServiceAccount(defaultConfig *rest.Config, client kubernetes.Interface, namespace, name string) *rest.Config {
	sl, err := client.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())

	var secretName string
	for _, s := range sl.Items {
		if s.Type == corev1.SecretTypeServiceAccountToken {
			n := s.Name
			if len(n) > 12 && n[0:len(n)-12] == name {
				secretName = s.Name
				break
			}
		}
	}
	Expect(secretName).ShouldNot(BeEmpty())

	secret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())

	token, ok := secret.Data["token"]
	Expect(ok).Should(BeTrue())

	return &rest.Config{
		Host:        defaultConfig.Host,
		APIPath:     defaultConfig.APIPath,
		BearerToken: string(token),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func createAuthServiceAccount(client kubernetes.Interface, namespace string) string {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: authServiceAccountName,
		},
	}

	_, err := client.CoreV1().ServiceAccounts(namespace).Create(sa)
	Expect(err).ToNot(HaveOccurred())

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: authServiceAccountName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"persistentvolumeclaims",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				APIGroups: []string{
					"cdi.kubevirt.io",
				},
				Resources: []string{
					"datavolumes",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	_, err = client.RbacV1().Roles(namespace).Create(role)
	Expect(err).ToNot(HaveOccurred())

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: authServiceAccountName,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     authServiceAccountName,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      authServiceAccountName,
				Namespace: namespace,
			},
		},
	}

	_, err = client.RbacV1().RoleBindings(namespace).Create(rb)
	Expect(err).ToNot(HaveOccurred())

	return authServiceAccountName
}

func yamlFiletoStruct(fileName string, o *map[string]interface{}) error {
	yamlFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, o)
	if err != nil {
		return err
	}
	return nil

}

func structToYamlFile(fileName string, o interface{}) error {
	yamlOutput, err := yaml.Marshal(o)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fileName, yamlOutput, 0644)
	if err != nil {
		return err
	}

	return nil
}
