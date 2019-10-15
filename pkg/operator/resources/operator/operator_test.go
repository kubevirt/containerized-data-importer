package operator

import (
	"fmt"
	"github.com/RHsyseng/operator-utils/pkg/validation"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"

	"strings"
	"testing"
)

var crdTypeMap = map[string]interface{}{
	OperatorCdiCRD: &cdiv1alpha1.CDI{},
}

func TestCRDSchemas(t *testing.T) {
	for crdFileName, cdiObjType := range crdTypeMap {

		schema := getSchema(t)
		missingEntries := schema.GetMissingEntries(cdiObjType)
		for _, missing := range missingEntries {
			if strings.HasPrefix(missing.Path, "/status") {
				//Not using subresources, so status is not expected to appear in CRD
			} else {
				assert.Fail(t, "Discrepancy between CRD and Struct",
					"Missing or incorrect schema validation at %v, expected type %v  in CRD file %v", missing.Path, missing.Type, crdFileName)
			}
		}
	}
}

func TestSampleCustomResources(t *testing.T) {

	var crFileName = "cdi-cr.yaml"
	root := "./../../../../_out/manifests/release/"
	schema := getSchema(t)
	yamlString, err := ioutil.ReadFile(root + crFileName)
	assert.NoError(t, err, "Error reading %v CR yaml", crFileName)
	var input map[string]interface{}
	assert.NoError(t, yaml.Unmarshal([]byte(yamlString), &input))
	assert.NoError(t, schema.Validate(input), "File %v does not validate against the CRD schema", crFileName)
}

func TestInvalidCustomResources(t *testing.T) {

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
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	schema := getSchema(t)

	var input map[string]interface{}
	assert.NoError(t, yaml.Unmarshal([]byte(crFileName), &input))
	err = schema.Validate(input)
	if err != nil {

		t.Log(err)
	}
	assert.Errorf(t, err, "File %v does not validate against the CRD schema", crFileName)
}

func getSchema(t *testing.T) validation.Schema {

	crdFiles, err := yaml.Marshal(createCDIListCRD())
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	yamlString := string(crdFiles)
	assert.NoError(t, err, "Error reading CRD yaml %v", yamlString)
	schema, err := validation.New([]byte(yamlString))
	assert.NoError(t, err)

	return schema
}
