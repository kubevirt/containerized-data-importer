package importer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("Registry artifact import", func() {
	matching := func(annotations map[string]string) *cdiv1.LayerSelector {
		return &cdiv1.LayerSelector{MatchAnnotations: annotations}
	}

	Context("selecting the layer", func() {
		const (
			nameAnnotation = "org.example.disk.name"
			sizeAnnotation = "org.example.disk.size"
		)
		named := func(name string) *cdiv1.LayerSelector {
			return matching(map[string]string{nameAnnotation: name})
		}

		rootdisk := types.BlobInfo{Digest: "sha256:root", Annotations: map[string]string{nameAnnotation: "rootdisk", sizeAnnotation: "10Gi"}}
		datadisk := types.BlobInfo{Digest: "sha256:data", Annotations: map[string]string{nameAnnotation: "datadisk", sizeAnnotation: "10Gi"}}
		layers := []types.BlobInfo{rootdisk, datadisk}

		DescribeTable("should select the one layer carrying every annotation", func(selector *cdiv1.LayerSelector, expected types.BlobInfo) {
			Expect(selectLayer(layers, selector)).To(Equal(expected))
		},
			Entry("the first layer", named("rootdisk"), rootdisk),
			Entry("a later layer", named("datadisk"), datadisk),
			Entry("by several annotations", matching(map[string]string{nameAnnotation: "rootdisk", sizeAnnotation: "10Gi"}), rootdisk),
		)

		DescribeTable("should fail when the annotations select no layer", func(selector *cdiv1.LayerSelector) {
			_, err := selectLayer(layers, selector)
			Expect(err).To(MatchError(errLayerNotFound))
		},
			Entry("when no layer carries the annotation", matching(map[string]string{"org.example.disk.unknown": "rootdisk"})),
			Entry("when no layer annotation has the value", named("unknowndisk")),
			Entry("when only some of the annotations match", matching(map[string]string{nameAnnotation: "rootdisk", sizeAnnotation: "5Gi"})),
		)

		It("should refuse a selector with no annotations rather than match every layer", func() {
			_, err := selectLayer([]types.BlobInfo{rootdisk}, matching(nil))
			Expect(err).To(MatchError(errEmptyLayerSelector))
		})

		It("should fail when the annotations select more than one layer", func() {
			_, err := selectLayer(layers, matching(map[string]string{sizeAnnotation: "10Gi"}))
			Expect(err).To(MatchError(errLayerAmbiguous))
			Expect(err).To(MatchError(ContainSubstring("match 2 layers (sha256:root, sha256:data)")))
		})
	})

	Context("extracting the disk image", func() {
		rawDisk := func(content string) []byte {
			return bytes.Repeat([]byte(content), 1024)
		}

		var destDir string

		BeforeEach(func() {
			destDir = GinkgoT().TempDir()
		})

		artifact := func() artifactDisk {
			return artifactDisk{disk: diskWriter{destDir: destDir}, pathPrefix: "disk"}
		}
		diskImage := func() string {
			return filepath.Join(destDir, "disk", "disk.img")
		}

		It("should write out the whole blob of the layer", func() {
			layers, open := openLayers(rawDisk("root"))

			Expect(extractDisk(context.Background(), artifact(), layers, open)).To(Succeed())
			Expect(os.ReadFile(diskImage())).To(Equal(rawDisk("root")))
		})

		It("should fail on the layer being unreadable rather than report it holds no disk", func() {
			layers, open := openLayers(nil)

			err := extractDisk(context.Background(), artifact(), layers, open)
			Expect(err).To(MatchError(ContainSubstring("layer unavailable")))
			Expect(err).ToNot(MatchError(errDiskImageNotFound))
			Expect(diskImage()).ToNot(BeAnExistingFile())
		})
	})

	Context("from a registry", func() {
		// the fixtures follow the layout KubeVirt exports, which names each disk layer by this annotation
		const diskNameAnnotation = "io.kubevirt.disk.name"
		diskNamed := func(name string) *cdiv1.LayerSelector {
			return matching(map[string]string{diskNameAnnotation: name})
		}

		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		diskImage := func() string {
			return filepath.Join(tmpDir, containerDiskImageDir, artifactDiskImageName)
		}

		When("the artifact is an image index", func() {
			// an OCI artifact: an image index with an amd64 manifest holding a rootdisk and a
			// datadisk raw+zstd layer, and an arm64 manifest holding a rootdisk layer.
			ociArtifactSource := "oci-archive:" + filepath.Join(imageDir, "oci-disk-artifact.tar")
			const (
				amd64RootDiskChecksum = "sha256:f670e4e408beaec3329c14a99ebee6a3fd23ecbde36386ed0da99b51218e66ef"
				amd64DataDiskChecksum = "sha256:aa8b53f1d73aa7fe15b0cb0a07bbf97e3eae4e216bd4cccdee3446f0742fe8c4"
				arm64RootDiskChecksum = "sha256:d444acc912c6ada37afb62f63d079e0cc95f4d32b95be232a897d8cd42d1c2d5"
			)

			DescribeTable("should import the selected layer of the manifest for the architecture", func(architecture string, selector *cdiv1.LayerSelector, checksum string) {
				info, err := (&RegistryDataSource{endpoint: ociArtifactSource, imageArchitecture: architecture, layer: selector}).copyImage(tmpDir, containerDiskImageDir, false)
				Expect(err).ToNot(HaveOccurred())
				// an OCI artifact layer is the disk image, there is no image to inspect for labels
				Expect(info).To(BeNil())

				data, err := os.ReadFile(diskImage())
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.FromBytes(data)).To(Equal(digest.Digest(checksum)))
			},
				Entry("rootdisk of the amd64 manifest", "amd64", diskNamed("rootdisk"), amd64RootDiskChecksum),
				Entry("datadisk of the amd64 manifest", "amd64", diskNamed("datadisk"), amd64DataDiskChecksum),
				Entry("rootdisk of the arm64 manifest", "arm64", diskNamed("rootdisk"), arm64RootDiskChecksum),
			)

			DescribeTable("should fail to resolve the layer", func(architecture string, selector *cdiv1.LayerSelector) {
				_, err := (&RegistryDataSource{endpoint: ociArtifactSource, imageArchitecture: architecture, layer: selector}).copyImage(tmpDir, containerDiskImageDir, false)
				Expect(err).To(HaveOccurred())
				Expect(diskImage()).ToNot(BeAnExistingFile())
			},
				Entry("when the layer belongs to another architecture's manifest", "arm64", diskNamed("datadisk")),
				Entry("when the image index has no manifest for the architecture", "invalid", diskNamed("rootdisk")),
			)

			It("should be what Transfer imports when a layer is selected", func() {
				ds := NewRegistryDataSource(ociArtifactSource, "", "", "amd64", diskNamed("rootdisk"), "", false)
				phase, err := ds.Transfer(tmpDir, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(phase).To(Equal(ProcessingPhaseConvert))
				Expect(ds.GetURL().Path).To(Equal(diskImage()))
				Expect(ds.GetTerminationMessage()).To(BeNil())
			})
		})

		When("the artifact is a plain manifest", func() {
			plainSource := "oci-archive:" + filepath.Join(imageDir, "oci-disk-artifact-manifest.tar")
			const plainRootDiskChecksum = "sha256:e275c4159e6bf00278fc526c4aac2fe32aa12b8411b96ffcaab5c49d40d5b019"

			DescribeTable("should import the selected layer", func(architecture string) {
				rd := &RegistryDataSource{endpoint: plainSource, imageArchitecture: architecture, layer: diskNamed("rootdisk")}
				_, err := rd.copyImage(tmpDir, containerDiskImageDir, false)
				Expect(err).ToNot(HaveOccurred())

				data, err := os.ReadFile(diskImage())
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.FromBytes(data)).To(Equal(digest.Digest(plainRootDiskChecksum)))
			},
				Entry("without an architecture requested", ""),
				// a plain manifest declares no platform to check the request against
				Entry("with an architecture requested", "arm64"),
			)
		})
	})
})
