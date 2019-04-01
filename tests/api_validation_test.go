package tests

import (
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
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

			pvc, err := utils.CreatePVCFromDefinition(f.K8sClient, f.Namespace.Name, pvc)
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
		table.DescribeTable("with manifests Datavolume should", func(destinationFile string, expectError bool) {
			By("Verifying kubectl apply")
			_, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
			table.Entry("[test_id:1760]fail with blank image source and contentType archive", "manifests/dvBlankArchive.yaml", true),
			table.Entry("[test_id:1761]fail with invalid contentType", "manifests/dvInvalidContentType.yaml", true),
			table.Entry("[test_id:1762]fail with missing source", "manifests/dvMissingSource.yaml", true),
			table.Entry("[test_id:1763]fail with multiple sources", "manifests/dvMultiSource.yaml", true),
			table.Entry("[test_id:1764]fail with invalid URL for http source", "manifests/dvInvalidURL.yaml", true),
			table.Entry("[test_id:1765]fail with invalid source PVC", "manifests/dvInvalidSourcePVC.yaml", true),
			table.Entry("[test_id:1766][posneg:positive]succeed with valid source http", "manifests/datavolume.yaml", false),
			table.Entry("[test_id:1767]fail with missing PVC spec", "manifests/dvMissingPVCSpec.yaml", true),
			table.Entry("fail with missing PVC accessModes", "manifests/dvMissingPVCAccessModes.yaml", true),
			table.Entry("[test_id:1768]fail with missing resources spec", "manifests/dvMissingResourcesSpec.yaml", true),
			table.Entry("[test_id:1769]fail with 0 size PVC", "manifests/dv0SizePVC.yaml", true),
		)

	})
})

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
