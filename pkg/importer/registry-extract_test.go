package importer

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containers/image/v5/types"
)

// tarEntry is one file to place in a synthetic image layer.
type tarEntry struct {
	name     string
	typeflag byte
	content  string
}

// tarLayer builds an uncompressed tar blob holding the given entries.
func tarLayer(entries ...tarEntry) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		Expect(tw.WriteHeader(&tar.Header{
			Name:     entry.name,
			Typeflag: typeflag,
			Mode:     0644,
			Size:     int64(len(entry.content)),
		})).To(Succeed())
		_, err := tw.Write([]byte(entry.content))
		Expect(err).ToNot(HaveOccurred())
	}
	Expect(tw.Close()).To(Succeed())
	return buf.Bytes()
}

// openLayers serves the given layer bodies in the order extract walks them. A nil body
// stands for a layer the registry cannot hand over.
func openLayers(bodies ...[]byte) ([]types.BlobInfo, blobOpener) {
	next := 0
	open := func(context.Context, types.BlobInfo) (io.ReadCloser, error) {
		body := bodies[next]
		next++
		if body == nil {
			return nil, errors.New("layer unavailable")
		}
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return make([]types.BlobInfo, len(bodies)), open
}

var _ = Describe("File extractor", func() {
	var destDir string
	var extractor fileExtractor

	BeforeEach(func() {
		var err error
		destDir, err = os.MkdirTemp("", "extract")
		Expect(err).ToNot(HaveOccurred())
		extractor = fileExtractor{destDir: destDir, pathPrefix: containerDiskImageDir}
	})

	AfterEach(func() {
		os.RemoveAll(destDir)
	})

	extractedDiskImage := func() string {
		content, err := os.ReadFile(filepath.Join(destDir, containerDiskImageDir, "disk.img"))
		Expect(err).ToNot(HaveOccurred())
		return string(content)
	}

	It("should extract the first file under the path prefix", func() {
		layers, open := openLayers(tarLayer(
			tarEntry{name: "etc/hosts", content: "ignored, wrong prefix"},
			tarEntry{name: "disk/disk.img", content: "the disk image"},
		))

		Expect(extractor.extract(context.Background(), layers, open)).To(Succeed())
		Expect(extractedDiskImage()).To(ContainSubstring("the disk image"))
	})

	It("should stop at the first layer holding a match", func() {
		layers, open := openLayers(
			tarLayer(tarEntry{name: "disk/disk.img", content: "from the lower layer"}),
			tarLayer(tarEntry{name: "disk/disk.img", content: "from the upper layer"}),
		)

		Expect(extractor.extract(context.Background(), layers, open)).To(Succeed())
		Expect(extractedDiskImage()).To(ContainSubstring("from the lower layer"))
	})

	It("should skip whiteout markers and directories", func() {
		layers, open := openLayers(tarLayer(
			tarEntry{name: "disk/", typeflag: tar.TypeDir},
			tarEntry{name: "disk/" + whFilePrefix + "removed.img", content: "deleted upstream"},
			tarEntry{name: "disk/disk.img", content: "the disk image"},
		))

		Expect(extractor.extract(context.Background(), layers, open)).To(Succeed())
		Expect(extractedDiskImage()).To(ContainSubstring("the disk image"))
		Expect(filepath.Join(destDir, containerDiskImageDir, whFilePrefix+"removed.img")).ToNot(BeAnExistingFile())
	})

	DescribeTable("should skip a layer it cannot read", func(bad []byte) {
		layers, open := openLayers(bad, tarLayer(tarEntry{name: "disk/disk.img", content: "the disk image"}))

		Expect(extractor.extract(context.Background(), layers, open)).To(Succeed())
		Expect(extractedDiskImage()).To(ContainSubstring("the disk image"))
	},
		Entry("when the blob cannot be fetched", nil),
		Entry("when the blob is not a readable archive", bytes.Repeat([]byte("junk"), 256)),
	)

	It("should fail when no layer holds a matching file", func() {
		layers, open := openLayers(tarLayer(tarEntry{name: "etc/hosts", content: "no disk here"}))

		err := extractor.extract(context.Background(), layers, open)
		Expect(err).To(MatchError("Failed to find VM disk image file in the container image"))
	})

	It("should refuse a file path escaping the destination directory", func() {
		layers, open := openLayers(tarLayer(tarEntry{name: "disk/../../escaped.img", content: "zip slip"}))

		err := extractor.extract(context.Background(), layers, open)
		Expect(err).To(MatchError(ContainSubstring("content filepath is tainted")))
		Expect(filepath.Join(filepath.Dir(destDir), "escaped.img")).ToNot(BeAnExistingFile())
	})
})
