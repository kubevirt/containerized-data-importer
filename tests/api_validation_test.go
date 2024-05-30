package tests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ghodss/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	// DataVolume validation
	dataVolumeName     = "test-dv"
	pvcName            = "test-pvc"
	validURL           = "http://www.example.com/example.img"
	invalidURLFormat   = "invalidURL"
	datavolumeTestFile = "manifests/datavolume.yaml"
	destinationFile    = "/var/tmp/datavolume_test.yaml"
	// Populator validation
	populatorDestinationFile = "/var/tmp/populator_test.yaml"
	importPopulatorTestFile  = "manifests/volumeimportsource.yaml"
	uploadPopulatorTestFile  = "manifests/volumeuploadsource.yaml"
)

var _ = Describe("[rfe_id:1130][crit:medium][posneg:negative][vendor:cnv-qe@redhat.com][level:component]Validation tests", Serial, func() {
	f := framework.NewFramework("api-validation-func-test")

	setSourceType := func(object map[string]interface{}, sourceType string, args []string) {
		switch sourceType {
		case "http":
			url := args[0]
			object["spec"].(map[string]interface{})["source"] = map[string]interface{}{"http": map[string]interface{}{"url": url}}
		case "s3":
			url := args[0]
			object["spec"].(map[string]interface{})["source"] = map[string]interface{}{"s3": map[string]interface{}{"url": url}}
		case "pvc":
			namespace := args[0]
			name := args[1]
			object["spec"].(map[string]interface{})["source"] = map[string]interface{}{
				"pvc": map[string]interface{}{
					"namespace": namespace,
					"name":      name}}
		case "imageio":
			url := args[0]
			secretName := args[1]
			configMap := args[2]
			diskID := args[3]
			object["spec"].(map[string]interface{})["source"] = map[string]interface{}{
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
			object["spec"].(map[string]interface{})["source"] = map[string]interface{}{
				"vddk": map[string]interface{}{
					"url":         url,
					"secretRef":   secretName,
					"uuid":        uuid,
					"backingFile": backingFile,
					"thumbprint":  thumbprint}}
		}
	}

	Describe("Verify DataVolume validation", func() {
		Context("when creating Datavolume", func() {
			dv := map[string]interface{}{}

			AfterEach(func() {
				err := os.Remove(destinationFile)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("with Datavolume source validation should", func(sourceType string, args ...string) {

				By("Reading yaml file from: " + datavolumeTestFile)
				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				setSourceType(dv, sourceType, args)
				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					_, err := f.RunKubectlCommand("create", "-f", destinationFile, "-n", f.Namespace.Name)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())

			},
				Entry("[test_id:1321]fail with http source with invalid url format", "http", invalidURLFormat),
				Entry("[test_id:1322]fail with http source with empty url", "http", ""),
				Entry("[test_id:1323][crit:low]fail with s3 source with invalid url format", "s3", invalidURLFormat),
				Entry("[test_id:1324][crit:low]fail with s3 source with empty url", "s3", ""),
				Entry("[test_id:1325]fail with empty PVC source namespace", "pvc", "", "test-pvc"),
				Entry("[test_id:1326]fail with empty PVC source name", "pvc", "test", ""),
				Entry("[test_id:3917]fail with source PVC doesn't exist", "pvc", "test", "test-pvc"),
				Entry("[test_id:3918]fail with empty Imageio source diskId", "imageio", validURL, "secret", "tls-cert", ""),
				Entry("[test_id:3926]fail with empty VDDK source UUID", "vddk", validURL, "secret", "", "backingfile", "thumbprint"),
				Entry("[test_id:3927]fail with empty VDDK source backing file", "vddk", validURL, "secret", "uuid", "", "thumbprint"),
				Entry("[test_id:3928]fail with empty VDDK source thumbprint", "vddk", validURL, "secret", "uuid", "backingfile", ""),
			)

			DescribeTable("with DataVolume sourceRef validation should", func(kind, namespace, name string) {
				By("Reading yaml file from: " + datavolumeTestFile)
				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				delete(dv["spec"].(map[string]interface{}), "source")
				dv["spec"].(map[string]interface{})["sourceRef"] = map[string]interface{}{
					"kind":      kind,
					"namespace": &namespace,
					"name":      name}

				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					out, err := f.RunKubectlCommand("create", "-f", destinationFile, "-n", f.Namespace.Name)
					By("out: " + out)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())

			},
				Entry("[test_id:6780]fail with empty kind", "", "test", "test-pvc"),
				Entry("[test_id:6781]fail with unsupported kind", "no-such-kind", "test", "test-pvc"),
				Entry("[test_id:6782]fail with empty sourceRef name", "DataSource", "test", ""),
				Entry("[test_id:6783]fail with sourceRef DataSource doesn't exist", "DataSource", "test", "test-pvc"),
			)

			DescribeTable("with Datavolume PVC size should", func(size string) {

				By("Reading yaml file from: " + datavolumeTestFile)
				err := yamlFiletoStruct(datavolumeTestFile, &dv)
				Expect(err).ToNot(HaveOccurred())

				dv["spec"].(map[string]interface{})["pvc"].(map[string]interface{})["resources"].(map[string]interface{})["requests"].(map[string]interface{})["storage"] = size
				err = structToYamlFile(destinationFile, dv)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl apply")
				Eventually(func() bool {
					_, err := f.RunKubectlCommand("create", "-f", destinationFile, "-n", f.Namespace.Name)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())
			},
				Entry("[test_id:1033]fail with zero PVC size", "0"),
				Entry("[test_id:1327]fail with negative PVC size", "-500m"),
				Entry("[test_id:1328]fail with invalid PVC size", "invalid_size"),
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

				_, err := f.RunKubectlCommand("create", "-f", datavolumeTestFile, "-n", f.Namespace.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("DataVolume destination PVC", func() {
		BeforeEach(func() {
			_, err := f.CreatePVCFromDefinition(utils.NewPVCDefinition(dataVolumeName, "50Mi", nil, nil))
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeletePVC(f.K8sClient, f.Namespace.Name, pvc.Name)
			Expect(err).ToNot(HaveOccurred())
		})
		It("[test_id:1759]should fail creating a DataVolume with already existing destination pvc", func() {
			By("Verifying kubectl create")
			Eventually(func() bool {

				_, err := f.RunKubectlCommand("create", "-f", datavolumeTestFile, "-n", f.Namespace.Name)
				return err != nil
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

				_, err := f.RunKubectlCommand("create", "-f", "manifests/dvImageio.yaml", "-n", f.Namespace.Name)
				return err != nil
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

				_, err := f.RunKubectlCommand("create", "-f", "manifests/dvVddk.yaml", "-n", f.Namespace.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())

		})
	})

	Context("when creating data volumes from manual manifests", func() {
		DescribeTable("with manifests Datavolume should", func(destinationFile string, expectError bool, errorContains ...string) {
			By("Verifying kubectl apply")
			out, err := f.RunKubectlCommand("create", "-f", destinationFile, "-n", f.Namespace.Name)
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
			Entry("[test_id:1760]fail with blank image source and contentType archive", "manifests/dvBlankArchive.yaml", true, "SourceType cannot be blank and the contentType be archive"),
			Entry("[test_id:1761]fail with invalid contentType", "manifests/dvInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive", "Unsupported value: \"invalid\": supported values: \"kubevirt\", \"archive\""),
			Entry("[test_id:1762]fail with missing both source and sourceRef", "manifests/dvMissingSource.yaml", true, "Data volume should have either Source or SourceRef", "spec.source in body must be of type object", "spec.sourceRef in body must be of type object"),
			Entry("[test_id:1763]fail with multiple sources", "manifests/dvMultiSource.yaml", true, "Multiple Data volume sources"),
			Entry("[test_id:1764]fail with invalid URL for http source", "manifests/dvInvalidURL.yaml", true, "spec.source Invalid source URL"),
			Entry("[test_id:1765]fail with invalid source PVC", "manifests/dvInvalidSourcePVC.yaml", true, "spec.source.pvc.name in body is required", "spec.source.pvc.name: Required value", "missing required field \"name\" in io.kubevirt.cdi.v1beta1.DataVolume.spec.source.pvc"),
			Entry("[test_id:1766][posneg:positive]succeed with valid source http", "manifests/datavolume.yaml", false, ""),
			Entry("[test_id:1767]fail with missing PVC spec", "manifests/dvMissingPVCSpec.yaml", true, "Missing Data volume PVC", "missing required field \"pvc\" in io.kubevirt.cdi.v1beta1.DataVolume.spec", "invalid: spec.pvc: Required value"),
			Entry("[test_id:3920]fail with missing PVC accessModes", "manifests/dvMissingPVCAccessModes.yaml", true, "spec.pvc.accessModes in body is required", "spec.pvc.accessModes: Required value", "Required value: at least 1 access mode is required"),
			Entry("[test_id:1768]fail with missing resources spec", "manifests/dvMissingResourceSpec.yaml", true, "spec.pvc.resources in body is required", "spec.pvc.resources: Required value", "PVC size is missing"),
			Entry("[test_id:3921]fail with missing PVC size", "manifests/dvMissingPVCSize.yaml", true, "PVC size is missing", "spec.pvc.resources.requests in body must be of type object"),
			Entry("[test_id:1769]fail with 0 size PVC", "manifests/dv0SizePVC.yaml", true, "PVC size can't be equal or less than zero"),
			Entry("[test_id:1937]fail with invalid content type on blank image", "manifests/dvBlankInvalidContentType.yaml", true, "ContentType not one of: kubevirt, archive", "Unsupported value: \"test\": supported values: \"kubevirt\", \"archive\""),
			Entry("[test_id:1931][posneg:positive]succeed with leading zero in requests storage size", "manifests/dvLeadingZero.yaml", false, ""),
			Entry("[test_id:1925]fail with invalid request storage size", "manifests/dvInvalidStorageSizeQuantity.yaml", true, "quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"),
			Entry("[test_id:1923]fail with missing storage size", "manifests/dvMissingRequestSpec.yaml", true, "PVC size is missing", "spec.pvc.resources in body must be of type object"),
			Entry("[test_id:1915]fail with invalid access modes", "manifests/dvInvalidAccessModes.yaml", true, "supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\""),
			Entry("[test_id:3922]fail with multiple access modes", "manifests/dvMultipleAccessModes.yaml", true, "PVC multiple accessModes"),
			Entry("[test_id:1861]fail with missing both source and sourceRef (but having both keys)", "manifests/dvMissingSource2.yaml", true, "Data volume should have either Source or SourceRef"),
			Entry("[test_id:1860]fail with missing http url key", "manifests/dvMissingSourceHttp.yaml", true, "Missing Data volume source", "spec.source.http in body must be of type object"),
			Entry("[test_id:1858]fail with missing datavolume spec", "manifests/dvMissingCompleteSpec.yaml", true, "Missing Data volume source", "missing required field \"spec\" in io.kubevirt.cdi.v1beta1.DataVolume", " invalid: spec: Required value"),
			// k8s < 1.15 return Required value: name or generateName is required, >= 1.15 return error validating data: unknown object type "nil" in DataVolume.metadata
			Entry("[test_id:1857]fail without datavolume name", "manifests/dvNoName.yaml", true, "Required value: name or generateName is required", "error validating data: unknown object type \"nil\" in DataVolume.metadata"),
			Entry("[test_id:1856]fail without meta data", "manifests/dvNoMetaData.yaml", true, "Required value: name or generateName is required"),
			Entry("[test_id:6786]fail with both source and sourceRef", "manifests/dvBothSourceAndSourceRef.yaml", true, "Data volume should have either Source or SourceRef"),
		)

		It("[test_id:4895][posneg:positive]report progress while importing 1024Mi PVC", func() {
			By("Verifying kubectl create")
			out, err := f.RunKubectlCommand("create", "-f", "manifests/out/dv1024MiPVC.yaml", "-n", f.Namespace.Name)
			fmt.Fprintf(GinkgoWriter, "INFO: Output from kubectl: %s\n", out)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying pvc was created")
			pvc, err := utils.WaitForPVC(f.K8sClient, f.Namespace.Name, dataVolumeName)
			Expect(err).ToNot(HaveOccurred())
			f.ForceBindIfWaitForFirstConsumer(pvc)

			//Due to the rate limit, this will take a while, so we can expect the phase to be in progress.
			By(fmt.Sprintf("Waiting for datavolume to match phase %s", string(cdiv1.ImportInProgress)))
			err = utils.WaitForDataVolumePhase(f, f.Namespace.Name, cdiv1.ImportInProgress, dataVolumeName)
			if err != nil {
				f.PrintControllerLog()
				dv, dverr := f.CdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
				Expect(dverr).ToNot(HaveOccurred(), "datavolume %s phase %s", dv.Name, dv.Status.Phase)
			}
			Expect(err).ToNot(HaveOccurred())
			progressRegExp := regexp.MustCompile(`\d{1,3}\.?\d{1,2}%`)
			By("Waiting for datavolume to indicate progress")
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

	Describe("Populator validation", func() {
		Context("when creating", func() {
			populatorSource := map[string]interface{}{}

			AfterEach(func() {
				err := os.Remove(populatorDestinationFile)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("volumeimportsource should", func(contentType, sourceType string, args ...string) {
				By("Reading yaml file from: " + importPopulatorTestFile)
				err := yamlFiletoStruct(importPopulatorTestFile, &populatorSource)
				Expect(err).ToNot(HaveOccurred())

				setSourceType(populatorSource, sourceType, args)
				populatorSource["spec"].(map[string]interface{})["contentType"] = contentType
				err = structToYamlFile(populatorDestinationFile, populatorSource)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					_, err := f.RunKubectlCommand("create", "-f", populatorDestinationFile, "-n", f.Namespace.Name)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())

			},
				Entry("fail with no source", "", ""),
				Entry("fail with http source with invalid url format", "", "http", invalidURLFormat),
				Entry("fail with http source with empty url", "http", "", ""),
				Entry("fail with s3 source with invalid url format", "", "s3", invalidURLFormat),
				Entry("fail with s3 source with empty url", "", "s3", ""),
				Entry("fail with empty Imageio source diskId", "", "imageio", validURL, "secret", "tls-cert", ""),
				Entry("fail with empty VDDK source UUID", "", "vddk", validURL, "secret", "", "backingfile", "thumbprint"),
				Entry("fail with empty VDDK source backing file", "", "vddk", validURL, "secret", "uuid", "", "thumbprint"),
				Entry("fail with empty VDDK source thumbprint", "", "vddk", validURL, "secret", "uuid", "backingfile", ""),
				Entry("fail with invalid content type", "invalid", "http", validURL),
				Entry("succeed with valid http source", "", "http", validURL),
			)

			It("volumeimportsource should fail when multiple sources are set", func() {
				By("Reading yaml file from: " + importPopulatorTestFile)
				err := yamlFiletoStruct(importPopulatorTestFile, &populatorSource)
				Expect(err).ToNot(HaveOccurred())

				populatorSource["spec"].(map[string]interface{})["source"] = map[string]interface{}{"http": map[string]interface{}{"url": "http://foo.bar"}}
				populatorSource["spec"].(map[string]interface{})["source"] = map[string]interface{}{"s3": map[string]interface{}{"url": "http://foo.bar"}}
				err = structToYamlFile(populatorDestinationFile, populatorSource)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					_, err := f.RunKubectlCommand("create", "-f", populatorDestinationFile, "-n", f.Namespace.Name)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())

			})

			DescribeTable("volumeuploadsource should", func(contentType string) {
				By("Reading yaml file from: " + uploadPopulatorTestFile)
				err := yamlFiletoStruct(uploadPopulatorTestFile, &populatorSource)
				Expect(err).ToNot(HaveOccurred())

				err = structToYamlFile(populatorDestinationFile, populatorSource)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying kubectl create")
				Eventually(func() bool {
					_, err := f.RunKubectlCommand("create", "-f", populatorDestinationFile, "-n", f.Namespace.Name)
					return err != nil
				}, timeout, pollingInterval).Should(BeTrue())
			},
				Entry("fail with invalid content type", "invalid"),
				Entry("succeed with empty content type", ""),
				Entry("succeed with archive content type", "archive"),
				Entry("succeed with kubevirt content type", "kubevirt"),
			)
		})

		Context("when updating", func() {
			It("should fail when updating volumeImportSource spec", func() {
				By("Creating Import Populator CR with HTTP source")
				importSource := &cdiv1.VolumeImportSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "import-populator-test",
						Namespace: f.Namespace.Name,
					},
					Spec: cdiv1.VolumeImportSourceSpec{
						Source: &cdiv1.ImportSourceType{Blank: &cdiv1.DataVolumeBlankImage{}},
					},
				}
				importSource, err := f.CdiClient.CdiV1beta1().VolumeImportSources(f.Namespace.Name).Create(
					context.TODO(), importSource, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				updatedImportSource := importSource.DeepCopy()
				updatedImportSource.Spec.ContentType = cdiv1.DataVolumeArchive
				_, err = f.CdiClient.CdiV1beta1().VolumeImportSources(updatedImportSource.Namespace).Update(context.TODO(), updatedImportSource, metav1.UpdateOptions{})
				Expect(err).To(HaveOccurred())
			})

			It("should fail when updating volumeUploadSource spec", func() {
				uploadSource := &cdiv1.VolumeUploadSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "upload-populator-test",
						Namespace: f.Namespace.Name,
					},
					Spec: cdiv1.VolumeUploadSourceSpec{},
				}
				uploadSource, err := f.CdiClient.CdiV1beta1().VolumeUploadSources(f.Namespace.Name).Create(
					context.TODO(), uploadSource, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				updatedUploadSource := uploadSource.DeepCopy()
				updatedUploadSource.Spec.ContentType = cdiv1.DataVolumeArchive
				_, err = f.CdiClient.CdiV1beta1().VolumeUploadSources(updatedUploadSource.Namespace).Update(context.TODO(), updatedUploadSource, metav1.UpdateOptions{})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func yamlFiletoStruct(fileName string, o *map[string]interface{}) error {
	yamlFile, err := os.ReadFile(fileName)
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

	err = os.WriteFile(fileName, yamlOutput, 0600)
	if err != nil {
		return err
	}

	return nil
}
