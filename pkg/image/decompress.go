package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/xi2/xz"
)

func GzReader(r io.ReadCloser) (io.ReadCloser, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("GzReader: error creating gz Reader: %v\n", err)
	}
	glog.V(Vadmin).Infof("gzip: extracting %q\n", gz.Name)
	return gz, nil
}

func XzReader(r io.ReadCloser) (io.ReadCloser, error) {
	glog.V(Vdebug).Infoln("XzReader: xz format")
	xz, err := xz.NewReader(r, 0) //note: default dict size may be too small
	if err != nil {
		return nil, fmt.Errorf("XzReader: error creating xz Reader: %v\n", err)
	}
	return ioutil.NopCloser(xz), nil
}

func TarReader(r io.ReadCloser) (io.ReadCloser, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return r, fmt.Errorf("TarReader: reading tarfile header: %v\n", err)
	}
	glog.V(Vadmin).Infof("tar: extracting %q\n", hdr.Name)
	return ioutil.NopCloser(tr), nil
}
