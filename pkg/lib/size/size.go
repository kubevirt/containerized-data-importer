package size

import (
	"io"

	"github.com/kubevirt/containerized-data-importer/pkg/importer"
	"github.com/pkg/errors"
)

// Return the size in bytes of the provided endpoint. If the endpoint was archived, compressed or converted to
// qcow2 the original image size is returned.
func ImageSize(endpoint, accessKey, secKey string) (int64, error) {
	ds, err := importer.NewDataStream(endpoint, accessKey, secKey)
	if err != nil {
		return -1, errors.Wrapf(err, "unable to create data stream")
	}
	defer importer.CloseReaders(ds.Readers)
	rdr := ds.Readers[len(ds.Readers)-1]
	return Size(rdr, ds.Qemu)
}

// Return the size of the endpoint corresponding to the passed-in reader.
func Size(rdr io.ReadCloser, qemu bool) (int64, error) {
	// TODO: figure out the size!
	return 0, nil
}
