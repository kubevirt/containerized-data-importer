package operator

import (
	"fmt"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var crdTypeMap = map[string]interface{}{
	"operator-crd": &cdiv1.CDI{},
}

var _ = Describe("Operator resource test", func() {
	It("Test CRD schemas", func() {
		// github.com/RHsyseng/operator-utils/pkg/validation does not handle metav1.Duration correctly
		// because it serializes to json as string not pointer to int64
		// this is all kind of reduntant with `make generate-verify` which validates the schema
		Skip("validation library needs to be updated")

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
		  "apiVersion":"cdi.kubevirt.io/v1beta1",
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
		Expect(err).To(HaveOccurred())
	})
})

func getSchema() validation.Schema {

	crdFiles, err := yaml.Marshal(createCDIListCRD())
	Expect(err).ToNot(HaveOccurred())
	yamlString := string(crdFiles)
	Expect(err).ToNot(HaveOccurred())
	schema, err := validation.NewVersioned([]byte(yamlString), "v1beta1")
	Expect(err).ToNot(HaveOccurred())

	return schema
}
