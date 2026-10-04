package importer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"

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

	Context("parsing the artifact manifest", func() {
		ociManifest := func(artifactType string) []byte {
			manBlob, err := json.Marshal(imgspecv1.Manifest{
				Versioned:    specs.Versioned{SchemaVersion: 2},
				MediaType:    imgspecv1.MediaTypeImageManifest,
				ArtifactType: artifactType,
				Config:       imgspecv1.DescriptorEmptyJSON,
			})
			Expect(err).ToNot(HaveOccurred())
			return manBlob
		}

		DescribeTable("should accept the KubeVirt artifact types", func(artifactType string) {
			man, err := parseOCIManifest(ociManifest(artifactType), imgspecv1.MediaTypeImageManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(man.ArtifactType).To(Equal(artifactType))
		},
			Entry("a virtual machine", "application/vnd.kubevirt.virtualmachine.v1"),
			Entry("a virtual machine template", "application/vnd.kubevirt.virtualmachinetemplate.v1"),
		)

		DescribeTable("should refuse any other manifest", func(artifactType string) {
			_, err := parseOCIManifest(ociManifest(artifactType), imgspecv1.MediaTypeImageManifest)
			Expect(err).To(MatchError(errArtifactType))
		},
			Entry("a container image, declaring no artifact type", ""),
			Entry("an artifact of another type", "application/vnd.example.disk.v1"),
			Entry("an artifact of an unknown version", "application/vnd.kubevirt.virtualmachine.v2"),
		)
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

		DescribeTable("should refuse a tar layer rather than import the archive as a raw disk", func(compress func([]byte) []byte) {
			// random content keeps the compressed layer longer than the header the format is detected by
			content := make([]byte, 4096)
			Expect(rand.Read(content)).To(Equal(len(content)))
			var archive bytes.Buffer
			tw := tar.NewWriter(&archive)
			Expect(tw.WriteHeader(&tar.Header{Name: "disk.img", Mode: 0600, Size: int64(len(content))})).To(Succeed())
			Expect(tw.Write(content)).To(Equal(len(content)))
			Expect(tw.Close()).To(Succeed())
			layers, open := openLayers(compress(archive.Bytes()))

			err := extractDisk(context.Background(), artifact(), layers, open)
			Expect(err).To(MatchError(errLayerTar))
			Expect(diskImage()).ToNot(BeAnExistingFile())
		},
			Entry("an uncompressed tar", func(b []byte) []byte { return b }),
			Entry("a gzipped tar", func(b []byte) []byte {
				var gz bytes.Buffer
				zw := gzip.NewWriter(&gz)
				_, err := zw.Write(b)
				Expect(err).ToNot(HaveOccurred())
				Expect(zw.Close()).To(Succeed())
				return gz.Bytes()
			}),
		)

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
				ds, err := NewRegistryDataSource(ociArtifactSource, "", "", "amd64", `{"`+diskNameAnnotation+`":"rootdisk"}`, "", false)
				Expect(err).ToNot(HaveOccurred())
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

			It("should import the selected layer", func() {
				rd := &RegistryDataSource{endpoint: plainSource, layer: diskNamed("rootdisk")}
				_, err := rd.copyImage(tmpDir, containerDiskImageDir, false)
				Expect(err).ToNot(HaveOccurred())

				data, err := os.ReadFile(diskImage())
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.FromBytes(data)).To(Equal(digest.Digest(plainRootDiskChecksum)))
			})

			It("should refuse a requested architecture it cannot check", func() {
				rd := &RegistryDataSource{endpoint: plainSource, imageArchitecture: "amd64", layer: diskNamed("rootdisk")}
				_, err := rd.copyImage(tmpDir, containerDiskImageDir, false)
				Expect(err).To(MatchError(errPlatformUndeclared))
				Expect(diskImage()).ToNot(BeAnExistingFile())
			})
		})
	})
})
