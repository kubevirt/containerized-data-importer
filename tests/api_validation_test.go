package tests

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	dataVolumeName     = "test-dv"
	validURL           = "http://www.example.com/example.img"
	invalidURLFormat   = "invalidURL"
	datavolumeTestFile = "tests/manifests/datavolume.yaml"
	destinationFile    = "/var/tmp/datavolume_test.yaml"
)

var _ = Describe("Validation tests", func() {

	f := framework.NewFrameworkOrDie("api-validation-func-test")

	Describe("Verify DataVolume validation", func() {
		Context("when creating Datavolume", func() {
			dv := map[string]interface{}{}

			AfterEach(func() {
				err := os.Remove(destinationFile)
				Expect(err).ToNot(HaveOccurred())
			})

			table.DescribeTable("with Datavolume source validation should", func(sourceType string, args ...string) {

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

				By(fmt.Sprint("Verifying kubectl create"))
				Eventually(func() bool {
					_, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
					if err != nil {
						return true
					}
					return false
				}, timeout, pollingInterval).Should(BeTrue())

			},
				table.Entry("fail with http source with invalid url format", "http", invalidURLFormat),
				table.Entry("fail with http source with empty url", "http", ""),
				table.Entry("fail with s3 source with invalid url format", "s3", invalidURLFormat),
				table.Entry("fail with s3 source with empty url", "s3", ""),
				table.Entry("fail with empty PVC source namespace", "pvc", "", "test-pvc"),
				table.Entry("fail with empty PVC source name", "pvc", "test", ""),
			)

			table.DescribeTable("with Datavolume PVC size should", func(size string) {

				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				dv["spec"].(map[string]interface{})["pvc"].(map[string]interface{})["resources"].(map[string]interface{})["requests"].(map[string]interface{})["storage"] = size
				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprint("Verifying kubectl apply"))
				Eventually(func() bool {
					_, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
					if err != nil {
						return true
					}
					return false
				}, timeout, pollingInterval).Should(BeTrue())
			},
				table.Entry("fail with zero PVC size", "0"),
				table.Entry("fail with negative PVC size", "-500m"),
				table.Entry("fail with invalid PVC size", "invalid_size"),
			)

		})
	})

	Context("when creating Datavolume", func() {
		BeforeEach(func() {
			dataVolume := utils.NewDataVolumeWithHTTPImport(dataVolumeName, "500Mi", validURL)

			dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should fail creating an already existing DataVolume", func() {
			By(fmt.Sprint("Verifying kubectl create"))
			Eventually(func() bool {

				_, err := RunKubectlCommand(f, "create", "-f", datavolumeTestFile, "-n", f.Namespace.Name)
				if err != nil {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

		})
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
