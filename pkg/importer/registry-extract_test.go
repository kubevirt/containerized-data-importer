package importer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containers/image/v5/types"
)

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

type scriptedDisk struct {
	outcomes []scriptedRead
	reads    *int
}

type scriptedRead struct {
	found bool
	err   error
}

func (d scriptedDisk) resolve(context.Context, *types.SystemContext, types.ImageSource, types.BlobInfoCache) ([]types.BlobInfo, *types.ImageInspectInfo, error) {
	return nil, nil, nil
}

func (d scriptedDisk) read(_ context.Context, layer types.BlobInfo, _ blobOpener) (bool, error) {
	*d.reads++
	outcome := d.outcomes[layer.Size]
	return outcome.found, outcome.err
}

var _ = Describe("Registry disk extraction", func() {
	Context("from candidate layers", func() {
		var (
			found      = scriptedRead{found: true}
			missing    = scriptedRead{}
			unreadable = scriptedRead{err: fmt.Errorf("%w: layer unavailable", errReadingLayer)}
			broken     = scriptedRead{err: errors.New("disk full")}
		)

		extract := func(outcomes ...scriptedRead) (error, int) {
			reads := 0
			candidates := make([]types.BlobInfo, len(outcomes))
			for i := range candidates {
				candidates[i].Size = int64(i)
			}
			err := extractDisk(context.Background(), scriptedDisk{outcomes: outcomes, reads: &reads}, candidates, nil)
			return err, reads
		}

		DescribeTable("should stop at the first candidate holding the disk image", func(outcomes []scriptedRead, reads int) {
			err, read := extract(outcomes...)
			Expect(err).ToNot(HaveOccurred())
			Expect(read).To(Equal(reads))
		},
			Entry("when it is the first", []scriptedRead{found, missing}, 1),
			Entry("past one that does not hold it", []scriptedRead{missing, found, missing}, 2),
			Entry("past one that cannot be read", []scriptedRead{unreadable, found}, 2),
		)

		It("should stop at a failure that is not a read error", func() {
			err, read := extract(broken, found)
			Expect(err).To(MatchError("disk full"))
			Expect(read).To(Equal(1))
		})

		DescribeTable("should report that no candidate holds the disk image", func(outcomes ...scriptedRead) {
			err, _ := extract(outcomes...)
			Expect(err).To(MatchError(errDiskImageNotFound))
			Expect(err).ToNot(MatchError(errReadingLayer))
		},
			Entry("when there are none"),
			Entry("when none holds it", missing, missing),
		)

		It("should say why when a candidate could not be read", func() {
			err, _ := extract(unreadable, missing)
			Expect(err).To(MatchError(errDiskImageNotFound))
			Expect(err).To(MatchError(errReadingLayer))
		})
	})

	Context("writing the disk image", func() {
		var destDir string

		BeforeEach(func() {
			destDir = GinkgoT().TempDir()
		})

		It("should write the stream under the destination directory", func() {
			Expect(diskWriter{destDir: destDir}.write(bytes.NewReader([]byte("the disk image")), "disk/disk.img")).To(Succeed())
			Expect(os.ReadFile(filepath.Join(destDir, "disk", "disk.img"))).To(ContainSubstring("the disk image"))
		})

		It("should refuse a name escaping the destination directory", func() {
			err := diskWriter{destDir: destDir}.write(bytes.NewReader([]byte("zip slip")), "../escaped.img")
			Expect(err).To(MatchError(ContainSubstring("content filepath is tainted")))
			Expect(filepath.Join(filepath.Dir(destDir), "escaped.img")).ToNot(BeAnExistingFile())
		})
	})
})
