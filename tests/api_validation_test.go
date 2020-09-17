package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
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
	f := framework.NewFramework("api-validation-func-test")

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
				case "imageio":
					url := args[0]
					secretName := args[1]
					configMap := args[2]
					diskID := args[3]
					dv["spec"].(map[string]interface{})["source"] = map[string]interface{}{
						"imageio": map[string]interface{}{
							"url":           url,
							"secretRef":     secretName,
							"certConfigMap": configMap,
							"diskId":        diskID}}
				case "vddk":
					url := args[0]
					secretName := args[1]
					uuid := args[2]
					backingFile := args[3]
					thumbprint := args[4]
					dv["spec"].(map[string]interface{})["source"] = map[string]interface{}{
						"vddk": map[string]interface{}{
							"url":         url,
							"secretRef":   secretName,
							"uuid":        uuid,
							"backingFile": backingFile,
							"thumbprint":  thumbprint}}
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
				table.Entry("[test_id:3917]fail with source PVC doesn't exist", "pvc", "test", "test-pvc"),
				table.Entry("[test_id:3918]fail with empty Imageio source diskId", "imageio", validURL, "secret", "tls-cert", ""),
				table.Entry("[test_id:3926]fail with empty VDDK source UUID", "vddk", validURL, "secret", "", "backingfile", "thumbprint"),
				table.Entry("[test_id:3927]fail with empty VDDK source backing file", "vddk", validURL, "secret", "uuid", "", "thumbprint"),
				table.Entry("[test_id:3928]fail with empty VDDK source thumbprint", "vddk", validURL, "secret", "uuid", "backingfile", ""),
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
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
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

	Context("DataVolume destination imageio", func() {
		BeforeEach(func() {
			dataVolume := utils.NewDataVolumeWithImageioImport(dataVolumeName, "500Mi", validURL, "secret", "tls-cert", "1")

			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})
		It("[test_id:3919]should fail creating a DataVolume with already existing destination imageio", func() {
			By("Verifying kubectl create")
			Eventually(func() bool {

				_, err := RunKubectlCommand(f, "create", "-f", "manifests/dvImageio.yaml", "-n", f.Namespace.Name)
				if err != nil {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("DataVolume destination VDDK", func() {
		BeforeEach(func() {
			dataVolume := utils.NewDataVolumeWithVddkImport(dataVolumeName, "500Mi", "testfile", "secret", "thumbprint", validURL, "uuid")

			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})
		It("[test_id:3925]should fail creating a DataVolume with already existing destination VDDK", func() {
			By("Verifying kubectl create")
			Eventually(func() bool {

				_, err := RunKubectlCommand(f, "create", "-f", "manifests/dvVddk.yaml", "-n", f.Namespace.Name)
				if err != nil {
					return true
				}
				return false
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("when creating data volumes from manual manifests", func() {
		table.DescribeTable("with manifests Datavolume should", func(destinationFile string, expectError bool, errorContains ...string) {
			By("Verifying kubectl apply")
			out, err := RunKubectlCommand(f, "create", "-f", destinationFile, "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			if expectError {
				Expect(err).To(HaveOccurred())
				By("Verifying stderr contains one of the errorContains string(s)")
				containsFound := false
				for _, v := range errorContains {
					if strings.Contains(out, v) {
						containsFound = true
					}
				}
				Expect(containsFound).To(BeTrue())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
			table.Entry("[test_id:1760]fail with blank image source and contentType archive", "manifests/dvBlankArchive.yaml", true, "SourceType cannot be blank and the contentType be archive"),
			table.Entry("[test_id:1761]fail with invalid contentType", "manifests/dvInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive", "Unsupported value: \"invalid\": supported values: \"kubevirt\", \"archive\""),
			table.Entry("[test_id:1762]fail with missing source", "manifests/dvMissingSource.yaml", true, "Missing Data volume source", "spec.source in body must be of type object", "missing required field \"source\" in io.kubevirt.cdi.v1beta1.DataVolume.spec"),
			table.Entry("[test_id:1763]fail with multiple sources", "manifests/dvMultiSource.yaml", true, "Multiple Data volume sources"),
			table.Entry("[test_id:1764]fail with invalid URL for http source", "manifests/dvInvalidURL.yaml", true, "spec.source Invalid source URL"),
			table.Entry("[test_id:1765]fail with invalid source PVC", "manifests/dvInvalidSourcePVC.yaml", true, "spec.source.pvc.name in body is required", "spec.source.pvc.name: Required value", "missing required field \"name\" in io.kubevirt.cdi.v1beta1.DataVolume.spec.source.pvc"),
			table.Entry("[test_id:1766][posneg:positive]succeed with valid source http", "manifests/datavolume.yaml", false, ""),
			table.Entry("[test_id:1767]fail with missing PVC spec", "manifests/dvMissingPVCSpec.yaml", true, "Missing Data volume PVC", "missing required field \"pvc\" in io.kubevirt.cdi.v1beta1.DataVolume.spec", "invalid: spec.pvc: Required value"),
			table.Entry("[test_id:3920]fail with missing PVC accessModes", "manifests/dvMissingPVCAccessModes.yaml", true, "spec.pvc.accessModes in body is required", "spec.pvc.accessModes: Required value", "Required value: at least 1 access mode is required"),
			table.Entry("[test_id:1768]fail with missing resources spec", "manifests/dvMissingResourceSpec.yaml", true, "spec.pvc.resources in body is required", "spec.pvc.resources: Required value", "admission webhook \"datavolume-validate.cdi.kubevirt.io\" denied the request:  PVC size is missing"),
			table.Entry("[test_id:3921]fail with missing PVC size", "manifests/dvMissingPVCSize.yaml", true, "PVC size is missing", "spec.pvc.resources.requests in body must be of type object"),
			table.Entry("[test_id:1769]fail with 0 size PVC", "manifests/dv0SizePVC.yaml", true, "PVC size can't be equal or less than zero"),
			table.Entry("[test_id:1937]fail with invalid content type on blank image", "manifests/dvBlankInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive", "Unsupported value: \"test\": supported values: \"kubevirt\", \"archive\""),
			table.Entry("[test_id:1931][posneg:positive]succeed with leading zero in requests storage size", "manifests/dvLeadingZero.yaml", false, ""),
			table.Entry("[test_id:1925]fail with invalid request storage size", "manifests/dvInvalidStorageSizeQuantity.yaml", true, "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"),
			table.Entry("[test_id:1923]fail with missing storage size", "manifests/dvMissingRequestSpec.yaml", true, "PVC size is missing", "spec.pvc.resources in body must be of type object"),
			table.Entry("[test_id:1915]fail with invalid access modes", "manifests/dvInvalidAccessModes.yaml", true, "supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\""),
			table.Entry("[test_id:3922]fail with multiple access modes", "manifests/dvMultipleAccessModes.yaml", true, "PVC multiple accessModes"),
			table.Entry("[test_id:1861]fail with missing source (but source key)", "manifests/dvMissingSource2.yaml", true, "Missing Data volume source", "missing required field \"source\" in io.kubevirt.cdi.v1beta1.DataVolume.spec", "invalid: spec.source: Required value"),
			table.Entry("[test_id:1860]fail with missing http url key", "manifests/dvMissingSourceHttp.yaml", true, "Missing Data volume source", "spec.source.http in body must be of type object"),
			table.Entry("[test_id:1858]fail with missing datavolume spec", "manifests/dvMissingCompleteSpec.yaml", true, "Missing Data volume source", "missing required field \"spec\" in io.kubevirt.cdi.v1beta1.DataVolume", " invalid: spec: Required value"),
			// k8s < 1.15 return Required value: name or generateName is required, >= 1.15 return error validating data: unknown object type "nil" in DataVolume.metadata
			table.Entry("[test_id:1857]fail without datavolume name", "manifests/dvNoName.yaml", true, "Required value: name or generateName is required", "error validating data: unknown object type \"nil\" in DataVolume.metadata"),
			table.Entry("[test_id:1856]fail without meta data", "manifests/dvNoMetaData.yaml", true, "Required value: name or generateName is required"),
		)

		It("[test_id:4895][posneg:positive]report progress while importing 1024Mi PVC", func() {
			By("Verifying kubectl create")
			out, err := RunKubectlCommand(f, "create", "-f", "manifests/out/dv1024MiPVC.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			err = utils.WaitForDataVolumePhase(f.CdiClient, f.Namespace.Name, cdiv1.ImportInProgress, dataVolumeName)
			if err != nil {
				PrintControllerLog(f)
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
				Expect(dverr).ToNot(HaveOccurred(), "datavolume %s phase %s", dv.Name, dv.Status.Phase)
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile("\\d{1,3}\\.?\\d{1,2}%")
			Eventually(func() bool {
				dv, err := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				progress := dv.Status.Progress
				return progressRegExp.MatchString(string(progress))
			}, timeout, pollingInterval).Should(BeTrue())
		})
	})

	Context("Cannot update datavolume spec", func() {
		var dataVolume *cdiv1.DataVolume

		BeforeEach(func() {
			dataVolume = utils.NewDataVolumeWithHTTPImport(dataVolumeName, "500Mi", validURL)

			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:3923]should fail when updating DataVolume spec", func() {
			updatedDataVolume := dataVolume.DeepCopy()
			updatedDataVolume.Spec.Source.HTTP.URL = "http://foo.bar"

			_, err := f.CdiClient.CdiV1beta1().DataVolumes(updatedDataVolume.Namespace).Update(context.TODO(), updatedDataVolume, metav1.UpdateOptions{})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Can update datavolume meta", func() {
		var dataVolume *cdiv1.DataVolume

		BeforeEach(func() {
			dataVolume = utils.NewDataVolumeWithHTTPImport(dataVolumeName, "500Mi", validURL)

			_, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := utils.DeleteDataVolume(f.CdiClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
		})

		It("[test_id:3924]should fail when updating DataVolume spec", func() {
			updatedDataVolume := dataVolume.DeepCopy()
			if updatedDataVolume.Annotations == nil {
				updatedDataVolume.Annotations = make(map[string]string)
			}
			updatedDataVolume.Annotations["foo"] = "bar"

			_, err := f.CdiClient.CdiV1beta1().DataVolumes(updatedDataVolume.Namespace).Update(context.TODO(), updatedDataVolume, metav1.UpdateOptions{})
			Expect(err).To(HaveOccurred())
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
