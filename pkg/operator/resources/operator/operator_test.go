package operator

import (
	"fmt"
	"io/ioutil"

	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var crdTypeMap = map[string]interface{}{
	"operator-crd": &cdiv1alpha1.CDI{},
}

var _ = Describe("Operator resource test", func() {
	It("Test CRD schemas", func() {
		for crdFileName, cdiObjType := range crdTypeMap {

			schema := getSchema()
			missingEntries := schema.GetMissingEntries(cdiObjType)
			for _, missing := range missingEntries {
				if strings.HasPrefix(missing.Path, "/status") {
					//Not using subresources, so status is not expected to appear in CRD
				} else {
					Fail(fmt.Sprintf("Discrepancy between CRD and Struct"+
						"Missing or incorrect schema validation at %v, expected type %v  in CRD file %v", missing.Path, missing.Type, crdFileName))
				}
			}
		}
	})

	It("Test sample custom resources", func() {
		var crFileName = "cdi-cr.yaml"
		root := "./../../../../_out/manifests/release/"
		schema := getSchema()
		yamlString, err := ioutil.ReadFile(root + crFileName)
		Expect(err).ToNot(HaveOccurred())
		var input map[string]interface{}
		err = yaml.Unmarshal([]byte(yamlString), &input)
		Expect(err).ToNot(HaveOccurred())

		err = schema.Validate(input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Test invalid custom resources", func() {
		crFileName := []byte(` {
		  "apiVersion":"cdi.kubevirt.io/v1alpha1",
		  "kind":"CDI",
		  "metadata": {
		    "name":"cdi",
		    "namespace":"cdi"
		  },
		  "spec": {
		    "imagePullPolicy":"noValue"
		  }
		}`)
		crFileName, err := yaml.JSONToYAML(crFileName)
		Expect(err).ToNot(HaveOccurred())

		schema := getSchema()

		var input map[string]interface{}
		err = yaml.Unmarshal([]byte(crFileName), &input)
		Expect(err).ToNot(HaveOccurred())
		err = schema.Validate(input)
		By(fmt.Sprintf("err: %d, File %v does not validate against the CRD schema", err, crFileName))
	})
})

func getSchema() validation.Schema {

	crdFiles, err := yaml.Marshal(createCDIListCRD())
	Expect(err).ToNot(HaveOccurred())
	yamlString := string(crdFiles)
	Expect(err).ToNot(HaveOccurred())
	schema, err := validation.New([]byte(yamlString))
	Expect(err).ToNot(HaveOccurred())

	return schema
}
